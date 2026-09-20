// Copyright 2026 HAProxy Technologies LLC
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

//go:build e2e_sequential

package namespaceselector

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

const (
	gatewayClassName = "ns-selector-gwc"
	gatewayName      = "ns-selector-gw"
	gatewayListener  = "tcp"
	gatewayRouteName = "ns-selector-route"
	gatewayRoutePort = 8001
)

type gatewayFixture struct {
	ClassName    string
	GatewayName  string
	ListenerName string
	RouteName    string
	Port         int
}

// Test_GatewayAllowedRoutesFollowNamespaceLabel checks a Gateway listener with
// allowedRoutes.namespaces.from=Selector: the TCPRoute is attached while the
// namespace carries the selector label and detached once only that label
// changes, without editing the route. The HAProxy configuration of the
// controller is the observable, so the listener port does not need to be
// exposed to the test host.
func (suite *NamespaceSelectorSuite) Test_GatewayAllowedRoutesFollowNamespaceLabel() {
	if !gatewayAPIEnabled(suite.originalArgs) {
		suite.T().Skip("Gateway API not enabled in the e2e cluster (create.sh EXPERIMENTAL_GWAPI=1)")
	}

	ns := suite.test.GetNS()
	fixture := gatewayFixture{
		ClassName:    gatewayClassName,
		GatewayName:  gatewayName,
		ListenerName: gatewayListener,
		RouteName:    gatewayRouteName,
		Port:         gatewayRoutePort,
	}
	frontend := ns + "-" + fixture.GatewayName + "-" + fixture.ListenerName
	backend := ns + "_" + fixture.RouteName

	suite.Require().NoError(suite.test.Apply("config/gateway.yaml.tmpl", ns, fixture))
	suite.Require().NoError(suite.test.Apply("config/tcproute.yaml.tmpl", ns, fixture))
	suite.T().Cleanup(func() {
		_, _ = kubectl("-n", ns, "delete", "tcproute", fixture.RouteName, "--ignore-not-found=true")
		_, _ = kubectl("-n", ns, "delete", "gateway", fixture.GatewayName, "--ignore-not-found=true")
		_, _ = kubectl("delete", "gatewayclass", fixture.ClassName, "--ignore-not-found=true")
		_, _ = kubectl("label", "ns", ns, "routes-")
	})

	// Select the namespace (app=watch) and allow the route (routes=allowed).
	suite.Require().NoError(labelNS(ns, "app=watch"))
	suite.Require().NoError(labelNS(ns, "routes=allowed"))
	suite.waitGatewayRouteAttached(frontend, backend, true)

	// Only the AllowedRoutes label changes; the namespace stays selected.
	suite.Require().NoError(labelNS(ns, "routes=denied"))
	suite.waitGatewayRouteAttached(frontend, backend, false)
}

func (suite *NamespaceSelectorSuite) waitGatewayRouteAttached(frontend, backend string, want bool) {
	suite.Require().Eventually(func() bool {
		cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
		if err != nil {
			suite.T().Log(err)
			return false
		}
		hasFrontend := strings.Contains(cfg, "frontend "+frontend)
		hasBackend := strings.Contains(cfg, "default_backend "+backend)
		return hasFrontend && (hasBackend == want)
	}, e2e.WaitDuration, e2e.TickDuration)
}

// gatewayAPIEnabled reports whether the e2e cluster and controller are set up
// for Gateway API (deploy/tests/create.sh with EXPERIMENTAL_GWAPI=1).
func gatewayAPIEnabled(controllerArgs []string) bool {
	if !hasControllerArg(controllerArgs, "--gateway-controller-name") {
		return false
	}
	out, err := kubectl("get", "crd", "tcproutes.gateway.networking.k8s.io", "-o", "name")
	return err == nil && strings.Contains(out, "tcproutes.gateway.networking.k8s.io")
}

func hasControllerArg(args []string, name string) bool {
	for _, arg := range args {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}
