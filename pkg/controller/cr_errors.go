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
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
)

// disableRejectedCRs disables the custom resources behind the sections HAProxy
// rejected. dir holds the rejected configuration file.
// Returns whether a resource was disabled, so the sync has to be replayed.
func (c *HAProxyController) disableRejectedCRs(configErr error, dir string) (rerun bool) {
	sections, err := annotations.SectionsInError(configErr, dir)
	logger.Error(err)
	for _, section := range sections {
		ref, fromCR := c.store.SectionsFromCR[section.Key()]
		if !fromCR {
			continue
		}
		generation, ok := c.store.DisableCR(ref)
		if !ok {
			continue
		}
		logger.Errorf("%s generation %d rejected by HAProxy in section '%s': disabled until edited",
			ref, generation, section.Key())
		rerun = true
	}
	return rerun
}
