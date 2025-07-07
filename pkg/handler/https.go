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
	"path"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type HTTPS struct {
	AddrIPv4  string
	AddrIPv6  string
	CertDir   string
	alpn      string
	Port      int64
	Enabled   bool
	IPv4      bool
	IPv6      bool
	strictSNI bool
}

//nolint:golint, stylecheck
const (
	HTTPS_PORT_SSLPASSTHROUGH int64 = 8444
	BIND_UNIX_SOCKET                = "unixsock"
	BIND_IP_V4                      = "v4"
	BIND_IP_V6                      = "v6"
)

func (handler *HTTPS) bindList(h haproxy.HAProxy) (binds []models.Bind) {
	if handler.IPv4 {
		binds = append(binds, models.Bind{
			Address: handler.AddrIPv4,
			Port:    utils.PtrInt64(handler.Port),
			BindParams: models.BindParams{
				Name:        BIND_IP_V4,
				AcceptProxy: false,
			},
		})
	}
	if handler.IPv6 {
		binds = append(binds, models.Bind{
			Address: handler.AddrIPv6,
			Port:    utils.PtrInt64(handler.Port),
			BindParams: models.BindParams{
				AcceptProxy: false,
				Name:        BIND_IP_V6,
				V4v6:        true,
			},
		})
	}
	return binds
}

func (handler *HTTPS) bindListPassthrough(h haproxy.HAProxy) (binds []models.Bind) {
	binds = append(binds, models.Bind{
		Address: "unix@" + handler.unixSocketPath(h),
		BindParams: models.BindParams{
			Name:        BIND_UNIX_SOCKET,
			AcceptProxy: true,
		},
	})
	return binds
}

func (handler *HTTPS) unixSocketPath(h haproxy.HAProxy) string {
	return path.Join(h.Env.RuntimeDir, "ssl-frontend.sock")
}

func (handler *HTTPS) handleClientTLSAuth(k store.K8s, h haproxy.HAProxy) (err error) {
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
				return err
			}
		}
		instance.Reload("removed client TLS authentication")
		return
	}
	// Updating config
	logger.Info("configuring client TLS authentication")
	for i := range binds {
		binds[i].SslCafile = caFile
		binds[i].Verify = verify
		if err = h.FrontendBindEdit(h.FrontHTTPS, *binds[i]); err != nil {
			return err
		}
	}
	instance.Reload("configured client TLS authentication")
	return
}

func (handler *HTTPS) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	if !handler.Enabled {
		logger.Debug("Cannot proceed with SSL Passthrough update, HTTPS is disabled")
		return nil
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
			instance.Reload("SSL offload enabled")
		}
		err := handler.handleClientTLSAuth(k, h)
		if err != nil {
			return err
		}
	} else if sslOffloadEnabled {
		logger.Panic(h.FrontendDisableSSLOffload(h.FrontHTTPS))
		instance.Reload("SSL offload disabled")
	}
	// ssl-passthrough
	_, errFtSSL := h.FrontendGet(h.FrontSSL)
	if haproxy.SSLPassthrough {
		if errFtSSL != nil || !h.BackendExists(h.BackSSL) {
			logger.Error(handler.enableSSLPassthrough(h))
			instance.Reload("SSLPassthrough enabled")
		}
		logger.Error(handler.sslPassthroughRules(k, h, a))
	} else if errFtSSL == nil {
		logger.Error(handler.disableSSLPassthrough(h))
		instance.Reload("SSLPassthrough disabled")
	}

	instance.ReloadIf(h.CertsUpdated(), "certificates updated")

	return nil
}

func (handler *HTTPS) enableSSLPassthrough(h haproxy.HAProxy) (err error) {
	// Create TCP frontend for ssl-passthrough
	frontend := models.FrontendBase{
		Name:           h.FrontSSL,
		Mode:           "tcp",
		LogFormat:      "'%ci:%cp [%t] %ft %b/%s %Tw/%Tc/%Tt %B %ts %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs SNI: %[var(sess.sni)]'",
		DefaultBackend: h.BackSSL,
	}
	err = h.FrontendCreate(frontend)
	if err != nil {
		return err
	}
	for _, b := range handler.bindList(h) {
		if err = h.FrontendBindCreate(h.FrontSSL, b); err != nil {
			return fmt.Errorf("cannot create bind for SSL Passthrough: %w", err)
		}
	}
	// Create backend for proxy chaining (chaining
	// ssl-passthrough frontend to ssl-offload backend)
	h.BackendCreatePermanently(models.Backend{
		BackendBase: models.BackendBase{
			Name: h.BackSSL,
			Mode: "tcp",
		},
	})
	var errors utils.Errors

	errors.Add(
		h.BackendServerCreateOrUpdate(h.BackSSL, models.Server{
			Name:         h.FrontHTTPS,
			Address:      "unix@" + handler.unixSocketPath(h),
			ServerParams: models.ServerParams{SendProxyV2: "enabled"},
		}),
		h.BackendSwitchingRuleCreate(0, h.FrontSSL, models.BackendSwitchingRule{
			Name: "%[var(txn.sni_match),field(1,.)]",
		}),
		handler.toggleSSLPassthrough(true, h))
	return errors.Result()
}

func (handler *HTTPS) disableSSLPassthrough(h haproxy.HAProxy) (err error) {
	err = h.FrontendDelete(h.FrontSSL)
	if err != nil {
		return err
	}
	h.DeleteFTRules(h.FrontSSL)
	h.BackendDelete(h.BackSSL)
	if err = handler.toggleSSLPassthrough(false, h); err != nil {
		return err
	}
	return nil
}

func (handler *HTTPS) toggleSSLPassthrough(passthrough bool, h haproxy.HAProxy) (err error) {
	handler.deleteHTTPSFrontendBinds(h)
	bindListFunc := handler.bindList
	if passthrough {
		bindListFunc = handler.bindListPassthrough
	}
	for _, bind := range bindListFunc(h) {
		if err = h.FrontendBindCreate(h.FrontHTTPS, bind); err != nil {
			return err
		}
	}
	if h.FrontendSSLOffloadEnabled(h.FrontHTTPS) || h.FrontCertsInUse() {
		logger.Panic(h.FrontendEnableSSLOffload(h.FrontHTTPS, handler.CertDir, handler.alpn, handler.strictSNI))
	}
	return nil
}

func (handler *HTTPS) deleteHTTPSFrontendBinds(h haproxy.HAProxy) {
	bindsToDelete := []string{BIND_IP_V4, BIND_IP_V6, BIND_UNIX_SOCKET}
	for _, bind := range bindsToDelete {
		if err := h.FrontendBindDelete(h.FrontHTTPS, bind); err != nil {
			logger.Tracef("cannot delete bind %s: %s", bind, err)
		}
	}
}

func (handler *HTTPS) sslPassthroughRules(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) error {
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
