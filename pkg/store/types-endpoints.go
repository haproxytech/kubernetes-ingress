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

package store

import (
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
)

func (a Endpoints) GetType() k8ssync.SyncType {
	return k8ssync.ENDPOINTS
}

func (a Endpoints) GetName() string {
	return a.SliceName
}

func (a Endpoints) GetNamespace() string {
	return a.Namespace
}

func (a Endpoints) GetStatus() string {
	return string(a.Status)
}
