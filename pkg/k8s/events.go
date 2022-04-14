package k8s

import (
	"errors"
	"time"

	"k8s.io/client-go/tools/cache"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func (k *K8s) EventsNamespaces(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Namespace)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, obj)
					return
				}
				status := store.ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					// detect services that are in terminating state
					status = store.DELETED
				}
				item := &store.Namespace{
					Name:           data.GetName(),
					Endpoints:      make(map[string]map[string]*store.Endpoints),
					Services:       make(map[string]*store.Service),
					Ingresses:      make(map[string]*store.Ingress),
					Secret:         make(map[string]*store.Secret),
					HAProxyRuntime: make(map[string]map[string]*store.RuntimeBackend),
					CRs: &store.CustomResources{
						Global:     make(map[string]*models.Global),
						Defaults:   make(map[string]*models.Defaults),
						LogTargets: make(map[string]models.LogTargets),
						Backends:   make(map[string]*models.Backend),
					},
					Status: status,
				}
				logger.Tracef("%s %s: %s", NAMESPACE, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: NAMESPACE, Namespace: item.Name, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Namespace)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, obj)
					return
				}
				status := store.DELETED
				item := &store.Namespace{
					Name:           data.GetName(),
					Endpoints:      make(map[string]map[string]*store.Endpoints),
					Services:       make(map[string]*store.Service),
					Ingresses:      make(map[string]*store.Ingress),
					Secret:         make(map[string]*store.Secret),
					HAProxyRuntime: make(map[string]map[string]*store.RuntimeBackend),
					CRs: &store.CustomResources{
						Global:     make(map[string]*models.Global),
						Defaults:   make(map[string]*models.Defaults),
						LogTargets: make(map[string]models.LogTargets),
						Backends:   make(map[string]*models.Backend),
					},
					Status: status,
				}
				logger.Tracef("%s %s: %s", NAMESPACE, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: NAMESPACE, Namespace: item.Name, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1, ok := oldObj.(*corev1.Namespace)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, oldObj)
					return
				}
				data2, ok := newObj.(*corev1.Namespace)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", NAMESPACE, newObj)
					return
				}
				status := store.MODIFIED
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
				logger.Tracef("%s %s: %s", SERVICE, item2.Status, item2.Name)
				channel <- SyncDataEvent{SyncType: NAMESPACE, Namespace: item2.Name, Data: item2}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsEndpointSlices(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, store.ADDED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			logger.Tracef("%s %s: %s", ENDPOINTS, item.Status, item.Service)
			channel <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		DeleteFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, store.DELETED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			logger.Tracef("%s %s: %s", ENDPOINTS, item.Status, item.Service)
			channel <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			item1, err := k.convertToEndpoints(oldObj, store.EMPTY)
			if errors.Is(err, ErrIgnored) {
				return
			}
			item2, _ := k.convertToEndpoints(newObj, store.MODIFIED)
			if item2.Equal(item1) {
				return
			}
			// fix modified state for ones that are deleted,new,same
			logger.Tracef("%s %s: %s", ENDPOINTS, item2.Status, item2.Service)
			channel <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item2.Namespace, Data: item2}
		},
	})
	go informer.Run(stop)
}

