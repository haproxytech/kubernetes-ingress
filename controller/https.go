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
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

func (c *HAProxyController) cleanCertDir(usedCerts map[string]struct{}) error {
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

func (c *HAProxyController) handleTLSSecret(ingress store.Ingress, tls store.IngressTLS, certs map[string]struct{}) (reload bool) {
	secretData := strings.Split(tls.SecretName.Value, "/")
	namespaceName := ingress.Namespace
	var secretName string
	if len(secretData) > 1 {
		namespaceName = secretData[0]
		secretName = secretData[1]
	} else {
		secretName = secretData[0] // only secretname is here
	}
	namespace, namespaceOK := c.Store.Namespaces[namespaceName]
	if !namespaceOK {
		if tls.Status != EMPTY {
			c.Logger.Warningf("namespace [%s] does not exist, ignoring.", namespaceName)
		}
		return false
	}
	secret, secretOK := namespace.Secret[secretName]
	if !secretOK {
		if tls.Status != EMPTY {
			c.Logger.Warningf("secret [%s/%s] does not exist, ignoring.", namespaceName, secretName)
		}
		return false
	}
	if secret.Status == DELETED || tls.Status == DELETED {
		return false
	}
	writeSecret := true
	if secret.Status == EMPTY && tls.Status == EMPTY {
		writeSecret = false
	}
	reload, _ = HandleSecret(ingress, *secret, writeSecret, certs, c.Logger)
	return reload
}

func (c *HAProxyController) handleHTTPS(usedCerts map[string]struct{}) (reload bool) {
	// ssl-passthrough
	if len(c.cfg.BackendSwitchingRules[FrontendSSL]) > 0 {
		if !c.cfg.SSLPassthrough {
			c.Logger.Info("Enabling ssl-passthrough")
			c.Logger.Panic(c.enableSSLPassthrough())
			c.cfg.SSLPassthrough = true
			reload = true
		}
	} else if c.cfg.SSLPassthrough {
		c.Logger.Info("Disabling ssl-passthrough")
		c.Logger.Panic(c.disableSSLPassthrough())
		c.cfg.SSLPassthrough = false
		reload = true
	}
	// ssl-offload
	if len(usedCerts) > 0 {
		if !c.cfg.HTTPS {
			c.Logger.Panic(c.enableSSLOffload(FrontendHTTPS, true))
			c.cfg.HTTPS = true
			reload = true
		}
	} else if c.cfg.HTTPS {
		c.Logger.Info("Disabling ssl offload")
		c.Logger.Panic(c.disableSSLOffload(FrontendHTTPS))
		c.cfg.HTTPS = false
		reload = true
	}
	//remove certs that are not needed
	c.Logger.Error(c.cleanCertDir(usedCerts))

	return reload
}

func (c *HAProxyController) enableSSLOffload(frontendName string, alpn bool) (err error) {
	binds, err := c.Client.FrontendBindsGet(frontendName)
	if err != nil {
		return err
	}
	for _, bind := range binds {
		bind.Ssl = true
		bind.SslCertificate = HAProxyCertDir
		if alpn {
			bind.Alpn = "h2,http/1.1"
		}
		err = c.Client.FrontendBindEdit(frontendName, *bind)
	}
	if err != nil {
		return err
	}
	return err
}

func (c *HAProxyController) disableSSLOffload(frontendName string) (err error) {
	binds, err := c.Client.FrontendBindsGet(frontendName)
	if err != nil {
		return err
	}
	for _, bind := range binds {
		bind.Ssl = false
		bind.SslCertificate = ""
		bind.Alpn = ""
		err = c.Client.FrontendBindEdit(frontendName, *bind)
	}
	if err != nil {
		return err
	}
	return err
}

func (c *HAProxyController) enableSSLPassthrough() (err error) {
	// Create TCP frontend for ssl-passthrough
	backendHTTPS := "https"
	frontend := models.Frontend{
		Name:           FrontendSSL,
		Mode:           "tcp",
		LogFormat:      "'%ci:%cp [%t] %ft %b/%s %Tw/%Tc/%Tt %B %ts %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs %[var(sess.sni)]'",
		DefaultBackend: backendHTTPS,
	}
	err = c.Client.FrontendCreate(frontend)
	if err != nil {
		return err
	}
	err = c.Client.FrontendBindCreate(FrontendSSL, models.Bind{
		Address: "0.0.0.0:443",
		Name:    "bind_1",
	})
	if err != nil {
		return err
	}
	err = c.Client.FrontendBindCreate(FrontendSSL, models.Bind{
		Address: ":::443",
		Name:    "bind_2",
		V4v6:    true,
	})
	if err != nil {
		return err
	}
	err = c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Action:   "accept",
		Type:     "content",
		Cond:     "if",
		CondTest: "{ req_ssl_hello_type 1 }",
	})
	if err != nil {
		return err
	}
	err = c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
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
	err = c.Client.FrontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Type:    "inspect-delay",
		Index:   utils.PtrInt64(0),
		Timeout: utils.PtrInt64(5000),
	})
	if err != nil {
		return err
	}
	// Create backend for proxy chaining (chaining
	// ssl-passthrough frontend to ssl-offload backend)
	err = c.Client.BackendCreate(models.Backend{
		Name: backendHTTPS,
		Mode: "tcp",
	})
	if err != nil {
		return err
	}
	err = c.Client.BackendServerCreate(backendHTTPS, models.Server{
		Name:        FrontendHTTPS,
		Address:     "127.0.0.1:8443",
		SendProxyV2: "enabled",
	})
	if err != nil {
		return err
	}

	if err = c.bindSSLPassthrough(true); err != nil {
		return err
	}
	// Some TCP rules depend on ssl-passthrough
	c.cfg.FrontendRulesModified[TCP] = true
	return nil
}

func (c *HAProxyController) disableSSLPassthrough() (err error) {
	backendHTTPS := "https"
	err = c.Client.FrontendDelete(FrontendSSL)
	if err != nil {
		return err
	}
	err = c.Client.BackendDelete(backendHTTPS)
	if err != nil {
		return err
	}
	if err = c.bindSSLPassthrough(false); err != nil {
		return err
	}
	// Some TCP rules depend on ssl-passthrough
	c.cfg.FrontendRulesModified[TCP] = true
	return nil
}

func (c *HAProxyController) bindSSLPassthrough(enabled bool) (err error) {
	binds, err := c.Client.FrontendBindsGet(FrontendHTTPS)
	if err != nil {
		return err
	}
	var ipv4, ipv6 string
	if enabled {
		ipv4 = "127.0.0.1:8443"
		ipv6 = "127.0.0.1:8443"
	} else {
		ipv4 = "0.0.0.0:443"
		ipv6 = ":::443"
	}
	for _, bind := range binds {
		if bind.Name == "bind_1" {
			bind.Address = ipv4
			bind.Port = nil
			bind.AcceptProxy = enabled
		} else if bind.Name == "bind_2" {
			bind.Address = ipv6
			bind.Port = nil
			bind.AcceptProxy = enabled
		}
		if err = c.Client.FrontendBindEdit(FrontendHTTPS, *bind); err != nil {
			return err
		}
	}
	return nil
}
