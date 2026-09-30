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

//go:build e2e_sequential

package namespaceselector

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *NamespaceSelectorSuite) Test_UnlabeledAtStart_Is404() {
	suite.waitStatus(suite.client, 404)
	suite.waitStatus(suite.foreignClient, 404)
}

// ConfigMap namespace is always watched (local default backend, default cert)
// but its own Ingress stays out of HAProxy until it carries the selector labels.
func (suite *NamespaceSelectorSuite) Test_ConfigMapNamespaceOwnIngressNeedsLabel() {
	host := controllerNS + ".cmns.watch.test"
	client, err := e2e.NewHTTPClient(host)
	suite.Require().NoError(err)
	suite.T().Cleanup(func() {
		suite.Require().NoError(clearWatchLabel(controllerNS))
		for _, kind := range []string{"ingress", "svc", "deploy"} {
			out, delErr := kubectl("-n", controllerNS, "delete", kind, "http-echo", "--ignore-not-found=true")
			suite.NoError(delErr, out)
		}
	})
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml", controllerNS, nil))
	suite.Require().NoError(suite.test.Apply(
		"config/ingress.yaml.tmpl",
		controllerNS,
		struct{ Host string }{Host: host},
	))

	suite.waitStatus(client, 404)

	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.waitStatus(client, 404)

	suite.Require().NoError(labelNS(controllerNS, "app=watch"))
	suite.waitStatus(client, 200)
	suite.waitStatus(suite.client, 200)

	suite.Require().NoError(clearWatchLabel(controllerNS))
	suite.waitStatus(client, 404)
	suite.waitStatus(suite.client, 200)
}

func (suite *NamespaceSelectorSuite) Test_LabelStartsWatchWithoutTouchingIngress() {
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.waitStatus(suite.foreignClient, 404)
}

func (suite *NamespaceSelectorSuite) Test_TwoMatchingNamespaces() {
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.Require().NoError(labelNS(suite.foreignNS, "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.waitStatus(suite.foreignClient, 200)
}

func (suite *NamespaceSelectorSuite) Test_PrelabeledNamespacesBootstrapTogether() {
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.Require().NoError(labelNS(suite.foreignNS, "app=watch"))
	suite.Require().NoError(restartController())
	suite.waitStatus(suite.client, 200)
	suite.waitStatus(suite.foreignClient, 200)
}

func (suite *NamespaceSelectorSuite) Test_UnlabelDropsRouteAndConfig() {
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)

	suite.Require().NoError(labelNS(suite.test.GetNS(), "app-"))
	suite.waitStatus(suite.client, 404)

	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err)
	suite.NotContains(cfg, suite.host)
}

func (suite *NamespaceSelectorSuite) Test_LabelValueChangeDropsRoute() {
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=nowatch"))
	suite.waitStatus(suite.client, 404)
}

func (suite *NamespaceSelectorSuite) Test_BounceRelabelRestoresWithoutIngressEdit() {
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app-"))
	suite.waitStatus(suite.client, 404)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)
}

func (suite *NamespaceSelectorSuite) Test_UnlabelMutationRelabelUsesLatestIngress() {
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app-"))
	suite.waitStatus(suite.client, 404)

	updatedHost := "updated-" + suite.host
	updatedClient, err := e2e.NewHTTPClient(updatedHost)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.test.Apply(
		"config/ingress.yaml.tmpl",
		suite.test.GetNS(),
		struct{ Host string }{Host: updatedHost},
	))
	suite.T().Cleanup(func() {
		suite.Require().NoError(suite.test.Apply(
			"config/ingress.yaml.tmpl",
			suite.test.GetNS(),
			struct{ Host string }{Host: suite.host},
		))
	})

	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 404)
	suite.waitStatus(updatedClient, 200)
}

