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

package store_test

import (
	_ "embed"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func getResourceList(t *testing.T, paths []string) *store.TCPs {
	t.Helper()
	resourceList := &store.TCPs{}
	resourceList.Items = make([]*store.TCPResource, 0)
	for _, path := range paths {
		resourceList.Items = append(resourceList.Items, getTCPResourceFromFile(t, path))
	}
	return resourceList
}

func getTCPResourceFromFile(t *testing.T, path string) *store.TCPResource {
	t.Helper()
	tcpJ, err := os.ReadFile(path)
	require.NoError(t, err)
	var tcp store.TCPResource
	_ = yaml.Unmarshal(tcpJ, &tcp)
	return &tcp
}

func getTCPResourceListFromFile(t *testing.T, path string) *store.TCPResourceList {
	t.Helper()
	tcpJ, err := os.ReadFile(path)
	require.NoError(t, err)
	var tcpResourceList store.TCPResourceList
	_ = yaml.Unmarshal(tcpJ, &tcpResourceList)
	return &tcpResourceList
}

func getCollisionMapFromFile(t *testing.T, path string) map[string][]*store.TCPResource {
	t.Helper()
	mapJ, err := os.ReadFile(path)
	require.NoError(t, err)
	var mapRes map[string][]*store.TCPResource
	_ = yaml.Unmarshal(mapJ, &mapRes)
	return mapRes
}

func TestTCPs_Order(t *testing.T) {
	tests := []struct {
		name          string
		unorderedPath string
		expectedPath  string
	}{
		{
			name:          "test_1",
			unorderedPath: "manifests/unordered.yaml",
			expectedPath:  "expectations/ordered.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unordered := getTCPResourceListFromFile(t, tt.unorderedPath)
			unordered.Order()
			expected := getTCPResourceListFromFile(t, tt.expectedPath)
			got, _ := json.Marshal(unordered)
			want, _ := json.Marshal(expected)

			exp := strings.ReplaceAll(string(want), "\n", "")
			if exp != string(got) {
				t.Errorf("Order() = %v, want %v", string(got), exp)
			}
		})
	}
}

