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
	"github.com/haproxytech/models"
	"hash/fnv"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	defaultCaptureLen = 128
)

func captureHash(s string) uint64 {
	h := fnv.New64a()
	_, err := h.Write([]byte(s))
	LogErr(err)
	return h.Sum64()
}

func generateCaptureFile(captureHosts map[uint64][]string) (err error) {
	var f *os.File
	for capture, hosts := range captureHosts {

		filename := path.Join(HAProxyCaptureDir, strconv.FormatUint(capture, 10)) + ".lst"
		if f, err = os.Create(filename); err != nil {
			log.Println(err)
			return err
		}
		defer f.Close()

		for _, host := range hosts {
			if _, err = f.WriteString(host + "\n"); err != nil {
				log.Println(err)
				return err
			}
		}
	}
	return nil
}

func cleanCapturefiles() {
	err := os.RemoveAll(HAProxyCaptureDir)
	LogErr(err)
	err = os.MkdirAll(HAProxyCaptureDir, 0755)
	LogErr(err)
}

func isMember(ss []string, e string) bool {
	for _, s := range ss {
		if s == e {
			return true
		}
	}
	return false
}

func (c *HAProxyController) handleHTTPRequestCapture(
	ingress *Ingress,
	captureHosts map[uint64][]string) (needReload bool, err error) {

	reload := false

	capturesAnn, err := GetValueFromAnnotations("request-capture", ingress.Annotations)
	if err != nil {
		return false, nil
	}

	var len int64
	captureLenAnn, err := GetValueFromAnnotations("request-capture-len", ingress.Annotations)
	if err == nil {
		len, err = strconv.ParseInt(captureLenAnn.Value, 10, 64)
		if err != nil {
			len = defaultCaptureLen
		}
		if captureLenAnn.Status == DELETED {
			len = defaultCaptureLen
			reload = true
		}
	} else {
		len = defaultCaptureLen
	}

	status := ingress.Status

	rules := []models.HTTPRequestRule{}
	for _, sample := range strings.Split(capturesAnn.Value, "\n") {
		if capturesAnn.Status == DELETED {
			break
		}
		if sample == "" {
			continue
		}
		key := captureHash(fmt.Sprintf("%s%d", sample, len))
		filename := path.Join(HAProxyCaptureDir, strconv.FormatUint(key, 10)) + ".lst"
		rule := &models.HTTPRequestRule{
			ID:            ptrInt64(0),
			Type:          "capture",
			CaptureSample: sample,
			Cond:          "if",
			CaptureLen:    len,
			CondTest:      fmt.Sprintf("{ req.hdr(Host) -f %s }", filename),
		}

		for hostname := range ingress.Rules {
			if hostname == "" {
				continue
			}
			if _, ok := captureHosts[key]; !ok {
				rules = append(rules, *rule)
			}
			if !isMember(captureHosts[key], hostname) {
				captureHosts[key] = append(captureHosts[key], hostname)
			}
		}
	}

	addRules := func() error {
		err = generateCaptureFile(captureHosts)
		if err != nil {
			log.Println(err)
			return err
		}
		c.cfg.HTTPRequests[HTTP_REQUEST_CAPTURE] = append(c.cfg.HTTPRequests[HTTP_REQUEST_CAPTURE], rules...)
		return nil
	}

	if status == DELETED ||
		capturesAnn.Status == DELETED ||
		(captureLenAnn != nil && captureLenAnn.Status == DELETED) {
		cleanCapturefiles()
		reload = true
	}

	switch status {
	case MODIFIED, ADDED, DELETED:
		c.cfg.HTTPRequestsStatus = MODIFIED
		reload = true
	}

	if err = addRules(); err != nil {
		return false, err
	}

	return reload, nil
}
