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
	"io"
	"os/exec"
	"time"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *NamespaceSelectorSuite) setupNamespace(name string, labels string) {
	cmd := exec.Command("kubectl", "create", "ns", name)
	_ = cmd.Run()
	if labels != "" {
		cmd = exec.Command("kubectl", "label", "ns", name, labels)
		_ = cmd.Run()
	}
}

func (suite *NamespaceSelectorSuite) deleteNamespace(name string) {
	cmd := exec.Command("kubectl", "delete", "ns", name)
	_ = cmd.Run()
}

func (suite *NamespaceSelectorSuite) Test_NamespaceSelector() {
	ignoredNS := suite.test.GetNS() + "-ignored"
	matchedNS := suite.test.GetNS() + "-matched"

	suite.setupNamespace(ignoredNS, "")
	suite.setupNamespace(matchedNS, "watch=true")

	defer suite.deleteNamespace(ignoredNS)
	defer suite.deleteNamespace(matchedNS)

	// Deploy apps to both namespaces
	tmplIgnored := tmplData{
		Host:        ignoredNS + ".test",
		Namespace:   ignoredNS,
		IngressName: "ingress-ignored",
		ServiceName: "service-ignored",
	}
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", ignoredNS, tmplIgnored))
	suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", ignoredNS, tmplIgnored))

	tmplMatched := tmplData{
		Host:        matchedNS + ".test",
		Namespace:   matchedNS,
		IngressName: "ingress-matched",
		ServiceName: "service-matched",
	}
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", matchedNS, tmplMatched))
	suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", matchedNS, tmplMatched))

	// Wait for deployments to be ready
	cmd := exec.Command("kubectl", "wait", "--for=condition=available", "deployment", tmplIgnored.ServiceName, "-n", ignoredNS, "--timeout=60s")
	suite.Require().NoError(cmd.Run())
	cmd = exec.Command("kubectl", "wait", "--for=condition=available", "deployment", tmplMatched.ServiceName, "-n", matchedNS, "--timeout=60s")
	suite.Require().NoError(cmd.Run())

	// Give HAProxy time to sync initial state
	time.Sleep(5 * time.Second)

	// Test 1: Ignored NS should give 404
	clientIgnored, err := e2e.NewHTTPClient(tmplIgnored.Host)
	suite.Require().NoError(err)
	res, closeFunc, err := clientIgnored.Do()
	suite.Require().NoError(err)
	defer closeFunc()
	suite.Require().Equal(404, res.StatusCode, "Ignored namespace should return 404")

	// Test 2: Matched NS should give 200
	clientMatched, err := e2e.NewHTTPClient(tmplMatched.Host)
	suite.Require().NoError(err)
	res, closeFunc, err = clientMatched.Do()
	suite.Require().NoError(err)
	defer closeFunc()
	suite.Require().Equal(200, res.StatusCode, "Matched namespace should return 200")
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	suite.Require().Contains(string(body), tmplMatched.ServiceName)

	// Test 3: Dynamic Addition
	cmd = exec.Command("kubectl", "label", "ns", ignoredNS, "watch=true")
	suite.Require().NoError(cmd.Run())
	
	// Wait for controller to process the namespace label addition and sync
	suite.Eventually(func() bool {
		res, closeFunc, err := clientIgnored.Do()
		if err != nil {
			return false
		}
		defer closeFunc()
		return res.StatusCode == 200
	}, 15*time.Second, 1*time.Second, "Dynamically added namespace should eventually return 200")

	// Test 4: Dynamic Removal
	cmd = exec.Command("kubectl", "label", "ns", matchedNS, "watch-")
	suite.Require().NoError(cmd.Run())
	
	// Wait for controller to process the namespace label removal and sync
	suite.Eventually(func() bool {
		res, closeFunc, err := clientMatched.Do()
		if err != nil {
			return false
		}
		defer closeFunc()
		return res.StatusCode == 404
	}, 15*time.Second, 1*time.Second, "Dynamically removed namespace should eventually return 404")
}

func (suite *NamespaceSelectorSuite) Test_NamespaceSelector_Secret() {
	tlsNS := suite.test.GetNS() + "-tls"

	suite.setupNamespace(tlsNS, "")
	defer suite.deleteNamespace(tlsNS)

	tmplTLS := tmplData{
		Host:        "offload-test.haproxy", // Must match the hardcoded Secret CN
		Namespace:   tlsNS,
		IngressName: "ingress-tls",
		ServiceName: "service-tls",
		TLSEnabled:  true,
	}
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", tlsNS, tmplTLS))
	suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", tlsNS, tmplTLS))

	// Wait for deployments to be ready
	cmd := exec.Command("kubectl", "wait", "--for=condition=available", "deployment", tmplTLS.ServiceName, "-n", tlsNS, "--timeout=60s")
	suite.Require().NoError(cmd.Run())

	// Give HAProxy time to sync initial state
	time.Sleep(5 * time.Second)

	// Since the namespace doesn't have the watch label, HAProxy shouldn't serve this route correctly
	// The client might connect to the default backend or get 404, and the certificate might be the default fallback cert.
	
	// Dynamically add the label
	cmd = exec.Command("kubectl", "label", "ns", tlsNS, "watch=true")
	suite.Require().NoError(cmd.Run())

	clientTLS, err := e2e.NewHTTPSClient(tmplTLS.Host, 0)
	suite.Require().NoError(err)

	// Wait for controller to process the namespace label addition and sync Ingress + Secret
	suite.Eventually(func() bool {
		res, closeFunc, err := clientTLS.Do()
		if err != nil {
			return false
		}
		defer closeFunc()
		
		if res.StatusCode != 200 {
			return false
		}
		if len(res.TLS.PeerCertificates) == 0 {
			return false
		}
		// Check that the returned certificate matches our specific secret's CN
		return res.TLS.PeerCertificates[0].Subject.CommonName == "offload-test.haproxy"
	}, 15*time.Second, 1*time.Second, "Dynamically added TLS namespace should eventually serve the correct certificate")
}
