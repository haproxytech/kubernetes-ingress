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
	"strings"

	"github.com/haproxytech/client-native/misc"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
)

var ratelimitACL1 = models.ACL{
	ID:        utils.PtrInt64(0),
	ACLName:   "ratelimit_is_abuse",
	Criterion: "src_http_req_rate(RateLimit)",
	Value:     "ge 10",
}
var ratelimitACL2 = models.ACL{
	ID:        utils.PtrInt64(0),
	ACLName:   "ratelimit_inc_cnt_abuse",
	Criterion: "src_inc_gpc0(RateLimit)",
	Value:     "gt 0",
}
var ratelimitACL3 = models.ACL{
	ID:        utils.PtrInt64(0),
	ACLName:   "ratelimit_cnt_abuse",
	Criterion: "src_get_gpc0(RateLimit)",
	Value:     "gt 0",
}

func (c *HAProxyController) handleRateLimiting(usingHTTPS bool) (needReload bool, err error) {
	needReload = false
	annRateLimit, _ := GetValueFromAnnotations("rate-limit", c.cfg.ConfigMap.Annotations)

	annRateLimitExpire, _ := GetValueFromAnnotations("rate-limit-expire", c.cfg.ConfigMap.Annotations)
	annRateLimitInterval, _ := GetValueFromAnnotations("rate-limit-interval", c.cfg.ConfigMap.Annotations)
	annRateLimitSize, _ := GetValueFromAnnotations("rate-limit-size", c.cfg.ConfigMap.Annotations)
	rateLimitExpire, _ := utils.ParseTime(annRateLimitExpire.Value)
	rateLimitSize := misc.ParseSize(annRateLimitSize.Value)

	enabled, err := GetBoolValue(annRateLimit.Value, "rate-limit")
	if err != nil {
		return false, err
	}
	status := annRateLimit.Status
	if status == DELETED {
		c.cfg.RateLimitingEnabled = false
	}
	if status == ADDED || status == MODIFIED {
		if enabled {
			c.cfg.RateLimitingEnabled = true
		} else {
			status = DELETED
			c.cfg.RateLimitingEnabled = false
		}
	}
	if c.cfg.RateLimitingEnabled {
		if annRateLimitExpire.Status == MODIFIED {
			status = MODIFIED
		}
		if annRateLimitInterval.Status == MODIFIED {
			status = MODIFIED
		}
		if annRateLimitSize.Status == MODIFIED {
			status = MODIFIED
		}
	}

	tcpRequest1 := &models.TCPRequestRule{
		ID:     utils.PtrInt64(0),
		Type:   "connection",
		Action: "track-sc0 src table RateLimit",
	}
	tcpRequest2 := &models.TCPRequestRule{
		ID:       utils.PtrInt64(0),
		Type:     "connection",
		Action:   "reject",
		Cond:     "if",
		CondTest: ratelimitACL3.ACLName,
	}
	httpRequest1 := &models.HTTPRequestRule{
		ID:       utils.PtrInt64(0),
		Type:     "deny",
		Cond:     "if",
		CondTest: fmt.Sprintf("%s %s", ratelimitACL1.ACLName, ratelimitACL2.ACLName),
	}
	httpRequest2 := &models.HTTPRequestRule{
		ID:       utils.PtrInt64(0),
		Type:     "deny",
		Cond:     "if",
		CondTest: ratelimitACL3.ACLName,
	}

	addRateLimiting := func() {
		err := c.backendCreate(models.Backend{
			Name: "RateLimit",
			StickTable: &models.BackendStickTable{
				Type:   "ip",
				Expire: rateLimitExpire,
				Size:   rateLimitSize,
				Store:  fmt.Sprintf("gpc0,http_req_rate(%s)", annRateLimitInterval.Value),
			},
		})
		utils.LogErr(err)

		c.addACL(ratelimitACL1)
		c.addACL(ratelimitACL2)
		c.addACL(ratelimitACL3)

		c.cfg.TCPRequests[RATE_LIMIT] = []models.TCPRequestRule{
			*tcpRequest1,
			*tcpRequest2,
		}
		c.cfg.HTTPRequests[RATE_LIMIT] = []models.HTTPRequestRule{
			*httpRequest1,
			*httpRequest2,
		}

	}

	removeRateLimiting := func() {
		_, err := c.backendGet("RateLimit")
		if err == nil {
			err = c.backendDelete("RateLimit")
			utils.LogErr(err)
		}
		c.removeACL(ratelimitACL1, FrontendHTTP, FrontendHTTPS)
		c.removeACL(ratelimitACL2, FrontendHTTP, FrontendHTTPS)
		c.removeACL(ratelimitACL3, FrontendHTTP, FrontendHTTPS)

		c.cfg.HTTPRequests[RATE_LIMIT] = []models.HTTPRequestRule{}
		c.cfg.HTTPRequestsStatus = MODIFIED
		c.cfg.TCPRequests[RATE_LIMIT] = []models.TCPRequestRule{}
		c.cfg.TCPRequestsStatus = MODIFIED

		c.cfg.TCPRequests[RATE_LIMIT] = []models.TCPRequestRule{}
		c.cfg.HTTPRequests[RATE_LIMIT] = []models.HTTPRequestRule{}
	}

	switch status {
	case ADDED:
		if enabled {
			addRateLimiting()
		} else {
			removeRateLimiting()
		}
		needReload = true
	case MODIFIED:
		removeRateLimiting()
		addRateLimiting()
		needReload = true
	case DELETED:
		removeRateLimiting()
		needReload = true
	}
	return needReload, nil
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
				ID:       utils.PtrInt64(0),
				Type:     "allow",
				Cond:     "if",
				CondTest: fmt.Sprintf("{ path_beg %s } { src %s }", path.Path, strings.Replace(annWhitelist.Value, ",", " ", -1)),
			}
			httpRequest2 := &models.HTTPRequestRule{
				ID:       utils.PtrInt64(0),
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
