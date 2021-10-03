// CopyriFiles 2019 HAProxy Technologies LLC
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

package maps

import (
	"hash/fnv"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type MapFiles map[Name]*mapFile

type Name string

type Path string

// module logger
var logger = utils.GetLogger()

var mapDir string

//nolint:golint,stylecheck
const (
	SNI         Name = "sni"
	HOST        Name = "host"
	PATH_EXACT  Name = "path-exact"
	PATH_PREFIX Name = "path-prefix"
)

type mapFile struct {
	rows     []string
	hash     uint64
	preserve bool
}

func (mf *mapFile) getContent() (string, uint64) {
	var b strings.Builder
	sort.Strings(mf.rows)
	for _, r := range mf.rows {
		b.WriteString(r)
		b.WriteRune('\n')
	}
	content := b.String()
	h := fnv.New64a()
	_, _ = h.Write([]byte(content))
	return content, h.Sum64()
}

func New(path string) *MapFiles {
	mapDir = path
	var maps MapFiles = map[Name]*mapFile{
		// Map files required for HAProxy Rules
		SNI:         {preserve: true},
		HOST:        {preserve: true},
		PATH_EXACT:  {preserve: true},
		PATH_PREFIX: {preserve: true},
	}
	return &maps
}

func (m MapFiles) Exists(name Name) bool {
	return m[name] != nil && len(m[name].rows) != 0
}

// AppendRow appends row to mapFile
func (m MapFiles) AppendRow(name Name, row string) {
	if row == "" {
		return
	}
	if m[name] == nil {
		m[name] = &mapFile{}
	}
	m[name].rows = append(m[name].rows, row)
}

func (m MapFiles) Clean() {
	for _, mapFile := range m {
		mapFile.rows = []string{}
	}
}

func (m MapFiles) Refresh(client api.HAProxyClient) (reload bool) {
	for name, mapFile := range m {
		content, hash := mapFile.getContent()
		if mapFile.hash == hash {
			continue
		}
		mapFile.hash = hash
		var f *os.File
		var err error
		filename := GetPath(name)
		if content == "" && !mapFile.preserve {
			logger.Error(os.Remove(string(filename)))
			delete(m, name)
			continue
		} else if f, err = os.Create(string(filename)); err != nil {
			logger.Error(err)
			continue
		}
		defer f.Close()
		if _, err = f.WriteString(content); err != nil {
			logger.Error(err)
			continue
		}
		logger.Error(f.Sync())
		reload = true
		logger.Debugf("Map file '%s' updated, reload required", name)
		// if err = client.SetMapContent(name, content); err != nil {
		// 	if strings.HasPrefix(err.Error(), "maps dir doesn't exists") {
		// 		logger.Debugf("creating Map file %s", name)
		// 	} else {
		// 		logger.Warningf("dynamic update of '%s' Map file failed: %s", name, err.Error()[:200])
		// 	}
		// 	reload = true
		// }
	}
	return reload
}

func GetPath(name Name) Path {
	return Path(path.Join(mapDir, string(name)) + ".map")
}
