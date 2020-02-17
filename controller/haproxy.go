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
	"log"
	goruntime "runtime"
	"strconv"
	"strings"

	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/config-parser/v2/types"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

func (c *HAProxyController) updateHAProxy() error {
	needsReload := false

	err := c.apiStartTransaction()
	if err != nil {
		utils.LogErr(err)
		return err
	}
	defer func() {
		c.apiDisposeTransaction()
	}()
	c.handleDefaultTimeouts()

	maxconnAnn, err := GetValueFromAnnotations("maxconn", c.cfg.ConfigMap.Annotations)
	if err == nil {
		if maxconnAnn.Status == DELETED {
			err = c.handleMaxconn(nil, FrontendHTTP, FrontendHTTPS)
			if err != nil {
				return err
			}
		} else if maxconnAnn.Status != "" {
			var value int64
			value, err = strconv.ParseInt(maxconnAnn.Value, 10, 64)
			if err == nil {
				err = c.handleMaxconn(&value, FrontendHTTP, FrontendHTTPS)
				if err != nil {
					return err
				}
			}
		}
	}

	reload, err := c.handleGlobalAnnotations()
	utils.LogErr(err)
	needsReload = needsReload || reload

	reload, err = c.handleDefaultService()
	utils.LogErr(err)
	needsReload = needsReload || reload

	captureHosts := map[uint64][]string{}
	usedCerts := map[string]struct{}{}

	for _, namespace := range c.cfg.Namespace {
		if !namespace.Relevant {
			continue
		}
		for _, ingress := range namespace.Ingresses {
			annClass, _ := GetValueFromAnnotations("ingress.class", ingress.Annotations) // default is ""
			if annClass.Value != "" && annClass.Value != c.osArgs.IngressClass {
				continue
			}
			if c.cfg.PublishService != nil && ingress.Status != DELETED {
				utils.LogErr(c.k8s.UpdateIngressStatus(ingress, c.cfg.PublishService))
			}
			// handle Default Backend
			if ingress.DefaultBackend != nil {
				reload, err = c.handlePath(namespace, ingress, &IngressRule{}, ingress.DefaultBackend)
				utils.LogErr(err)
				needsReload = needsReload || reload
			}
			// handle Ingress rules
			for _, rule := range ingress.Rules {
				for _, path := range rule.Paths {
					reload, err = c.handlePath(namespace, ingress, rule, path)
					needsReload = needsReload || reload
					utils.LogErr(err)
				}
			}
			//handle certs
			ingressSecrets := map[string]struct{}{}
			for _, tls := range ingress.TLS {
				if _, ok := ingressSecrets[tls.SecretName.Value]; !ok {
					ingressSecrets[tls.SecretName.Value] = struct{}{}
					reload = c.handleTLSSecret(*ingress, *tls, usedCerts)
					needsReload = needsReload || reload
				}
			}

			reload, err = c.handleCaptureRequest(ingress, captureHosts)
			utils.LogErr(err)
			needsReload = needsReload || reload
		}
	}

	reload = c.handleDefaultCertificate(usedCerts)
	needsReload = needsReload || reload

	reload = c.handleHTTPS(usedCerts)
	needsReload = needsReload || reload

	reload, err = c.handleRateLimiting(c.cfg.HTTPS)
	if err != nil {
		return err
	}
	needsReload = needsReload || reload

	reload, err = c.handleHTTPRedirect(c.cfg.HTTPS)
	if err != nil {
		return err
	}
	needsReload = needsReload || reload

	reload, err = c.requestsTCPRefresh()
	utils.LogErr(err)
	needsReload = needsReload || reload

	reload, err = c.RequestsHTTPRefresh()
	utils.LogErr(err)
	needsReload = needsReload || reload

	reload, err = c.handleTCPServices()
	utils.LogErr(err)
	needsReload = needsReload || reload

	reload = c.refreshBackendSwitching()
	needsReload = needsReload || reload

	err = c.apiCommitTransaction()
	if err != nil {
		utils.LogErr(err)
		return err
	}
	c.cfg.Clean()
	if needsReload {
		if err := c.HAProxyReload(); err != nil {
			utils.LogErr(err)
		} else {
			log.Println("HAProxy reloaded")
		}
	}
	return nil
}

func (c *HAProxyController) handleMaxconn(maxconn *int64, frontends ...string) error {
	for _, frontendName := range frontends {
		if frontend, err := c.frontendGet(frontendName); err == nil {
			frontend.Maxconn = maxconn
			err1 := c.frontendEdit(frontend)
			utils.LogErr(err1)
		} else {
			return err
		}
	}
	return nil
}

