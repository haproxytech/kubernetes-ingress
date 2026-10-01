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
	"testing"

	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertIngressKeepsEverySecretForSameHost(t *testing.T) {
	ig := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "ing"},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{Hosts: []string{"a.example.com", "b.example.com"}, SecretName: "cert-rsa"},
				{Hosts: []string{"a.example.com", "b.example.com"}, SecretName: "cert-ecdsa"},
			},
		},
	}

	ingress, err := ConvertToIngress(ig, false)
	require.NoError(t, err)

	secrets := map[string]struct{}{}
	for _, tls := range ingress.TLS {
		secrets[tls.SecretName] = struct{}{}
	}
	require.Equal(t, map[string]struct{}{"cert-rsa": {}, "cert-ecdsa": {}}, secrets)
	require.Len(t, ingress.TLS, 2)
}
