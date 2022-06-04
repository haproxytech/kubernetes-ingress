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

package handler

import (
	"errors"
	"fmt"

	"github.com/haproxytech/client-native/v3/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type HTTPS struct {
	Enabled   bool
	IPv4      bool
	IPv6      bool
	strictSNI bool
	Port      int64
	AddrIPv4  string
	AddrIPv6  string
	CertDir   string
	alpn      string
}

func (handler HTTPS) bindList(passhthrough bool) (binds []models.Bind) {
	if handler.IPv4 {
		binds = append(binds, models.Bind{
			Address: func() (addr string) {
				addr = handler.AddrIPv4
				if passhthrough {
					addr = "127.0.0.1"
				}
				return
			}(),
			Port: utils.PtrInt64(handler.Port),
			BindParams: models.BindParams{
				Name:        "v4",
				AcceptProxy: passhthrough,
			},
		})
	}
	if handler.IPv6 {
		binds = append(binds, models.Bind{
			Address: func() (addr string) {
				addr = handler.AddrIPv6
				if passhthrough {
					addr = "::1"
				}
				return
			}(),
			Port: utils.PtrInt64(handler.Port),
			BindParams: models.BindParams{
				AcceptProxy: passhthrough,
				Name:        "v6",
				V4v6:        true,
			},
		})
	}
	return binds
}

func (handler HTTPS) handleClientTLSAuth(k store.K8s, h haproxy.HAProxy) (reload bool, err error) {
	// Parsing
	var caFile string
	var notFound store.ErrNotFound
	secret, annErr := annotations.Secret("client-ca", "", k, k.ConfigMaps.Main.Annotations)
	if annErr != nil {
		if errors.Is(annErr, notFound) {
			logger.Warningf("client TLS Auth: %s", annErr)
		} else {
			err = fmt.Errorf("client TLS Auth: %w", annErr)
			return
		}
	}
	if secret != nil {
		caFile, err = h.Certificates.AddSecret(secret, certs.CA_CERT)
		if err != nil {
			err = fmt.Errorf("client TLS Auth: %w", err)
			return
		}
	}

	binds, bindsErr := h.FrontendBindsGet(h.FrontHTTPS)
	if bindsErr != nil {
		err = fmt.Errorf("client TLS Auth: %w", bindsErr)
		return
	}

	var enabled bool
	verify := "required"
	enabled, annErr = annotations.Bool("client-crt-optional", k.ConfigMaps.Main.Annotations)
	logger.Error(annErr)
	if enabled {
		verify = "optional"
	}

	// No changes
	if binds[0].SslCafile == caFile && (caFile == "" || binds[0].Verify == verify) {
		return
	}
	// Removing config
	if caFile == "" {
		logger.Info("removing client TLS authentication")
		for i := range binds {
			binds[i].SslCafile = ""
			binds[i].Verify = ""
			if err = h.FrontendBindEdit(h.FrontHTTPS, *binds[i]); err != nil {
				return false, err
			}
		}
		reload = true
		return
	}
	// Updating config
	logger.Info("configuring client TLS authentication")
	for i := range binds {
		binds[i].SslCafile = caFile
		binds[i].Verify = verify
		if err = h.FrontendBindEdit(h.FrontHTTPS, *binds[i]); err != nil {
			return false, err
		}
	}
	reload = true
	return
}

func (handler HTTPS) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (reload bool, err error) {
	if !handler.Enabled {
		logger.Debug("Cannot proceed with SSL Passthrough update, HTTPS is disabled")
		return false, nil
	}

	// Fetch tls-alpn value for when SSL offloading is enabled
	handler.alpn = a.String("tls-alpn", k.ConfigMaps.Main.Annotations)

	handler.strictSNI, err = annotations.Bool("client-strict-sni", k.ConfigMaps.Main.Annotations)
	logger.Error(err)

	// ssl-offload
	sslOffloadEnabled := h.FrontendSSLOffloadEnabled(h.FrontHTTPS)
	if h.FrontCertsInUse() {
		if !sslOffloadEnabled {
			logger.Panic(h.FrontendEnableSSLOffload(h.FrontHTTPS, handler.CertDir, handler.alpn, handler.strictSNI))
			reload = true
			logger.Debug("SSLOffload enabled, reload required")
		}
		r, err := handler.handleClientTLSAuth(k, h)
		if err != nil {
			return r, err
		}
		reload = reload || r
	} else if sslOffloadEnabled {
		logger.Panic(h.FrontendDisableSSLOffload(h.FrontHTTPS))
		reload = true
		logger.Debug("SSLOffload disabled, reload required")
	}
	// ssl-passthrough
	_, errFtSSL := h.FrontendGet(h.FrontSSL)
	_, errBdSSL := h.BackendGet(h.BackSSL)
	if haproxy.SSLPassthrough {
		if errFtSSL != nil || errBdSSL != nil {
			logger.Error(handler.enableSSLPassthrough(h))
			reload = true
			logger.Debug("SSLPassthrough enabled, reload required")
		}
		logger.Error(handler.sslPassthroughRules(k, h, a))
	} else if errFtSSL == nil {
		logger.Error(handler.disableSSLPassthrough(h))
		reload = true
		logger.Debug("SSLPassthrough disabled, reload required")
	}
	if h.CertsUpdated() {
		reload = true
	}

	return reload, nil
}

