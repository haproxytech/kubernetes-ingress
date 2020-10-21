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
	"io/ioutil"
	"os"
	"path"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type HTTPS struct {
	ipv4    bool
	ipv6    bool
	port    int64
	certDir string
}

func (h HTTPS) bindList() (binds []models.Bind) {
	if h.ipv4 {
		binds = append(binds, models.Bind{
			Address: "0.0.0.0",
			Port:    utils.PtrInt64(h.port),
			Name:    "bind_1",
		})
	}
	if h.ipv6 {
		binds = append(binds, models.Bind{
			Address: "::",
			Port:    utils.PtrInt64(h.port),
			Name:    "bind_2",
			V4v6:    true,
		})
	}
	return
}

func (h HTTPS) Update(k store.K8s, cfg Configuration, api api.HAProxyClient) (reload bool, err error) {
	if !cfg.HTTPS {
		logger.Debugf("Cannot proceed with SSL Passthrough update, HTTPS is disabled")
		return false, nil
	}
	// ssl-passthrough
	if len(cfg.BackendSwitchingRules[FrontendSSL]) > 0 {
		if !cfg.SSLPassthrough {
			logger.Info("Enabling ssl-passthrough")
			logger.Panic(h.enableSSLPassthrough(cfg, api))
			cfg.SSLPassthrough = true
			reload = true
		}
	} else if cfg.SSLPassthrough {
		logger.Info("Disabling ssl-passthrough")
		logger.Panic(h.disableSSLPassthrough(cfg, api))
		cfg.SSLPassthrough = false
		reload = true
	}
	// ssl-offload
	if len(cfg.UsedCerts) > 0 {
		if !cfg.HTTPS {
			logger.Panic(api.FrontendEnableSSLOffload(FrontendHTTPS, h.certDir, true))
			cfg.HTTPS = true
			reload = true
		}
	} else if cfg.HTTPS {
		logger.Info("Disabling ssl offload")
		logger.Panic(api.FrontendDisableSSLOffload(FrontendHTTPS))
		cfg.HTTPS = false
		reload = true
	}
	//remove certs that are not needed
	logger.Error(h.CleanCertDir(cfg.UsedCerts))

	return reload, nil
}

func (h HTTPS) enableSSLPassthrough(cfg Configuration, api api.HAProxyClient) (err error) {
	// Create TCP frontend for ssl-passthrough
	backendHTTPS := "https"
	frontend := models.Frontend{
		Name:           FrontendSSL,
		Mode:           "tcp",
		LogFormat:      "'%ci:%cp [%t] %ft %b/%s %Tw/%Tc/%Tt %B %ts %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs %[var(sess.sni)]'",
		DefaultBackend: backendHTTPS,
	}
	err = api.FrontendCreate(frontend)
	if err != nil {
		return err
	}
	for _, b := range h.bindList() {
		if err = api.FrontendBindCreate(FrontendSSL, b); err != nil {
			return fmt.Errorf("cannot create bind for SSL Passthrough: %s", err.Error())
		}
	}
	err = api.FrontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Action:   "accept",
		Type:     "content",
		Cond:     "if",
		CondTest: "{ req_ssl_hello_type 1 }",
	})
	if err != nil {
		return err
	}
	err = api.FrontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Action:   "set-var",
		VarName:  "sni",
		VarScope: "sess",
		Expr:     "req_ssl_sni",
		Type:     "content",
	})
	if err != nil {
		return err
	}
	err = api.FrontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Type:    "inspect-delay",
		Index:   utils.PtrInt64(0),
		Timeout: utils.PtrInt64(5000),
	})
	if err != nil {
		return err
	}
	// Create backend for proxy chaining (chaining
	// ssl-passthrough frontend to ssl-offload backend)
	err = api.BackendCreate(models.Backend{
		Name: backendHTTPS,
		Mode: "tcp",
	})
	if err != nil {
		return err
	}
	err = api.BackendServerCreate(backendHTTPS, models.Server{
		Name:        FrontendHTTPS,
		Address:     "127.0.0.1:8443",
		SendProxyV2: "enabled",
	})
	if err != nil {
		return err
	}

	if err = h.bindSSLPassthrough(true, api); err != nil {
		return err
	}
	// Some TCP rules depend on ssl-passthrough
	cfg.FrontendRulesModified[TCP] = true
	return nil
}

func (h HTTPS) disableSSLPassthrough(cfg Configuration, api api.HAProxyClient) (err error) {
	backendHTTPS := "https"
	err = api.FrontendDelete(FrontendSSL)
	if err != nil {
		return err
	}
	err = api.BackendDelete(backendHTTPS)
	if err != nil {
		return err
	}
	if err = h.bindSSLPassthrough(false, api); err != nil {
		return err
	}
	// Some TCP rules depend on ssl-passthrough
	cfg.FrontendRulesModified[TCP] = true
	return nil
}

// Upon SSL passthrough we have to select a hard-coded port number (8443):
// this could collide if running the Ingress Controller in HostNetwork mode.
// TODO(prometherion): making this configurable
func (h HTTPS) bindSSLPassthrough(enabled bool, api api.HAProxyClient) (err error) {
	for _, bind := range h.bindList() {
		bind.Port = utils.PtrInt64(8443)
		bind.AcceptProxy = enabled
		if err = api.FrontendBindEdit(FrontendHTTPS, bind); err != nil {
			return err
		}
	}
	return nil
}

func (h HTTPS) CleanCertDir(usedCerts map[string]struct{}) error {
	files, err := ioutil.ReadDir(HAProxyCertDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		filename := path.Join(HAProxyCertDir, f.Name())
		_, isOK := usedCerts[filename]
		if !isOK {
			os.Remove(filename)
		}
	}
	return nil
}
