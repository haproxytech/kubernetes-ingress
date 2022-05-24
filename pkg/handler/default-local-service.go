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

package handler

import (
	"strconv"

	"github.com/haproxytech/client-native/v3/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type DefaultLocalService struct {
	Name string
	Port int
}

func (handler DefaultLocalService) Update(k store.K8s, api haproxy.HAProxy, ann annotations.Annotations) (reload bool, err error) {
	_, err = api.ServerGet("ingress_controller", handler.Name)
	if err != nil {
		/*err = api.BackendCreate(models.Backend{
			Name: DefaultLocalBackend,
			Mode: "http",
		})
		if err != nil {
			return false, err
		}*/
		err = api.BackendServerCreate(handler.Name, models.Server{
			Name:    "ingress_controller",
			Address: "127.0.0.1:" + strconv.Itoa(handler.Port),
		})
		if err != nil {
			return false, err
		}
		logger.Debug("DefaultLocalService backend created")
	}
	return false, nil
}
