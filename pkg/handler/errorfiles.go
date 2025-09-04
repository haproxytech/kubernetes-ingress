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

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type ErrorFiles struct {
	files files
}

func (handler *ErrorFiles) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	handler.files.dir = h.ErrFileDir
	if k.ConfigMaps.Errorfiles == nil {
		return nil
	}
	// Update Files
	for code, content := range k.ConfigMaps.Errorfiles.Annotations {
		logger.Error(handler.writeFile(code, content))
	}
	apiInput := handler.refresh()
	// Update API
	defaults, err := h.DefaultsGetConfiguration()
	if err != nil {
		return err
	}
	defaults.ErrorFiles = apiInput
	return h.DefaultsPushConfiguration(*defaults)
}

func (handler *ErrorFiles) writeFile(code, content string) (err error) {
	// Update file
	if _, ok := handler.files.data[code]; !ok {
		err = checkCode(code)
		if err != nil {
			return err
		}
	}
	err = handler.files.writeFile(code, content)
	if err != nil {
		err = fmt.Errorf("failed writing errorfile for code '%s': %w", code, err)
	}
	return err
}

func (handler *ErrorFiles) refresh() (result []*models.Errorfile) {
	for code, f := range handler.files.data {
		if !f.inUse {
			instance.Reload("removal of error file for '%s' code", code)
			err := handler.files.deleteFile(code)
			if err != nil {
				logger.Errorf("failed deleting errorfile for code '%s': %s", code, err)
			}
			continue
		}

		instance.ReloadIf(f.updated, "update of error file for '%s' code", code)

		c, _ := strconv.Atoi(code) // code already checked in newCode
		result = append(result, &models.Errorfile{
			Code: int64(c),
			File: filepath.Join(handler.files.dir, code),
		})
		f.inUse = false
		f.updated = false
	}
	return result
}

func checkCode(code string) error {
	codes := [15]string{"200", "400", "401", "403", "404", "405", "407", "408", "410", "425", "429", "500", "502", "503", "504"}
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