func (suite *NamespaceSelectorSuite) Test_DormantServiceMutationRelabelUsesLatestEndpoints() {
	namespace := suite.test.GetNS()
	restoreSelector := func(value string) error {
		patch := `{"spec":{"selector":{"app":"` + value + `"}}}`
		out, err := kubectl("-n", namespace, "patch", "service", "http-echo", "--type=merge", "-p", patch)
		if err != nil {
			return fmt.Errorf("patch service selector: %w: %s", err, out)
		}
		return nil
	}
	suite.T().Cleanup(func() {
		suite.NoError(restoreSelector("http-echo"))
	})

	suite.Require().NoError(labelNS(namespace, "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(clearWatchLabel(namespace))
	suite.waitStatus(suite.client, 404)

	// The sticky session must keep Service and EndpointSlice watches current
	// while the namespace is irrelevant.
	suite.Require().NoError(restoreSelector("missing"))
	suite.Require().NoError(labelNS(namespace, "app=watch"))
	suite.waitStatus(suite.client, 503)

	suite.Require().NoError(clearWatchLabel(namespace))
	suite.waitStatus(suite.client, 404)
	suite.Require().NoError(restoreSelector("http-echo"))
	suite.Require().NoError(labelNS(namespace, "app=watch"))
	suite.waitStatus(suite.client, 200)
}

func (suite *NamespaceSelectorSuite) Test_SetBasedSelectorRequiresAllTerms() {
	patched := append([]string{}, suite.originalArgs...)
	patched = append(patched, "--namespace-label-selector=app in (watch,observe),tenant=enabled")
	suite.Require().NoError(restoreControllerArgs(patched))
	suite.T().Cleanup(func() {
		suite.NoError(labelNS(suite.test.GetNS(), "tenant-"))
		suite.Require().NoError(restoreControllerArgs(suite.originalArgs))
		suite.Require().NoError(setControllerSelector())
		suite.Require().NoError(waitControllerRollout())
	})

	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 404)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "tenant=enabled"))
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=observe"))
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "tenant=disabled"))
	suite.waitStatus(suite.client, 404)
}

func (suite *NamespaceSelectorSuite) Test_TCPCRLabelUnlabelBounce() {
	suite.Require().NoError(suite.test.Apply("config/tcp.yaml", suite.test.GetNS(), nil))
	client, err := e2e.NewHTTPClient(suite.host, 32766)
	suite.Require().NoError(err)
	suite.T().Cleanup(func() {
		out, err := kubectl(
			"-n", suite.test.GetNS(),
			"delete", "tcp.ingress.v3.haproxy.org", "namespace-selector-tcp",
			"--ignore-not-found=true",
		)
		suite.Require().NoError(err, out)
		suite.waitUnavailable(client)
		suite.waitNoTCPFrontends()
		// The Ingress backend is shared with the TCP CR. Wait until HTTP is
		// restored while this namespace is still selected, otherwise the next
		// test sees EOF/500 from a leftover TCP-mode backend.
		suite.waitStatus(suite.client, 200)
	})

	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(client, 200)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app-"))
	suite.waitUnavailable(client)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(client, 200)
}

func (suite *NamespaceSelectorSuite) Test_DeletedNamespaceDropsRoute() {
	namespace := suite.test.GetNS() + "-deleted"
	out, err := kubectl("create", "ns", namespace)
	suite.Require().NoError(err, out)
	suite.T().Cleanup(func() {
		out, err := kubectl("delete", "ns", namespace, "--ignore-not-found=true", "--wait=false")
		suite.Require().NoError(err, out)
	})
	host := namespace + ".watch.test"
	client, err := e2e.NewHTTPClient(host)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml", namespace, nil))
	suite.Require().NoError(suite.test.Apply(
		"config/ingress.yaml.tmpl",
		namespace,
		struct{ Host string }{Host: host},
	))
	suite.Require().NoError(labelNS(namespace, "app=watch"))
	suite.waitStatus(client, 200)

	out, err = kubectl("delete", "ns", namespace, "--wait=true", "--timeout=120s")
	suite.Require().NoError(err, out)
	suite.waitStatus(client, 404)
	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err)
	suite.NotContains(cfg, host)
}

