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
	"strings"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"sigs.k8s.io/yaml"
)

//go:embed manifests/tcp1.yaml
var mtcp1 []byte

//go:embed manifests/tcp1-coll-address-port.yaml
var mtcp1CollAddPort []byte

//go:embed manifests/tcp1-coll-fe-name.yaml
var mtcp1CollFeName []byte

//go:embed manifests/tcp2.yaml
var mtcp2 []byte

//go:embed manifests/tcp3.yaml
var mtcp3 []byte

//go:embed manifests/tcp4.yaml
var mtcp4 []byte

//go:embed manifests/unordered.yaml
var mUnordered []byte

//go:embed expectations/ordered.yaml
var mExpectationsOrdered []byte

//go:embed expectations/has-collisions/coll-address-port.yaml
var mExpectedHasCollAddPort []byte

//go:embed expectations/no-collision.yaml
var mExpectedNoColl []byte

//go:embed expectations/has-collisions/coll-fe-name.yaml
var mExpectedHasCollFeName []byte

//go:embed expectations/check-collisions/coll-address-port.yaml
var mExpectedCheckCollAddPort []byte

//go:embed expectations/check-collisions/coll-fe-name.yaml
var mExpectedCheckCollFeName []byte

var (
	tcp1                        *store.TCPResource
	tcp2                        *store.TCPResource
	tcp1CollAddPort             *store.TCPResource
	tcp1CollFeName              *store.TCPResource
	tcp3                        *store.TCPResource
	tcp4                        *store.TCPResource
	tcpExpectedHasCollAddPort   *store.TCPResourceList
	tcpExpectedNoColl           *store.TCPResourceList
	tcpExpectedHasCollFeName    *store.TCPResourceList
	tcpExpectedCheckCollAddPort *store.TCPResourceList
	tcpExpectedCheckCollFeName  *store.TCPResourceList
	tcpUnordered                *store.TCPs
	tcpExpectedOrdered          *store.TCPs
)

func initFromYaml() {
	tcp1 = &store.TCPResource{}
	_ = yaml.Unmarshal(mtcp1, tcp1)
	tcp2 = &store.TCPResource{}
	_ = yaml.Unmarshal(mtcp2, tcp2)
	tcp1CollAddPort = &store.TCPResource{}
	_ = yaml.Unmarshal(mtcp1CollAddPort, tcp1CollAddPort)
	tcp1CollFeName = &store.TCPResource{}
	_ = yaml.Unmarshal(mtcp1CollFeName, tcp1CollFeName)
	tcp3 = &store.TCPResource{}
	_ = yaml.Unmarshal(mtcp3, tcp3)
	tcp4 = &store.TCPResource{}
	_ = yaml.Unmarshal(mtcp4, tcp4)
	tcpExpectedHasCollAddPort = &store.TCPResourceList{}
	_ = yaml.Unmarshal(mExpectedHasCollAddPort, tcpExpectedHasCollAddPort)
	tcpExpectedCheckCollAddPort = &store.TCPResourceList{}
	_ = yaml.Unmarshal(mExpectedCheckCollAddPort, tcpExpectedCheckCollAddPort)
	tcpExpectedNoColl = &store.TCPResourceList{}
	_ = yaml.Unmarshal(mExpectedNoColl, tcpExpectedNoColl)
	tcpExpectedHasCollFeName = &store.TCPResourceList{}
	_ = yaml.Unmarshal(mExpectedHasCollFeName, tcpExpectedHasCollFeName)
	tcpExpectedCheckCollFeName = &store.TCPResourceList{}
	_ = yaml.Unmarshal(mExpectedCheckCollFeName, tcpExpectedCheckCollFeName)
	tcpUnordered = &store.TCPs{}
	_ = yaml.Unmarshal(mUnordered, tcpUnordered)
	tcpExpectedOrdered = &store.TCPs{}
	_ = yaml.Unmarshal(mExpectationsOrdered, tcpExpectedOrdered)
}

func TestTCPs_Order(t *testing.T) {
	initFromYaml()
	tests := []struct {
		name      string
		unordered *store.TCPs
		expected  *store.TCPs
	}{
		{
			name:      "test_1",
			unordered: tcpUnordered,
			expected:  tcpExpectedOrdered,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.unordered.Order()
			got, _ := json.Marshal(tt.unordered)
			want, _ := json.Marshal(tt.expected)

			exp := strings.ReplaceAll(string(want), "\n", "")
			if exp != string(got) {
				t.Errorf("Order() = %v, want %v", string(got), exp)
			}
		})
	}
}

