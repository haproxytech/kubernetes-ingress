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
	"log"
	"os"
	"path"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
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

func (c *HAProxyController) writeCert(filename string, key, crt []byte) error {
	var f *os.File
	var err error
	if f, err = os.Create(filename); err != nil {
		log.Println(err)
		return err
	}
	defer f.Close()
	if _, err = f.Write(key); err != nil {
		log.Println(err)
		return err
	}
	//Force writing a newline so that parsing does not barf
	if len(key) > 0 && key[len(key)-1] != byte('\n') {
		log.Println("Warning: secret key in", filename, "does not end with \\n, appending it to avoid mangling key and certificate")
		if _, err = f.WriteString("\n"); err != nil {
			log.Println(err)
			return err
		}
	}
	if _, err = f.Write(crt); err != nil {
		log.Println(err)
		return err
	}
	if err = f.Sync(); err != nil {
		log.Println(err)
		return err
	}
	if err = f.Close(); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (c *HAProxyController) handleSecret(ingress Ingress, secret Secret, writeSecret bool, certs map[string]struct{}) (reload bool) {
	reload = false
	for _, k := range []string{"tls", "rsa", "ecdsa"} {
		key, keyOk := secret.Data[k+".key"]
		crt, crtOk := secret.Data[k+".crt"]
		if keyOk && crtOk {
			filename := path.Join(HAProxyCertDir, fmt.Sprintf("%s_%s_%s.pem.rsa", ingress.Name, secret.Namespace, secret.Name))
			if writeSecret {
				if err := c.writeCert(filename, key, crt); err != nil {
					utils.LogErr(err)
					return false
				}
				reload = true
			}
			certs[filename] = struct{}{}
		}
	}
	return reload
}

func (c *HAProxyController) handleDefaultCertificate(certs map[string]struct{}) (reload bool) {
	secretAnn, defSecretErr := GetValueFromAnnotations("ssl-certificate", c.cfg.ConfigMap.Annotations)
	writeSecret := false
	if defSecretErr == nil {
		if secretAnn.Status != DELETED && secretAnn.Status != EMPTY {
			writeSecret = true
		}
		secretData := strings.Split(secretAnn.Value, "/")
		namespace, namespaceOK := c.cfg.Namespace[secretData[0]]
		if len(secretData) == 2 && namespaceOK {
			secret, ok := namespace.Secret[secretData[1]]
			if ok {
				if secret.Status != EMPTY && secret.Status != DELETED {
					writeSecret = true
				}
				return c.handleSecret(Ingress{
					Name: "0",
				}, *secret, writeSecret, certs)
			}
		}
	}
	return false
}

func (c *HAProxyController) handleTLSSecret(ingress Ingress, tls IngressTLS, certs map[string]struct{}) (reload bool) {
	secretData := strings.Split(tls.SecretName.Value, "/")
	namespaceName := ingress.Namespace
	var secretName string
	if len(secretData) > 1 {
		namespaceName = secretData[0]
		secretName = secretData[1]
	} else {
		secretName = secretData[0] // only secretname is here
	}
	namespace, namespaceOK := c.cfg.Namespace[namespaceName]
	if !namespaceOK {
		if tls.Status != EMPTY {
			log.Printf("namespace '%s' does not exist, ignoring.", namespaceName)
		}
		return false
	}
	secret, secretOK := namespace.Secret[secretName]
	if !secretOK {
		if tls.Status != EMPTY {
			log.Printf("secret '%s/%s' does not exist, ignoring.", namespaceName, secretName)
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
	return c.handleSecret(ingress, *secret, writeSecret, certs)
}

func (c *HAProxyController) handleHTTPS(usedCerts map[string]struct{}) (reload bool) {
	// ssl-passthrough
	if len(c.cfg.BackendSwitchingRules[FrontendSSL]) > 0 {
		if !c.cfg.SSLPassthrough {
			utils.PanicErr(c.enableSSLPassthrough())
			c.cfg.SSLPassthrough = true
			reload = true
		}
	} else if c.cfg.SSLPassthrough {
		utils.PanicErr(c.disableSSLPassthrough())
		c.cfg.SSLPassthrough = false
		reload = true
	}
	// ssl-offload
	if len(usedCerts) > 0 {
		if !c.cfg.HTTPS {
			utils.PanicErr(c.enableSSLOffload(FrontendHTTPS, true))
			c.cfg.HTTPS = true
			reload = true
		}
	} else if c.cfg.HTTPS {
		utils.PanicErr(c.disableSSLOffload(FrontendHTTPS))
		c.cfg.HTTPS = false
		reload = true
	}
	//remove certs that are not needed
	utils.LogErr(c.cleanCertDir(usedCerts))

	return reload
}

func (c *HAProxyController) enableSSLOffload(frontendName string, alpn bool) (err error) {
	binds, _ := c.frontendBindsGet(frontendName)
	for _, bind := range binds {
		bind.Ssl = true
		bind.SslCertificate = HAProxyCertDir
		if alpn {
			bind.Alpn = "h2,http/1.1"
		}
		err = c.frontendBindEdit(frontendName, *bind)
	}
	if err != nil {
		return err
	}
	return err
}

func (c *HAProxyController) disableSSLOffload(frontendName string) (err error) {
	binds, _ := c.frontendBindsGet(frontendName)
	for _, bind := range binds {
		bind.Ssl = false
		bind.SslCertificate = ""
		bind.Alpn = ""
		err = c.frontendBindEdit(frontendName, *bind)
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
	err = c.frontendCreate(frontend)
	if err != nil {
		return err
	}
	err = c.frontendBindCreate(FrontendSSL, models.Bind{
		Address: "0.0.0.0:443",
		Name:    "bind_1",
	})
	if err != nil {
		return err
	}
	err = c.frontendBindCreate(FrontendSSL, models.Bind{
		Address: ":::443",
		Name:    "bind_2",
		V4v6:    true,
	})
	if err != nil {
		return err
	}
	err = c.frontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Index:    utils.PtrInt64(0),
		Action:   "accept",
		Type:     "content",
		Cond:     "if",
		CondTest: "{ req_ssl_hello_type 1 }",
	})
	if err != nil {
		return err
	}
	err = c.frontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
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
	err = c.frontendTCPRequestRuleCreate(FrontendSSL, models.TCPRequestRule{
		Type:    "inspect-delay",
		Index:   utils.PtrInt64(0),
		Timeout: utils.PtrInt64(5000),
	})
	if err != nil {
		return err
	}
	// Create backend for proxy chaining (chaining
	// ssl-passthrough frontend to ssl-offload backend)
	err = c.backendCreate(models.Backend{
		Name: backendHTTPS,
		Mode: "tcp",
	})
	if err != nil {
		return err
	}
	err = c.backendServerCreate(backendHTTPS, models.Server{
		Name:    FrontendHTTPS,
		Address: "127.0.0.1:8443",
	})
	if err != nil {
		return err
	}

	// Update HTTPS backend to listen for connections from FrontendSSL
	err = c.frontendBindDeleteAll(FrontendHTTPS)
	if err != nil {
		return err
	}
	err = c.frontendBindCreate(FrontendHTTPS, models.Bind{
		Address: "127.0.0.1:8443",
		Name:    "bind_1",
	})
	return err
}

func (c *HAProxyController) disableSSLPassthrough() (err error) {
	var ssl bool
	var sslCertificate string
	var alpn string
	backendHTTPS := "https"
	err = c.frontendDelete(FrontendSSL)
	if err != nil {
		return err
	}
	err = c.backendDelete(backendHTTPS)
	if err != nil {
		return err
	}
	if c.cfg.HTTPS {
		ssl = true
		sslCertificate = HAProxyCertDir
		alpn = "h2,http/1.1"
	} else {
		ssl = false
		sslCertificate = ""
		alpn = ""
	}
	err = c.frontendBindEdit(FrontendHTTPS, models.Bind{
		Address:        "0.0.0.0:443",
		Name:           "bind_1",
		Ssl:            ssl,
		SslCertificate: sslCertificate,
		Alpn:           alpn,
	})
	if err != nil {
		return err
	}
	err = c.frontendBindCreate(FrontendHTTPS, models.Bind{
		Address:        ":::443",
		Name:           "bind_2",
		V4v6:           true,
		Ssl:            ssl,
		SslCertificate: sslCertificate,
		Alpn:           alpn,
	})
	return err
}
