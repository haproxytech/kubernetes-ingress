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

package meta

import (
	"fmt"
	"sync"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

var (
	metaInfoStore     *MetaStore
	metaInfoStoreOnce sync.Once
)

type ProcessedResourceVersionSafe struct {
	// key are <type> and <ns>/<name>/<uid>
	// value is resourceVersion
	processedResourceVersion map[string]map[string]string
	sync.RWMutex
	logger     utils.Logger
	getKeyFunc func(MetaInfoer, types.UID) string
}

type MetaStore struct {
	ProcessedResourceVersion ProcessedResourceVersionSafe
}

func GetMetaStore() *MetaStore {
	metaInfoStoreOnce.Do(func() {
		metaInfoStore = &MetaStore{
			ProcessedResourceVersion: ProcessedResourceVersionSafe{
				processedResourceVersion: make(map[string]map[string]string),
				logger:                   utils.GetK8sLogger(),
				getKeyFunc:               getKey,
			},
		}
	})
	return metaInfoStore
}

func (rv *ProcessedResourceVersionSafe) IsProcessed(meta MetaInfoer, uid types.UID, resourceVersion string) bool {
	rv.RLock()
	defer rv.RUnlock()
	key := rv.getKeyFunc(meta, uid)
	var v string
	var ok bool
	objType := string(meta.GetType())
	if _, ok = rv.processedResourceVersion[objType]; !ok {
		return false
	}
	if v, ok = rv.processedResourceVersion[objType][key]; !ok {
		return false
	}
	// By safety, if uid or resourceVersion are empty, consider it as not processed
	if uid == "" || resourceVersion == "" {
		rv.logger.Tracef("uid or resourceVersion is empty for %s", rv.getKeyFunc(meta, uid))
		return false
	}
	return v == resourceVersion
}

func (rv *ProcessedResourceVersionSafe) Set(meta MetaInfoer, uid types.UID, resourceVersion string) {
	rv.Lock()
	defer rv.Unlock()

	// By safety, if uid or resourceVersion are empty, do not store
	if uid == "" || resourceVersion == "" {
		rv.logger.Tracef("uid or resourceVersion is empty for %s", rv.getKeyFunc(meta, uid))
		return
	}
	objType := string(meta.GetType())
	if _, ok := rv.processedResourceVersion[objType]; !ok {
		rv.processedResourceVersion[objType] = make(map[string]string)
	}
	rv.processedResourceVersion[objType][rv.getKeyFunc(meta, uid)] = resourceVersion
}

func (rv *ProcessedResourceVersionSafe) Delete(meta MetaInfoer, uid types.UID) {
	rv.Lock()
	defer rv.Unlock()
	objType := string(meta.GetType())
	if _, ok := rv.processedResourceVersion[objType]; !ok {
		return
	}
	delete(rv.processedResourceVersion[objType], rv.getKeyFunc(meta, uid))
}

func getKey(meta MetaInfoer, uid types.UID) string {
	return string(uid)
}

func getKeyTestMode(meta MetaInfoer, uid types.UID) string {
	return fmt.Sprintf("%s/%s/%s", meta.GetNamespace(), meta.GetName(), uid)
}

func (rv *ProcessedResourceVersionSafe) SetTestMode() {
	rv.Lock()
	defer rv.Unlock()
	rv.getKeyFunc = getKeyTestMode
}