func (handler HTTPS) enableSSLPassthrough(h haproxy.HAProxy) (err error) {
	// Create TCP frontend for ssl-passthrough
	frontend := models.Frontend{
		Name:           h.FrontSSL,
		Mode:           "tcp",
		LogFormat:      "'%ci:%cp [%t] %ft %b/%s %Tw/%Tc/%Tt %B %ts %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs SNI: %[var(sess.sni)]'",
		DefaultBackend: h.BackSSL,
	}
	err = h.FrontendCreate(frontend)
	if err != nil {
		return err
	}
	for _, b := range handler.bindList(false) {
		if err = h.FrontendBindCreate(h.FrontSSL, b); err != nil {
			return fmt.Errorf("cannot create bind for SSL Passthrough: %w", err)
		}
	}
	// Create backend for proxy chaining (chaining
	// ssl-passthrough frontend to ssl-offload backend)
	var errors utils.Errors
	errors.Add(
		h.BackendCreate(models.Backend{
			Name: h.BackSSL,
			Mode: "tcp",
		}),
		h.BackendServerCreate(h.BackSSL, models.Server{
			Name:        h.FrontHTTPS,
			Address:     "127.0.0.1",
			Port:        utils.PtrInt64(handler.Port),
			SendProxyV2: "enabled",
		}),
		h.BackendSwitchingRuleCreate(h.FrontSSL, models.BackendSwitchingRule{
			Index: utils.PtrInt64(0),
			Name:  fmt.Sprintf("%%[var(txn.sni_match),field(1,.)]"),
		}),
		handler.toggleSSLPassthrough(true, h))
	return errors.Result()
}

func (handler HTTPS) disableSSLPassthrough(h haproxy.HAProxy) (err error) {
	err = h.FrontendDelete(h.FrontSSL)
	if err != nil {
		return err
	}
	h.DeleteFTRules(h.FrontSSL)
	err = h.BackendDelete(h.BackSSL)
	if err != nil {
		return err
	}
	if err = handler.toggleSSLPassthrough(false, h); err != nil {
		return err
	}
	return nil
}

func (handler HTTPS) toggleSSLPassthrough(passthrough bool, h haproxy.HAProxy) (err error) {
	for _, bind := range handler.bindList(passthrough) {
		if err = h.FrontendBindEdit(h.FrontHTTPS, bind); err != nil {
			return err
		}
	}
	if h.FrontendSSLOffloadEnabled(h.FrontHTTPS) {
		logger.Panic(h.FrontendEnableSSLOffload(h.FrontHTTPS, handler.CertDir, handler.alpn, handler.strictSNI))
	}
	return nil
}

func (handler HTTPS) sslPassthroughRules(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) error {
	inspectTimeout, err := a.Timeout("timeout-client", k.ConfigMaps.Main.Annotations)
	if inspectTimeout == nil {
		if err != nil {
			logger.Errorf("SSL Passthrough: %s", err)
		}
		inspectTimeout = utils.PtrInt64(5000)
	}
	errors := utils.Errors{}
	errors.Add(h.Rules.AddRule(h.FrontSSL, rules.ReqAcceptContent{}, false),
		h.Rules.AddRule(h.FrontSSL, rules.ReqInspectDelay{
			Timeout: inspectTimeout,
		}, false),
		h.AddRule(h.FrontSSL, rules.ReqSetVar{
			Name:       "sni",
			Scope:      "sess",
			Expression: "req_ssl_sni",
		}, false),
		h.AddRule(h.FrontSSL, rules.ReqSetVar{
			Name:       "sni_match",
			Scope:      "txn",
			Expression: fmt.Sprintf("req_ssl_sni,map(%s)", maps.GetPath(route.SNI)),
		}, false),
		h.AddRule(h.FrontSSL, rules.ReqSetVar{
			Name:       "sni_match",
			Scope:      "txn",
			Expression: fmt.Sprintf("req_ssl_sni,regsub(^[^.]*,,),map(%s)", maps.GetPath(route.SNI)),
			CondTest:   "!{ var(txn.sni_match) -m found }",
		}, false),
	)
	return errors.Result()
}
