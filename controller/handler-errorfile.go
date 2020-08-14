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
	"os"
	"path/filepath"

	"github.com/google/renameio"
	"github.com/haproxytech/config-parser/v2/types"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type ErrorFile struct {
	httpErrorCodes []string
	modified       bool
}

func (e ErrorFile) Update(k store.K8s, cfg Configuration, api api.HAProxyClient) (reload bool, err error) {
	if k.ConfigMaps[Errorfiles] == nil {
		return false, nil
	}

	var codes = [15]string{"200", "400", "401", "403", "404", "405", "407", "408", "410", "425", "429", "500", "502", "503", "504"}

	for code, value := range k.ConfigMaps[Errorfiles].Annotations {
		filePath := filepath.Join(HAProxyErrFileDir, code)
		switch value.Status {
		case EMPTY:
			e.httpErrorCodes = append(e.httpErrorCodes, code)
			continue
		case DELETED:
			logger.Debugf("deleting errorfile associated to '%s' error code ", code)
			if err = os.Remove(filePath); err != nil {
				logger.Errorf("failed deleting '%s': %s", filePath, err)
			}
			e.modified = true
		case ADDED, MODIFIED:
			var c string
			for _, c = range codes {
				if code == c {
					break
				}
			}
			if c != code {
				logger.Error("HTTP error code '%s' not supported", code)
				continue
			}
			e.httpErrorCodes = append(e.httpErrorCodes, code)
			logger.Debugf("Setting errorfile associated to '%s' error code", code)
			if err = renameio.WriteFile(filePath, []byte(value.Value), os.ModePerm); err != nil {
				logger.Errorf("failed writing errorfile '%s': %s", filePath, err)
				continue
			}
			e.modified = true
		}
	}
	if e.modified {
		return e.updateAPI(api), nil
	}
	return false, nil
}

func (e ErrorFile) updateAPI(api api.HAProxyClient) (reload bool) {
	logger.Error(api.SetDefaultErrorFile(nil, -1))
	for index, code := range e.httpErrorCodes {
		err := api.SetDefaultErrorFile(&types.ErrorFile{Code: code, File: filepath.Join(HAProxyErrFileDir, code)}, index)

		if err == nil {
			reload = true
		} else {
			logger.Error(err)
		}
	}
	return reload
}
