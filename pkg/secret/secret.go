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
	"fmt"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"k8s.io/apimachinery/pkg/types"
)

type OwnerType string

//nolint:golint,stylecheck
const (
	OWNERTYPE_INGRESS OwnerType = "Ingress"
	OWNERTYPE_TCP_CR  OwnerType = "TCP_CR"
)

type Secret struct {
	Name       types.NamespacedName
	OwnerType  OwnerType
	OwnerName  string
	SecretType certs.SecretType
}

// Add writes the certificate sec asks for and returns the path it was written
// to.
//
// The same Secret can be needed twice at the same time, as the certificate of
// an Ingress and as the one of a TCP custom resource: the kind of certificate
// is what tells them apart, each copy going to its own directory. The
// certificate manager writes a Secret once per kind, and leaves it alone when
// its certificate is already there and the Secret did not change, so a caller
// can ask for the Secrets it depends on as many times as it meets them.
//
// The caller has to report the error: advertising a certificate that was not
// written makes HAProxy reject the whole configuration.
func Add(k store.K8s, c certs.Certificates, sec Secret) (certPath string, err error) {
	secret, errGet := k.GetSecret(sec.Name.Namespace, sec.Name.Name)
	if errGet != nil {
		// The error from the store already names the Secret.
		return "", fmt.Errorf("%s '%s': %w", sec.OwnerType, sec.OwnerName, errGet)
	}
	return c.AddSecret(secret, sec.SecretType)
}
