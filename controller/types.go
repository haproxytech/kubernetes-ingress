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

package controller

import (
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// SyncType represents type of k8s received message
type SyncType string

// SyncDataEvent represents converted k8s received message
type SyncDataEvent struct {
	_ [0]int
	SyncType
	Namespace string
	Data      interface{}
}

type Mode string

//nolint:golint,stylecheck
const (
	CONTROLLER_CLASS = "haproxy.org/ingress-controller"
	// SyncType values
	COMMAND       SyncType = "COMMAND"
	CONFIGMAP     SyncType = "CONFIGMAP"
	ENDPOINTS     SyncType = "ENDPOINTS"
	INGRESS       SyncType = "INGRESS"
	INGRESS_CLASS SyncType = "INGRESS_CLASS"
	NAMESPACE     SyncType = "NAMESPACE"
	SERVICE       SyncType = "SERVICE"
	SECRET        SyncType = "SECRET"
	// sections
	FrontendHTTP      = "http"
	FrontendHTTPS     = "https"
	FrontendSSL       = "ssl"
	SSLDefaultBaceknd = "ssl"
	// Modes
	HTTP Mode = "http"
	TCP  Mode = "tcp"
	// Status
	ADDED    = store.ADDED
	DELETED  = store.DELETED
	ERROR    = store.ERROR
	EMPTY    = store.EMPTY
	MODIFIED = store.MODIFIED
)

var logger = utils.GetLogger()
