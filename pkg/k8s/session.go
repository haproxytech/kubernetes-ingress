// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8s

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	crinformersv1 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v1/informers/externalversions"
	crinformersv3 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/informers/externalversions"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"k8s.io/client-go/tools/cache"
	gatewaynetworking "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
)

type sessionPhase uint8

const (
	sessionStarting sessionPhase = iota
	sessionReady
	sessionStopping
)

type nsSession struct {
	namespace    string
	epoch        uint64
	phase        sessionPhase
	stopCh       chan struct{}
	stopOnce     *sync.Once
	shutdownOnce sync.Once
	handlers     []cache.ResourceEventHandlerRegistration
	shutdown     func()
	crV1         crinformersv1.SharedInformerFactory
	crV3         crinformersv3.SharedInformerFactory
	gw           gatewaynetworking.SharedInformerFactory
	proxy        chan k8ssync.SyncDataEvent
	// proxyMu coordinates non-informer sends with proxy closure.
	proxyMu     sync.RWMutex
	proxyClosed bool
	constructed bool
	// run starts informers. Start calls it only after the session is in the
	// manager map so Accept sees this generation and Stop can cancel it.
	run func()
}

// sessionStarter creates factories and handlers for one namespace. It must not
// start informers or wait for cache sync; Start publishes the session, then
// calls sess.run, then waits for ready.
type sessionStarter func(namespace string, epoch uint64, stopCh chan struct{}) (*nsSession, error)

type sessionManager struct {
	mu        sync.RWMutex
	readyCond *sync.Cond
	// crRegMu serializes sess.run with late CR informer registration so they
	// cannot mutate the same SharedInformerFactory concurrently. It is not
	// sessions.mu: Accept is on the SyncData hot path and must not wait for
	// factory.Start across every session.
	crRegMu  sync.Mutex
	sessions map[string]*nsSession
	// nextEpoch is process-wide so retired namespace names do not accumulate.
	nextEpoch uint64
	closed    bool
	eventChan chan k8ssync.SyncDataEvent
	starter   sessionStarter
}

func newSessionManager(eventChan chan k8ssync.SyncDataEvent, starter sessionStarter) *sessionManager {
	m := &sessionManager{
		sessions:  map[string]*nsSession{},
		eventChan: eventChan,
		starter:   starter,
	}
	m.readyCond = sync.NewCond(&m.mu)
	return m
}

// Start creates a session for namespace if none is current. It does not wait
// for informer sync. The session is published before informers run so Accept
// and Stop observe it during starter(), and a DELETE can cancel an in-flight Start.
func (m *sessionManager) Start(namespace string) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	if _, ok := m.sessions[namespace]; ok {
		m.mu.Unlock()
		return nil
	}
	m.nextEpoch++
	epoch := m.nextEpoch
	stopCh := make(chan struct{})
	placeholder := &nsSession{
		namespace: namespace,
		epoch:     epoch,
		phase:     sessionStarting,
		stopCh:    stopCh,
		stopOnce:  &sync.Once{},
	}
	m.sessions[namespace] = placeholder
	m.mu.Unlock()

	sess, err := m.starter(namespace, epoch, stopCh)
	if err != nil {
		m.dropPlaceholder(namespace, placeholder)
		m.shutdownSession(placeholder)
		return fmt.Errorf("start namespace session %s: %w", namespace, err)
	}
	if sess == nil {
		m.dropPlaceholder(namespace, placeholder)
		m.shutdownSession(placeholder)
		return fmt.Errorf("start namespace session %s: starter returned nil", namespace)
	}
	sess.namespace = namespace
	sess.epoch = epoch
	sess.phase = sessionStarting
	if sess.stopCh == nil {
		sess.stopCh = stopCh
	}
	// The placeholder and constructed session own the same stop channel. Share
	// its Once so Stop or Close during starter cannot make the returned session
	// close an already-closed channel.
	sess.stopOnce = placeholder.stopOnce

	m.mu.Lock()
	current, ok := m.sessions[namespace]
	if m.closed || !ok || current != placeholder {
		m.mu.Unlock()
		m.shutdownSession(sess)
		return nil
	}
	sess.constructed = true
	m.sessions[namespace] = sess
	// factory.Start returns after spawning informer goroutines. Accept's RLock
	// waits only for this one session; late-CR registration across many
	// sessions uses crRegMu so it does not hold sessions.mu. Keep run under
	// sessions.mu so Drain cannot overtake informer setup.
	if sess.run != nil {
		m.crRegMu.Lock()
		sess.run()
		m.crRegMu.Unlock()
	}
	m.mu.Unlock()
	logger.Infof("namespace-label-selector: started watch for namespace %s epoch %d", namespace, epoch)
	go m.waitAndSignalReady(sess)
	return nil
}

