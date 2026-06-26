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

package service

import (
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServiceForTest(svcName, namespace, dns string, ports []store.ServicePort, path *store.IngressPath) *Service {
	return &Service{
		resource: &store.Service{
			Name:      svcName,
			Namespace: namespace,
			DNS:       dns,
			Ports:     ports,
		},
		path: path,
	}
}

func newK8sWithRuntime(namespace, svcName, portName string, backend *store.RuntimeBackend) store.K8s {
	k8s := store.K8s{
		Namespaces: map[string]*store.Namespace{
			namespace: {
				HAProxyRuntime: map[string]map[string]*store.RuntimeBackend{
					svcName: {
						portName: backend,
					},
				},
			},
		},
	}
	return k8s
}

// TestGetRuntimeBackend_ExternalName_IgnoresOrphanEndpointSlices verifies that an
// ExternalName service always resolves via DNS even when orphan EndpointSlices
// (left over from a prior ClusterIP state) populate HAProxyRuntime.
func TestGetRuntimeBackend_ExternalName_IgnoresOrphanEndpointSlices(t *testing.T) {
	path := &store.IngressPath{
		SvcName:       "nginx",
		SvcNamespace:  "haproxy-controller",
		SvcPortInt:    80,
		SvcPortString: "",
		SvcPortResolved: &store.ServicePort{
			Name: "",
			Port: 80,
		},
	}

	svc := newServiceForTest(
		"nginx", "haproxy-controller", "api1.example.com",
		[]store.ServicePort{{Port: 80}},
		path,
	)

	// Simulate orphan EndpointSlice: HAProxyRuntime is populated with a named port
	// "http" from the previous ClusterIP state.
	orphanBackend := &store.RuntimeBackend{
		Endpoints: store.RuntimeEndpoints{store.RuntimeEndpoint{Port: 80}: {}},
	}
	k8s := newK8sWithRuntime("haproxy-controller", "nginx", "http", orphanBackend)

	backend, err := svc.getRuntimeBackend(k8s)

	require.NoError(t, err)
	require.Len(t, backend.HAProxySrvs, 1)
	assert.Equal(t, "api1.example.com", backend.HAProxySrvs[0].Address)
	assert.Equal(t, "SRV_1", backend.HAProxySrvs[0].Name)
	assert.Equal(t, int64(80), backend.HAProxySrvs[0].Port)
}

// TestGetRuntimeBackend_ExternalName_NoRuntime verifies that an ExternalName
// service resolves correctly even when HAProxyRuntime has no entry for it.
func TestGetRuntimeBackend_ExternalName_NoRuntime(t *testing.T) {
	path := &store.IngressPath{
		SvcName:      "my-ext-svc",
		SvcNamespace: "default",
		SvcPortInt:   443,
		SvcPortResolved: &store.ServicePort{
			Port: 443,
		},
	}

	svc := newServiceForTest(
		"my-ext-svc", "default", "api.example.com",
		[]store.ServicePort{{Port: 443}},
		path,
	)

	k8s := store.K8s{
		Namespaces: map[string]*store.Namespace{
			"default": {
				HAProxyRuntime: map[string]map[string]*store.RuntimeBackend{},
			},
		},
	}

	backend, err := svc.getRuntimeBackend(k8s)

	require.NoError(t, err)
	require.Len(t, backend.HAProxySrvs, 1)
	assert.Equal(t, "api.example.com", backend.HAProxySrvs[0].Address)
	assert.Equal(t, int64(443), backend.HAProxySrvs[0].Port)
}

// TestGetRuntimeBackend_ClusterIP_MatchingPort verifies that a regular ClusterIP
// service resolves correctly from HAProxyRuntime when the port name matches.
func TestGetRuntimeBackend_ClusterIP_MatchingPort(t *testing.T) {
	path := &store.IngressPath{
		SvcName:      "my-svc",
		SvcNamespace: "default",
		SvcPortResolved: &store.ServicePort{
			Name: "http",
			Port: 80,
		},
	}

	svc := newServiceForTest("my-svc", "default", "", nil, path)

	expectedBackend := &store.RuntimeBackend{
		Endpoints: store.RuntimeEndpoints{store.RuntimeEndpoint{Port: 80}: {}},
	}
	k8s := newK8sWithRuntime("default", "my-svc", "http", expectedBackend)

	backend, err := svc.getRuntimeBackend(k8s)

	require.NoError(t, err)
	assert.Equal(t, expectedBackend, backend)
}

// TestGetRuntimeBackend_ClusterIP_NoRuntime verifies that a ClusterIP service
// with no HAProxyRuntime entry returns an error.
func TestGetRuntimeBackend_ClusterIP_NoRuntime(t *testing.T) {
	path := &store.IngressPath{
		SvcName:      "my-svc",
		SvcNamespace: "default",
		SvcPortResolved: &store.ServicePort{
			Name: "http",
			Port: 80,
		},
	}

	svc := newServiceForTest("my-svc", "default", "", nil, path)

	k8s := store.K8s{
		Namespaces: map[string]*store.Namespace{
			"default": {
				HAProxyRuntime: map[string]map[string]*store.RuntimeBackend{},
			},
		},
	}

	_, err := svc.getRuntimeBackend(k8s)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no available endpoints")
}

// TestGetRuntimeBackend_ClusterIP_PortMismatch verifies that a ClusterIP service
// returns an error when the resolved port name does not match any runtime entry.
func TestGetRuntimeBackend_ClusterIP_PortMismatch(t *testing.T) {
	path := &store.IngressPath{
		SvcName:       "my-svc",
		SvcNamespace:  "default",
		SvcPortString: "web",
		SvcPortResolved: &store.ServicePort{
			Name: "web",
			Port: 8080,
		},
	}

	svc := newServiceForTest("my-svc", "default", "", nil, path)

	// HAProxyRuntime has "http" port but path expects "web"
	k8s := newK8sWithRuntime("default", "my-svc", "http", &store.RuntimeBackend{})

	_, err := svc.getRuntimeBackend(k8s)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "web")
}

func TestScaleHAProxySrvsOrdersEndpointsDeterministically(t *testing.T) {
	svc := &Service{
		backend: &models.Backend{BackendBase: models.BackendBase{Name: "backend"}},
	}
	backend := &store.RuntimeBackend{
		Endpoints: store.RuntimeEndpoints{
			{Address: "10.244.0.10", Port: 8080}: {},
			{Address: "10.244.0.2", Port: 8080}:  {},
			{Address: "10.244.0.2", Port: 8443}:  {},
		},
	}

	svc.scaleHAProxySrvs(backend)

	require.Len(t, backend.HAProxySrvs, 42)
	assert.Equal(t, "SRV_1", backend.HAProxySrvs[0].Name)
	assert.Equal(t, "10.244.0.2", backend.HAProxySrvs[0].Address)
	assert.Equal(t, int64(8080), backend.HAProxySrvs[0].Port)
	assert.Equal(t, "SRV_2", backend.HAProxySrvs[1].Name)
	assert.Equal(t, "10.244.0.2", backend.HAProxySrvs[1].Address)
	assert.Equal(t, int64(8443), backend.HAProxySrvs[1].Port)
	assert.Equal(t, "SRV_3", backend.HAProxySrvs[2].Name)
	assert.Equal(t, "10.244.0.10", backend.HAProxySrvs[2].Address)
	assert.Equal(t, int64(8080), backend.HAProxySrvs[2].Port)
	assert.Empty(t, backend.Endpoints)
}
