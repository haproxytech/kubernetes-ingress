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

// +build integration

package ingressmatch

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

// For each test, requests made to "paths" should be
// answered by the corresponding service
// Ref: https://kubernetes.io/docs/concepts/services-networking/ingress/#path-types
type test struct {
	target string
	host   string
	paths  []string
}

var ingressRules = []IngressRule{
	{Service: "http-echo-1", Host: "app.haproxy", Path: "/"},
	{Service: "http-echo-2", Host: "app.haproxy", Path: "/a"},
	{Service: "http-echo-3", Host: "app.haproxy", Path: "/a/b"},
	{Service: "http-echo-4", Host: "app.haproxy", Path: "/exact", PathType: "Exact"},
	{Service: "http-echo-5", Host: "app.haproxy", Path: "/exactslash/", PathType: "Exact"},
	{Service: "http-echo-6", Host: "app.haproxy", Path: "/prefix", PathType: "Prefix"},
	{Service: "http-echo-7", Host: "app.haproxy", Path: "/prefixslash", PathType: "Prefix"},
	{Service: "http-echo-8", Host: "sub.app.haproxy"},
	{Service: "http-echo-9", Host: "*.haproxy"},
	{Service: "http-echo-10", Path: "/test"},
}
var tests = []test{
	{ingressRules[0].Service, "app.haproxy", []string{"/", "/test", "/exact/", "/exactslash", "/exactslash/foo", "/prefixxx"}},
	{ingressRules[1].Service, "app.haproxy", []string{"/a"}},
	{ingressRules[2].Service, "app.haproxy", []string{"/a/b"}},
	{ingressRules[3].Service, "app.haproxy", []string{"/exact"}},
	{ingressRules[4].Service, "app.haproxy", []string{"/exactslash/"}},
	{ingressRules[5].Service, "app.haproxy", []string{"/prefix", "/prefix/", "/prefix/foo"}},
	{ingressRules[6].Service, "app.haproxy", []string{"/prefixslash", "/prefixslash/", "/prefixslash/foo/bar"}},
	{ingressRules[7].Service, "sub.app.haproxy", []string{"/test"}},
	{ingressRules[8].Service, "test.haproxy", []string{"/test"}},
	{ingressRules[9].Service, "foo.bar", []string{"/test"}},
}

func (suite *IngressMatchSuite) BeforeTest(suiteName, testName string) {
	suite.tmplData.PathTypeSupported = true
	major, minor, err := suite.test.GetK8sVersion()
	suite.NoError(err)
	if major == 1 && minor < 18 {
		suite.tmplData.PathTypeSupported = false
		tests[0] = test{ingressRules[0].Service, "app.haproxy", []string{"/", "/test", "/prefixxx"}}
	}
}

func (suite *IngressMatchSuite) Test_Http_MatchPath() {
	suite.tmplData.Apps = make([]int, len(ingressRules))
	for i := 0; i < len(ingressRules); i++ {
		suite.tmplData.Apps[i] = i + 1
	}
	suite.tmplData.Rules = ingressRules
	suite.Require().NoError(suite.test.DeployYamlTemplate("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
	suite.Require().NoError(suite.test.DeployYamlTemplate("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))

	for _, test := range tests {
		for _, path := range test.paths {
			suite.Run("test="+test.host+path, func() {
				suite.Eventually(func() bool {
					suite.client.Host = test.host
					suite.client.Path = path
					res, cls, err := suite.client.Do()
					if res == nil {
						suite.T().Log(err)
						return false
					}
					defer cls()
					body, err := ioutil.ReadAll(res.Body)
					if err != nil {
						return false
					}
					type echoServerResponse struct {
						OS struct {
							Hostname string `json:"hostname"`
						} `json:"os"`
					}
					response := &echoServerResponse{}
					err = json.Unmarshal(body, response)
					if err != nil {
						return false
					}
					actual := response.OS.Hostname
					pass := strings.HasPrefix(actual, test.target)
					if !pass {
						suite.T().Logf("Expected %s but got %s", test.target, actual)
					}
					return pass
				}, e2e.WaitDuration, e2e.TickDuration)
			})
		}
	}
}
