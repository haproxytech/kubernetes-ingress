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
	"os"
	"path/filepath"

	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type PatternFiles struct {
}

func (t PatternFiles) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	var f *os.File
	var errors utils.Errors
	if k.ConfigMaps.PatternFiles == nil {
		return false, nil
	}
	for name, content := range k.ConfigMaps.PatternFiles.Annotations {
		filename := filepath.Join(cfg.Env.PatternDir, name)
		switch content.Status {
		case store.EMPTY:
			continue
		case store.DELETED:
			logger.Error(os.Remove(filename))
		default:
			f, err = os.Create(filename)
			if err != nil {
				errors.Add(err)
				continue
			}
			defer f.Close()
			_, err = f.WriteString(content.Value)
			if err != nil {
				errors.Add(err)
				continue
			}
			reload = true
		}
	}
	return reload, errors.Result()
}
