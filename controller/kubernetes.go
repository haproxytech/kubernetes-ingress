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
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//networking "k8s.io/api/networking/v1beta1"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const DEBUG_API = false //nolint golint

var ErrIgnored = errors.New("Ignored resource") //nolint golint

//K8s is structure with all data required to synchronize with k8s
type K8s struct {
	API *kubernetes.Clientset
}

//GetKubernetesClient returns new client that communicates with k8s
func GetKubernetesClient() (*K8s, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	//log.Println(config)
	if err != nil {
		panic(err.Error())
	}
	return &K8s{API: clientset}, nil
}

//GetRemoteKubernetesClient returns new client that communicates with k8s
func GetRemoteKubernetesClient(kubeconfig string) (*K8s, error) {

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
	return &K8s{API: clientset}, nil
}

func (k *K8s) EventsNamespaces(channel chan *Namespace, stop chan struct{}) {
	watchlist := cache.NewListWatchFromClient(
		k.API.CoreV1().RESTClient(),
		string("namespaces"),
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&corev1.Namespace{},
		1*time.Second, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*corev1.Namespace)
				var status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &Namespace{
					Name:      data.GetName(),
					Endpoints: make(map[string]*Endpoints),
					Services:  make(map[string]*Service),
					Ingresses: make(map[string]*Ingress),
					Secret:    make(map[string]*Secret),
					Status:    status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", NAMESPACE, item.Status, item.Name)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*corev1.Namespace)
				var status = DELETED
				item := &Namespace{
					Name:      data.GetName(),
					Endpoints: make(map[string]*Endpoints),
					Services:  make(map[string]*Service),
					Ingresses: make(map[string]*Ingress),
					Secret:    make(map[string]*Secret),
					Status:    status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", NAMESPACE, item.Status, item.Name)
				}
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1 := oldObj.(*corev1.Namespace)
				data2 := newObj.(*corev1.Namespace)
				var status = MODIFIED
				item1 := &Namespace{
					Name:   data1.GetName(),
					Status: status,
				}
				item2 := &Namespace{
					Name:   data2.GetName(),
					Status: status,
				}
				if item1.Name == item2.Name {
					return
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", SERVICE, item2.Status, item2.Name)
				}
				channel <- item2
			},
		},
	)
	go controller.Run(stop)
}

func (k *K8s) EventsEndpoints(channel chan *Endpoints, stop chan struct{}) {
	watchlist := cache.NewListWatchFromClient(
		k.API.CoreV1().RESTClient(),
		string("endpoints"),
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&corev1.Endpoints{},
		1*time.Second, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				item, err := k.convertToEndpoints(obj, ADDED)
				if err == ErrIgnored {
					return
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", ENDPOINTS, item.Status, item.Service)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				item, err := k.convertToEndpoints(obj, DELETED)
				if err == ErrIgnored {
					return
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", ENDPOINTS, item.Status, item.Service)
				}
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
				if DEBUG_API {
					log.Printf("%s %s: %s \n", ENDPOINTS, item2.Status, item2.Service)
				}
				channel <- item2
			},
		},
	)
	go controller.Run(stop)
}

func (k *K8s) convertToEndpoints(obj interface{}, status Status) (*Endpoints, error) {
	data := obj.(*corev1.Endpoints)
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
	item := &Endpoints{
		Namespace: data.GetNamespace(),
		Service:   StringW{Value: data.GetName()},
		Ports:     &EndpointPorts{},
		Addresses: &EndpointIPs{},
		Status:    status,
	}
	for _, subset := range data.Subsets {
		for _, address := range subset.Addresses {
			eip := &EndpointIP{
				IP:          address.IP,
				HAProxyName: "",
				Disabled:    false,
				Status:      status,
			}
			var key string
			if address.TargetRef != nil {
				eip.Name = address.TargetRef.Name
				key = string(address.TargetRef.UID)
			} else {
				key = fmt.Sprintf("%s%s%v", address.IP, address.Hostname, address.NodeName)
			}
			(*item.Addresses)[key] = eip
		}
		for _, port := range subset.Ports {
			*item.Ports = append(*item.Ports, &EndpointPort{
				Name:     port.Name,
				Protocol: string(port.Protocol),
				Port:     int64(port.Port),
				Status:   status,
			})
		}
	}
	return item, nil
}

