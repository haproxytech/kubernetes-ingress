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

package secret

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// fakeCerts records what AddSecret was asked to write.
type fakeCerts struct {
	certs.Certificates
	written []certs.SecretType
	err     error
}

func (f *fakeCerts) AddSecret(secret *store.Secret, secretType certs.SecretType) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.written = append(f.written, secretType)
	return fmt.Sprintf("/etc/haproxy/certs/%d/%s_%s.pem", secretType, secret.Namespace, secret.Name), nil
}

func k8sStore() store.K8s {
	return store.K8s{
		Namespaces: map[string]*store.Namespace{
			"web": {Secret: map[string]*store.Secret{
				"app-tls": {Namespace: "web", Name: "app-tls", Status: store.EMPTY},
			}},
		},
	}
}

func ingressSecret() Secret {
	return Secret{
		Name:       types.NamespacedName{Namespace: "web", Name: "app-tls"},
		OwnerType:  OWNERTYPE_INGRESS,
		OwnerName:  "web/bridge",
		SecretType: certs.FT_CERT,
	}
}

func tcpSecret() Secret {
	return Secret{
		Name:       types.NamespacedName{Namespace: "web", Name: "app-tls"},
		OwnerType:  OWNERTYPE_TCP_CR,
		OwnerName:  "web/app-tcp",
		SecretType: certs.TCP_CERT,
	}
}

// TestAddWritesTheSameSecretOncePerCertificateType covers the shared Secret case: the
// certificate of an Ingress and the one of a TCP custom resource come from the same Secret,
// and both have to be written, each to its own directory. Skipping the second one leaves its
// file missing, which the cleanup pass then reports as a certificate to delete, reloading
// HAProxy on a pass where nothing changed.
func TestAddWritesTheSameSecretOncePerCertificateType(t *testing.T) {
	fake := &fakeCerts{}

	ingressPath, err := Add(k8sStore(), fake, ingressSecret())
	require.NoError(t, err)

	tcpPath, err := Add(k8sStore(), fake, tcpSecret())
	require.NoError(t, err)

	require.Equal(t, []certs.SecretType{certs.FT_CERT, certs.TCP_CERT}, fake.written,
		"the same Secret must be written once per certificate type")
	require.NotEqual(t, ingressPath, tcpPath, "each type is written to its own directory")
}

// TestAddReportsAMissingSecret checks that a Secret the store does not have is reported to
// the caller instead of being silently ignored.
func TestAddReportsAMissingSecret(t *testing.T) {
	fake := &fakeCerts{}

	_, err := Add(store.K8s{Namespaces: map[string]*store.Namespace{}}, fake, tcpSecret())
	require.ErrorContains(t, err, "TCP_CR 'web/app-tcp'")
	require.Empty(t, fake.written, "no certificate can be written from a missing Secret")
}

// TestAddReportsAWriteFailure checks that the error of the certificate writer reaches the
// caller: advertising a certificate that was not written makes HAProxy reject the whole
// configuration.
func TestAddReportsAWriteFailure(t *testing.T) {
	fake := &fakeCerts{err: errors.New("certificate or private key missing")}

	_, err := Add(k8sStore(), fake, tcpSecret())
	require.ErrorContains(t, err, "certificate or private key missing")
	require.Empty(t, fake.written)
}
