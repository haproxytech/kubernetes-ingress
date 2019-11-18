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

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

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

func (c *HAProxyController) handleSecret(ingress Ingress, secret Secret, writeSecret bool, certs map[string]struct{}) (reloadRequested bool) {
	reloadRequested = false
	//two options are allowed, tls, rsa+ecdsa
	rsaKey, rsaKeyOK := secret.Data["rsa.key"]
	rsaCrt, rsaCrtOK := secret.Data["rsa.crt"]
	ecdsaKey, ecdsaKeyOK := secret.Data["ecdsa.key"]
	ecdsaCrt, ecdsaCrtOK := secret.Data["ecdsa.crt"]
	//log.Println(secretName.Value, rsaCrtOK, rsaKeyOK, ecdsaCrtOK, ecdsaKeyOK)
	if rsaKeyOK && rsaCrtOK || ecdsaKeyOK && ecdsaCrtOK {
		if rsaKeyOK && rsaCrtOK {
			filename := path.Join(HAProxyCertDir, fmt.Sprintf("%s_%s_%s.pem.rsa", secret.Namespace, ingress.Name, secret.Name))
			if writeSecret {
				errCrt := c.writeCert(filename, rsaKey, rsaCrt)
				if errCrt != nil {
					err1 := c.removeHTTPSListeners()
					LogErr(err1)
					return false
				}
				reloadRequested = true
			}
			certs[filename] = struct{}{}
		}
		if ecdsaKeyOK && ecdsaCrtOK {
			filename := path.Join(HAProxyCertDir, fmt.Sprintf("%s_%s_%s.pem.ecdsa", secret.Namespace, ingress.Name, secret.Name))
			if writeSecret {
				errCrt := c.writeCert(filename, ecdsaKey, ecdsaCrt)
				if errCrt != nil {
					err1 := c.removeHTTPSListeners()
					LogErr(err1)
					return false
				}
				reloadRequested = true
			}
			certs[filename] = struct{}{}
		}
	} else {
		tlsKey, tlsKeyOK := secret.Data["tls.key"]
		tlsCrt, tlsCrtOK := secret.Data["tls.crt"]
		if tlsKeyOK && tlsCrtOK {
			filename := path.Join(HAProxyCertDir, fmt.Sprintf("%s_%s_%s.pem", secret.Namespace, ingress.Name, secret.Name))
			if writeSecret {
				errCrt := c.writeCert(filename, tlsKey, tlsCrt)
				if errCrt != nil {
					err1 := c.removeHTTPSListeners()
					LogErr(err1)
					return false
				}
				reloadRequested = true
			}
			certs[filename] = struct{}{}
		}
	}
	return reloadRequested
}

func (c *HAProxyController) handleDefaultCertificate(certs map[string]struct{}) (reloadRequested bool) {
	reloadRequested = false
	secretAnn, defSecretErr := GetValueFromAnnotations("ssl-certificate", c.cfg.ConfigMap.Annotations)
	writeSecret := true
	if defSecretErr == nil {
		if secretAnn.Status == DELETED || secretAnn.Status == EMPTY {
			writeSecret = false
		}
		secretData := strings.Split(secretAnn.Value, "/")
		namespace, namespaceOK := c.cfg.Namespace[secretData[0]]
		if len(secretData) == 2 && namespaceOK {
			secret, ok := namespace.Secret[secretData[1]]
			if ok {
				if secret.Status == EMPTY || secret.Status == DELETED {
					writeSecret = false
				}
				reloadRequested = c.handleSecret(Ingress{
					Name: "DEFAULT_CERT",
				}, *secret, writeSecret, certs)
				c.UseHTTPS = BoolW{
					Value:  true,
					Status: MODIFIED,
				}
				return reloadRequested
			}
		}
	}
	return false
}

func (c *HAProxyController) handleTLSSecret(ingress Ingress, tls IngressTLS, certs map[string]struct{}) (reloadRequested bool) {
	reloadRequested = false
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
			log.Printf("namespace %s does not exists, ignoring.", namespaceName)
		}
		return false
	}
	secret, secretOK := namespace.Secret[secretName]
	if !secretOK {
		if tls.Status != EMPTY {
			log.Printf("secret %s/%s does not exists, ignoring.", namespaceName, secretName)
		}
		return false
	}
	writeSecret := true
	if secret.Status == EMPTY && tls.Status == EMPTY {
		writeSecret = false
	}
	if secret.Status == DELETED {
		writeSecret = false
	}
	reloadRequested = c.handleSecret(ingress, *secret, writeSecret, certs)
	if reloadRequested {
		c.UseHTTPS = BoolW{
			Value:  true,
			Status: MODIFIED,
		}
		return true
	}
	return false
}

func (c *HAProxyController) initHTTPS() {
	port := int64(443)
	listenerV4 := models.Bind{
		Address: "0.0.0.0",
		Alpn:    "h2,http/1.1",
		Port:    &port,
		Name:    "bind_1",
		//Ssl:     true,
		//SslCertificate: HAProxyCertDir,
	}
	listenerV4v6 := models.Bind{
		Address: "::",
		Alpn:    "h2,http/1.1",
		Port:    &port,
		Name:    "bind_2",
		V4v6:    true,
		//Ssl:     true,
		//SslCertificate: HAProxyCertDir,
	}
	for _, listener := range []models.Bind{listenerV4, listenerV4v6} {
		if err := c.frontendBindCreate(FrontendHTTPS, listener); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				if err = c.frontendBindEdit(FrontendHTTPS, listener); err != nil {
					return
				}
			} else {
				return
			}
		} else {
			LogErr(err)
		}
	}
}

func (c *HAProxyController) enableCerts() {
	binds, _ := c.frontendBindsGet(FrontendHTTPS)
	for _, bind := range binds {
		bind.Ssl = true
		bind.SslCertificate = HAProxyCertDir
		err := c.frontendBindEdit(FrontendHTTPS, *bind)
		LogErr(err)
	}
}
