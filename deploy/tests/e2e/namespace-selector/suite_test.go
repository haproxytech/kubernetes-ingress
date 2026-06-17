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
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

type NamespaceSelectorSuite struct {
	suite.Suite
	test     e2e.Test
	client   *e2e.Client
	tmplData tmplData
}

type tmplData struct {
	Host        string
	Namespace   string
	IngressName string
	ServiceName string
	TLSEnabled  bool
}

func (suite *NamespaceSelectorSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.Require().NoError(err)
	suite.client, err = e2e.NewHTTPClient(suite.test.GetNS() + ".test")
	suite.Require().NoError(err)

	// Patch deployment to add --namespace-label-selector=watch=true
	patch := `[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--namespace-label-selector=watch=true"}]`
	cmd := exec.Command("kubectl", "patch", "deployment", "haproxy-kubernetes-ingress", "-n", "haproxy-controller", "--type=json", "-p", patch)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	suite.Require().NoError(err, "Failed to patch deployment: %s", out.String())

	// Wait for rollout
	cmd = exec.Command("kubectl", "rollout", "status", "deployment", "haproxy-kubernetes-ingress", "-n", "haproxy-controller", "--timeout=120s")
	err = cmd.Run()
	suite.Require().NoError(err, "Failed to wait for deployment rollout")
	
	// Give controller some time to sync after restart
	time.Sleep(5 * time.Second)
}

func (suite *NamespaceSelectorSuite) TearDownSuite() {
	// Revert the deployment by removing the --namespace-label-selector arg
	// instead of hardcoding the entire args array.
	cmd := exec.Command("kubectl", "get", "deployment", "haproxy-kubernetes-ingress", "-n", "haproxy-controller",
		"-o", "jsonpath={.spec.template.spec.containers[0].args}")
	out, err := cmd.Output()
	if err == nil {
		var args []string
		if json.Unmarshal(out, &args) == nil {
			idx := -1
			for i, arg := range args {
				if strings.HasPrefix(arg, "--namespace-label-selector") {
					idx = i
					break
				}
			}
			if idx >= 0 {
				patch := fmt.Sprintf(`[{"op": "remove", "path": "/spec/template/spec/containers/0/args/%d"}]`, idx)
				cmd = exec.Command("kubectl", "patch", "deployment", "haproxy-kubernetes-ingress", "-n", "haproxy-controller",
					"--type=json", "-p", patch)
				var patchOut bytes.Buffer
				cmd.Stdout = &patchOut
				cmd.Stderr = &patchOut
				if err := cmd.Run(); err != nil {
					suite.T().Logf("Failed to revert deployment args: %v\n%s", err, patchOut.String())
				}
			}
		}
	}

	cmd = exec.Command("kubectl", "rollout", "status", "deployment", "haproxy-kubernetes-ingress", "-n", "haproxy-controller", "--timeout=120s")
	_ = cmd.Run()

	suite.test.TearDown()
}

func TestNamespaceSelectorSuite(t *testing.T) {
	suite.Run(t, new(NamespaceSelectorSuite))
}
