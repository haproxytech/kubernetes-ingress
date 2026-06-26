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

package service

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const (
	defaultSourceIPPersistenceSize   = "1m"
	defaultSourceIPPersistenceExpire = "30m"
	localPeerSection                 = "localinstance"
)

type SourceIPPersistence struct {
	backend    *models.Backend
	name       string
	nameSize   string
	nameExpire string
}

func NewSourceIPPersistence(n string, b *models.Backend) *SourceIPPersistence {
	return &SourceIPPersistence{
		name:       n,
		nameSize:   n + "-size",
		nameExpire: n + "-expire",
		backend:    b,
	}
}

func (a *SourceIPPersistence) GetName() string {
	return a.name
}

func (a *SourceIPPersistence) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		a.clear()
		return nil
	}

	enabled, err := utils.GetBoolValue(input, a.GetName())
	if err != nil {
		return fmt.Errorf("%s: %w", a.GetName(), err)
	}
	if !enabled {
		a.clear()
		return nil
	}

	sizeInput := common.GetValue(a.nameSize, annotations...)
	if sizeInput == "" {
		sizeInput = defaultSourceIPPersistenceSize
	}
	size, err := utils.ParseSize(sizeInput)
	if err != nil {
		return fmt.Errorf("%s: %w", a.nameSize, err)
	}

	expireInput := common.GetValue(a.nameExpire, annotations...)
	if expireInput == "" {
		expireInput = defaultSourceIPPersistenceExpire
	}
	expire, err := utils.ParseTime(expireInput)
	if err != nil {
		return fmt.Errorf("%s: %w", a.nameExpire, err)
	}

	a.backend.StickTable = &models.ConfigStickTable{
		Type:   "ip",
		Size:   size,
		Expire: expire,
		Peers:  localPeerSection,
	}
	a.backend.StickRuleList = models.StickRules{
		{
			Type:    "on",
			Pattern: "src",
		},
	}
	return nil
}

func (a *SourceIPPersistence) clear() {
	a.backend.StickTable = nil
	a.backend.StickRuleList = nil
}
