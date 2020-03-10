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
	"hash/fnv"
	"net"
	"path"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
)

const (
	defaultCaptureLen      = 128
	defaultSSLRedirectCode = 302
)

func (c *HAProxyController) handleHTTPRedirect(ingress *Ingress) error {
	//  Get and validate annotations
	annSSLRedirect, _ := GetValueFromAnnotations("ssl-redirect", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annRedirectCode, _ := GetValueFromAnnotations("ssl-redirect-code", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if annSSLRedirect == nil {
		return nil
	}
	enabled, err := utils.GetBoolValue(annSSLRedirect.Value, "ssl-redirect")
	if err != nil {
		return err
	}
	var sslRedirectCode int64
	if sslRedirectCode, err = strconv.ParseInt(annRedirectCode.Value, 10, 64); err != nil {
		sslRedirectCode = defaultSSLRedirectCode
	}
	// Get Status
	status := ingress.Status
	if status == MODIFIED {
		if annSSLRedirect.Status != EMPTY {
			status = annSSLRedirect.Status
		}
	}
	// Update rules
	mapFiles := c.cfg.MapFiles
	key := hashStrToUint(fmt.Sprintf("%s-%d", HTTP_REDIRECT, sslRedirectCode))
	if status != EMPTY {
		if !enabled && !mapFiles.Exists(key) {
			return nil
		}
		mapFiles.Modified(key)
		c.cfg.HTTPRequestsStatus = MODIFIED
		if status == DELETED {
			return nil
		}
	}
	for hostname := range ingress.Rules {
		mapFiles.AppendHost(key, hostname)
	}

	mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
	httpRule := models.HTTPRequestRule{
		ID:         utils.PtrInt64(0),
		Type:       "redirect",
		RedirCode:  sslRedirectCode,
		RedirValue: "https",
		RedirType:  "scheme",
		Cond:       "if",
		CondTest:   fmt.Sprintf("{ req.hdr(Host) -f %s } !{ ssl_fc }", mapFile),
	}
	c.cfg.HTTPRequests[HTTP_REDIRECT][key] = httpRule
	return nil
}

func (c *HAProxyController) handleProxyProtocol() error {
	//  Get and validate annotations
	annProxyProtocol, _ := GetValueFromAnnotations("proxy-protocol", c.cfg.ConfigMap.Annotations)
	if annProxyProtocol == nil {
		return nil
	}
	value := strings.Replace(annProxyProtocol.Value, ",", " ", -1)
	for _, address := range strings.Fields(value) {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return fmt.Errorf("incorrect value for proxy-protocol annotation ")
			}
		}
	}

	// Get Rules status
	status := annProxyProtocol.Status

	// Update rules
	// Since this is a Configmap Annotation ONLY, no need to
	// track ingress hosts in Map file
	if status != EMPTY {
		c.cfg.TCPRequestsStatus = MODIFIED
		if status == DELETED {
			return nil
		}
	}

	tcpRule := models.TCPRequestRule{
		ID:       utils.PtrInt64(0),
		Type:     "connection",
		Action:   "expect-proxy layer4",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ src %s }", value),
	}
	c.cfg.TCPRequests[PROXY_PROTOCOL][0] = tcpRule

	return nil
}

