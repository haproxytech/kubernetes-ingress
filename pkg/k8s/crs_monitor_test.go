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
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func TestRememberLateCRIsWatchedOnRematch(t *testing.T) {
	t.Parallel()
	k := &k8s{
		crsV1: map[string]CRV1{},
		crsV3: map[string]CRV3{},
	}
	gk := GroupKind{Group: "ingress.v3.haproxy.org", Kind: "TCP"}
	crsV1, crsV3 := lateCRMaps(gk, utils.OSArgs{})
	if _, ok := crsV3["TCP"]; !ok {
		t.Fatal("lateCRMaps must produce a TCP watcher")
	}
	k.rememberLateCR(gk, crsV1, crsV3)
	if _, ok := k.crsV3["ingress.v3.haproxy.org - TCP"]; !ok {
		t.Fatal("rematch sessions read k.crsV3; late TCP CRD must be recorded there")
	}
	if _, ok := k.crsV3["ingress.v3.haproxy.org - TCP"]; ok {
		if k.crsV3["ingress.v3.haproxy.org - TCP"].GetKind() != "TCP" {
			t.Fatal("recorded CR must be TCP")
		}
	}

	gkV1 := GroupKind{Group: "ingress.v1.haproxy.org", Kind: "Backend"}
	v1, v3 := lateCRMaps(gkV1, utils.OSArgs{})
	k.rememberLateCR(gkV1, v1, v3)
	if _, ok := k.crsV1["ingress.v1.haproxy.org - Backend"]; !ok {
		t.Fatal("late v1 Backend CRD must be recorded for rematch")
	}
}

func TestRememberLateCRSkipLookupMatchesMonitor(t *testing.T) {
	t.Parallel()
	k := &k8s{crsV3: map[string]CRV3{}}
	gk := GroupKind{Group: "ingress.v3.haproxy.org", Kind: "Frontend"}
	_, crsV3 := lateCRMaps(gk, utils.OSArgs{})
	k.rememberLateCR(gk, nil, crsV3)
	if _, ok := k.crsV3["ingress.v3.haproxy.org - "+gk.Kind]; !ok {
		t.Fatal("monitor skip key must match rememberLateCR key")
	}
}
