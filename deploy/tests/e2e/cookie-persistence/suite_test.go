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

//go:build e2e_parallel

package cookiepersistence

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	parser "github.com/haproxytech/client-native/v5/config-parser"
	"github.com/haproxytech/client-native/v5/config-parser/params"
	"github.com/haproxytech/client-native/v5/config-parser/types"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type CookiePersistenceSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	CookiePersistenceDynamic   bool
	CookiePersistenceNoDynamic bool
	Host                       string
}

func (suite *CookiePersistenceSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.Require().NoError(err)
	suite.tmplData = tmplData{Host: suite.test.GetNS() + ".test"}
	suite.client, err = e2e.NewHTTPClient(suite.tmplData.Host)
	suite.Require().NoError(err)
}

func (suite *CookiePersistenceSuite) TearDownSuite() {
	suite.test.TearDown()
}

func TestCookiePersistenceSuite(t *testing.T) {
	suite.Run(t, new(CookiePersistenceSuite))
}

// Check that the server serverName for backend backendName
// has a "cookie" params with a value equal to the server name
func (suite *CookiePersistenceSuite) checkServerCookie(p parser.Parser, backendName, serverName string) {
	v, err := p.Get(parser.Backends, backendName, "server")
	suite.Require().NoError(err, "Could not get Haproxy config parser servers for backend %s", backendName)

	ondiskServers, ok := v.([]types.Server)
	suite.Require().Equal(ok, true, "Could not get Haproxy config parser servers for backend %s", backendName)

	paramName := "cookie"
	var cookieParam *params.ServerOptionValue
	for _, server := range ondiskServers {
		if server.Name != serverName {
			continue
		}
		serverParams := server.Params
		for _, serverParam := range serverParams {
			optionValue, ok := serverParam.(*params.ServerOptionValue)
			if !ok {
				continue
			}
			if optionValue.Name != paramName {
				continue
			}
			cookieParam = optionValue
			break
		}
	}
	suite.Require().NotNil(cookieParam)
	suite.Require().Equal(cookieParam.Value, serverName)
}

// Check that the server serverName for backend backendName
// has a NOT a "cookie" params
func (suite *CookiePersistenceSuite) checkServerNoCookie(p parser.Parser, backendName, serverName string) error {
	v, err := p.Get(parser.Backends, backendName, "server")
	if err != nil {
		return errors.New("Could not get Haproxy config parser servers for backend " + backendName)
	}

	ondiskServers, ok := v.([]types.Server)
	if !ok {
		return errors.New("Could not get Haproxy config parser servers for backend " + backendName)
	}

	paramName := "cookie"
	cookieParamFound := false
	for _, server := range ondiskServers {
		if server.Name != serverName {
			continue
		}
		serverParams := server.Params
		for _, serverParam := range serverParams {
			optionValue, ok := serverParam.(*params.ServerOptionValue)
			if !ok {
				continue
			}
			if optionValue.Name != paramName {
				continue
			}
			cookieParamFound = true
			break
		}
	}
	if cookieParamFound {
		return errors.New("Found cookie param for server " + serverName)
	}
	return nil
}
