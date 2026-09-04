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
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

const (
	controllerNS   = "haproxy-controller"
	controllerName = "haproxy-kubernetes-ingress"
	selectorFlag   = "--namespace-label-selector=app=watch"
)

type NamespaceSelectorSuite struct {
	suite.Suite
	test          e2e.Test
	client        *e2e.Client
	foreignClient *e2e.Client
	host          string
	foreignHost   string
	foreignNS     string
	originalArgs  []string
}

func TestNamespaceSelectorSuite(t *testing.T) {
	suite.Run(t, new(NamespaceSelectorSuite))
}

func (suite *NamespaceSelectorSuite) SetupSuite() {
	var err error
	suite.test, err = e2e.NewTest()
	suite.Require().NoError(err)

	suite.originalArgs, err = controllerArgs()
	suite.Require().NoError(err)
	suite.T().Cleanup(func() {
		if err := restoreControllerArgs(suite.originalArgs); err != nil {
			suite.T().Errorf("failed to restore controller args: %v", err)
		}
	})

	// Default backend and controller ConfigMaps live in the controller
	// namespace. Selector mode 503s unmatched hosts unless that namespace is
	// selected.
	suite.Require().NoError(labelNS(controllerNS, "app=watch"))
	suite.T().Cleanup(func() {
		if err := clearWatchLabel(controllerNS); err != nil {
			suite.T().Errorf("failed to clear controller namespace label: %v", err)
		}
	})

	suite.Require().NoError(setControllerSelector())
	suite.Require().NoError(waitControllerRollout())

	suite.host = suite.test.GetNS() + ".watch.test"
	suite.client, err = e2e.NewHTTPClient(suite.host)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml", suite.test.GetNS(), nil))
	suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", suite.test.GetNS(), struct{ Host string }{Host: suite.host}))

	suite.foreignNS = suite.test.GetNS() + "-foreign"
	out, err := kubectl("create", "ns", suite.foreignNS)
	suite.Require().NoError(err, out)
	suite.test.AddTearDown(func() error {
		_, delErr := kubectl("delete", "ns", suite.foreignNS, "--wait=false")
		return delErr
	})
	suite.foreignHost = suite.foreignNS + ".watch.test"
	suite.foreignClient, err = e2e.NewHTTPClient(suite.foreignHost)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.test.Apply("config/deploy.yaml", suite.foreignNS, nil))
	suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", suite.foreignNS, struct{ Host string }{Host: suite.foreignHost}))
}

func (suite *NamespaceSelectorSuite) SetupTest() {
	suite.Require().NoError(clearWatchLabel(suite.test.GetNS()))
	suite.Require().NoError(clearWatchLabel(suite.foreignNS))
}

func (suite *NamespaceSelectorSuite) TearDownSuite() {
	suite.Require().NoError(suite.test.TearDown())
}

func (suite *NamespaceSelectorSuite) waitStatus(client *e2e.Client, want int) {
	suite.Require().Eventually(func() bool {
		r, cls, err := client.Do()
		if err != nil {
			suite.T().Log(err)
			return false
		}
		defer cls()
		return r.StatusCode == want
	}, e2e.WaitDuration, e2e.TickDuration)
}

func (suite *NamespaceSelectorSuite) waitUnavailable(client *e2e.Client) {
	suite.Require().Eventually(func() bool {
		_, cls, err := client.Do()
		if err != nil {
			return true
		}
		if cls != nil {
			cls()
		}
		return false
	}, e2e.WaitDuration, e2e.TickDuration)
}

func kubectl(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func controllerArgs() ([]string, error) {
	out, err := kubectl("-n", controllerNS, "get", "deploy", controllerName, "-o", "json")
	if err != nil {
		return nil, err
	}
	var deploy struct {
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Args []string `json:"args"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}
	if err := json.Unmarshal([]byte(out), &deploy); err != nil {
		return nil, err
	}
	if len(deploy.Spec.Template.Spec.Containers) == 0 {
		return nil, errors.New("controller deployment has no containers")
	}
	return deploy.Spec.Template.Spec.Containers[0].Args, nil
}

func restoreControllerArgs(args []string) error {
	raw, err := json.Marshal(args)
	if err != nil {
		return err
	}
	patch := `[{"op":"replace","path":"/spec/template/spec/containers/0/args","value":` + string(raw) + `}]`
	out, err := kubectl("-n", controllerNS, "patch", "deploy", controllerName, "--type=json", "-p", patch)
	if err != nil {
		return err
	}
	_ = out
	return waitControllerRollout()
}

func setControllerSelector() error {
	patch := `[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"` + selectorFlag + `"}]`
	if _, err := kubectl("-n", controllerNS, "patch", "deploy", controllerName, "--type=json", "-p", patch); err != nil {
		return err
	}
	return nil
}

func waitControllerRollout() error {
	_, err := kubectl("-n", controllerNS, "rollout", "status", "deploy/"+controllerName, "--timeout=120s")
	return err
}

func restartController() error {
	if _, err := kubectl("-n", controllerNS, "rollout", "restart", "deploy/"+controllerName); err != nil {
		return err
	}
	return waitControllerRollout()
}

func clearWatchLabel(name string) error {
	out, err := kubectl("label", "ns", name, "app-")
	if err != nil && (strings.Contains(out, "not found") || strings.Contains(out, "not labeled")) {
		return nil
	}
	return err
}

func labelNS(name, label string) error {
	out, err := kubectl("label", "ns", name, label, "--overwrite")
	if err != nil {
		return err
	}
	_ = out
	return nil
}
