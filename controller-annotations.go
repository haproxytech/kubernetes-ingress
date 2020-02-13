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

func (c *HAProxyController) handleSSLPassthrough(ingress *Ingress, service *Service, path *IngressPath, backend *models.Backend, newBackend bool) (updateBackendSwitching bool) {

	if path.IsTCPService || path.IsDefaultBackend {
		return false
	}
	updateBackendSwitching = false
	annSSLPassthrough, _ := GetValueFromAnnotations("ssl-passthrough", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	status := annSSLPassthrough.Status
	if status == EMPTY {
		status = path.Status
	}
	if status != EMPTY || newBackend {
		enabled, err := GetBoolValue(annSSLPassthrough.Value, "ssl-passthrough")
		if err != nil {
			LogErr(fmt.Errorf("ssl-passthrough annotation: %s", err))
			return updateBackendSwitching
		}
		if enabled {
			if !path.IsSSLPassthrough {
				path.IsSSLPassthrough = true
				backend.Mode = "tcp"
				updateBackendSwitching = true

			}
		} else if path.IsSSLPassthrough {
			path.IsSSLPassthrough = false
			backend.Mode = "http"
			updateBackendSwitching = true
		}
	}
	return updateBackendSwitching
}

// Update backend with annotations values.
func (c *HAProxyController) handleBackendAnnotations(ingress *Ingress, service *Service, backendModel *models.Backend, newBackend bool) (activeAnnotations bool) {
	activeAnnotations = false
	backend := Backend(*backendModel)
	backendAnnotations := make(map[string]*StringW, 5)

	backendAnnotations["abortonclose"], _ = GetValueFromAnnotations("abortonclose", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["cookie-persistence"], _ = GetValueFromAnnotations("cookie-persistence", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["load-balance"], _ = GetValueFromAnnotations("load-balance", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["timeout-check"], _ = GetValueFromAnnotations("timeout-check", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if backend.Mode == "http" {
		backendAnnotations["check-http"], _ = GetValueFromAnnotations("check-http", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
		backendAnnotations["forwarded-for"], _ = GetValueFromAnnotations("forwarded-for", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	}

	// The DELETED status of an annotation is handled explicitly
	// only when there is no default annotation value.
	for k, v := range backendAnnotations {
		if v == nil {
			continue
		}
		if v.Status != EMPTY || newBackend {
			switch k {
			case "abortonclose":
				if err := backend.updateAbortOnClose(v); err != nil {
					LogErr(err)
					continue
				}
				activeAnnotations = true
			case "check-http":
				if v.Status == DELETED && !newBackend {
					backend.Httpchk = nil
				} else if err := backend.updateHttpchk(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				activeAnnotations = true
			case "cookie-persistence":
				if v.Status == DELETED && !newBackend {
					backend.Cookie = nil
				} else {
					annotations := c.getCookieAnnotations(ingress, service)
					if err := backend.updateCookie(v, annotations); err != nil {
						LogErr(fmt.Errorf("%s annotation: %s", k, err))
						continue
					}
				}
				activeAnnotations = true
			case "forwarded-for":
				if err := backend.updateForwardfor(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				activeAnnotations = true
			case "load-balance":
				if err := backend.updateBalance(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				activeAnnotations = true
			case "timeout-check":
				if v.Status == DELETED && !newBackend {
					backend.CheckTimeout = nil
				} else if err := backend.updateCheckTimeout(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				activeAnnotations = true
			}
		}
	}
	*backendModel = models.Backend(backend)
	return activeAnnotations

}

// Update server with annotations values.
func (c *HAProxyController) handleServerAnnotations(ingress *Ingress, service *Service, serverModel *models.Server) (activeAnnotations bool) {
	activeAnnotations = false
	server := Server(*serverModel)

	serverAnnotations := make(map[string]*StringW, 5)
	serverAnnotations["cookie-persistence"], _ = GetValueFromAnnotations("cookie-persistence", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	serverAnnotations["check"], _ = GetValueFromAnnotations("check", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	serverAnnotations["check-interval"], _ = GetValueFromAnnotations("check-interval", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	serverAnnotations["pod-maxconn"], _ = GetValueFromAnnotations("pod-maxconn", service.Annotations)
	serverAnnotations["server-ssl"], _ = GetValueFromAnnotations("server-ssl", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)

	// The DELETED status of an annotation is handled explicitly
	// only when there is no default annotation value.
	for k, v := range serverAnnotations {
		if v == nil {
			continue
		}
		if v.Status != EMPTY {
			switch k {
			case "cookie-persistence":
				if v.Status == DELETED {
					server.Cookie = ""
				} else {
					server.Cookie = server.Name
				}
				activeAnnotations = true
			case "check":
				if err := server.updateCheck(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				activeAnnotations = true
			case "check-interval":
				if v.Status == DELETED {
					server.Inter = nil
				} else if err := server.updateInter(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				activeAnnotations = true
			case "pod-maxconn":
				if v.Status == DELETED {
					server.Maxconn = nil
				} else if err := server.updateMaxconn(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				activeAnnotations = true
			case "server-ssl":
				if err := server.updateServerSsl(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				activeAnnotations = true
			}
		}
	}
	*serverModel = models.Server(server)
	return activeAnnotations
}

func (c *HAProxyController) handleRateLimitingAnnotations(ingress *Ingress, service *Service, path *IngressPath) {
	//Annotations with default values don't need error checking.
	annWhitelist, _ := GetValueFromAnnotations("whitelist", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annWhitelistRL, _ := GetValueFromAnnotations("whitelist-with-rate-limit", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	allowRateLimiting := annWhitelistRL.Value != "" && annWhitelistRL.Value != "OFF"
	status := annWhitelist.Status
	if status == EMPTY {
		if annWhitelistRL.Status != EMPTY {
			data, ok := c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)]
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
			httpRequest1 := &models.HTTPRequestRule{
				ID:       ptrInt64(0),
				Type:     "allow",
				Cond:     "if",
				CondTest: fmt.Sprintf("{ path_beg %s } { src %s }", path.Path, strings.Replace(annWhitelist.Value, ",", " ", -1)),
			}
			httpRequest2 := &models.HTTPRequestRule{
				ID:       ptrInt64(0),
				Type:     "deny",
				Cond:     "if",
				CondTest: fmt.Sprintf("{ path_beg %s }", path.Path),
			}
			if allowRateLimiting {
				c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)] = []models.HTTPRequestRule{
					*httpRequest1,
				}
			} else {
				c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)] = []models.HTTPRequestRule{
					*httpRequest2, //reverse order
					*httpRequest1,
				}
			}
		} else {
			c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)] = []models.HTTPRequestRule{}
		}
		c.cfg.HTTPRequestsStatus = MODIFIED
	case DELETED:
		c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)] = []models.HTTPRequestRule{}
	}
}

func GetBoolValue(dataValue, dataName string) (result bool, err error) {
	result, err = strconv.ParseBool(dataValue)
	if err != nil {
		switch strings.ToLower(dataValue) {
		case "enabled", "on":
			log.Println(fmt.Sprintf(`WARNING: %s - [%s] is DEPRECATED, use "true" or "false"`, dataName, dataValue))
			result = true
		case "disabled", "off":
			log.Println(fmt.Sprintf(`WARNING: %s - [%s] is DEPRECATED, use "true" or "false"`, dataName, dataValue))
			result = false
		default:
			return false, err
		}
	}
	return result, nil
}

func (c *HAProxyController) getCookieAnnotations(ingress *Ingress, service *Service) map[string]*StringW {

	cookieAnnotations := make(map[string]*StringW, 11)
	cookieAnnotations["cookie-domain"], _ = GetValueFromAnnotations("cookie-domain", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-dynamic"], _ = GetValueFromAnnotations("cookie-dynamic", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-httponly"], _ = GetValueFromAnnotations("cookie-httponly", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-indirect"], _ = GetValueFromAnnotations("cookie-indirect", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-maxidle"], _ = GetValueFromAnnotations("cookie-maxidle", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-maxlife"], _ = GetValueFromAnnotations("cookie-maxlife", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-nocache"], _ = GetValueFromAnnotations("cookie-nocache", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-postonly"], _ = GetValueFromAnnotations("cookie-postonly", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-preserve"], _ = GetValueFromAnnotations("cookie-preserve", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-secure"], _ = GetValueFromAnnotations("cookie-secure", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	cookieAnnotations["cookie-type"], _ = GetValueFromAnnotations("cookie-type", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	return cookieAnnotations
}
