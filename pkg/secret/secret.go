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

package secret

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

type Manager struct {
	store   store.K8s
	haproxy haproxy.HAProxy
}

type Secret struct {
	Name       types.NamespacedName
	OwnerType  OwnerType
	OwnerName  string
	SecretType certs.SecretType
}

// module logger
var logger = utils.GetLogger()

type OwnerType string

//nolint:golint,stylecheck
const (
	OWNERTYPE_INGRESS OwnerType = "Ingress"
	OWNERTYPE_TCP_CR  OwnerType = "TCP_CR"
)

func NewManager(store store.K8s, h haproxy.HAProxy) *Manager {
	return &Manager{
		store:   store,
		haproxy: h,
	}
}

func (s Manager) Store(sec Secret) {
	if _, ok := s.store.SecretsProcessed[sec.Name.String()]; ok {
		return
	}
	secret, secErr := s.store.GetSecret(sec.Name.Namespace, sec.Name.Name)
	if secErr != nil {
		logger.Warningf("%s '%s/%s': %s", sec.OwnerType, sec.Name.Namespace, sec.OwnerName, secErr)
		return
	}
	s.store.SecretsProcessed[sec.Name.String()] = struct{}{}
	_, err := s.haproxy.AddSecret(secret, sec.SecretType)
	logger.Error(err)
}
