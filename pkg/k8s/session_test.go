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
	"strings"
	"sync"
	"testing"
	"time"

	crinformersv1 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v1/informers/externalversions"
	crinformersv3 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/informers/externalversions"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/client-go/tools/cache"
)

func fakeStarter() sessionStarter {
	return func(namespace string, epoch uint64, stopCh chan struct{}) (*nsSession, error) {
		return &nsSession{
			namespace: namespace,
			epoch:     epoch,
			stopCh:    stopCh,
			shutdown:  func() {},
		}, nil
	}
}

func drainReady(ch <-chan k8ssync.SyncDataEvent, n int, timeout time.Duration) int {
	got := 0
	deadline := time.After(timeout)
	for got < n {
		select {
		case ev := <-ch:
			if ev.EventProcessed != nil {
				close(ev.EventProcessed)
			}
			if ev.SyncType == k8ssync.NAMESPACE_SESSION_READY {
				got++
			}
		case <-deadline:
			return got
		}
	}
	return got
}

func TestNewSessionManagerBindsStarterAfterSessions(t *testing.T) {
	t.Parallel()
	k := &k8s{osArgs: utils.OSArgs{NamespaceLabelSelector: "app=watch"}}
	ch := make(chan k8ssync.SyncDataEvent, 1)
	got := k.NewSessionManager(ch, false)
	if got == nil {
		t.Fatal("selector-only NewSessionManager must return a manager")
	}
	if k.sessions == nil || k.sessions.starter == nil {
		t.Fatal("sessions and starter must both be set")
	}
	defer func() {
		if rec := recover(); rec != nil {
			t.Logf("starter panicked without a client (expected): %v", rec)
		}
	}()
	_, err := k.sessions.starter("app", 1, make(chan struct{}))
	if err != nil && strings.Contains(err.Error(), "session manager is nil") {
		t.Fatalf("method-value bind copied k before sessions was assigned: %v", err)
	}
}

func TestSessionManagerStartStopEpoch(t *testing.T) {
	t.Parallel()
	ch := make(chan k8ssync.SyncDataEvent, 8)
	m := newSessionManager(ch, fakeStarter())

	if err := m.Start("app"); err != nil {
		t.Fatal(err)
	}
	if err := m.Start("app"); err != nil {
		t.Fatal(err)
	}
	if got := drainReady(ch, 1, 2*time.Second); got != 1 {
		t.Fatalf("ready events = %d, want 1", got)
	}
	if !m.Accept("app", 1) {
		t.Fatal("current epoch 1 must be accepted")
	}
	if m.Accept("app", 99) {
		t.Fatal("stale epoch must be rejected")
	}

	m.Stop("app")
	if m.Accept("app", 1) {
		t.Fatal("stopped session must reject")
	}

	if err := m.Start("app"); err != nil {
		t.Fatal(err)
	}
	if got := drainReady(ch, 1, 2*time.Second); got != 1 {
		t.Fatalf("second generation ready = %d", got)
	}
	if m.Accept("app", 1) {
		t.Fatal("old generation must be rejected after rematch")
	}
	if !m.Accept("app", 2) {
		t.Fatal("new generation must be accepted")
	}
}

func TestStartPublishesSessionBeforeInformersRun(t *testing.T) {
	t.Parallel()
	ch := make(chan k8ssync.SyncDataEvent, 8)
	ran := make(chan struct{})
	m := newSessionManager(ch, nil)
	m.starter = func(namespace string, epoch uint64, stopCh chan struct{}) (*nsSession, error) {
		if !m.Accept(namespace, epoch) {
			t.Error("session must be visible to Accept before informers start")
		}
		return &nsSession{
			namespace: namespace,
			epoch:     epoch,
			stopCh:    stopCh,
			shutdown:  func() {},
			run:       func() { close(ran) },
		}, nil
	}
	if err := m.Start("app"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("run was not called")
	}
	if !m.Accept("app", 1) {
		t.Fatal("session must remain accepted after run")
	}
	m.Stop("app")
	select {
	case ev := <-ch:
		if ev.EventProcessed != nil {
			close(ev.EventProcessed)
		}
	default:
	}
}

