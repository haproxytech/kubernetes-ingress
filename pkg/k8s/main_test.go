package k8s

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func TestSupportsEndpointSliceMirroring(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		majorVersion string
		minorVersion string
		expected     bool
	}{
		{
			name:         "before mirroring support",
			majorVersion: "1",
			minorVersion: "18",
			expected:     false,
		},
		{
			name:         "first mirroring version",
			majorVersion: "1",
			minorVersion: "19",
			expected:     true,
		},
		{
			name:         "invalid version falls back",
			majorVersion: "1",
			minorVersion: "invalid",
			expected:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := supportsEndpointSliceMirroring(test.majorVersion, test.minorVersion)
			if actual != test.expected {
				t.Fatalf("supportsEndpointSliceMirroring(%q, %q) = %t, want %t", test.majorVersion, test.minorVersion, actual, test.expected)
			}
		})
	}
}

// TestResolveBuiltInAPIs checks the one-time discovery lookup used to keep the
// SyncData hot path free of ServerResourcesForGroupVersion calls.
func TestResolveBuiltInAPIs(t *testing.T) {
	t.Parallel()

	serve := func(t *testing.T, discoveryV1 bool, discoveryV1Beta1 bool) *kubernetes.Clientset {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/apis/networking.k8s.io/v1":
				_, _ = w.Write([]byte(
					`{"kind":"APIResourceList","groupVersion":"networking.k8s.io/v1",` +
						`"resources":[{"name":"ingresses","kind":"Ingress","namespaced":true},` +
						`{"name":"ingressclasses","kind":"IngressClass","namespaced":false}]}`,
				))
			case "/apis/discovery.k8s.io/v1":
				if !discoveryV1 {
					http.NotFound(w, r)
					return
				}
				_, _ = w.Write([]byte(`{"kind":"APIResourceList","groupVersion":"discovery.k8s.io/v1","resources":[{"name":"endpointslices","kind":"EndpointSlice","namespaced":true}]}`))
			case "/apis/discovery.k8s.io/v1beta1":
				if !discoveryV1Beta1 {
					http.NotFound(w, r)
					return
				}
				_, _ = w.Write([]byte(`{"kind":"APIResourceList","groupVersion":"discovery.k8s.io/v1beta1","resources":[{"name":"endpointslices","kind":"EndpointSlice","namespaced":true}]}`))
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(server.Close)
		return kubernetes.NewForConfigOrDie(&rest.Config{Host: server.URL})
	}

	got := resolveBuiltInAPIs(serve(t, true, true))
	if !got.ingress || !got.ingressClass {
		t.Fatalf("networking resources not cached: %+v", got)
	}
	if got.endpointSliceVersion != "v1" {
		t.Fatalf("endpointSliceVersion = %q, want v1", got.endpointSliceVersion)
	}

	got = resolveBuiltInAPIs(serve(t, false, true))
	if got.endpointSliceVersion != "v1beta1" {
		t.Fatalf("fallback endpointSliceVersion = %q, want v1beta1", got.endpointSliceVersion)
	}

	got = resolveBuiltInAPIs(serve(t, false, false))
	if got.endpointSliceVersion != "" {
		t.Fatalf("missing endpointSliceVersion = %q, want empty", got.endpointSliceVersion)
	}

	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "discovery unavailable", http.StatusInternalServerError)
	}))
	t.Cleanup(fail.Close)
	if got = resolveBuiltInAPIs(kubernetes.NewForConfigOrDie(&rest.Config{Host: fail.URL})); got != nil {
		t.Fatalf("failed networking discovery must not be cached, got %+v", got)
	}
}

// TestGetInformersUseCachedBuiltInAPIs is the regression test for the session
// construction blocking SyncData on rate-limited discovery calls: with the
// cache populated, building the per-namespace informers must not call the API.
func TestGetInformersUseCachedBuiltInAPIs(t *testing.T) {
	t.Parallel()

	var discoveryCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discoveryCalls.Add(1)
		http.Error(w, "discovery must not be called", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := kubernetes.NewForConfigOrDie(&rest.Config{Host: server.URL})
	k := k8s{
		builtInClient: client,
		builtInAPIs:   &builtInAPIs{ingress: true, ingressClass: true, endpointSliceVersion: "v1"},
		handlerRegs:   &[]cache.ResourceEventHandlerRegistration{},
	}
	eventChan := make(chan k8ssync.SyncDataEvent, 1)
	factory := informers.NewSharedInformerFactory(client, 0)

	ii, ici := k.getIngressInformers(eventChan, factory, utils.OSArgs{})
	if ii == nil || ici == nil {
		t.Fatalf("expected ingress and ingressclass informers, got ii=%v ici=%v", ii, ici)
	}
	if epsi := k.getEndpointSliceInformer(eventChan, factory); epsi == nil {
		t.Fatal("expected an endpointslices informer")
	}
	if n := discoveryCalls.Load(); n != 0 {
		t.Fatalf("session construction made %d discovery calls, want 0", n)
	}
}
