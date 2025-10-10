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

package routeacl

import (
	"os"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	c "github.com/haproxytech/kubernetes-ingress/pkg/controller"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/suite"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type UseBackendSuite struct {
	suite.Suite
	test Test
}

type updateStatusManager struct{}

func (m *updateStatusManager) AddIngress(ingress *ingress.Ingress) {}
func (m *updateStatusManager) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	return err
}

func TestUseBackend(t *testing.T) {
	suite.Run(t, new(UseBackendSuite))
}

type Test struct {
	Controller *c.HAProxyController
	TempDir    string
}

func (suite *UseBackendSuite) BeforeTest(suiteName, testName string) {
	tempDir, err := os.MkdirTemp("", "tnr-"+testName+"-*")
	if err != nil {
		suite.T().Fatalf("Suite '%s': Test '%s' : error : %s", suiteName, testName, err)
	}
	suite.test.TempDir = tempDir
	suite.T().Logf("temporary configuration dir %s", suite.test.TempDir)
}

var haproxyConfig = `global
daemon
master-worker
pidfile /var/run/haproxy.pid
stats socket /var/run/haproxy-runtime-api.sock level admin expose-fd listeners
default-path config

peers localinstance
 peer local 127.0.0.1:10000

frontend https
mode http
http-request set-var(txn.base) base
use_backend %[var(txn.path_match),field(1,.)]

frontend http
mode http
http-request set-var(txn.base) base
use_backend %[var(txn.path_match),field(1,.)]

frontend healthz
mode http
monitor-uri /healthz
option dontlog-normal

frontend stats
  mode http
  stats enable
  stats uri /
  stats refresh 10s
  http-request set-var(txn.base) base
  http-request use-service prometheus-exporter if { path /metrics }
 `

func (suite *UseBackendSuite) UseBackendFixture() (eventChan chan k8ssync.SyncDataEvent) {
	var osArgs utils.OSArgs
	os.Args = []string{os.Args[0], "-e", "-t", "--config-dir=" + suite.test.TempDir}
	parser := flags.NewParser(&osArgs, flags.IgnoreUnknown)
	_, errParsing := parser.Parse() //nolint:ifshort
	if errParsing != nil {
		suite.T().Fatal(errParsing)
	}

	s := store.NewK8sStore(osArgs)

	haproxyEnv := env.Env{
		CfgDir: suite.test.TempDir,
		Proxies: env.Proxies{
			FrontHTTP:  "http",
			FrontHTTPS: "https",
			FrontSSL:   "ssl",
			BackSSL:    "ssl-backend",
		},
	}

	eventChan = make(chan k8ssync.SyncDataEvent, watch.DefaultChanSize*6)
	controller := c.NewBuilder().
		WithHaproxyCfgFile([]byte(haproxyConfig)).
		WithEventChan(eventChan).
		WithStore(s).
		WithHaproxyEnv(haproxyEnv).
		WithUpdateStatusManager(&updateStatusManager{}).
		WithArgs(osArgs).Build()

	go controller.Start()

	// Now sending store events for test setup
	ns := store.Namespace{Name: "ns", Status: store.ADDED}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE, Namespace: ns.Name, Data: &ns}

	endpoints := &store.Endpoints{
		SliceName: "myappservice",
		Service:   "myappservice",
		Namespace: ns.Name,
		Ports: map[string]*store.PortEndpoints{
			"https": {
				Port:      int64(3001),
				Addresses: map[string]struct{}{"10.244.0.9": {}},
			},
		},
		Status: store.ADDED,
	}

	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.ENDPOINTS, Namespace: endpoints.Namespace, Data: endpoints}

	service := &store.Service{
		Name:        "myappservice",
		Namespace:   ns.Name,
		Annotations: map[string]string{"route-acl": "cookie(staging) -m found"},
		Ports: []store.ServicePort{
			{
				Name:     "https",
				Protocol: "TCP",
				Port:     8443,
				Status:   store.ADDED,
			},
		},
		Status: store.ADDED,
	}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SERVICE, Namespace: service.Namespace, Data: service}

	ingressClass := &store.IngressClass{
		Name:       "haproxy",
		Controller: "haproxy.org/ingress-controller",
		Status:     store.ADDED,
	}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.INGRESS_CLASS, Data: ingressClass}

	prefixPathType := networkingv1.PathTypePrefix
	ingress := &store.Ingress{
		IngressCore: store.IngressCore{
			APIVersion: store.NETWORKINGV1,
			Name:       "myapping",
			Namespace:  ns.Name,
			Class:      "haproxy",
			Rules: map[string]*store.IngressRule{
				"": {
					Paths: map[string]*store.IngressPath{
						string(prefixPathType) + "-/": {
							Path:          "/",
							PathTypeMatch: string(prefixPathType),
							SvcNamespace:  service.Namespace,
							SvcPortString: "https",
							SvcName:       service.Name,
						},
					},
				},
			},
		},
		Status: store.ADDED,
	}

	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.INGRESS, Namespace: ingress.Namespace, Data: ingress}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND}
	// The service is modified by the addition of an annotation.
	// It should not duplicate this line in haproxy.cfg:
	// use_backend ns_myappservice_https if { path -m beg / } { cookie(staging) -m found }
	serviceClone := *service
	serviceClone.Status = store.MODIFIED
	serviceClone.Annotations["anyannotation"] = "anyvalue"
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SERVICE, Namespace: serviceClone.Namespace, Data: &serviceClone}
	controllerHasWorked := make(chan struct{})
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND, EventProcessed: controllerHasWorked}
	<-controllerHasWorked
	return eventChan
}