func TestStopDuringStartPreventsInformersRun(t *testing.T) {
	t.Parallel()
	ch := make(chan k8ssync.SyncDataEvent, 8)
	inStarter := make(chan struct{})
	release := make(chan struct{})
	ran := make(chan struct{})
	m := newSessionManager(ch, nil)
	m.starter = func(namespace string, epoch uint64, stopCh chan struct{}) (*nsSession, error) {
		close(inStarter)
		<-release
		return &nsSession{
			stopCh:   stopCh,
			shutdown: func() {},
			run:      func() { close(ran) },
		}, nil
	}
	errCh := make(chan error, 1)
	go func() { errCh <- m.Start("app") }()
	<-inStarter
	if !m.Accept("app", 1) {
		t.Fatal("placeholder must be accepted during Start")
	}
	m.Stop("app")
	close(release)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if m.Accept("app", 1) {
		t.Fatal("stopped in-flight session must not remain")
	}
	select {
	case <-ran:
		t.Fatal("informers must not start after Stop")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestStopWaitsForSessionRunSetup(t *testing.T) {
	t.Parallel()
	ch := make(chan k8ssync.SyncDataEvent, 8)
	runStarted := make(chan struct{})
	releaseRun := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			close(releaseRun)
		})
	}
	t.Cleanup(release)

	m := newSessionManager(ch, nil)
	m.starter = func(namespace string, epoch uint64, stopCh chan struct{}) (*nsSession, error) {
		return &nsSession{
			namespace: namespace,
			epoch:     epoch,
			stopCh:    stopCh,
			run: func() {
				close(runStarted)
				<-releaseRun
			},
			shutdown: func() {},
		}, nil
	}

	startDone := make(chan error, 1)
	go func() {
		startDone <- m.Start("app")
	}()
	select {
	case <-runStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("session run setup did not start")
	}

	stopStarted := make(chan struct{})
	stopDone := make(chan struct{})
	go func() {
		close(stopStarted)
		m.Stop("app")
		close(stopDone)
	}()
	<-stopStarted
	select {
	case <-stopDone:
		t.Fatal("Stop returned before session run setup completed")
	case <-time.After(100 * time.Millisecond):
	}

	release()
	select {
	case err := <-startDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after run setup completed")
	}
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after run setup completed")
	}
	if m.Accept("app", 1) {
		t.Fatal("stopped session must not remain accepted")
	}
}

func TestSessionManagerMarkReadyAndClose(t *testing.T) {
	t.Parallel()
	ch := make(chan k8ssync.SyncDataEvent, 8)
	m := newSessionManager(ch, fakeStarter())
	if err := m.Start("app"); err != nil {
		t.Fatal(err)
	}
	go func() { drainReady(ch, 1, 2*time.Second) }()
	if !m.WaitAllReady(make(chan struct{})) {
		t.Fatal("empty stop, session should become ready")
	}
	if !m.MarkReady("app", 1) {
		t.Fatal("MarkReady current epoch")
	}
	if m.MarkReady("app", 2) {
		t.Fatal("MarkReady wrong epoch")
	}

	m.Close()
	if err := m.Start("other"); err != nil {
		t.Fatal(err)
	}
	if m.Accept("other", 1) {
		t.Fatal("Start after Close must be no-op")
	}
	if m.Accept("app", 1) {
		t.Fatal("Close must drop sessions")
	}
}

