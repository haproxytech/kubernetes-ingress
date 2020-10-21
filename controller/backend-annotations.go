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
	"strconv"
	"strings"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/configsnippet"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

func (c *HAProxyController) handleSSLPassthrough(ingress *store.Ingress, service *store.Service, path *store.IngressPath, backend *models.Backend, newBackend bool) (updateBackendSwitching bool) {

	if path.IsTCPService || path.IsDefaultBackend {
		return false
	}
	updateBackendSwitching = false
	annSSLPassthrough, _ := c.Store.GetValueFromAnnotations("ssl-passthrough", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	status := annSSLPassthrough.Status
	if status == EMPTY {
		status = path.Status
	}
	if status != EMPTY || newBackend {
		enabled, err := utils.GetBoolValue(annSSLPassthrough.Value, "ssl-passthrough")
		if err != nil {
			logger.Errorf("ssl-passthrough annotation: %s", err)
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
func (c *HAProxyController) handleBackendAnnotations(ingress *store.Ingress, service *store.Service, backendModel *models.Backend, newBackend bool) (activeAnnotations bool) {
	activeAnnotations = false
	backend := haproxy.Backend(*backendModel)
	backendAnnotations := make(map[string]*store.StringW, 7)

	backendAnnotations["abortonclose"], _ = c.Store.GetValueFromAnnotations("abortonclose", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	backendAnnotations["config-snippet"], _ = c.Store.GetValueFromAnnotations("config-snippet", ingress.Annotations, service.Annotations)
	backendAnnotations["cookie-persistence"], _ = c.Store.GetValueFromAnnotations("cookie-persistence", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	backendAnnotations["load-balance"], _ = c.Store.GetValueFromAnnotations("load-balance", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	backendAnnotations["timeout-check"], _ = c.Store.GetValueFromAnnotations("timeout-check", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	if backend.Mode == "http" {
		backendAnnotations["check-http"], _ = c.Store.GetValueFromAnnotations("check-http", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
		backendAnnotations["forwarded-for"], _ = c.Store.GetValueFromAnnotations("forwarded-for", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	}

	var cs string
	if a, ok := backendAnnotations["config-snippet"]; ok && a != nil {
		cs = a.Value
	}

	// The DELETED status of an annotation is handled explicitly
	// only when there is no default annotation value.
	for k, v := range backendAnnotations {
		if v == nil {
			continue
		}
		if v.Status != EMPTY || newBackend {
			logger.Debugf("Backend '%s': Configuring '%s' annotation", backend.Name, k)
			switch k {
			case "abortonclose":
				if err := configsnippet.NewGenericAttribute("option abort").Overridden(cs); err != nil {
					logger.Warning(err.Error())
					continue
				}
				if err := backend.UpdateAbortOnClose(v.Value); err != nil {
					logger.Error(err)
					continue
				}
				activeAnnotations = true
			case "check-http":
				if err := configsnippet.NewGenericAttribute("option httpchk").Overridden(cs); err != nil {
					logger.Warning(err.Error())
					continue
				}
				if v.Status == DELETED && !newBackend {
					backend.Httpchk = nil
				} else if err := backend.UpdateHttpchk(v.Value); err != nil {
					logger.Errorf("%s annotation: %s", k, err)
					continue
				}
				activeAnnotations = true
			case "config-snippet":
				var err error
				if v.Status == DELETED && !newBackend {
					err = c.Client.BackendCfgSnippetSet(backend.Name, nil)
				} else {
					value := strings.SplitN(strings.Trim(v.Value, "\n"), "\n", -1)
					if len(value) == 0 {
						continue
					}
					err = c.Client.BackendCfgSnippetSet(backend.Name, &value)
				}
				if err != nil {
					logger.Errorf("%s annotation: %s", k, err)
				} else {
					activeAnnotations = true
				}
			case "cookie-persistence":
				if err := configsnippet.NewGenericAttribute("cookie").Overridden(cs); err != nil {
					logger.Warning(err.Error())
					continue
				}
				if v.Status == DELETED && !newBackend {
					backend.Cookie = nil
				} else {
					cookie := c.handleCookieAnnotations(ingress, service)
					if err := backend.UpdateCookie(&cookie); err != nil {
						logger.Errorf("%s annotation: %s", k, err)
						continue
					}
				}
				activeAnnotations = true
			case "forwarded-for":
				if err := configsnippet.NewGenericAttribute("option forwardfor").Overridden(cs); err != nil {
					logger.Warning(err.Error())
					continue
				}
				if err := backend.UpdateForwardfor(v.Value); err != nil {
					logger.Errorf("%s annotation: %s", k, err)
					continue
				}
				activeAnnotations = true
			case "load-balance":
				if err := configsnippet.NewGenericAttribute("balance").Overridden(cs); err != nil {
					logger.Warning(err.Error())
					continue
				}
				if err := backend.UpdateBalance(v.Value); err != nil {
					logger.Errorf("%s annotation: %s", k, err)
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
func (c *HAProxyController) handleServerAnnotations(serverModel *models.Server, annotations map[string]*store.StringW) {
	server := haproxy.Server(*serverModel)

	// The DELETED status of an annotation is handled explicitly
	// only when there is no default annotation value.
	for k, v := range annotations {
		if v == nil {
			continue
		}
		logger.Tracef("Server '%s': Configuring '%s' annotation", server.Name, k)
		switch k {
		case "cookie-persistence":
			if v.Status == DELETED {
				server.Cookie = ""
			} else {
				server.Cookie = server.Name
			}
		case "check":
			if err := server.UpdateCheck(v.Value); err != nil {
				logger.Errorf("%s annotation: %s", k, err)
				continue
			}
		case "check-interval":
			if v.Status == DELETED {
				server.Inter = nil
			} else if err := server.UpdateInter(v.Value); err != nil {
				logger.Errorf("%s annotation: %s", k, err)
				continue
			}
		case "pod-maxconn":
			if v.Status == DELETED {
				server.Maxconn = nil
			} else if err := server.UpdateMaxconn(v.Value); err != nil {
				logger.Errorf("%s annotation: %s", k, err)
				continue
			}
		case "server-ssl":
			if err := server.UpdateServerSsl(v.Value); err != nil {
				logger.Errorf("%s annotation: %s", k, err)
				continue
			}
		case "send-proxy-protocol":
			if v.Status == DELETED || len(v.Value) == 0 {
				server.ResetSendProxy()
				continue
			}
			if err := server.UpdateSendProxy(v.Value); err != nil {
				logger.Errorf("%s annotation: %s", k, err)
				continue
			}
		}
	}
	*serverModel = models.Server(server)
}

func (c *HAProxyController) handleCookieAnnotations(ingress *store.Ingress, service *store.Service) models.Cookie {

	cookieAnnotations := make(map[string]*store.StringW, 11)
	cookieAnnotations["cookie-persistence"], _ = c.Store.GetValueFromAnnotations("cookie-persistence", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-domain"], _ = c.Store.GetValueFromAnnotations("cookie-domain", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-dynamic"], _ = c.Store.GetValueFromAnnotations("cookie-dynamic", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-httponly"], _ = c.Store.GetValueFromAnnotations("cookie-httponly", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-indirect"], _ = c.Store.GetValueFromAnnotations("cookie-indirect", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-maxidle"], _ = c.Store.GetValueFromAnnotations("cookie-maxidle", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-maxlife"], _ = c.Store.GetValueFromAnnotations("cookie-maxlife", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-nocache"], _ = c.Store.GetValueFromAnnotations("cookie-nocache", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-postonly"], _ = c.Store.GetValueFromAnnotations("cookie-postonly", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-preserve"], _ = c.Store.GetValueFromAnnotations("cookie-preserve", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-secure"], _ = c.Store.GetValueFromAnnotations("cookie-secure", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-type"], _ = c.Store.GetValueFromAnnotations("cookie-type", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	cookie := models.Cookie{}
	for k, v := range cookieAnnotations {
		if v == nil {
			continue
		}
		switch k {
		case "cookie-domain":
			var domains []*models.Domain
			for _, domain := range strings.Fields(v.Value) {
				domains = append(domains, &models.Domain{Value: domain})
			}
			cookie.Domains = domains
		case "cookie-dynamic":
			dynamic, err := utils.GetBoolValue(v.Value, "cookie-dynamic")
			logger.Error(err)
			cookie.Dynamic = dynamic
		case "cookie-httponly":
			httponly, err := utils.GetBoolValue(v.Value, "cookie-httponly")
			logger.Error(err)
			cookie.Httponly = httponly
		case "cookie-indirect":
			indirect, err := utils.GetBoolValue(v.Value, "cookie-indirect")
			logger.Error(err)
			cookie.Indirect = indirect
		case "cookie-maxidle":
			maxidle, err := strconv.ParseInt(v.Value, 10, 64)
			logger.Error(err)
			cookie.Maxidle = maxidle
		case "cookie-maxlife":
			maxlife, err := strconv.ParseInt(v.Value, 10, 64)
			logger.Error(err)
			cookie.Maxlife = maxlife
		case "cookie-nocache":
			nocache, err := utils.GetBoolValue(v.Value, "cookie-nocache")
			logger.Error(err)
			cookie.Nocache = nocache
		case "cookie-persistence":
			cookie.Name = utils.PtrString(v.Value)
		case "cookie-postonly":
			postonly, err := utils.GetBoolValue(v.Value, "cookie-postonly")
			logger.Error(err)
			cookie.Postonly = postonly
		case "cookie-preserve":
			preserve, err := utils.GetBoolValue(v.Value, "cookie-preserve")
			logger.Error(err)
			cookie.Preserve = preserve
		case "cookie-secure":
			secure, err := utils.GetBoolValue(v.Value, "cookie-secure")
			logger.Error(err)
			cookie.Secure = secure
		case "cookie-type":
			cookie.Type = v.Value
		}
	}
	return cookie
}

func (c *HAProxyController) getServerAnnotations(ingress *store.Ingress, service *store.Service) (srvAnnotations map[string]*store.StringW, activeAnnotations bool) {
	srvAnnotations = make(map[string]*store.StringW, 5)
	srvAnnotations["cookie-persistence"], _ = c.Store.GetValueFromAnnotations("cookie-persistence", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	srvAnnotations["check"], _ = c.Store.GetValueFromAnnotations("check", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	srvAnnotations["check-interval"], _ = c.Store.GetValueFromAnnotations("check-interval", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	srvAnnotations["pod-maxconn"], _ = c.Store.GetValueFromAnnotations("pod-maxconn", service.Annotations)
	srvAnnotations["server-ssl"], _ = c.Store.GetValueFromAnnotations("server-ssl", service.Annotations, ingress.Annotations, c.Store.ConfigMaps[Main].Annotations)
	srvAnnotations["send-proxy-protocol"], _ = c.Store.GetValueFromAnnotations("send-proxy-protocol", service.Annotations)
	for k, v := range srvAnnotations {
		if v == nil {
			delete(srvAnnotations, k)
			continue
		}
		if v.Status != EMPTY {
			activeAnnotations = true
			break
		}
	}
	return srvAnnotations, activeAnnotations
}
