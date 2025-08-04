// Copyright 2023 HAProxy Technologies LLC
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

package configsnippet_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	networkingv1 "k8s.io/api/networking/v1"
)

type exceptedSnippet struct {
	count   int
	comment string
}

func newAppSvc() *store.Service {
	return &store.Service{
		Annotations: map[string]string{
			"backend-config-snippet": "http-send-name-header x-dst-server",
		},
		Name:      serviceName,
		Namespace: appNs,
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
			APIVersion: store.NETWORKINGV1,
			Name:       ingressName,
			Namespace:  appNs,
			Annotations: map[string]string{
				"backend-config-snippet": "http-send-name-header x-dst-server",
			},
			Class: "haproxy",
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

func newConfigMap(annotations map[string]string) *store.ConfigMap {
	return &store.ConfigMap{
		Annotations: annotations,
		Namespace:   configMapNamespace,
		Name:        configMapName,
		Status:      store.ADDED,
	}
}

func newConfigMapWithGlobalSnippet() *store.ConfigMap {
	return newConfigMap(map[string]string{"global-config-snippet": "ssl-default-bind-options no-sslv3 no-tlsv10 no-tlsv11"})
}

func newConfigMapWithFrontendSnippet() *store.ConfigMap {
	return newConfigMap(map[string]string{"frontend-config-snippet": "unique-id-format %{+X}o\\ %ci:%cp_%fi:%fp_%Ts_%rt:%pid\nunique-id-header X-Unique-ID"})
}

func newConfigMapWithBackendSnippet() *store.ConfigMap {
	return newConfigMap(map[string]string{"backend-config-snippet": "http-send-name-header x-dst-server"})
}

func (suite *DisableConfigSnippetSuite) TestDisableGlobalConfigSnippet() {
	tests := []struct {
		name                 string
		controllerSnippetArg string
		expectedSnippets     []exceptedSnippet
	}{
		{
			name:                 "Global config snippet enabled",
			controllerSnippetArg: "",
			expectedSnippets:     []exceptedSnippet{{count: 1, comment: "###_config-snippet_### BEGIN"}},
		},
		{
			name:                 "Global config snippet disabled - 'global'",
			controllerSnippetArg: "--disable-config-snippets=global",
			expectedSnippets:     []exceptedSnippet{{count: 0, comment: "###_config-snippet_### BEGIN"}},
		},
		{
			name:                 "Global config snippet disabled - 'all'",
			controllerSnippetArg: "--disable-config-snippets=all",
			expectedSnippets:     []exceptedSnippet{{count: 0, comment: "###_config-snippet_### BEGIN"}},
		},
		{
			name:                 "Global config snippet disabled - 'frontend', should not disable",
			controllerSnippetArg: "--disable-config-snippets=frontend",
			expectedSnippets:     []exceptedSnippet{{count: 1, comment: "###_config-snippet_### BEGIN"}},
		},
		{
			name:                 "Global config snippet disabled - several types disabled",
			controllerSnippetArg: "--disable-config-snippets=global,frontend",
			expectedSnippets:     []exceptedSnippet{{count: 0, comment: "###_config-snippet_### BEGIN"}},
		},
	}
	for _, tt := range tests {
		suite.setupControllerSnippetArg(tt.controllerSnippetArg)
		suite.StartController()
		suite.setupTest()
		event := k8ssync.SyncDataEvent{
			SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: newConfigMapWithGlobalSnippet(),
		}
		suite.disableConfigSnippetFixture(event)
		suite.expectSnippet(tt.expectedSnippets)
		suite.StopController()
	}
}

func (suite *DisableConfigSnippetSuite) setupControllerSnippetArg(snippetArg string) {
	arg, _ := strings.CutPrefix(snippetArg, "--disable-config-snippets=")
	suite.TestControllers[suite.T().Name()].OSArgs.DisableConfigSnippets = arg
}

func (suite *DisableConfigSnippetSuite) expectSnippet(expectedSnippets []exceptedSnippet) {
	testController := suite.TestControllers[suite.T().Name()]

	content, err := os.ReadFile(filepath.Join(testController.TempDir, "haproxy.cfg"))
	if err != nil {
		suite.T().Error(err.Error())
	}
	for _, expectedSnippet := range expectedSnippets {
		c := strings.Count(string(content), expectedSnippet.comment)
		suite.Exactly(expectedSnippet.count, c, fmt.Sprintf("%s is repeated %d times but expected %d", expectedSnippet.comment, c, expectedSnippet.count))
	}
}

func getSvcSnippetComment(appNs, serviceName string) string {
	return fmt.Sprintf("### service:%s_svc_%s_%s/%s/%s ###", appNs, serviceName, "https", appNs, serviceName)
}

func getIngSnippetComment(appNs, serviceName, ingressName string) string {
	return fmt.Sprintf("### ingress:%s_svc_%s_%s/%s/%s ###", appNs, serviceName, "https", appNs, ingressName)
}

func getConfigMapComment() string {
	return fmt.Sprintf("### configmap:%s/%s ###", configMapNamespace, configMapName)
}

func (suite *DisableConfigSnippetSuite) TestDisableFrontendConfigSnippet() {
	tests := []struct {
		name                 string
		controllerSnippetArg string
		expectedSnippets     []exceptedSnippet
	}{
		{
			name:                 "Frontend config snippet enabled",
			controllerSnippetArg: "",
			// 2 snippets - frontend: http, https
			expectedSnippets: []exceptedSnippet{{count: 2, comment: "###_config-snippet_### BEGIN"}},
		},
		{
			name:                 "Frontend config snippet disabled - 'frontend'",
			controllerSnippetArg: "--disable-config-snippets=frontend",
			expectedSnippets:     []exceptedSnippet{{count: 0, comment: "###_config-snippet_### BEGIN"}},
		},
		{
			name:                 "Frontend config snippet disabled - 'all'",
			controllerSnippetArg: "--disable-config-snippets=all",
			expectedSnippets:     []exceptedSnippet{{count: 0, comment: "###_config-snippet_### BEGIN"}},
		},
		{
			name:                 "Frontend config snippet disabled - 'backend', should not disable",
			controllerSnippetArg: "--disable-config-snippets=backend",
			// 2 snippets - frontend: http, https
			expectedSnippets: []exceptedSnippet{{count: 2, comment: "###_config-snippet_### BEGIN"}},
		},
	}
	for _, tt := range tests {
		suite.setupControllerSnippetArg(tt.controllerSnippetArg)
		suite.StartController()
		suite.setupTest()
		event := k8ssync.SyncDataEvent{
			SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: newConfigMapWithFrontendSnippet(),
		}
		suite.disableConfigSnippetFixture(event)
		suite.expectSnippet(tt.expectedSnippets)
		suite.StopController()
	}
}

func (suite *DisableConfigSnippetSuite) TestDisableBackendConfigSnippet() {
	tests := []struct {
		name                 string
		controllerSnippetArg string
		events               []k8ssync.SyncDataEvent
		expectedSnippets     []exceptedSnippet
	}{
		{
			name:                 "Backend config snippet enabled - Configmap",
			controllerSnippetArg: "",
			events:               []k8ssync.SyncDataEvent{{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: newConfigMapWithBackendSnippet()}},
			expectedSnippets:     []exceptedSnippet{{count: 1, comment: getConfigMapComment()}},
		},
		{
			name:                 "Backend config snippet disabled - 'backend' - Configmap",
			controllerSnippetArg: "--disable-config-snippets=backend",
			events:               []k8ssync.SyncDataEvent{{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: newConfigMapWithBackendSnippet()}},
			expectedSnippets:     []exceptedSnippet{{count: 0, comment: getConfigMapComment()}},
		},
		{
			name:                 "Backend config snippet disabled - 'all' - Configmap",
			controllerSnippetArg: "--disable-config-snippets=all",
			events:               []k8ssync.SyncDataEvent{{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: newConfigMapWithBackendSnippet()}},
			expectedSnippets:     []exceptedSnippet{{count: 0, comment: getConfigMapComment()}},
		},
		{
			name:                 "Backend config snippet disabled - 'global', should not disable - Configmap",
			controllerSnippetArg: "--disable-config-snippets=global",
			events:               []k8ssync.SyncDataEvent{{SyncType: k8ssync.CONFIGMAP, Namespace: configMapNamespace, Name: configMapName, Data: newConfigMapWithBackendSnippet()}},
			expectedSnippets:     []exceptedSnippet{{count: 1, comment: getConfigMapComment()}},
		},
		{
			name:                 "Backend config snippet enabled - Service, Ingress",
			controllerSnippetArg: "",
			events: []k8ssync.SyncDataEvent{
				{SyncType: k8ssync.SERVICE, Namespace: appNs, Name: serviceName, Data: newAppSvc()},
				{SyncType: k8ssync.INGRESS, Namespace: appNs, Name: ingressName, Data: newAppIngress()},
			},
			expectedSnippets: []exceptedSnippet{
				{count: 1, comment: getSvcSnippetComment(appNs, serviceName)},
				{count: 1, comment: getIngSnippetComment(appNs, serviceName, ingressName)},
			},
		},
		{
			name:                 "Backend config snippet disabled - 'backend' - Service, Ingress",
			controllerSnippetArg: "--disable-config-snippets=backend",
			events: []k8ssync.SyncDataEvent{
				{SyncType: k8ssync.SERVICE, Namespace: appNs, Name: serviceName, Data: newAppSvc()},
				{SyncType: k8ssync.INGRESS, Namespace: appNs, Name: ingressName, Data: newAppIngress()},
			},
			expectedSnippets: []exceptedSnippet{
				{count: 0, comment: getSvcSnippetComment(appNs, serviceName)},
				{count: 0, comment: getIngSnippetComment(appNs, serviceName, ingressName)},
			},
		},
		{
			name:                 "Backend config snippet disabled - 'frontend' should not disable - Service, Ingress",
			controllerSnippetArg: "--disable-config-snippets=frontend",
			events: []k8ssync.SyncDataEvent{
				{SyncType: k8ssync.SERVICE, Namespace: appNs, Name: serviceName, Data: newAppSvc()},
				{SyncType: k8ssync.INGRESS, Namespace: appNs, Name: ingressName, Data: newAppIngress()},
			},
			expectedSnippets: []exceptedSnippet{
				{count: 1, comment: getSvcSnippetComment(appNs, serviceName)},
				{count: 1, comment: getIngSnippetComment(appNs, serviceName, ingressName)},
			},
		},
	}
	for _, tt := range tests {
		suite.setupControllerSnippetArg(tt.controllerSnippetArg)
		suite.StartController()
		suite.setupTest()
		suite.disableConfigSnippetFixture(tt.events...)
		suite.expectSnippet(tt.expectedSnippets)
		suite.StopController()
	}
}
