package ingress

import (
	"context"
	"fmt"
	"net"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type UpdateStatus func(ingresses []*store.Ingress, publishServiceAddresses []string)

func NewStatusIngressUpdater(client *kubernetes.Clientset, k store.K8s, class string, emptyClass bool, a annotations.Annotations) UpdateStatus {
	return func(ingresses []*store.Ingress, publishServiceAddresses []string) {
		for _, ingress := range ingresses {
			if ing := New(k, ingress, class, emptyClass, a); ing != nil {
				logger.Error(ing.UpdateStatus(client, publishServiceAddresses))
			}
		}
	}
}

func (i *Ingress) UpdateStatus(client *kubernetes.Clientset, addresses []string) (err error) {
	var lbi []corev1.LoadBalancerIngress

	if utils.EqualSliceStringsWithoutOrder(i.resource.Addresses, addresses) {
		return
	}

	for _, addr := range addresses {
		if net.ParseIP(addr) == nil {
			lbi = append(lbi, corev1.LoadBalancerIngress{Hostname: addr})
		} else {
			lbi = append(lbi, corev1.LoadBalancerIngress{IP: addr})
		}
	}

	switch i.resource.APIVersion {
	case "networking.k8s.io/v1":
		var ingSource *networkingv1.Ingress
		ingSource, err = client.NetworkingV1().Ingresses(i.resource.Namespace).Get(context.Background(), i.resource.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = networkingv1.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
		_, err = client.NetworkingV1().Ingresses(i.resource.Namespace).UpdateStatus(context.Background(), ingCopy, metav1.UpdateOptions{})
	}

	if k8serror.IsNotFound(err) {
		return fmt.Errorf("update ingress status: failed to get ingress %s/%s: %w", i.resource.Namespace, i.resource.Name, err)
	}
	if err != nil {
		return fmt.Errorf("failed to update LoadBalancer status of ingress %s/%s: %w", i.resource.Namespace, i.resource.Name, err)
	}
	logger.Tracef("Successful update of LoadBalancer status in ingress %s/%s", i.resource.Namespace, i.resource.Name)
	// Allow to store the publish service addresses affected to the ingress for future comparison in update test.
	i.resource.Addresses = addresses
	return nil
}

func UpdatePublishService(ingresses []*Ingress, api *kubernetes.Clientset, publishServiceAddresses []string) {
	for _, i := range ingresses {
		logger.Error(i.UpdateStatus(api, publishServiceAddresses))
	}
}
