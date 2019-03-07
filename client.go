package main

import (
	"log"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const DEBUG_API = false

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

//GetAll fetches all k8s resources
func (k *K8s) GetAll() (ns, svc, pod, ingress, config, secrets watch.Interface) {
	nsWatch, err := k.GetNamespaces()
	if err != nil {
		log.Println(err, nsWatch)
		//log.Panic(err)
	}

	log.Println("++++++++++++++++ 1")
	svcWatch, err := k.GetServices()
	if err != nil {
		log.Panic(err)
	}
	log.Println("++++++++++++++++ 2")

	podWatch, err := k.GetPods()
	if err != nil {
		log.Panic(err)
	}

	ingressWatch, err := k.GetIngresses()
	if err != nil {
		log.Panic(err)
	}

	configMapWatch, err := k.GetConfigMap()
	if err != nil {
		log.Panic(err)
	}

	secretsWatch, err := k.GetSecrets()
	if err != nil {
		log.Panic(err)
	}

	return nsWatch, svcWatch, podWatch, ingressWatch, configMapWatch, secretsWatch
}

//GetNamespaces returns namespaces
func (k *K8s) GetNamespaces() (watch.Interface, error) {
	watchChanges, err := k.API.CoreV1().Namespaces().Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
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
		0, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*corev1.Namespace)
				var status Status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &Namespace{
					Name:     data.GetName(),
					Relevant: data.GetName() == "default",
					//Annotations
					Pods:      make(map[string]*Pod),
					PodNames:  make(map[string]bool),
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
				var status Status = DELETED
				item := &Namespace{
					Name:     data.GetName(),
					Relevant: data.GetName() == "default",
					//Annotations
					Pods:      make(map[string]*Pod),
					PodNames:  make(map[string]bool),
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
				var status Status = MODIFIED
				item1 := &Namespace{
					Name:     data1.GetName(),
					Relevant: data1.GetName() == "default",
					Status:   status,
				}
				item2 := &Namespace{
					Name:     data2.GetName(),
					Relevant: data2.GetName() == "default",
					Status:   status,
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

//GetPods returns pods
func (k *K8s) GetPods() (watch.Interface, error) {
	watchChanges, err := k.API.CoreV1().Pods("default").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
}

func (k *K8s) EventsPods(channel chan *Pod, stop chan struct{}) {
	watchlist := cache.NewListWatchFromClient(
		k.API.CoreV1().RESTClient(),
		string(corev1.ResourcePods),
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&corev1.Pod{},
		0, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*corev1.Pod)
				var status Status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect pods that are in terminating state
					status = DELETED
				}
				item := &Pod{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Labels:    ConvertToMapStringW(data.Labels),
					IP:        data.Status.PodIP,
					Status:    status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", POD, item.Status, item.Name)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*corev1.Pod)
				var status Status = DELETED
				item := &Pod{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Labels:    ConvertToMapStringW(data.Labels),
					IP:        data.Status.PodIP,
					Status:    status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", POD, item.Status, item.Name)
				}
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1 := oldObj.(*corev1.Pod)
				data2 := newObj.(*corev1.Pod)
				var status Status = MODIFIED
				item1 := &Pod{
					Namespace: data1.GetNamespace(),
					Name:      data1.GetName(),
					Labels:    ConvertToMapStringW(data1.Labels),
					IP:        data1.Status.PodIP,
					Status:    status,
				}
				item2 := &Pod{
					Namespace: data2.GetNamespace(),
					Name:      data2.GetName(),
					Labels:    ConvertToMapStringW(data2.Labels),
					IP:        data2.Status.PodIP,
					Status:    status,
				}
				if item2.Equal(item1) {
					return
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", POD, item2.Status, item2.Name)
				}
				channel <- item2
			},
		},
	)
	go controller.Run(stop)
}

//GetIngresses returns ingresses
func (k *K8s) GetIngresses() (watch.Interface, error) {
	watchChanges, err := k.API.Extensions().Ingresses("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
}

func (k *K8s) EventsIngresses(channel chan *Ingress, stop chan struct{}) {
	watchlist := cache.NewListWatchFromClient(
		k.API.Extensions().RESTClient(),
		string("ingresses"),
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&extensionsv1beta1.Ingress{},
		0, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*extensionsv1beta1.Ingress)
				var status Status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &Ingress{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: ConvertToMapStringW(data.ObjectMeta.Annotations),
					Rules:       ConvertIngressRules(data.Spec.Rules),
					Status:      status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", INGRESS, item.Status, item.Name)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*extensionsv1beta1.Ingress)
				var status Status = DELETED
				item := &Ingress{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: ConvertToMapStringW(data.ObjectMeta.Annotations),
					Rules:       ConvertIngressRules(data.Spec.Rules),
					Status:      status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", INGRESS, item.Status, item.Name)
				}
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1 := oldObj.(*extensionsv1beta1.Ingress)
				data2 := newObj.(*extensionsv1beta1.Ingress)
				var status Status = MODIFIED
				item1 := &Ingress{
					Namespace:   data1.GetNamespace(),
					Name:        data1.GetName(),
					Annotations: ConvertToMapStringW(data1.ObjectMeta.Annotations),
					Rules:       ConvertIngressRules(data1.Spec.Rules),
					Status:      status,
				}
				item2 := &Ingress{
					Namespace:   data2.GetNamespace(),
					Name:        data2.GetName(),
					Annotations: ConvertToMapStringW(data2.ObjectMeta.Annotations),
					Rules:       ConvertIngressRules(data2.Spec.Rules),
					Status:      status,
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

//GetServices returns services
func (k *K8s) GetServices() (watch.Interface, error) {
	watchChanges, err := k.API.CoreV1().Services("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
}

func (k *K8s) EventsServices(channel chan *Service, stop chan struct{}) {
	watchlist := cache.NewListWatchFromClient(
		k.API.CoreV1().RESTClient(),
		string(corev1.ResourceServices),
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer( // also take a look at NewSharedIndexInformer
		watchlist,
		&corev1.Service{},
		0, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*corev1.Service)
				var status Status = ADDED
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detect services that are in terminating state
					status = DELETED
				}
				item := &Service{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: ConvertToMapStringW(data.ObjectMeta.Annotations),
					Selector:    ConvertToMapStringW(data.Spec.Selector),
					Status:      status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", SERVICE, item.Status, item.Name)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*corev1.Service)
				var status Status = DELETED
				item := &Service{
					Namespace:   data.GetNamespace(),
					Name:        data.GetName(),
					Annotations: ConvertToMapStringW(data.ObjectMeta.Annotations),
					Selector:    ConvertToMapStringW(data.Spec.Selector),
					Status:      status,
				}
				if DEBUG_API {
					log.Printf("%s %s: %s \n", SERVICE, item.Status, item.Name)
				}
				channel <- item
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1 := oldObj.(*corev1.Service)
				data2 := newObj.(*corev1.Service)
				var status Status = MODIFIED
				item1 := &Service{
					Namespace:   data1.GetNamespace(),
					Name:        data1.GetName(),
					Annotations: ConvertToMapStringW(data1.ObjectMeta.Annotations),
					Selector:    ConvertToMapStringW(data1.Spec.Selector),
					Status:      status,
				}
				item2 := &Service{
					Namespace:   data2.GetNamespace(),
					Name:        data2.GetName(),
					Annotations: ConvertToMapStringW(data2.ObjectMeta.Annotations),
					Selector:    ConvertToMapStringW(data2.Spec.Selector),
					Status:      status,
				}
				if item2.Equal(item1) {
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

//GetConfigMap returns config map for controller
func (k *K8s) GetConfigMap() (watch.Interface, error) {

	watchChanges, err := k.API.CoreV1().ConfigMaps("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
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
		0, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*corev1.ConfigMap)
				var status Status = ADDED
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
				var status Status = DELETED
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
				var status Status = MODIFIED
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

//GetSecrets returns kubernetes secrets
func (k *K8s) GetSecrets() (watch.Interface, error) {
	watchChanges, err := k.API.CoreV1().Secrets("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
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
		0, //Duration is int64
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				data := obj.(*corev1.Secret)
				var status Status = ADDED
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
					//log.Printf("%s %s: %s \n", SECRET, item.Status, item.Name)
				}
				channel <- item
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*corev1.Secret)
				var status Status = DELETED
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
				var status Status = MODIFIED
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
