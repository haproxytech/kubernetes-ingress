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

package controller

import (
	"context"
	"errors"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta "k8s.io/api/networking/v1beta1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

//TRACE_API outputs all k8s events received from k8s API
//nolint golint
const (
	TRACE_API       = false
	CONTROLLER_NAME = "haproxy.org/ingress-controller"
)

var ErrIgnored = errors.New("Ignored resource") //nolint golint

//K8s is structure with all data required to synchronize with k8s
type K8s struct {
	API    *kubernetes.Clientset
	Logger utils.Logger
}

//GetKubernetesClient returns new client that communicates with k8s
func GetKubernetesClient() (*K8s, error) {
	logger = utils.GetK8sAPILogger()
	if !TRACE_API {
		logger.SetLevel(utils.Info)
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	logger.Trace(config)
	if err != nil {
		panic(err.Error())
	}
	return &K8s{
		API:    clientset,
		Logger: logger,
	}, nil
}

//GetRemoteKubernetesClient returns new client that communicates with k8s
func GetRemoteKubernetesClient(kubeconfig string) (*K8s, error) {
	logger = utils.GetK8sAPILogger()
	if !TRACE_API {
		logger.SetLevel(utils.Info)
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}
	return &K8s{
		API:    clientset,
		Logger: logger,
	}, nil
}

func (k *K8s) EventsNamespaces(channel chan *store.Namespace, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Namespace)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, obj)
					return
				}
				var status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &store.Namespace{
					Name:      data.GetName(),
					Endpoints: make(map[string]*store.Endpoints),
					Services:  make(map[string]*store.Service),
					Ingresses: make(map[string]*store.Ingress),
					Secret:    make(map[string]*store.Secret),
					Status:    status,
				}
				k.Logger.Tracef("%s %s: %s", NAMESPACE, item.Status, item.Name)
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Namespace)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, obj)
					return
				}
				var status = DELETED
				item := &store.Namespace{
					Name:      data.GetName(),
					Endpoints: make(map[string]*store.Endpoints),
					Services:  make(map[string]*store.Service),
					Ingresses: make(map[string]*store.Ingress),
					Secret:    make(map[string]*store.Secret),
					Status:    status,
				}
				k.Logger.Tracef("%s %s: %s", NAMESPACE, item.Status, item.Name)
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1, ok := oldObj.(*corev1.Namespace)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, oldObj)
					return
				}
				data2, ok := newObj.(*corev1.Namespace)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, newObj)
					return
				}
				var status = MODIFIED
				item1 := &store.Namespace{
					Name:   data1.GetName(),
					Status: status,
				}
				item2 := &store.Namespace{
					Name:   data2.GetName(),
					Status: status,
				}
				if item1.Name == item2.Name {
					return
				}
				k.Logger.Tracef("%s %s: %s", SERVICE, item2.Status, item2.Name)
				channel <- item2
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsEndpoints(channel chan *store.Endpoints, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, ADDED)
			if err == ErrIgnored {
				return
			}
			k.Logger.Tracef("%s %s: %s", ENDPOINTS, item.Status, item.Service)
			channel <- item
		},
		DeleteFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, DELETED)
			if err == ErrIgnored {
				return
			}
			k.Logger.Tracef("%s %s: %s", ENDPOINTS, item.Status, item.Service)
			channel <- item
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			item1, err := k.convertToEndpoints(oldObj, EMPTY)
			if err == ErrIgnored {
				return
			}
			item2, _ := k.convertToEndpoints(newObj, MODIFIED)
			if item2.Equal(item1) {
				return
			}
			//fix modified state for ones that are deleted,new,same
			k.Logger.Tracef("%s %s: %s", ENDPOINTS, item2.Status, item2.Service)
			channel <- item2
		},
	})
	go informer.Run(stop)
}

