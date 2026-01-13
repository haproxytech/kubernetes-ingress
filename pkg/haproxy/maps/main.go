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
	"hash/fnv"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/google/renameio"
	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type Maps interface {
	// AppendRow appends row to mapFile
	MapAppend(name Name, row string)
	// Exists returns true if a map exists and is not empty
	MapExists(name Name) bool
	// Refresh refreshs maps content
	RefreshMaps(client api.HAProxyClient)
	// Clean cleans maps content
	CleanMaps()
}

type mapFiles map[Name]*mapFile

type Name string

type Path string

// module logger
var logger = utils.GetLogger()

var mapDir string

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
		if chunk.Len()+len(r) >= api.BufferSize {
			result = append(result, chunk.String())
			chunk.Reset()
		}
		chunk.WriteString(r)
		_, _ = chunk.WriteRune('\n')
		_, _ = h.Write([]byte(r))
	}
	if chunk.Len() > 0 {
		result = append(result, chunk.String())
	}
	return result, h.Sum64()
}

func New(dir string, persistentMaps []Name) (Maps, error) { //nolint:ireturn
	if dir == "" {
		return nil, errors.New("empty name for map directory")
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

func (m mapFiles) RefreshMaps(client api.HAProxyClient) {
	mapFilesToDelete := make([]Name, 0, len(m))
	var wgWriter sync.WaitGroup
	var mapMutex sync.Mutex

	for name, mapFile := range m {
		content, hash := mapFile.getContent()
		if mapFile.hash == hash {
			continue
		}
		wgWriter.Add(1)
		// parallelize writing of files
		fs.Writer.Write(func() {
			defer wgWriter.Done()
			var err error
			filename := GetPath(name)
			if len(content) == 0 && !mapFile.persistent {
				fs.AddDelayedFunc(string(filename), func() {
					logger.Error(os.Remove(string(filename)))
				})
				mapMutex.Lock()
				mapFilesToDelete = append(mapFilesToDelete, name)
				mapMutex.Unlock()
				return
			}
			if _, err = os.Stat(string(filename)); err != nil {
				if !os.IsNotExist(err) {
					logger.Error(err)
					return
				}
				err = renameio.WriteFile(string(filename), []byte{}, 0o666)
				if err != nil {
					logger.Error(err)
					return
				}
			}
			var buff strings.Builder
			buff.Grow(api.BufferSize * len(content))
			for _, d := range content {
				buff.WriteString(d)
			}
			fs.AddDelayedFunc(string(filename), func() {
				err = renameio.WriteFile(string(filename), []byte(buff.String()), 0o666)
				if err != nil {
					logger.Error(err)
					return
				}
			})

			mapFile.hash = hash
			if err = client.SetMapContent(string(name), content); err != nil {
				if errors.Is(err, api.ErrMapNotFound) {
					instance.Reload("Map file %s created", string(name))
				} else {
					instance.Reload("Runtime update of map file '%s' failed : %s", string(name), err.Error())
				}
			}
		})
	}

	wgWriter.Wait()
	for _, mapFileToDelete := range mapFilesToDelete {
		delete(m, mapFileToDelete)
	}
}

func GetPath(name Name) Path {
	return Path(path.Join(mapDir, string(name)) + ".map")
}
