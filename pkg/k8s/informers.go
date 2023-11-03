package k8s

import (
	"errors"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"k8s.io/apimachinery/pkg/fields"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"

	"github.com/haproxytech/client-native/v3/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	gatewaynetworking "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
)

func (k k8s) getNamespaceInfomer(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Core().V1().Namespaces().Informer()
	_, err := informer.AddEventHandler(
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
					Gateways:        make(map[string]*store.Gateway),
					TCPRoutes:       make(map[string]*store.TCPRoute),
					ReferenceGrants: make(map[string]*store.ReferenceGrant),
					Labels:          utils.CopyMap(data.Labels),
					Status:          status,
				}
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", NAMESPACE, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: NAMESPACE, Namespace: item.Name, Data: item}
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
					Gateways:        make(map[string]*store.Gateway),
					TCPRoutes:       make(map[string]*store.TCPRoute),
					ReferenceGrants: make(map[string]*store.ReferenceGrant),
					Labels:          utils.CopyMap(data.Labels),
					Status:          status,
				}
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", NAMESPACE, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: NAMESPACE, Namespace: item.Name, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				_, ok := oldObj.(*corev1.Namespace)
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

				item2 := &store.Namespace{
					Name:   data2.GetName(),
					Status: status,
					Labels: utils.CopyMap(data2.Labels),
				}
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", NAMESPACE, item2.Status, item2.Name)
				eventChan <- SyncDataEvent{SyncType: NAMESPACE, Namespace: item2.Name, Data: item2}
			},
		},
	)
	logger.Error(err)
	return informer
}

func (k k8s) getServiceInformer(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Core().V1().Services().Informer()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
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
			logger.Tracef("[RUNTIME] [K8s] %s %s: %s", SERVICE, item.Status, item.Name)
			eventChan <- SyncDataEvent{SyncType: SERVICE, Namespace: item.Namespace, Data: item}
			if k.publishSvc != nil && k.publishSvc.Namespace == item.Namespace && k.publishSvc.Name == item.Name {
				// item copy because of ADDED handler in events.go which must modify the STATUS based solely on addresses
				itemCopy := *item
				itemCopy.Addresses = getServiceAddresses(data)
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", PUBLISH_SERVICE, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: PUBLISH_SERVICE, Namespace: item.Namespace, Data: &itemCopy}
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
			logger.Tracef("[RUNTIME] [K8s] %s %s: %s", SERVICE, item.Status, item.Name)
			eventChan <- SyncDataEvent{SyncType: SERVICE, Namespace: item.Namespace, Data: item}
			if k.publishSvc != nil && k.publishSvc.Namespace == item.Namespace && k.publishSvc.Name == item.Name {
				item.Addresses = getServiceAddresses(data)
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", PUBLISH_SERVICE, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: PUBLISH_SERVICE, Namespace: data.Namespace, Data: item}
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

			status := store.MODIFIED

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

			logger.Tracef("[RUNTIME] [K8s] %s %s: %s", SERVICE, item2.Status, item2.Name)
			eventChan <- SyncDataEvent{SyncType: SERVICE, Namespace: item2.Namespace, Data: item2}

			if k.publishSvc != nil && k.publishSvc.Namespace == item2.Namespace && k.publishSvc.Name == item2.Name {
				item2.Addresses = getServiceAddresses(data2)
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", PUBLISH_SERVICE, item2.Status, item2.Name)
				eventChan <- SyncDataEvent{SyncType: PUBLISH_SERVICE, Namespace: item2.Namespace, Data: item2}
			}
		},
	})
	logger.Error(err)
	return informer
}

func (k k8s) getSecretInformer(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Core().V1().Secrets().Informer()
	_, err := informer.AddEventHandler(
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
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", SECRET, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: SECRET, Namespace: item.Namespace, Data: item}
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
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", SECRET, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: SECRET, Namespace: item.Namespace, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				_, ok := oldObj.(*corev1.Secret)
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

				item2 := &store.Secret{
					Namespace: data2.GetNamespace(),
					Name:      data2.GetName(),
					Data:      data2.Data,
					Status:    status,
				}

				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", SECRET, item2.Status, item2.Name)
				eventChan <- SyncDataEvent{SyncType: SECRET, Namespace: item2.Namespace, Data: item2}
			},
		},
	)
	logger.Error(err)
	return informer
}

