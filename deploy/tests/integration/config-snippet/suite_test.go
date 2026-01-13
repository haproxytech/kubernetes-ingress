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
	"os"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/integration"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/stretchr/testify/suite"
)

var (
	appNs              = "appNs"
	serviceName        = "appSvcName"
	ingressName        = "appIngName"
	configMapNamespace = "haproxy-controller"
	configMapName      = "haproxy-kubernetes-ingress"
)

type DisableConfigSnippetSuite struct {
	integration.BaseSuite
}

func TestDisableConfigSnippet(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(DisableConfigSnippetSuite))
}

func (suite *DisableConfigSnippetSuite) BeforeTest(suiteName, testName string) {
	suite.BaseSuite.BeforeTest(suiteName, testName)
	// Add any needed update to the controller setting
	// by updating suite.TestControllers[suite.T().Name()].XXXXX
	_ = os.Unsetenv("POD_NAME")
	_ = os.Unsetenv("POD_NAMESPACE")
	testController := suite.TestControllers[suite.T().Name()]
	testController.OSArgs.ConfigMap.Name = configMapName
	testController.OSArgs.ConfigMap.Namespace = configMapNamespace
}

func (suite *DisableConfigSnippetSuite) setupTest() {
	testController := suite.TestControllers[suite.T().Name()]

	ns := store.Namespace{Name: appNs, Status: store.ADDED}
	testController.EventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.NAMESPACE, Namespace: ns.Name, Data: &ns}
	ingressClass := &store.IngressClass{
		Name:       "haproxy",
		Controller: "haproxy.org/ingress-controller",
		Status:     store.ADDED,
	}
	testController.EventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.INGRESS_CLASS, Data: ingressClass}
	testController.EventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND}
	controllerHasWorked := make(chan struct{})
	testController.EventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND, EventProcessed: controllerHasWorked}
	<-controllerHasWorked
}

func (suite *DisableConfigSnippetSuite) disableConfigSnippetFixture(events ...k8ssync.SyncDataEvent) {
	testController := suite.TestControllers[suite.T().Name()]

	// Now sending store events for test setup
	for _, e := range events {
		testController.EventChan <- e
	}
	testController.EventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND}
	controllerHasWorked := make(chan struct{})
	testController.EventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND, EventProcessed: controllerHasWorked}
	<-controllerHasWorked
}
