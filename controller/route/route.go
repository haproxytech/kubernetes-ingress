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

package route

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

//nolint:golint,stylecheck
const (
	// MapFiles
	SNI         = "sni"
	HOST        = "host"
	PATH_EXACT  = "path-exact"
	PATH_PREFIX = "path-prefix"
)

type Route struct {
	Host           string
	Path           *store.IngressPath
	HAProxyRules   []haproxy.RuleID
	BackendName    string
	SSLPassthrough bool
}

// AddHostPathRoute adds Host/Path ingress route to haproxy Map files used for backend switching.
func AddHostPathRoute(route Route, mapFiles haproxy.Maps) error {
	if route.BackendName == "" {
		return fmt.Errorf("backendName missing")
	}
	// Wildcard host
	if route.Host != "" && route.Host[0] == '*' {
		route.Host = route.Host[1:]
	}
	value := route.BackendName
	for _, id := range route.HAProxyRules {
		value += "." + strconv.Itoa(int(id))
	}
	// SSLPassthrough
	if route.SSLPassthrough {
		if route.Host == "" {
			return fmt.Errorf("empty SNI for backend %s,", route.BackendName)
		}
		mapFiles.AppendRow(SNI, route.Host+"\t\t\t"+value)
		return nil
	}
	// HTTP
	if route.Host != "" {
		mapFiles.AppendRow(HOST, route.Host+"\t\t\t"+route.Host)
	} else if route.Path.Path == "" {
		return fmt.Errorf("neither Host nor Path are provided for backend %v,", route.BackendName)
	}
	// if PathTypeExact is not set, PathTypePrefix will be applied
	path := route.Path.Path
	switch {
	case route.Path.ExactPathMatch:
		mapFiles.AppendRow(PATH_EXACT, route.Host+path+"\t\t\t"+value)
	case path == "" || path == "/":
		mapFiles.AppendRow(PATH_PREFIX, route.Host+"/"+"\t\t\t"+value)
	default:
		path = strings.TrimSuffix(path, "/")
		mapFiles.AppendRow(PATH_EXACT, route.Host+path+"\t\t\t"+value)
		mapFiles.AppendRow(PATH_PREFIX, route.Host+path+"/"+"\t\t\t"+value)
	}
	return nil
}
