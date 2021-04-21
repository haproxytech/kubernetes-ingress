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
	"errors"
	"fmt"

	"github.com/haproxytech/models/v2"

	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type HTTPS struct {
	enabled  bool
	ipv4     bool
	ipv6     bool
	port     int64
	addrIpv4 string
	addrIpv6 string
	certDir  string
}

func (h HTTPS) bindList(passhthrough bool) (binds []models.Bind) {
	if h.ipv4 {
		binds = append(binds, models.Bind{
			Address: func() (addr string) {
				addr = h.addrIpv4
				if passhthrough {
					addr = "127.0.0.1"
				}
				return
			}(),
			Port:        utils.PtrInt64(h.port),
			Name:        "v4",
			AcceptProxy: passhthrough,
		})
	}
	if h.ipv6 {
		binds = append(binds, models.Bind{
			Address: func() (addr string) {
				addr = h.addrIpv6
				if passhthrough {
					addr = "::1"
				}
				return
			}(),
			Port:        utils.PtrInt64(h.port),
			AcceptProxy: passhthrough,
			Name:        "v6",
			V4v6:        true,
		})
	}
	return binds
}

func (h HTTPS) handleClientTLSAuth(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	annTLSAuth, _ := k.GetValueFromAnnotations("client-ca", k.ConfigMaps.Main.Annotations)
	if annTLSAuth == nil {
		return false, nil
	}
	binds, err := api.FrontendBindsGet(cfg.FrontHTTPS)
	if err != nil {
		return false, err
	}
	caFile, secretUpdated, secretErr := cfg.Certificates.HandleTLSSecret(k, haproxy.SecretCtx{
		DefaultNS:  "",
		SecretPath: annTLSAuth.Value,
		SecretType: haproxy.CA_CERT,
	})
	// Annotation or secret DELETED
	if annTLSAuth.Status == DELETED || (secretUpdated && caFile == "") {
		logger.Infof("removing client TLS authentication")
		for i := range binds {
			binds[i].SslCafile = ""
			binds[i].Verify = ""
			if err = api.FrontendBindEdit(cfg.FrontHTTPS, *binds[i]); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	// Handle secret errors
	if secretErr != nil {
		if errors.Is(secretErr, haproxy.ErrCertNotFound) {
			logger.Warning("unable to configure TLS authentication secret '%s' not found", annTLSAuth.Value)
			return false, nil
		}
		return false, secretErr
	}
	// No changes
	if annTLSAuth.Status == EMPTY && !secretUpdated {
		return false, nil
	}
	// Configure TLS Authentication
	logger.Infof("enabling client TLS authentication")
	for i := range binds {
		binds[i].SslCafile = caFile
		binds[i].Verify = "required"
		if err = api.FrontendBindEdit(cfg.FrontHTTPS, *binds[i]); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (h HTTPS) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	if !h.enabled {
		logger.Debugf("Cannot proceed with SSL Passthrough update, HTTPS is disabled")
		return false, nil
	}
	// ssl-offload
	if cfg.Certificates.FrontendCertsEnabled() {
		if !cfg.HTTPS {
			logger.Panic(api.FrontendEnableSSLOffload(cfg.FrontHTTPS, h.certDir, true))
			cfg.HTTPS = true
			reload = true
			logger.Debug("SSLOffload enabeld, reload required")
		}
		r, err := h.handleClientTLSAuth(k, cfg, api)
		if err != nil {
			return r, err
		}
		reload = reload || r
	} else if cfg.HTTPS {
		logger.Panic(api.FrontendDisableSSLOffload(cfg.FrontHTTPS))
		cfg.HTTPS = false
		reload = true
		logger.Debug("SSLOffload disabled, reload required")
	}
	// ssl-passthrough
	_, errFtSSL := api.FrontendGet(cfg.FrontSSL)
	if cfg.SSLPassthrough {
		if errFtSSL != nil {
			logger.Error(h.enableSSLPassthrough(cfg, api))
			cfg.SSLPassthrough = true
			reload = true
			logger.Debug("SSLPassthrough enabled, reload required")
		}
		logger.Error(h.sslPassthroughRules(k, cfg))
	} else if errFtSSL == nil {
		logger.Error(h.disableSSLPassthrough(cfg, api))
		cfg.SSLPassthrough = false
		reload = true
		logger.Debug("SSLPassthrough disabled, reload required")
	}

	return reload, nil
}

func (h HTTPS) enableSSLPassthrough(cfg *config.ControllerCfg, api api.HAProxyClient) (err error) {
	// Create TCP frontend for ssl-passthrough
	frontend := models.Frontend{
		Name:           cfg.FrontSSL,
		Mode:           "tcp",
		LogFormat:      "'%ci:%cp [%t] %ft %b/%s %Tw/%Tc/%Tt %B %ts %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs haproxy.MAP_SNI: %[var(sess.sni)]'",
		DefaultBackend: cfg.BackSSL,
	}
	err = api.FrontendCreate(frontend)
	if err != nil {
		return err
	}
	for _, b := range h.bindList(false) {
		if err = api.FrontendBindCreate(cfg.FrontSSL, b); err != nil {
			return fmt.Errorf("cannot create bind for SSL Passthrough: %w", err)
		}
	}
	// Create backend for proxy chaining (chaining
	// ssl-passthrough frontend to ssl-offload backend)
	var errors utils.Errors
	errors.Add(
		api.BackendCreate(models.Backend{
			Name: cfg.BackSSL,
			Mode: "tcp",
		}),
		api.BackendServerCreate(cfg.BackSSL, models.Server{
			Name:        cfg.FrontHTTPS,
			Address:     "127.0.0.1",
			Port:        utils.PtrInt64(h.port),
			SendProxyV2: "enabled",
		}),
		api.BackendSwitchingRuleCreate(cfg.FrontSSL, models.BackendSwitchingRule{
			Index: utils.PtrInt64(0),
			Name:  fmt.Sprintf("%%[var(txn.sni_match),field(1,.)]"),
		}),
		h.toggleSSLPassthrough(true, cfg, api))
	return errors.Result()
}

func (h HTTPS) disableSSLPassthrough(cfg *config.ControllerCfg, api api.HAProxyClient) (err error) {
	err = api.FrontendDelete(cfg.FrontSSL)
	if err != nil {
		return err
	}
	cfg.HAProxyRules.DeleteFrontend(cfg.FrontSSL)
	err = api.BackendDelete(cfg.BackSSL)
	if err != nil {
		return err
	}
	if err = h.toggleSSLPassthrough(false, cfg, api); err != nil {
		return err
	}
	return nil
}

func (h HTTPS) toggleSSLPassthrough(passthrough bool, cfg *config.ControllerCfg, api api.HAProxyClient) (err error) {
	for _, bind := range h.bindList(passthrough) {
		if err = api.FrontendBindEdit(cfg.FrontHTTPS, bind); err != nil {
			return err
		}
	}
	if cfg.HTTPS {
		logger.Panic(api.FrontendEnableSSLOffload(cfg.FrontHTTPS, h.certDir, true))
	}
	return nil
}

func (h HTTPS) sslPassthroughRules(k store.K8s, cfg *config.ControllerCfg) error {
	inspectTimeout := utils.PtrInt64(5000)
	annTimeout, _ := k.GetValueFromAnnotations("timeout-client", k.ConfigMaps.Main.Annotations)
	if annTimeout != nil {
		if value, errParse := utils.ParseTime(annTimeout.Value); errParse == nil {
			inspectTimeout = value
		} else {
			logger.Error(errParse)
		}
	}
	errors := utils.Errors{}
	errors.Add(cfg.HAProxyRules.AddRule(rules.ReqAcceptContent{}, "", cfg.FrontSSL),
		cfg.HAProxyRules.AddRule(rules.ReqInspectDelay{
			Timeout: inspectTimeout,
		}, "", cfg.FrontSSL),
		cfg.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "sni",
			Scope:      "sess",
			Expression: "req_ssl_sni",
		}, "", cfg.FrontSSL),
		cfg.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "sni_match",
			Scope:      "txn",
			Expression: fmt.Sprintf("req_ssl_sni,map(%s)", haproxy.GetMapPath(haproxy.MAP_SNI)),
		}, "", cfg.FrontSSL),
		cfg.HAProxyRules.AddRule(rules.ReqSetVar{
			Name:       "sni_match",
			Scope:      "txn",
			Expression: fmt.Sprintf("req_ssl_sni,regsub(^[^.]*,,),map(%s)", haproxy.GetMapPath(haproxy.MAP_SNI)),
			CondTest:   "!{ var(txn.sni_match) -m found }",
		}, "", cfg.FrontSSL),
	)
	return errors.Result()
}
