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
	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
)

// Handle Global and default Annotations
func (c *HAProxyController) handleGlobalAnnotations() (restart bool, reload bool) {
	var r annotations.Result
	for name, annotation := range annotations.Global {
		cfgMapAnn, _ := c.Store.GetValueFromAnnotations(name, c.Store.ConfigMaps[Main].Annotations)
		if cfgMapAnn == nil {
			continue
		}
		switch cfgMapAnn.Status {
		case EMPTY:
			continue
		case DELETED:
			r = annotation.Delete(c.Client)
		default:
			if err := annotation.Parse(cfgMapAnn.Value); err != nil {
				logger.Error(err)
			} else {
				r = annotation.Update(c.Client)
			}
		}
		if r == 1 {
			reload = true
		} else if r == 2 {
			restart = true
		}
	}
	return restart, reload
}
