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
	client            *kubernetes.Clientset
	ingressClass      string
	updateIngresses   []*ingress.Ingress
	emptyIngressClass bool
}

func New(client *kubernetes.Clientset, ingressClass string, emptyIngressClass bool) UpdateStatusManager {
	return &UpdateStatusManagerImpl{
		client:            client,
		ingressClass:      ingressClass,
		emptyIngressClass: emptyIngressClass,
	}
}

func (m *UpdateStatusManagerImpl) AddIngress(ingress *ingress.Ingress) {
	m.updateIngresses = append(m.updateIngresses, ingress)
}

func (m *UpdateStatusManagerImpl) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
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
				i := ingress.New(ingResource, m.ingressClass, m.emptyIngressClass, a)
				supported := i.Supported(k, a)

				if (!supported && (len(ingResource.Addresses) == 0 || !utils.EqualSliceStringsWithoutOrder(k.PublishServiceAddresses, ingResource.Addresses))) ||
					(supported && utils.EqualSliceStringsWithoutOrder(k.PublishServiceAddresses, ingResource.Addresses)) {
					continue
				}

				if supported {
					ingResource.Addresses = k.PublishServiceAddresses
				} else {
					ingResource.Addresses = []string{""}
				}
				logger.Debugf("new ingress status ip address of '%s/%s' will be %+v", ingResource.Namespace, ingResource.Name, ingResource.Addresses)

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
	return err
}
