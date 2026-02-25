// Copyright 2026 HAProxy Technologies LLC
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

package differentports

import (
	"io"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *DifferentPortsSuite) Test_Different_Ports() {
	suite.Run("different port added without scaling", func() {
		suite.tmplData.Instances = append(suite.tmplData.Instances, instanceTmplData{Port: 8001, Replicas: 1})
		suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Eventually(suite.everyReplicaReachable, e2e.WaitDuration, e2e.TickDuration)
	})

	suite.Run("different port added with scaling", func() {
		suite.tmplData.Instances = append(suite.tmplData.Instances, instanceTmplData{Port: 8002, Replicas: 8})
		suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Eventually(suite.everyReplicaReachable, e2e.WaitDuration, e2e.TickDuration)
	})

	suite.Run("different port added while previous port removed", func() {
		suite.tmplData.Instances = append(suite.tmplData.Instances, instanceTmplData{Port: 8003, Replicas: 1})
		suite.tmplData.Instances[len(suite.tmplData.Instances)-2].Replicas = 0
		suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Eventually(suite.everyReplicaReachable, e2e.WaitDuration, e2e.TickDuration)
	})

	suite.Run("standalone different ports", func() {
		suite.tmplData.Instances = append(suite.tmplData.Instances, instanceTmplData{Port: 8004, Replicas: 1})
		suite.tmplData.IngAnnotations = []struct{ Key, Value string }{{Key: "haproxy.org/standalone-backend", Value: "true"}}
		suite.Require().NoError(suite.test.Apply("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().NoError(suite.test.Apply("config/ingress.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Eventually(suite.everyReplicaReachable, e2e.WaitDuration, e2e.TickDuration)
	})
}

func (suite *DifferentPortsSuite) everyReplicaReachable() bool {
	totalReplicas := 0
	counter := map[int]map[string]int{}
	for instanceIndex, instance := range suite.tmplData.Instances {
		totalReplicas += instance.Replicas
		counter[instanceIndex] = make(map[string]int)
	}

	for i := 0; i < 2*totalReplicas; i++ {
		func() {
			res, cls, err := suite.client.Do()
			if err != nil {
				suite.T().Log(err.Error())
			}
			defer cls()
			if res.StatusCode == 200 {
				body, err := io.ReadAll(res.Body)
				if err != nil {
					suite.T().Log(err.Error())
					return
				}

				pod := strings.TrimSpace(string(body))
				instanceIndex, err := strconv.Atoi(strings.Split(pod, "-")[2]) // http-echo-<index>-<hash>-<random>
				if err != nil {
					suite.T().Log(err.Error())
					return
				}
				counter[instanceIndex][pod]++
			}
		}()
	}

	for instanceIndex, instance := range suite.tmplData.Instances {
		if len(counter[instanceIndex]) != instance.Replicas {
			return false
		}
		for _, v := range counter[instanceIndex] {
			if v != 2 {
				return false
			}
		}
	}
	return true
}
