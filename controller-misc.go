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
	"strconv"
	"strings"

	goruntime "runtime"

	parser "github.com/haproxytech/config-parser"
	"github.com/haproxytech/config-parser/types"
	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleGlobalAnnotations() (reloadRequested bool, err error) {
	reloadRequested = false
	maxProcs := goruntime.GOMAXPROCS(0)
	numThreads := int64(maxProcs)
	annNbthread, errNumThread := GetValueFromAnnotations("nbthread", c.cfg.ConfigMap.Annotations)
	annSyslogSrv, errSyslogSrv := GetValueFromAnnotations("syslog-server", c.cfg.ConfigMap.Annotations)
	var errParser error

	if errNumThread == nil {
		if numthr, errConv := strconv.Atoi(annNbthread.Value); errConv == nil {
			if numthr < maxProcs {
				numThreads = int64(numthr)
			}
			if annNbthread.Status == DELETED {
				errParser = c.NativeParser.Delete(parser.Global, parser.GlobalSectionName, "nbthread")
				reloadRequested = true
			} else if annNbthread.Status != EMPTY {
				errParser = c.NativeParser.Insert(parser.Global, parser.GlobalSectionName, "nbthread", types.Int64C{
					Value: numThreads,
				})
				reloadRequested = true
			}
			LogErr(errParser)
		}
	}

	if errSyslogSrv == nil {
		if annSyslogSrv.Status == DELETED {
			errParser = c.NativeParser.Set(parser.Global, parser.GlobalSectionName, "log", nil)
			LogErr(errParser)
			reloadRequested = true
		} else if annSyslogSrv.Status != EMPTY {
			errParser = c.NativeParser.Set(parser.Global, parser.GlobalSectionName, "log", nil)
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
						LogErr(fmt.Errorf("incorrect syslog param: %s", paramLst))
						continue
					}
				}
				if _, ok := logMap["address"]; ok {
					logData := new(types.Log)
					for k, v := range logMap {
						switch strings.ToLower(k) {
						case "address":
							logData.Address = v
						case "port":
							logData.Address += ":" + v
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
							LogErr(fmt.Errorf("unkown syslog param: %s ", k))
							continue
						}
					}
					errParser = c.NativeParser.Insert(parser.Global, parser.GlobalSectionName, "log", logData, index)
					reloadRequested = true
				}
			}
		}
		LogErr(errParser)
	}

	return reloadRequested, err
}

func (c *HAProxyController) removeHTTPSListeners() (err error) {
	return nil
}

func (c *HAProxyController) handleHTTPRedirect(usingHTTPS bool) (reloadRequested bool, err error) {
	//see if we need to add redirect to https redirect scheme https if !{ ssl_fc }
	// no need for error checking, we have default value,
	//if not defined as OFF, we always do redirect
	reloadRequested = false
	sslRedirect, _ := GetValueFromAnnotations("ssl-redirect", c.cfg.ConfigMap.Annotations)
	useSSLRedirect := sslRedirect.Value != "OFF"
	if !usingHTTPS {
		useSSLRedirect = false
	}
	var state Status
	if useSSLRedirect {
		if c.cfg.SSLRedirect == "" {
			c.cfg.SSLRedirect = "ON"
			state = ADDED
		} else if c.cfg.SSLRedirect == "OFF" {
			c.cfg.SSLRedirect = "ON"
			state = ADDED
		}
	} else {
		if c.cfg.SSLRedirect == "" {
			c.cfg.SSLRedirect = "OFF"
			state = ""
		} else if c.cfg.SSLRedirect != "OFF" {
			c.cfg.SSLRedirect = "OFF"
			state = DELETED
		}
	}
	redirectCode := int64(302)
	annRedirectCode, _ := GetValueFromAnnotations("ssl-redirect-code", c.cfg.ConfigMap.Annotations)
	if value, err := strconv.ParseInt(annRedirectCode.Value, 10, 64); err == nil {
		redirectCode = value
	}
	if state == "" && annRedirectCode.Status != "" {
		state = MODIFIED
	}
	id := int64(0)
	rule := models.HTTPRequestRule{
		ID:         &id,
		Type:       "redirect",
		RedirCode:  redirectCode,
		RedirValue: "https",
		RedirType:  "scheme",
		Cond:       "if",
		CondTest:   "!{ ssl_fc }",
	}
	switch state {
	case ADDED:
		c.cfg.HTTPRequests[HTTP_REDIRECT] = []models.HTTPRequestRule{rule}
		c.cfg.HTTPRequestsStatus = MODIFIED
		c.cfg.SSLRedirect = "ON"
		reloadRequested = true
	case MODIFIED:
		c.cfg.HTTPRequests[HTTP_REDIRECT] = []models.HTTPRequestRule{rule}
		c.cfg.HTTPRequestsStatus = MODIFIED
		reloadRequested = true
	case DELETED:
		c.cfg.HTTPRequests[HTTP_REDIRECT] = []models.HTTPRequestRule{}
		c.cfg.HTTPRequestsStatus = MODIFIED
		c.cfg.SSLRedirect = "OFF"
		reloadRequested = true
	}
	return reloadRequested, nil
}
