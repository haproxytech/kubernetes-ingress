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

package cors

import (
	"net/http"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

const (
	AccessControlAllowOrigin     = "Access-Control-Allow-Origin"
	AccessControlAllowMethods    = "Access-Control-Allow-Methods"
	AccessControlAllowHeaders    = "Access-Control-Allow-Headers"
	AccessControlMaxAge          = "Access-Control-Max-Age"
	AccessControlAllowCredential = "Access-Control-Allow-Credentials"
	AnnotationCorsEnable         = "cors-enable"
	AnnotationCorsOrigin         = "cors-allow-origin"
	AnnotationCorsMethods        = "cors-allow-methods"
	AnnotationCorsHeaders        = "cors-allow-headers"
	AnnotationCorsAge            = "cors-max-age"
	AnnotationCorsCredential     = "cors-allow-credentials"
	Star                         = "*"
)

func (suite *CorsSuite) Test_Ingress_Alone() {
	suite.Run("Default", func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {Star},
			AccessControlMaxAge:       {"5"},
		}
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, http.Header{})
	})

	suite.Run("CorsOriginAlone", func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {"http://" + suite.tmplData.Host},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {Star},
			AccessControlMaxAge:       {"5"},
		}
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsOrigin, q("http://" + suite.tmplData.Host)},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, http.Header{})
	})

	suite.Run("CorsMethodsAlone", func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {"GET"},
			AccessControlAllowHeaders: {Star},
			AccessControlMaxAge:       {"5"},
		}
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsMethods, q("GET")},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, http.Header{})
	})

	suite.Run("CorsMethodsHeaders", func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {"Accept"},
			AccessControlMaxAge:       {"5"},
		}
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsHeaders, q("Accept")},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, http.Header{})
	})

	suite.Run("CorsMethodsAge", func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {Star},
			AccessControlMaxAge:       {"500"},
		}
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsAge, q("500s")},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, http.Header{})
	})

	suite.Run("CorsMethodsCredential", func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:     {Star},
			AccessControlAllowMethods:    {Star},
			AccessControlAllowHeaders:    {Star},
			AccessControlAllowCredential: {"true"},
			AccessControlMaxAge:          {"5"},
		}
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsCredential, q("true")},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, http.Header{})
	})

	suite.Run("CorsDisable", func() {
		unexpectedHeaders := http.Header{
			AccessControlAllowOrigin:     {},
			AccessControlAllowMethods:    {},
			AccessControlAllowHeaders:    {},
			AccessControlMaxAge:          {},
			AccessControlAllowCredential: {},
		}

		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("false")},
			{AnnotationCorsOrigin, q("http://wrong.com")},
			{AnnotationCorsCredential, q("true")},
			{AnnotationCorsMethods, q("GET")},
			{AnnotationCorsHeaders, q("Accept")},
		}

		suite.NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

		suite.eventuallyReturns(http.Header{}, unexpectedHeaders)
	})
}

func (suite *CorsSuite) eventuallyReturns(expecedHeaders, unexpectedHeaders http.Header) {
	suite.Eventually(func() bool {
		res, cls, err := suite.client.Do()
		if err != nil {
			suite.T().Logf("Connection ERROR: %s", err.Error())
			return false
		}
		defer cls()
		for expectedHeader, expectedValues := range expecedHeaders {
			values, ok := res.Header[expectedHeader]
			if !ok || len(values) != 1 || values[0] != expectedValues[0] {
				return false
			}

		}
		for unexpectedHeader := range unexpectedHeaders {
			if _, ok := res.Header[unexpectedHeader]; ok {
				return false
			}
		}
		return true
	}, e2e.WaitDuration, e2e.TickDuration)
}

func q(value string) string {
	return "\"" + value + "\""
}
