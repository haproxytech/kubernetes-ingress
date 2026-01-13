package ingress

import (
	"context"
	"fmt"
	"net"

	networkingv1 "k8s.io/api/networking/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (i *Ingress) UpdateStatus(client *kubernetes.Clientset) (err error) {
	var lbi []networkingv1.IngressLoadBalancerIngress

	for _, addr := range i.resource.Addresses {
		if net.ParseIP(addr) == nil {
			lbi = append(lbi, networkingv1.IngressLoadBalancerIngress{Hostname: addr})
		} else {
			lbi = append(lbi, networkingv1.IngressLoadBalancerIngress{IP: addr})
		}
	}

	//revive:disable-next-line:unnecessary-stmt
	switch i.resource.APIVersion {
	case "networking.k8s.io/v1":
		var ingSource *networkingv1.Ingress
		ingSource, err = client.NetworkingV1().Ingresses(i.resource.Namespace).Get(context.Background(), i.resource.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{Ingress: lbi}}
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
	return nil
}
