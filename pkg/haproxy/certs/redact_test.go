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

package certs

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
)

// mockHAProxyClient provides a minimal mock of api.HAProxyClient for testing
// cert runtime operations. Only the Cert-related methods used by updateRuntime
// are implemented; calling any other method will panic (embedded nil interface).
type mockHAProxyClient struct {
	api.HAProxyClient
	onCertEntryCreate func(string) error
	onCertEntrySet    func(string, []byte) error
	onCertEntryCommit func(string) error
	onCertEntryAbort  func(string) error
}

func (m *mockHAProxyClient) CertEntryCreate(filename string) error {
	if m.onCertEntryCreate != nil {
		return m.onCertEntryCreate(filename)
	}
	return nil
}

func (m *mockHAProxyClient) CertEntrySet(filename string, payload []byte) error {
	if m.onCertEntrySet != nil {
		return m.onCertEntrySet(filename, payload)
	}
	return nil
}

func (m *mockHAProxyClient) CertEntryCommit(filename string) error {
	if m.onCertEntryCommit != nil {
		return m.onCertEntryCommit(filename)
	}
	return nil
}

func (m *mockHAProxyClient) CertEntryAbort(filename string) error {
	if m.onCertEntryAbort != nil {
		return m.onCertEntryAbort(filename)
	}
	return nil
}

// generateTestPEM creates an ephemeral ECDSA private key and self-signed
// certificate encoded as PEM. The material is generated fresh on every call
// so no real secrets are stored in the source code.
func generateTestPEM(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal test key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create test certificate: %v", err)
	}

	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatalf("failed to encode test key PEM: %v", err)
	}
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("failed to encode test cert PEM: %v", err)
	}

	return buf.String()
}

// simulateHAProxyRuntimeError creates an error message identical to what the
// HAProxy runtime API returns when a cert operation via the UNIX socket fails.
//
// In the real system, the client-native library's ExecuteWithResponse() formats
// errors as: fmt.Errorf("[%c] %s [%s]", severity, response, command)
// where command is the full "set ssl cert <path> <<\n<PEM PAYLOAD>\n".
// This means the FULL private key and certificate are embedded in the error.
func simulateHAProxyRuntimeError(certPath, pemContent string) error {
	haproxyResponse := fmt.Sprintf(
		" unable to load certificate from file '%s': NO_START_LINE.\n"+
			"Can't load the payload\n"+
			"Can't update %s!",
		certPath, certPath,
	)
	command := fmt.Sprintf("set ssl cert %s <<\n%s\n", certPath, pemContent)
	// This is the exact format from ExecuteWithResponse in client-native
	return fmt.Errorf("[3] %s [%s]", haproxyResponse, command)
}

// TestCertErrorForLog_MustNotContainPEMContent verifies that certErrorForLog()
// strips PEM-encoded blocks (private keys, certificates) from error messages
// before they are used in log output.
//
// SECURITY CONTEXT:
// When a runtime cert update fails, the HAProxy runtime API echoes the full
// PEM payload (including the private key) in its error response. The controller
// passes this error directly to instance.Reload() which logs at INFO level.
// Anyone with access to controller logs can extract TLS private keys.
func TestCertErrorForLog_MustNotContainPEMContent(t *testing.T) {
	certPath := "/etc/haproxy/certs/frontend/test-cert.pem"
	runtimeErr := simulateHAProxyRuntimeError(certPath, generateTestPEM(t))

	result := certErrorForLog(runtimeErr)

	// The result MUST NOT contain any PEM-encoded block
	if strings.Contains(result, "-----BEGIN PRIVATE KEY-----") {
		t.Fatal("SECURITY BUG: certErrorForLog() output contains PRIVATE KEY material.\n" +
			"This content is passed to instance.Reload() and logged at INFO level,\n" +
			"exposing TLS private keys to anyone with access to controller logs.")
	}
	if strings.Contains(result, "-----BEGIN CERTIFICATE-----") {
		t.Fatal("SECURITY BUG: certErrorForLog() output contains certificate PEM block.\n" +
			"While less critical than a private key leak, certificate content\n" +
			"should also be redacted from error log messages.")
	}
}