func (m *sessionManager) dropPlaceholder(namespace string, placeholder *nsSession) {
	m.mu.Lock()
	if current, ok := m.sessions[namespace]; ok && current == placeholder {
		delete(m.sessions, namespace)
		m.readyCond.Broadcast()
	}
	m.mu.Unlock()
}

func (m *sessionManager) waitAndSignalReady(sess *nsSession) {
	synced := true
	if len(sess.handlers) > 0 {
		fns := make([]cache.InformerSynced, 0, len(sess.handlers))
		for _, h := range sess.handlers {
			if h != nil {
				fns = append(fns, h.HasSynced)
			}
		}
		if len(fns) > 0 {
			synced = cache.WaitForCacheSync(sess.stopCh, fns...)
		}
	}
	if !synced {
		m.mu.RLock()
		stillStarting := !m.closed && m.sessions[sess.namespace] == sess && sess.phase == sessionStarting
		m.mu.RUnlock()
		if stillStarting {
			logger.Warningf("namespace-label-selector: namespace %s epoch %d cache sync stopped before ready", sess.namespace, sess.epoch)
		}
		return
	}
	m.mu.Lock()
	current, ok := m.sessions[sess.namespace]
	stillCurrent := ok && current == sess && current.phase == sessionStarting && !m.closed
	m.mu.Unlock()
	if !stillCurrent {
		return
	}
	done := make(chan struct{})
	ev := k8ssync.SyncDataEvent{
		SyncType:       k8ssync.NAMESPACE_SESSION_READY,
		Namespace:      sess.namespace,
		NamespaceEpoch: sess.epoch,
		EventProcessed: done,
	}
	readyChan := m.eventChan
	if sess.proxy != nil {
		readyChan = sess.proxy
	}
	if !sess.sendEvent(readyChan, ev, nil) {
		return
	}
	select {
	case <-sess.stopCh:
		return
	case <-done:
	}
	m.mu.Lock()
	current, ok = m.sessions[sess.namespace]
	if ok && current == sess && current.phase == sessionStarting && !m.closed {
		sess.phase = sessionReady
		m.readyCond.Broadcast()
	}
	m.mu.Unlock()
}

