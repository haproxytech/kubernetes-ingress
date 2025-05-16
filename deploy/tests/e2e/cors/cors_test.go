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
	AccessControlAllowOrigin        = "Access-Control-Allow-Origin"
	AccessControlAllowMethods       = "Access-Control-Allow-Methods"
	AccessControlAllowHeaders       = "Access-Control-Allow-Headers"
	AccessControlMaxAge             = "Access-Control-Max-Age"
	AccessControlAllowCredential    = "Access-Control-Allow-Credentials"
	AnnotationCorsEnable            = "cors-enable"
	AnnotationCorsOrigin            = "cors-allow-origin"
	AnnotationCorsMethods           = "cors-allow-methods"
	AnnotationCorsHeaders           = "cors-allow-headers"
	AnnotationCorsAge               = "cors-max-age"
	AnnotationCorsCredential        = "cors-allow-credentials"
	AnnotationsCorsRespondToOptions = "cors-respond-to-options"
	Star                            = "*"
)

func (suite *CorsSuite) Test_Configmap_Alone() {
	suite.Run("Default", suite.Default(false))
	suite.Run("DefaultWithNoContent", suite.DefaultWithNoContent(false))
	suite.Run("CorsOriginAlone", suite.CorsOriginAlone(false))
	suite.Run("CorsMethodsAlone", suite.CorsMethodsAlone(false))
	suite.Run("CorsMethodsHeadersAlone", suite.CorsMethodsHeadersAlone(false))
	suite.Run("CorsMethodsAgeAlone", suite.CorsMethodsAgeAlone(false))
	suite.Run("CorsMethodsCredentialAlone", suite.CorsMethodsCredentialAlone(false))
	suite.Run("CorsDisable", suite.CorsDisable(false))
	suite.Run("CorsMethodsCredentialDisable", suite.CorsMethodsCredentialDisable(false))
	suite.Require().NoError(suite.test.Apply("../../config/2.configmap.yaml", "", nil))
}

func (suite *CorsSuite) Test_Ingress_Alone() {
	suite.Run("Default", suite.Default(true))
	suite.Run("DefaultWithNoContent", suite.DefaultWithNoContent(true))
	suite.Run("CorsOriginAlone", suite.CorsOriginAlone(true))
	suite.Run("CorsMethodsAlone", suite.CorsMethodsAlone(true))
	suite.Run("CorsMethodsHeadersAlone", suite.CorsMethodsHeadersAlone(true))
	suite.Run("CorsMethodsAgeAlone", suite.CorsMethodsAgeAlone(true))
	suite.Run("CorsMethodsCredentialAlone", suite.CorsMethodsCredentialAlone(true))
	suite.Run("CorsDisable", suite.CorsDisable(true))
	suite.Run("CorsMethodsCredentialDisable", suite.CorsMethodsCredentialDisable(true))
}

func (suite *CorsSuite) eventuallyReturns(expectedHeaders, unexpectedHeaders http.Header) {
	suite.eventuallyReturnsWithNoContentOption(expectedHeaders, unexpectedHeaders, false)
}

