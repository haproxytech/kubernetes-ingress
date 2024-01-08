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

package e2e

import "strings"

func (t Test) GetIngressControllerFile(path string) (string, error) {
	po, err := t.getIngressControllerPod()
	if err != nil {
		return "", err
	}
	out, errExec := t.execute("", "kubectl", "exec", "-i", "-n", "haproxy-controller",
		po, "--", "cat", path)

	return out, errExec
}

func (t Test) getIngressControllerPod() (string, error) {
	out, errExec := t.execute("", "kubectl", "get", "pods", "-n", "haproxy-controller",
		"-l", "run=haproxy-ingress", "-o", "name", "--field-selector=status.phase==Running", "-l", "run=haproxy-ingress")
	return strings.TrimSuffix(out, "\n"), errExec
}
