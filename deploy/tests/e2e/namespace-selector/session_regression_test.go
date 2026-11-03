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

//go:build e2e_sequential

package namespaceselector

import (
	"encoding/json"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

// These tests exercise real API/watch traffic, not a prescribed interleaving
// between READY, shutdown, bootstrap snapshots and CRD registration. The narrow
// races need deterministic unit tests as well.
func (suite *NamespaceSelectorSuite) Test_RapidSelectionChurnDoesNotRestartController() {
	suite.cleanupSelection()
	suite.Require().NoError(labelNS(suite.foreignNS, "app=watch"))
	suite.waitStatus(suite.foreignClient, 200)
	suite.waitStatus(suite.client, 404)
	before := suite.waitControllerSnapshot()

	for round := 0; round < 3; round++ {
		suite.churnSelection(4)
		suite.waitStatus(suite.client, 200)
		suite.waitStatus(suite.foreignClient, 200)
		suite.Require().NoError(clearWatchLabel(suite.test.GetNS()))
		suite.waitStatus(suite.client, 404)
		suite.waitStatus(suite.foreignClient, 200)
		suite.Equal(before, suite.waitControllerSnapshot(), "selection churn must not replace or restart the controller")
	}
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.Equal(before, suite.waitControllerSnapshot())
}

func (suite *NamespaceSelectorSuite) Test_BootstrapMembershipChurnRelabelRecovers() {
	suite.cleanupSelection()
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.Require().NoError(labelNS(suite.foreignNS, "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.waitStatus(suite.foreignClient, 200)
	before := suite.waitControllerSnapshot()

	// Do not wait for rollout before changing membership. API scheduling may
	// still coalesce these events; this is bootstrap stress, not a race proof.
	out, err := kubectl("-n", controllerNS, "rollout", "restart", "deploy/"+controllerName)
	suite.Require().NoError(err, out)
	suite.churnSelection(12)
	suite.Require().NoError(clearWatchLabel(suite.test.GetNS()))
	suite.Require().NoError(waitControllerRollout())
	suite.waitStatus(suite.client, 404)
	suite.waitStatus(suite.foreignClient, 200)
	after := suite.waitControllerSnapshot()
	suite.NotEqual(before.UID, after.UID, "bootstrap must run in a new pod")
	suite.Zero(after.Restarts, "the intentionally replaced controller must not crash during bootstrap")

	// Relabel without touching the Ingress: a stale bootstrap session must not
	// suppress a fresh LIST/READY and strand this namespace as irrelevant.
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(clearWatchLabel(suite.foreignNS))
	suite.waitStatus(suite.foreignClient, 404)
	suite.waitStatus(suite.client, 200)
	suite.Require().NoError(labelNS(suite.foreignNS, "app=watch"))
	suite.waitStatus(suite.foreignClient, 200)
	suite.Equal(after, suite.waitControllerSnapshot())
}

func (suite *NamespaceSelectorSuite) Test_LateTCPCRDWithSelectionChurn() {
	if os.Getenv("NAMESPACE_SELECTOR_ISOLATED_CRD_TEST") != "1" {
		suite.T().Skip("requires explicit NAMESPACE_SELECTOR_ISOLATED_CRD_TEST=1 on a disposable, exclusively owned cluster")
	}
	const crd = "tcps.ingress.v1.haproxy.org"
	out, err := kubectl("get", "crd", crd, "--ignore-not-found", "-o", "name")
	suite.Require().NoError(err, out)
	if out != "" {
		suite.T().Skip("late CRD test requires v1 TCP CRD absent; never deletes a pre-existing CRD")
	}
	suite.cleanupSelection()
	suite.Require().NoError(labelNS(suite.foreignNS, "app=watch"))
	suite.waitStatus(suite.foreignClient, 200)
	before := suite.waitControllerSnapshot()

	// create (not apply) prevents taking ownership of an existing definition.
	out, err = kubectl("create", "-f", "../../../../crs/definition/ingress.v1.haproxy.org_tcps.yaml")
	suite.Require().NoError(err, out)
	suite.T().Cleanup(func() {
		// Only this test owns this definition and its instances. Delete the
		// objects first so a leftover TCP frontend cannot poison later HTTP
		// tests. Restart after CRD removal because CRD deletion is not monitored.
		for _, ns := range []string{suite.test.GetNS(), suite.foreignNS} {
			out, err := kubectl("-n", ns, "delete", "tcp.ingress.v1.haproxy.org", "namespace-selector-late-tcp", "--ignore-not-found=true")
			suite.NoError(err, out)
		}
		out, err := kubectl("delete", "crd", crd, "--ignore-not-found", "--timeout=120s")
		suite.NoError(err, out)
		suite.NoError(restartController())
		suite.waitNoTCPFrontends()
	})
	out, err = kubectl("wait", "--for=condition=Established", "crd/"+crd, "--timeout=60s")
	suite.Require().NoError(err, out)

	for _, fixture := range []struct {
		Namespace string
		Port      int
	}{
		{suite.test.GetNS(), 32766},
		{suite.foreignNS, 32767},
	} {
		suite.Require().NoError(suite.test.Apply("config/late-tcp.yaml.tmpl", fixture.Namespace, fixture))
	}
	client, err := e2e.NewHTTPClient(suite.host, 32766)
	suite.Require().NoError(err)
	foreignClient, err := e2e.NewHTTPClient(suite.foreignHost, 32767)
	suite.Require().NoError(err)
	// Keep churning while the existing, healthy session discovers the new
	// CRD. Do not rely on a sleep matching the controller's discovery delay.
	suite.Require().Eventually(func() bool {
		for _, label := range []string{"app=watch", "app-", "app=watch"} {
			if err := labelNS(suite.test.GetNS(), label); err != nil {
				suite.T().Log(err)
				return false
			}
		}
		response, closeResponse, err := foreignClient.Do()
		if closeResponse != nil {
			defer closeResponse()
		}
		return err == nil && response.StatusCode == 200
	}, e2e.WaitDuration, e2e.TickDuration)
	suite.waitStatus(client, 200)
	suite.waitStatus(foreignClient, 200)

	suite.Require().NoError(clearWatchLabel(suite.test.GetNS()))
	suite.waitUnavailable(client)
	suite.waitStatus(foreignClient, 200)
	suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	suite.waitStatus(client, 200)
	suite.waitStatus(foreignClient, 200)
	suite.Equal(before, suite.waitControllerSnapshot(), "late CRD and selection churn must not restart the controller")
}

func (suite *NamespaceSelectorSuite) cleanupSelection() {
	suite.T().Cleanup(func() {
		suite.NoError(clearWatchLabel(suite.test.GetNS()))
		suite.NoError(clearWatchLabel(suite.foreignNS))
	})
}

func (suite *NamespaceSelectorSuite) churnSelection(cycles int) {
	for i := 0; i < cycles; i++ {
		suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
		suite.Require().NoError(clearWatchLabel(suite.test.GetNS()))
		suite.Require().NoError(labelNS(suite.test.GetNS(), "app=watch"))
	}
}

type controllerSnapshot struct {
	UID      string
	Restarts int32
}

func (suite *NamespaceSelectorSuite) waitControllerSnapshot() controllerSnapshot {
	var snapshot controllerSnapshot
	suite.Require().Eventually(func() bool {
		var err error
		snapshot, err = readControllerSnapshot()
		if err != nil {
			suite.T().Log(err)
		}
		return err == nil
	}, e2e.WaitDuration, e2e.TickDuration)
	return snapshot
}

func readControllerSnapshot() (controllerSnapshot, error) {
	out, err := kubectl("-n", controllerNS, "get", "pods", "-l", "run=haproxy-ingress", "-o", "json")
	if err != nil {
		return controllerSnapshot{}, fmt.Errorf("get controller pods: %w: %s", err, out)
	}
	var pods corev1.PodList
	if err := json.Unmarshal([]byte(out), &pods); err != nil {
		return controllerSnapshot{}, err
	}
	// This suite uses the single-replica deployment in deploy/tests/config.
	// Requiring one pod also avoids hiding a rolling replacement in the sample.
	if len(pods.Items) != 1 {
		return controllerSnapshot{}, fmt.Errorf("expected one controller pod, got %d", len(pods.Items))
	}
	pod := pods.Items[0]
	if pod.DeletionTimestamp != nil {
		return controllerSnapshot{}, fmt.Errorf("controller pod %s is terminating", pod.Name)
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == "haproxy-ingress" && status.Ready && status.State.Running != nil {
			return controllerSnapshot{UID: string(pod.UID), Restarts: status.RestartCount}, nil
		}
	}
	return controllerSnapshot{}, fmt.Errorf("controller pod %s is not ready", pod.Name)
}
