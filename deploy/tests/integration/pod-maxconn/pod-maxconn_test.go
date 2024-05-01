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

package podmaxconn

import (
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	networkingv1 "k8s.io/api/networking/v1"
)

func (suite *PodMaxConnSuite) TestPodMaxConnConfigMap() {
	suite.StartController()
	cm := suite.setupTest()

	///////////////////////////////////////
	// pod-maxconn setup in:
	// - configmap only
	cm.Status = store.MODIFIED
	cm.Annotations["pod-maxconn"] = "128"
	svc := newAppSvc()
	ing := newAppIngress()
	events := []k8ssync.SyncDataEvent{
		{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: cm},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic1", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic1",
		}},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic2", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic2",
		}},
		{SyncType: k8ssync.SERVICE, Namespace: appNs, Name: serviceName, Data: svc},
		{SyncType: k8ssync.INGRESS, Namespace: appNs, Name: ingressName, Data: ing},
	}
	suite.fixture(events...)
	// Expected occurrences of "default-server check maxconn 64"
	// 64 = 128 / 2 instances of IC
	// 1- backend haproxy-controller_default-local-service_http
	// 1- backend appNs_appSvcName_https
	suite.ExpectHaproxyConfigContains("default-server check maxconn 64", 2)

	suite.StopController()
}

func (suite *PodMaxConnSuite) TestPodMaxConnConfigMapMisc() {
	suite.StartController()
	cm := suite.setupTest()

	///////////////////////////////////////
	// pod-maxconn setup in:
	// - configmap only
	cm.Status = store.MODIFIED
	cm.Annotations["pod-maxconn"] = "128"
	svc := newAppSvc()
	ing := newAppIngress()
	events := []k8ssync.SyncDataEvent{
		{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: cm},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic1", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic1",
		}},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic2", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic2",
		}},
		{SyncType: k8ssync.SERVICE, Namespace: appNs, Name: serviceName, Data: svc},
		{SyncType: k8ssync.INGRESS, Namespace: appNs, Name: ingressName, Data: ing},
	}
	suite.fixture(events...)
	// Expected occurrences of "default-server check maxconn 64"
	// 64 = 128 / 2 instances of IC
	// 1- backend haproxy-controller_default-local-service_http
	// 1- backend appNs_appSvcName_https
	suite.ExpectHaproxyConfigContains("default-server check maxconn 64", 2)

	// -------------------------------------
	// Resend ADDED => should change nothing
	events = []k8ssync.SyncDataEvent{
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic1", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic1",
		}},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic2", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic2",
		}},
	}
	suite.fixture(events...)
	// Expected occurrences of "default-server check maxconn 64"
	// 64 = 128 / 2 instances of IC
	// 1- backend haproxy-controller_default-local-service_http
	// 1- backend appNs_appSvcName_https
	suite.ExpectHaproxyConfigContains("default-server check maxconn 64", 2)

	// -------------------------------------
	// SEND MODIFIED => should change nothing
	events = []k8ssync.SyncDataEvent{
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic1", Data: store.PodEvent{
			Status: store.MODIFIED,
			Name:   "ic1",
		}},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic2", Data: store.PodEvent{
			Status: store.MODIFIED,
			Name:   "ic2",
		}},
	}
	suite.fixture(events...)
	// Expected occurrences of "default-server check maxconn 64"
	// 64 = 128 / 2 instances of IC
	// 1- backend haproxy-controller_default-local-service_http
	// 1- backend appNs_appSvcName_https
	suite.ExpectHaproxyConfigContains("default-server check maxconn 64", 2)

	// --------------------------------------------------
	// Send MODIFIED on a non-existing POD -> should increment
	events = []k8ssync.SyncDataEvent{
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic3", Data: store.PodEvent{
			Status: store.MODIFIED,
			Name:   "ic3",
		}},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic4", Data: store.PodEvent{
			Status: store.MODIFIED,
			Name:   "ic4",
		}},
	}
	suite.fixture(events...)
	// Expected occurrences of "default-server check maxconn 32" (divided by 4)
	// 64 = 128 / 2 instances of IC
	// 1- backend haproxy-controller_default-local-service_http
	// 1- backend appNs_appSvcName_https
	suite.ExpectHaproxyConfigContains("default-server check maxconn 32", 2)

	suite.StopController()
}

func (suite *PodMaxConnSuite) TestPodMaxConnService() {
	suite.StartController()
	cm := suite.setupTest()

	///////////////////////////////////////
	// pod-maxconn setup in:
	// - configmap (128)
	// - ingress (126)
	// - app svc (124) (appNs_appSvcName_https)
	cm.Status = store.MODIFIED
	cm.Annotations["pod-maxconn"] = "128"
	ing := newAppIngress()
	ing.Annotations["pod-maxconn"] = "126"
	svc := newAppSvc()
	svc.Annotations["pod-maxconn"] = "124"

	events := []k8ssync.SyncDataEvent{
		{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: cm},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic1", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic1",
		}},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic2", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic2",
		}},
		{SyncType: k8ssync.SERVICE, Namespace: appNs, Name: serviceName, Data: svc},
		{SyncType: k8ssync.INGRESS, Namespace: appNs, Name: ingressName, Data: ing},
	}
	suite.fixture(events...)
	// -- Expected occurrences of "default-server check maxconn 64" #1 (from configmap)
	// backend haproxy-controller_default-local-service_http
	// -- Expected occurrences of "default-server check maxconn 62": #1 (from service)
	// backend appNs_appSvcName_https
	suite.ExpectHaproxyConfigContains("default-server check maxconn 64", 1)
	suite.ExpectHaproxyConfigContains("default-server check maxconn 62", 1)
	suite.ExpectHaproxyConfigContains("default-server check maxconn 63", 0)

	suite.StopController()
}

func (suite *PodMaxConnSuite) TestPodMaxConnIngress() {
	suite.StartController()
	cm := suite.setupTest()

	// pod-maxconn setup in:
	// timeout server setup in:
	// - configmap (77000)
	// - ingress (76000)
	cm.Status = store.MODIFIED
	cm.Annotations["pod-maxconn"] = "128"
	ing := newAppIngress()
	ing.Annotations["pod-maxconn"] = "126"
	svc := newAppSvc()

	events := []k8ssync.SyncDataEvent{
		{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: cm},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic1", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic1",
		}},
		{SyncType: k8ssync.POD, Namespace: configMapName, Name: "ic2", Data: store.PodEvent{
			Status: store.ADDED,
			Name:   "ic2",
		}},
		{SyncType: k8ssync.SERVICE, Namespace: appNs, Name: serviceName, Data: svc},
		{SyncType: k8ssync.INGRESS, Namespace: appNs, Name: ingressName, Data: ing},
	}
	suite.fixture(events...)
	// -- Expected occurrences of "default-server check maxconn 64" #1 (from configmap)
	// backend haproxy-controller_default-local-service_http
	// -- Expected occurrences of "default-server check maxconn 62": #1 (from service)
	// backend appNs_appSvcName_https
	suite.ExpectHaproxyConfigContains("default-server check maxconn 64", 1)
	suite.ExpectHaproxyConfigContains("default-server check maxconn 62", 0)
	suite.ExpectHaproxyConfigContains("default-server check maxconn 63", 1)

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
