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

	"github.com/google/renameio"

	config "github.com/haproxytech/kubernetes-ingress/pkg/configuration"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type PatternFiles struct {
	files files
}

func (h *PatternFiles) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	h.files.dir = cfg.Env.PatternDir
	if k.ConfigMaps.PatternFiles == nil {
		return false, nil
	}
	for name, v := range k.ConfigMaps.PatternFiles.Annotations {
		err = h.files.writeFile(name, v)
		if err != nil {
			logger.Errorf("failed writing patternfile '%s': %s", name, err)
		}
	}

	for name, f := range h.files.data {
		if !f.inUse {
			err = h.files.deleteFile(name)
			if err != nil {
				logger.Errorf("failed deleting atternfile '%s': %s", name, err)
			}
			continue
		}
		if f.updated {
			logger.Debugf("patternfile '%s' updated: reload required", name)
			reload = true
		}
		f.inUse = false
		f.updated = false
	}
	return reload, nil
}

type files struct {
	dir  string
	data map[string]*file
}

type file struct {
	hash    string
	inUse   bool
	updated bool
}

func (f *files) deleteFile(name string) error {
	delete(f.data, name)
	err := os.Remove(filepath.Join(f.dir, name))
	return err
}

// writeFile checks if content hash has changed before writing it.
func (f *files) writeFile(name, content string) error {
	newHash := utils.Hash([]byte(content))
	if f.data == nil {
		f.data = map[string]*file{}
	}
	if f.data[name] == nil {
		f.data[name] = &file{}
	}
	file := f.data[name]
	if file.hash != newHash {
		if err := renameio.WriteFile(filepath.Join(f.dir, name), []byte(content), os.ModePerm); err != nil {
			return err
		}
		file.hash = newHash
		file.updated = true
	}
	file.inUse = true
	return nil
}