func (suite *CorsSuite) eventuallyReturnsWithNoContentOption(expectedHeaders, unexpectedHeaders http.Header, noContent bool) {
	suite.Eventually(func() bool {
		do := suite.client.Do
		if noContent {
			do = suite.client.DoOptions
		}
		res, cls, err := do()
		if err != nil {
			suite.T().Logf("Connection ERROR: %s", err.Error())
			return false
		}
		defer cls()

		if res.StatusCode == 503 {
			return false
		}
		if noContent && res.StatusCode != 204 {
			return false
		}

		for expectedHeader, expectedValues := range expectedHeaders {
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

func (suite *CorsSuite) Default(ingressCors bool) func() {
	return func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {Star},
			AccessControlMaxAge:       {"5"},
		}
		unexpectedHeaders := http.Header{
			AccessControlAllowCredential: {},
		}
		annotations := &suite.tmplData.IngAnnotations
		if !ingressCors {
			annotations = &suite.tmplData.ConfigMapAnnotations
		}
		*annotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
		}

		yamlFile := "config/deploy.yaml.tmpl"
		ns := suite.test.GetNS()
		if !ingressCors {
			yamlFile = "config/configmap.yaml.tmpl"
			ns = ""
		}
		suite.Require().NoError(suite.test.Apply(yamlFile, ns, suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, unexpectedHeaders)
	}
}

func (suite *CorsSuite) DefaultWithNoContent(ingressCors bool) func() {
	return func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {Star},
		}
		unexpectedHeaders := http.Header{
			AccessControlAllowCredential: {},
		}
		annotations := &suite.tmplData.IngAnnotations
		if !ingressCors {
			annotations = &suite.tmplData.ConfigMapAnnotations
		}
		*annotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationsCorsRespondToOptions, q("true")},
		}

		yamlFile := "config/deploy.yaml.tmpl"
		ns := suite.test.GetNS()
		if !ingressCors {
			yamlFile = "config/configmap.yaml.tmpl"
			ns = ""
		}
		suite.Require().NoError(suite.test.Apply(yamlFile, ns, suite.tmplData))

		suite.eventuallyReturnsWithNoContentOption(expectedHeaders, unexpectedHeaders, true)
	}
}

func (suite *CorsSuite) CorsOriginAlone(ingressCors bool) func() {
	return func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {"http://" + suite.tmplData.Host},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {Star},
			AccessControlMaxAge:       {"5"},
		}
		unexpectedHeaders := http.Header{
			AccessControlAllowCredential: {},
		}
		annotations := &suite.tmplData.IngAnnotations
		if !ingressCors {
			annotations = &suite.tmplData.ConfigMapAnnotations
		}
		*annotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsOrigin, q("http://" + suite.tmplData.Host)},
		}

		yamlFile := "config/deploy.yaml.tmpl"
		ns := suite.test.GetNS()
		if !ingressCors {
			yamlFile = "config/configmap.yaml.tmpl"
			ns = ""
		}
		suite.Require().NoError(suite.test.Apply(yamlFile, ns, suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, unexpectedHeaders)
	}
}

func (suite *CorsSuite) CorsMethodsAlone(ingressCors bool) func() {
	return func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {"GET"},
			AccessControlAllowHeaders: {Star},
			AccessControlMaxAge:       {"5"},
		}
		unexpectedHeaders := http.Header{
			AccessControlAllowCredential: {},
		}
		annotations := &suite.tmplData.IngAnnotations
		if !ingressCors {
			annotations = &suite.tmplData.ConfigMapAnnotations
		}
		*annotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsMethods, q("GET")},
		}

		yamlFile := "config/deploy.yaml.tmpl"
		ns := suite.test.GetNS()
		if !ingressCors {
			yamlFile = "config/configmap.yaml.tmpl"
			ns = ""
		}
		suite.Require().NoError(suite.test.Apply(yamlFile, ns, suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, unexpectedHeaders)
	}
}

func (suite *CorsSuite) CorsMethodsHeadersAlone(ingressCors bool) func() {
	return func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {"Accept"},
			AccessControlMaxAge:       {"5"},
		}
		unexpectedHeaders := http.Header{
			AccessControlAllowCredential: {},
		}
		annotations := &suite.tmplData.IngAnnotations
		if !ingressCors {
			annotations = &suite.tmplData.ConfigMapAnnotations
		}
		*annotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsHeaders, q("Accept")},
		}

		yamlFile := "config/deploy.yaml.tmpl"
		ns := suite.test.GetNS()
		if !ingressCors {
			yamlFile = "config/configmap.yaml.tmpl"
			ns = ""
		}
		suite.Require().NoError(suite.test.Apply(yamlFile, ns, suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, unexpectedHeaders)
	}
}

