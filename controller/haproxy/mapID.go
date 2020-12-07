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
	"path"
	"strconv"
	"strings"
)

type MapID uint64

func hash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.ToLower(s)))
	return h.Sum64()
}

func NewMapID(s string) MapID {
	return MapID(hash(s))
}

func (mapID MapID) Path() string {
	return path.Join(mapDir, strconv.FormatUint(uint64(mapID), 10)) + ".map"
}
