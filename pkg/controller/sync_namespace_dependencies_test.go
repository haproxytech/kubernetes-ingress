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
	"reflect"
	"testing"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/metrics"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// Stop at the transaction boundary: observe real COMMAND scheduling without
// needing a running HAProxy or introducing reload goroutines into these tests.
type dependencyTransactionProbe struct {
	api.HAProxyClient
	starts   int
	backends []*store.RuntimeBackend
}

func (p *dependencyTransactionProbe) APIStartTransaction() error {
	p.starts++
	return errors.New("dependency test: transaction observed")
}

func (p *dependencyTransactionProbe) SyncBackendSrvs(backend *store.RuntimeBackend) error {
	p.backends = append(p.backends, backend)
	return nil
}

func dependencyController(
	t *testing.T,
	alwaysSelected, ready, accept bool,
) (*HAProxyController, *dependencyTransactionProbe) {
	t.Helper()
	instance.Reset()
	t.Cleanup(instance.Reset)
	args := utils.OSArgs{
		NamespaceLabelSelector: "watch=true",
		ConfigMap:              utils.NamespaceValue{Namespace: "base", Name: "config"},
		DefaultCertificate:     utils.NamespaceValue{Namespace: "base", Name: "default-cert"},
		PublishService:         "base/publish",
	}
	st := store.NewK8sStore(args)
	if !alwaysSelected {
		delete(st.NamespacesAccess.AlwaysSelected, "base")
	}
	// A selected namespace may remain watched after losing its matching label.
	ns := st.GetNamespace("base")
	if ns.Relevant || !st.SkipNamespaceInConfig(ns) {
		t.Fatal("fixture must be non-Relevant in selector mode")
	}
	probe := &dependencyTransactionProbe{}
	c := &HAProxyController{
		store: st, eventChan: make(chan k8ssync.SyncDataEvent, 2),
		sessions:                 &mockSessions{allow: accept, ready: ready},
		haproxy:                  haproxy.HAProxy{HAProxyClient: probe},
		prometheusMetricsManager: metrics.New(), osArgs: args,
	}
	done := make(chan struct{})
	go func() { c.SyncData(); close(done) }()
	t.Cleanup(func() { close(c.eventChan); <-done })
	return c, probe
}

