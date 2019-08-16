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
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/haproxytech/models"
)

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

func (c *HAProxyController) handleHTTPS() (reloadRequested bool, usingHTTPS bool, err error) {
	usingHTTPS = false
	reloadRequested = false
	status := EMPTY
	if c.osArgs.DefaultCertificate.Name == "" {
		err = c.removeHTTPSListeners()
		return reloadRequested, usingHTTPS, err
	}
	secretAnn, errSecret := GetValueFromAnnotations("ssl-certificate", c.cfg.ConfigMap.Annotations)
	secretData := strings.Split(secretAnn.Value, "/")
	if len(secretData) != 2 {
		return reloadRequested, usingHTTPS, errors.New("invalid secret data")
	}

	namespace, ok := c.cfg.Namespace[secretData[0]]
	if !ok {
		return reloadRequested, usingHTTPS, errors.New("invalid namespace " + secretData[0])
	}
	if secretAnn.Status != EMPTY {
		status = MODIFIED
	}

	if errSecret == nil && (status != "") {
		secret, ok := namespace.Secret[secretData[1]]
		if !ok {
			log.Println("secret not found", secretData[1])
			return reloadRequested, usingHTTPS, err
		}
		//two options are allowed, tls, rsa+ecdsa
		rsaKey, rsaKeyOK := secret.Data["rsa.key"]
		rsaCrt, rsaCrtOK := secret.Data["rsa.crt"]
		ecdsaKey, ecdsaKeyOK := secret.Data["ecdsa.key"]
		ecdsaCrt, ecdsaCrtOK := secret.Data["ecdsa.crt"]
		haveCert := false
		//log.Println(secretName.Value, rsaCrtOK, rsaKeyOK, ecdsaCrtOK, ecdsaKeyOK)
		if rsaKeyOK && rsaCrtOK || ecdsaKeyOK && ecdsaCrtOK {
			if rsaKeyOK && rsaCrtOK {
				errCrt := c.writeCert(HAProxyCertDir+"cert.pem.rsa", rsaKey, rsaCrt)
				if errCrt != nil {
					err1 := c.removeHTTPSListeners()
					LogErr(err1)
					return reloadRequested, usingHTTPS, err
				}
				haveCert = true
			}
			if ecdsaKeyOK && ecdsaCrtOK {
				errCrt := c.writeCert(HAProxyCertDir+"cert.pem.ecdsa", ecdsaKey, ecdsaCrt)
				if errCrt != nil {
					err1 := c.removeHTTPSListeners()
					LogErr(err1)
					return reloadRequested, usingHTTPS, err
				}
				haveCert = true
			}
		} else {
			tlsKey, tlsKeyOK := secret.Data["tls.key"]
			tlsCrt, tlsCrtOK := secret.Data["tls.crt"]
			if tlsKeyOK && tlsCrtOK {
				errCrt := c.writeCert(HAProxyCertDir+"cert.pem", tlsKey, tlsCrt)
				if errCrt != nil {
					err1 := c.removeHTTPSListeners()
					LogErr(err1)
					return reloadRequested, usingHTTPS, err
				}
				haveCert = true
			}
		}
		if !haveCert {
			errL := c.removeHTTPSListeners()
			LogErr(errL)
			return reloadRequested, usingHTTPS, fmt.Errorf("no certificate")
		}

		port := int64(443)
		listenerV4 := models.Bind{
			Address:        "0.0.0.0",
			Alpn:           "h2,http/1.1",
			Port:           &port,
			Name:           "bind_1",
			Ssl:            true,
			SslCertificate: HAProxyCertDir,
		}
		listenerV4v6 := models.Bind{
			Address:        "::",
			Alpn:           "h2,http/1.1",
			Port:           &port,
			Name:           "bind_2",
			V4v6:           true,
			Ssl:            true,
			SslCertificate: HAProxyCertDir,
		}
		usingHTTPS = true
		switch status {
		case ADDED:
			for _, listener := range []models.Bind{listenerV4, listenerV4v6} {
				if err = c.frontendBindCreate(FrontendHTTPS, listener); err != nil {
					if strings.Contains(err.Error(), "already exists") {
						if err = c.frontendBindEdit( FrontendHTTPS, listener); err != nil {
							return reloadRequested, usingHTTPS, err
						}
					} else {
						return reloadRequested, usingHTTPS, err
					}
				}
			}
		case MODIFIED:
			for _, listener := range []models.Bind{listenerV4, listenerV4v6} {
				if err = c.frontendBindEdit( FrontendHTTPS, listener); err != nil {
					return reloadRequested, usingHTTPS, err
				}
			}
		case DELETED:
			for _, listener := range []models.Bind{listenerV4, listenerV4v6} {
				if err = c.frontendBindDelete(FrontendHTTPS, listener.Name); err != nil {
					return reloadRequested, usingHTTPS, err
				}
			}
		}
	}
	return reloadRequested, usingHTTPS, nil
}
