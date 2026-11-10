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

package handler

import (
	"testing"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/sha256_crypt"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

const (
	promNamespace  = "haproxy-controller"
	promSecretName = "prometheus-credentials"
	promPassword   = "hunter2"
)

// resetPrometheusState clears the package globals the handler writes to, so that one test
// does not observe the users another one registered.
func resetPrometheusState(t *testing.T) {
	t.Helper()
	prometheusMu.Lock()
	defer prometheusMu.Unlock()
	prometheusUsers = nil
	prometheusUsersActive = false
}

// storeWithPrometheusSecret builds the smallest store the handler needs: a main configmap
// pointing at a secret, and that secret holding the given user/password pairs.
func storeWithPrometheusSecret(data map[string][]byte) store.K8s {
	return store.K8s{
		ConfigMaps: store.ConfigMaps{
			Main: &store.ConfigMap{
				Namespace: promNamespace,
				Name:      "haproxy-kubernetes-ingress",
				Annotations: map[string]string{
					"prometheus-endpoint-auth-secret": promNamespace + "/" + promSecretName,
				},
			},
		},
		Namespaces: map[string]*store.Namespace{
			promNamespace: {
				Name: promNamespace,
				Secret: map[string]*store.Secret{
					promSecretName: {
						Namespace: promNamespace,
						Name:      promSecretName,
						Status:    store.ADDED,
						Data:      data,
					},
				},
			},
		},
	}
}

func updatePrometheus(t *testing.T, data map[string][]byte) error {
	t.Helper()
	resetPrometheusState(t)
	handler := PrometheusEndpoint{PodNs: promNamespace}
	return handler.Update(storeWithPrometheusSecret(data), haproxy.HAProxy{}, nil)
}

// sha256Hash returns what `mkpasswd -m SHA-256` would produce for the given password and
// salt, so the tests assert against a real hash rather than a hand-written lookalike.
func sha256Hash(t *testing.T, password, salt string) string {
	t.Helper()
	hash, err := crypt.SHA256.New().Generate([]byte(password), []byte(salt))
	require.NoError(t, err)
	return hash
}

// storedUser runs one update and returns what the handler registered for that user.
func storedUser(t *testing.T, user string, data map[string][]byte) (prometheusAuthUser, bool) {
	t.Helper()
	prometheusMu.RLock()
	defer prometheusMu.RUnlock()
	stored, ok := prometheusUsers[user]
	return stored, ok
}

// A password that is not a crypt hash used to index fields that were not there, which
// panicked the whole controller from its sync goroutine.
func TestPrometheusPlaintextPasswordIsRejectedWithoutPanic(t *testing.T) {
	err := updatePrometheus(t, map[string][]byte{"admin": []byte(promPassword)})

	require.Error(t, err, "a password that is not a crypt hash must be reported")
	require.Contains(t, err.Error(), "admin")

	_, ok := storedUser(t, "admin", nil)
	require.False(t, ok, "an unusable password must not register a user")

	prometheusMu.RLock()
	defer prometheusMu.RUnlock()
	require.True(t, prometheusUsersActive, "auth stays on, so the endpoint stays closed")
}

// Anything that is not a hash crypt can decode has to be reported, not silently kept with
// a salt read out of it. The "$"-prefixed entries are the ones a purely syntactic check
// waves through: they carry enough '$' to look like a hash without being one.
func TestPrometheusMalformedPasswordsAreRejectedWithoutPanic(t *testing.T) {
	for name, password := range map[string]string{
		"empty":                  "",
		"plaintext":              promPassword,
		"one dollar":             "$5",
		"two dollars":            "$5$salt",
		"no leading dollar":      "5$salt$hash",
		"dollars only":           "$$",
		"plaintext with dollars": "$ecret$pass$word",
		"unknown algorithm":      "$6$abcdefgh$0123456789",
		"apr1 hash":              "$apr1$abcdefgh$0123456789",
	} {
		t.Run(name, func(t *testing.T) {
			require.NotPanics(t, func() {
				err := updatePrometheus(t, map[string][]byte{"admin": []byte(password)})
				require.Error(t, err, "must be reported rather than registered")
				_, ok := storedUser(t, "admin", nil)
				require.False(t, ok)
			})
		})
	}
}

// The registered hash has to verify against the password it was built from, which is the
// comparison prometheusHandler performs on every request. A "rounds=" hash is the case a
// hand-rolled salt parser gets wrong while still looking plausible.
func TestPrometheusStoredHashVerifies(t *testing.T) {
	for name, salt := range map[string]string{
		"plain salt":       "$5$abcdefgh",
		"salt with rounds": "$5$rounds=1000$abcdefgh",
	} {
		t.Run(name, func(t *testing.T) {
			hash := sha256Hash(t, promPassword, salt)

			err := updatePrometheus(t, map[string][]byte{"admin": []byte(hash)})
			require.NoError(t, err)

			stored, ok := storedUser(t, "admin", nil)
			require.True(t, ok)
			require.Equal(t, hash, stored.Password, "the hash is stored verbatim")

			// Replays exactly what prometheusHandler computes on every request.
			computed, err := crypt.SHA256.New().Generate([]byte(promPassword), []byte(stored.Salt))
			require.NoError(t, err)
			require.Equal(t, stored.Password, computed, "the right password must authenticate")

			wrong, err := crypt.SHA256.New().Generate([]byte("wrong"), []byte(stored.Salt))
			require.NoError(t, err)
			require.NotEqual(t, stored.Password, wrong, "a wrong password must not")
		})
	}
}

// One bad entry must not cost the other users their access.
func TestPrometheusBadPasswordDoesNotDropValidUsers(t *testing.T) {
	hash := sha256Hash(t, promPassword, "$5$abcdefgh$")

	err := updatePrometheus(t, map[string][]byte{
		"admin": []byte(hash),
		"guest": []byte("plaintext"),
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "guest")

	_, adminOK := storedUser(t, "admin", nil)
	_, guestOK := storedUser(t, "guest", nil)
	require.True(t, adminOK)
	require.False(t, guestOK)
}

func TestCryptSalt(t *testing.T) {
	for name, tc := range map[string]struct {
		password string
		salt     string
		ok       bool
	}{
		"sha256 hash":            {"$5$abcdefgh$0123456789", "$5$abcdefgh", true},
		"rounds kept":            {"$5$rounds=1000$abcdefgh$0123456789", "$5$rounds=1000$abcdefgh", true},
		"empty hash field":       {"$5$abcdefgh$", "$5$abcdefgh", true},
		"plaintext":              {"hunter2", "", false},
		"empty":                  {"", "", false},
		"missing hash":           {"$5$abcdefgh", "", false},
		"no leading dollar":      {"5$abcdefgh$0123456789", "", false},
		"plaintext with dollars": {"$ecret$pass$word", "", false},
		"unknown algorithm":      {"$6$abcdefgh$0123456789", "", false},
	} {
		t.Run(name, func(t *testing.T) {
			salt, ok := cryptSalt(tc.password)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.salt, salt)
		})
	}
}
