package status

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// storeWithIngress returns a store holding one ingress of the given class, and the ingress
// itself. The class resource is declared with the controller of this project suffixed by
// "haproxy", which is what --ingress.class=haproxy matches on.
func storeWithIngress(class string, controller string, addresses []string) (store.K8s, *store.Ingress) {
	k := store.NewK8sStore(utils.OSArgs{})
	k.IngressClasses[class] = &store.IngressClass{Name: class, Controller: controller, Status: store.ADDED}
	ns := k.GetNamespace("ns")
	ing := &store.Ingress{
		IngressCore: store.IngressCore{Namespace: "ns", Name: "app", Class: class},
		Status:      store.ADDED,
		Addresses:   addresses,
	}
	ns.Ingresses[ing.Name] = ing
	k.PublishServiceAddresses = []string{"192.168.1.1"}
	return k, ing
}

// TestQueuedIngressGetsThePublishAddresses is the fix: an ingress queued by the
// reconciliation, outside of a full sweep, used to reach UpdateStatus with whatever the
// conversion had read off the live object - so a freshly created ingress published the empty
// status it already had, until the next publish service event.
func TestQueuedIngressGetsThePublishAddresses(t *testing.T) {
	k, ing := storeWithIngress("haproxy", "haproxy.org/ingress-controller/haproxy", nil)
	m := New(nil, "haproxy", false, true)
	m.AddIngress(ingress.New(ing, "haproxy", false, nil))

	require.False(t, k.UpdateAllIngresses, "the point is the queued path, not the sweep")
	require.NoError(t, m.Update(k, haproxy.HAProxy{}, nil))

	require.Equal(t, k.PublishServiceAddresses, ing.Addresses,
		"a queued ingress must be given the addresses to publish, not the ones it already carried")
}

// TestQueuedIngressAlreadyRightIsLeftAlone pins the other half of the resolution: an ingress
// whose recorded addresses are already the ones to publish is dropped before any API call.
func TestQueuedIngressAlreadyRightIsLeftAlone(t *testing.T) {
	k, ing := storeWithIngress("haproxy", "haproxy.org/ingress-controller/haproxy", []string{"192.168.1.1"})
	m := New(nil, "haproxy", false, true)
	m.AddIngress(ingress.New(ing, "haproxy", false, nil))

	require.NoError(t, m.Update(k, haproxy.HAProxy{}, nil))

	require.Equal(t, []string{"192.168.1.1"}, ing.Addresses)
}

// TestSweepResolvesTheIngressesOfRelevantNamespacesOnly covers the other entry: a publish
// service event sets UpdateAllIngresses, and the sweep then rebuilds the list from the store
// rather than from what the reconciliation queued. Namespaces the controller does not watch
// are out of it - their ingresses are none of its business, and it holds no route for them.
func TestSweepResolvesTheIngressesOfRelevantNamespacesOnly(t *testing.T) {
	k, watched := storeWithIngress("haproxy", "haproxy.org/ingress-controller/haproxy", nil)
	other := k.GetNamespace("unwatched")
	other.Relevant = false
	ignored := &store.Ingress{
		IngressCore: store.IngressCore{Namespace: other.Name, Name: "app", Class: "haproxy"},
		Status:      store.ADDED,
	}
	other.Ingresses[ignored.Name] = ignored

	k.UpdateAllIngresses = true
	m := New(nil, "haproxy", false, true)
	require.NoError(t, m.Update(k, haproxy.HAProxy{}, nil))

	require.Equal(t, k.PublishServiceAddresses, watched.Addresses)
	require.Nil(t, ignored.Addresses, "an ingress of a namespace the controller does not watch must be left alone")
}

// TestUpdateEmptiesItsQueue pins that a queued ingress is consumed, not kept: the queue is what
// one reconciliation asked to publish. Were it kept, every later sync would resolve and publish
// it again for as long as the controller lives.
func TestUpdateEmptiesItsQueue(t *testing.T) {
	k, ing := storeWithIngress("haproxy", "haproxy.org/ingress-controller/haproxy", nil)
	m := New(nil, "haproxy", false, true)
	m.AddIngress(ingress.New(ing, "haproxy", false, nil))

	require.NoError(t, m.Update(k, haproxy.HAProxy{}, nil))
	require.Equal(t, k.PublishServiceAddresses, ing.Addresses)

	// Whatever happens to the ingress afterwards, an Update nobody asked anything of must not
	// touch it again.
	ing.Addresses = nil
	require.NoError(t, m.Update(k, haproxy.HAProxy{}, nil))
	require.Nil(t, ing.Addresses, "the ingress was consumed by the first Update, nothing must resolve it again")
}

// TestIngressWhichNoLongerQualifiesIsMarked covers the branch kept on purpose: an ingress
// still carrying our addresses while its class no longer points at us is recorded with an
// empty one. That is what makes it visible - in the status and in the debug line - rather
// than leaving a stale address behind.
func TestIngressWhichNoLongerQualifiesIsMarked(t *testing.T) {
	k, ing := storeWithIngress("shared", "example.com/other-controller", []string{"192.168.1.1"})
	m := New(nil, "haproxy", false, true)
	m.AddIngress(ingress.New(ing, "haproxy", false, nil))

	require.NoError(t, m.Update(k, haproxy.HAProxy{}, nil))

	require.Equal(t, []string{""}, ing.Addresses,
		"an ingress which had our addresses and no longer qualifies must be marked, not left as it was")
}
