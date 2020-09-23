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
	"os"
	"path"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

func HandleSecret(ingress store.Ingress, secret store.Secret, writeSecret bool, certs map[string]struct{}, logger utils.Logger) (reload bool, err error) {
	reload = false
	for _, k := range []string{"tls", "rsa", "ecdsa"} {
		key := secret.Data[k+".key"]
		crt := secret.Data[k+".crt"]
		if len(key) != 0 && len(crt) != 0 {
			filename := path.Join(HAProxyCertDir, fmt.Sprintf("%s_%s_%s.pem.rsa", ingress.Name, secret.Namespace, secret.Name))
			if writeSecret {
				if err := WriteCert(filename, key, crt, logger); err != nil {
					logger.Error(err)
					return false, err
				}
				logger.Debugf("Using certificate from secret '%s/%s'", secret.Namespace, secret.Name)
				reload = true
			}
			certs[filename] = struct{}{}
		}
	}
	return reload, nil
}

func WriteCert(filename string, key, crt []byte, logger utils.Logger) error {
	var f *os.File
	var err error
	if f, err = os.Create(filename); err != nil {
		logger.Error(err)
		return err
	}
	defer f.Close()
	if _, err = f.Write(key); err != nil {
		logger.Error(err)
		return err
	}
	//Force writing a newline so that parsing does not barf
	if len(key) > 0 && key[len(key)-1] != byte('\n') {
		logger.Warningf("secret key in %s does not end with \\n, appending it to avoid mangling key and certificate", filename)
		if _, err = f.WriteString("\n"); err != nil {
			logger.Error(err)
			return err
		}
	}
	if _, err = f.Write(crt); err != nil {
		logger.Error(err)
		return err
	}
	if err = f.Sync(); err != nil {
		logger.Error(err)
		return err
	}
	if err = f.Close(); err != nil {
		logger.Error(err)
		return err
	}
	return nil
}
