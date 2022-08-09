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
	"strings"

	"github.com/haproxytech/client-native/v3/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

//nolint:golint,stylecheck
const (
	// Main frontends
	FrontendHTTP  = "http"
	FrontendHTTPS = "https"
	// Routing Maps
	SNI         maps.Name = "sni"
	HOST        maps.Name = "host"
	PATH_EXACT  maps.Name = "path-exact"
	PATH_PREFIX maps.Name = "path-prefix"
)

var (
	CustomRoutes = make(map[string]string)
	logger       = utils.GetLogger()
)

type Route struct {
	Host           string
	Path           *store.IngressPath
	HAProxyRules   []rules.RuleID
	BackendName    string
	SSLPassthrough bool
}

// AddHostPathRoute adds Host/Path ingress route to haproxy Map files used for backend switching.
func AddHostPathRoute(route Route, mapFiles maps.Maps) error {
	if route.BackendName == "" {
		return fmt.Errorf("backendName missing")
	}
	// Wildcard host
	if route.Host != "" && route.Host[0] == '*' {
		route.Host = route.Host[1:]
	}
	value := route.BackendName
	for _, id := range route.HAProxyRules {
		value += "." + string(id)
	}
	// SSLPassthrough
	if route.SSLPassthrough {
		if route.Host == "" {
			return fmt.Errorf("empty SNI for backend %s,", route.BackendName)
		}
		mapFiles.MapAppend(SNI, route.Host+"\t\t\t"+value)
	}
	// HTTP
	if route.Host != "" {
		mapFiles.MapAppend(HOST, route.Host+"\t\t\t"+route.Host)
	} else if route.Path.Path == "" {
		return fmt.Errorf("neither Host nor Path are provided for backend %v,", route.BackendName)
	}

	path := route.Path.Path
	switch {
	case route.Path.PathTypeMatch == store.PATH_TYPE_EXACT:
		mapFiles.MapAppend(PATH_EXACT, route.Host+path+"\t\t\t"+value)
	case path == "" || path == "/":
		mapFiles.MapAppend(PATH_PREFIX, route.Host+"/"+"\t\t\t"+value)
	case route.Path.PathTypeMatch == store.PATH_TYPE_PREFIX:
		path = strings.TrimSuffix(path, "/")
		mapFiles.MapAppend(PATH_EXACT, route.Host+path+"\t\t\t"+value)
		mapFiles.MapAppend(PATH_PREFIX, route.Host+path+"/"+"\t\t\t"+value)
	case route.Path.PathTypeMatch == store.PATH_TYPE_IMPLEMENTATION_SPECIFIC:
		path = strings.TrimSuffix(path, "/")
		mapFiles.MapAppend(PATH_EXACT, route.Host+path+"\t\t\t"+value)
		mapFiles.MapAppend(PATH_PREFIX, route.Host+path+"\t\t\t"+value)
	default:
		return fmt.Errorf("unknown path type '%s' with backend '%s'", route.Path.PathTypeMatch, route.BackendName)
	}
	return nil
}

// AddCustomRoute adds an ingress route with specific ACL via use_backend haproxy directive
func AddCustomRoute(route Route, routeACLAnn string, api api.HAProxyClient) (reload bool, err error) {
	var routeCond string
	if route.Host != "" {
		routeCond = fmt.Sprintf("{ var(txn.host) -m str %s } ", route.Host)
	}
	if route.Path.Path != "" {
		if route.Path.PathTypeMatch == store.PATH_TYPE_EXACT {
			routeCond = fmt.Sprintf("%s { path %s } ", routeCond, route.Path.Path)
		} else {
			routeCond = fmt.Sprintf("%s { path -m beg %s } ", routeCond, route.Path.Path)
		}
	}
	routeCond = fmt.Sprintf("%s { %s } ", routeCond, routeACLAnn)

	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		err = api.BackendSwitchingRuleCreate(frontend, models.BackendSwitchingRule{
			Cond:     "if",
			CondTest: routeCond,
			Name:     route.BackendName,
			Index:    utils.PtrInt64(0),
		})
		if err != nil {
			return
		}
	}
	if acl := CustomRoutes[route.BackendName]; acl != routeCond {
		CustomRoutes[route.BackendName] = routeCond
		reload = true
		logger.Debugf("Custom Route to backend '%s' added, reload required", route.BackendName)
	}
	return reload, err
}

func CustomRoutesReset(api api.HAProxyClient) (err error) {
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		err = api.BackendSwitchingRuleDeleteAll(frontend)
		if err != nil {
			break
		}
		err = api.BackendSwitchingRuleCreate(frontend, models.BackendSwitchingRule{
			Name:  "%[var(txn.path_match),field(1,.)]",
			Index: utils.PtrInt64(0),
		})
		if err != nil {
			return fmt.Errorf("unable to create main backendSwitching rule !!: %w", err)
		}
	}
	return
}
