package main

import (
	"log"
	"strconv"

	goruntime "runtime"

	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleGlobalAnnotations(transaction *models.Transaction) (reloadRequested bool, err error) {
	reloadRequested = false
	maxProcs := goruntime.GOMAXPROCS(0)
	numThreads := maxProcs
	annNbthread, errNumThread := GetValueFromAnnotations("nbthread", c.cfg.ConfigMap.Annotations)

	if errNumThread == nil {
		if numthr, err := strconv.Atoi(annNbthread.Value); err == nil {
			if numthr < maxProcs {
				numThreads = numthr
			}
			log.Println(numThreads)
		}
	}

	return reloadRequested, err
}

func (c *HAProxyController) removeHTTPSListeners(transaction *models.Transaction) (err error) {
	return nil
}

func (c *HAProxyController) handleHTTPRedirect(usingHTTPS bool, transaction *models.Transaction) (reloadRequested bool, err error) {
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
