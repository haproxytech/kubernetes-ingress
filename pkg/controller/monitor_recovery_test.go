// Copyright 2026 HAProxy Technologies LLC
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

package controller

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/process"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/metrics"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestSyncDataRetriesFailedConfiguration(t *testing.T) {
	for _, failure := range []string{"", "start", "commit", "reload"} {
		name := failure
		if name == "" {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			c, probe := newRecoveryController(t)
			before := readRecoveryConfig(t, c)
			probe.failAt = failure
			sendRecoveryConfig(t, c, map[string]string{"maxconn": "2345", "timeout-connect": "7s"})
			c.store.UpdateAllIngresses = true
			sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
			first := readRecoveryConfig(t, c)
			if failure == "start" || failure == "commit" {
				require.Equal(t, before, first, "the failed transaction must not commit")
			} else {
				require.Contains(t, first, "maxconn 2345")
				require.Contains(t, first, "timeout connect 7000")
			}
			if failure != "" {
				require.Equal(t, before, probe.applied, "a failed attempt must not reach the process")
				// Only a periodic COMMAND follows, not another resource event.
				sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
			}
			require.Contains(t, probe.applied, "maxconn 2345", "the next COMMAND must apply pending configuration")
			require.Contains(t, probe.applied, "timeout connect 7000")
			require.False(t, c.store.UpdateAllIngresses)
			require.False(t, instance.NeedReload())
			attempts := probe.starts
			reloads := probe.reloads
			sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
			require.Equal(t, attempts, probe.starts, "successful idle COMMAND must not start another transaction")
			require.Equal(t, reloads, probe.reloads)
		})
	}
}

func TestSyncDataFailedReloadKeepsPendingState(t *testing.T) {
	c, probe := newRecoveryController(t)
	probe.failAt = "reload"
	sendRecoveryConfig(t, c, map[string]string{"maxconn": "2345"})
	c.store.UpdateAllIngresses = true
	sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})

	require.True(t, c.store.UpdateAllIngresses, "reload failure must not run successful store cleanup")
	require.Zero(t, probe.snapshots, "reload failure must not replace the last successful backend snapshot")
	require.True(t, instance.NeedReload(), "committed config still needs to reach the process")
}

func TestSyncDataKeepsReloadRequestedDuringReload(t *testing.T) {
	c, probe := newRecoveryController(t)
	probe.afterReload = func() { instance.Reload("configuration changed during reload") }
	sendRecoveryConfig(t, c, map[string]string{"maxconn": "2345"})
	sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
	require.True(t, instance.NeedReload(), "finishing a reload must not clear a newer request")
	sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
	require.Equal(t, 2, probe.reloads)
	require.False(t, instance.NeedReload())
	sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
	require.Equal(t, 2, probe.starts)
}

func TestSyncDataRecoversRejectedGlobal(t *testing.T) {
	for _, test := range []struct{ name, failure, fallbackFailure string }{
		{name: "commit", failure: "commit"},
		{name: "reload", failure: "reload"},
		{name: "fallback start", failure: "reload", fallbackFailure: "start"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, probe := newRecoveryController(t)
			probe.failAt = test.failure
			probe.fallbackFailure = test.fallbackFailure
			probe.rejectGlobal = true
			global := &v3.Global{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "global", Generation: 1},
				Spec: v3.GlobalSpec{Global: models.Global{GlobalBase: models.GlobalBase{
					PerformanceOptions: &models.PerformanceOptions{Maxconn: 3456},
				}}},
			}
			sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.CR_GLOBAL, Namespace: "ns", Name: "global", Data: global})
			sendRecoveryConfig(t, c, map[string]string{"cr-global": "ns/global", "maxconn": "2345"})
			sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})

			require.True(t, c.store.CRDisabled(store.CRRef{Kind: store.CRKindGlobal, Namespace: "ns", Name: "global"}))
			require.Equal(t, 2, probe.starts, "the existing custom resource fallback must run immediately")
			if test.fallbackFailure != "" {
				sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
				require.Equal(t, 3, probe.starts, "a failed recursive attempt must remain pending")
			}
			require.Contains(t, probe.applied, "maxconn 2345")
			require.False(t, instance.NeedReload())
			require.Zero(t, probe.transactionOverlaps, "recursive recovery must dispose the previous transaction before starting another")
			attempts := probe.starts
			sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
			require.Equal(t, attempts, probe.starts)
		})
	}
}

// The native configuration client and SyncData are real. Only transaction errors
// and process reload results are injected; no HAProxy process runs in this test.
type recoveryProbe struct {
	api.HAProxyClient
	process.Process
	configFile          string
	failAt              string
	fallbackFailure     string
	applied             string
	afterReload         func()
	starts              int
	reloads             int
	snapshots           int
	rejectGlobal        bool
	transactionActive   bool
	transactionOverlaps int
}

