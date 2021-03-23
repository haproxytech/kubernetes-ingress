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

package configuration

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ControllerCfg struct {
	MapFiles        *haproxy.Maps
	HAProxyRules    *haproxy.Rules
	Certificates    *haproxy.Certificates
	ActiveBackends  map[string]struct{}
	RateLimitTables []string
	FrontHTTP       string
	FrontHTTPS      string
	FrontSSL        string
	BackSSL         string
	Env             Env
	HTTPS           bool
	SSLPassthrough  bool
}

// Directories and files required by haproxy and controller
type Env struct {
	HAProxyBinary   string
	RuntimeSocket   string
	PIDFile         string
	MainCFGFile     string
	AuxCFGFile      string
	CfgDir          string
	RuntimeDir      string
	CertDir         string
	FrontendCertDir string
	BackendCertDir  string
	CaCertDir       string
	StateDir        string
	MapDir          string
	PatternDir      string
	ErrFileDir      string
	TransactionDir  string
}

// Init initialize configuration
func (c *ControllerCfg) Init() (err error) {
	c.FrontHTTP = "http"
	c.FrontHTTPS = "https"
	c.FrontSSL = "ssl"
	c.BackSSL = "ssl"
	if err = c.envInit(); err != nil {
		return err
	}
	c.MapFiles = haproxy.NewMapFiles(c.Env.MapDir)
	if err := c.haproxyRulesInit(); err != nil {
		return err
	}
	c.Certificates = haproxy.NewCertificates(c.Env.CaCertDir, c.Env.FrontendCertDir, c.Env.BackendCertDir)
	c.ActiveBackends = make(map[string]struct{})
	return nil
}

func (c *ControllerCfg) haproxyRulesInit() error {
	if c.HAProxyRules == nil {
		c.HAProxyRules = haproxy.NewRules()
	} else {
		c.HAProxyRules.Clean(c.FrontHTTP, c.FrontHTTPS, c.FrontSSL)
	}
	var errors utils.Errors
	errors.Add(
		// ForwardedProto rule
		c.HAProxyRules.AddRule(rules.SetHdr{
			ForwardedProto: true,
		}, "", c.FrontHTTPS),
		// txn.base var used for logging
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "base",
			Scope:      "txn",
			Expression: "base",
		}, "", c.FrontHTTP, c.FrontHTTPS),
		// Backend switching rules.
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "path",
			Scope:      "txn",
			Expression: "path",
		}, "", c.FrontHTTP, c.FrontHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "host",
			Scope:      "txn",
			Expression: "req.hdr(Host),field(1,:),lower",
		}, "", c.FrontHTTP, c.FrontHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "host_match",
			Scope:      "txn",
			Expression: fmt.Sprintf("var(txn.host),map(%s)", haproxy.GetMapPath(haproxy.MAP_HOST)),
		}, "", c.FrontHTTP, c.FrontHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "host_match",
			Scope:      "txn",
			Expression: fmt.Sprintf("var(txn.host),regsub(^[^.]*,,),map(%s,'')", haproxy.GetMapPath(haproxy.MAP_HOST)),
			CondTest:   "!{ var(txn.host_match) -m found }",
		}, "", c.FrontHTTP, c.FrontHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "path_match",
			Scope:      "txn",
			Expression: fmt.Sprintf("var(txn.host_match),concat(,txn.path,),map(%s)", haproxy.GetMapPath(haproxy.MAP_PATH_EXACT)),
		}, "", c.FrontHTTP, c.FrontHTTPS),
		c.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "path_match",
			Scope:      "txn",
			Expression: fmt.Sprintf("var(txn.host_match),concat(,txn.path,),map_beg(%s)", haproxy.GetMapPath(haproxy.MAP_PATH_PREFIX)),
			CondTest:   "!{ var(txn.path_match) -m found }",
		}, "", c.FrontHTTP, c.FrontHTTPS),
	)

	return errors.Result()
}

func (c *ControllerCfg) envInit() (err error) {
	for _, dir := range []string{c.Env.CfgDir, c.Env.RuntimeDir, c.Env.StateDir} {
		if dir == "" {
			return fmt.Errorf("failed to init controller config: missing config directories")
		}
	}
	// Binary and main files
	if c.Env.MainCFGFile == "" {
		c.Env.MainCFGFile = filepath.Join(c.Env.CfgDir, "haproxy.cfg")
	}
	if c.Env.AuxCFGFile == "" {
		c.Env.AuxCFGFile = filepath.Join(c.Env.CfgDir, "haproxy-aux.cfg")
	}
	if c.Env.PIDFile == "" {
		c.Env.PIDFile = filepath.Join(c.Env.RuntimeDir, "haproxy.pid")
	}
	if c.Env.RuntimeSocket == "" {
		c.Env.RuntimeSocket = filepath.Join(c.Env.RuntimeDir, "haproxy-runtime-api.sock")
	}
	for _, file := range []string{c.Env.HAProxyBinary, c.Env.MainCFGFile} {
		if _, err = os.Stat(file); err != nil {
			return err
		}
	}
	// Directories
	if c.Env.CertDir == "" {
		c.Env.CertDir = filepath.Join(c.Env.CfgDir, "certs")
	}
	c.Env.FrontendCertDir = filepath.Join(c.Env.CertDir, "frontend")
	c.Env.BackendCertDir = filepath.Join(c.Env.CertDir, "backend")
	c.Env.CaCertDir = filepath.Join(c.Env.CertDir, "ca")

	if c.Env.MapDir == "" {
		c.Env.MapDir = filepath.Join(c.Env.CfgDir, "maps")
	}
	if c.Env.PatternDir == "" {
		c.Env.PatternDir = filepath.Join(c.Env.CfgDir, "patterns")
	}
	if c.Env.ErrFileDir == "" {
		c.Env.ErrFileDir = filepath.Join(c.Env.CfgDir, "errors")
	}
	if c.Env.TransactionDir == "" {
		c.Env.TransactionDir = filepath.Join(c.Env.CfgDir, "transactions")
	}

	for _, d := range []string{c.Env.CertDir, c.Env.FrontendCertDir, c.Env.BackendCertDir, c.Env.CaCertDir, c.Env.MapDir, c.Env.ErrFileDir, c.Env.StateDir, c.Env.TransactionDir, c.Env.PatternDir} {
		err = os.MkdirAll(d, 0755)
		if err != nil {
			return err
		}
	}
	_, err = os.Create(filepath.Join(c.Env.StateDir, "global"))
	return err
}

// Clean cleans all the statuses of various data that was changed
// deletes them completely or just resets them if needed
func (c *ControllerCfg) Clean() error {
	c.RateLimitTables = []string{}
	c.ActiveBackends = make(map[string]struct{})
	c.MapFiles.Clean()
	c.Certificates.Clean()
	return c.haproxyRulesInit()
}