func TestTCPs_Equal(t *testing.T) {
	initFromYaml()
	tests := []struct {
		name string
		a    *store.TCPs
		b    *store.TCPs
		want bool
	}{
		{
			name: "equality",
			a:    &store.TCPs{Items: store.TCPResourceList{tcp1, tcp2}},
			b:    &store.TCPs{Items: store.TCPResourceList{tcp1, tcp2}},
			want: true,
		},
		{
			name: "equality - between same object unordered and ordered",
			a:    tcpUnordered,
			b:    tcpExpectedOrdered,
			want: true,
		},
		{
			name: "not equal",
			a:    &store.TCPs{Items: store.TCPResourceList{tcp1, tcp3}},
			b:    &store.TCPs{Items: store.TCPResourceList{tcp1, tcp2}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Errorf("%s TCPs.Equal() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestTCPs_HasCollisionAddressPort(t *testing.T) {
	initFromYaml()
	tests := []struct {
		name               string
		resource           *store.TCPResource
		resourceList       *store.TCPs
		parent             string
		hasCollision       bool
		expectedCollisions *store.TCPResourceList
	}{
		{
			name:         "no collision-1",
			resource:     tcp3,
			resourceList: &store.TCPs{Items: store.TCPResourceList{tcp1, tcp2}},
			parent:       "tcp-1",
		},
		{
			name:         "no collisions-2",
			resource:     tcp3,
			resourceList: &store.TCPs{Items: store.TCPResourceList{tcp3, tcp4}},
			parent:       "tcp-1",
		},
		{
			name:               "collision tcp2 and tcp1CollAddPort on address port",
			resource:           tcp1, // has collision with tcp1CollAddPort on AddressPort
			resourceList:       &store.TCPs{Items: store.TCPResourceList{tcp1CollAddPort, tcp2}},
			parent:             "tcp-1",
			hasCollision:       true,
			expectedCollisions: tcpExpectedHasCollAddPort,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.resource.ParentName = tt.parent
			for _, v := range tt.resourceList.Items {
				v.ParentName = tt.parent
			}
			tt.resourceList.Order()

			hasColl, got := tt.resource.HasCollisionAddressPort(tt.resourceList.Items)
			if hasColl != tt.hasCollision {
				t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got = %v, want %v", tt.name, hasColl, tt.hasCollision)
			}
			if tt.hasCollision == true {
				if got == nil {
					t.Errorf("test %s TCPs.HasCollisionAddressPort() got should not be nil", tt.name)
				}
				gotM, _ := json.Marshal(got)
				expected, _ := json.Marshal(tt.expectedCollisions)
				if string(gotM) != string(expected) {
					t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got = %v, want %v", tt.name, string(gotM), string(expected))
				}
			}
		})
	}
}

func TestTCPs_HasCollisionFrontendName(t *testing.T) {
	initFromYaml()
	tests := []struct {
		name               string
		resource           *store.TCPResource
		resourceList       *store.TCPs
		parent             string
		hasCollision       bool
		expectedCollisions *store.TCPResourceList
	}{
		{
			name:         "no collision-1",
			resource:     tcp1,
			resourceList: &store.TCPs{Items: store.TCPResourceList{tcp1, tcp2}},
			parent:       "tcp-1",
		},
		{
			name:         "no collisions-2",
			resource:     tcp3,
			resourceList: &store.TCPs{Items: store.TCPResourceList{tcp3, tcp4}},
			parent:       "tcp-1",
		},
		{
			name:               "collision tcp1 and tcp1CollFeName on Frontend name",
			resource:           tcp1,
			resourceList:       &store.TCPs{Items: store.TCPResourceList{tcp1CollFeName, tcp2, tcp3, tcp4}},
			parent:             "tcp-1",
			hasCollision:       true,
			expectedCollisions: tcpExpectedHasCollFeName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.resource.ParentName = tt.parent
			for _, v := range tt.resourceList.Items {
				v.ParentName = tt.parent
			}

			hasColl, got := tt.resource.HasCollisionFrontendName(tt.resourceList.Items)
			if hasColl != tt.hasCollision {
				t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got = %v, want %v", tt.name, hasColl, tt.hasCollision)
			}
			if tt.hasCollision == true {
				if got == nil {
					t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got should not be nil", tt.name)
				}
				gotM, _ := json.Marshal(got)
				expected, _ := json.Marshal(tt.expectedCollisions)
				if string(gotM) != string(expected) {
					t.Errorf("test %s TCPs.TestTCPs_HasCollisionFrontendName() got = %v, want %v", tt.name, string(gotM), string(expected))
				}
			}
		})
	}
}

func TestTCPs_CheckCollision(t *testing.T) {
	initFromYaml()

	tests := []struct {
		name         string
		resourceList store.TCPResourceList
		parent       string
		hasCollision bool
		expected     *store.TCPResourceList
	}{
		{
			name:         "no collision -1",
			resourceList: store.TCPResourceList{tcp1, tcp2},
			parent:       "tcp-1",
			expected:     tcpExpectedNoColl,
		},
		{
			name:         "collision tcp1 and tcp1CollAddPort on address port",
			resourceList: store.TCPResourceList{tcp1, tcp1CollAddPort, tcp2},
			parent:       "tcp-1",
			hasCollision: true,
			expected:     tcpExpectedCheckCollAddPort,
		},
		{
			name:         "collision tcp1 and tcp1CollFeName on Frontend name",
			resourceList: store.TCPResourceList{tcp1, tcp1CollFeName, tcp2, tcp3, tcp4},
			parent:       "tcp-1",
			hasCollision: true,
			expected:     tcpExpectedCheckCollFeName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, v := range tt.resourceList {
				v.ParentName = tt.parent
			}

			tt.resourceList.CheckCollision()
			got, _ := json.Marshal(tt.resourceList)
			expected, _ := json.Marshal(tt.expected)
			if string(got) != string(expected) {
				t.Errorf("test %s TCPs.CheckCollision() got = %v, want %v", tt.name, string(got), string(expected))
			}
		})
	}
}
