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

package crd

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// Adding GlobalSuite, just to be able to debug directly here and not from CRDSuite
type DeployCRDSuite struct {
	CRDSuite
}

func TestDeployCRDSuite(t *testing.T) {
	suite.Run(t, new(DeployCRDSuite))
}

func (suite *DeployCRDSuite) Test_CRD_Deploy_OK() {
	suite.Run("CRs OK", func() {
		suite.tmplData.BackendHashTypeFunction = "sdbm"    // enum ok
		suite.tmplData.GlobalBuffersReserve = 3            // min ok
		suite.tmplData.GlobalEventsMaxEventsAtOnce = 10000 // max ok
		suite.tmplData.DefaultCookieDomain = "example.com" // pattern ok
		suite.Require().NoError(suite.test.Apply("config/global.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().NoError(suite.test.Apply("config/defaults.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().NoError(suite.test.Apply("config/backend.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	})
}

func (suite *DeployCRDSuite) Test_CRD_Deploy_NOK() {
	suite.Run("CRs Enum NOK", func() {
		suite.tmplData.BackendHashTypeFunction = "somethingelse" // enum NOK
		suite.tmplData.GlobalBuffersReserve = 3                  // min ok
		suite.tmplData.GlobalEventsMaxEventsAtOnce = 10000       // max ok
		suite.tmplData.DefaultCookieDomain = "example.com"       // pattern ok
		suite.Require().NoError(suite.test.Apply("config/global.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().NoError(suite.test.Apply("config/defaults.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		// NOK enum
		err := suite.test.Apply("config/backend.yaml.tmpl", suite.test.GetNS(), suite.tmplData)
		suite.Require().ErrorContains(err, `The Backend "backends-test" is invalid: spec.config.hash_type.function: Unsupported value:`)
	})

	suite.Run("CRs Minimum NOK", func() {
		suite.tmplData.BackendHashTypeFunction = "sdbm"    // enum ok
		suite.tmplData.GlobalBuffersReserve = 1            // min NOK
		suite.tmplData.GlobalEventsMaxEventsAtOnce = 10000 // max ok
		suite.tmplData.DefaultCookieDomain = "example.com" // pattern ok
		// Min NOK
		err := suite.test.Apply("config/global.yaml.tmpl", suite.test.GetNS(), suite.tmplData)
		suite.Require().ErrorContains(err, `The Global "globals-test" is invalid: spec.config.tune_options.buffers_reserve: Invalid value:`)
		suite.Require().NoError(suite.test.Apply("config/defaults.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().NoError(suite.test.Apply("config/backend.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	})

	suite.Run("CRs Maximum NOK", func() {
		suite.tmplData.BackendHashTypeFunction = "sdbm"    // enum ok
		suite.tmplData.GlobalBuffersReserve = 2            // min ok
		suite.tmplData.GlobalEventsMaxEventsAtOnce = 10005 // max NOK
		suite.tmplData.DefaultCookieDomain = "example.com" // pattern ok
		// Min NOK
		err := suite.test.Apply("config/global.yaml.tmpl", suite.test.GetNS(), suite.tmplData)
		suite.Require().ErrorContains(err, `The Global "globals-test" is invalid: spec.config.tune_options.events_max_events_at_once: Invalid value:`)
		suite.Require().NoError(suite.test.Apply("config/defaults.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().NoError(suite.test.Apply("config/backend.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	})

	suite.Run("CRs Pattern NOK", func() {
		suite.tmplData.BackendHashTypeFunction = "sdbm"    // enum ok
		suite.tmplData.GlobalBuffersReserve = 2            // min ok
		suite.tmplData.GlobalEventsMaxEventsAtOnce = 10000 // max ok
		suite.tmplData.DefaultCookieDomain = "two words"   // pattern NOK
		suite.Require().NoError(suite.test.Apply("config/global.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		// Pattern NOK
		err := suite.test.Apply("config/defaults.yaml.tmpl", suite.test.GetNS(), suite.tmplData)
		suite.Require().ErrorContains(err, `The Defaults "defaults-test" is invalid: spec.config.cookie.domain[0].value: Invalid value:`)
		suite.Require().NoError(suite.test.Apply("config/backend.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	})
}