func (c *HAProxyController) handleGlobalAnnotations() (reloadRequested bool, err error) {
	reloadRequested = false
	maxProcs := goruntime.GOMAXPROCS(0)
	numThreads := int64(maxProcs)
	annNbthread, errNumThread := GetValueFromAnnotations("nbthread", c.cfg.ConfigMap.Annotations)
	// syslog-server has default value
	annSyslogSrv, _ := GetValueFromAnnotations("syslog-server", c.cfg.ConfigMap.Annotations)
	var errParser error
	config, _ := c.ActiveConfiguration()
	if errNumThread == nil {
		if numthr, errConv := strconv.Atoi(annNbthread.Value); errConv == nil {
			if numthr < maxProcs {
				numThreads = int64(numthr)
			}
			if annNbthread.Status == DELETED {
				errParser = config.Delete(parser.Global, parser.GlobalSectionName, "nbthread")
				reloadRequested = true
			} else if annNbthread.Status != EMPTY {
				errParser = config.Insert(parser.Global, parser.GlobalSectionName, "nbthread", types.Int64C{
					Value: numThreads,
				})
				reloadRequested = true
			}
			utils.LogErr(errParser)
		}
	}

	if annSyslogSrv.Status != EMPTY {
		stdoutLog := false
		errParser = config.Set(parser.Global, parser.GlobalSectionName, "log", nil)
		utils.LogErr(errParser)
		for index, syslogSrv := range strings.Split(annSyslogSrv.Value, "\n") {
			if syslogSrv == "" {
				continue
			}
			syslogSrv = strings.Join(strings.Fields(syslogSrv), "")
			logMap := make(map[string]string)
			for _, paramStr := range strings.Split(syslogSrv, ",") {
				paramLst := strings.Split(paramStr, ":")
				if len(paramLst) == 2 {
					logMap[paramLst[0]] = paramLst[1]
				} else {
					utils.LogErr(fmt.Errorf("incorrect syslog param: %s", paramLst))
					continue
				}
			}
			if address, ok := logMap["address"]; ok {
				logData := new(types.Log)
				logData.Address = address
				for k, v := range logMap {
					switch strings.ToLower(k) {
					case "address":
						if v == "stdout" {
							stdoutLog = true
						}
					case "port":
						if logMap["address"] != "stdout" {
							logData.Address += ":" + v
						}
					case "length":
						if length, errConv := strconv.Atoi(v); errConv == nil {
							logData.Length = int64(length)
						}
					case "format":
						logData.Format = v
					case "facility":
						logData.Facility = v
					case "level":
						logData.Level = v
					case "minlevel":
						logData.Level = v
					default:
						utils.LogErr(fmt.Errorf("unkown syslog param: %s ", k))
						continue
					}
				}
				errParser = config.Insert(parser.Global, parser.GlobalSectionName, "log", logData, index)
				reloadRequested = true
			}
			utils.LogErr(errParser)
		}
		if stdoutLog {
			errParser = config.Delete(parser.Global, parser.GlobalSectionName, "daemon")
		} else {
			errParser = config.Insert(parser.Global, parser.GlobalSectionName, "daemon", types.Enabled{})
		}
		utils.LogErr(errParser)
	}

	return reloadRequested, err
}

// handles defaultBackned configured via cli param "default-backend-service"
func (c *HAProxyController) handleDefaultService() (needsReload bool, err error) {
	needsReload = false
	dsvcData, _ := GetValueFromAnnotations("default-backend-service")
	dsvc := strings.Split(dsvcData.Value, "/")

	if len(dsvc) != 2 {
		return needsReload, fmt.Errorf("default service invalid data")
	}
	if dsvc[0] == "" || dsvc[1] == "" {
		return needsReload, nil
	}
	namespace, ok := c.cfg.Namespace[dsvc[0]]
	if !ok {
		return needsReload, fmt.Errorf("default service invalid namespace " + dsvc[0])
	}
	service, ok := namespace.Services[dsvc[1]]
	if !ok {
		return needsReload, fmt.Errorf("service '" + dsvc[1] + "' does not exist")
	}
	ingress := &Ingress{
		Namespace:   namespace.Name,
		Name:        "DefaultService",
		Annotations: MapStringW{},
		Rules:       map[string]*IngressRule{},
	}
	path := &IngressPath{
		ServiceName:      service.Name,
		ServicePortInt:   service.Ports[0].ServicePort,
		IsDefaultBackend: true,
	}
	return c.handlePath(namespace, ingress, &IngressRule{}, path)
}
