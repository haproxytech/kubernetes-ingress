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

package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// ingressObject builds a minimal Ingress carrying the given spec.ingressClassName. An empty
// name means the field is unset, which is a distinct case: it is what the
// is-default-class annotation on an IngressClass resolves later, in the store.
func ingressObject(name, class string) *networkingv1.Ingress {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", UID: k8stypes.UID("uid-" + name), ResourceVersion: "1"},
	}
	if class != "" {
		ing.Spec.IngressClassName = &class
	}
	return ing
}

// ingested runs the ingress informer over the given objects and returns the names of the
// ingresses whose events reached the channel. The channel is buffered because the handlers
// write to it synchronously, with nobody consuming during the sync.
func ingested(t *testing.T, ingressClass string, objects ...*networkingv1.Ingress) []string {
	t.Helper()

	declared := make([]runtime.Object, 0, len(objects))
	for _, o := range objects {
		declared = append(declared, o)
	}
	client := fake.NewClientset(declared...)

	eventChan := make(chan k8ssync.SyncDataEvent, len(objects)+1)
	factory := informers.NewSharedInformerFactory(client, 0)
	informer := factory.Networking().V1().Ingresses().Informer()

	k := k8s{}
	k.addIngressHandlers(eventChan, informer, utils.OSArgs{IngressClass: ingressClass})

	stop := make(chan struct{})
	defer close(stop)
	factory.Start(stop)
	require.True(t, len(factory.WaitForCacheSync(stop)) > 0, "the ingress informer must have synced")

	names := []string{}
	for {
		select {
		case event := <-eventChan:
			ing, ok := event.Data.(*store.Ingress)
			require.True(t, ok, "an ingress informer must only emit ingress events")
			names = append(names, ing.Name)
		case <-time.After(200 * time.Millisecond):
			return names
		}
	}
}

// TestIngressWithForeignClassNameIsStillIngested is the regression test for the ingestion
// filter: the informer used to drop, on the sole comparison of spec.ingressClassName against
// --ingress.class, everything whose class was named otherwise.
//
// Two reasons the name is the wrong thing to compare. Matching is on the IngressClass's
// spec.controller, so a class named anything can point at us; and the informer cannot know,
// the IngressClass resources being in the store, which this handler has no access to. Which
// is why the decision belongs downstream, to K8s.IsIngressClassSupported.
func TestIngressWithForeignClassNameIsStillIngested(t *testing.T) {
	ingested := ingested(t, "haproxy",
		ingressObject("named-otherwise", "haproxy-external"),
		ingressObject("named-as-the-flag", "haproxy"),
		ingressObject("no-class-at-all", ""),
		ingressObject("someone-elses", "nginx"),
	)

	require.ElementsMatch(t,
		[]string{"named-otherwise", "named-as-the-flag", "no-class-at-all", "someone-elses"},
		ingested,
		"every ingress must reach the store: the class decision is taken there, not at ingestion")
}

// TestIngressOfAForeignClassIsIngestedSoItCanBeAdmittedLater is the case the filter made
// unrecoverable, stated from the ingestion side: an ingress on a class which is not ours today
// must be in the store anyway, because the class may be handed over to us later and no Ingress
// event follows a change to the IngressClass object. What is not ingested can never be
// reconsidered.
func TestIngressOfAForeignClassIsIngestedSoItCanBeAdmittedLater(t *testing.T) {
	require.Equal(t,
		[]string{"on-a-foreign-class"},
		ingested(t, "haproxy", ingressObject("on-a-foreign-class", "someone-else")),
		"an ingress refused today must be in the store, or the class being handed over to us later cannot be noticed")
}
