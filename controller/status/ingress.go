package status

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
)

func UpdateIngress(client *kubernetes.Clientset, k store.K8s, channel chan SyncIngress) {
	addresses := []string{}
	for status := range channel {
		// Published Service updated: Update all Ingresses
		if status.Service != nil && getServiceAddresses(status.Service, &addresses) {
			logger.Debug("Addresses of Ingress Controller service changed, status of all ingress resources are going to be updated")
			for _, ns := range k.Namespaces {
				for _, ingress := range k.Namespaces[ns.Name].Ingresses {
					logger.Error(updateIngressStatus(client, ingress, addresses))
				}
			}
		}
		if status.Ingress != nil {
			logger.Error(updateIngressStatus(client, status.Ingress, addresses))
		}
	}
}

func getServiceAddresses(service *corev1.Service, curAddr *[]string) (updated bool) {
	addresses := []string{}
	switch service.Spec.Type {
	case corev1.ServiceTypeExternalName:
		addresses = []string{service.Spec.ExternalName}
	case corev1.ServiceTypeClusterIP:
		addresses = []string{service.Spec.ClusterIP}
	case corev1.ServiceTypeNodePort:
		if service.Spec.ExternalIPs != nil {
			addresses = append(addresses, service.Spec.ExternalIPs...)
		} else {
			addresses = append(addresses, service.Spec.ClusterIP)
		}
	case corev1.ServiceTypeLoadBalancer:
		for _, ip := range service.Status.LoadBalancer.Ingress {
			if ip.IP == "" {
				addresses = append(addresses, ip.Hostname)
			} else {
				addresses = append(addresses, ip.IP)
			}
		}
		addresses = append(addresses, service.Spec.ExternalIPs...)
	default:
		logger.Errorf("Unable to extract IP address/es from service %s/%s", service.Namespace, service.Name)
		return
	}

	if len(*curAddr) != len(addresses) {
		updated = true
		*curAddr = addresses
		return
	}
	for i, address := range addresses {
		if address != (*curAddr)[i] {
			updated = true
			break
		}
	}
	if updated {
		*curAddr = addresses
	}
	return
}

func updateIngressStatus(client *kubernetes.Clientset, ingress *store.Ingress, addresses []string) (err error) {
	logger.Trace("Updating status of Ingress %s/%s", ingress.Namespace, ingress.Name)
	var lbi []corev1.LoadBalancerIngress
	for _, addr := range addresses {
		if net.ParseIP(addr) == nil {
			lbi = append(lbi, corev1.LoadBalancerIngress{Hostname: addr})
		} else {
			lbi = append(lbi, corev1.LoadBalancerIngress{IP: addr})
		}
	}

	switch ingress.APIVersion {
	// Required for Kubernetes < 1.14
	case "extensions/v1beta1":
		var ingSource *extensionsv1beta1.Ingress
		ingSource, err = client.ExtensionsV1beta1().Ingresses(ingress.Namespace).Get(context.Background(), ingress.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = extensionsv1beta1.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
		_, err = client.ExtensionsV1beta1().Ingresses(ingress.Namespace).UpdateStatus(context.Background(), ingCopy, metav1.UpdateOptions{})
		// Required for Kubernetes < 1.19
	case "networking.k8s.io/v1beta1":
		var ingSource *networkingv1beta.Ingress
		ingSource, err = client.NetworkingV1beta1().Ingresses(ingress.Namespace).Get(context.Background(), ingress.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = networkingv1beta.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
		_, err = client.NetworkingV1beta1().Ingresses(ingress.Namespace).UpdateStatus(context.Background(), ingCopy, metav1.UpdateOptions{})
	case "networking.k8s.io/v1":
		var ingSource *networkingv1.Ingress
		ingSource, err = client.NetworkingV1().Ingresses(ingress.Namespace).Get(context.Background(), ingress.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = networkingv1.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
		_, err = client.NetworkingV1().Ingresses(ingress.Namespace).UpdateStatus(context.Background(), ingCopy, metav1.UpdateOptions{})
	}

	if k8serror.IsNotFound(err) {
		return fmt.Errorf("update ingress status: failed to get ingress %s/%s: %w", ingress.Namespace, ingress.Name, err)
	}
	if err != nil {
		return fmt.Errorf("failed to update LoadBalancer status of ingress %s/%s: %w", ingress.Namespace, ingress.Name, err)
	}
	logger.Tracef("Successful update of LoadBalancer status in ingress %s/%s", ingress.Namespace, ingress.Name)

	return nil
}