func (k *K8s) EventsIngresses(channel chan *Ingress, stop chan struct{}) {
	watchlist := cache.NewListWatchFromClient(
		k.API.ExtensionsV1beta1().RESTClient(),
		string("ingresses"),
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&extensions.Ingress{},
		1*time.Second, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*extensions.Ingress)
				var status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &Ingress{
					Namespace:      data.GetNamespace(),
					Name:           data.GetName(),
					Annotations:    ConvertToMapStringW(data.ObjectMeta.Annotations),
					Rules:          ConvertIngressRules(data.Spec.Rules),
					DefaultBackend: ConvertIngressBackend(data.Spec.Backend),
					TLS:            ConvertIngressTLS(data.Spec.TLS),
					Status:         status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", INGRESS, item.Status, item.Name)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*extensions.Ingress)
				var status = DELETED
				item := &Ingress{
					Namespace:      data.GetNamespace(),
					Name:           data.GetName(),
					Annotations:    ConvertToMapStringW(data.ObjectMeta.Annotations),
					Rules:          ConvertIngressRules(data.Spec.Rules),
					DefaultBackend: ConvertIngressBackend(data.Spec.Backend),
					TLS:            ConvertIngressTLS(data.Spec.TLS),
					Status:         status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", INGRESS, item.Status, item.Name)
				}
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1 := oldObj.(*extensions.Ingress)
				data2 := newObj.(*extensions.Ingress)
				var status = MODIFIED
				item1 := &Ingress{
					Namespace:      data1.GetNamespace(),
					Name:           data1.GetName(),
					Annotations:    ConvertToMapStringW(data1.ObjectMeta.Annotations),
					Rules:          ConvertIngressRules(data1.Spec.Rules),
					DefaultBackend: ConvertIngressBackend(data1.Spec.Backend),
					TLS:            ConvertIngressTLS(data1.Spec.TLS),
					Status:         status,
				}
				item2 := &Ingress{
					Namespace:      data2.GetNamespace(),
					Name:           data2.GetName(),
					Annotations:    ConvertToMapStringW(data2.ObjectMeta.Annotations),
					Rules:          ConvertIngressRules(data2.Spec.Rules),
					DefaultBackend: ConvertIngressBackend(data2.Spec.Backend),
					TLS:            ConvertIngressTLS(data2.Spec.TLS),
					Status:         status,
				}
				if item2.Equal(item1) {
					return
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", INGRESS, item2.Status, item2.Name)
				}
				channel <- item2
			},
		},
	)
	go controller.Run(stop)
}

func (k *K8s) EventsServices(channel chan *Service, stop chan struct{}, publishSvc *Service) {
	watchlist := cache.NewListWatchFromClient(
		k.API.CoreV1().RESTClient(),
		string(corev1.ResourceServices),
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&corev1.Service{},
		1*time.Second, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*corev1.Service)
				var status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &Service{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: ConvertToMapStringW(data.ObjectMeta.Annotations),
					Selector:    ConvertToMapStringW(data.Spec.Selector),
					Ports:       []ServicePort{},
					Status:      status,
				}
				for _, sp := range data.Spec.Ports {
					item.Ports = append(item.Ports, ServicePort{
						Name:          sp.Name,
						Protocol:      string(sp.Protocol),
						ServicePort:   int64(sp.Port),
						TargetPortInt: int64(sp.TargetPort.IntVal),
						TargetPortStr: sp.TargetPort.StrVal,
					})
				}
				if publishSvc != nil {
					if publishSvc.Namespace == item.Namespace && publishSvc.Name == item.Name {
						k.GetPublishServiceAddresses(data, publishSvc)
					}
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", SERVICE, item.Status, item.Name)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*corev1.Service)
				var status = DELETED
				item := &Service{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: ConvertToMapStringW(data.ObjectMeta.Annotations),
					Selector:    ConvertToMapStringW(data.Spec.Selector),
					Status:      status,
				}
				if publishSvc != nil {
					if publishSvc.Namespace == item.Namespace && publishSvc.Name == item.Name {
						publishSvc.Status = DELETED
					}
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", SERVICE, item.Status, item.Name)
				}
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1 := oldObj.(*corev1.Service)
				data2 := newObj.(*corev1.Service)
				var status = MODIFIED
				item1 := &Service{
					Namespace:   data1.GetNamespace(),
					Name:        data1.GetName(),
					Annotations: ConvertToMapStringW(data1.ObjectMeta.Annotations),
					Selector:    ConvertToMapStringW(data1.Spec.Selector),
					Ports:       []ServicePort{},
					Status:      status,
				}
				for _, sp := range data1.Spec.Ports {
					item1.Ports = append(item1.Ports, ServicePort{
						Name:          sp.Name,
						Protocol:      string(sp.Protocol),
						ServicePort:   int64(sp.Port),
						TargetPortInt: int64(sp.TargetPort.IntVal),
						TargetPortStr: sp.TargetPort.StrVal,
					})
				}

				item2 := &Service{
					Namespace:   data2.GetNamespace(),
					Name:        data2.GetName(),
					Annotations: ConvertToMapStringW(data2.ObjectMeta.Annotations),
					Selector:    ConvertToMapStringW(data2.Spec.Selector),
					Ports:       []ServicePort{},
					Status:      status,
				}
				for _, sp := range data2.Spec.Ports {
					item2.Ports = append(item2.Ports, ServicePort{
						Name:          sp.Name,
						Protocol:      string(sp.Protocol),
						ServicePort:   int64(sp.Port),
						TargetPortInt: int64(sp.TargetPort.IntVal),
						TargetPortStr: sp.TargetPort.StrVal,
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
				if DEBUG_API {
					log.Printf("%s %s: %s \n", SERVICE, item2.Status, item2.Name)
				}
				channel <- item2
			},
		},
	)
	go controller.Run(stop)
}

func (k *K8s) EventsConfigfMaps(channel chan *ConfigMap, stop chan struct{}) {
	watchlist := cache.NewListWatchFromClient(
		k.API.CoreV1().RESTClient(),
		string(corev1.ResourceConfigMaps),
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&corev1.ConfigMap{},
		1*time.Second, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*corev1.ConfigMap)
				var status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &ConfigMap{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: ConvertToMapStringW(data.Data),
					Status:      status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", CONFIGMAP, item.Status, item.Name)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*corev1.ConfigMap)
				var status = DELETED
				item := &ConfigMap{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: ConvertToMapStringW(data.Data),
					Status:      status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", CONFIGMAP, item.Status, item.Name)
				}
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1 := oldObj.(*corev1.ConfigMap)
				data2 := newObj.(*corev1.ConfigMap)
				var status = MODIFIED
				item1 := &ConfigMap{
					Namespace:   data1.GetNamespace(),
					Name:        data1.GetName(),
					Annotations: ConvertToMapStringW(data1.Data),
					Status:      status,
				}
				item2 := &ConfigMap{
					Namespace:   data2.GetNamespace(),
					Name:        data2.GetName(),
					Annotations: ConvertToMapStringW(data2.Data),
					Status:      status,
				}
				if item2.Equal(item1) {
					return
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", CONFIGMAP, item2.Status, item2.Name)
				}
				channel <- item2
			},
		},
	)
	go controller.Run(stop)
}