func (k k8s) getConfigMapInformer(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Core().V1().ConfigMaps().Informer()
	_, err := informer.AddEventHandler(
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
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", CONFIGMAP, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item.Namespace, Data: item}
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
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", CONFIGMAP, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item.Namespace, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				_, ok := oldObj.(*corev1.ConfigMap)
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
				item2 := &store.ConfigMap{
					Namespace:   data2.GetNamespace(),
					Name:        data2.GetName(),
					Annotations: store.CopyAnnotations(data2.Data),
					Status:      status,
				}

				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", CONFIGMAP, item2.Status, item2.Name)
				eventChan <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item2.Namespace, Data: item2}
			},
		},
	)
	logger.Error(err)
	return informer
}

func (k k8s) getIngressInformers(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) (ii, ici cache.SharedIndexInformer) { //nolint:ireturn
	apiGroup := "networking.k8s.io/v1"

	resources, err := k.builtInClient.ServerResourcesForGroupVersion(apiGroup)
	if err != nil {
		return
	}
	for _, rs := range resources.APIResources {
		if rs.Name == "ingresses" {
			ii = factory.Networking().V1().Ingresses().Informer()
			logger.Debugf("watching ingress resources of apiGroup %s:", apiGroup)
		}
		if rs.Name == "ingressclasses" {
			ici = factory.Networking().V1().IngressClasses().Informer()
		}
	}
	if ii != nil {
		k.addIngressHandlers(eventChan, ii)
		if ici != nil {
			k.addIngressClassHandlers(eventChan, ici)
		}
		return
	}
	return
}

func (k k8s) getEndpointSliceInformer(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	for i, apiGroup := range []string{"discovery.k8s.io/v1", "discovery.k8s.io/v1beta1"} {
		resources, err := k.builtInClient.ServerResourcesForGroupVersion(apiGroup)
		if err != nil {
			continue
		}

		for _, rs := range resources.APIResources {
			if rs.Name == "endpointslices" {
				var informer cache.SharedIndexInformer
				switch i {
				case 0:
					logger.Debug("Using discovery.k8s.io/v1 endpointslices")
					informer = factory.Discovery().V1().EndpointSlices().Informer()
				case 1:
					logger.Debug("Using discovery.k8s.io/v1beta1 endpointslices")
					informer = factory.Discovery().V1beta1().EndpointSlices().Informer()
				}
				k.addEndpointSliceHandlers(eventChan, informer)
				return informer
			}
		}
	}
	return nil
}

func (k k8s) getEndpointsInformer(eventChan chan SyncDataEvent, factory informers.SharedInformerFactory) cache.SharedIndexInformer { //nolint:ireturn
	informer := factory.Core().V1().Endpoints().Informer()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, store.ADDED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			logger.Tracef("[RUNTIME] [K8s] %s %s: %s %s", ENDPOINTS, item.Status, item.Service, item.SliceName)
			eventChan <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		DeleteFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, store.DELETED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			logger.Tracef("[RUNTIME] [K8s] %s %s: %s %s", ENDPOINTS, item.Status, item.Service, item.SliceName)
			eventChan <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			_, err := k.convertToEndpoints(oldObj, store.EMPTY)
			if errors.Is(err, ErrIgnored) {
				return
			}
			item2, _ := k.convertToEndpoints(newObj, store.MODIFIED)
			// fix modified state for ones that are deleted,new,same
			logger.Tracef("[RUNTIME] [K8s] %s %s: %s %s", ENDPOINTS, item2.Status, item2.Service, item2.SliceName)
			eventChan <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item2.Namespace, Data: item2}
		},
	})
	logger.Error(err)
	return informer
}

func (k *k8s) getPodInformer(namespace, podPrefix string, resyncPeriod time.Duration, eventChan chan SyncDataEvent) cache.Controller { //nolint:ireturn
	var prefix string
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
				eventChan <- SyncDataEvent{SyncType: POD, Namespace: meta.Namespace, Data: store.PodEvent{Status: store.ADDED, Name: meta.Name}}
			},
			DeleteFunc: func(obj interface{}) {
				meta := obj.(*corev1.Pod).ObjectMeta //nolint:forcetypeassert
				prefix, _ = utils.GetPodPrefix(meta.Name)
				if prefix != podPrefix {
					return
				}
				eventChan <- SyncDataEvent{SyncType: POD, Namespace: meta.Namespace, Data: store.PodEvent{Status: store.DELETED, Name: meta.Name}}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				meta := newObj.(*corev1.Pod).ObjectMeta //nolint:forcetypeassert
				prefix, _ = utils.GetPodPrefix(meta.Name)
				if prefix != podPrefix {
					return
				}
				eventChan <- SyncDataEvent{SyncType: POD, Namespace: meta.Namespace, Data: store.PodEvent{Status: store.MODIFIED, Name: meta.Name}}
			},
		},
	)
	return eController
}

