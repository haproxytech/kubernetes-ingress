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
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type Maps interface {
	// AppendRow appends row to mapFile
	MapAppend(name Name, row string)
	// Exists returns true if a map exists and is not empty
	MapExists(name Name) bool
	// Refresh refreshs maps content and returns true if a map content has changed(crated/deleted/updated)
	RefreshMaps(client api.HAProxyClient) bool
	// Clean cleans maps content
	CleanMaps()
}

type mapFiles map[Name]*mapFile

type Name string

type Path string

// module logger
var logger = utils.GetLogger()

var mapDir string

// bufsize is the default value of HAproxy tune.bufsize.
// Map payload cannot be bigger than tune.bufsize
const bufSize = 16000

type mapFile struct {
	rows       []string
	hash       uint64
	persistent bool
	// A persistent map will not be removed even if the map is empty
	// because it is always referenced in a haproxy rule.
}

// getContent returns the content of a haproxy map file in a list of chunks
// where each chunk is <= bufsie. It also returns a hash of the map content
func (mf *mapFile) getContent() (result []string, hash uint64) {
	var chunk strings.Builder
	sort.Strings(mf.rows)
	h := fnv.New64a()
	for _, r := range mf.rows {
		if chunk.Len()+len(r) >= bufSize {
			result = append(result, chunk.String())
			chunk.Reset()
		}
		chunk.WriteString(r)
		chunk.WriteRune('\n')
		_, _ = h.Write([]byte(r))
	}
	if chunk.Len() > 0 {
		result = append(result, chunk.String())
	}
	return result, h.Sum64()
}

func New(dir string, persistentMaps []Name) (Maps, error) { //nolint:ireturn
	if dir == "" {
		return nil, fmt.Errorf("empty name for map directory")
	}
	mapDir = dir
	var maps mapFiles = make(map[Name]*mapFile, len(persistentMaps))
	for _, name := range persistentMaps {
		maps[name] = &mapFile{persistent: true}
	}
	return &maps, nil
}

func (m mapFiles) MapExists(name Name) bool {
	return m[name] != nil && len(m[name].rows) != 0
}

func (m mapFiles) MapAppend(name Name, row string) {
	if row == "" {
		return
	}
	if m[name] == nil {
		m[name] = &mapFile{}
	}
	m[name].rows = append(m[name].rows, row)
}

func (m mapFiles) CleanMaps() {
	for _, mapFile := range m {
		mapFile.rows = []string{}
	}
}

func (m mapFiles) RefreshMaps(client api.HAProxyClient) (reload bool) {
	for name, mapFile := range m {
		content, hash := mapFile.getContent()
		if mapFile.hash == hash {
			continue
		}
		mapFile.hash = hash
		var f *os.File
		var err error
		filename := GetPath(name)
		if len(content) == 0 && !mapFile.persistent {
			logger.Error(os.Remove(string(filename)))
			delete(m, name)
			continue
		} else if f, err = os.Create(string(filename)); err != nil {
			logger.Error(err)
			continue
		}
		defer f.Close()
		for _, d := range content {
			if _, err = f.WriteString(d); err != nil {
				logger.Error(err)
				return
			}
		}
		logger.Error(f.Sync())
		if err = client.SetMapContent(string(name), content); err != nil {
			if errors.Is(err, api.ErrMapNotFound) {
				logger.Debugf("Map file %s created, reload required", name)
			} else {
				logger.Debugf("Runtime update of map file '%s' failed, reload required: %s", name, err)
			}
			reload = true
		}
	}
	return reload
}

func GetPath(name Name) Path {
	return Path(path.Join(mapDir, string(name)) + ".map")
}
