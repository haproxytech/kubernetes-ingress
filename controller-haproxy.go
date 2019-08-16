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
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/haproxytech/models"
)

func (c *HAProxyController) updateHAProxy() error {
	needsReload := false
	nativeAPI := c.NativeAPI

	c.handleDefaultTimeouts()
	version, err := nativeAPI.Configuration.GetVersion("")
	if err != nil || version < 1 {
		//silently fallback to 1
		version = 1
	}
	//log.Println("Config version:", version)
	transaction, err := nativeAPI.Configuration.StartTransaction(version)
	c.ActiveTransaction = transaction.ID
	defer func() {
		c.ActiveTransaction = ""
	}()
	if err != nil {
		log.Println(err)
		return err
	}
	maxconnAnn, err := GetValueFromAnnotations("maxconn", c.cfg.ConfigMap.Annotations)
	if err == nil {
		if maxconnAnn.Status == DELETED {
			err = c.handleMaxconn(transaction, nil, FrontendHTTP, FrontendHTTPS)
			if err != nil {
				return err
			}
		} else if maxconnAnn.Status != "" {
			var value int64
			value, err = strconv.ParseInt(maxconnAnn.Value, 10, 64)
			if err == nil {
				err = c.handleMaxconn(transaction, &value, FrontendHTTP, FrontendHTTPS)
				if err != nil {
					return err
				}
			}
		}
	}

	reload, err := c.handleGlobalAnnotations(transaction)
	LogErr(err)
	needsReload = needsReload || reload

	var usingHTTPS bool
	reload, usingHTTPS, err = c.handleHTTPS(transaction)
	if err != nil {
		return err
	}
	needsReload = needsReload || reload

	reload, err = c.handleRateLimiting(transaction, usingHTTPS)
	if err != nil {
		return err
	}
	needsReload = needsReload || reload

	reload, err = c.handleHTTPRedirect(usingHTTPS, transaction)
	if err != nil {
		return err
	}
	needsReload = needsReload || reload
	backendsUsed := map[string]struct{}{}
	for _, namespace := range c.cfg.Namespace {
		if !namespace.Relevant {
			continue
		}
		for _, ingress := range namespace.Ingresses {
			pathIndex := 0
			annClass, _ := GetValueFromAnnotations("ingress.class", ingress.Annotations) // default is ""
			if annClass.Value != "" && annClass.Value != c.osArgs.IngressClass {
				ingress.Status = DELETED
			}
			//no need for switch/case for now
			sortedList := make([]string, len(ingress.Rules))
			index := 0
			for name := range ingress.Rules {
				sortedList[index] = name
				index++
			}
			sort.Strings(sortedList)
			for _, ruleName := range sortedList {
				rule := ingress.Rules[ruleName]
				indexedPaths := make([]*IngressPath, len(rule.Paths))
				for _, path := range rule.Paths {
					if path.Status != DELETED && ingress.Status != DELETED {
						indexedPaths[path.PathIndex] = path
					} else {
						delete(c.cfg.UseBackendRules, fmt.Sprintf("R%s%s%0006d", namespace.Name, ingress.Name, pathIndex))
						c.cfg.UseBackendRulesStatus = MODIFIED
					}
				}
				for i := range indexedPaths {
					path := indexedPaths[i]
					if path == nil {
						continue
					}
					reload, err = c.handlePath(pathIndex, namespace, ingress, rule, path, backendsUsed, transaction)
					needsReload = needsReload || reload
					LogErr(err)
					pathIndex++
				}
			}
		}
	}
	//handle default service
	reload, err = c.handleDefaultService(backendsUsed, transaction)
	LogErr(err)
	needsReload = needsReload || reload

	reload, err = c.requestsTCPRefresh(transaction)
	LogErr(err)
	needsReload = needsReload || reload

	reload, err = c.RequestsHTTPRefresh()
	LogErr(err)
	needsReload = needsReload || reload

	reload = c.useBackendRuleRefresh()
	needsReload = needsReload || reload

	_, err = nativeAPI.Configuration.CommitTransaction(transaction.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	c.cfg.Clean()
	if needsReload {
		if err := c.HAProxyReload(); err != nil {
			log.Println(err)
		} else {
			log.Println("HAProxy reloaded")
		}
	}
	return nil
}

func (c *HAProxyController) handleMaxconn(transaction *models.Transaction, maxconn *int64, frontends ...string) error {
	for _, frontendName := range frontends {
		if frontend, err := c.frontendGet(frontendName); err == nil {
			frontend.Maxconn = maxconn
			err1 := c.frontendEdit(frontend)
			LogErr(err1)
		} else {
			return err
		}
	}
	return nil
}

func (c *HAProxyController) handleDefaultService(backendsUsed map[string]struct{}, transaction *models.Transaction) (needsReload bool, err error) {
	needsReload = false
	dsvcData, _ := GetValueFromAnnotations("default-backend-service")
	dsvc := strings.Split(dsvcData.Value, "/")

	if len(dsvc) != 2 {
		return needsReload, errors.New("default service invalid data")
	}
	namespace, ok := c.cfg.Namespace[dsvc[0]]
	if !ok {
		return needsReload, errors.New("default service invalid namespace " + dsvc[0])
	}
	ingress := &Ingress{
		Namespace:   namespace.Name,
		Annotations: MapStringW{},
		Rules:       map[string]*IngressRule{},
	}
	path := &IngressPath{
		ServiceName: dsvc[1],
		PathIndex:   -1,
	}
	return c.handlePath(0, namespace, ingress, nil, path, backendsUsed, transaction)
}