func (k k8s) addIngressClassHandlers(eventChan chan SyncDataEvent, informer cache.SharedIndexInformer) {
	_, err := informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				item, err := store.ConvertToIngressClass(obj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS_CLASS, obj)
					return
				}
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", INGRESS_CLASS, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: INGRESS_CLASS, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				item, err := store.ConvertToIngressClass(obj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS_CLASS, obj)
					return
				}
				item.Status = store.DELETED
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", INGRESS_CLASS, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: INGRESS_CLASS, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				item, err := store.ConvertToIngressClass(newObj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS_CLASS, oldObj)
					return
				}
				item.Status = store.MODIFIED

				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", INGRESS_CLASS, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: INGRESS_CLASS, Data: item}
			},
		},
	)
	logger.Error(err)
}

func (k k8s) addIngressHandlers(eventChan chan SyncDataEvent, informer cache.SharedIndexInformer) {
	_, err := informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				item, err := store.ConvertToIngress(obj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, obj)
					return
				}
				item.Status = store.ADDED
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", INGRESS, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			},
			DeleteFunc: func(obj interface{}) {
				item, err := store.ConvertToIngress(obj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, obj)
					return
				}
				item.Status = store.DELETED
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", INGRESS, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				item, err := store.ConvertToIngress(newObj)
				if err != nil {
					logger.Errorf("%s: Invalid data from k8s api, %s", INGRESS, oldObj)
					return
				}
				item.Status = store.MODIFIED
				logger.Tracef("[RUNTIME] [K8s] %s %s: %s", INGRESS, item.Status, item.Name)
				eventChan <- SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			},
		},
	)
	logger.Error(err)
}

func (k k8s) addEndpointSliceHandlers(eventChan chan SyncDataEvent, informer cache.SharedIndexInformer) {
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, store.ADDED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			logger.Tracef("[RUNTIME] [K8s] %s %s: %s %s", ENDPOINTS, item.Status, item.Service, item.SliceName)
			eventChan <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		DeleteFunc: func(obj interface{}) {
			item, err := k.convertToEndpoints(obj, store.DELETED)
			if errors.Is(err, ErrIgnored) {
				return
			}
			logger.Tracef("[RUNTIME] [K8s] %s %s: %s %s", ENDPOINTS, item.Status, item.Service, item.SliceName)
			eventChan <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item.Namespace, Data: item}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			_, err := k.convertToEndpoints(oldObj, store.EMPTY)
			if errors.Is(err, ErrIgnored) {
				return
			}
			item2, _ := k.convertToEndpoints(newObj, store.MODIFIED)
			// fix modified state for ones that are deleted,new,same
			logger.Tracef("[RUNTIME] [K8s] %s %s: %s %s", ENDPOINTS, item2.Status, item2.Service, item2.SliceName)
			eventChan <- SyncDataEvent{SyncType: ENDPOINTS, Namespace: item2.Namespace, Data: item2}
		},
	})
	logger.Error(err)
}

