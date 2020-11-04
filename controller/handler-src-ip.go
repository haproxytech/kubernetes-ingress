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
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type SourceIPHeader struct{}

func (p SourceIPHeader) Update(k store.K8s, cfg *Configuration, api api.HAProxyClient) (reload bool, err error) {
	//  Get annotation status
	srcIPHeader, _ := k.GetValueFromAnnotations("src-ip-header", k.ConfigMaps[Main].Annotations)
	if srcIPHeader == nil {
		return false, nil
	}
	if srcIPHeader.Status == DELETED || len(srcIPHeader.Value) == 0 {
		logger.Debugf("Deleting Source IP configuration")
		return false, nil
	}
	id, _ := haproxy.NewMapID(srcIPHeader.Value)
	return true, cfg.HAProxyRules.AddRule(rules.ReqSetSrc{
		HeaderName: srcIPHeader.Value,
	}, id, FrontendHTTP, FrontendHTTPS)
}
