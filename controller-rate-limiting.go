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

	"github.com/haproxytech/client-native/misc"
	"github.com/haproxytech/models"
)

var ratelimit_acl1 = models.ACL{
	ID:        ptrInt64(0),
	ACLName:   "ratelimit_is_abuse",
	Criterion: "src_http_req_rate(RateLimit)",
	Value:     "ge 10",
}
var ratelimit_acl2 = models.ACL{
	ID:        ptrInt64(0),
	ACLName:   "ratelimit_inc_cnt_abuse",
	Criterion: "src_inc_gpc0(RateLimit)",
	Value:     "gt 0",
}
var ratelimit_acl3 = models.ACL{
	ID:        ptrInt64(0),
	ACLName:   "ratelimit_cnt_abuse",
	Criterion: "src_get_gpc0(RateLimit)",
	Value:     "gt 0",
}

func (c *HAProxyController) handleRateLimiting(transaction *models.Transaction, usingHTTPS bool) (needReload bool, err error) {
	nativeAPI := c.NativeAPI
	needReload = false
	annRateLimit, _ := GetValueFromAnnotations("rate-limit", c.cfg.ConfigMap.Annotations)

	annRateLimitExpire, _ := GetValueFromAnnotations("rate-limit-expire", c.cfg.ConfigMap.Annotations)
	annRateLimitInterval, _ := GetValueFromAnnotations("rate-limit-interval", c.cfg.ConfigMap.Annotations)
	annRateLimitSize, _ := GetValueFromAnnotations("rate-limit-size", c.cfg.ConfigMap.Annotations)
	rateLimitExpire := misc.ParseTimeout(annRateLimitExpire.Value)
	rateLimitSize := misc.ParseSize(annRateLimitSize.Value)

	status := annRateLimit.Status
	if status == DELETED {
		c.cfg.RateLimitingEnabled = false
	}
	if status == ADDED || status == MODIFIED {
		if annRateLimit.Value != "OFF" {
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

	ID := int64(0)
	tcpRequest1 := &models.TCPRequestRule{
		ID:     &ID,
		Type:   "connection",
		Action: "track-sc0 src table RateLimit",
	}
	tcpRequest2 := &models.TCPRequestRule{
		ID:       &ID,
		Type:     "connection",
		Action:   "reject",
		Cond:     "if",
		CondTest: ratelimit_acl3.ACLName,
	}
	httpRequest1 := &models.HTTPRequestRule{
		ID:       &ID,
		Type:     "deny",
		Cond:     "if",
		CondTest: fmt.Sprintf("%s %s", ratelimit_acl1.ACLName, ratelimit_acl2.ACLName),
	}
	httpRequest2 := &models.HTTPRequestRule{
		ID:       &ID,
		Type:     "deny",
		Cond:     "if",
		CondTest: ratelimit_acl3.ACLName,
	}

	addRateLimiting := func() {
		backend := &models.Backend{
			Name: "RateLimit",
			StickTable: &models.BackendStickTable{
				Type:   "ip",
				Expire: rateLimitExpire,
				Size:   rateLimitSize,
				Store:  fmt.Sprintf("gpc0,http_req_rate(%s)", annRateLimitInterval.Value),
			},
		}
		err := nativeAPI.Configuration.CreateBackend(backend, transaction.ID, 0)
		LogErr(err)

		c.addACL(ratelimit_acl1)
		c.addACL(ratelimit_acl2)
		c.addACL(ratelimit_acl3)

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
		_, _, err := nativeAPI.Configuration.GetBackend("RateLimit", transaction.ID)
		if err == nil {
			err = nativeAPI.Configuration.DeleteBackend("RateLimit", transaction.ID, 0)
			LogErr(err)
		}
		c.removeACL(ratelimit_acl1, FrontendHTTP, FrontendHTTPS)
		c.removeACL(ratelimit_acl2, FrontendHTTP, FrontendHTTPS)
		c.removeACL(ratelimit_acl3, FrontendHTTP, FrontendHTTPS)

		c.cfg.HTTPRequests[RATE_LIMIT] = []models.HTTPRequestRule{}
		c.cfg.HTTPRequestsStatus = MODIFIED
		c.cfg.TCPRequests[RATE_LIMIT] = []models.TCPRequestRule{}
		c.cfg.TCPRequestsStatus = MODIFIED

		c.cfg.TCPRequests[RATE_LIMIT] = []models.TCPRequestRule{}
		c.cfg.HTTPRequests[RATE_LIMIT] = []models.HTTPRequestRule{}
	}

	switch status {
	case ADDED:
		if annRateLimit.Value != "OFF" {
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
