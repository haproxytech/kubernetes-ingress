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

	"github.com/haproxytech/client-native/v6/models"
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	rc "github.com/haproxytech/kubernetes-ingress/pkg/reference-counter"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const (
	TCPProtocolType string = "TCP"
)

type TCPResource struct {
	CreationTimestamp time.Time `json:"creation_timestamp" yaml:"creation_timestamp"` //nolint: tagliatelle
	// ParentName is the name of TCP CR containing this TCP resource
	ParentName      string `json:"parent_name,omitempty" yaml:"parent_name,omitempty"`           //nolint: tagliatelle
	Namespace       string `json:"namespace,omitempty" yaml:"namespace,omitempty"`               //nolint: tagliatelle
	CollisionStatus Status `json:"collision_status,omitempty" yaml:"collision_status,omitempty"` //nolint: tagliatelle
	// If Status is ERROR, Reason will contain the reason
	// Leave it as a fully flexible string
	Reason      string            `json:"reason,omitempty" yaml:"reason,omitempty"` //nolint: tagliatelle
	v3.TCPModel `json:"tcpmodel"` //nolint: tagliatellet
}

type TCPResourceList []*TCPResource

type TCPs struct {
	Status       Status          `json:"status,omitempty"`
	IngressClass string          `json:"ingress_class,omitempty"`
	Namespace    string          `json:"namespace,omitempty"`
	Name         string          `json:"name,omitempty"`
	Items        TCPResourceList `json:"items"`
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
	if a.IngressClass != b.IngressClass {
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

func AddressPort(b models.Bind) string {
	port := int64(0)
	if b.Port != nil {
		port = *b.Port
	}
	return b.Address + ":" + strconv.FormatInt(port, 10)
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
}

func (a TCPResourceList) OrderByCreationTime() {
	sort.SliceStable(a, func(i, j int) bool {
		if a[i].CreationTimestamp.Equal(a[j].CreationTimestamp) {
			return a[i].Name < a[j].Name
		}
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
	hasCollAddPort, collAddPorts := a.HasCollisionAddressPort()
	if hasCollAddPort {
		for _, collOneAddPort := range collAddPorts {
			// Set all Items to ERROR except the oldest one
			if len(collOneAddPort) > 0 {
				collOneAddPort[len(collOneAddPort)-1].CollisionStatus = ""
				collOneAddPort[len(collOneAddPort)-1].Reason = ""
			}
		}
	}

	hasCollFeName, collFeNames := a.HasCollisionFrontendName()
	if hasCollFeName {
		for _, collOneFeName := range collFeNames {
			// Set all Items to ERROR except the oldest one
			if len(collOneFeName) > 0 {
				collOneFeName[len(collOneFeName)-1].CollisionStatus = ""
				collOneFeName[len(collOneFeName)-1].Reason = ""
			}
		}
	}
}

type bindWithResource struct {
	bind     models.Bind
	resource *TCPResource
}

func (a *TCPResourceList) HasCollisionAddressPort() (bool, map[string]TCPResourceList) {
	// map [address:port] -> (map[resource name] *resource)
	collisions := make(map[string]map[string]*TCPResource)

	bindsWithResourcesMap := make(map[string]bindWithResource)

	for _, atcp := range *a {
		for _, aBind := range atcp.Frontend.Binds {
			if bBindWithResource, ok := bindsWithResourcesMap[AddressPort(aBind)]; ok {
				btcp := bBindWithResource.resource
				areEqual := atcp.Equal(btcp)
				if !areEqual {
					// Collision detected
					resKey := AddressPort(aBind)
					if _, ok := collisions[resKey]; !ok {
						collisions[resKey] = make(map[string]*TCPResource)
					}
					collisions[resKey][atcp.Name] = atcp
					collisions[resKey][btcp.Name] = btcp
					// res[res_key] = append(res[res_key], atcp, btcp)
					atcp.CollisionStatus = ERROR
					atcp.Reason += fmt.Sprintf("-- Collision AddPort %s with %s/%s ", AddressPort(bBindWithResource.bind), btcp.Namespace, btcp.WithParentName())
					btcp.CollisionStatus = ERROR
					btcp.Reason += fmt.Sprintf("-- Collision AddPort %s with %s/%s ", AddressPort(aBind), atcp.Namespace, atcp.WithParentName())
				}
				continue
			}
			bindsWithResourcesMap[AddressPort(aBind)] = bindWithResource{bind: aBind, resource: atcp}
		}
	}

	if len(collisions) != 0 {
		res := make(map[string]TCPResourceList)
		for k, v := range collisions {
			sl := mapValuesToSlice(v)
			sl.OrderByCreationTime()
			if _, ok := res[k]; !ok {
				res[k] = make([]*TCPResource, 0)
			}
			res[k] = append(res[k], sl...)
		}
		return true, res
	}

	return false, nil
}

type frontendNameWithResource struct {
	feName   string
	resource *TCPResource
}

func (a *TCPResourceList) HasCollisionFrontendName() (bool, map[string]TCPResourceList) {
	// map [feName] -> []resource
	res := make(map[string]TCPResourceList)
	feNameWithResource := make(map[string]frontendNameWithResource)

	for _, atcp := range *a {
		if bBindWithResource, ok := feNameWithResource[atcp.Frontend.Name]; ok {
			btcp := bBindWithResource.resource
			areEqual := atcp.Equal(btcp)
			if !areEqual {
				// Collision detected
				resKey := atcp.Frontend.Name
				if _, ok := res[resKey]; !ok {
					res[resKey] = make([]*TCPResource, 0)
				}
				res[resKey] = append(res[resKey], atcp, btcp)
				atcp.CollisionStatus = ERROR
				atcp.Reason += "-- Collision FE.Name with " + btcp.Namespace + "/" + btcp.WithParentName()
				btcp.CollisionStatus = ERROR
				btcp.Reason += "-- Collision FE.Name with " + atcp.Namespace + "/" + atcp.WithParentName()
			}
			continue
		}
		feNameWithResource[atcp.Frontend.Name] = frontendNameWithResource{feName: atcp.Frontend.Name, resource: atcp}
	}

	if len(res) != 0 {
		for _, v := range res {
			v.OrderByCreationTime()
		}
		return true, res
	}

	return false, nil
}

func mapValuesToSlice(m map[string]*TCPResource) TCPResourceList {
	res := make(TCPResourceList, 0)
	// res := make([]*TCPResource, 0)
	for _, v := range m {
		res = append(res, v)
	}
	return res
}