func (k *K8s) EventsSecrets(channel chan *Secret, stop chan struct{}) {
	watchlist := cache.NewListWatchFromClient(
		k.API.CoreV1().RESTClient(),
		string(corev1.ResourceSecrets),
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&corev1.Secret{},
		1*time.Second, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*corev1.Secret)
				var status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &Secret{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Data:      data.Data,
					Status:    status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", SECRET, item.Status, item.Name)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*corev1.Secret)
				var status = DELETED
				item := &Secret{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Data:      data.Data,
					Status:    status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", SECRET, item.Status, item.Name)
				}
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1 := oldObj.(*corev1.Secret)
				data2 := newObj.(*corev1.Secret)
				var status = MODIFIED
				item1 := &Secret{
					Namespace: data1.GetNamespace(),
					Name:      data1.GetName(),
					Data:      data1.Data,
					Status:    status,
				}
				item2 := &Secret{
					Namespace: data2.GetNamespace(),
					Name:      data2.GetName(),
					Data:      data2.Data,
					Status:    status,
				}
				if item2.Equal(item1) {
					return
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", SECRET, item2.Status, item2.Name)
				}
				channel <- item2
			},
		},
	)
	go controller.Run(stop)
}

func (k *K8s) UpdateIngressStatus(ingress *Ingress, publishSvc *Service) (err error) {
	var ingSource *extensions.Ingress
	var ingCopy extensions.Ingress
	status := publishSvc.Status
	lbi := []corev1.LoadBalancerIngress{}
	if status == EMPTY {
		if ingress.Status == EMPTY {
			return nil
		}
		status = ingress.Status
	}

	// Get Ingress
	if ingSource, err = k.API.ExtensionsV1beta1().Ingresses(ingress.Namespace).Get(ingress.Name, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("update ingress status: failed to get ingress %s/%s: %v", ingress.Namespace, ingress.Name, err)
	}
	ingCopy = *ingSource
	ingCopy.Status = extensions.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
	// Update addresses
	if status == ADDED || status == MODIFIED {
		for _, addr := range publishSvc.Addresses {
			if net.ParseIP(addr) == nil {
				lbi = append(lbi, corev1.LoadBalancerIngress{Hostname: addr})
			} else {
				lbi = append(lbi, corev1.LoadBalancerIngress{IP: addr})
			}
		}
		ingCopy.Status = extensions.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: lbi}}
	}
	// Remove addresses
	if _, err = k.API.ExtensionsV1beta1().Ingresses(ingress.Namespace).UpdateStatus(&ingCopy); err != nil {
		return fmt.Errorf("failed to update LoadBalancer status of ingress%s/%s: %v", ingress.Namespace, ingress.Name, err)
	}
	log.Printf("successful update of LoadBalancer status of ingress %s/%s", ingress.Namespace, ingress.Name)
	return nil

}

func (k *K8s) GetPublishServiceAddresses(service *corev1.Service, publishSvc *Service) {
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
		log.Printf("Unable to extract IP address/es from service %v", service)
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