func (k k8s) convertToEndpoints(obj interface{}, status store.Status) (*store.Endpoints, error) {
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

func getServiceAddresses(service *corev1.Service) (addresses []string) {
	switch service.Spec.Type {
	case corev1.ServiceTypeExternalName:
		addresses = []string{service.Spec.ExternalName}
	case corev1.ServiceTypeClusterIP:
		addresses = []string{service.Spec.ClusterIP}
		addresses = append(addresses, service.Spec.ExternalIPs...)
	case corev1.ServiceTypeNodePort:
		if service.Spec.ClusterIP == "" {
			addresses = append(addresses, service.Spec.ClusterIP)
		}
		addresses = append(addresses, service.Spec.ExternalIPs...)
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
	if addresses == nil {
		addresses = []string{}
	}
	return
}

type InformerGetter interface {
	Informer() cache.SharedIndexInformer
}

type GatewayRelatedType interface {
	*gatewayv1beta1.GatewayClass | *gatewayv1beta1.Gateway | *gatewayv1alpha2.TCPRoute | *gatewayv1alpha2.ReferenceGrant
}

type GatewayInformerFunc[GWType GatewayRelatedType] func(gwObj GWType, eventChan chan SyncDataEvent, status store.Status)

func manageGatewayClass(gatewayclass *gatewayv1beta1.GatewayClass, eventChan chan SyncDataEvent, status store.Status) {
	logger.Infof("gwapi: gatewayclass: informers: got '%s'", gatewayclass.Name)
	item := store.GatewayClass{
		Name:           gatewayclass.Name,
		ControllerName: string(gatewayclass.Spec.ControllerName),
		Description:    gatewayclass.Spec.Description,
		Generation:     gatewayclass.Generation,
		Status:         status,
	}
	logger.Tracef("[RUNTIME] [K8s] %s %s: %s", GATEWAYCLASS, item.Status, item.Name)
	eventChan <- SyncDataEvent{SyncType: GATEWAYCLASS, Data: &item}
}

func manageGateway(gateway *gatewayv1beta1.Gateway, eventChan chan SyncDataEvent, status store.Status) {
	logger.Infof("gwapi: gateway: informers: got '%s/%s'", gateway.Namespace, gateway.Name)
	listeners := make([]store.Listener, len(gateway.Spec.Listeners))
	for i, listener := range gateway.Spec.Listeners {
		listeners[i] = store.Listener{
			Name:        string(listener.Name),
			Port:        int32(listener.Port),
			Protocol:    string(listener.Protocol),
			Hostname:    (*string)(listener.Hostname),
			GwNamespace: gateway.Namespace,
			GwName:      gateway.Name,
		}
		if listener.AllowedRoutes != nil {
			listeners[i].AllowedRoutes = &store.AllowedRoutes{}
			if listener.AllowedRoutes.Namespaces != nil {
				var from *string
				if listener.AllowedRoutes.Namespaces.From != nil {
					tmpFrom := string(*listener.AllowedRoutes.Namespaces.From)
					from = &tmpFrom
				}
				listeners[i].AllowedRoutes.Namespaces = &store.RouteNamespaces{
					From:     from,
					Selector: ConvertFromK8SLabelSelector(listener.AllowedRoutes.Namespaces.Selector),
				}
			}
			rgks := make([]store.RouteGroupKind, len(listener.AllowedRoutes.Kinds))
			for j, rgk := range listener.AllowedRoutes.Kinds {
				rgks[j] = store.RouteGroupKind{
					Group: (*string)(rgk.Group),
					Kind:  string(rgk.Kind),
				}
			}
			listeners[i].AllowedRoutes.Kinds = rgks
		}
	}
	item := store.Gateway{
		Name:             gateway.Name,
		Namespace:        gateway.Namespace,
		GatewayClassName: string(gateway.Spec.GatewayClassName),
		Listeners:        listeners,
		Generation:       gateway.Generation,
		Status:           status,
	}
	logger.Tracef("[RUNTIME] [K8s] %s %s: %s", GATEWAY, item.Status, item.Name)
	eventChan <- SyncDataEvent{SyncType: GATEWAY, Namespace: item.Namespace, Data: &item}
}

func manageTCPRoute(tcproute *gatewayv1alpha2.TCPRoute, eventChan chan SyncDataEvent, status store.Status) {
	logger.Debugf("gwapi: tcproute: informers: got '%s/%s'", tcproute.Namespace, tcproute.Name)
	backendRefs := []store.BackendRef{}
	for _, rule := range tcproute.Spec.Rules {
		for _, backendref := range rule.BackendRefs {
			backendRefs = append(backendRefs, store.BackendRef{
				Name: string(backendref.Name),
				Namespace: func() *string {
					if backendref.Namespace != nil {
						return (*string)(backendref.Namespace)
					}
					return nil
				}(),
				Port:   (*int32)(backendref.Port),
				Group:  (*string)(backendref.Group),
				Kind:   (*string)(backendref.Kind),
				Weight: backendref.Weight,
			})
		}
	}
	parentRefs := make([]store.ParentRef, 0, len(tcproute.Spec.ParentRefs))
	for _, parentRefSpec := range tcproute.Spec.ParentRefs {
		// Ensure ParentRefs is only about Gateway resources.
		parentRefGroup := "gateway.networking.k8s.io"
		if parentRefSpec.Group != nil {
			parentRefGroup = *(*string)(parentRefSpec.Group)
		}
		parentRefKind := "Gateway"
		if parentRefSpec.Kind != nil {
			parentRefKind = *(*string)(parentRefSpec.Kind)
		}
		if parentRefGroup != "gateway.networking.k8s.io" || parentRefKind != "Gateway" {
			logger.Errorf("invalid parent reference in tcproute '%s/%s': parent reference must of kind 'Gateway' from group 'gateway.networking.k8s.io'", tcproute.Namespace, tcproute.Name)
			continue
		}
		parentRefNs := (*string)(parentRefSpec.Namespace)
		if parentRefNs == nil {
			parentRefNs = &tcproute.Namespace
		}
		parentRef := store.ParentRef{
			Namespace:   parentRefNs,
			Name:        string(parentRefSpec.Name),
			SectionName: (*string)(parentRefSpec.SectionName),
			Port:        (*int32)(parentRefSpec.Port),
			Group:       parentRefGroup,
			Kind:        parentRefKind,
		}
		parentRefs = append(parentRefs, parentRef)
	}

	item := store.TCPRoute{
		Name:         tcproute.Name,
		Namespace:    tcproute.Namespace,
		BackendRefs:  backendRefs,
		ParentRefs:   parentRefs,
		CreationTime: tcproute.CreationTimestamp.Time,
		Generation:   tcproute.Generation,
		Status:       status,
	}
	logger.Tracef("[RUNTIME] [K8s] %s %s: %s", TCPROUTE, item.Status, item.Name)
	eventChan <- SyncDataEvent{SyncType: TCPROUTE, Namespace: item.Namespace, Data: &item}
}

func (k k8s) getGatewayClassesInformer(eventChan chan SyncDataEvent, factory gatewaynetworking.SharedInformerFactory) cache.SharedIndexInformer {
	informer := factory.Gateway().V1beta1().GatewayClasses()
	PopulateInformer(eventChan, informer, GatewayInformerFunc[*gatewayv1beta1.GatewayClass](manageGatewayClass))
	return informer.Informer()
}

func (k k8s) getGatewayInformer(eventChan chan SyncDataEvent, factory gatewaynetworking.SharedInformerFactory) cache.SharedIndexInformer {
	informer := factory.Gateway().V1beta1().Gateways()
	PopulateInformer(eventChan, informer, GatewayInformerFunc[*gatewayv1beta1.Gateway](manageGateway))
	return informer.Informer()
}

func (k k8s) getTCPRouteInformer(eventChan chan SyncDataEvent, factory gatewaynetworking.SharedInformerFactory) cache.SharedIndexInformer {
	informer := factory.Gateway().V1alpha2().TCPRoutes()
	PopulateInformer(eventChan, informer, GatewayInformerFunc[*gatewayv1alpha2.TCPRoute](manageTCPRoute))
	return informer.Informer()
}

func PopulateInformer[IT InformerGetter, GWType GatewayRelatedType, GWF GatewayInformerFunc[GWType]](eventChan chan SyncDataEvent, informer IT, handler GWF) cache.SharedIndexInformer {
	_, err := informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			gatewaytype := obj.(GWType)
			handler(gatewaytype, eventChan, store.ADDED)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			gatewaytype := newObj.(GWType)
			handler(gatewaytype, eventChan, store.MODIFIED)
		},
		DeleteFunc: func(obj interface{}) {
			gatewaytype := obj.(GWType)
			handler(gatewaytype, eventChan, store.DELETED)
		},
	})
	logger.Error(err)
	return informer.Informer()
}