func (c *HAProxyController) handleRequestCapture(ingress *Ingress) error {
	//  Get and validate annotations
	annReqCapture, _ := GetValueFromAnnotations("request-capture", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annCaptureLen, _ := GetValueFromAnnotations("request-capture-len", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if annReqCapture == nil {
		return nil
	}
	var captureLen int64
	var err error
	if annCaptureLen != nil {
		captureLen, err = strconv.ParseInt(annCaptureLen.Value, 10, 64)
		if err != nil {
			captureLen = defaultCaptureLen
		}
		if annCaptureLen.Status == DELETED {
			captureLen = defaultCaptureLen
		}
	} else {
		captureLen = defaultCaptureLen
	}

	// Get status
	status := ingress.Status
	if status == MODIFIED {
		if annReqCapture.Status != EMPTY {
			status = annReqCapture.Status
		}
	}

	// Update rules
	mapFiles := c.cfg.MapFiles
	for _, sample := range strings.Split(annReqCapture.Value, "\n") {
		key := hashStrToUint(fmt.Sprintf("%s-%s-%d", REQUEST_CAPTURE, sample, captureLen))
		if status != EMPTY {
			mapFiles.Modified(key)
			c.cfg.HTTPRequestsStatus = MODIFIED
			c.cfg.TCPRequestsStatus = MODIFIED
			if status == DELETED {
				break
			}
		}
		if sample == "" {
			continue
		}
		for hostname := range ingress.Rules {
			mapFiles.AppendHost(key, hostname)
		}

		mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
		httpRule := models.HTTPRequestRule{
			ID:            utils.PtrInt64(0),
			Type:          "capture",
			CaptureSample: sample,
			Cond:          "if",
			CaptureLen:    captureLen,
			CondTest:      fmt.Sprintf("{ req.hdr(Host) -f %s }", mapFile),
		}
		tcpRule := models.TCPRequestRule{
			ID:       utils.PtrInt64(0),
			Type:     "content",
			Action:   "capture " + sample + " len " + strconv.FormatInt(captureLen, 10),
			Cond:     "if",
			CondTest: fmt.Sprintf("{ req_ssl_sni -f %s }", mapFile),
		}
		c.cfg.HTTPRequests[REQUEST_CAPTURE][key] = httpRule
		c.cfg.TCPRequests[REQUEST_CAPTURE][key] = tcpRule
	}

	return err
}

func (c *HAProxyController) handleWhitelisting(ingress *Ingress) error {
	//  Get and validate annotations
	annWhitelist, _ := GetValueFromAnnotations("whitelist", ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if annWhitelist == nil {
		return nil
	}
	value := strings.Replace(annWhitelist.Value, ",", " ", -1)
	for _, address := range strings.Fields(value) {
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return fmt.Errorf("incorrect value for whitelist annotation in ingress '%s'", ingress.Name)
			}
		}
	}

	// Get Rules status
	status := ingress.Status
	if status == MODIFIED {
		if annWhitelist.Status != EMPTY {
			status = annWhitelist.Status
		}
	}

	// Update rules
	mapFiles := c.cfg.MapFiles
	key := hashStrToUint(fmt.Sprintf("%s-%s", WHITELIST, annWhitelist.Value))
	if status != EMPTY {
		mapFiles.Modified(key)
		c.cfg.HTTPRequestsStatus = MODIFIED
		c.cfg.TCPRequestsStatus = MODIFIED
		if status == DELETED {
			return nil
		}
	}
	for hostname := range ingress.Rules {
		mapFiles.AppendHost(key, hostname)
	}

	mapFile := path.Join(HAProxyMapDir, strconv.FormatUint(key, 10)) + ".lst"
	httpRule := models.HTTPRequestRule{
		ID:         utils.PtrInt64(0),
		Type:       "deny",
		DenyStatus: 403,
		Cond:       "if",
		CondTest:   fmt.Sprintf("{ req.hdr(Host) -f %s } !{ src %s }", mapFile, value),
	}
	tcpRule := models.TCPRequestRule{
		ID:       utils.PtrInt64(0),
		Type:     "content",
		Action:   "reject",
		Cond:     "if",
		CondTest: fmt.Sprintf("{ req_ssl_sni -f %s } !{ src %s }", mapFile, value),
	}
	c.cfg.HTTPRequests[WHITELIST][key] = httpRule
	c.cfg.TCPRequests[WHITELIST][key] = tcpRule

	return nil
}

func hashStrToUint(s string) uint64 {
	h := fnv.New64a()
	_, err := h.Write([]byte(strings.ToLower(s)))
	utils.LogErr(err)
	return h.Sum64()
}
