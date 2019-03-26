package main

import (
	"fmt"

	"github.com/haproxytech/client-native/misc"
	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleRateLimiting(transaction *models.Transaction, usingHTTPS bool) (err error) {
	nativeAPI := c.NativeAPI
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
	acl1 := &models.ACL{
		ID:        &ID,
		ACLName:   "ratelimit_is_abuse",
		Criterion: "src_http_req_rate(RateLimit)",
		Value:     "ge 10",
	}
	acl2 := &models.ACL{
		ID:        &ID,
		ACLName:   "ratelimit_inc_cnt_abuse",
		Criterion: "src_inc_gpc0(RateLimit)",
		Value:     "gt 0",
	}
	acl3 := &models.ACL{
		ID:        &ID,
		ACLName:   "ratelimit_cnt_abuse",
		Criterion: "src_get_gpc0(RateLimit)",
		Value:     "gt 0",
	}
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
		CondTest: "ratelimit_cnt_abuse",
	}
	httpRequest1 := &models.HTTPRequestRule{
		ID:       &ID,
		Type:     "deny",
		Cond:     "if",
		CondTest: "ratelimit_cnt_abuse",
	}
	httpRequest2 := &models.HTTPRequestRule{
		ID:       &ID,
		Type:     "deny",
		Cond:     "if",
		CondTest: "ratelimit_is_abuse ratelimit_inc_cnt_abuse",
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

		err = nativeAPI.Configuration.CreateACL("frontend", "http", acl1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateACL("frontend", "http", acl2, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateACL("frontend", "http", acl3, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateACL("frontend", "https", acl1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateACL("frontend", "https", acl2, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateACL("frontend", "https", acl3, transaction.ID, 0)
		LogErr(err)

		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", "http", tcpRequest1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", "http", tcpRequest2, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "http", httpRequest1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "http", httpRequest2, transaction.ID, 0)
		LogErr(err)

		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", "https", tcpRequest1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", "https", tcpRequest2, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "https", httpRequest1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "https", httpRequest2, transaction.ID, 0)
		LogErr(err)

	}

	removeACLs := func(frontendName string) {
		startIDToRemove := int64(-1)
		aclsModel, err := nativeAPI.Configuration.GetACLs("frontend", frontendName, transaction.ID)
		if err == nil {
			data := aclsModel.Data
			for _, acl := range data {
				switch acl.ACLName {
				case acl1.ACLName, acl2.ACLName, acl3.ACLName:
					if startIDToRemove < 0 {
						startIDToRemove = *acl.ID
					} else {
						if startIDToRemove > *acl.ID {
							startIDToRemove = *acl.ID
						}
					}

				}
			}
			if startIDToRemove >= 0 {
				//this is not a mistake, just delete all three that are created (they are together)
				err = nativeAPI.Configuration.DeleteACL(startIDToRemove, "frontend", frontendName, transaction.ID, 0)
				LogErr(err)
				err = nativeAPI.Configuration.DeleteACL(startIDToRemove, "frontend", frontendName, transaction.ID, 0)
				LogErr(err)
				err = nativeAPI.Configuration.DeleteACL(startIDToRemove, "frontend", frontendName, transaction.ID, 0)
				LogErr(err)
			}
		}
	}

	removeTCPRules := func(frontendName string) {
		startIDToRemove := int64(-1)
		tcpRModel, err := nativeAPI.Configuration.GetTCPRequestRules("frontend", frontendName, transaction.ID)
		if err == nil {
			data := tcpRModel.Data
			for _, rule := range data {
				switch rule.CondTest {
				case tcpRequest1.CondTest, tcpRequest2.CondTest:
					if startIDToRemove < 0 {
						startIDToRemove = *rule.ID
					} else {
						if startIDToRemove > *rule.ID {
							startIDToRemove = *rule.ID
						}
					}

				}
			}
			if startIDToRemove >= 0 {
				//this is not a mistake, just delete all three that are created (they are together)
				err = nativeAPI.Configuration.DeleteTCPRequestRule(startIDToRemove, "frontend", frontendName, transaction.ID, 0)
				LogErr(err)
				err = nativeAPI.Configuration.DeleteTCPRequestRule(startIDToRemove, "frontend", frontendName, transaction.ID, 0)
				LogErr(err)
			}
		}
	}

	removeHTTPRules := func(frontendName string) {
		startIDToRemove := int64(-1)
		httpRModel, err := nativeAPI.Configuration.GetHTTPRequestRules("frontend", frontendName, transaction.ID)
		if err == nil {
			data := httpRModel.Data
			for _, rule := range data {
				switch rule.CondTest {
				case httpRequest1.CondTest, httpRequest2.CondTest:
					if startIDToRemove < 0 {
						startIDToRemove = *rule.ID
					} else {
						if startIDToRemove > *rule.ID {
							startIDToRemove = *rule.ID
						}
					}

				}
			}
			if startIDToRemove >= 0 {
				//this is not a mistake, just delete all three that are created (they are together)
				err = nativeAPI.Configuration.DeleteTCPRequestRule(startIDToRemove, "frontend", frontendName, transaction.ID, 0)
				LogErr(err)
				err = nativeAPI.Configuration.DeleteTCPRequestRule(startIDToRemove, "frontend", frontendName, transaction.ID, 0)
				LogErr(err)
			}
		}
	}

	removeRateLimiting := func() {
		err := nativeAPI.Configuration.DeleteBackend("RateLimit", transaction.ID, 0)
		LogErr(err)
		removeACLs("http")
		removeACLs("https")
		removeTCPRules("http")
		removeTCPRules("https")
		removeHTTPRules("http")
		removeHTTPRules("https")
	}

	switch status {
	case ADDED:
		if annRateLimit.Value != "OFF" {
			addRateLimiting()
		} else {
			removeRateLimiting()
		}
	case MODIFIED:
		removeRateLimiting()
		addRateLimiting()
	case DELETED:
		removeRateLimiting()
	}
	return nil
}
