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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const (
	globalCRName   = "appGlobal"
	defaultsCRName = "appDefaults"
	frontendCRName = "appFrontend"
	tcpCRName      = "appTCP"
	tcpName        = "echo"
)

func crMeta(name string, generation int64) metav1.ObjectMeta {
	return metav1.ObjectMeta{Namespace: appNs, Name: name, Generation: generation}
}

// Every kind gets a directive no annotation default produces, so its presence
// in haproxy.cfg tells whether the resource was rendered.
var disabledCRCases = []struct {
	ref       store.CRRef
	directive string
	event     func(generation int64) k8ssync.SyncDataEvent
}{
	{
		ref:       store.CRRef{Kind: store.CRKindGlobal, Namespace: appNs, Name: globalCRName},
		directive: "maxconn 4242",
		event: func(generation int64) k8ssync.SyncDataEvent {
			return k8ssync.SyncDataEvent{
				SyncType: k8ssync.CR_GLOBAL, Namespace: appNs, Name: globalCRName,
				Data: &v3.Global{ObjectMeta: crMeta(globalCRName, generation), Spec: v3.GlobalSpec{Global: models.Global{
					GlobalBase: models.GlobalBase{PerformanceOptions: &models.PerformanceOptions{Maxconn: 4242}},
				}}},
			}
		},
	},
	{
		ref:       store.CRRef{Kind: store.CRKindDefaults, Namespace: appNs, Name: defaultsCRName},
		directive: "retries 7",
		event: func(generation int64) k8ssync.SyncDataEvent {
			return k8ssync.SyncDataEvent{
				SyncType: k8ssync.CR_DEFAULTS, Namespace: appNs, Name: defaultsCRName,
				Data: &v3.Defaults{ObjectMeta: crMeta(defaultsCRName, generation), Spec: v3.DefaultsSpec{Defaults: models.Defaults{
					DefaultsBase: models.DefaultsBase{Retries: utils.Ptr(int64(7))},
				}}},
			}
		},
	},
	{
		ref:       store.CRRef{Kind: store.CRKindFrontend, Namespace: appNs, Name: frontendCRName},
		directive: "http-request set-header X-CR ok",
		event: func(generation int64) k8ssync.SyncDataEvent {
			return k8ssync.SyncDataEvent{
				SyncType: k8ssync.CR_FRONTEND, Namespace: appNs, Name: frontendCRName,
				Data: &v3.Frontend{ObjectMeta: crMeta(frontendCRName, generation), Spec: v3.FrontendSpec{Frontend: models.Frontend{
					HTTPRequestRuleList: models.HTTPRequestRules{{Type: "set-header", HdrName: "X-CR", HdrFormat: "ok"}},
				}}},
			}
		},
	},
	{
		ref:       store.CRRef{Kind: store.CRKindTCP, Namespace: appNs, Name: tcpCRName},
		directive: "frontend tcpcr_" + appNs + "_" + tcpName,
		event: func(generation int64) k8ssync.SyncDataEvent {
			return k8ssync.SyncDataEvent{
				SyncType: k8ssync.CR_TCP, Namespace: appNs, Name: tcpCRName,
				Data: &store.TCPs{
					Generation: generation, Status: store.ADDED, Namespace: appNs, Name: tcpCRName,
					Items: store.TCPResourceList{{Namespace: appNs, ParentName: tcpCRName, TCPModel: v3.TCPModel{
						Name: tcpName,
						Frontend: models.Frontend{
							FrontendBase: models.FrontendBase{Name: tcpName},
							Binds:        map[string]models.Bind{"b1": {Name: "b1", Address: "0.0.0.0", Port: utils.Ptr(int64(3000))}},
						},
						Service: v3.TCPService{Name: serviceName, Port: 443},
					}}},
				},
			}
		},
	},
}

// TestDisabledCRSections pins the fallback for every custom resource kind once
// HAProxy rejected it: the section is rendered as if the resource was not
// referenced, and the edited resource, with its new generation, is rendered again.
func (suite *CustomResourceSuite) TestDisabledCRSections() {
	testController := suite.TestControllers[suite.T().Name()]
	suite.StartController()
	suite.setupApp()
	configMap := &store.ConfigMap{Namespace: configMapNamespace, Name: configMapName, Status: store.MODIFIED, Annotations: map[string]string{
		"cr-global":        appNs + "/" + globalCRName,
		"cr-defaults":      appNs + "/" + defaultsCRName,
		"cr-frontend-http": appNs + "/" + frontendCRName,
	}}
	suite.sync(k8ssync.SyncDataEvent{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: configMap})
	plainService := appServiceEvent()
	plainService.Data.(*store.Service).Annotations = nil
	suite.sync(plainService, appIngressEvent())

	for _, tt := range disabledCRCases {
		suite.sync(tt.event(1))
		suite.ExpectHaproxyConfigContains(tt.directive, 1)

		_, ok := testController.Store.DisableCR(tt.ref)
		suite.Require().True(ok, "%s: the store knows the resource", tt.ref)
		suite.sync(forceSync())
		suite.ExpectHaproxyConfigContains(tt.directive, 0)

		suite.sync(tt.event(2))
		suite.ExpectHaproxyConfigContains(tt.directive, 1)
	}
	suite.StopController()
}

// forceSync makes the next command run a sync without changing any resource.
func forceSync() k8ssync.SyncDataEvent {
	return k8ssync.SyncDataEvent{SyncType: k8ssync.CUSTOM_RESOURCE}
}