func (k *K8s) convertToEndpoints(obj interface{}, status store.Status) (*store.Endpoints, error) {
	getServiceName := func(labels map[string]string) string {
		return labels["kubernetes.io/service-name"]
	}

	shouldIgnoreObject := func(namespace string, labels map[string]string) bool {
		serviceName := getServiceName(labels)
		if namespace == "kube-system" {
			if serviceName == "kube-controller-manager" ||
				serviceName == "kube-scheduler" ||
				serviceName == "kubernetes-dashboard" ||
				serviceName == "kube-dns" {
				return true
			}
		}
		return false
	}
	switch data := obj.(type) {
	case *discoveryv1beta1.EndpointSlice:
		if shouldIgnoreObject(data.GetNamespace(), data.GetLabels()) {
			return nil, ErrIgnored
		}
		item := &store.Endpoints{
			SliceName: data.Name,
			Namespace: data.GetNamespace(),
			Service:   getServiceName(data.GetLabels()),
			Ports:     make(map[string]*store.PortEndpoints),
			Status:    status,
		}
		addresses := make(map[string]struct{})
		for _, endpoints := range data.Endpoints {
			if endpoints.Conditions.Ready == nil || !*endpoints.Conditions.Ready {
				continue
			}
			for _, address := range endpoints.Addresses {
				addresses[address] = struct{}{}
			}
		}
		for _, port := range data.Ports {
			item.Ports[*port.Name] = &store.PortEndpoints{
				Port:      int64(*port.Port),
				Addresses: addresses,
			}
		}
		return item, nil
	case *discoveryv1.EndpointSlice:
		if shouldIgnoreObject(data.GetNamespace(), data.GetLabels()) {
			return nil, ErrIgnored
		}
		item := &store.Endpoints{
			SliceName: data.Name,
			Namespace: data.GetNamespace(),
			Service:   getServiceName(data.GetLabels()),
			Ports:     make(map[string]*store.PortEndpoints),
			Status:    status,
		}
		addresses := make(map[string]struct{})
		for _, endpoints := range data.Endpoints {
			if endpoints.Conditions.Ready == nil || !*endpoints.Conditions.Ready {
				continue
			}
			for _, address := range endpoints.Addresses {
				addresses[address] = struct{}{}
			}
		}
		for _, port := range data.Ports {
			item.Ports[*port.Name] = &store.PortEndpoints{
				Port:      int64(*port.Port),
				Addresses: addresses,
			}
		}
		return item, nil
	case *corev1.Endpoints:
		item := &store.Endpoints{
			Namespace: data.GetNamespace(),
			Service:   data.GetName(),
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
					Port:      int64(port.Port),
					Addresses: addresses,
				}
			}
		}
		return item, nil
	default:
		logger.Errorf("%s: Invalid data from k8s api, %s", ENDPOINTS, obj)
		return nil, ErrIgnored
	}
}

func (k *K8s) EventsEndpoints(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, store.ADDED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			logger.Tracef("%s %s: %s", ENDPOINTS, item.Status, item.Service)
			channel <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		DeleteFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, store.DELETED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			logger.Tracef("%s %s: %s", ENDPOINTS, item.Status, item.Service)
			channel <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			item1, err := k.convertToEndpoints(oldObj, store.EMPTY)
			if errors.Is(err, ErrIgnored) {
				return
			}
			item2, _ := k.convertToEndpoints(newObj, store.MODIFIED)
			if item2.Equal(item1) {
				return
			}
			// fix modified state for ones that are deleted,new,same
			logger.Tracef("%s %s: %s", ENDPOINTS, item2.Status, item2.Service)
			channel <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item2.Namespace, Data: item2}
		},
	})
	go informer.Run(stop)
}

