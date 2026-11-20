package store

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func newPublishTestStore() (*K8s, *Namespace) {
	k := &K8s{
		PublishService: &utils.NamespaceValue{Namespace: "haproxy-controller", Name: "publish"},
	}
	ns := &Namespace{
		Name:     "haproxy-controller",
		Relevant: true,
		Services: map[string]*Service{
			"publish": {Namespace: "haproxy-controller", Name: "publish", Status: EMPTY},
		},
	}
	k.Namespaces = map[string]*Namespace{ns.Name: ns}
	return k, ns
}

// TestNewK8sStorePublishService covers --publish-service staying optional: an
// unset or malformed value leaves the mirroring disabled instead of failing.
func TestNewK8sStorePublishService(t *testing.T) {
	cases := []struct {
		value string
		want  *utils.NamespaceValue
	}{
		{value: ""},
		{value: "nonsense"},
		{value: "ns/publish", want: &utils.NamespaceValue{Namespace: "ns", Name: "publish"}},
	}
	for _, tc := range cases {
		k := NewK8sStore(utils.OSArgs{PublishService: tc.value})
		require.Equal(t, tc.want, k.PublishService, "--publish-service=%q", tc.value)
	}
}

// TestEventServicePublishAddressChanges covers the transitions of the external
// addresses of the publish Service. PublishServiceAddresses is the only previous
// state, so a metadata change arriving in the same update as the address change -
// what a LoadBalancer controller typically produces - cannot hide it.
func TestEventServicePublishAddressChanges(t *testing.T) {
	cases := []struct {
		name        string
		published   []string
		addresses   []string
		wantPublish bool
	}{
		{name: "allocation with a metadata change", addresses: []string{"192.0.2.10"}, wantPublish: true},
		{name: "replacement with a metadata change", published: []string{"192.0.2.10"}, addresses: []string{"192.0.2.11"}, wantPublish: true},
		{name: "removal with a metadata change", published: []string{"192.0.2.10"}, wantPublish: true},
		{name: "unchanged addresses with a metadata change", published: []string{"192.0.2.10"}, addresses: []string{"192.0.2.10"}},
		{name: "reordered addresses", published: []string{"192.0.2.10", "192.0.2.11"}, addresses: []string{"192.0.2.11", "192.0.2.10"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, ns := newPublishTestStore()
			k.PublishServiceAddresses = tc.published

			changed := k.EventService(ns, &Service{
				Namespace:   ns.Name,
				Name:        "publish",
				Status:      MODIFIED,
				Addresses:   tc.addresses,
				Annotations: map[string]string{"metallb.io/ip-allocated-from-pool": "pool"},
			})
			if !changed {
				t.Fatal("the metadata change must still update the routing model")
			}
			if k.UpdateAllIngresses != tc.wantPublish {
				t.Fatalf("UpdateAllIngresses=%v, want %v", k.UpdateAllIngresses, tc.wantPublish)
			}
			if !utils.EqualSliceStringsWithoutOrder(k.PublishServiceAddresses, tc.addresses) {
				t.Fatalf("published %v, want %v", k.PublishServiceAddresses, tc.addresses)
			}
		})
	}
}

// TestEventServicePublishAddressOnlyChange is the case the routing comparison
// cannot see: nothing else about the Service changed, only its addresses.
func TestEventServicePublishAddressOnlyChange(t *testing.T) {
	k, ns := newPublishTestStore()
	k.PublishServiceAddresses = []string{"192.0.2.10"}

	changed := k.EventService(ns, &Service{
		Namespace: ns.Name,
		Name:      "publish",
		Status:    MODIFIED,
		Addresses: []string{"192.0.2.11"},
	})
	if !changed || !k.UpdateAllIngresses {
		t.Fatalf("an address-only change must publish: changed=%v UpdateAllIngresses=%v",
			changed, k.UpdateAllIngresses)
	}
	if !utils.EqualSliceStringsWithoutOrder(k.PublishServiceAddresses, []string{"192.0.2.11"}) {
		t.Fatalf("published %v", k.PublishServiceAddresses)
	}
}

func TestEventServicePublishDeleteClearsTheCache(t *testing.T) {
	k, ns := newPublishTestStore()
	k.PublishServiceAddresses = []string{"192.0.2.10"}

	changed := k.EventService(ns, &Service{
		Namespace: ns.Name,
		Name:      "publish",
		Status:    DELETED,
	})
	if !changed || !k.UpdateAllIngresses || len(k.PublishServiceAddresses) != 0 {
		t.Fatalf("delete must clear the published addresses: changed=%v updateAll=%v published=%v",
			changed, k.UpdateAllIngresses, k.PublishServiceAddresses)
	}
}

func TestEventServiceOtherServiceLeavesThePublishAddressesAlone(t *testing.T) {
	k, ns := newPublishTestStore()
	k.PublishServiceAddresses = []string{"192.0.2.10"}

	changed := k.EventService(ns, &Service{
		Namespace: ns.Name,
		Name:      "other",
		Status:    ADDED,
		Addresses: []string{"192.0.2.99"},
	})
	if !changed {
		t.Fatal("a new Service must still update the routing model")
	}
	if k.UpdateAllIngresses || len(k.PublishServiceAddresses) != 1 {
		t.Fatalf("another Service must not republish: updateAll=%v published=%v",
			k.UpdateAllIngresses, k.PublishServiceAddresses)
	}
}
