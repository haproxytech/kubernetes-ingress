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

package crdfrontend

import (
	"strings"
	"testing"

	models "github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

// Adding FrontendSuite, just to be able to debug directly here and not from CRDSuite
type FrontendSuite struct {
	CRDFrontendSuite
}

func TestFrontendSuite(t *testing.T) {
	suite.Run(t, new(FrontendSuite))
}

func (suite *FrontendSuite) Test_CR_Frontend() {
	var port int64 = 8080
	var portTest int64 = 9080
	binds := map[string]models.Bind{
		"v4": {
			BindParams: models.BindParams{
				Name: "v4",
			},
			Address: "0.0.0.0",
			Port:    &port,
		},
		"v6": {
			BindParams: models.BindParams{
				Name: "v6",
			},
			Address: "::",
			Port:    &port,
		},
		"test-http": {
			BindParams: models.BindParams{
				Name: "test-http",
			},
			Address: "127.0.0.1",
			Port:    &portTest,
		},
	}

	suite.Run("CRs OK", func() {
		// Add HTTP frontend custom resource
		frontendPath := "config/frontend-http.yaml"
		suite.Require().NoError(suite.test.Apply(frontendPath, "", nil))
		suite.test.AddTearDown(func() error {
			return suite.test.Delete(frontendPath)
		})

		// Add frontend custom resource to configmap
		configmapFrontendPath := "config/configmap-frontend-http.yaml"
		suite.Require().NoError(suite.test.Apply(configmapFrontendPath, "", nil))
		suite.test.AddTearDown(func() error {
			return suite.test.Apply("../../config/2.configmap.yaml", "", nil)
		})

		suite.Require().Eventually(func() bool {
			httpFrontend, err := suite.getFrontendConfiguration("http")
			if err != nil || httpFrontend == nil {
				return false
			}

			return httpFrontend.Name == "http" && httpFrontend.Mode == "http" &&
				EqualBinds(httpFrontend.Binds, binds)
		}, e2e.WaitDuration, e2e.TickDuration, "waiting for HTTP frontend custom resource to be applied")
	})
}

func EqualBinds(left, right map[string]models.Bind) bool {
	if len(left) != len(right) {
		return false
	}
	for k, bind := range left {
		if !bind.Equal(right[k]) {
			return false
		}
	}
	return true
}

// sslPassthroughBinds: the only binds of the ssl frontend while passthrough is on.
var sslPassthroughBinds = []string{"v4", "v6"}

// sslCRBindName: the bind of the custom resource, ignored by the controller.
const sslCRBindName = "should-be-ignored"

// sslCRBackend: the default backend of the custom resource, ignored by the controller.
const sslCRBackend = "should-be-ignored"

const sslCRLogFormat = "custom-ssl-cr"

const sslCRCRRuleCondTest = "5.6.7.8"

// sslCRConfigTmplData feeds the configmap template with the merge mode suffix.
type sslCRConfigTmplData struct {
	Suffix string
}

// sslCRInspectDelayIndex returns the index of the generated inspect-delay rule, or -1.
func sslCRInspectDelayIndex(rules models.TCPRequestRules) int {
	for i, rule := range rules {
		if rule.Type == "inspect-delay" {
			return i
		}
	}
	return -1
}

// sslCRCRRuleIndex returns the index of the custom resource rule, or -1.
func sslCRCRRuleIndex(rules models.TCPRequestRules) int {
	for i, rule := range rules {
		if rule.Action == "reject" && strings.Contains(rule.CondTest, sslCRCRRuleCondTest) {
			return i
		}
	}
	return -1
}

// sslCRGuardsHold verifies the controller-owned fields are never amended:
// name, mode, passthrough chaining backend and binds.
func (suite *FrontendSuite) sslCRGuardsHold(f *models.Frontend) bool {
	if f.Name != "ssl" || f.Mode != "tcp" {
		return false
	}
	if f.DefaultBackend == "" || f.DefaultBackend == sslCRBackend {
		return false
	}
	if len(f.Binds) != len(sslPassthroughBinds) {
		return false
	}
	if _, ignored := f.Binds[sslCRBindName]; ignored {
		return false
	}
	for _, name := range sslPassthroughBinds {
		if _, ok := f.Binds[name]; !ok {
			return false
		}
	}
	return true
}

// sslCRACLApplied verifies the acl of the custom resource is applied.
func (suite *FrontendSuite) sslCRACLApplied(f *models.Frontend) bool {
	if len(f.ACLList) != 1 {
		return false
	}
	return f.ACLList[0].ACLName == "cr_ssl_test"
}

// Test_CR_Frontend_SSL covers the cr-frontend-ssl annotation.
func (suite *FrontendSuite) Test_CR_Frontend_SSL() {
	frontendPath := "config/frontend-ssl.yaml"
	configmapFrontendPath := "config/configmap-frontend-ssl.yaml.tmpl"

	// Enable SSL passthrough; the route targets an endpoint-less service.
	ingressPath := "config/ingress-ssl-passthrough.yaml"
	suite.Require().NoError(suite.test.Apply(ingressPath, suite.test.GetNS(), nil))

	// Add ssl frontend custom resource
	suite.Require().NoError(suite.test.Apply(frontendPath, "", nil))
	suite.test.AddTearDown(func() error {
		return suite.test.Delete(frontendPath)
	})

	// Add frontend custom resource to configmap, restored in teardown
	suite.test.AddTearDown(func() error {
		return suite.test.Apply("../../config/2.configmap.yaml", "", nil)
	})

	suite.Run("ssl frontend created by passthrough", func() {
		suite.Require().Eventually(func() bool {
			sslFrontend, err := suite.getFrontendConfiguration("ssl")
			if err != nil || sslFrontend == nil {
				return false
			}
			return sslFrontend.Name == "ssl" && sslFrontend.Mode == "tcp" &&
				len(sslFrontend.Binds) == len(sslPassthroughBinds) && sslCRInspectDelayIndex(sslFrontend.TCPRequestRuleList) >= 0
		}, e2e.WaitDuration, e2e.TickDuration, "waiting for ssl passthrough frontend to be created")
	})

	// applyConfigMap applies the controller configmap with a merge mode suffix.
	applyConfigMap := func(suffix string) {
		suite.Require().NoError(suite.test.Apply(configmapFrontendPath, "", sslCRConfigTmplData{Suffix: suffix}))
	}

	suite.Run("prepend puts the custom resource rules first", func() {
		applyConfigMap(":prepend")
		suite.Require().Eventually(func() bool {
			sslFrontend, err := suite.getFrontendConfiguration("ssl")
			if err != nil || sslFrontend == nil {
				return false
			}
			crRule := sslCRCRRuleIndex(sslFrontend.TCPRequestRuleList)
			inspectDelay := sslCRInspectDelayIndex(sslFrontend.TCPRequestRuleList)
			return suite.sslCRGuardsHold(sslFrontend) && suite.sslCRACLApplied(sslFrontend) &&
				strings.Contains(sslFrontend.LogFormat, sslCRLogFormat) &&
				crRule >= 0 && inspectDelay >= 0 && crRule < inspectDelay
		}, e2e.WaitDuration, e2e.TickDuration, "waiting for ssl frontend custom resource to be applied")
	})

	suite.Run("append is the default", func() {
		applyConfigMap("")
		suite.Require().Eventually(func() bool {
			sslFrontend, err := suite.getFrontendConfiguration("ssl")
			if err != nil || sslFrontend == nil {
				return false
			}
			crRule := sslCRCRRuleIndex(sslFrontend.TCPRequestRuleList)
			inspectDelay := sslCRInspectDelayIndex(sslFrontend.TCPRequestRuleList)
			return suite.sslCRGuardsHold(sslFrontend) && suite.sslCRACLApplied(sslFrontend) &&
				strings.Contains(sslFrontend.LogFormat, sslCRLogFormat) &&
				crRule >= 0 && inspectDelay >= 0 && crRule > inspectDelay
		}, e2e.WaitDuration, e2e.TickDuration, "waiting for ssl frontend custom resource to be appended")
	})

	suite.Run("override replaces the controller-generated rules", func() {
		applyConfigMap(":override")
		suite.Require().Eventually(func() bool {
			sslFrontend, err := suite.getFrontendConfiguration("ssl")
			if err != nil || sslFrontend == nil {
				return false
			}
			return suite.sslCRGuardsHold(sslFrontend) && suite.sslCRACLApplied(sslFrontend) &&
				strings.Contains(sslFrontend.LogFormat, sslCRLogFormat) &&
				sslCRCRRuleIndex(sslFrontend.TCPRequestRuleList) == 0 &&
				len(sslFrontend.TCPRequestRuleList) == 1
		}, e2e.WaitDuration, e2e.TickDuration, "waiting for ssl frontend custom resource to override the rules")
	})

	suite.Run("removal reverts to the controller-generated configuration", func() {
		suite.Require().NoError(suite.test.Apply("../../config/2.configmap.yaml", "", nil))
		suite.Require().Eventually(func() bool {
			sslFrontend, err := suite.getFrontendConfiguration("ssl")
			if err != nil || sslFrontend == nil {
				return false
			}
			return suite.sslCRGuardsHold(sslFrontend) &&
				!strings.Contains(sslFrontend.LogFormat, sslCRLogFormat) &&
				len(sslFrontend.ACLList) == 0 &&
				sslCRCRRuleIndex(sslFrontend.TCPRequestRuleList) < 0 &&
				sslCRInspectDelayIndex(sslFrontend.TCPRequestRuleList) >= 0
		}, e2e.WaitDuration, e2e.TickDuration, "waiting for ssl frontend custom resource to be removed")
	})
}