func (k k8s) getReferenceGrantInformer(eventChan chan SyncDataEvent, factory gatewaynetworking.SharedInformerFactory) cache.SharedIndexInformer {
	informer := factory.Gateway().V1alpha2().ReferenceGrants()
	PopulateInformer(eventChan, informer, GatewayInformerFunc[*gatewayv1alpha2.ReferenceGrant](manageReferenceGrant))
	return informer.Informer()
}

func manageReferenceGrant(referenceGrant *gatewayv1alpha2.ReferenceGrant, eventChan chan SyncDataEvent, status store.Status) {
	logger.Debugf("gwapi: referencegrant: informers: got '%s/%s'", referenceGrant.Namespace, referenceGrant.Name)
	item := store.ReferenceGrant{
		Name:       referenceGrant.Name,
		Namespace:  referenceGrant.Namespace,
		Generation: referenceGrant.Generation,
		Status:     status,
	}
	item.From = make([]store.ReferenceGrantFrom, len(referenceGrant.Spec.From))
	item.To = make([]store.ReferenceGrantTo, len(referenceGrant.Spec.To))

	for i, from := range referenceGrant.Spec.From {
		item.From[i] = store.ReferenceGrantFrom{
			Group:     string(from.Group),
			Kind:      string(from.Kind),
			Namespace: string(from.Namespace),
		}
	}

	for i, to := range referenceGrant.Spec.To {
		item.To[i] = store.ReferenceGrantTo{
			Group: string(to.Group),
			Kind:  string(to.Kind),
			Name:  (*string)(to.Name),
		}
	}

	logger.Tracef("[RUNTIME] [K8s] %s %s: %s", REFERENCEGRANT, item.Status, item.Name)
	eventChan <- SyncDataEvent{SyncType: REFERENCEGRANT, Namespace: item.Namespace, Data: &item}
}
