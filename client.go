package main

import (
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const DEBUG_API = true

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
	go watchNew(config, clientset)
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

//GetPods returns pods
func (k *K8s) GetPods() (watch.Interface, error) {
	a, err := k.API.CoreV1().Pods("default").List(metav1.ListOptions{})
	log.Println(len(a.Items))
	watchChanges, err := k.API.CoreV1().Pods("default").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
}

func startWatchPods(config *rest.Config, clientset *kubernetes.Clientset) {
	watchlist := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
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
				fmt.Printf("pod added: %s \n", data.GetName())
				status := "ADDED"
				if data.ObjectMeta.GetDeletionTimestamp() != nil {
					//detetct pods that are in terminating state
					status = DELETED
				}
				pod := &Pod{
					Namespace: data.GetNamespace(),
					Name:      data.GetName(),
					Labels:    ConvertToMapStringW(obj.Labels),
					IP:        obj.Status.PodIP,
					Status:    status,
				}
				if DEBUG_API {
					fmt.Printf("POD %s: %s \n", pod.Status, pod.Name)
				}
				//fmt.Printf("pod added: %s \n", obj)
			},
			DeleteFunc: func(obj interface{}) {
				data := obj.(*corev1.Pod)
				fmt.Printf("pod deleted: %s \n", data.GetName())
				//fmt.Printf("pod deleted: %s \n", obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				data1 := oldObj.(*corev1.Pod)
				data2 := newObj.(*corev1.Pod)
				//fmt.Printf("service changed \n")
				fmt.Printf("pod changed \n")
			},
		},
	)
	stop := make(chan struct{})
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

//GetServices returns services
func (k *K8s) GetServices() (watch.Interface, error) {
	watchChanges, err := k.API.CoreV1().Services("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
}

//GetConfigMap returns config map for controller
func (k *K8s) GetConfigMap() (watch.Interface, error) {

	watchChanges, err := k.API.CoreV1().ConfigMaps("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
}

//GetSecrets returns kubernetes secrets
func (k *K8s) GetSecrets() (watch.Interface, error) {
	watchChanges, err := k.API.CoreV1().Secrets("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
}
