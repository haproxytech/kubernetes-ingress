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

package haproxy

import (
	"os"
	"path"
	"strconv"
)

type Maps interface {
	AppendHost(key uint64, host string)
	Clean()
	Modified(key uint64)
	Refresh() (reload bool, err error)
}

type mapFiles map[uint64]*mapFile

var mapDir string

type mapFile struct {
	hosts    []string
	modified bool
}

func NewMapFiles(path string) Maps {
	mapDir = path
	var maps mapFiles = make(map[uint64]*mapFile)
	return maps
}

func (m mapFiles) AppendHost(key uint64, host string) {
	if host == "" {
		return
	}
	if m[key] == nil {
		m[key] = &mapFile{
			hosts: []string{host},
		}
		return
	}
	for _, h := range m[key].hosts {
		if h == host {
			return
		}
	}
	m[key].hosts = append(m[key].hosts, host)
}

func (m mapFiles) Clean() {
	for _, mapFile := range m {
		mapFile.hosts = []string{}
		mapFile.modified = false
	}
}

func (m mapFiles) Modified(key uint64) {
	if m[key] == nil {
		m[key] = &mapFile{
			modified: true,
		}
		return
	}
	m[key].modified = true
}

func (m mapFiles) Refresh() (reload bool, err error) {
	reload = false
	for key, mapFile := range m {
		if mapFile.modified {
			var f *os.File
			filename := path.Join(mapDir, strconv.FormatUint(key, 10)) + ".lst"
			hosts := ""
			for _, host := range mapFile.hosts {
				hosts += host + "\n"
			}
			if hosts == "" {
				err = os.Remove(filename)
				return reload, err
			} else if f, err = os.Create(filename); err != nil {
				return reload, err
			}
			defer f.Close()
			if _, err = f.WriteString(hosts); err != nil {
				return reload, err
			}
			reload = true
		}
	}
	return reload, nil
}
