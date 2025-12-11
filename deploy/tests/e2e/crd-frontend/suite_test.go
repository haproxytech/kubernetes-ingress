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

	"github.com/stretchr/testify/suite"

	parser "github.com/haproxytech/client-native/v6/config-parser"
	"github.com/haproxytech/client-native/v6/config-parser/options"
	"github.com/haproxytech/client-native/v6/configuration"
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type CRDFrontendSuite struct {
	suite.Suite
	test   e2e.Test
	client *e2e.Client
}

func (suite *CRDFrontendSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.Require().NoError(err)
	suite.client, err = e2e.NewHTTPClient(suite.test.GetNS() + ".test")
	suite.Require().NoError(err)
	crdPath := "../../../../crs/definition/ingress.v3.haproxy.org_frontends.yaml"
	suite.Require().NoError(suite.test.Apply(crdPath, suite.test.GetNS(), nil))
}

func (suite *CRDFrontendSuite) TearDownSuite() {
	suite.test.TearDown()
}

func (suite *CRDFrontendSuite) getFrontendConfiguration(frontendName string) (*models.Frontend, error) {
	cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
	suite.Require().NoError(err, "Could not get Haproxy config")
	reader := strings.NewReader(cfg)
	p, err := parser.New(options.Reader(reader))
	suite.Require().NoError(err, "Could not get Haproxy config parser")

	f := &models.Frontend{FrontendBase: models.FrontendBase{Name: frontendName}}
	if err := configuration.ParseSection(&f.FrontendBase, parser.Frontends, frontendName, p); err != nil {
		return nil, err
	}

	acls, err := configuration.ParseACLs(configuration.FrontendParentName, frontendName, p)
	if err != nil {
		return nil, err
	}
	f.ACLList = acls

	binds, err := configuration.ParseBinds(configuration.FrontendParentName, frontendName, p)
	if err != nil {
		return nil, err
	}
	f.Binds = ConvertBinds(binds)
	return f, nil
}

func ConvertBinds(binds models.Binds) map[string]models.Bind {
	convertedBinds := map[string]models.Bind{}
	for _, bind := range binds {
		convertedBinds[bind.Name] = *bind
	}
	return convertedBinds
}
