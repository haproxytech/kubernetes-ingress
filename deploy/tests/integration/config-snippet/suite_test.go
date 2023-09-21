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
	"testing"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/integration"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
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
	testController := suite.TestControllers[suite.T().Name()]
	testController.OSArgs.ConfigMap.Name = configMapName
	testController.OSArgs.ConfigMap.Namespace = configMapNamespace
}

func (suite *DisableConfigSnippetSuite) setupTest() {
	testController := suite.TestControllers[suite.T().Name()]

	ns := store.Namespace{Name: appNs, Status: store.ADDED}
	testController.EventChan <- k8s.SyncDataEvent{SyncType: k8s.NAMESPACE, Namespace: ns.Name, Data: &ns}
	testController.EventChan <- k8s.SyncDataEvent{SyncType: k8s.COMMAND}
	controllerHasWorked := make(chan struct{})
	testController.EventChan <- k8s.SyncDataEvent{SyncType: k8s.COMMAND, EventProcessed: controllerHasWorked}
	<-controllerHasWorked
}

func (suite *DisableConfigSnippetSuite) disableConfigSnippetFixture(events ...k8s.SyncDataEvent) {
	testController := suite.TestControllers[suite.T().Name()]

	// Now sending store events for test setup
	for _, e := range events {
		testController.EventChan <- e
	}
	testController.EventChan <- k8s.SyncDataEvent{SyncType: k8s.COMMAND}
	controllerHasWorked := make(chan struct{})
	testController.EventChan <- k8s.SyncDataEvent{SyncType: k8s.COMMAND, EventProcessed: controllerHasWorked}
	<-controllerHasWorked
}
