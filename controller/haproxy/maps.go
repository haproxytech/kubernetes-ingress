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
	"fmt"
	"os"
	"strings"
)

type Maps map[MapID]*mapFile

var mapDir string

type mapFile struct {
	rows   map[string]bool
	hasNew bool
}

func (mf *mapFile) getContent() string {
	var content strings.Builder
	for r, removed := range mf.rows {
		if removed {
			continue
		}
		content.WriteString(r)
		content.WriteRune('\n')
	}
	return content.String()
}

func (mf *mapFile) isModified() bool {
	for _, removed := range mf.rows {
		if removed {
			return true
		}
	}
	return mf.hasNew
}

func NewMapFiles(path string) Maps {
	mapDir = path
	var maps Maps = make(map[MapID]*mapFile)
	return maps
}

func (m Maps) AppendRow(id MapID, row string) {
	if row == "" {
		return
	}
	if m[id] == nil {
		m[id] = &mapFile{
			rows: make(map[string]bool),
		}
	}
	if _, ok := m[id].rows[row]; !ok {
		m[id].hasNew = true
	}
	m[id].rows[row] = false
}

func (m Maps) Clean() {
	for _, mapFile := range m {
		for id, removed := range mapFile.rows {
			if removed {
				delete(mapFile.rows, id)
				continue
			}
			mapFile.rows[id] = true
		}
		mapFile.hasNew = false
	}
}

type mapRefreshError struct {
	error
}

func (m *mapRefreshError) add(nErr error) {
	if nErr == nil {
		return
	}
	if m.error == nil {
		m.error = nErr
		return
	}
	m.error = fmt.Errorf("%w\n%s", m.error, nErr)
}

func (m Maps) Refresh() (reload bool, err error) {
	reload = false
	var retErr mapRefreshError
	for id, mapFile := range m {
		if mapFile.isModified() {
			content := mapFile.getContent()
			var f *os.File
			filename := id.Path()
			if content == "" {
				rErr := os.Remove(filename)
				retErr.add(rErr)
				delete(m, id)
				continue
			} else if f, err = os.Create(filename); err != nil {
				retErr.add(err)
				continue
			}
			defer f.Close()
			if _, err = f.WriteString(content); err != nil {
				return reload, err
			}
			reload = true
		}
	}
	return reload, retErr.error
}
