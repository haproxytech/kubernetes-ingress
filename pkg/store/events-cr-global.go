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

package store

import (
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
)

func (k *K8s) EventGlobalCR(namespace, name string, data *v1.Global) bool {
	ns := k.GetNamespace(namespace)
	if data == nil {
		delete(ns.CRs.Global, name)
		delete(ns.CRs.LogTargets, name)
		return true
	}
	ns.CRs.Global[name] = data.Spec.Config
	ns.CRs.LogTargets[name] = data.Spec.LogTargets
	return true
}
