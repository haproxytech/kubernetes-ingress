// Copyright 2026 HAProxy Technologies LLC
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

package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/haproxytech/kubernetes-ingress/pkg/status"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestNewIngressPublishesStatusAfterUpdatesBeforeReconciliation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		update func(*networkingv1.Ingress)
	}{
		{name: "create only"},
		{
			name: "Gardener DNS finalizer",
			update: func(i *networkingv1.Ingress) {
				i.Finalizers = []string{"garden.dns.gardener.cloud/ingress-dns"}
			},
		},
		{
			name: "annotation",
			update: func(i *networkingv1.Ingress) {
				i.Annotations = map[string]string{"haproxy.org/ssl-redirect": "false"}
			},
		},
		{
			name: "spec",
			update: func(i *networkingv1.Ingress) {
				i.Spec.Rules[0].Host = "updated.example.ingress"
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, resource := setupIngressStatusPublication(t)
			storeIngressEvent(t, c, resource, store.ADDED)
			if tc.update != nil {
				resource = resource.DeepCopy()
				resource.ResourceVersion = "2"
				tc.update(resource)
				storeIngressEvent(t, c, resource, store.MODIFIED)
			}

			queued := publishReconciledIngresses(t, c)

			require.Equal(t, 1, queued, "the initial status publication must survive updates before reconciliation")
			ing := c.store.GetNamespace(resource.Namespace).Ingresses[resource.Name]
			require.Equal(t, c.store.PublishServiceAddresses, ing.Addresses)
			require.False(t, ing.ClassUpdated, "the ingress class did not change")
		})
	}
}

func TestPublishedIngressMetadataUpdateDoesNotQueueInitialPublication(t *testing.T) {
	c, resource := setupIngressStatusPublication(t)
	storeIngressEvent(t, c, resource, store.ADDED)
	require.Equal(t, 1, publishReconciledIngresses(t, c))
	c.store.Clean()

	// The status was observed before another controller added its finalizer.
	resource.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{IP: "192.0.2.10"}}
	resource.ResourceVersion = "2"
	resource.Finalizers = []string{"garden.dns.gardener.cloud/ingress-dns"}
	storeIngressEvent(t, c, resource, store.MODIFIED)

	require.Zero(t, publishReconciledIngresses(t, c), "an established ingress must not be queued as newly added")
	ing := c.store.GetNamespace(resource.Namespace).Ingresses[resource.Name]
	require.Equal(t, store.EMPTY, ing.Status)
	require.False(t, ing.ClassUpdated)
	require.Equal(t, c.store.PublishServiceAddresses, ing.Addresses)
}

func setupIngressStatusPublication(t *testing.T) (*HAProxyController, *networkingv1.Ingress) {
	t.Helper()
	c := controllerWithClass(t)
	declareIngressClass(c, "haproxy", ourController)
	addBackendService(t, c.store, c.store.GetNamespace("ns"), "app")
	// The publish Service address was already cached at startup. No Service event
	// requests a full sweep while these Ingress events are processed.
	c.store.PublishServiceAddresses = []string{"192.0.2.10"}

	class := "haproxy"
	pathType := networkingv1.PathTypePrefix
	resource := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "app-ing", UID: "app-ing", ResourceVersion: "1"},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &class,
			Rules: []networkingv1.IngressRule{{
				Host: "example.ingress",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Path: "/", PathType: &pathType,
						Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
							Name: "app", Port: networkingv1.ServiceBackendPort{Name: "http"},
						}},
					}},
				}},
			}},
		},
	}
	return c, resource
}

func storeIngressEvent(t *testing.T, c *HAProxyController, resource *networkingv1.Ingress, eventStatus store.Status) {
	t.Helper()
	ing, err := store.ConvertToIngress(resource, false)
	require.NoError(t, err)
	ing.Status = eventStatus
	c.store.EventIngress(c.store.GetNamespace(resource.Namespace), ing, resource.UID, resource.ResourceVersion)
}

// publishReconciledIngresses checks both admission to the status queue and the
// resulting addresses. Kubernetes API writes are disabled in this test harness.
func publishReconciledIngresses(t *testing.T, c *HAProxyController) int {
	t.Helper()
	require.False(t, c.store.UpdateAllIngresses, "publication must use the individual ingress queue")
	recorder := &recordingStatusManager{}
	c.updateStatusManager = recorder
	reconcile(t, c)

	publisher := status.New(nil, c.osArgs.IngressClass, c.osArgs.EmptyIngressClass, true)
	for _, ing := range recorder.queued {
		publisher.AddIngress(ing)
	}
	require.NoError(t, publisher.Update(c.store, c.haproxy, c.annotations))
	return len(recorder.queued)
}
