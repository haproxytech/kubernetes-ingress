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
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/haproxytech/client-native/v2/models"

	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type ErrorFile struct {
	files     files
	updateAPI bool
}

func (h *ErrorFile) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	h.files.dir = cfg.Env.ErrFileDir
	if k.ConfigMaps.Errorfiles == nil {
		return false, nil
	}

	for code, v := range k.ConfigMaps.Errorfiles.Annotations {
		_, ok := h.files.data[code]
		if ok {
			err = h.files.updateFile(code, v)
			if err != nil {
				logger.Errorf("failed updating errorfile for code '%s': %s", code, err)
			}
			continue
		}
		err = checkCode(code)
		if err != nil {
			logger.Errorf("failed creating errorfile for code '%s': %s", code, err)
		}
		err = h.files.newFile(code, v)
		if err != nil {
			logger.Errorf("failed creating errorfile for code '%s': %s", code, err)
		}
		h.updateAPI = true
	}

	var apiInput = []*models.Errorfile{}
	for code, f := range h.files.data {
		if !f.inUse {
			h.updateAPI = true
			err = h.files.deleteFile(code)
			if err != nil {
				logger.Errorf("failed deleting errorfile for code '%s': %s", code, err)
			}
			continue
		}
		if f.updated {
			logger.Debugf("updating errorfile for code '%s': reload required", code)
			reload = true
		}
		c, _ := strconv.Atoi(code) // code already checked in newCode
		apiInput = append(apiInput, &models.Errorfile{
			Code: int64(c),
			File: filepath.Join(h.files.dir, code),
		})
		f.inUse = false
		f.updated = false
	}
	// HAProxy config update
	if h.updateAPI {
		defaults, err := api.DefaultsGetConfiguration()
		if err != nil {
			logger.Error(err)
			return reload, err
		}
		defaults.ErrorFiles = apiInput
		if err = api.DefaultsPushConfiguration(*defaults); err != nil {
			logger.Error(err)
			return reload, err
		}
		h.updateAPI = false
	}
	return reload, nil
}

func checkCode(code string) error {
	var codes = [15]string{"200", "400", "401", "403", "404", "405", "407", "408", "410", "425", "429", "500", "502", "503", "504"}
	var c string
	for _, c = range codes {
		if code == c {
			break
		}
	}
	if c != code {
		return fmt.Errorf("HTTP error code '%s' not supported", code)
	}
	return nil
}