func (k *K8s) EventsIngressClass(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				item, err := store.ConvertToIngressClass(obj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS_CLASS, obj)
					return
				}
				logger.Tracef("%s %s: %s", INGRESS_CLASS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS_CLASS, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				item, err := store.ConvertToIngressClass(obj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS_CLASS, obj)
					return
				}
				item.Status = store.DELETED
				logger.Tracef("%s %s: %s", INGRESS_CLASS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS_CLASS, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				item, err := store.ConvertToIngressClass(newObj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, oldObj)
					return
				}
				item.Status = store.MODIFIED

				logger.Tracef("%s %s: %s", INGRESS_CLASS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS_CLASS, Data: item}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsIngresses(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				item, err := store.ConvertToIngress(obj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, obj)
					return
				}
				item.Status = store.ADDED
				logger.Tracef("%s %s: %s", INGRESS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				item, err := store.ConvertToIngress(obj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, obj)
					return
				}
				item.Status = store.DELETED
				logger.Tracef("%s %s: %s", INGRESS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				item, err := store.ConvertToIngress(newObj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, oldObj)
					return
				}
				item.Status = store.MODIFIED
				logger.Tracef("%s %s: %s", INGRESS, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsServices(channel chan SyncDataEvent, ingressChan chan ingress.Sync, stop chan struct{}, informer cache.SharedIndexInformer, publishSvc *utils.NamespaceValue) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			data, ok := obj.(*corev1.Service)
			if !ok {
				logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, obj)
				return
			}
			if data.Spec.Type == corev1.ServiceTypeExternalName && k.disableSvcExternalName {
				logger.Tracef("forwarding to ExternalName Services for %v is disabled", data)
				return
			}
			status := store.ADDED
			if data.ObjectMeta.GetDeletionTimestamp() != nil {
				// detect services that are in terminating state
				status = store.DELETED
			}
			item := &store.Service{
				Namespace:   data.GetNamespace(),
				Name:        data.GetName(),
				Annotations: store.CopyAnnotations(data.ObjectMeta.Annotations),
				Ports:       []store.ServicePort{},
				Status:      status,
			}
			if data.Spec.Type == corev1.ServiceTypeExternalName {
				item.DNS = data.Spec.ExternalName
			}
			for _, sp := range data.Spec.Ports {
				item.Ports = append(item.Ports, store.ServicePort{
					Name:     sp.Name,
					Protocol: string(sp.Protocol),
					Port:     int64(sp.Port),
				})
			}
			logger.Tracef("%s %s: %s", SERVICE, item.Status, item.Name)
			channel <- SyncDataEvent{SyncType: SERVICE, Namespace: item.Namespace, Data: item}
			if publishSvc != nil && publishSvc.Namespace == data.Namespace && publishSvc.Name == data.Name {
				ingressChan <- ingress.Sync{Service: data}
			}
		},
		DeleteFunc: func(obj interface{}) {
			data, ok := obj.(*corev1.Service)
			if !ok {
				logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, obj)
				return
			}
			if data.Spec.Type == corev1.ServiceTypeExternalName && k.disableSvcExternalName {
				return
			}
			status := store.DELETED
			item := &store.Service{
				Namespace:   data.GetNamespace(),
				Name:        data.GetName(),
				Annotations: store.CopyAnnotations(data.ObjectMeta.Annotations),
				Status:      status,
			}
			if data.Spec.Type == corev1.ServiceTypeExternalName {
				item.DNS = data.Spec.ExternalName
			}
			logger.Tracef("%s %s: %s", SERVICE, item.Status, item.Name)
			channel <- SyncDataEvent{SyncType: SERVICE, Namespace: item.Namespace, Data: item}
			if publishSvc != nil && publishSvc.Namespace == data.Namespace && publishSvc.Name == data.Name {
				ingressChan <- ingress.Sync{Service: data}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			data1, ok := oldObj.(*corev1.Service)
			if !ok {
				logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, oldObj)
				return
			}
			if data1.Spec.Type == corev1.ServiceTypeExternalName && k.disableSvcExternalName {
				logger.Tracef("forwarding to ExternalName Services for %v is disabled", data1)
				return
			}
			data2, ok := newObj.(*corev1.Service)
			if !ok {
				logger.Errorf("%s: Invalid data from k8s api, %s", SERVICE, newObj)
				return
			}
			if data2.Spec.Type == corev1.ServiceTypeExternalName && k.disableSvcExternalName {
				logger.Tracef("forwarding to ExternalName Services for %v is disabled", data2)
				return
			}
			if publishSvc != nil && publishSvc.Namespace == data2.Namespace && publishSvc.Name == data2.Name {
				ingressChan <- ingress.Sync{Service: data2}
			}
			status := store.MODIFIED
			item1 := &store.Service{
				Namespace:   data1.GetNamespace(),
				Name:        data1.GetName(),
				Annotations: store.CopyAnnotations(data1.ObjectMeta.Annotations),
				Ports:       []store.ServicePort{},
				Status:      status,
			}
			if data1.Spec.Type == corev1.ServiceTypeExternalName {
				item1.DNS = data1.Spec.ExternalName
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
				Annotations: store.CopyAnnotations(data2.ObjectMeta.Annotations),
				Ports:       []store.ServicePort{},
				Status:      status,
			}
			if data2.Spec.Type == corev1.ServiceTypeExternalName {
				item2.DNS = data2.Spec.ExternalName
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
			logger.Tracef("%s %s: %s", SERVICE, item2.Status, item2.Name)
			channel <- SyncDataEvent{SyncType: SERVICE, Namespace: item2.Namespace, Data: item2}
		},
	})
	go informer.Run(stop)
}

