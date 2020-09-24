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

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type DefaultCertificate struct{}

func (d DefaultCertificate) Update(k store.K8s, cfg Configuration, api api.HAProxyClient) (reload bool, err error) {
	secretAnn, defSecretErr := k.GetValueFromAnnotations("ssl-certificate", k.ConfigMaps[Main].Annotations)
	writeSecret := false
	if defSecretErr == nil {
		if secretAnn.Status != DELETED && secretAnn.Status != EMPTY {
			writeSecret = true
		}
		secretData := strings.Split(secretAnn.Value, "/")
		namespace, namespaceOK := k.Namespaces[secretData[0]]
		if len(secretData) == 2 && namespaceOK {
			secret, ok := namespace.Secret[secretData[1]]
			if ok {
				if secret.Status != EMPTY && secret.Status != DELETED {
					writeSecret = true
				}
				reload, err = HandleSecret(store.Ingress{
					Name: "0",
				}, *secret, writeSecret, cfg.UsedCerts, logger)
				return reload, err
			}
		}
	}
	return false, nil
}
