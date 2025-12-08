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

//go:build e2e_sequential

package crd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	parser "github.com/haproxytech/client-native/v5/config-parser"
	"github.com/haproxytech/client-native/v5/config-parser/common"
	"github.com/haproxytech/client-native/v5/config-parser/options"
	"github.com/haproxytech/client-native/v5/config-parser/params"
	"github.com/haproxytech/client-native/v5/config-parser/types"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
	"github.com/stretchr/testify/suite"
)

// Adding GlobalSuite, just to be able to debug directly here and not from CRDSuite
type GlobalSuite struct {
	CRDSuite
}

func TestGlobalSuite(t *testing.T) {
	suite.Run(t, new(GlobalSuite))
}

func (suite *GlobalSuite) Test_CR_Global() {
	suite.Run("CRs OK", func() {
		initialVersion := suite.getVersion()

		manifest := "config/cr/global-full.yaml"
		suite.Require().NoError(suite.test.Apply(manifest, "", nil))
		// Wait for version to be incremented
		suite.Eventually(func() bool {
			versionUpdated := suite.getVersion()
			return versionUpdated != initialVersion

			// Get updated config and do all needed param checks
			cfg, err := suite.test.GetIngressControllerFile("/etc/haproxy/haproxy.cfg")
			if err != nil {
				suite.T().Log(err)
				return false
			}
			reader := strings.NewReader(cfg)
			p, err := parser.New(options.Reader(reader))
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "default-path", &types.DefaultPath{
				Type: "config",
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "cpu-map", []types.CPUMap{
				{
					Process: "1",
					CPUSet:  "1/1",
				},
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "daemon", &types.Enabled{})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "group", &types.StringC{Value: "root"})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "user", &types.StringC{Value: "root"})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "h1-case-adjust", []types.H1CaseAdjust{
				{
					From: "content-length",
					To:   "Content-length",
				},
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "httpclient.resolvers.disabled", &types.StringC{Value: "on"})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "httpclient.resolvers.prefer", &types.HTTPClientResolversPrefer{
				Type: "ipv6",
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "httpclient.ssl.verify", &types.HTTPClientSSLVerify{
				Type: "none",
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			// log-server-state-from-file
			// CN: models/global.go: LoadServerStateFromFile not implemented
			// not serialized in SerializeGlobalSection
			// # https://gitlab.int.haproxy.com/haproxy-controller/gophers/-/issues/1488
			// Check to do when fix is done in client-native
			err = suite.checkGlobalParam(p, "log-send-hostname", &types.StringC{Value: "aword"})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "lua-prepend-path", []types.LuaPrependPath{
				{
					Path: "aword",
					Type: "cpath",
				},
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "maxconn", &types.Int64C{Value: 1007})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "numa-cpu-mapping", &types.NumaCPUMapping{
				NoOption: false,
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "presetenv", []types.StringKeyValueC{
				{
					Key:   "test1",
					Value: "test1",
				},
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "profiling.tasks", &types.StringC{Value: "auto"})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "stats socket", []types.Socket{
				{
					Path: "/var/run/haproxy-runtime-api.sock",
					Params: []params.BindOption{
						&params.BindOptionDoubleWord{
							Name:  "expose-fd",
							Value: "listeners",
						},
						&params.BindOptionValue{
							Name:  "level",
							Value: "admin",
						},
					},
				},
				{
					Path: "0.0.0.0:31025",
					Params: []params.BindOption{
						&params.BindOptionValue{
							Name:  "name",
							Value: "aword",
						},
						&params.BindOptionValue{
							Name:  "tcp-ut",
							Value: "0",
						},
						&params.BindOptionValue{
							Name:  "verify",
							Value: "optional",
						},
						&params.BindOptionValue{
							Name:  "alpn",
							Value: "aword",
						},
						&params.BindOptionValue{
							Name:  "level",
							Value: "admin",
						},
						&params.BindOptionValue{
							Name:  "severity-output",
							Value: "none",
						},
						&params.BindOptionValue{
							Name:  "maxconn",
							Value: "10005",
						},
						&params.BindOptionValue{
							Name:  "ssl-max-ver",
							Value: "SSLv3",
						},
						&params.BindOptionValue{
							Name:  "ssl-min-ver",
							Value: "TLSv1.1",
						},
						&params.BindOptionValue{
							Name:  "thread",
							Value: "all",
						},
						&params.BindOptionValue{
							Name:  "quic-cc-algo",
							Value: "newreno",
						},
						&params.BindOptionWord{
							Name: "quic-force-retry",
						},
					},
				},
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "set-var", []types.SetVar{
				{
					Expr: common.Expression{Expr: []string{"int(100)"}},
					Name: "proc.test2",
				},
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "set-var-fmt", []types.SetVarFmt{
				{
					Format: "primary",
					Name:   "proc.current_state",
				},
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "setcap", &types.StringC{Value: "cap_net_bind_service"})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			// err = suite.checkGlobalParam(p, "ssl-mode-async", &types.SslModeAsync{})
			err = suite.checkGlobalParam(p, "ssl-server-verify", &types.StringC{Value: "none"})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "stats maxconn", &types.Int64C{Value: 10008})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "stats timeout", &types.StringC{Value: "6000"})
			if err != nil {
				suite.T().Log(err)
				return false
			}

			err = suite.checkGlobalParam(p, "tune.buffers.reserve", &types.Int64C{Value: 3})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "tune.http.maxhdr", &types.Int64C{Value: 100})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "tune.listener.default-shards", &types.StringC{Value: "by-process"})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "tune.quic.frontend.conn-tx-buffers.limit", &types.Int64C{Value: 7})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "tune.quic.frontend.max-streams-bidi", &types.Int64C{Value: 8})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "tune.quic.max-frame-loss", &types.Int64C{Value: 10})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "tune.quic.retry-threshold", &types.Int64C{Value: 10})
			if err != nil {
				suite.T().Log(err)
				return false
			}
			err = suite.checkGlobalParam(p, "tune.quic.socket-owner", &types.QuicSocketOwner{Owner: "connection"})
			if err != nil {
				suite.T().Log(err)
				return false
			}

			err = suite.checkLogTargetParam(p, "log", types.Log{
				Address:  "1.2.3.4",
				Facility: "mail",
				Format:   "rfc3164",
				Level:    "emerg",
				MinLevel: "debug",
			})
			if err != nil {
				suite.T().Log(err)
				return false
			}

			return true
		}, e2e.WaitDuration, e2e.TickDuration)
		suite.Require().NoError(suite.test.Delete(manifest))
	})
}

func (suite *GlobalSuite) checkGlobalParam(p parser.Parser, param string, value common.ParserData) error {
	v, err := p.Get(parser.Global, parser.GlobalSectionName, param)
	if err != nil {
		return err
	}
	if !cmp.Equal(v, value) {
		return fmt.Errorf("Global param %s should be equal to %v but is %v", param, value, v)
	}
	return nil
}

func (suite *GlobalSuite) checkLogTargetParam(p parser.Parser, param string, value common.ParserData) error {
	v, err := p.GetOne(parser.Global, parser.GlobalSectionName, param, 0)
	if err != nil {
		return err
	}
	if !cmp.Equal(v, value) {
		return fmt.Errorf("Global param %s should be equal to %v but is %v", param, value, v)
	}
	return nil
}
