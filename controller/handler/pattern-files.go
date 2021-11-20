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

	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
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
		_, ok := h.files.data[name]
		if ok {
			err = h.files.updateFile(name, v)
			if err != nil {
				logger.Errorf("failed updating patternFile '%s': %s", name, err)
			}
		} else {
			err = h.files.newFile(name, v)
			if err != nil {
				logger.Errorf("failed creating patternFile '%s': %s", name, err)
			}
		}
	}

	for name, f := range h.files.data {
		if !f.inUse {
			err = h.files.deleteFile(name)
			if err != nil {
				logger.Errorf("failed deleting PatternFile '%s': %s", name, err)
			}
			continue
		}
		if f.updated {
			logger.Debugf("updating PatternFile '%s': reload required", name)
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

func (f *files) deleteFile(code string) error {
	delete(f.data, code)
	err := os.Remove(filepath.Join(f.dir, code))
	return err
}

func (f *files) newFile(code, value string) error {
	if err := renameio.WriteFile(filepath.Join(f.dir, code), []byte(value), os.ModePerm); err != nil {
		return err
	}
	if f.data == nil {
		f.data = map[string]*file{}
	}
	f.data[code] = &file{
		hash:    utils.Hash([]byte(value)),
		inUse:   true,
		updated: true,
	}
	return nil
}

func (f *files) updateFile(name, value string) error {
	newHash := utils.Hash([]byte(value))
	file := f.data[name]
	if file.hash != newHash {
		err := renameio.WriteFile(filepath.Join(f.dir, name), []byte(value), os.ModePerm)
		if err != nil {
			return err
		}
		file.hash = newHash
		file.updated = true
	}
	file.inUse = true
	f.data[name] = file
	return nil
}
