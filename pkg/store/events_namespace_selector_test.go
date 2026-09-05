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
	"testing"
	"time"

	"github.com/haproxytech/client-native/v6/models"
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	rc "github.com/haproxytech/kubernetes-ingress/pkg/reference-counter"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

func selectorStore() K8s {
	return NewK8sStore(utils.OSArgs{NamespaceLabelSelector: "watch=true"})
}

func addSelectedNamespace(k *K8s, name string) *Namespace {
	k.EventNamespace(nil, &Namespace{
		Name:   name,
		Status: ADDED,
		Labels: map[string]string{"watch": "true"},
	})
	return k.Namespaces[name]
}

func selectorTCP(ns, name, fe string, port int64) *TCPs {
	p := port
	return &TCPs{
		Name:      name,
		Namespace: ns,
		Status:    ADDED,
		Items: TCPResourceList{
			{
				ParentName: name,
				Namespace:  ns,
				TCPModel: v3.TCPModel{
					Name: "item",
					Frontend: models.Frontend{
						FrontendBase: models.FrontendBase{Name: fe},
						Binds: map[string]models.Bind{
							"bind": {Address: "0.0.0.0", Port: &p},
						},
					},
				},
			},
		},
	}
}

func selectorIngress(ns, name, svcNs, svcName string) *Ingress {
	return &Ingress{
		IngressCore: IngressCore{
			Namespace:    ns,
			Name:         name,
			CreationTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Rules: map[string]*IngressRule{
				"h": {
					Host: "app.example",
					Paths: map[string]*IngressPath{
						"/": {Path: "/", SvcNamespace: svcNs, SvcName: svcName},
					},
				},
			},
		},
		Status: ADDED,
	}
}

func TestGetNamespace_WhitelistStillCreatesMissing(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{NamespaceWhitelist: []string{"app"}})
	if k.NamespacesAccess.LabelSelectorActive {
		t.Fatal("whitelist must keep selector inactive")
	}
	ns := k.GetNamespace("missing")
	if _, ok := k.Namespaces["missing"]; !ok {
		t.Fatal("non-selector GetNamespace must insert")
	}
	if ns.Name != "missing" {
		t.Fatalf("name = %q", ns.Name)
	}
}

func TestGetNamespace_DefaultStillCreatesMissing(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	k.GetNamespace("default")
	if _, ok := k.Namespaces["default"]; !ok {
		t.Fatal("default GetNamespace must insert")
	}
}

func TestSelectorUnmatchRemovesStoreAndIndexes(t *testing.T) {
	k := selectorStore()
	ns := addSelectedNamespace(&k, "app")
	if !k.MarkNamespaceReady("app") {
		t.Fatal("MarkNamespaceReady")
	}

	ing := selectorIngress("app", "web", "app", "svc")
	k.EventIngress(ns, ing, types.UID("ing-1"), "1")
	k.EventService(ns, &Service{Namespace: "app", Name: "svc", Status: ADDED})
	k.EventSecret(ns, &Secret{Namespace: "app", Name: "tls", Status: ADDED})

	k.EventTCPCR("app", "tcp", selectorTCP("app", "tcp", "fe-app", 8000))
	storedTCP := k.Namespaces["app"].CRs.TCPsPerCR["tcp"]
	if storedTCP == nil || len(storedTCP.Items) == 0 {
		t.Fatal("expected stored TCP CR items")
	}
	fe := rc.HaproxyCfgResourceName("tcpcr_app_fe-app")
	k.FrontendRC.AddOwner(fe, storedTCP.Items[0].Owner())

	if k.IngressesByService["app/svc"] == nil {
		t.Fatal("expected IngressesByService entry")
	}

	k.EventNamespace(ns, &Namespace{Name: "app", Status: DELETED})

	if _, ok := k.Namespaces["app"]; ok {
		t.Fatal("selector DELETE must drop the namespace from the store")
	}
	if _, ok := k.NamespacesAccess.Selected["app"]; ok {
		t.Fatal("selector DELETE must unselect")
	}
	if set := k.IngressesByService["app/svc"]; set != nil && len(set.Items()) > 0 {
		t.Fatalf("IngressesByService leftover: %#v", set.Items())
	}
	if _, ok := k.IngressesByService["app/svc"]; ok {
		t.Fatal("empty IngressesByService key must be deleted")
	}
	if k.FrontendRC.HasOwners(fe) {
		t.Fatal("selector DELETE must drop TCP frontend owners")
	}
}

func TestSelectorUnmatchDoesNotResurrectOnLookup(t *testing.T) {
	k := selectorStore()
	addSelectedNamespace(&k, "app")
	k.EventNamespace(nil, &Namespace{Name: "app", Status: DELETED})

	got := k.GetNamespace("app")
	if _, ok := k.Namespaces["app"]; ok {
		t.Fatal("GetNamespace after unmatch must not insert")
	}
	if got.CRs == nil || got.Services == nil || got.Ingresses == nil || got.Secret == nil {
		t.Fatal("detached namespace must have nested maps initialized")
	}
}

