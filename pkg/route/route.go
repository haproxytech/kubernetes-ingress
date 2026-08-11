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
	"errors"
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

//nolint:golint,stylecheck
const (
	// Main frontends
	FrontendHTTP  = "http"
	FrontendHTTPS = "https"
	// Routing Maps
	SNI               maps.Name = "sni"
	HOST              maps.Name = "host"
	PATH_EXACT        maps.Name = "path-exact"
	PATH_PREFIX_EXACT maps.Name = "path-prefix-exact"
	PATH_PREFIX       maps.Name = "path-prefix"
)

var (
	CurentCustomRoutes = make([]string, 0)
	CustomRoutes       = make([]string, 0)
)

type Route struct {
	Path           *store.IngressPath
	Host           string
	BackendName    string
	HAProxyRules   []rules.RuleID
	SSLPassthrough bool
}

// Row is one line of a map file: the map it belongs to, the key haproxy looks up and the
// value it resolves to.
type Row struct {
	Map   maps.Name
	Key   string
	Value string
}

// Line returns the row as written in the map file.
func (r Row) Line() string {
	return r.Key + "\t\t\t" + r.Value
}

// MapRows returns the map rows a route is made of.
//
// It is the single description of what routing a path means in terms of map files, so that
// anything needing to reason about a route before it is written - a collision on a key, for
// instance - reasons about the rows haproxy will actually look up rather than about a
// reconstruction of them.
func MapRows(route Route) ([]Row, error) {
	if route.BackendName == "" {
		return nil, errors.New("backendName missing")
	}
	// Wildcard host
	if route.Host != "" && route.Host[0] == '*' {
		route.Host = route.Host[1:]
	}
	value := route.BackendName
	for _, id := range route.HAProxyRules {
		value += "." + string(id)
	}
	rows := make([]Row, 0, 3)
	// SSLPassthrough
	if route.SSLPassthrough {
		if route.Host == "" {
			return nil, fmt.Errorf("empty SNI for backend %s,", route.BackendName)
		}
		rows = append(rows, Row{Map: SNI, Key: route.Host, Value: value})
	}
	// HTTP
	if route.Host != "" {
		rows = append(rows, Row{Map: HOST, Key: route.Host, Value: route.Host})
	} else if route.Path.Path == "" {
		return nil, fmt.Errorf("neither Host nor Path are provided for backend %v,", route.BackendName)
	}

	path := route.Path.Path
	switch {
	case route.Path.PathTypeMatch == store.PATH_TYPE_EXACT:
		rows = append(rows, Row{Map: PATH_EXACT, Key: route.Host + path, Value: value})
	case path == "" || path == "/":
		rows = append(rows, Row{Map: PATH_PREFIX, Key: route.Host + "/", Value: value})
	case route.Path.PathTypeMatch == store.PATH_TYPE_PREFIX:
		path = strings.TrimSuffix(path, "/")
		rows = append(rows,
			Row{Map: PATH_PREFIX_EXACT, Key: route.Host + path, Value: value},
			Row{Map: PATH_PREFIX, Key: route.Host + path + "/", Value: value})
	case route.Path.PathTypeMatch == store.PATH_TYPE_IMPLEMENTATION_SPECIFIC:
		path = strings.TrimSuffix(path, "/")
		rows = append(rows,
			Row{Map: PATH_PREFIX_EXACT, Key: route.Host + path, Value: value},
			Row{Map: PATH_PREFIX, Key: route.Host + path, Value: value})
	default:
		return nil, fmt.Errorf("unknown path type '%s' with backend '%s'", route.Path.PathTypeMatch, route.BackendName)
	}
	return rows, nil
}

// AddHostPathRoute adds Host/Path ingress route to haproxy Map files used for backend switching.
func AddHostPathRoute(route Route, mapFiles maps.Maps) error {
	rows, err := MapRows(route)
	if err != nil {
		return err
	}
	for _, row := range rows {
		mapFiles.MapAppend(row.Map, row.Line())
	}
	return nil
}

// AddCustomRoute adds an ingress route with specific ACL via use_backend haproxy directive
func AddCustomRoute(route Route, routeACLAnn string, api api.HAProxyClient) (err error) {
	var routeCond string
	if route.Host != "" {
		if route.Host[0] == '*' {
			// Wildcard host - use suffix matching
			routeCond = fmt.Sprintf("{ var(txn.host) -m end %s } ", route.Host[1:])
		} else {
			// Regular host - use string matching
			routeCond = fmt.Sprintf("{ var(txn.host) -m str %s } ", route.Host)
		}
	}
	if route.Path.Path != "" {
		if route.Path.PathTypeMatch == store.PATH_TYPE_EXACT {
			routeCond = fmt.Sprintf("%s{ path %s }", routeCond, route.Path.Path)
		} else {
			if route.Path.Path == "/" {
				routeCond = fmt.Sprintf("%s{ path -m beg %s }", routeCond, route.Path.Path)
			} else {
				path := strings.TrimSuffix(route.Path.Path, "/")
				routeCond = fmt.Sprintf("%s{ path -m reg ^%s($|/) }", routeCond, path)
			}
		}
	}
	routeCond = fmt.Sprintf("%s { %s } ", routeCond, routeACLAnn)

	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		err = api.BackendSwitchingRuleCreate(0, frontend, models.BackendSwitchingRule{
			Cond:     "if",
			CondTest: routeCond,
			Name:     route.BackendName,
		})
		if err != nil {
			return err
		}
	}

	CustomRoutes = append(CustomRoutes, routeCond)
	return err
}

func CustomRoutesReset(api api.HAProxyClient) (err error) {
	for _, frontend := range []string{FrontendHTTP, FrontendHTTPS} {
		err = api.BackendSwitchingRuleDeleteAll(frontend)
		if err != nil {
			break
		}
		err = api.BackendSwitchingRuleCreate(0, frontend, models.BackendSwitchingRule{
			Name: "%[var(txn.path_match),field(1,.)]",
		})
		if err != nil {
			return fmt.Errorf("unable to create main backendSwitching rule !!: %w", err)
		}
	}
	CustomRoutes = make([]string, 0)
	return err
}