func (p *recoveryProbe) APIStartTransaction() error {
	p.starts++
	if p.transactionActive {
		p.transactionOverlaps++
	}
	if p.failAt == "start" {
		p.failAt = ""
		return errors.New("injected transaction start failure")
	}
	err := p.HAProxyClient.APIStartTransaction()
	if err == nil {
		p.transactionActive = true
	}
	return err
}

func (p *recoveryProbe) APIDisposeTransaction() {
	p.HAProxyClient.APIDisposeTransaction()
	p.transactionActive = false
}

func (p *recoveryProbe) APIFinalCommitTransaction() error {
	if p.failAt == "commit" {
		p.failAt = ""
		if p.rejectGlobal {
			// Inject a global-section diagnostic in the failed-config directory
			// read by the real custom resource fallback.
			return p.rejectConfiguration(filepath.Join(filepath.Dir(p.configFile), "failed"))
		}
		return errors.New("injected transaction commit failure")
	}
	return p.HAProxyClient.APIFinalCommitTransaction()
}

func (p *recoveryProbe) rejectConfiguration(dir string) error {
	config, err := os.ReadFile(p.configFile)
	if err != nil {
		return err
	}
	for index, line := range strings.Split(string(config), "\n") {
		if line != "global" {
			continue
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		filename := filepath.Join(dir, filepath.Base(p.configFile))
		if err := os.WriteFile(filename, config, 0o600); err != nil {
			return err
		}
		p.failAt = p.fallbackFailure
		return fmt.Errorf("config : parsing [%s:%d] : injected global rejection", filename, index+2)
	}
	return errors.New("global section not found")
}

func (p *recoveryProbe) PushPreviousBackends() error {
	p.snapshots++
	return p.HAProxyClient.PushPreviousBackends()
}

func (p *recoveryProbe) Service(action string) (string, error) {
	if action != "reload" {
		return "", fmt.Errorf("unexpected service action %q", action)
	}
	p.reloads++
	if p.failAt == "reload" {
		p.failAt = ""
		if p.rejectGlobal {
			err := p.rejectConfiguration(filepath.Dir(p.configFile))
			return "[ALERT] " + err.Error(), err
		}
		return "", errors.New("injected reload failure")
	}
	config, err := os.ReadFile(p.configFile)
	if err != nil {
		return "", err
	}
	p.applied = string(config)
	if p.afterReload != nil {
		p.afterReload()
		p.afterReload = nil
	}
	return "", nil
}

func newRecoveryController(t *testing.T) (*HAProxyController, *recoveryProbe) {
	t.Helper()
	args := os.Args
	t.Cleanup(func() { os.Args = args })
	instance.Reset()
	c := buildGlobalTestController(t)
	setupFrontends(t, c)
	c.ready = true
	c.processIngress = c.processIngressesDefaultImplementation
	c.prometheusMetricsManager = metrics.New()
	// No local default backend or endpoints are needed for global/defaults sync.
	c.osArgs.DefaultBackendService.Namespace = "ns"
	c.osArgs.DefaultBackendService.Name = "unused"
	c.store.ConfigMaps.Main.Namespace = "ns"
	c.store.ConfigMaps.Main.Name = "config"
	probe := &recoveryProbe{HAProxyClient: c.haproxy.HAProxyClient, Process: c.haproxy.Process, configFile: c.haproxy.MainCFGFile}
	c.haproxy.HAProxyClient = probe
	c.haproxy.Process = probe
	c.eventChan = make(chan k8ssync.SyncDataEvent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.SyncData()
	}()
	t.Cleanup(func() {
		close(c.eventChan)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("SyncData did not stop")
		}
		fs.Writer.WaitUntilWritesDone()
		fs.RunDelayedFuncs()
		instance.Reset()
	})
	sendRecoveryConfig(t, c, map[string]string{"maxconn": "1234", "timeout-connect": "5s"})
	sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
	require.Contains(t, probe.applied, "maxconn 1234")
	require.False(t, instance.NeedReload())
	probe.starts, probe.reloads, probe.snapshots = 0, 0, 0
	return c, probe
}

func sendRecoveryConfig(t *testing.T, c *HAProxyController, annotations map[string]string) {
	t.Helper()
	sendRecoveryEvent(t, c, k8ssync.SyncDataEvent{
		SyncType: k8ssync.CONFIGMAP, Namespace: "ns", Name: "config",
		Data: &store.ConfigMap{Namespace: "ns", Name: "config", Status: store.MODIFIED, Annotations: annotations},
	})
}

func sendRecoveryEvent(t *testing.T, c *HAProxyController, event k8ssync.SyncDataEvent) {
	t.Helper()
	event.EventProcessed = make(chan struct{})
	select {
	case c.eventChan <- event:
	case <-time.After(5 * time.Second):
		t.Fatal("SyncData did not receive event")
	}
	select {
	case <-event.EventProcessed:
	case <-time.After(5 * time.Second):
		t.Fatal("SyncData did not finish event")
	}
}

func readRecoveryConfig(t *testing.T, c *HAProxyController) string {
	t.Helper()
	config, err := os.ReadFile(c.haproxy.MainCFGFile)
	require.NoError(t, err)
	return string(config)
}
