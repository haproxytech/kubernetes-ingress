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
	"sync"

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
	namespace string
	epoch     uint64
	phase     sessionPhase
	stopCh    chan struct{}
	stopOnce  sync.Once
	handlers  []cache.ResourceEventHandlerRegistration
	shutdown  func()
	crV1      crinformersv1.SharedInformerFactory
	crV3      crinformersv3.SharedInformerFactory
	gw        gatewaynetworking.SharedInformerFactory
	proxy     chan k8ssync.SyncDataEvent
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
	sessions  map[string]*nsSession
	nextEpoch map[string]uint64
	closed    bool
	eventChan chan k8ssync.SyncDataEvent
	starter   sessionStarter
}

func newSessionManager(eventChan chan k8ssync.SyncDataEvent, starter sessionStarter) *sessionManager {
	m := &sessionManager{
		sessions:  map[string]*nsSession{},
		nextEpoch: map[string]uint64{},
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
	m.nextEpoch[namespace]++
	epoch := m.nextEpoch[namespace]
	stopCh := make(chan struct{})
	placeholder := &nsSession{
		namespace: namespace,
		epoch:     epoch,
		phase:     sessionStarting,
		stopCh:    stopCh,
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

	m.mu.Lock()
	current, ok := m.sessions[namespace]
	if m.closed || !ok || current != placeholder {
		m.mu.Unlock()
		if sess.shutdown != nil {
			go sess.shutdown()
		}
		return nil
	}
	m.sessions[namespace] = sess
	// Starting the informer goroutines is part of publishing the session.
	// Keep it serialized with Stop so shutdown cannot overtake run setup.
	if sess.run != nil {
		sess.run()
	}
	m.mu.Unlock()
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
	select {
	case <-sess.stopCh:
		return
	case readyChan <- ev:
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

func (m *sessionManager) shutdownSession(sess *nsSession) {
	if sess == nil {
		return
	}
	sess.stopOnce.Do(func() {
		if sess.stopCh != nil {
			close(sess.stopCh)
		}
	})
	if sess.shutdown != nil {
		go sess.shutdown()
	}
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

func (m *sessionManager) WaitAllReady(stop <-chan struct{}) bool {
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
		allReady := true
		for _, sess := range m.sessions {
			if sess.phase == sessionStarting {
				allReady = false
				break
			}
		}
		if allReady {
			return true
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
// WaitGroup) then closes proxy so the stamp goroutine can exit. wait must
// return only after informers will no longer send on proxy.
func closeSessionProxyAfter(wait func(), proxy chan k8ssync.SyncDataEvent, stampDone *sync.WaitGroup) {
	if wait != nil {
		wait()
	}
	close(proxy)
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

// drainSessionEvents queues a COMMAND on the session proxy (so it is FIFO
// after List events already sent there) and waits until SyncData has
// processed it. Sending COMMAND on processChan directly can overtake events
// still in the proxy. Returns true if process stop is closed.
func drainSessionEvents(sess *nsSession, processChan chan k8ssync.SyncDataEvent, stop <-chan struct{}) bool {
	if sess == nil {
		return false
	}
	ch := sessionResourceChan(sess, processChan)
	if ch == nil {
		return false
	}
	ep := make(chan struct{})
	ev := k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND, EventProcessed: ep}
	var sessionStop <-chan struct{}
	if sess.stopCh != nil {
		sessionStop = sess.stopCh
	}
	select {
	case <-stop:
		return true
	case <-sessionStop:
		return false
	case ch <- ev:
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

func (m *sessionManager) snapshot() []*nsSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*nsSession, 0, len(m.sessions))
	for _, sess := range m.sessions {
		out = append(out, sess)
	}
	return out
}
