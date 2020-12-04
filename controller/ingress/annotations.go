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

package ingress

import (
	"strconv"
	"strings"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/configsnippet"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// Update backend with annotations values.
func (route *Route) handleBackendAnnotations(backendModel *models.Backend) (activeAnnotations bool) {
	activeAnnotations = false
	backend := haproxy.Backend(*backendModel)
	backendAnnotations := make(map[string]*store.StringW, 7)

	backendAnnotations["abortonclose"], _ = k8sStore.GetValueFromAnnotations("abortonclose", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	backendAnnotations["config-snippet"], _ = k8sStore.GetValueFromAnnotations("config-snippet", route.Ingress.Annotations, route.service.Annotations)
	backendAnnotations["cookie-persistence"], _ = k8sStore.GetValueFromAnnotations("cookie-persistence", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	backendAnnotations["load-balance"], _ = k8sStore.GetValueFromAnnotations("load-balance", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	backendAnnotations["timeout-check"], _ = k8sStore.GetValueFromAnnotations("timeout-check", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	if backend.Mode == "http" {
		backendAnnotations["check-http"], _ = k8sStore.GetValueFromAnnotations("check-http", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
		backendAnnotations["forwarded-for"], _ = k8sStore.GetValueFromAnnotations("forwarded-for", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
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
		if v.Status != EMPTY || route.NewBackend {
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
				if v.Status == DELETED && !route.NewBackend {
					backend.Httpchk = nil
				} else if err := backend.UpdateHttpchk(v.Value); err != nil {
					logger.Errorf("%s annotation: %s", k, err)
					continue
				}
				activeAnnotations = true
			case "config-snippet":
				var err error
				if v.Status == DELETED && !route.NewBackend {
					err = client.BackendCfgSnippetSet(backend.Name, nil)
				} else {
					value := strings.Split(strings.Trim(v.Value, "\n"), "\n")
					if len(value) == 0 {
						continue
					}
					err = client.BackendCfgSnippetSet(backend.Name, &value)
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
				if v.Status == DELETED && !route.NewBackend {
					backend.Cookie = nil
				} else {
					cookie := handleCookieAnnotations(route.Ingress, route.service)
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
	route.backendAnnotations = backendAnnotations
	return activeAnnotations

}

func handleCookieAnnotations(ingress *store.Ingress, service *store.Service) models.Cookie {

	cookieAnnotations := make(map[string]*store.StringW, 11)
	cookieAnnotations["cookie-persistence"], _ = k8sStore.GetValueFromAnnotations("cookie-persistence", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-domain"], _ = k8sStore.GetValueFromAnnotations("cookie-domain", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-dynamic"], _ = k8sStore.GetValueFromAnnotations("cookie-dynamic", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-httponly"], _ = k8sStore.GetValueFromAnnotations("cookie-httponly", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-indirect"], _ = k8sStore.GetValueFromAnnotations("cookie-indirect", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-maxidle"], _ = k8sStore.GetValueFromAnnotations("cookie-maxidle", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-maxlife"], _ = k8sStore.GetValueFromAnnotations("cookie-maxlife", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-nocache"], _ = k8sStore.GetValueFromAnnotations("cookie-nocache", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-postonly"], _ = k8sStore.GetValueFromAnnotations("cookie-postonly", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-preserve"], _ = k8sStore.GetValueFromAnnotations("cookie-preserve", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-secure"], _ = k8sStore.GetValueFromAnnotations("cookie-secure", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	cookieAnnotations["cookie-type"], _ = k8sStore.GetValueFromAnnotations("cookie-type", service.Annotations, ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
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

// Update server with annotations values.
func handleServerAnnotations(serverModel *models.Server, annotations map[string]*store.StringW) {
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

func (route *Route) getServerAnnotations() (activeAnnotations bool) {
	srvAnnotations := make(map[string]*store.StringW, 5)
	srvAnnotations["cookie-persistence"], _ = k8sStore.GetValueFromAnnotations("cookie-persistence", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	srvAnnotations["check"], _ = k8sStore.GetValueFromAnnotations("check", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	srvAnnotations["check-interval"], _ = k8sStore.GetValueFromAnnotations("check-interval", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	srvAnnotations["pod-maxconn"], _ = k8sStore.GetValueFromAnnotations("pod-maxconn", route.service.Annotations)
	srvAnnotations["server-ssl"], _ = k8sStore.GetValueFromAnnotations("server-ssl", route.service.Annotations, route.Ingress.Annotations, k8sStore.ConfigMaps[Main].Annotations)
	srvAnnotations["send-proxy-protocol"], _ = k8sStore.GetValueFromAnnotations("send-proxy-protocol", route.service.Annotations)
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
	route.srvAnnotations = srvAnnotations
	route.reload = activeAnnotations
	return activeAnnotations
}
