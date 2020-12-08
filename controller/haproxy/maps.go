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
	"hash/fnv"
	"os"
	"path"
	"strings"
)

type Maps map[string]*mapFile

var mapDir string

type mapFile struct {
	rows     []string
	hash     uint64
	preserve bool
}

func (mf *mapFile) getContent() (string, uint64) {
	var b strings.Builder
	for _, r := range mf.rows {
		b.WriteString(r)
		b.WriteRune('\n')
	}
	content := b.String()
	h := fnv.New64a()
	_, _ = h.Write([]byte(content))
	return content, h.Sum64()
}

func NewMapFiles(path string) Maps {
	mapDir = path
	var maps Maps = make(map[string]*mapFile)
	return maps
}

// AppendRow appends row to mapFile
func (m Maps) AppendRow(name string, row string) {
	if row == "" {
		return
	}
	if m[name] == nil {
		m[name] = &mapFile{}
	}
	m[name].rows = append(m[name].rows, row)
}

func (m Maps) Clean() {
	for _, mapFile := range m {
		mapFile.rows = []string{}
	}
}

func (m Maps) Refresh() (reload bool) {
	reload = false
	for name, mapFile := range m {
		content, hash := mapFile.getContent()
		if mapFile.hash == hash {
			continue
		}
		mapFile.hash = hash
		var f *os.File
		var err error
		filename := GetMapPath(name)
		if content == "" && !mapFile.preserve {
			logger.Error(os.Remove(filename))
			delete(m, name)
			continue
		} else if f, err = os.Create(filename); err != nil {
			logger.Error(err)
			continue
		}
		defer f.Close()
		if _, err = f.WriteString(content); err != nil {
			return reload
		}
		reload = true
	}
	return reload
}

// SetPreserve sets the preserve flag on a mapFile
func (m Maps) SetPreserve(preserve bool, maps ...string) {
	for _, name := range maps {
		if m[name] == nil {
			m[name] = &mapFile{}
		}
		m[name].preserve = preserve
	}
}

func GetMapPath(name string) string {
	return path.Join(mapDir, name) + ".map"
}
