package ingress

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta "k8s.io/api/networking/v1beta1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type UpdateStatus func(service store.Service, ingresses []*store.Ingress)

func NewStatusIngressUpdater(client *kubernetes.Clientset, k store.K8s, class string, emptyClass bool) UpdateStatus {
	return func(service store.Service, ingresses []*store.Ingress) {
		for _, ingress := range ingresses {
			if ing := New(k, ingress, class, emptyClass); ing != nil {
				logger.Error(ing.UpdateStatus(client, service.Addresses))
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
	// Required for Kubernetes < 1.14
	case "extensions/v1beta1":
		var ingSource *extensionsv1beta1.Ingress
		ingSource, err = client.ExtensionsV1beta1().Ingresses(i.resource.Namespace).Get(context.Background(), i.resource.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = extensionsv1beta1.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
		_, err = client.ExtensionsV1beta1().Ingresses(i.resource.Namespace).UpdateStatus(context.Background(), ingCopy, metav1.UpdateOptions{})
		// Required for Kubernetes < 1.19
	case "networking.k8s.io/v1beta1":
		var ingSource *networkingv1beta.Ingress
		ingSource, err = client.NetworkingV1beta1().Ingresses(i.resource.Namespace).Get(context.Background(), i.resource.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = networkingv1beta.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
		_, err = client.NetworkingV1beta1().Ingresses(i.resource.Namespace).UpdateStatus(context.Background(), ingCopy, metav1.UpdateOptions{})
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