func TestSelectorLateEventsAfterUnmatchDoNotStick(t *testing.T) {
	k := selectorStore()
	addSelectedNamespace(&k, "app")
	k.EventNamespace(nil, &Namespace{Name: "app", Status: DELETED})

	detached := k.GetNamespace("app")
	ing := selectorIngress("app", "web", "app", "svc")
	if k.EventIngress(detached, ing, types.UID("late"), "1") {
		t.Fatal("late Ingress ADDED on detached ns must be dropped")
	}
	if _, ok := k.Namespaces["app"]; ok {
		t.Fatal("late Ingress must not resurrect namespace")
	}
	if k.IngressesByService["app/svc"] != nil {
		t.Fatal("late Ingress must not write IngressesByService")
	}

	if k.EventService(detached, &Service{Namespace: "app", Name: "svc", Status: ADDED}) {
		t.Fatal("late Service ADDED must be dropped")
	}
	if k.EventSecret(detached, &Secret{Namespace: "app", Name: "tls", Status: ADDED}) {
		t.Fatal("late Secret ADDED must be dropped")
	}
	if k.EventTCPCR("app", "tcp", &TCPs{Name: "tcp", Namespace: "app", Status: ADDED}) {
		t.Fatal("late TCP CR ADDED must be dropped")
	}
}

func TestSelectorAddedDoesNotResetReady(t *testing.T) {
	k := selectorStore()
	addSelectedNamespace(&k, "app")
	if !k.MarkNamespaceReady("app") {
		t.Fatal("ready")
	}
	k.EventNamespace(nil, &Namespace{
		Name:   "app",
		Status: ADDED,
		Labels: map[string]string{"watch": "true", "extra": "1"},
	})
	ns := k.Namespaces["app"]
	if !ns.Relevant {
		t.Fatal("resync ADD must not reset Relevant on a Ready namespace")
	}
	if ns.Labels["extra"] != "1" {
		t.Fatalf("labels not updated: %#v", ns.Labels)
	}
}

func TestSelectorSkipIngressInConfigUsesIngressNamespace(t *testing.T) {
	k := selectorStore()
	addSelectedNamespace(&k, "ready")
	addSelectedNamespace(&k, "starting")
	k.MarkNamespaceReady("ready")

	readyNS := k.Namespaces["ready"]
	k.EventService(readyNS, &Service{Namespace: "ready", Name: "svc", Status: ADDED})

	foreign := selectorIngress("starting", "web", "ready", "svc")
	k.EventIngress(k.Namespaces["starting"], foreign, types.UID("f"), "1")

	if !k.SkipNamespaceInConfig(k.Namespaces["starting"]) {
		t.Fatal("starting namespace must be skipped")
	}
	if k.SkipNamespaceInConfig(k.Namespaces["ready"]) {
		t.Fatal("ready namespace must not be skipped")
	}
	if !k.SkipIngressInConfig(foreign) {
		t.Fatal("ingress from starting ns must be skipped even when its backend is in a ready ns")
	}
}

func TestSelectorGetSecretTreatsStartingAsMissing(t *testing.T) {
	k := selectorStore()
	addSelectedNamespace(&k, "app")
	k.EventSecret(k.Namespaces["app"], &Secret{Namespace: "app", Name: "tls", Status: ADDED})
	if _, err := k.GetSecret("app", "tls"); err == nil {
		t.Fatal("Starting namespace secrets must be treated as missing")
	}
	k.MarkNamespaceReady("app")
	if _, err := k.GetSecret("app", "tls"); err != nil {
		t.Fatalf("Ready namespace secrets must resolve: %v", err)
	}
}

func TestSelectorStartingTCPDoesNotCollideReady(t *testing.T) {
	k := selectorStore()
	addSelectedNamespace(&k, "ready")
	addSelectedNamespace(&k, "starting")
	k.MarkNamespaceReady("ready")

	k.EventTCPCR("ready", "tcp-ready", selectorTCP("ready", "tcp-ready", "fe-ready", 8000))
	k.EventTCPCR("starting", "tcp-start", selectorTCP("starting", "tcp-start", "fe-start", 8000))

	readyTCP := k.Namespaces["ready"].CRs.TCPsPerCR["tcp-ready"]
	if readyTCP.Items[0].CollisionStatus == ERROR {
		t.Fatalf("Starting TCP must not mark Ready TCP as colliding: %s", readyTCP.Items[0].Reason)
	}
}

func TestNonSelectorNamespaceDeleteDoesNotRunSelectorTeardown(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{})
	ns := k.GetNamespace("app")
	ing := selectorIngress("app", "web", "app", "svc")
	k.EventIngress(ns, ing, types.UID("1"), "1")
	k.EventNamespace(ns, &Namespace{Name: "app", Status: DELETED})
	if _, ok := k.Namespaces["app"]; ok {
		t.Fatal("namespace map entry should be gone")
	}
	// Master does not clean IngressesByService on namespace DELETE.
	if k.IngressesByService["app/svc"] == nil {
		t.Fatal("non-selector DELETE must not run selector teardown of IngressesByService")
	}
}

func TestSelectorPublishServiceClearedOnUnmatch(t *testing.T) {
	k := NewK8sStore(utils.OSArgs{
		NamespaceLabelSelector: "watch=true",
		PublishService:         "app/pub",
	})
	ns := addSelectedNamespace(&k, "app")
	k.EventService(ns, &Service{Namespace: "app", Name: "pub", Status: ADDED})
	k.EventPublishService(ns, &Service{Namespace: "app", Name: "pub", Status: ADDED, Addresses: []string{"1.2.3.4"}})
	if len(k.PublishServiceAddresses) == 0 {
		t.Fatal("publish addresses should be set")
	}
	k.EventNamespace(ns, &Namespace{Name: "app", Status: DELETED})
	if k.PublishServiceAddresses != nil {
		t.Fatalf("publish addresses leftover: %v", k.PublishServiceAddresses)
	}
	if !k.UpdateAllIngresses {
		t.Fatal("unmatch of publish service must set UpdateAllIngresses")
	}
}