func TestTCPs_Equal(t *testing.T) {
	tests := []struct {
		name           string
		resource1Paths []string
		resource2Paths []string
		want           bool
	}{
		{
			name:           "equality",
			resource1Paths: []string{"manifests/tcp1.yaml", "manifests/tcp2.yaml"},
			resource2Paths: []string{"manifests/tcp1.yaml", "manifests/tcp2.yaml"},
			want:           true,
		},
		{
			name:           "equality - between same object unordered and ordered",
			resource1Paths: []string{"manifests/tcp1.yaml", "manifests/tcp2.yaml", "manifests/tcp3.yaml"},
			resource2Paths: []string{"manifests/tcp3.yaml", "manifests/tcp1.yaml", "manifests/tcp2.yaml"},
			want:           true,
		},
		{
			name:           "not equal",
			resource1Paths: []string{"manifests/tcp1.yaml", "manifests/tcp2.yaml"},
			resource2Paths: []string{"manifests/tcp1.yaml", "manifests/tcp3.yaml"},
			want:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceList1 := getResourceList(t, tt.resource1Paths)
			resourceList2 := getResourceList(t, tt.resource2Paths)

			if got := resourceList1.Equal(resourceList2); got != tt.want {
				t.Errorf("%s TCPs.Equal() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestTCPs_HasCollisionAddressPort(t *testing.T) {
	tests := []struct {
		name                   string
		resourcePaths          []string
		parent                 string
		hasCollision           bool
		expectedCollisionsPath string
	}{
		{
			name:          "no collision-1",
			resourcePaths: []string{"manifests/tcp1.yaml", "manifests/tcp2.yaml", "manifests/tcp3.yaml"},
			parent:        "tcp-1",
		},
		{
			name:          "no collisions-2",
			resourcePaths: []string{"manifests/tcp3.yaml", "manifests/tcp4.yaml"},
			parent:        "tcp-1",
		},
		{
			name:          "no collisions-3",
			resourcePaths: []string{"manifests/tcp3.yaml", "manifests/tcp3.yaml"},
			parent:        "tcp-1",
		},
		{
			name:                   "collision tcp1 and tcp1CollAddPort on address port",
			resourcePaths:          []string{"manifests/tcp1.yaml", "manifests/tcp1-coll-address-port.yaml", "manifests/tcp2.yaml"},
			parent:                 "tcp-1",
			hasCollision:           true,
			expectedCollisionsPath: "expectations/has-collisions/coll-address-port.yaml",
		},
		{
			name:                   "collision tcp1/tcp1CollAddPort and tcp2/tcp2CollAddPort on address port",
			resourcePaths:          []string{"manifests/tcp1.yaml", "manifests/tcp1-coll-address-port.yaml", "manifests/tcp2.yaml", "manifests/tcp2-coll-address-port.yaml"},
			parent:                 "tcp-1",
			hasCollision:           true,
			expectedCollisionsPath: "expectations/has-collisions/coll-address-port-2.yaml",
		},
		{
			name:                   "collision tcp1/tcp1CollAddPort/tcp1CollAddPort2",
			resourcePaths:          []string{"manifests/tcp1.yaml", "manifests/tcp1-coll-address-port.yaml", "manifests/tcp1-coll-address-port-2.yaml", "manifests/tcp2.yaml"},
			parent:                 "tcp-1",
			hasCollision:           true,
			expectedCollisionsPath: "expectations/has-collisions/coll-address-port-3.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceList := getResourceList(t, tt.resourcePaths)

			for _, v := range resourceList.Items {
				v.ParentName = tt.parent
			}
			resourceList.Order()

			hasColl, got := resourceList.Items.HasCollisionAddressPort()
			if hasColl != tt.hasCollision {
				t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got = %v, want %v", tt.name, hasColl, tt.hasCollision)
			}
			var gotList store.TCPResourceList
			for _, v := range got {
				gotList = v
				gotList.OrderByCreationTime()
			}

			if tt.hasCollision == true {
				if got == nil {
					t.Errorf("test %s TCPs.HasCollisionAddressPort() got should not be nil", tt.name)
				}
				gotM, _ := json.Marshal(got)
				expectedCollisions := getCollisionMapFromFile(t, tt.expectedCollisionsPath)
				expected, _ := json.Marshal(expectedCollisions)
				if string(gotM) != string(expected) {
					t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got = %v, want %v", tt.name, string(gotM), string(expected))
				}
			}
		})
	}
}

func TestTCPs_HasCollisionFrontendName(t *testing.T) {
	tests := []struct {
		name                   string
		resourceListPath       []string
		parent                 string
		hasCollision           bool
		expectedCollisionsPath string
	}{
		{
			name:             "no collision-1",
			resourceListPath: []string{"manifests/tcp1.yaml", "manifests/tcp2.yaml"},
			parent:           "tcp-1",
		},
		{
			name:             "no collisions-2",
			resourceListPath: []string{"manifests/tcp3.yaml", "manifests/tcp4.yaml"},
			parent:           "tcp-1",
		},
		{
			name:                   "collision tcp1 and tcp1CollFeName on Frontend name",
			resourceListPath:       []string{"manifests/tcp1.yaml", "manifests/tcp2.yaml", "manifests/tcp3.yaml", "manifests/tcp4.yaml", "manifests/tcp1-coll-fe-name.yaml"},
			parent:                 "tcp-1",
			hasCollision:           true,
			expectedCollisionsPath: "expectations/has-collisions/coll-fe-name.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceList := getResourceList(t, tt.resourceListPath)

			for _, v := range resourceList.Items {
				v.ParentName = tt.parent
			}

			hasColl, got := resourceList.Items.HasCollisionFrontendName()
			var gotList store.TCPResourceList
			for _, v := range got {
				gotList = v
				gotList.OrderByCreationTime()
			}

			if hasColl != tt.hasCollision {
				t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got = %v, want %v", tt.name, hasColl, tt.hasCollision)
			}
			if tt.hasCollision == true {
				if got == nil {
					t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got should not be nil", tt.name)
				}
				gotM, _ := json.Marshal(got)
				expectedCollisions := getCollisionMapFromFile(t, tt.expectedCollisionsPath)
				expected, _ := json.Marshal(expectedCollisions)
				if string(gotM) != string(expected) {
					t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got = %v, want %v", tt.name, string(gotM), string(expected))
				}
			}
		})
	}
}

func TestTCPs_CheckCollision(t *testing.T) {
	tests := []struct {
		name             string
		resourceListPath []string
		parent           string
		hasCollision     bool
		expectedPath     string
	}{
		{
			name:             "no collision -1",
			resourceListPath: []string{"manifests/tcp1.yaml", "manifests/tcp2.yaml"},
			parent:           "tcp-1",
			expectedPath:     "expectations/no-collision.yaml",
		},
		{
			name:             "collision tcp1 and tcp1CollAddPort on address port",
			resourceListPath: []string{"manifests/tcp1.yaml", "manifests/tcp1-coll-address-port.yaml", "manifests/tcp2.yaml"},
			parent:           "tcp-1",
			hasCollision:     true,
			expectedPath:     "expectations/check-collisions/coll-address-port.yaml",
		},
		{
			name:             "collision tcp1 and tcp1CollFeName on Frontend name",
			resourceListPath: []string{"manifests/tcp1.yaml", "manifests/tcp1-coll-fe-name.yaml", "manifests/tcp3.yaml", "manifests/tcp4.yaml"},
			parent:           "tcp-1",
			hasCollision:     true,
			expectedPath:     "expectations/check-collisions/coll-fe-name.yaml",
		},
		{
			name:             "collision tcp1/tcp1CollFeName and tcp2/tcp2AddPort",
			resourceListPath: []string{"manifests/tcp1.yaml", "manifests/tcp1-coll-fe-name.yaml", "manifests/tcp2.yaml", "manifests/tcp2-coll-address-port.yaml"},
			parent:           "tcp-1",
			hasCollision:     true,
			expectedPath:     "expectations/check-collisions/2-collisions.yaml",
		},
		{
			name:             "collision tcp1/tcp1CollAddPort/tcp1CollAddPort2",
			resourceListPath: []string{"manifests/tcp1.yaml", "manifests/tcp1-coll-address-port.yaml", "manifests/tcp1-coll-address-port-2.yaml", "manifests/tcp2.yaml"},
			parent:           "tcp-1",
			hasCollision:     true,
			expectedPath:     "expectations/check-collisions/coll-address-port-2.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceList := getResourceList(t, tt.resourceListPath)

			for _, v := range resourceList.Items {
				v.ParentName = tt.parent
			}

			resourceList.Items.CheckCollision()
			resourceList.Items.OrderByCreationTime()
			got, _ := json.Marshal(resourceList.Items)

			expected := getTCPResourceListFromFile(t, tt.expectedPath)
			expectedM, _ := json.Marshal(expected)
			if string(got) != string(expectedM) {
				t.Errorf("test %s TCPs.CheckCollision() got = %v, want %v", tt.name, string(got), string(expectedM))
			}
		})
	}
}