// TestCertErrorForLog_PreservesUsefulErrorContext verifies that after redacting
// PEM content, the error message still contains useful diagnostic information
// (the file path, the HAProxy error description, etc.)
func TestCertErrorForLog_PreservesUsefulErrorContext(t *testing.T) {
	certPath := "/etc/haproxy/certs/frontend/test-cert.pem"
	runtimeErr := simulateHAProxyRuntimeError(certPath, generateTestPEM(t))

	result := certErrorForLog(runtimeErr)

	// These diagnostic details must be preserved
	checks := []struct {
		substr string
		desc   string
	}{
		{"unable to load certificate", "HAProxy error description"},
		{"NO_START_LINE", "OpenSSL error code"},
		{certPath, "certificate file path"},
	}
	for _, check := range checks {
		if !strings.Contains(result, check.substr) {
			t.Errorf("certErrorForLog() should preserve %s (%q) in the output.\nGot: %s",
				check.desc, check.substr, result)
		}
	}
}

// TestCertErrorForLog_NilError verifies certErrorForLog handles nil errors.
func TestCertErrorForLog_NilError(t *testing.T) {
	result := certErrorForLog(nil)
	if result != "" {
		t.Errorf("certErrorForLog(nil) = %q, want empty string", result)
	}
}

// TestCertErrorForLog_ErrorWithoutPEM verifies that errors without PEM content
// are returned unchanged.
func TestCertErrorForLog_ErrorWithoutPEM(t *testing.T) {
	err := errors.New("connection refused: /var/run/haproxy-runtime-api.sock")
	result := certErrorForLog(err)
	if result != err.Error() {
		t.Errorf("certErrorForLog() modified a non-PEM error.\nGot:  %s\nWant: %s",
			result, err.Error())
	}
}

// TestUpdateRuntime_CommitFailure_LogSafeError is an integration test that
// proves the error returned by updateRuntime contains PEM content, and that
// certErrorForLog properly sanitizes it before it reaches the logs.
//
// In the current code, writeCert() passes err.Error() directly to
// instance.Reload() which logs at INFO level. This test verifies that
// certErrorForLog() removes private key material from such errors.
func TestUpdateRuntime_CommitFailure_LogSafeError(t *testing.T) {
	certPath := "/etc/haproxy/certs/frontend/app-cert.pem"
	pemData := generateTestPEM(t)
	payload := []byte(pemData)

	runtimeErr := simulateHAProxyRuntimeError(certPath, pemData)

	mock := &mockHAProxyClient{
		onCertEntryCreate: func(string) error { return nil },
		onCertEntrySet:    func(string, []byte) error { return nil },
		onCertEntryCommit: func(string) error { return runtimeErr },
		onCertEntryAbort:  func(string) error { return nil },
	}

	c := &certs{
		client: mock,
		mu:     &sync.Mutex{},
	}

	_, err := c.updateRuntime(certPath, payload, false)
	if err == nil {
		t.Fatal("expected error from updateRuntime when commit fails, got nil")
	}

	// The raw error from updateRuntime will contain PEM (this comes from client-native).
	// Verify certErrorForLog sanitizes it before it would reach the logs.
	sanitized := certErrorForLog(err)
	if strings.Contains(sanitized, "BEGIN PRIVATE KEY") {
		t.Fatal("SECURITY BUG: certErrorForLog() does not redact private key from runtime error.\n" +
			"In writeCert(), this unsanitized error is passed to instance.Reload()\n" +
			"and logged at INFO level, leaking TLS private keys to controller logs.")
	}
}

// TestUpdateRuntime_SetFailure_LogSafeError proves the same sanitization works
// when CertEntrySet fails (not just CertEntryCommit). Any failure in the
// cert update pipeline must have PEM content redacted before logging.
func TestUpdateRuntime_SetFailure_LogSafeError(t *testing.T) {
	certPath := "/etc/haproxy/certs/frontend/app-cert.pem"
	pemData := generateTestPEM(t)
	payload := []byte(pemData)

	runtimeErr := simulateHAProxyRuntimeError(certPath, pemData)

	mock := &mockHAProxyClient{
		onCertEntryCreate: func(string) error { return nil },
		onCertEntrySet:    func(string, []byte) error { return runtimeErr },
		onCertEntryCommit: func(string) error { return nil },
		onCertEntryAbort:  func(string) error { return nil },
	}

	c := &certs{
		client: mock,
		mu:     &sync.Mutex{},
	}

	_, err := c.updateRuntime(certPath, payload, false)
	if err == nil {
		t.Fatal("expected error from updateRuntime when set fails, got nil")
	}

	sanitized := certErrorForLog(err)
	if strings.Contains(sanitized, "BEGIN PRIVATE KEY") {
		t.Fatal("SECURITY BUG: certErrorForLog() does not redact private key on Set failure.")
	}
}
