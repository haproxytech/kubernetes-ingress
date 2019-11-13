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
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/misc"
	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/config-parser/v2/types"
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
	return hasChanges
}

func (c *HAProxyController) handleDefaultTimeout(timeout string, hasDefault bool) bool {
	config, _ := c.ActiveConfiguration()
	annTimeout, err := GetValueFromAnnotations(fmt.Sprintf("timeout-%s", timeout), c.cfg.ConfigMap.Annotations)
	if err != nil {
		if hasDefault {
			log.Println(err)
		}
		return false
	}
	if annTimeout.Status != "" {
		//log.Println(fmt.Sprintf("timeout [%s]", timeout), annTimeout.Value, annTimeout.OldValue, annTimeout.Status)
		data, err := config.Get(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout))
		if err != nil {
			if hasDefault {
				log.Println(err)
				return false
			}
			errSet := config.Set(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout), types.SimpleTimeout{
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
	backendAnnotations["annCheckHttp"], _ = GetValueFromAnnotations("check-http", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["annForwardedFor"], _ = GetValueFromAnnotations("forwarded-for", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["annTimeoutCheck"], _ = GetValueFromAnnotations("timeout-check", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["annAbortOnClose"], _ = GetValueFromAnnotations("abortonclose", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)

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
			case "annCheckHttp":
				if v.Status == DELETED && !newBackend {
					backend.Httpchk = nil
				} else if err := backend.updateHttpchk(v); err != nil {
					LogErr(err)
					continue
				}
				needReload = true
			case "annForwardedFor":
				if err := backend.updateForwardfor(v); err != nil {
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
			case "annAbortOnClose":
				if err := backend.updateAbortOnClose(v); err != nil {
					LogErr(err)
					continue
				}
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

// Update server with annotations values.
func (c *HAProxyController) handleServerAnnotations(ingress *Ingress, service *Service, server *models.Server) (annnotationsActive bool) {
	annMaxconn, errMaxConn := GetValueFromAnnotations("pod-maxconn", service.Annotations)
	annCheck, _ := GetValueFromAnnotations("check", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annCheckInterval, errCheckInterval := GetValueFromAnnotations("check-interval", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annnotationsActive = false
	if annMaxconn != nil {
		if annMaxconn.Status != DELETED && errMaxConn == nil {
			if maxconn, err := strconv.ParseInt(annMaxconn.Value, 10, 64); err == nil {
				server.Maxconn = &maxconn
			}
			if annMaxconn.Status != "" {
				annnotationsActive = true
			}
		}
	}
	if annCheck != nil {
		if annCheck.Status != DELETED {
			if annCheck.Value == "enabled" {
				server.Check = "enabled"
				//see if we have port and interval defined
			}
		}
		if annCheck.Status != "" {
			annnotationsActive = true
		}
	}
	if errCheckInterval == nil {
		server.Inter = misc.ParseTimeout(annCheckInterval.Value)
		if annCheckInterval.Status != EMPTY {
			annnotationsActive = true
		}
	} else {
		server.Inter = nil
	}
	return annnotationsActive
}

func (c *HAProxyController) handleRateLimitingAnnotations(ingress *Ingress, service *Service, path *IngressPath) {
	//Annotations with default values don't need error checking.
	annWhitelist, _ := GetValueFromAnnotations("whitelist", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annWhitelistRL, _ := GetValueFromAnnotations("whitelist-with-rate-limit", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	allowRateLimiting := annWhitelistRL.Value != "" && annWhitelistRL.Value != "OFF"
	status := annWhitelist.Status
	if status == EMPTY {
		if annWhitelistRL.Status != EMPTY {
			data, ok := c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", path.Path)]
			if ok && len(data) > 0 {
				status = MODIFIED
			}
		}
		if annWhitelistRL.Value != "" && path.Status == ADDED {
			status = MODIFIED
		}
	}
	switch status {
	case ADDED, MODIFIED:
		if annWhitelist.Value != "" {
			ID := int64(0)
			httpRequest1 := &models.HTTPRequestRule{
				ID:       &ID,
				Type:     "allow",
				Cond:     "if",
				CondTest: fmt.Sprintf("{ path_beg %s } { src %s }", path.Path, strings.Replace(annWhitelist.Value, ",", " ", -1)),
			}
			httpRequest2 := &models.HTTPRequestRule{
				ID:       &ID,
				Type:     "deny",
				Cond:     "if",
				CondTest: fmt.Sprintf("{ path_beg %s }", path.Path),
			}
			if allowRateLimiting {
				c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", path.Path)] = []models.HTTPRequestRule{
					*httpRequest1,
				}
			} else {
				c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", path.Path)] = []models.HTTPRequestRule{
					*httpRequest2, //reverse order
					*httpRequest1,
				}
			}
		} else {
			c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", path.Path)] = []models.HTTPRequestRule{}
		}
		c.cfg.HTTPRequestsStatus = MODIFIED
	case DELETED:
		c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", path.Path)] = []models.HTTPRequestRule{}
	}
}
