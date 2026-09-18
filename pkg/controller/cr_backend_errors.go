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

package controller

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
)

// disableRejectedBackendCRs disables the Backend custom resources behind the
// backends HAProxy rejected. dir holds the rejected configuration file.
// Returns whether a resource was disabled, so the sync has to be replayed.
func (c *HAProxyController) disableRejectedBackendCRs(configErr error, dir string) (rerun bool) {
	backends, err := annotations.BackendsInError(configErr, dir)
	logger.Error(err)
	for _, backend := range backends {
		crKey, fromCR := c.store.BackendsFromCR[backend]
		if !fromCR {
			continue
		}
		namespace, name, _ := strings.Cut(crKey, "/")
		generation, ok := c.store.DisableBackendCR(namespace, name)
		if !ok {
			continue
		}
		logger.Errorf("backend custom resource '%s' generation %d rejected by HAProxy in backend '%s': disabled until edited",
			crKey, generation, backend)
		rerun = true
	}
	return rerun
}