func (k *K8s) convertToEndpoints(obj interface{}, status store.Status) (*store.Endpoints, error) {
	data, ok := obj.(*corev1.Endpoints)
	if !ok {
		k.Logger.Errorf("%s: Invalid data from k8s api, %s", ENDPOINTS, obj)
		return nil, ErrIgnored
	}
	if data.GetNamespace() == "kube-system" {
		if data.ObjectMeta.Name == "kube-controller-manager" ||
			data.ObjectMeta.Name == "kube-scheduler" ||
			data.ObjectMeta.Name == "kubernetes-dashboard" ||
			data.ObjectMeta.Name == "kube-dns" {
			return nil, ErrIgnored
		}
	}
	if data.ObjectMeta.GetDeletionTimestamp() != nil {
		//detect endpoints that are in terminating state
		status = DELETED
	}
	item := &store.Endpoints{
		Namespace: data.GetNamespace(),
		Service:   store.StringW{Value: data.GetName()},
		Ports:     make(map[string]*store.PortEndpoints),
		Status:    status,
	}
	for _, subset := range data.Subsets {
		for _, port := range subset.Ports {
			addresses := make(map[string]struct{})
			for _, address := range subset.Addresses {
				addresses[address.IP] = struct{}{}
			}
			item.Ports[port.Name] = &store.PortEndpoints{
				Port:        int64(port.Port),
				AddrCount:   len(addresses),
				AddrNew:     addresses,
				HAProxySrvs: make([]*store.HAProxySrv, 0, len(addresses)),
			}
		}
	}
	return item, nil
}

func (k *K8s) EventsIngresses(channel chan *store.Ingress, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				item, err := store.ConvertToIngress(obj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, obj)
					return
				}
				k.Logger.Tracef("%s %s: %s", INGRESS, item.Status, item.Name)
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				item, err := store.ConvertToIngress(obj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, obj)
					return
				}
				item.Status = DELETED
				k.Logger.Tracef("%s %s: %s", INGRESS, item.Status, item.Name)
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				item1, err := store.ConvertToIngress(oldObj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, oldObj)
					return
				}
				item1.Status = MODIFIED

				item2, err := store.ConvertToIngress(newObj)
				if err != nil {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, oldObj)
					return
				}
				item1.Status = MODIFIED

				if item2.Equal(item1) {
					return
				}
				k.Logger.Tracef("%s %s: %s", INGRESS, item2.Status, item2.Name)
				channel <- item2
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsServices(channel chan *store.Service, stop chan struct{}, informer cache.SharedIndexInformer, publishSvc *store.Service) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			data, ok := obj.(*corev1.Service)
			if !ok {
				k.Logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, obj)
				return
			}
			var status = ADDED
			if data.ObjectMeta.GetDeletionTimestamp() != nil {
				//detect services that are in terminating state
				status = DELETED
			}
			item := &store.Service{
				Namespace:   data.GetNamespace(),
				Name:        data.GetName(),
				Annotations: store.ConvertToMapStringW(data.ObjectMeta.Annotations),
				Selector:    store.ConvertToMapStringW(data.Spec.Selector),
				Ports:       []store.ServicePort{},
				DNS:         data.Spec.ExternalName,
				Status:      status,
			}
			for _, sp := range data.Spec.Ports {
				item.Ports = append(item.Ports, store.ServicePort{
					Name:     sp.Name,
					Protocol: string(sp.Protocol),
					Port:     int64(sp.Port),
				})
			}
			if publishSvc != nil {
				if publishSvc.Namespace == item.Namespace && publishSvc.Name == item.Name {
					k.GetPublishServiceAddresses(data, publishSvc)
				}
			}
			k.Logger.Tracef("%s %s: %s", SERVICE, item.Status, item.Name)
			channel <- item
		},
		DeleteFunc: func(obj interface{}) {
			data, ok := obj.(*corev1.Service)
			if !ok {
				k.Logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, obj)
				return
			}
			var status = DELETED
			item := &store.Service{
				Namespace:   data.GetNamespace(),
				Name:        data.GetName(),
				Annotations: store.ConvertToMapStringW(data.ObjectMeta.Annotations),
				Selector:    store.ConvertToMapStringW(data.Spec.Selector),
				Status:      status,
			}
			if publishSvc != nil {
				if publishSvc.Namespace == item.Namespace && publishSvc.Name == item.Name {
					publishSvc.Status = DELETED
				}
			}
			k.Logger.Tracef("%s %s: %s", SERVICE, item.Status, item.Name)
			channel <- item
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			data1, ok := oldObj.(*corev1.Service)
			if !ok {
				k.Logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, oldObj)
				return
			}
			data2, ok := newObj.(*corev1.Service)
			if !ok {
				k.Logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, newObj)
				return
			}
			var status = MODIFIED
			item1 := &store.Service{
				Namespace:   data1.GetNamespace(),
				Name:        data1.GetName(),
				Annotations: store.ConvertToMapStringW(data1.ObjectMeta.Annotations),
				Selector:    store.ConvertToMapStringW(data1.Spec.Selector),
				Ports:       []store.ServicePort{},
				DNS:         data1.Spec.ExternalName,
				Status:      status,
			}
			for _, sp := range data1.Spec.Ports {
				item1.Ports = append(item1.Ports, store.ServicePort{
					Name:     sp.Name,
					Protocol: string(sp.Protocol),
					Port:     int64(sp.Port),
				})
			}

			item2 := &store.Service{
				Namespace:   data2.GetNamespace(),
				Name:        data2.GetName(),
				Annotations: store.ConvertToMapStringW(data2.ObjectMeta.Annotations),
				Selector:    store.ConvertToMapStringW(data2.Spec.Selector),
				Ports:       []store.ServicePort{},
				DNS:         data1.Spec.ExternalName,
				Status:      status,
			}
			for _, sp := range data2.Spec.Ports {
				item2.Ports = append(item2.Ports, store.ServicePort{
					Name:     sp.Name,
					Protocol: string(sp.Protocol),
					Port:     int64(sp.Port),
				})
			}
			if item2.Equal(item1) {
				return
			}
			if publishSvc != nil {
				if publishSvc.Namespace == item2.Namespace && publishSvc.Name == item2.Name {
					k.GetPublishServiceAddresses(data2, publishSvc)
				}
			}
			k.Logger.Tracef("%s %s: %s", SERVICE, item2.Status, item2.Name)
			channel <- item2
		},
	})
	go informer.Run(stop)
}