func TestSyncDataAlwaysSelectedDependencies(t *testing.T) {
	dependencies := []k8ssync.SyncType{
		k8ssync.SECRET, k8ssync.SERVICE, k8ssync.ENDPOINTS,
		k8ssync.CR_GLOBAL, k8ssync.CR_DEFAULTS, k8ssync.CR_BACKEND, k8ssync.CR_FRONTEND,
		// Produced only for --publish-service; its addresses feed all Relevant ingresses.
		k8ssync.PUBLISH_SERVICE,
	}
	business := []k8ssync.SyncType{
		k8ssync.INGRESS, k8ssync.GATEWAY, k8ssync.TCPROUTE,
		k8ssync.REFERENCEGRANT, k8ssync.CR_TCP, k8ssync.CUSTOM_RESOURCE,
	}
	states := []struct {
		name   string
		always bool
		ready  bool
		accept bool
	}{
		{name: "always-ready", always: true, ready: true, accept: true},
		{name: "ordinary-dormant", ready: true, accept: true},
		{name: "always-starting", always: true, accept: true},
		{name: "ordinary-starting", accept: true},
		{name: "always-stale", always: true, ready: true},
	}
	for _, state := range states {
		for _, kind := range append(dependencies, business...) {
			for _, status := range []store.Status{store.MODIFIED, store.DELETED} {
				t.Run(fmt.Sprintf("%s/%s/%s", state.name, kind, status), func(t *testing.T) {
					c, probe := dependencyController(t, state.always, state.ready, state.accept)
					ev, changed := dependencyEventFixture(&c.store, kind, status)
					if changed != nil && changed() {
						t.Fatal("fixture must start with old data")
					}
					waitProcessed(t, c.eventChan, ev)
					if changed != nil && changed() != state.accept {
						t.Errorf("store mutation = %t, want %t", changed(), state.accept)
					}
					waitProcessed(t, c.eventChan, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
					isDependency := false
					for _, dependency := range dependencies {
						isDependency = isDependency || kind == dependency
					}
					want := 0
					if state.always && state.ready && state.accept && isDependency {
						want = 1
						checkDependencyConsumer(t, c.store, ev, status)
					}
					if probe.starts != want {
						t.Errorf("HAProxy transactions = %d, want %d", probe.starts, want)
					}
					if c.store.Namespaces["base"].Relevant || !c.store.SkipNamespaceInConfig(c.store.Namespaces["base"]) {
						t.Error("dependency events must not grant business namespace eligibility")
					}
				})
			}
		}
	}
}

// Seed a real old resource, then return an effective update/delete and an
// assertion for the corresponding store mutation (not just the event's data).
func dependencyEventFixture(
	st *store.K8s,
	kind k8ssync.SyncType,
	status store.Status,
) (k8ssync.SyncDataEvent, func() bool) {
	ns := st.GetNamespace("base")
	ev := k8ssync.SyncDataEvent{SyncType: kind, Namespace: ns.Name, Name: "model", NamespaceEpoch: 1}
	deleted := status == store.DELETED
	switch kind {
	case k8ssync.SECRET:
		old := &store.Secret{Namespace: ns.Name, Name: "default-cert", Data: map[string][]byte{"tls.crt": []byte("old")}}
		data := &store.Secret{
			Namespace: ns.Name, Name: old.Name, Status: status,
			Data: map[string][]byte{"tls.crt": []byte("rotated")},
		}
		ns.Secret[old.Name] = old
		ev.Data = data
		return ev, func() bool {
			if deleted {
				return ns.Secret[old.Name].Status == store.DELETED
			}
			return ns.Secret[old.Name] == data
		}
	case k8ssync.SERVICE:
		old := &store.Service{
			Namespace: ns.Name, Name: "default-backend", Ports: []store.ServicePort{{Name: "http", Port: 8080}},
		}
		data := &store.Service{
			Namespace: ns.Name, Name: old.Name, Status: status, Ports: []store.ServicePort{{Name: "http", Port: 9090}},
		}
		ns.Services[old.Name] = old
		ev.Data = data
		return ev, func() bool {
			if deleted {
				return ns.Services[old.Name].Status == store.DELETED
			}
			return ns.Services[old.Name] == data
		}
	case k8ssync.ENDPOINTS:
		old := dependencyEndpoints("10.0.0.1", store.EMPTY)
		data := dependencyEndpoints("10.0.0.2", status)
		ns.Endpoints[old.Service] = map[string]*store.Endpoints{old.SliceName: old}
		ev.Data = data
		return ev, func() bool { return ns.Endpoints[data.Service][data.SliceName] == data }
	case k8ssync.CR_GLOBAL:
		old, data := &v3.Global{}, &v3.Global{}
		old.Spec.Global.Description, data.Spec.Global.Description = "old", "new"
		ns.CRs.Global[ev.Name] = &old.Spec.Global
		if !deleted {
			ev.Data = data
		}
		return ev, func() bool {
			if deleted {
				return ns.CRs.Global[ev.Name] == nil
			}
			return ns.CRs.Global[ev.Name] == &data.Spec.Global
		}
	case k8ssync.CR_DEFAULTS:
		old, data := &v3.Defaults{}, &v3.Defaults{}
		old.Spec.Defaults.Mode, data.Spec.Defaults.Mode = "tcp", "http"
		ns.CRs.Defaults[ev.Name] = &old.Spec.Defaults
		if !deleted {
			ev.Data = data
		}
		return ev, func() bool {
			if deleted {
				return ns.CRs.Defaults[ev.Name] == nil
			}
			return ns.CRs.Defaults[ev.Name] == &data.Spec.Defaults
		}
	case k8ssync.CR_BACKEND:
		old, data := &v3.Backend{}, &v3.Backend{}
		old.Spec.Backend.Mode, data.Spec.Backend.Mode = "tcp", "http"
		ns.CRs.Backends[ev.Name] = &old.Spec
		if !deleted {
			ev.Data = data
		}
		return ev, func() bool {
			if deleted {
				return ns.CRs.Backends[ev.Name] == nil
			}
			return ns.CRs.Backends[ev.Name] == &data.Spec
		}
	case k8ssync.CR_FRONTEND:
		old, data := &v3.Frontend{}, &v3.Frontend{}
		old.Spec.Frontend.Mode, data.Spec.Frontend.Mode = "tcp", "http"
		ns.CRs.Frontends[ev.Name] = &old.Spec
		if !deleted {
			ev.Data = data
		}
		return ev, func() bool {
			if deleted {
				return ns.CRs.Frontends[ev.Name] == nil
			}
			return ns.CRs.Frontends[ev.Name] == &data.Spec
		}
	case k8ssync.PUBLISH_SERVICE:
		old := &store.Service{Namespace: ns.Name, Name: "publish", Addresses: []string{"192.0.2.1"}}
		ns.Services[old.Name] = old
		st.PublishServiceAddresses = old.Addresses
		ev.Data = &store.Service{Namespace: ns.Name, Name: old.Name, Status: status, Addresses: []string{"192.0.2.2"}}
		return ev, func() bool {
			want := []string{"192.0.2.2"}
			if deleted {
				want = nil
			}
			return st.UpdateAllIngresses && reflect.DeepEqual(st.PublishServiceAddresses, want) &&
				reflect.DeepEqual(old.Addresses, want)
		}
	case k8ssync.INGRESS:
		old := &store.Ingress{IngressCore: store.IngressCore{Namespace: ns.Name, Name: ev.Name}}
		data := &store.Ingress{
			IngressCore: store.IngressCore{Namespace: ns.Name, Name: ev.Name, Class: "changed"}, Status: status,
		}
		ns.Ingresses[ev.Name] = old
		ev.Data = data
		return ev, func() bool {
			if deleted {
				return ns.Ingresses[ev.Name] == nil
			}
			return ns.Ingresses[ev.Name] == data
		}
	case k8ssync.GATEWAY:
		ns.Gateways[ev.Name] = &store.Gateway{Name: ev.Name, Generation: 1}
		data := &store.Gateway{Namespace: ns.Name, Name: ev.Name, Generation: 2, GatewayClassName: "changed", Status: status}
		ev.Data = data
		return ev, func() bool { return ns.Gateways[ev.Name] == data }
	case k8ssync.TCPROUTE:
		ns.TCPRoutes[ev.Name] = &store.TCPRoute{Name: ev.Name, Generation: 1}
		data := &store.TCPRoute{
			Namespace: ns.Name, Name: ev.Name, Generation: 2, Status: status,
			BackendRefs: []store.BackendRef{{Name: "changed"}},
		}
		ev.Data = data
		return ev, func() bool { return ns.TCPRoutes[ev.Name] == data }
	case k8ssync.REFERENCEGRANT:
		ns.ReferenceGrants[ev.Name] = &store.ReferenceGrant{Name: ev.Name, Generation: 1}
		data := &store.ReferenceGrant{
			Namespace: ns.Name, Name: ev.Name, Generation: 2, Status: status,
			From: []store.ReferenceGrantFrom{{Namespace: "changed"}},
		}
		ev.Data = data
		return ev, func() bool {
			if deleted {
				return ns.ReferenceGrants[ev.Name] == nil
			}
			return ns.ReferenceGrants[ev.Name] == data
		}
	case k8ssync.CR_TCP:
		ns.CRs.TCPsPerCR[ev.Name] = &store.TCPs{Namespace: ns.Name, Name: ev.Name, Generation: 1}
		data := &store.TCPs{Namespace: ns.Name, Name: ev.Name, Generation: 2, IngressClass: "changed", Status: status}
		ev.Data = data
		return ev, func() bool {
			if deleted {
				return ns.CRs.TCPsPerCR[ev.Name] == nil
			}
			return ns.CRs.TCPsPerCR[ev.Name] == data
		}
	default:
		// CUSTOM_RESOURCE signals a refresh; it has no store object to mutate.
		return ev, nil
	}
}

// Exercise the same readers used by controller defaults and CR annotations.
func checkDependencyConsumer(t *testing.T, st store.K8s, ev k8ssync.SyncDataEvent, status store.Status) {
	t.Helper()
	var got, want interface{}
	var err error
	refs := map[string]string{"ref": ev.Namespace + "/" + ev.Name}
	switch ev.SyncType {
	case k8ssync.SECRET:
		got, err = st.GetSecret(ev.Namespace, "default-cert")
		want = ev.Data
	case k8ssync.SERVICE:
		got, err = st.GetService(ev.Namespace, "default-backend")
		want = ev.Data
	case k8ssync.CR_GLOBAL:
		got, err = annotations.ModelGlobal("ref", "", st, refs)
		if ev.Data != nil {
			want = &ev.Data.(*v3.Global).Spec.Global
		}
	case k8ssync.CR_DEFAULTS:
		got, err = annotations.ModelDefaults("ref", "", st, refs)
		if ev.Data != nil {
			want = &ev.Data.(*v3.Defaults).Spec.Defaults
		}
	case k8ssync.CR_BACKEND:
		got, err = annotations.ModelBackend("ref", "", st, refs)
		if ev.Data != nil {
			want = &ev.Data.(*v3.Backend).Spec
		}
	case k8ssync.CR_FRONTEND:
		got, err = annotations.ModelFrontend("ref", "", st, refs)
		if ev.Data != nil {
			want = &ev.Data.(*v3.Frontend).Spec.Frontend
		}
	default:
		// Endpoints runtime consumption and global publish addresses are checked separately.
		return
	}
	if status == store.DELETED {
		if err == nil {
			t.Error("consumer must observe the deleted dependency as missing")
		}
		return
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("consumer got %+v, %v; want %+v", got, err, want)
	}
}

func TestSyncDataAlwaysSelectedConfiguredConfigMap(t *testing.T) {
	// CONFIGMAP is process-level, not a namespaced session event: even a
	// Starting session must not change the existing configured ConfigMap path.
	c, probe := dependencyController(t, true, false, true)
	data := &store.ConfigMap{Namespace: "base", Name: "config", Status: store.MODIFIED,
		Annotations: map[string]string{"timeout-connect": "10s"}}
	waitProcessed(t, c.eventChan, k8ssync.SyncDataEvent{SyncType: k8ssync.CONFIGMAP, Namespace: "base", Data: data})
	waitProcessed(t, c.eventChan, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
	if probe.starts != 1 || c.store.ConfigMaps.Main.Annotations["timeout-connect"] != "10s" {
		t.Fatal("configured ConfigMap must retain its store and scheduling behavior")
	}
}

func dependencyEndpoints(address string, status store.Status) *store.Endpoints {
	return &store.Endpoints{
		Namespace: "base", Service: "default-backend", SliceName: "slice", Status: status,
		Ports: map[string]*store.PortEndpoints{"http": {Port: 8080, Addresses: map[string]struct{}{address: {}}}},
	}
}

func TestSyncDataAlwaysSelectedDefaultBackendEndpoints(t *testing.T) {
	for _, state := range []string{"ready", "starting", "dormant", "stale"} {
		for _, status := range []store.Status{store.MODIFIED, store.DELETED} {
			t.Run(fmt.Sprintf("%s/%s", state, status), func(t *testing.T) {
				c, probe := dependencyController(t, state != "dormant", state != "starting", state != "stale")
				ns := c.store.GetNamespace("base")
				old := dependencyEndpoints("10.0.0.1", store.EMPTY)
				other := dependencyEndpoints("10.0.0.3", store.EMPTY)
				other.SliceName = "other"
				ns.Endpoints[old.Service] = map[string]*store.Endpoints{old.SliceName: old, other.SliceName: other}
				backend := &store.RuntimeBackend{Name: "default_backend", HAProxySrvs: map[string]*store.HAProxySrv{}}
				ns.HAProxyRuntime[old.Service] = map[string]*store.RuntimeBackend{"http": backend}
				ns.HAProxyRuntimeStandalone[old.Service] = map[string]map[string]*store.RuntimeBackend{
					"http": {"standalone-default": {Name: "standalone-default"}},
				}
				data := dependencyEndpoints("10.0.0.2", status)
				waitProcessed(t, c.eventChan, k8ssync.SyncDataEvent{
					SyncType: k8ssync.ENDPOINTS, Namespace: "base", NamespaceEpoch: 1, Data: data,
				})
				stored := ns.Endpoints[data.Service][data.SliceName]
				if (stored == data) != (state != "stale") {
					t.Fatal("endpoint cache mutation did not respect the session epoch")
				}
				wantCalls, wantTransactions := 0, 0
				if state == "ready" {
					wantCalls, wantTransactions = 2, 1
				}
				if len(probe.backends) != wantCalls {
					t.Errorf("runtime callback calls = %d, want %d", len(probe.backends), wantCalls)
				}
				for _, got := range probe.backends {
					want := store.RuntimeEndpoints{{Address: "10.0.0.3", Port: 8080}: {}}
					if status != store.DELETED {
						want[store.RuntimeEndpoint{Address: "10.0.0.2", Port: 8080}] = struct{}{}
					}
					if !reflect.DeepEqual(got.Endpoints, want) || (got.Name != "default_backend" && got.Name != "standalone-default") {
						t.Errorf("runtime callback got %+v, want endpoints %+v", got, want)
					}
				}
				waitProcessed(t, c.eventChan, k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND})
				if probe.starts != wantTransactions {
					t.Errorf("HAProxy transactions = %d, want %d", probe.starts, wantTransactions)
				}
			})
		}
	}
}
