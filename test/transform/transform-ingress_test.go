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

package k8stransform_test

import (
	"os"
	"testing"

	k8stransform "github.com/haproxytech/kubernetes-ingress/pkg/k8s/transform"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	networkingv1 "k8s.io/api/networking/v1"
)

func TestRemoveIngressDuplicates(t *testing.T) {
	tests := []struct {
		name         string
		inputPath    string
		expectedPath string
	}{
		{
			name:         "no duplicates",
			inputPath:    "data/ingress/no-duplicate.yaml",
			expectedPath: "data/ingress/expectation/no-duplicate.yaml",
		},
		{
			name:         "dupl-1",
			inputPath:    "data/ingress/dupl-1.yaml",
			expectedPath: "data/ingress/expectation/no-duplicate.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := getIngressFromManifest(t, tt.inputPath)
			k8stransform.RemoveDuplicates(ingress)

			expectedIngress := getIngressFromManifest(t, tt.expectedPath)
			sortHost := cmpopts.SortSlices(func(a, b string) bool {
				return a < b
			})
			if !cmp.Equal(ingress, expectedIngress, sortHost) {
				diff := cmp.Diff(ingress, expectedIngress, sortHost)
				t.Errorf("[%s] ingress does not match expectation\n%v", tt.name, diff)
			}
		})
	}
}

func TestRemoveIngressTLSDuplicates(t *testing.T) {
	tests := []struct {
		name         string
		inputPath    string
		expectedPath string
		hasDups      bool
	}{
		{
			name:         "no duplicates",
			inputPath:    "data/tls/no-duplicate.yaml",
			expectedPath: "data/tls/expectation/no-duplicate.yaml",
		},
		{
			name:         "multiple hosts same tls",
			inputPath:    "data/tls/dupl-hosts.yaml",
			expectedPath: "data/tls/expectation/no-duplicate.yaml",
			hasDups:      true,
		},
		{
			name:         "multiple hosts same tls",
			inputPath:    "data/tls/dupl-tls.yaml",
			expectedPath: "data/tls/expectation/no-duplicate.yaml",
			hasDups:      true,
		},
		{
			name:         "multiple hosts same tls-2",
			inputPath:    "data/tls/dupl-tls-2.yaml",
			expectedPath: "data/tls/expectation/dupl-tls-2.yaml",
			hasDups:      true,
		},
		{
			name:         "multiple hosts",
			inputPath:    "data/tls/dupl-both.yaml",
			expectedPath: "data/tls/expectation/no-duplicate.yaml",
			hasDups:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tls := getTLSFromManifest(t, tt.inputPath)
			got, hasDups := k8stransform.RemoveIngressTLSDuplicates(tls)

			expectedTLS := getTLSFromManifest(t, tt.expectedPath)
			sortHost := cmpopts.SortSlices(func(a, b string) bool {
				return a < b
			})
			if !cmp.Equal(got, expectedTLS, sortHost) {
				diff := cmp.Diff(got, expectedTLS, sortHost)
				t.Errorf("[%s] tls does not match expectation\n%v", tt.name, diff)
			}
			if hasDups != tt.hasDups {
				t.Errorf("[%s] hasDups got [%t] want [%t]", tt.name, hasDups, tt.hasDups)
			}
		})
	}
}

func TestRemoveIngressRuleDuplicates(t *testing.T) {
	tests := []struct {
		name         string
		inputPath    string
		expectedPath string
		hasDups      bool
	}{
		{
			name:         "no duplicates",
			inputPath:    "data/rule/no-duplicate.yaml",
			expectedPath: "data/rule/expectation/no-duplicate.yaml",
		},
		{
			name:         "dupl-1",
			inputPath:    "data/rule/dupl-1.yaml",
			expectedPath: "data/rule/expectation/no-duplicate.yaml",
			hasDups:      true,
		},
		{
			name:         "no dupl-1",
			inputPath:    "data/rule/no-dupl-1.yaml",
			expectedPath: "data/rule/no-dupl-1.yaml",
		},
		{
			name:         "no dupl-2",
			inputPath:    "data/rule/no-dupl-2.yaml",
			expectedPath: "data/rule/no-dupl-2.yaml", // same as input
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := getRuleFromManifest(t, tt.inputPath)
			got, hasDups := k8stransform.RemoveIngressRuleDuplicates(rules)

			expectedRules := getRuleFromManifest(t, tt.expectedPath)
			if !cmp.Equal(got, expectedRules) {
				diff := cmp.Diff(got, expectedRules)
				t.Errorf("[%s] rule does not match expectation \n%v\ngot %v\nwant %v", tt.name, diff, got, expectedRules)
			}
			if hasDups != tt.hasDups {
				t.Errorf("[%s] hasDups got [%t] want [%t]", tt.name, hasDups, tt.hasDups)
			}
		})
	}
}

func getIngressFromManifest(t *testing.T, path string) *networkingv1.Ingress {
	t.Helper()
	jIng, err := os.ReadFile(path)
	require.NoError(t, err)
	var ing networkingv1.Ingress
	_ = yaml.Unmarshal(jIng, &ing)
	return &ing
}

func getRuleFromManifest(t *testing.T, path string) []networkingv1.IngressRule {
	t.Helper()
	jTLS, err := os.ReadFile(path)
	require.NoError(t, err)
	var rules []networkingv1.IngressRule
	_ = yaml.Unmarshal(jTLS, &rules)
	return rules
}

func getTLSFromManifest(t *testing.T, path string) []networkingv1.IngressTLS {
	t.Helper()
	jTLS, err := os.ReadFile(path)
	require.NoError(t, err)
	var tls []networkingv1.IngressTLS
	_ = yaml.Unmarshal(jTLS, &tls)
	return tls
}