func (k *K8s) EventsConfigfMaps(channel chan *store.ConfigMap, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.ConfigMap)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, obj)
					return
				}
				var status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &store.ConfigMap{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: store.ConvertToMapStringW(data.Data),
					Status:      status,
				}
				k.Logger.Tracef("%s %s: %s", CONFIGMAP, item.Status, item.Name)
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.ConfigMap)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, obj)
					return
				}
				var status = DELETED
				item := &store.ConfigMap{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: store.ConvertToMapStringW(data.Data),
					Status:      status,
				}
				k.Logger.Tracef("%s %s: %s", CONFIGMAP, item.Status, item.Name)
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1, ok := oldObj.(*corev1.ConfigMap)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, oldObj)
					return
				}
				data2, ok := newObj.(*corev1.ConfigMap)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, newObj)
					return
				}
				var status = MODIFIED
				item1 := &store.ConfigMap{
					Namespace:   data1.GetNamespace(),
					Name:        data1.GetName(),
					Annotations: store.ConvertToMapStringW(data1.Data),
					Status:      status,
				}
				item2 := &store.ConfigMap{
					Namespace:   data2.GetNamespace(),
					Name:        data2.GetName(),
					Annotations: store.ConvertToMapStringW(data2.Data),
					Status:      status,
				}
				if item2.Equal(item1) {
					return
				}
				k.Logger.Tracef("%s %s: %s", CONFIGMAP, item2.Status, item2.Name)
				channel <- item2
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsSecrets(channel chan *store.Secret, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Secret)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, obj)
					return
				}
				var status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &store.Secret{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Data:      data.Data,
					Status:    status,
				}
				k.Logger.Tracef("%s %s: %s", SECRET, item.Status, item.Name)
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Secret)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, obj)
					return
				}
				var status = DELETED
				item := &store.Secret{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Data:      data.Data,
					Status:    status,
				}
				k.Logger.Tracef("%s %s: %s", SECRET, item.Status, item.Name)
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1, ok := oldObj.(*corev1.Secret)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, oldObj)
					return
				}
				data2, ok := newObj.(*corev1.Secret)
				if !ok {
					k.Logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, newObj)
					return
				}
				var status = MODIFIED
				item1 := &store.Secret{
					Namespace: data1.GetNamespace(),
					Name:      data1.GetName(),
					Data:      data1.Data,
					Status:    status,
				}
				item2 := &store.Secret{
					Namespace: data2.GetNamespace(),
					Name:      data2.GetName(),
					Data:      data2.Data,
					Status:    status,
				}
				if item2.Equal(item1) {
					return
				}
				k.Logger.Tracef("%s %s: %s", SECRET, item2.Status, item2.Name)
				channel <- item2
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) UpdateIngressStatus(ingress *store.Ingress, publishSvc *store.Service) (err error) {
	var status store.Status

	if status = publishSvc.Status; status == EMPTY {
		if ingress.Status == EMPTY {
			return nil
		}
		status = ingress.Status
	}

	var lbi []corev1.LoadBalancerIngress

	// Update addresses
	if status == ADDED || status == MODIFIED {
		for _, addr := range publishSvc.Addresses {
			if net.ParseIP(addr) == nil {
				lbi = append(lbi, corev1.LoadBalancerIngress{Hostname: addr})
			} else {
				lbi = append(lbi, corev1.LoadBalancerIngress{IP: addr})
			}
		}
	}

	switch ingress.APIVersion {
	// Required for Kubernetes < 1.14
	case "extensions/v1beta1":
		var ingSource *extensionsv1beta1.Ingress
		ingSource, err = k.API.ExtensionsV1beta1().Ingresses(ingress.Namespace).Get(context.Background(), ingress.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = extensionsv1beta1.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
		_, err = k.API.ExtensionsV1beta1().Ingresses(ingress.Namespace).UpdateStatus(context.Background(), ingCopy, metav1.UpdateOptions{})
	// Required for Kubernetes < 1.19
	case "networking.k8s.io/v1beta1":
		var ingSource *networkingv1beta.Ingress
		ingSource, err = k.API.NetworkingV1beta1().Ingresses(ingress.Namespace).Get(context.Background(), ingress.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = networkingv1beta.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
		_, err = k.API.NetworkingV1beta1().Ingresses(ingress.Namespace).UpdateStatus(context.Background(), ingCopy, metav1.UpdateOptions{})
	case "networking.k8s.io/v1":
		var ingSource *networkingv1.Ingress
		ingSource, err = k.API.NetworkingV1().Ingresses(ingress.Namespace).Get(context.Background(), ingress.Name, metav1.GetOptions{})
		if err != nil {
			break
		}
		ingCopy := ingSource.DeepCopy()
		ingCopy.Status = networkingv1.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
		_, err = k.API.NetworkingV1().Ingresses(ingress.Namespace).UpdateStatus(context.Background(), ingCopy, metav1.UpdateOptions{})
	}

	if k8serror.IsNotFound(err) {
		return fmt.Errorf("update ingress status: failed to get ingress %s/%s: %v", ingress.Namespace, ingress.Name, err)
	}
	if err != nil {
		return fmt.Errorf("failed to update LoadBalancer status of ingress %s/%s: %v", ingress.Namespace, ingress.Name, err)
	}
	k.Logger.Debugf("Successful update of LoadBalancer status in ingress %s/%s", ingress.Namespace, ingress.Name)

	return nil
}

func (k *K8s) GetPublishServiceAddresses(service *corev1.Service, publishSvc *store.Service) {
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
		k.Logger.Tracef("Unable to extract IP address/es from service %v", service)
		return
	}

	equal := false
	if len(publishSvc.Addresses) == len(addresses) {
		equal = true
		for i, address := range publishSvc.Addresses {
			if address != publishSvc.Addresses[i] {
				equal = false
				break
			}
		}
	}
	if equal {
		return
	}
	publishSvc.Addresses = addresses
	publishSvc.Status = MODIFIED
}

func (k *K8s) IsNetworkingV1Beta1ApiSupported() bool {
	vi, _ := k.API.Discovery().ServerVersion()
	major, _ := utils.ParseInt(vi.Major)
	minor, _ := utils.ParseInt(vi.Minor)

	return major == 1 && minor >= 14 && minor < 22
}

func (k *K8s) IsNetworkingV1ApiSupported() bool {
	vi, _ := k.API.Discovery().ServerVersion()
	major, _ := utils.ParseInt(vi.Major)
	minor, _ := utils.ParseInt(vi.Minor)

	return major == 1 && minor >= 19
}

func (k *K8s) IsMatchingSelectedIngressClass(name string) error {
	var controllerName string

	if k.IsNetworkingV1ApiSupported() {
		ic, err := k.API.NetworkingV1().IngressClasses().Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("the requested IngressClass %s doesn't exist", name)
		}
		controllerName = ic.Spec.Controller
	}
	if k.IsNetworkingV1Beta1ApiSupported() {
		ic, err := k.API.NetworkingV1beta1().IngressClasses().Get(context.Background(), name, metav1.GetOptions{})
		if err != nil && k8serror.IsNotFound(err) {
			k.Logger.Warningf("the requested IngressClass %s doesn't exist, for upcoming releases this will be mandatory", name)
			return nil
		}
		controllerName = ic.Spec.Controller
	}
	if len(controllerName) == 0 {
		k.Logger.Warning("Running on Kubernetes < 1.14, IngressClass resource is not implemented, ignoring")
		return nil
	}

	if controllerName != CONTROLLER_NAME {
		return fmt.Errorf("the selected IngressClass doesn't match the HAProxy Ingress Controller name, expected %v", CONTROLLER_NAME)
	}
	return nil
}