func TestSessionReadyFollowsInitialProxyEvents(t *testing.T) {
	t.Parallel()
	eventChan := make(chan k8ssync.SyncDataEvent, 4)
	proxy := make(chan k8ssync.SyncDataEvent, 2)
	startStamp := make(chan struct{})
	stampDone := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		<-startStamp
		stampSessionEvents(proxy, eventChan, nil, 7)
		close(stampDone)
	}()

	m := newSessionManager(eventChan, func(namespace string, epoch uint64, stopCh chan struct{}) (*nsSession, error) {
		proxy <- k8ssync.SyncDataEvent{SyncType: k8ssync.CR_TCP, Namespace: namespace, Name: "initial"}
		return &nsSession{
			namespace: namespace,
			epoch:     epoch,
			stopCh:    stopCh,
			proxy:     proxy,
			shutdown: func() {
				close(proxy)
				<-stampDone
				close(shutdownDone)
			},
		}, nil
	})
	if err := m.Start("app"); err != nil {
		t.Fatal(err)
	}
	close(startStamp)

	first := <-eventChan
	second := <-eventChan
	if first.SyncType != k8ssync.CR_TCP || second.SyncType != k8ssync.NAMESPACE_SESSION_READY {
		t.Fatalf("event order = [%s, %s], want [CR_TCP, NAMESPACE_SESSION_READY]", first.SyncType, second.SyncType)
	}
	if second.EventProcessed == nil {
		t.Fatal("READY must carry an EventProcessed barrier")
	}
	close(second.EventProcessed)
	m.Stop("app")
	select {
	case <-shutdownDone:
	case <-time.After(2 * time.Second):
		t.Fatal("session shutdown did not finish")
	}
}