func (suite *UseBackendSuite) NonWildcardHostFixture() (eventChan chan k8ssync.SyncDataEvent) {
	var osArgs utils.OSArgs
	os.Args = []string{os.Args[0], "-e", "-t", "--config-dir=" + suite.test.TempDir}
	parser := flags.NewParser(&osArgs, flags.IgnoreUnknown)
	_, errParsing := parser.Parse() //nolint:ifshort
	if errParsing != nil {
		suite.T().Fatal(errParsing)
	}

	s := store.NewK8sStore(osArgs)

	haproxyEnv := env.Env{
		CfgDir: suite.test.TempDir,
		Proxies: env.Proxies{
			FrontHTTP:  "http",
			FrontHTTPS: "https",
			FrontSSL:   "ssl",
			BackSSL:    "ssl-backend",
		},
	}

	eventChan = make(chan k8ssync.SyncDataEvent, watch.DefaultChanSize*6)
	controller := c.NewBuilder().
		WithHaproxyCfgFile([]byte(haproxyConfig)).
		WithEventChan(eventChan).
		WithStore(s).
		WithHaproxyEnv(haproxyEnv).
		WithUpdateStatusManager(&updateStatusManager{}).
		WithArgs(osArgs).Build()

	go controller.Start()

	// Now sending store events for test setup
	ns := store.Namespace{Name: "ns", Status: store.ADDED}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE, Namespace: ns.Name, Data: &ns}

	endpoints := &store.Endpoints{
		SliceName: "api-service",
		Service:   "api-service",
		Namespace: ns.Name,
		Ports: map[string]*store.PortEndpoints{
			"https": {
				Port:      int64(3001),
				Addresses: map[string]struct{}{"10.244.0.11": {}},
			},
		},
		Status: store.ADDED,
	}

	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.ENDPOINTS, Namespace: endpoints.Namespace, Data: endpoints}

	service := &store.Service{
		Name:        "api-service",
		Namespace:   ns.Name,
		Annotations: map[string]string{"route-acl": "path_reg path-in-bug-repro$"},
		Ports: []store.ServicePort{
			{
				Name:     "https",
				Protocol: "TCP",
				Port:     8443,
				Status:   store.ADDED,
			},
		},
		Status: store.ADDED,
	}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SERVICE, Namespace: service.Namespace, Data: service}

	ingressClass := &store.IngressClass{
		Name:       "haproxy",
		Controller: "haproxy.org/ingress-controller",
		Status:     store.ADDED,
	}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.INGRESS_CLASS, Data: ingressClass}

	prefixPathType := networkingv1.PathTypePrefix
	ingress := &store.Ingress{
		IngressCore: store.IngressCore{
			APIVersion: store.NETWORKINGV1,
			Name:       "api-ingress",
			Namespace:  ns.Name,
			Class:      "haproxy",
			Rules: map[string]*store.IngressRule{
				"api.example.local": {
					Host: "api.example.local", // Explicitly set the Host field
					Paths: map[string]*store.IngressPath{
						string(prefixPathType) + "-/": {
							Path:          "/",
							PathTypeMatch: string(prefixPathType),
							SvcNamespace:  service.Namespace,
							SvcPortString: "https",
							SvcName:       service.Name,
						},
					},
				},
			},
		},
		Status: store.ADDED,
	}

	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.INGRESS, Namespace: ingress.Namespace, Data: ingress}
	controllerHasWorked := make(chan struct{})
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND, EventProcessed: controllerHasWorked}
	<-controllerHasWorked
	return eventChan
}

