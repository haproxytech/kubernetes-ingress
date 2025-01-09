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

package acls

import (
	_ "embed"
	"os"
	"testing"

	"github.com/haproxytech/client-native/v5/models"
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	c "github.com/haproxytech/kubernetes-ingress/pkg/controller"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type FakeUpdateSatusManager struct{}

func (m *FakeUpdateSatusManager) AddIngress(ingress *ingress.Ingress) {}
func (m *FakeUpdateSatusManager) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	return
}

type ACLSuite struct {
	suite.Suite
	test Test
}

func TestACL(t *testing.T) {
	suite.Run(t, new(ACLSuite))
}

type Test struct {
	Controller *c.HAProxyController
	TempDir    string
}

func (suite *ACLSuite) BeforeTest(suiteName, testName string) {
	tempDir, err := os.MkdirTemp("", "tnr-"+testName+"-*")
	if err != nil {
		suite.T().Fatalf("Suite '%s': Test '%s' : error : %s", suiteName, testName, err)
	}
	suite.test.TempDir = tempDir
	suite.T().Logf("temporary configuration dir %s", suite.test.TempDir)
}

func (suite *ACLSuite) TearDownSuite() {
	os.Unsetenv("POD_NAME")
}

func (suite *ACLSuite) UseACLFixture() (eventChan chan k8ssync.SyncDataEvent) {
	var osArgs utils.OSArgs
	os.Args = []string{os.Args[0], "-e", "-t", "--config-dir=" + suite.test.TempDir}
	parser := flags.NewParser(&osArgs, flags.IgnoreUnknown)
	_, errParsing := parser.Parse() //nolint:ifshort
	if errParsing != nil {
		suite.T().Fatal(errParsing)
	}

	s := store.NewK8sStore(osArgs)
	os.Setenv("POD_NAME", "haproxy-kubernetes-ingress-68c9fc6d86-zn9qz")

	haproxyEnv := env.Env{
		CfgDir: suite.test.TempDir,
		Proxies: env.Proxies{
			FrontHTTP:  "http",
			FrontHTTPS: "https",
			FrontSSL:   "ssl",
			BackSSL:    "ssl",
		},
	}
	haproxyConfig, err := os.ReadFile("../../../../fs/usr/local/etc/haproxy/haproxy.cfg")
	if err != nil {
		//nolint:testifylint
		assert.Failf(suite.T(), "error in opening init haproxy configuration file", err.Error())
	}

	eventChan = make(chan k8ssync.SyncDataEvent, watch.DefaultChanSize*6)
	controller := c.NewBuilder().
		WithHaproxyCfgFile(haproxyConfig).
		WithEventChan(eventChan).
		WithStore(s).
		WithHaproxyEnv(haproxyEnv).
		WithUpdateStatusManager(&FakeUpdateSatusManager{}).
		WithArgs(osArgs).Build()

	go controller.Start()

	backend := v1.Backend{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend1cr",
			Namespace: "ns1",
		},
		Spec: v1.BackendSpec{
			Config: &models.Backend{
				Name: "backend1",
			},
			Acls: models.Acls{
				{
					ACLName:   "cookie_found",
					Criterion: "cook(JSESSIONID)",
					Index:     utils.Ptr[int64](0),
					Value:     "-m found",
				},
				{
					ACLName:   "is_ticket",
					Criterion: "path_beg",
					Index:     utils.Ptr[int64](1),
					Value:     "-i /ticket",
				},
			},
		},
	}

	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.CR_BACKEND, Namespace: backend.Namespace, Name: backend.Name, Data: &backend}

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
		Annotations: map[string]string{"cr-backend": backend.Namespace + "/" + backend.Name},
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

	ingress := &store.Ingress{
		IngressCore: store.IngressCore{
			APIVersion:  store.NETWORKINGV1,
			Name:        "myapping",
			Namespace:   ns.Name,
			Annotations: map[string]string{"haproxy.org/ingress.class": "haproxy"},
			Rules: map[string]*store.IngressRule{
				"": {
					Paths: map[string]*store.IngressPath{
						string(networkingv1.PathTypePrefix) + "-/": {
							Path:          "/",
							PathTypeMatch: string(networkingv1.PathTypePrefix),
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
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND}
	eventChan <- k8ssync.SyncDataEvent{EventProcessed: controllerHasWorked}
	<-controllerHasWorked
	return
}