func (k *K8s) EventsConfigfMaps(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.ConfigMap)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, obj)
					return
				}
				status := store.ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					// detect services that are in terminating state
					status = store.DELETED
				}
				item := &store.ConfigMap{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: store.CopyAnnotations(data.Data),
					Status:      status,
				}
				logger.Tracef("%s %s: %s", CONFIGMAP, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item.Namespace, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.ConfigMap)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, obj)
					return
				}
				status := store.DELETED
				item := &store.ConfigMap{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: store.CopyAnnotations(data.Data),
					Status:      status,
				}
				logger.Tracef("%s %s: %s", CONFIGMAP, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item.Namespace, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1, ok := oldObj.(*corev1.ConfigMap)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, oldObj)
					return
				}
				data2, ok := newObj.(*corev1.ConfigMap)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", CONFIGMAP, newObj)
					return
				}
				status := store.MODIFIED
				item1 := &store.ConfigMap{
					Namespace:   data1.GetNamespace(),
					Name:        data1.GetName(),
					Annotations: store.CopyAnnotations(data1.Data),
					Status:      status,
				}
				item2 := &store.ConfigMap{
					Namespace:   data2.GetNamespace(),
					Name:        data2.GetName(),
					Annotations: store.CopyAnnotations(data2.Data),
					Status:      status,
				}
				if item2.Equal(item1) {
					return
				}
				logger.Tracef("%s %s: %s", CONFIGMAP, item2.Status, item2.Name)
				channel <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item2.Namespace, Data: item2}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventsSecrets(channel chan SyncDataEvent, stop chan struct{}, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Secret)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, obj)
					return
				}
				status := store.ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					// detect services that are in terminating state
					status = store.DELETED
				}
				item := &store.Secret{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Data:      data.Data,
					Status:    status,
				}
				logger.Tracef("%s %s: %s", SECRET, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: SECRET, Namespace: item.Namespace, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				data, ok := obj.(*corev1.Secret)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, obj)
					return
				}
				status := store.DELETED
				item := &store.Secret{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Data:      data.Data,
					Status:    status,
				}
				logger.Tracef("%s %s: %s", SECRET, item.Status, item.Name)
				channel <- SyncDataEvent{SyncType: SECRET, Namespace: item.Namespace, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1, ok := oldObj.(*corev1.Secret)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, oldObj)
					return
				}
				data2, ok := newObj.(*corev1.Secret)
				if !ok {
					logger.Errorf("%s: Invalid data from k8s api, %s", SECRET, newObj)
					return
				}
				status := store.MODIFIED
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
				logger.Tracef("%s %s: %s", SECRET, item2.Status, item2.Name)
				channel <- SyncDataEvent{SyncType: SECRET, Namespace: item2.Namespace, Data: item2}
			},
		},
	)
	go informer.Run(stop)
}

func (k *K8s) EventPods(namespace, podPrefix string, resyncPeriod time.Duration, eventChan chan SyncDataEvent) {
	var prefix string
	if podPrefix == "" {
		return
	}
	watchlist := cache.NewListWatchFromClient(k.builtInClient.CoreV1().RESTClient(), "pods", namespace, fields.Nothing())
	_, eController := cache.NewInformer(
		watchlist,
		&corev1.Pod{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				meta := obj.(*corev1.Pod).ObjectMeta
				prefix, _ = utils.GetPodPrefix(meta.Name)
				if prefix != podPrefix {
					return
				}
				eventChan <- SyncDataEvent{SyncType: POD, Namespace: meta.Namespace, Data: store.PodEvent{Created: true}}
			},
			DeleteFunc: func(obj interface{}) {
				meta := obj.(*corev1.Pod).ObjectMeta
				prefix, _ = utils.GetPodPrefix(meta.Name)
				if prefix != podPrefix {
					return
				}
				eventChan <- SyncDataEvent{SyncType: POD, Namespace: meta.Namespace, Data: store.PodEvent{}}
			},
		},
	)
	go eController.Run(wait.NeverStop)
}
