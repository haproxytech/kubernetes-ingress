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

package timeoutserver

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	networkingv1 "k8s.io/api/networking/v1"
)

func (suite *TimeoutServerSuite) TestTimeoutServerConfigMap() {
	// name := suite.T().Name()
	// testController := suite.TestControllers[name]

	suite.StartController()
	cm := suite.setupTest()

	///////////////////////////////////////
	// timeout server setup in:
	// - configmap only
	cm.Status = store.MODIFIED
	cm.Annotations["timeout-server"] = "77000"
	svc := newAppSvc()
	ing := newAppIngress()
	events := []k8s.SyncDataEvent{
		{SyncType: k8s.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: cm},
		{SyncType: k8s.SERVICE, Namespace: appNs, Name: serviceName, Data: svc},
		{SyncType: k8s.INGRESS, Namespace: appNs, Name: ingressName, Data: ing},
	}
	suite.fixture(events...)
	// Expected occurrences of "timeout server 77000"
	// 1- defaults
	// 2- backend haproxy-controller_default-local-service_http
	// 3- backend appNs_appSvcName_https
	suite.ExpectHaproxyConfigContains("timeout server 77000", 3)

	suite.StopController()
}

func (suite *TimeoutServerSuite) TestTimeoutServerService() {
	// name := suite.T().Name()
	// testController := suite.TestControllers[name]

	suite.StartController()
	cm := suite.setupTest()

	///////////////////////////////////////
	// timeout server setup in:
	// - configmap (77000)
	// - ingress (76000)
	// - app svc (75000) (appNs_appSvcName_https)
	cm.Status = store.MODIFIED
	cm.Annotations["timeout-server"] = "77000"
	ing := newAppIngress()
	ing.Annotations["timeout-server"] = "76000"
	svc := newAppSvc()
	svc.Annotations["timeout-server"] = "75000"

	events := []k8s.SyncDataEvent{
		{SyncType: k8s.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: cm},
		{SyncType: k8s.SERVICE, Namespace: appNs, Name: serviceName, Data: svc},
		{SyncType: k8s.INGRESS, Namespace: appNs, Name: ingressName, Data: ing},
	}
	suite.fixture(events...)
	// Expected occurrences of "timeout server 77000": #2
	// 1- defaults
	// 2- backend haproxy-controller_default-local-service_http
	// Expected occurrences of "timeout server 75000": #1 (from service)
	// 1- backend appNs_appSvcName_https
	// Expected occurrences of "timeout server 76000": #0 (from ingress)
	suite.ExpectHaproxyConfigContains("timeout server 77000", 2)
	suite.ExpectHaproxyConfigContains("timeout server 75000", 1)
	suite.ExpectHaproxyConfigContains("timeout server 76000", 0)

	suite.StopController()
}

func (suite *TimeoutServerSuite) TestTimeoutServerIngress() {
	// name := suite.T().Name()
	// testController := suite.TestControllers[name]

	suite.StartController()
	cm := suite.setupTest()

	///////////////////////////////////////
	// timeout server setup in:
	// - configmap (77000)
	// - ingress (76000)
	cm.Status = store.MODIFIED
	cm.Annotations["timeout-server"] = "77000"
	ing := newAppIngress()
	ing.Annotations["timeout-server"] = "76000"
	svc := newAppSvc()

	events := []k8s.SyncDataEvent{
		{SyncType: k8s.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: cm},
		{SyncType: k8s.SERVICE, Namespace: appNs, Name: serviceName, Data: svc},
		{SyncType: k8s.INGRESS, Namespace: appNs, Name: ingressName, Data: ing},
	}
	suite.fixture(events...)
	// Expected occurrences of "timeout server 77000": #2
	// 1- defaults
	// 2- backend haproxy-controller_default-local-service_http
	// Expected occurrences of "timeout server 75000": #0 (from service)
	// Expected occurrences of "timeout server 76000": #1 (from ingress)
	// 1- backend appNs_appSvcName_https
	suite.ExpectHaproxyConfigContains("timeout server 77000", 2)
	suite.ExpectHaproxyConfigContains("timeout server 75000", 0)
	suite.ExpectHaproxyConfigContains("timeout server 76000", 1)

	suite.StopController()
}

func newAppSvc() *store.Service {
	return &store.Service{
		Annotations: map[string]string{},
		Name:        serviceName,
		Namespace:   appNs,
		Ports: []store.ServicePort{
			{
				Name:     "https",
				Protocol: "TCP",
				Port:     443,
				Status:   store.ADDED,
			},
		},
		Status: store.ADDED,
	}
}

func newAppIngress() *store.Ingress {
	return &store.Ingress{
		IngressCore: store.IngressCore{
			APIVersion:  store.NETWORKINGV1,
			Name:        ingressName,
			Namespace:   appNs,
			Annotations: map[string]string{},
			Rules: map[string]*store.IngressRule{
				"": {
					Paths: map[string]*store.IngressPath{
						string(networkingv1.PathTypePrefix) + "-/": {
							Path:          "/",
							PathTypeMatch: string(networkingv1.PathTypePrefix),
							SvcNamespace:  appNs,
							SvcPortString: "https",
							SvcName:       serviceName,
						},
					},
				},
			},
		},
		Status: store.ADDED,
	}
}
