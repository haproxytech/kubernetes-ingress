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

package customresources

import (
	"github.com/haproxytech/client-native/v6/models"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const (
	appNs              = "appNs"
	serviceName        = "appSvcName"
	ingressName        = "appIngName"
	backendCRName      = "appBackend"
	configMapNamespace = "haproxy-controller"
	configMapName      = "haproxy-kubernetes-ingress"

	backendHeader = "backend appNs_svc_appSvcName_https"
	crDirective   = "retries 7"
)

func backendCREvent(generation int64) k8ssync.SyncDataEvent {
	return k8ssync.SyncDataEvent{
		SyncType: k8ssync.CR_BACKEND, Namespace: appNs, Name: backendCRName,
		Data: &v3.Backend{
			ObjectMeta: metav1.ObjectMeta{Namespace: appNs, Name: backendCRName, Generation: generation},
			Spec:       v3.BackendSpec{Backend: models.Backend{BackendBase: models.BackendBase{Retries: utils.Ptr(int64(7))}}},
		},
	}
}

func appServiceEvent() k8ssync.SyncDataEvent {
	return k8ssync.SyncDataEvent{
		SyncType: k8ssync.SERVICE, Namespace: appNs, Name: serviceName,
		Data: &store.Service{
			Annotations: map[string]string{"cr-backend": backendCRName},
			Name:        serviceName,
			Namespace:   appNs,
			Ports:       []store.ServicePort{{Name: "https", Protocol: "TCP", Port: 443, Status: store.ADDED}},
			Status:      store.ADDED,
		},
	}
}

func appIngressEvent() k8ssync.SyncDataEvent {
	return k8ssync.SyncDataEvent{
		SyncType: k8ssync.INGRESS, Namespace: appNs, Name: ingressName,
		Data: &store.Ingress{
			IngressCore: store.IngressCore{
				APIVersion: store.NETWORKINGV1,
				Name:       ingressName,
				Namespace:  appNs,
				Class:      "haproxy",
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
		},
	}
}

func (suite *CustomResourceSuite) sync(events ...k8ssync.SyncDataEvent) {
	testController := suite.TestControllers[suite.T().Name()]
	for _, e := range events {
		testController.EventChan <- e
	}
	testController.EventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND}
	done := make(chan struct{})
	testController.EventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND, EventProcessed: done}
	<-done
}

func (suite *CustomResourceSuite) setupApp() {
	ns := store.Namespace{Name: appNs, Status: store.ADDED}
	ingressClass := &store.IngressClass{Name: "haproxy", Controller: "haproxy.org/ingress-controller", Status: store.ADDED}
	configMap := &store.ConfigMap{Namespace: configMapNamespace, Name: configMapName, Annotations: map[string]string{}, Status: store.ADDED}
	suite.sync(
		k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE, Namespace: appNs, Data: &ns},
		k8ssync.SyncDataEvent{SyncType: k8ssync.INGRESS_CLASS, Data: ingressClass},
		k8ssync.SyncDataEvent{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: configMap},
	)
}

// TestDisabledBackendCR pins what happens to a backend once HAProxy rejected the
// custom resource it is built from: it keeps working with the annotation defaults,
// as if the resource was not referenced. Editing the resource, which bumps its
// generation, puts it back under test.
func (suite *CustomResourceSuite) TestDisabledBackendCR() {
	testController := suite.TestControllers[suite.T().Name()]
	suite.StartController()
	suite.setupApp()

	suite.sync(backendCREvent(1), appServiceEvent(), appIngressEvent())
	suite.ExpectHaproxyConfigContains(backendHeader, 1)
	suite.ExpectHaproxyConfigContains(crDirective, 1)

	_, ok := testController.Store.DisableBackendCR(appNs, backendCRName)
	suite.Require().True(ok, "the store knows the resource")
	// Replaying the unchanged resource triggers a sync without a new generation.
	suite.sync(backendCREvent(1))
	suite.ExpectHaproxyConfigContains(backendHeader, 1)
	suite.ExpectHaproxyConfigContains(crDirective, 0)

	suite.sync(backendCREvent(2))
	suite.ExpectHaproxyConfigContains(backendHeader, 1)
	suite.ExpectHaproxyConfigContains(crDirective, 1)

	suite.StopController()
}
