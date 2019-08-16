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
	"log"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"

	//networking "k8s.io/api/networking/v1beta1"

	extensions "k8s.io/api/extensions/v1beta1"
)

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func LogErr(err error) {
	if err == nil {
		return
	}
	_, file, no, ok := runtime.Caller(1)
	if ok {
		file1 := strings.Replace(file, "/src/", "", 1)
		log.SetFlags(LogTypeShort)
		log.Printf("%s:%d %s\n", file1, no, err.Error())
		log.SetFlags(LogType)
	}
}

var chars = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

//RandomString returns random string of size n
func RandomString(n int) string {
	b := make([]rune, n)
	size := len(chars)
	for i := range b {
		b[i] = chars[rand.Intn(size)]
	}
	return string(b)
}

//ConvertIngressRules converts data from kubernetes format
func ConvertIngressRules(ingressRules []extensions.IngressRule) map[string]*IngressRule {
	rules := make(map[string]*IngressRule)
	for _, k8sRule := range ingressRules {
		paths := make(map[string]*IngressPath)
		for pathIndex, k8sPath := range k8sRule.HTTP.Paths {
			paths[k8sPath.Path] = &IngressPath{
				PathIndex:         pathIndex,
				Path:              k8sPath.Path,
				ServiceName:       k8sPath.Backend.ServiceName,
				ServicePortInt:    int64(k8sPath.Backend.ServicePort.IntValue()),
				ServicePortString: k8sPath.Backend.ServicePort.StrVal,
				Status:            "",
			}
		}
		rules[k8sRule.Host] = &IngressRule{
			Host:   k8sRule.Host,
			Paths:  paths,
			Status: "",
		}
	}
	return rules
}

func ptrInt64(value int64) *int64 {
	return &value
}

//nolint deadcode
func ptrString(value string) *string {
	return &value
}

func ParseTimeout(data string) (int64, error) {
	var v int64
	var err error
	switch {
	case strings.HasSuffix(data, "ms"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "ms"), 10, 64)
	case strings.HasSuffix(data, "s"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "s"), 10, 64)
		v *= 1000
	case strings.HasSuffix(data, "m"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "m"), 10, 64)
		v = v * 1000 * 60
	case strings.HasSuffix(data, "h"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "h"), 10, 64)
		v = v * 1000 * 60 * 60
	case strings.HasSuffix(data, "d"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "d"), 10, 64)
		v = v * 1000 * 60 * 60 * 24
	default:
		v, err = strconv.ParseInt(data, 10, 64)
	}
	return v, err
}

//annotationConvertToMS converts annotation time value to milisecon value
func annotationConvertTimeToMS(data StringW) (int64, error) {
	return ParseTimeout(data.Value)
}