func (suite *UseBackendSuite) WildcardHostFixture() (eventChan chan k8ssync.SyncDataEvent) {
	var osArgs utils.OSArgs
	os.Args = []string{os.Args[0], "-e", "-t", "--config-dir=" + suite.test.TempDir}
	parser := flags.NewParser(&osArgs, flags.IgnoreUnknown)
	_, errParsing := parser.Parse() //nolint:ifshort
	if errParsing != nil {
		suite.T().Fatal(errParsing)
	}

	s := store.NewK8sStore(osArgs)

	haproxyEnv := env.Env{
		CfgDir: suite.test.TempDir,
		Proxies: env.Proxies{
			FrontHTTP:  "http",
			FrontHTTPS: "https",
			FrontSSL:   "ssl",
			BackSSL:    "ssl-backend",
		},
	}

	eventChan = make(chan k8ssync.SyncDataEvent, watch.DefaultChanSize*6)
	controller := c.NewBuilder().
		WithHaproxyCfgFile([]byte(haproxyConfig)).
		WithEventChan(eventChan).
		WithStore(s).
		WithHaproxyEnv(haproxyEnv).
		WithUpdateStatusManager(&updateStatusManager{}).
		WithArgs(osArgs).Build()

	go controller.Start()

	// Now sending store events for test setup
	ns := store.Namespace{Name: "ns", Status: store.ADDED}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE, Namespace: ns.Name, Data: &ns}

	endpoints := &store.Endpoints{
		SliceName: "wildcard-service",
		Service:   "wildcard-service",
		Namespace: ns.Name,
		Ports: map[string]*store.PortEndpoints{
			"https": {
				Port:      int64(3001),
				Addresses: map[string]struct{}{"10.244.0.10": {}},
			},
		},
		Status: store.ADDED,
	}

	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.ENDPOINTS, Namespace: endpoints.Namespace, Data: endpoints}

	service := &store.Service{
		Name:        "wildcard-service",
		Namespace:   ns.Name,
		Annotations: map[string]string{"route-acl": "path_reg path-in-bug-repro$"},
		Ports: []store.ServicePort{
			{
				Name:     "https",
				Protocol: "TCP",
				Port:     8443,
				Status:   store.ADDED,
			},
		},
		Status: store.ADDED,
	}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.SERVICE, Namespace: service.Namespace, Data: service}

	ingressClass := &store.IngressClass{
		Name:       "haproxy",
		Controller: "haproxy.org/ingress-controller",
		Status:     store.ADDED,
	}
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.INGRESS_CLASS, Data: ingressClass}

	prefixPathType := networkingv1.PathTypePrefix
	ingress := &store.Ingress{
		IngressCore: store.IngressCore{
			APIVersion: store.NETWORKINGV1,
			Name:       "wildcard-ingress",
			Namespace:  ns.Name,
			Class:      "haproxy",
			Rules: map[string]*store.IngressRule{
				"*.example.local": {
					Host: "*.example.local", // Explicitly set the Host field
					Paths: map[string]*store.IngressPath{
						string(prefixPathType) + "-/": {
							Path:          "/",
							PathTypeMatch: string(prefixPathType),
							SvcNamespace:  service.Namespace,
							SvcPortString: "https",
							SvcName:       service.Name,
						},
					},
				},
			},
		},
		Status: store.ADDED,
	}

	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.INGRESS, Namespace: ingress.Namespace, Data: ingress}
	controllerHasWorked := make(chan struct{})
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND, EventProcessed: controllerHasWorked}
	<-controllerHasWorked
	return eventChan
}
