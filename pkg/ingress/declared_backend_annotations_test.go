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

package ingress

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func ingressWith(anns map[string]string) *Ingress {
	return &Ingress{
		resource:    &store.Ingress{IngressCore: store.IngressCore{Namespace: "ns", Name: "ing", Annotations: anns}},
		annotations: annotations.New(),
	}
}

// TestDeclaredBackendAnnotationsListsWhatIsLost is what makes the warning worth reading: an
// ingress which does not own a shared backend keeps being served, but the backend
// configuration it declared is not applied, and nothing else says which.
func TestDeclaredBackendAnnotationsListsWhatIsLost(t *testing.T) {
	i := ingressWith(map[string]string{
		"load-balance":  "uri",
		"forwarded-for": "true",
		"allow-list":    "10.0.0.0/8", // frontend rule, applied to its own routes
		"ssl-redirect":  "true",       // idem
	})

	require.Equal(t, []string{"load-balance", "forwarded-for"}, i.declaredBackendAnnotations(),
		"only the backend annotations are lost, and they must come out in registry order")
}

// TestDeclaredBackendAnnotationsEmptyWhenNothingIsLost pins the common case: an ingress
// sharing a service usually declares no backend annotation at all, and must produce no
// warning.
func TestDeclaredBackendAnnotationsEmptyWhenNothingIsLost(t *testing.T) {
	require.Empty(t, ingressWith(map[string]string{"allow-list": "10.0.0.0/8"}).declaredBackendAnnotations())
	require.Empty(t, ingressWith(map[string]string{}).declaredBackendAnnotations())
	require.Empty(t, ingressWith(nil).declaredBackendAnnotations())
}

// TestDeclaredBackendAnnotationsCoversTheUnregisteredOnes covers the two which are not in
// the Backend() registry, the config snippet being the most consequential of all to lose
// silently.
func TestDeclaredBackendAnnotationsCoversTheUnregisteredOnes(t *testing.T) {
	i := ingressWith(map[string]string{
		"backend-config-snippet": "http-request set-header X-Test 1",
		"cr-backend":             "ns/my-backend",
	})

	require.Equal(t, []string{"backend-config-snippet", "cr-backend"}, i.declaredBackendAnnotations())
}