func (suite *NamespaceSelectorSuite) Test_WhitelistPlusSelectorKeepsWhitelist() {
	patched := append([]string{}, suite.originalArgs...)
	patched = append(patched, "--namespace-whitelist="+suite.test.GetNS(), selectorFlag)
	suite.Require().NoError(restoreControllerArgs(patched))
	suite.T().Cleanup(func() {
		suite.Require().NoError(restoreControllerArgs(suite.originalArgs))
		suite.Require().NoError(setControllerSelector())
		suite.Require().NoError(waitControllerRollout())
	})
	// The namespace is deliberately unlabeled. Whitelist must take precedence.
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(labelNS(suite.foreignNS, "app=watch"))
	suite.waitStatus(suite.foreignClient, 404)
	logs, err := kubectl("-n", controllerNS, "logs", "deploy/"+controllerName, "--tail=200")
	suite.Require().NoError(err)
	suite.True(strings.Contains(logs, "--namespace-label-selector is ignored"), logs)
}

func (suite *NamespaceSelectorSuite) Test_BlacklistPlusSelectorKeepsBlacklist() {
	patched := append([]string{}, suite.originalArgs...)
	patched = append(patched, "--namespace-blacklist="+suite.foreignNS, selectorFlag)
	suite.Require().NoError(restoreControllerArgs(patched))
	suite.T().Cleanup(func() {
		suite.Require().NoError(restoreControllerArgs(suite.originalArgs))
		suite.Require().NoError(setControllerSelector())
		suite.Require().NoError(waitControllerRollout())
	})
	// Selector is ignored: the unlabeled, non-blacklisted namespace is watched.
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(labelNS(suite.foreignNS, "app=watch"))
	suite.waitStatus(suite.foreignClient, 404)
	logs, err := kubectl("-n", controllerNS, "logs", "deploy/"+controllerName, "--tail=200")
	suite.Require().NoError(err)
	suite.True(strings.Contains(logs, "--namespace-label-selector is ignored"), logs)
}

func (suite *NamespaceSelectorSuite) Test_DormantDeleteIngressRelabelStays404() {
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app-"))
	suite.waitStatus(suite.client, 404)

	out, err := kubectl("-n", suite.test.GetNS(), "delete", "ingress", "--all", "--wait=true")
	suite.Require().NoError(err, out)
	suite.T().Cleanup(func() {
		suite.Require().NoError(suite.test.Apply(
			"config/ingress.yaml.tmpl",
			suite.test.GetNS(),
			struct{ Host string }{Host: suite.host},
		))
	})

	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 404)
}

func (suite *NamespaceSelectorSuite) Test_SameNameNamespaceRecreateAfterDelete() {
	namespace := suite.test.GetNS() + "-recreate"
	host := namespace + ".watch.test"
	client, err := e2e.NewHTTPClient(host)
	suite.Require().NoError(err)
	suite.T().Cleanup(func() {
		out, delErr := kubectl("delete", "ns", namespace, "--ignore-not-found=true", "--wait=false")
		suite.Require().NoError(delErr, out)
	})

	out, err := kubectl("create", "ns", namespace)
	suite.Require().NoError(err, out)
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml", namespace, nil))
	suite.Require().NoError(suite.test.Apply(
		"config/ingress.yaml.tmpl",
		namespace,
		struct{ Host string }{Host: host},
	))
	suite.Require().NoError(labelNS(namespace, "app=watch"))
	suite.waitStatus(client, 200)

	snapshot := suite.waitControllerSnapshot()
	out, err = kubectl("delete", "ns", namespace, "--wait=true", "--timeout=120s")
	suite.Require().NoError(err, out)
	suite.waitStatus(client, 404)

	out, err = kubectl("create", "ns", namespace)
	suite.Require().NoError(err, out)
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml", namespace, nil))
	suite.Require().NoError(suite.test.Apply(
		"config/ingress.yaml.tmpl",
		namespace,
		struct{ Host string }{Host: host},
	))
	suite.Require().NoError(labelNS(namespace, "app=watch"))
	suite.waitStatus(client, 200)

	after := suite.waitControllerSnapshot()
	suite.Equal(snapshot.UID, after.UID, "same-name recreate must not restart the controller")
}
