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

package main

import (
	"fmt"
	"log"

	parser "github.com/haproxytech/config-parser"
	"github.com/haproxytech/config-parser/types"
	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleDefaultTimeouts() bool {
	hasChanges := false
	hasChanges = c.handleDefaultTimeout("http-request", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("connect", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("client", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("queue", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("server", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("tunnel", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("http-keep-alive", true) || hasChanges
	//no default values
	//timeout check is put in every backend, no need to put it here
	//hasChanges = c.handleDefaultTimeout("check", false) || hasChanges
	if hasChanges {
		err := c.NativeParser.Save(HAProxyGlobalCFG)
		LogErr(err)
	}
	return hasChanges
}

func (c *HAProxyController) handleDefaultTimeout(timeout string, hasDefault bool) bool {
	client := c.NativeParser
	annTimeout, err := GetValueFromAnnotations(fmt.Sprintf("timeout-%s", timeout), c.cfg.ConfigMap.Annotations)
	if err != nil {
		if hasDefault {
			log.Println(err)
		}
		return false
	}
	if annTimeout.Status != "" {
		//log.Println(fmt.Sprintf("timeout [%s]", timeout), annTimeout.Value, annTimeout.OldValue, annTimeout.Status)
		data, err := client.Get(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout))
		if err != nil {
			if hasDefault {
				log.Println(err)
				return false
			}
			errSet := client.Set(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout), types.SimpleTimeout{
				Value: annTimeout.Value,
			})
			if errSet != nil {
				log.Println(errSet)
			}
			return true
		}
		timeout := data.(*types.SimpleTimeout)
		timeout.Value = annTimeout.Value
		return true
	}
	return false
}

// Update backend with annotations values.
// Deleted annotations should be handled explicitly when there is no defualt value.
func (c *HAProxyController) handleBackendAnnotations(ingress *Ingress, service *Service, backendName string, newBackend bool) (needReload bool, err error) {
	needReload = false
	model, _ := c.backendGet(backendName)
	backend := backend(model)
	backendAnnotations := make(map[string]*StringW, 3)
	backendAnnotations["annBalanceAlg"], _ = GetValueFromAnnotations("load-balance", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["annForwardedFor"], _ = GetValueFromAnnotations("forwarded-for", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["annTimeoutCheck"], _ = GetValueFromAnnotations("timeout-check", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)

	for k, v := range backendAnnotations {
		if v == nil {
			continue
		}
		if v.Status != EMPTY || newBackend {
			switch k {
			case "annBalanceAlg":
				if err := backend.updateBalance(v); err != nil {
					LogErr(err)
					continue
				}
				needReload = true
			case "annTimeoutCheck":
				if v.Status == DELETED && !newBackend {
					backend.CheckTimeout = nil
				} else if err := backend.updateCheckTimeout(v); err != nil {
					LogErr(err)
					continue
				}
				needReload = true
			case "annForwardedFor":
				if err := backend.updateForwardFor(v); err != nil {
					LogErr(err)
					continue
				}
				needReload = true
			}
		}
	}

	if needReload {
		if err := c.backendEdit(models.Backend(backend)); err != nil {
			return needReload, err
		}
	}
	return needReload, nil
}