func TestDrainSessionEventsWaitsForPriorProxyEvents(t *testing.T) {
	t.Parallel()
	process := make(chan k8ssync.SyncDataEvent, 4)
	proxy := make(chan k8ssync.SyncDataEvent, 2)
	stop := make(chan struct{})
	sessStop := make(chan struct{})
	sess := &nsSession{proxy: proxy, stopCh: sessStop}

	stampDone := make(chan struct{})
	go func() {
		stampSessionEvents(proxy, process, nil, 4)
		close(stampDone)
	}()

	proxy <- k8ssync.SyncDataEvent{SyncType: k8ssync.CR_TCP, Namespace: "app", Name: "tcp"}

	drained := make(chan bool, 1)
	go func() {
		drained <- drainSessionEvents(sess, process, stop)
	}()

	first := <-process
	second := <-process
	if first.SyncType != k8ssync.CR_TCP {
		t.Fatalf("first = %s, want CR_TCP", first.SyncType)
	}
	if second.SyncType != k8ssync.COMMAND {
		t.Fatalf("second = %s, want COMMAND", second.SyncType)
	}
	if second.EventProcessed == nil {
		t.Fatal("drain COMMAND must carry EventProcessed")
	}
	close(second.EventProcessed)
	select {
	case stopped := <-drained:
		if stopped {
			t.Fatal("process stop was not closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drainSessionEvents did not return after EventProcessed")
	}
	close(proxy)
	<-stampDone
}

func TestDrainSessionEventsReturnsOnSessionStop(t *testing.T) {
	t.Parallel()
	process := make(chan k8ssync.SyncDataEvent)
	proxy := make(chan k8ssync.SyncDataEvent)
	stop := make(chan struct{})
	sessStop := make(chan struct{})
	close(sessStop)
	sess := &nsSession{proxy: proxy, stopCh: sessStop}

	done := make(chan bool, 1)
	go func() {
		done <- drainSessionEvents(sess, process, stop)
	}()
	select {
	case stopped := <-done:
		if stopped {
			t.Fatal("session stop must not report process stop")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drainSessionEvents must return when the session is stopped")
	}
	select {
	case <-process:
		t.Fatal("stopped session must not send a drain COMMAND")
	default:
	}
}

func TestRunCRInformersTracksSessionHandlers(t *testing.T) {
	t.Parallel()
	var regs []cache.ResourceEventHandlerRegistration
	k := k8s{handlerRegs: &regs}
	v1Factory := crinformersv1.NewSharedInformerFactory(nil, 0)
	v3Factory := crinformersv3.NewSharedInformerFactory(nil, 0)
	var synced []cache.InformerSynced
	k.runCRInformers(
		make(chan k8ssync.SyncDataEvent, 2),
		make(chan struct{}),
		"app",
		&synced,
		nil,
		map[string]CRV3{
			"TCP":             NewTCPCRV3(),
			"ValidationRules": NewValidationCRV3(),
		},
		utils.OSArgs{CustomValidationRules: utils.NamespaceValue{Namespace: "app", Name: "rules"}},
		false,
		v1Factory,
		v3Factory,
	)
	if len(regs) != 2 {
		t.Fatalf("handler registrations = %d, want 2", len(regs))
	}
	if len(synced) != 2 {
		t.Fatalf("informer sync funcs = %d, want 2", len(synced))
	}
}

func TestStampSessionEventsSetsEpochOnCRTCP(t *testing.T) {
	t.Parallel()
	in := make(chan k8ssync.SyncDataEvent, 1)
	out := make(chan k8ssync.SyncDataEvent, 1)
	done := make(chan struct{})
	go func() {
		stampSessionEvents(in, out, nil, 7)
		close(done)
	}()
	in <- k8ssync.SyncDataEvent{SyncType: k8ssync.CR_TCP, Namespace: "app", Name: "tcp"}
	close(in)
	ev := <-out
	if ev.NamespaceEpoch != 7 {
		t.Fatalf("NamespaceEpoch = %d, want 7", ev.NamespaceEpoch)
	}
	if ev.SyncType != k8ssync.CR_TCP {
		t.Fatalf("SyncType = %s", ev.SyncType)
	}
	<-done
}

func TestSessionResourceChanUsesProxy(t *testing.T) {
	t.Parallel()
	process := make(chan k8ssync.SyncDataEvent)
	proxy := make(chan k8ssync.SyncDataEvent)
	sess := &nsSession{proxy: proxy, epoch: 3}
	if got := sessionResourceChan(sess, process); got != proxy {
		t.Fatal("late CRD watchers must send on the session proxy so epoch is stamped")
	}
	if got := sessionResourceChan(nil, process); got != process {
		t.Fatal("non-session path must keep the process channel")
	}
}

func TestCloseSessionProxyAfterWaitsBeforeClose(t *testing.T) {
	t.Parallel()
	proxy := make(chan k8ssync.SyncDataEvent)
	var stampWg sync.WaitGroup
	stampWg.Add(1)
	go func() {
		defer stampWg.Done()
		for range proxy {
		}
	}()
	sent := make(chan struct{})
	wait := func() {
		proxy <- k8ssync.SyncDataEvent{SyncType: k8ssync.GATEWAY, Namespace: "app"}
		close(sent)
	}
	closeSessionProxyAfter(wait, &nsSession{proxy: proxy}, &stampWg)
	select {
	case <-sent:
	default:
		t.Fatal("proxy was closed before gateway wait returned")
	}
}

func TestStampSessionEventsUnblocksOnStop(t *testing.T) {
	t.Parallel()
	in := make(chan k8ssync.SyncDataEvent, 1)
	out := make(chan k8ssync.SyncDataEvent) // unbuffered, no receiver
	stop := make(chan struct{})
	in <- k8ssync.SyncDataEvent{SyncType: k8ssync.CR_TCP, Namespace: "app", Name: "tcp"}

	done := make(chan struct{})
	go func() {
		stampSessionEvents(in, out, stop, 1)
		close(done)
	}()

	close(stop)
	close(in)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stampSessionEvents stayed blocked on eventChan after stop")
	}
}

func TestWaitAllReadyReturnsOnStop(t *testing.T) {
	t.Parallel()
	ch := make(chan k8ssync.SyncDataEvent, 8)
	m := newSessionManager(ch, fakeStarter())
	if err := m.Start("app"); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan bool, 1)
	go func() {
		done <- m.WaitAllReady(stop)
	}()
	close(stop)
	select {
	case ready := <-done:
		if ready {
			t.Fatal("WaitAllReady must be false when stop is closed before Ready")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitAllReady did not return after stop")
	}
	// Unblock waitAndSignalReady so the test does not leak the READY waiter.
	select {
	case ev := <-ch:
		if ev.EventProcessed != nil {
			close(ev.EventProcessed)
		}
	default:
	}
}

func TestSessionManagerConcurrentStop(t *testing.T) {
	t.Parallel()
	ch := make(chan k8ssync.SyncDataEvent, 8)
	m := newSessionManager(ch, fakeStarter())
	if err := m.Start("app"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Stop("app")
			m.Stop("app")
		}()
	}
	wg.Wait()
	m.Close()
}
