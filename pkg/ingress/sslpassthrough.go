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

package ingress

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// SSLPassthroughEnabled reports whether the traffic of an ingress path is served in
// ssl-passthrough mode, that is at layer 4, with TLS terminated by the pod rather than by
// HAProxy.
//
// The annotation is resolved per path and not per ingress, because it describes the
// service: "the pod terminates TLS itself" is a property of what listens behind the port,
// not of the route which reaches it. The service is therefore consulted first, then the
// ingress carrying the path, then the configmap default. That precedence is the one
// service.New already applies to every other backend annotation, so a service value
// overrides an ingress one here as it does there.
//
// Two consequences are worth knowing. Resolving per path lets a single ingress route one
// path to a service which terminates TLS itself and another to a service which does not,
// which was not expressible while the mode was decided once per ingress. And a value set
// on the service settles the mode for every ingress referencing it, since they all resolve
// it from the same place - a backend has a single mode, so two ingresses sharing one must
// agree, and the service is where that agreement can be expressed.
//
// The returned error only reports an unparsable value. The bool is then false, the
// documented default, so a caller may log the error and carry on.
func SSLPassthroughEnabled(k store.K8s, path *store.IngressPath, ingressAnnotations map[string]string) (bool, error) {
	scopes := make([]map[string]string, 0, 3)
	// A path pointing at a service which does not exist is reported by service.New; here
	// it simply contributes no annotation.
	if svc, err := k.GetService(path.SvcNamespace, path.SvcName); err == nil {
		scopes = append(scopes, svc.Annotations)
	}
	scopes = append(scopes, ingressAnnotations, k.ConfigMaps.Main.Annotations)
	return annotations.Bool("ssl-passthrough", scopes...)
}
