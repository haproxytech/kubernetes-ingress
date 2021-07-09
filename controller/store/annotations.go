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

package store

import (
	"strings"
	"time"
)

// CopyAnnotations returns a copy of annotations map and removes prefixe from annotations name
func CopyAnnotations(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for name, value := range in {
		out[convertAnnotationName(name)] = value
	}
	return out
}

func convertAnnotationName(annotation string) string {
	split := strings.SplitN(annotation, "/", 2)
	return split[len(split)-1]
}

// GetValueFromAnnotations returns value by checking in multiple annotations.
// moves through list until it finds value
func (k K8s) GetValueFromAnnotations(annotationName string, annotations ...map[string]string) string {
	for _, a := range annotations {
		val, ok := a[annotationName]
		if ok {
			return val
		}
	}
	return defaultAnnotationValues[annotationName]
}

func (k K8s) GetTimeFromAnnotation(name string) time.Duration {
	d := k.GetValueFromAnnotations(name)
	if d == "" {
		logger.Panic("Empty annotation %s", name)
	}
	duration, parseErr := time.ParseDuration(d)
	if parseErr != nil {
		logger.Panic("Unable to parse annotation %s: %s", name, parseErr)
	}
	return duration
}

func (k K8s) SetDefaultAnnotation(annotation, value string) {
	defaultAnnotationValues[annotation] = value
}

var defaultAnnotationValues = map[string]string{
	"auth-realm":              "Protected Content",
	"check":                   "true",
	"cors-allow-origin":       "*",
	"cors-allow-methods":      "*",
	"cors-allow-headers":      "*",
	"cors-max-age":            "5s",
	"cookie-indirect":         "true",
	"cookie-nocache":          "true",
	"cookie-type":             "insert",
	"forwarded-for":           "true",
	"load-balance":            "roundrobin",
	"log-format":              "%ci:%cp [%tr] %ft %b/%s %TR/%Tw/%Tc/%Tr/%Ta %ST %B %CC %CS %tsc %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs \"%HM %[var(txn.base)] %HV\"",
	"rate-limit-size":         "100k",
	"rate-limit-period":       "1s",
	"rate-limit-status-code":  "403",
	"request-capture-len":     "128",
	"ssl-redirect-code":       "302",
	"request-redirect-code":   "302",
	"ssl-redirect-port":       "443",
	"ssl-passthrough":         "false",
	"server-ssl":              "false",
	"scale-server-slots":      "42",
	"syslog-server":           "address:127.0.0.1, facility: local0, level: notice",
	"timeout-http-request":    "5s",
	"timeout-connect":         "5s",
	"timeout-client":          "50s",
	"timeout-queue":           "5s",
	"timeout-server":          "50s",
	"timeout-tunnel":          "1h",
	"timeout-http-keep-alive": "1m",
	"hard-stop-after":         "1h",
	"client-crt-optional":     "false",
}
