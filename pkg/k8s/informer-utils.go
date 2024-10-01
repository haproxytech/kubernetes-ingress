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

package k8s

import (
	k8smeta "github.com/haproxytech/kubernetes-ingress/pkg/k8s/meta"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"k8s.io/apimachinery/pkg/types"
)

func ToSyncDataEvent(meta k8smeta.MetaInfoer, data interface{}, uid types.UID, resourceVersion string) k8ssync.SyncDataEvent {
	return k8ssync.SyncDataEvent{
		SyncType:        meta.GetType(),
		Namespace:       meta.GetNamespace(),
		Name:            meta.GetName(),
		Data:            data,
		UID:             uid,
		ResourceVersion: resourceVersion,
	}
}
