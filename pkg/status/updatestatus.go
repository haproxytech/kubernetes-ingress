package status

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/client-go/kubernetes"
)

var logger = utils.GetLogger()

type UpdateStatusManager interface {
	AddIngress(ingress *ingress.Ingress)
	Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error)
}

type UpdateStatusManagerImpl struct {
	client                     *kubernetes.Clientset
	ingressClass               string
	updateIngresses            []*ingress.Ingress
	emptyIngressClass          bool
	disableIngressStatusUpdate bool
}

func New(client *kubernetes.Clientset, ingressClass string, emptyIngressClass bool, disableIngressStatusUpdate bool) UpdateStatusManager {
	return &UpdateStatusManagerImpl{
		client:                     client,
		ingressClass:               ingressClass,
		emptyIngressClass:          emptyIngressClass,
		disableIngressStatusUpdate: disableIngressStatusUpdate,
	}
}

func (m *UpdateStatusManagerImpl) AddIngress(ingress *ingress.Ingress) {
	m.updateIngresses = append(m.updateIngresses, ingress)
}

// Update returns no error on purpose: the API calls happen in a goroutine which outlives
// this call, so nothing it could report would still be collectable here. It logs its own
// failures instead.
func (m *UpdateStatusManagerImpl) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) error {
	ingresses := m.updateIngresses

	if k.UpdateAllIngresses {
		ingresses = nil
		for _, namespace := range k.Namespaces {
			if !namespace.Relevant {
				continue
			}

			for _, ingResource := range namespace.Ingresses {
				i := ingress.New(ingResource, m.ingressClass, m.emptyIngressClass, a)
				ingresses = append(ingresses, i)
			}
		}
	}

	ingressesToUpdate := []*ingress.Ingress{}

	for _, ing := range ingresses {
		if ing == nil {
			continue
		}
		supported := ing.Supported(k)

		if (!supported && (len(ing.GetAddresses()) == 0 ||
			!utils.EqualSliceStringsWithoutOrder(k.PublishServiceAddresses, ing.GetAddresses()))) ||
			(supported && utils.EqualSliceStringsWithoutOrder(k.PublishServiceAddresses, ing.GetAddresses())) {
			continue
		}

		if supported {
			ing.SetAddresses(k.PublishServiceAddresses)
		} else {
			ing.SetAddresses([]string{""})
		}
		ingNamespacedName := ing.GetNamespacedName()
		logger.Debugf("new ingress status ip address of '%s/%s' will be %+v", ingNamespacedName.Namespace, ingNamespacedName.Name, ing.GetAddresses())

		ingressesToUpdate = append(ingressesToUpdate, ing)
	}

	if len(ingressesToUpdate) > 0 {
		go func() {
			for _, ing := range ingressesToUpdate {
				if ing != nil {
					logger.Error(ing.UpdateStatus(m.client, m.disableIngressStatusUpdate))
				}
			}
		}()
	}

	// UpdateAllIngresses is not reset here: this store is a copy, handlers receiving it by
	// value, so the assignment was a no-op and every sync took the sweep branch. K8s.Clean()
	// consumes it, with the Status fields it belongs with.
	m.updateIngresses = nil
	return nil
}
