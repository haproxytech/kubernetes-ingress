// Copyright 2026 HAProxy Technologies LLC
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

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestConvertToStoreTCPTombstone(t *testing.T) {
	t.Parallel()
	tcp := &v3.TCP{
		ObjectMeta: metav1.ObjectMeta{Name: "tcp", Namespace: "app"},
		Spec:       v3.TCPSpec{{Name: "item"}},
	}
	got := convertToStoreTCP(cache.DeletedFinalStateUnknown{Obj: tcp}, store.DELETED)
	if got == nil {
		t.Fatal("tombstone TCP CR must convert")
	}
	if got.Name != "tcp" || got.Namespace != "app" || got.Status != store.DELETED {
		t.Fatalf("tombstone identity: %+v", got)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "item" {
		t.Fatalf("tombstone items: %+v", got.Items)
	}
}