func (suite *CorsSuite) CorsMethodsAgeAlone(ingressCors bool) func() {
	return func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {Star},
			AccessControlMaxAge:       {"500"},
		}
		unexpectedHeaders := http.Header{
			AccessControlAllowCredential: {},
		}
		annotations := &suite.tmplData.IngAnnotations
		if !ingressCors {
			annotations = &suite.tmplData.ConfigMapAnnotations
		}
		*annotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsAge, q("500s")},
		}

		yamlFile := "config/deploy.yaml.tmpl"
		ns := suite.test.GetNS()
		if !ingressCors {
			yamlFile = "config/configmap.yaml.tmpl"
			ns = ""
		}
		suite.Require().NoError(suite.test.Apply(yamlFile, ns, suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, unexpectedHeaders)
	}
}

func (suite *CorsSuite) CorsMethodsCredentialDisable(ingressCors bool) func() {
	return func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:  {Star},
			AccessControlAllowMethods: {Star},
			AccessControlAllowHeaders: {Star},
			AccessControlMaxAge:       {"5"},
		}
		unexpectedHeaders := http.Header{
			AccessControlAllowCredential: {},
		}

		annotations := &suite.tmplData.IngAnnotations
		if !ingressCors {
			annotations = &suite.tmplData.ConfigMapAnnotations
		}
		*annotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsCredential, q("false")},
		}
		yamlFile := "config/deploy.yaml.tmpl"
		ns := suite.test.GetNS()
		if !ingressCors {
			yamlFile = "config/configmap.yaml.tmpl"
			ns = ""
		}
		suite.Require().NoError(suite.test.Apply(yamlFile, ns, suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, unexpectedHeaders)
	}
}

func (suite *CorsSuite) CorsMethodsCredentialAlone(ingressCors bool) func() {
	return func() {
		expectedHeaders := http.Header{
			AccessControlAllowOrigin:     {Star},
			AccessControlAllowMethods:    {Star},
			AccessControlAllowHeaders:    {Star},
			AccessControlAllowCredential: {"true"},
			AccessControlMaxAge:          {"5"},
		}
		annotations := &suite.tmplData.IngAnnotations
		if !ingressCors {
			annotations = &suite.tmplData.ConfigMapAnnotations
		}
		*annotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("true")},
			{AnnotationCorsCredential, q("true")},
		}
		yamlFile := "config/deploy.yaml.tmpl"
		ns := suite.test.GetNS()
		if !ingressCors {
			yamlFile = "config/configmap.yaml.tmpl"
			ns = ""
		}
		suite.Require().NoError(suite.test.Apply(yamlFile, ns, suite.tmplData))

		suite.eventuallyReturns(expectedHeaders, http.Header{})
	}
}

func (suite *CorsSuite) CorsDisable(ingressCors bool) func() {
	return func() {
		unexpectedHeaders := http.Header{
			AccessControlAllowOrigin:     {},
			AccessControlAllowMethods:    {},
			AccessControlAllowHeaders:    {},
			AccessControlMaxAge:          {},
			AccessControlAllowCredential: {},
		}

		annotations := &suite.tmplData.IngAnnotations
		if !ingressCors {
			annotations = &suite.tmplData.ConfigMapAnnotations
		}
		*annotations = []struct{ Key, Value string }{
			{AnnotationCorsEnable, q("false")},
			{AnnotationCorsOrigin, q("http://wrong.com")},
			{AnnotationCorsCredential, q("true")},
			{AnnotationCorsMethods, q("GET")},
			{AnnotationCorsHeaders, q("Accept")},
		}

		yamlFile := "config/deploy.yaml.tmpl"
		ns := suite.test.GetNS()
		if !ingressCors {
			yamlFile = "config/configmap.yaml.tmpl"
			ns = ""
		}
		suite.Require().NoError(suite.test.Apply(yamlFile, ns, suite.tmplData))
		suite.eventuallyReturns(http.Header{}, unexpectedHeaders)
	}
}
