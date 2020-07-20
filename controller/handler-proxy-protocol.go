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
	"fmt"
	"net"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type ProxyProtocol struct{}

func (p ProxyProtocol) Update(cfg Configuration, api api.HAProxyClient, logger utils.Logger) (reload bool, err error) {
	//  Get and validate annotations
	annProxyProtocol, _ := GetValueFromAnnotations("proxy-protocol", cfg.ConfigMap.Annotations)
	if annProxyProtocol == nil {
		return false, nil
	}
	value := strings.Replace(annProxyProtocol.Value, ",", " ", -1)
	for _, address := range strings.Fields(value) {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return false, fmt.Errorf("incorrect value for proxy-protocol annotation ")
			}
		}
	}

	// Get Rules status
	status := annProxyProtocol.Status

	// Update rules
	// Since this is a Configmap Annotation ONLY, no need to
	// track ingress hosts in Map file
	if status != EMPTY {
		cfg.FrontendRulesStatus[TCP] = MODIFIED
		if status == DELETED {
			logger.Debugf("Deleting ProxyProtcol configuration")
			return false, nil
		}
		logger.Debugf("Configuring ProxyProtcol annotation")
	}

	tcpRule := models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Type:     "connection",
		Action:   "expect-proxy layer4",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ src %s }", value),
	}
	cfg.FrontendTCPRules[PROXY_PROTOCOL][0] = tcpRule

	return false, nil
}
