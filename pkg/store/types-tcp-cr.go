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
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/haproxytech/client-native/v5/models"
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	rc "github.com/haproxytech/kubernetes-ingress/pkg/reference-counter"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const (
	TCPProtocolType string = "TCP"
)

type TCPResource struct {
	CreationTimestamp time.Time `json:"creation_timestamp"` //nolint: tagliatelle
	// ParentName is the name of TCP CR containing this TCP resource
	ParentName      string `json:"parent_name,omitempty"` //nolint: tagliatelle
	Namespace       string `json:"namespace,omitempty"`
	CollisionStatus Status `json:"collision_status,omitempty"` //nolint: tagliatelle
	// If Status is ERROR, Reason will contain the reason
	// Leave it as a fully flexible string
	Reason      string `json:"reason,omitempty"`
	v1.TCPModel `json:"tcpmodel"`
}

type TCPResourceList []*TCPResource

type TCPs struct {
	Status    Status          `json:"status,omitempty"`
	Namespace string          `json:"namespace,omitempty"`
	Name      string          `json:"name,omitempty"`
	Items     TCPResourceList `json:"items"`
}

func (a *TCPs) Equal(b *TCPs, opt ...models.Options) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if a.Namespace != b.Namespace {
		return false
	}
	// Always ordered before being added into the store, so no need to order here
	a.Items.Order()
	b.Items.Order()

	return utils.EqualSlice(a.Items, b.Items)
}

func (a *TCPResource) Equal(b *TCPResource, opt ...models.Options) bool {
	return a.TCPModel.Equal(b.TCPModel, opt...)
}

func (a TCPResource) WithParentName() string {
	return a.ParentName + "/" + a.Name
}

func AddressPort(b *models.Bind) string {
	if b == nil {
		return ""
	}
	port := int64(0)
	if b.Port != nil {
		port = *b.Port
	}
	return b.Address + ":" + strconv.FormatInt(port, 10)
}

func (a *TCPResource) OrderBinds() {
	if a.Frontend.Binds == nil {
		return
	}
	sort.SliceStable(a.TCPModel.Frontend.Binds, func(i, j int) bool {
		return AddressPort(a.TCPModel.Frontend.Binds[i]) < AddressPort(a.TCPModel.Frontend.Binds[j])
	})
}

func (a TCPResource) Owner() rc.Owner {
	return rc.NewOwner(rc.TCP_CR, a.Namespace, a.WithParentName())
}

// Order sorts a TCPResourceList based on:
// - first the TCP name
// - then the CreationTime descending if same names
func (a TCPResourceList) Order() {
	sort.SliceStable(a, func(i, j int) bool {
		// Sort on TCP names
		if a[i].TCPModel.Name != a[j].TCPModel.Name {
			return a[i].TCPModel.Name < a[j].TCPModel.Name
		}
		// Then CreationTime
		return a[i].CreationTimestamp.After(a[j].CreationTimestamp)
	})
	for _, v := range a {
		v.OrderBinds()
	}
}

func (a TCPResourceList) OrderByCreationTime() {
	sort.SliceStable(a, func(i, j int) bool {
		return (a)[i].CreationTimestamp.After((a)[j].CreationTimestamp)
	})
}

func (a TCPs) Order() {
	a.Items.Order()
}

func (a TCPResourceList) resetCollisionStatus() {
	for _, v := range a {
		v.CollisionStatus = ""
		v.Reason = ""
	}
}

func (a TCPResourceList) CheckCollision() {
	a.resetCollisionStatus()
	for i, atcp := range a {
		nextelems := a[i+1:]
		hasCollAddPort, bCollAddPorts := atcp.HasCollisionAddressPort(nextelems)
		hasCollFeName, bCollFeNames := atcp.HasCollisionFrontendName(nextelems)
		if hasCollAddPort || hasCollFeName {
			collisions := make(TCPResourceList, 0)
			collisions = append(collisions, atcp)
			collisions = append(collisions, bCollAddPorts...)
			collisions = append(collisions, bCollFeNames...)
			collisions.OrderByCreationTime()
			// Set all Items to ERROR except the oldest one
			if len(collisions) > 0 {
				collisions[len(collisions)-1].CollisionStatus = ""
				collisions[len(collisions)-1].Reason = ""
			}
		}
	}
}

func (a *TCPResource) HasCollisionAddressPort(b TCPResourceList) (bool, []*TCPResource) {
	res := make(map[string]*TCPResource)

	for _, btcp := range b {
		// Collision on Address Port
		for i, aBind := range a.TCPModel.Frontend.Binds {
			bBinds := btcp.TCPModel.Frontend.Binds[i:]
			for _, bBind := range bBinds {
				aAddressPort := AddressPort(aBind)
				bAddressPort := AddressPort(bBind)
				if aAddressPort == bAddressPort {
					areEqual := a.Equal(btcp)
					if !areEqual {
						// Collision detected
						res[btcp.Name] = btcp
						a.CollisionStatus = ERROR
						a.Reason += fmt.Sprintf("- Collistion AddPort %s with %s/%s ", bAddressPort, btcp.Namespace, btcp.WithParentName())
						btcp.CollisionStatus = ERROR
						btcp.Reason += fmt.Sprintf("- Collistion AddPort %s with %s/%s ", aAddressPort, a.Namespace, a.WithParentName())
					}
				}
			}
		}
	}
	if len(res) != 0 {
		return true, mapValuesToSlice(res)
	}

	return false, nil
}

func (a *TCPResource) HasCollisionFrontendName(b TCPResourceList) (bool, []*TCPResource) {
	res := make(map[string]*TCPResource)
	for _, btcp := range b {
		// Collision on Frontend Name
		if a.Frontend.Name == btcp.Frontend.Name && !a.Equal(btcp) {
			// Collision detected
			res[btcp.Name] = btcp
			a.CollisionStatus = ERROR
			a.Reason += "- Collistion FE.Name with " + btcp.Namespace + "/" + btcp.WithParentName()
			btcp.CollisionStatus = ERROR
			btcp.Reason += "- Collistion FE.Name with " + a.Namespace + "/" + a.WithParentName()
		}
	}
	if len(res) != 0 {
		return true, mapValuesToSlice(res)
	}

	return false, nil
}

func mapValuesToSlice(m map[string]*TCPResource) []*TCPResource {
	res := make([]*TCPResource, 0)
	for _, v := range m {
		res = append(res, v)
	}
	return res
}
