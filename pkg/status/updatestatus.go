package status

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/client-go/kubernetes"
)

type UpdateStatusManager interface {
	AddIngress(ingress *ingress.Ingress)
	Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (reload bool, err error)
}

type UpdateStatusManagerImpl struct {
	updateIngresses []*ingress.Ingress
	client          *kubernetes.Clientset
}

func New(client *kubernetes.Clientset) UpdateStatusManager {
	return &UpdateStatusManagerImpl{
		client: client,
	}
}

func (m *UpdateStatusManagerImpl) AddIngress(ingress *ingress.Ingress) {
	m.updateIngresses = append(m.updateIngresses, ingress)
}

func (m *UpdateStatusManagerImpl) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (reload bool, err error) {
	errs := utils.Errors{}
	defer func() {
		err = errs.Result()
	}()

	ingresses := m.updateIngresses

	if k.UpdateAllIngresses {
		ingresses = nil
		for _, namespace := range k.Namespaces {
			if !namespace.Relevant {
				continue
			}

			for _, ingResource := range namespace.Ingresses {
				previousAddresses := ingResource.Addresses
				i := ingress.New(k, ingResource, "haproxy", false, a)
				supported := i.Supported(k, a)
				if supported {
					ingResource.Addresses = k.PublishServiceAddresses
				}
				// If ingress is not managed by us, three cases can occur:
				// - it has no adddresses.
				// - it has addresses and they are different (maybe managed by someone else).
				// - it has addresses and they are ours. We managed it but not now anymore.
				// You can see we can't easily manage the case when while the IC is stopped, the ingress switches to unmanaged state (ingress class change) and the publish service addresses have also changed.
				if !supported && !utils.EqualSliceStringsWithoutOrder(previousAddresses, ingResource.Addresses) {
					continue
				}
				ingresses = append(ingresses, i)
			}
		}
	}

	if len(ingresses) > 0 {
		go func() {
			for _, ing := range ingresses {
				if ing != nil {
					errs.Add(ing.UpdateStatus(m.client))
				}
			}
		}()
	}

	k.UpdateAllIngresses = false
	m.updateIngresses = nil
	return
}