func (m *sessionManager) Stop(namespace string) {
	m.mu.Lock()
	sess, ok := m.sessions[namespace]
	if ok {
		sess.phase = sessionStopping
		delete(m.sessions, namespace)
		m.readyCond.Broadcast()
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	m.shutdownSession(sess)
}

// Drain stops resource informers for a deleted namespace but keeps the
// session in Stopping until FinishDrain after the RETIRED sentinel.
func (m *sessionManager) Drain(namespace string) bool {
	m.mu.Lock()
	sess, ok := m.sessions[namespace]
	if !ok {
		m.mu.Unlock()
		return false
	}
	if sess.phase == sessionStopping {
		m.mu.Unlock()
		return true
	}
	sess.phase = sessionStopping
	sess.stop()
	m.readyCond.Broadcast()
	m.mu.Unlock()
	logger.Infof("namespace-label-selector: draining watch for namespace %s epoch %d", namespace, sess.epoch)
	go m.drainAndRetire(sess)
	return true
}

func (m *sessionManager) drainAndRetire(sess *nsSession) {
	sess.shutdownAndWait()
	done := make(chan struct{})
	m.eventChan <- k8ssync.SyncDataEvent{
		SyncType:       k8ssync.NAMESPACE_WATCH_RETIRED,
		Namespace:      sess.namespace,
		NamespaceEpoch: sess.epoch,
		EventProcessed: done,
	}
	<-done
}

func (m *sessionManager) FinishDrain(namespace string) {
	m.mu.Lock()
	if sess, ok := m.sessions[namespace]; ok && sess.phase == sessionStopping {
		delete(m.sessions, namespace)
	}
	m.readyCond.Broadcast()
	m.mu.Unlock()
}

func (m *sessionManager) Draining(namespace string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[namespace]
	return ok && sess.phase == sessionStopping
}

func (m *sessionManager) Ready(namespace string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[namespace]
	return ok && sess.phase == sessionReady && !m.closed
}

func (m *sessionManager) shutdownSession(sess *nsSession) {
	if sess == nil {
		return
	}
	sess.stop()
	go sess.shutdownAndWait()
}

func (sess *nsSession) stop() {
	if sess.stopOnce == nil {
		sess.stopOnce = &sync.Once{}
	}
	sess.stopOnce.Do(func() {
		if sess.stopCh != nil {
			close(sess.stopCh)
		}
	})
}

func (sess *nsSession) shutdownAndWait() {
	sess.stop()
	sess.shutdownOnce.Do(func() {
		if sess.shutdown != nil {
			sess.shutdown()
		}
	})
}

func (m *sessionManager) Accept(namespace string, epoch uint64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[namespace]
	return ok && sess.epoch == epoch && sess.phase != sessionStopping && !m.closed
}

func (m *sessionManager) MarkReady(namespace string, epoch uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[namespace]
	if !ok || sess.epoch != epoch || sess.phase == sessionStopping || m.closed {
		return false
	}
	// Phase flips to Ready in waitAndSignalReady after EventProcessed, so
	// WaitAllReady does not return before the store is marked Relevant.
	return true
}

func (m *sessionManager) Close() {
	m.mu.Lock()
	m.closed = true
	all := make([]*nsSession, 0, len(m.sessions))
	for name, sess := range m.sessions {
		sess.phase = sessionStopping
		all = append(all, sess)
		delete(m.sessions, name)
	}
	m.readyCond.Broadcast()
	m.mu.Unlock()
	for _, sess := range all {
		m.shutdownSession(sess)
	}
}

// waitAllReadyStuckLogInterval is how often WaitAllReady reports namespaces
// still in Starting. A single namespace with e.g. partial RBAC would
// otherwise stall the first COMMAND silently.
var waitAllReadyStuckLogInterval = 10 * time.Second

func (m *sessionManager) WaitAllReady(stop <-chan struct{}, timeout time.Duration) bool {
	stopClosed := make(chan struct{})
	defer close(stopClosed)
	go func() {
		select {
		case <-stop:
			m.mu.Lock()
			m.readyCond.Broadcast()
			m.mu.Unlock()
		case <-stopClosed:
		}
	}()

	// Wake the waiter periodically so a namespace stuck in Starting is
	// logged instead of blocking bootstrap without any output.
	wakeDone := make(chan struct{})
	defer close(wakeDone)
	waker := time.NewTicker(waitAllReadyStuckLogInterval)
	go func() {
		defer waker.Stop()
		for {
			select {
			case <-waker.C:
				m.mu.Lock()
				m.readyCond.Broadcast()
				m.mu.Unlock()
			case <-wakeDone:
				return
			}
		}
	}()

	started := time.Now()
	lastLog := started
	if timeout > 0 {
		timer := time.AfterFunc(timeout, func() {
			m.mu.Lock()
			m.readyCond.Broadcast()
			m.mu.Unlock()
		})
		defer timer.Stop()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for {
		select {
		case <-stop:
			return false
		default:
		}
		if m.closed {
			return false
		}
		starting := make([]string, 0, len(m.sessions))
		for name, sess := range m.sessions {
			if sess.phase == sessionStarting {
				starting = append(starting, name)
			}
		}
		if len(starting) == 0 {
			return true
		}
		if timeout > 0 && time.Since(started) >= timeout {
			slices.Sort(starting)
			logger.Errorf("namespace sessions still starting after %s, continuing bootstrap without them: %s",
				timeout, strings.Join(starting, ", "))
			return true
		}
		if since := time.Since(lastLog); since >= waitAllReadyStuckLogInterval {
			lastLog = time.Now()
			slices.Sort(starting)
			logger.Warningf("namespace sessions not ready after %s, still starting: %s",
				time.Since(started).Round(time.Second), strings.Join(starting, ", "))
		}
		m.readyCond.Wait()
	}
}

// stampSessionEvents copies in to out, setting NamespaceEpoch on every event.
// Session informers must send on `in` (the per-session proxy), not the process
// eventChan, so late CRD watchers keep the current generation.
//
// When stop is closed, it stops blocking on out and keeps reading in until
// that channel is closed so informer Shutdown can finish. A nil stop never
// fires, which tests use.
func stampSessionEvents(in <-chan k8ssync.SyncDataEvent, out chan<- k8ssync.SyncDataEvent, stop <-chan struct{}, epoch uint64) {
	forward := func(ev k8ssync.SyncDataEvent) bool {
		ev.NamespaceEpoch = epoch
		select {
		case out <- ev:
			return true
		case <-stop:
			return false
		}
	}
	for {
		select {
		case ev, ok := <-in:
			if !ok {
				return
			}
			if !forward(ev) {
				for range in {
				}
				return
			}
		case <-stop:
			for range in {
			}
			return
		}
	}
}

// closeSessionProxyAfter runs wait (factory Shutdown / gateway informer
// WaitGroup), excludes READY/drain senders, then closes proxy so the stamp
// goroutine can exit.
//
// wait must return only after informers stop sending. For the client-go shared
// informer factories this relies on the Shutdown semantics of
// k8s.io/client-go >= v0.37: sharedInformerFactory.Shutdown waits for
// informer.RunWithContext, whose deferred processor WaitGroup drains the
// handler goroutines before returning (see informers/factory.go and
// tools/cache/shared_informer.go). With an older or replaced implementation
// that returns while a handler send is still in flight, closing proxy here
// would race that send and panic the process (runtime.HandleCrash defaults to
// ReallyCrash=true).
func closeSessionProxyAfter(wait func(), sess *nsSession, stampDone *sync.WaitGroup) {
	if wait != nil {
		wait()
	}
	sess.closeProxy()
	if stampDone != nil {
		stampDone.Wait()
	}
}

// sessionResourceChan is the channel late CR informers must send on. Using the
// process eventChan would leave NamespaceEpoch at 0 and SyncData would drop the
// events.
func sessionResourceChan(sess *nsSession, processChan chan k8ssync.SyncDataEvent) chan k8ssync.SyncDataEvent {
	if sess != nil && sess.proxy != nil {
		return sess.proxy
	}
	return processChan
}

// drainSessionEvents queues a BARRIER on the session proxy (so it is FIFO
// after List events already sent there) and waits until SyncData has
// processed it. A COMMAND here would rebuild HAProxy once per namespace when
// a CRD appears; the periodic sync COMMAND picks up hadChanges instead.
// Sending the barrier on processChan directly can overtake events still in
// the proxy. Returns true if process stop is closed.
func drainSessionEvents(sess *nsSession, processChan chan k8ssync.SyncDataEvent, stop <-chan struct{}) bool {
	if sess == nil {
		return false
	}
	ch := sessionResourceChan(sess, processChan)
	if ch == nil {
		return false
	}
	ep := make(chan struct{})
	ev := k8ssync.SyncDataEvent{SyncType: k8ssync.BARRIER, EventProcessed: ep}
	var sessionStop <-chan struct{}
	if sess.stopCh != nil {
		sessionStop = sess.stopCh
	}
	if !sess.sendEvent(ch, ev, stop) {
		select {
		case <-stop:
			return true
		default:
			return false
		}
	}
	select {
	case <-stop:
		return true
	case <-sessionStop:
		return false
	case <-ep:
		return false
	}
}

// sendEvent holds the proxy open until this send finishes. A stop check alone
// cannot protect a select containing a send on a closed channel.
func (sess *nsSession) sendEvent(ch chan k8ssync.SyncDataEvent, ev k8ssync.SyncDataEvent, stop <-chan struct{}) bool {
	sess.proxyMu.RLock()
	defer sess.proxyMu.RUnlock()
	if sess.proxyClosed {
		return false
	}
	select {
	case <-stop:
		return false
	case <-sess.stopCh:
		return false
	case ch <- ev:
		return true
	}
}

func (sess *nsSession) closeProxy() {
	sess.proxyMu.Lock()
	defer sess.proxyMu.Unlock()
	sess.proxyClosed = true
	close(sess.proxy)
}
