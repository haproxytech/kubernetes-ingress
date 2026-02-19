package certs

import (
	"bytes"
	"testing"
)

func TestNormalizePEM(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "single newline between certs",
			input:    []byte("cert1\n-----BEGIN CERTIFICATE-----\ncert2"),
			expected: []byte("cert1\n-----BEGIN CERTIFICATE-----\ncert2"),
		},
		{
			name:     "multiple newlines between certs",
			input:    []byte("cert1\n\n\n-----BEGIN CERTIFICATE-----\ncert2"),
			expected: []byte("cert1\n-----BEGIN CERTIFICATE-----\ncert2"),
		},
		{
			name:     "two newlines between certs",
			input:    []byte("-----END CERTIFICATE-----\n\n-----BEGIN CERTIFICATE-----\nintermediate"),
			expected: []byte("-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nintermediate"),
		},
		{
			name:     "empty input",
			input:    []byte(""),
			expected: []byte(""),
		},
		{
			name:     "no newlines",
			input:    []byte("single line"),
			expected: []byte("single line"),
		},
		{
			name:     "trailing newlines",
			input:    []byte("content\n\n\n"),
			expected: []byte("content\n"),
		},
		{
			name:     "leading newlines",
			input:    []byte("\n\n\ncontent"),
			expected: []byte("\ncontent"),
		},
		{
			name:     "multiple newlines throughout",
			input:    []byte("line1\n\nline2\n\n\nline3\n\n"),
			expected: []byte("line1\nline2\nline3\n"),
		},
		{
			name: "realistic certificate chain with multiple newlines",
			input: []byte(`-----END CERTIFICATE-----

-----BEGIN CERTIFICATE-----
MIIBoDCCAUWgAwIBAgIUIzgFRNsANKPpAq4aaEv4xggFvsQwCgYIKoZIzj0EAwIw
-----END CERTIFICATE-----`),
			expected: []byte(`-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBoDCCAUWgAwIBAgIUIzgFRNsANKPpAq4aaEv4xggFvsQwCgYIKoZIzj0EAwIw
-----END CERTIFICATE-----`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePEM(tt.input)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("normalizePEM() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCertContent(t *testing.T) {
	tests := []struct {
		name              string
		key               []byte
		cert              []byte
		expectedSubstring string
	}{
		{
			name:              "key and cert without trailing newline",
			key:               []byte("-----BEGIN PRIVATE KEY-----\nkey_content"),
			cert:              []byte("-----BEGIN CERTIFICATE-----\ncert_content"),
			expectedSubstring: "-----BEGIN PRIVATE KEY-----\nkey_content\n-----BEGIN CERTIFICATE-----\ncert_content",
		},
		{
			name:              "key with trailing newline",
			key:               []byte("-----BEGIN PRIVATE KEY-----\nkey_content\n"),
			cert:              []byte("-----BEGIN CERTIFICATE-----\ncert_content"),
			expectedSubstring: "-----BEGIN PRIVATE KEY-----\nkey_content\n-----BEGIN CERTIFICATE-----\ncert_content",
		},
		{
			name:              "certificate chain with multiple newlines",
			key:               []byte("key_data\n"),
			cert:              []byte("-----END CERTIFICATE-----\n\n-----BEGIN CERTIFICATE-----\nintermediate"),
			expectedSubstring: "key_data\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nintermediate",
		},
		{
			name:              "empty key",
			key:               []byte(""),
			cert:              []byte("cert_data"),
			expectedSubstring: "cert_data",
		},
		{
			name: "cert with multiple newlines between leaf and intermediate",
			key:  []byte("PRIVATE_KEY_CONTENT\n"),
			cert: []byte("-----BEGIN CERTIFICATE-----\nMIIChzCCAi2gAwIBAgIUXIC63krrMatmZZVL8+sy1cedepYwCgYIKoZIzj0EAwIw\n" +
				"-----END CERTIFICATE-----\n\n-----BEGIN CERTIFICATE-----\n" +
				"MIIBoDCCAUWgAwIBAgIUIzgFRNsANKPpAq4aaEv4xggFvsQwCgYIKoZIzj0EAwIw\n" +
				"-----END CERTIFICATE-----"),
			expectedSubstring: "PRIVATE_KEY_CONTENT\n-----BEGIN CERTIFICATE-----\nMIIChzCCAi2gAwIBAgIUXIC63krrMatmZZVL8+sy1cedepYwCgYIKoZIzj0EAwIw\n" +
				"-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\n" +
				"MIIBoDCCAUWgAwIBAgIUIzgFRNsANKPpAq4aaEv4xggFvsQwCgYIKoZIzj0EAwIw\n" +
				"-----END CERTIFICATE-----",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := certContent(tt.key, tt.cert)
			resultStr := string(result)
			if !bytes.Contains(result, []byte(tt.expectedSubstring)) {
				t.Errorf("certContent() result does not contain expected substring.\nGot: %q\nExpected to contain: %q", resultStr, tt.expectedSubstring)
			}

			if bytes.Contains(result, []byte("\n\n")) {
				t.Errorf("certContent() result contains multiple consecutive newlines: %q", resultStr)
			}
		})
	}
}
