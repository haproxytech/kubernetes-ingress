package validators_test

import (
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/validators"
	"github.com/stretchr/testify/assert"
)

func TestValidator_Get(t *testing.T) {
	v1, err1 := validators.Get()
	v2, err2 := validators.Get()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Same(t, v1, v2, "Get should return the same validator instance")
}

func TestValidator_Prefix(t *testing.T) {
	v, err := validators.Get()
	assert.NoError(t, err)

	v.SetPrefix("test.com")
	assert.Equal(t, "test.com/", v.Prefix())
}

func TestValidator_ValidateInput(t *testing.T) {
	config := []byte(`
prefix: "haproxy.org"
validation_rules:
  duration-rule:
    type: duration
    rule: "value > duration('1s') && value < duration('10s')"
  duration-rule-frontend:
    section: frontend
    type: duration
    rule: "value > duration('1s') && value < duration('10s')"
  int-rule:
    type: int
    rule: "value > 1 && value < 10"
  uint-rule:
    type: uint
    rule: "value > 1 && value < 10"
  bool-rule:
    type: bool
    rule: "value == true"
  string-rule:
    type: string
    rule: "value.startsWith('foo')"
  float-rule:
    type: float
    rule: "value > 1.0 && value < 10.0"
  json-rule:
    type: json
    rule: "has(value.key) && value.key == 'ok'"
`)
	v, err := validators.Get()
	assert.NoError(t, err)
	v.SetPrefix("haproxy.org")
	err = v.Init(config)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		ruleName string
		value    string
		section  string
		wantErr  bool
	}{
		// Duration
		{"duration-valid", "duration-rule", "5s", "backend", false},
		{"duration-invalid", "duration-rule", "1s", "backend", true},
		{"duration-valid-frontend", "duration-rule-frontend", "5s", "frontend", false},
		{"duration-invalid-frontend", "duration-rule-frontend", "1s", "frontend", true},
		{"duration-invalid-format", "duration-rule", "foo", "backend", true},
		// Int
		{"int-valid", "int-rule", "5", "backend", false},
		{"int-invalid", "int-rule", "1", "backend", true},
		{"int-invalid-format", "int-rule", "foo", "backend", true},
		// Uint
		{"uint-valid", "uint-rule", "5", "backend", false},
		{"uint-invalid", "uint-rule", "1", "backend", true},
		{"uint-invalid-format", "uint-rule", "foo", "backend", true},
		// Bool
		{"bool-valid", "bool-rule", "true", "backend", false},
		{"bool-invalid", "bool-rule", "false", "backend", true},
		{"bool-invalid-format", "bool-rule", "foo", "backend", true},
		// String
		{"string-valid", "string-rule", "foobar", "backend", false},
		{"string-invalid", "string-rule", "bar", "backend", true},
		// Float
		{"float-valid", "float-rule", "5.0", "backend", false},
		{"float-invalid", "float-rule", "1.0", "backend", true},
		{"float-invalid-format", "float-rule", "foo", "backend", true},
		// JSON
		{"json-valid", "json-rule", `{"key":"ok"}`, "backend", false},
		{"json-invalid", "json-rule", `{"key":"nok"}`, "backend", true},
		{"json-invalid-format", "json-rule", `{`, "backend", true},
		// Other
		{"rule-not-found", "non-existent", "foo", "backend", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateInput(tt.ruleName, tt.value, tt.section)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_GetSortedAnnotationKeys(t *testing.T) {
	correctExample := []byte(`
validation_rules:
  timeout-server:
    section: backend
    type: duration
    rule: "value > duration('0s')"
  max-connections:
    section: backend
    type: int
    rule: "value > 0"
    order_priority: 10
  another-rule:
    section: frontend
    type: string
    rule: "value != ''"
  service-specific-rule:
    section: backend
    type: string
    rule: "value != ''"
    resources: ["my-service"]
  frontend-specific-rule:
    section: backend
    type: string
    rule: "value != ''"
    resources: ["my-frontend"]
  backend-specific-rule:
    section: backend
    type: string
    rule: "value != ''"
    resources: ["my-backend"]
  namespace-specific-rule:
    section: backend
    type: string
    rule: "value != ''"
    namespaces: ["my-namespace"]
  ingress-specific-rule:
    section: backend
    type: string
    rule: "value != ''"
    ingresses: ["my-ingress"]
  all-sections-rule:
    section: all
    type: string
    rule: "value != ''"
`)

	tests := []struct {
		name        string
		annotations map[string]string
		filters     validators.FilterValues
		expected    []string
	}{
		{
			name:        "No annotations",
			annotations: map[string]string{},
			filters:     validators.FilterValues{Section: "backend"},
			expected:    []string{},
		},
		{
			name: "Annotations with matching rules, not filtered out",
			annotations: map[string]string{
				"timeout-server":  "5s",
				"max-connections": "100",
			},
			filters:  validators.FilterValues{Section: "backend"},
			expected: []string{"max-connections", "timeout-server"},
		},
		{
			name: "Annotations with matching rules, filtered out by section",
			annotations: map[string]string{
				"timeout-server": "5s",
				"another-rule":   "foo",
			},
			filters:  validators.FilterValues{Section: "backend"},
			expected: []string{"timeout-server"},
		},
		{
			name: "Rule with section 'all' should be included",
			annotations: map[string]string{
				"all-sections-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend"},
			expected: []string{"all-sections-rule"},
		},
		{
			name: "Service specific rule, matching",
			annotations: map[string]string{
				"service-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Service: "my-service"},
			expected: []string{"service-specific-rule"},
		},
		{
			name: "Service specific rule, not matching",
			annotations: map[string]string{
				"service-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Service: "other-service"},
			expected: []string{},
		},
		{
			name: "Frontend specific rule, matching",
			annotations: map[string]string{
				"frontend-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Frontend: "my-frontend"},
			expected: []string{"frontend-specific-rule"},
		},
		{
			name: "Frontend specific rule, matching with non-matching service",
			annotations: map[string]string{
				"frontend-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Service: "non-matching", Frontend: "my-frontend"},
			expected: []string{"frontend-specific-rule"},
		},
		{
			name: "Backend specific rule, matching",
			annotations: map[string]string{
				"backend-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Backend: "my-backend"},
			expected: []string{"backend-specific-rule"},
		},
		{
			name: "Backend specific rule, matching with non-matching service and frontend",
			annotations: map[string]string{
				"backend-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Service: "non-matching", Frontend: "non-matching", Backend: "my-backend"},
			expected: []string{"backend-specific-rule"},
		},
		{
			name: "Namespace specific rule, matching",
			annotations: map[string]string{
				"namespace-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Namespace: "my-namespace"},
			expected: []string{"namespace-specific-rule"},
		},
		{
			name: "Namespace specific rule, not matching",
			annotations: map[string]string{
				"namespace-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Namespace: "other-namespace"},
			expected: []string{},
		},
		{
			name: "Ingress specific rule, matching",
			annotations: map[string]string{
				"ingress-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Ingress: "my-ingress"},
			expected: []string{"ingress-specific-rule"},
		},
		{
			name: "Ingress specific rule, not matching",
			annotations: map[string]string{
				"ingress-specific-rule": "foo",
			},
			filters:  validators.FilterValues{Section: "backend", Ingress: "other-ingress"},
			expected: []string{},
		},
		{
			name: "Combination of filters",
			annotations: map[string]string{
				"timeout-server":          "5s",
				"service-specific-rule":   "foo",
				"namespace-specific-rule": "bar",
				"ingress-specific-rule":   "baz",
			},
			filters: validators.FilterValues{
				Section:   "backend",
				Service:   "my-service",
				Namespace: "my-namespace",
				Ingress:   "my-ingress",
			},
			expected: []string{"ingress-specific-rule", "namespace-specific-rule", "service-specific-rule", "timeout-server"},
		},
	}

	v, err := validators.Get()
	if err != nil {
		t.Fatalf("Failed to get validator: %v", err)
	}
	prefix := "haproxy.org/"
	v.SetPrefix(prefix)
	if err := v.Init(correctExample); err != nil {
		t.Fatalf("Failed to init validator: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := v.GetSortedAnnotationKeys(tt.annotations, tt.filters)
			assert.ElementsMatch(t, tt.expected, keys)
		})
	}
}

func TestValidator_GetResult(t *testing.T) {
	config := []byte(`
validation_rules:
  no-template:
    type: string
    rule: "true"
  with-template:
    type: string
    rule: "true"
    template: "http-request set-header X-Custom {{.value}}"
  json-template:
    type: json
    rule: "true"
    template: "http-request set-header X-{{.key}} {{.value}}"
`)
	v, err := validators.Get()
	assert.NoError(t, err)
	v.SetPrefix("haproxy.org")
	err = v.Init(config)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		ruleName string
		value    string
		env      map[string]any
		expected string
		wantErr  bool
	}{
		{
			name:     "No template",
			ruleName: "no-template",
			value:    "foo",
			expected: "no-template foo",
		},
		{
			name:     "With template",
			ruleName: "with-template",
			value:    "bar",
			expected: "http-request set-header X-Custom bar",
		},
		{
			name:     "JSON template",
			ruleName: "json-template",
			value:    `{"key": "MyKey", "value": "MyValue"}`,
			expected: "http-request set-header X-MyKey MyValue",
		},
		{
			name:     "Rule not found",
			ruleName: "non-existent",
			value:    "foo",
			wantErr:  true,
		},
		{
			name:     "Invalid JSON",
			ruleName: "json-template",
			value:    `{`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.GetResult(tt.ruleName, tt.value, tt.env)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
