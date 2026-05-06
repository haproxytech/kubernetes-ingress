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

package crdbackend

import (
	"strings"

	"github.com/stretchr/testify/suite"

	parser "github.com/haproxytech/client-native/v6/config-parser"
	"github.com/haproxytech/client-native/v6/config-parser/options"
	"github.com/haproxytech/client-native/v6/configuration"
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type CRDBackendSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	Host        string
	BackendName string
	HeaderName  string
	HeaderValue string
	HealthURI   string
	LogAddress  string
}

func (suite *CRDBackendSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.Require().NoError(err)
	// BackendName must match the section name the controller renders into
	// haproxy.cfg. The Backend CR's spec.name is overwritten by service.go
	// with "<namespace>_svc_<service>_<port>" — see GetBackendName.
	suite.tmplData = tmplData{
		Host:        suite.test.GetNS() + ".test",
		BackendName: suite.test.GetNS() + "_svc_http-echo_http",
	}
	suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
	suite.Require().NoError(err)

	crdPath := "../../../../crs/definition/ingress.v3.haproxy.org_backends.yaml"
	suite.Require().NoError(suite.test.Apply(crdPath, suite.test.GetNS(), nil))

	// Service + Ingress that the Backend CR will be bound to via cr-backend annotation.
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml", suite.test.GetNS(), suite.tmplData))
	suite.Require().Eventually(func() bool {
		r, cls, err := suite.client.Do()
		if err != nil {
			return false
		}
		defer cls()
		return r.StatusCode == 200
	}, e2e.WaitDuration, e2e.TickDuration)
}

func (suite *CRDBackendSuite) TearDownSuite() {
	suite.test.TearDown()
}

// getBackend fetches the rendered haproxy.cfg from the running ingress controller
// pod, parses it, and returns a populated *models.Backend that includes the rule
// lists we want to assert on.
func (suite *CRDBackendSuite) getBackend(name string) (*models.Backend, error) {
	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get HAProxy config")

	p, err := parser.New(options.Reader(strings.NewReader(cfg)))
	suite.Require().NoError(err, "Could not parse HAProxy config")

	b := &models.Backend{BackendBase: models.BackendBase{Name: name}}
	if err := configuration.ParseSection(&b.BackendBase, parser.Backends, name, p); err != nil {
		return nil, err
	}
	if rules, err := configuration.ParseHTTPRequestRules(configuration.BackendParentName, name, p); err == nil {
		b.HTTPRequestRuleList = rules
	}
	if rules, err := configuration.ParseHTTPResponseRules(configuration.BackendParentName, name, p); err == nil {
		b.HTTPResponseRuleList = rules
	}
	if rules, err := configuration.ParseHTTPAfterRules(configuration.BackendParentName, name, p); err == nil {
		b.HTTPAfterResponseRuleList = rules
	}
	if rules, err := configuration.ParseHTTPChecks(configuration.BackendParentName, name, p); err == nil {
		b.HTTPCheckList = rules
	}
	if rules, err := configuration.ParseLogTargets(configuration.BackendParentName, name, p); err == nil {
		b.LogTargetList = rules
	}
	if rules, err := configuration.ParseFilters(configuration.BackendParentName, name, p); err == nil {
		b.FilterList = rules
	}
	if rules, err := configuration.ParseServerSwitchingRules(name, p); err == nil {
		b.ServerSwitchingRuleList = rules
	}
	if rules, err := configuration.ParseStickRules(name, p); err == nil {
		b.StickRuleList = rules
	}
	if rules, err := configuration.ParseTCPRequestRules(configuration.BackendParentName, name, p); err == nil {
		b.TCPRequestRuleList = rules
	}
	if rules, err := configuration.ParseACLs(configuration.BackendParentName, name, p); err == nil {
		b.ACLList = rules
	}
	return b, nil
}
