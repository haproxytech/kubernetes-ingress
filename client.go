package main

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

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
	if err != nil {
		panic(err.Error())
	}
	return &K8s{API: clientset}, nil
}

//GetNamespaces returns namespaces
func (k *K8s) GetNamespaces() ([]string, watch.Interface, error) {
	items, err := k.API.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	watchChanges, err := k.API.CoreV1().Namespaces().Watch(metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	num := len(items.Items)
	itemList := make([]string, num, num)
	for i, ns := range items.Items {
		itemList[i] = ns.GetName()
	}
	return itemList, watchChanges, nil
}

//GetPods returns pods
func (k *K8s) GetPods() ([]string, watch.Interface, error) {
	items, err := k.API.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	watchChanges, err := k.API.CoreV1().Pods("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	num := len(items.Items)
	itemList := make([]string, num, num)
	for i, ns := range items.Items {
		itemList[i] = ns.GetName()
	}
	return itemList, watchChanges, nil
}

//GetIngresses returns ingresses
func (k *K8s) GetIngresses() ([]string, watch.Interface, error) {

	items, err := k.API.Extensions().Ingresses("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	watchChanges, err := k.API.Extensions().Ingresses("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	num := len(items.Items)
	itemList := make([]string, num, num)
	for i, ns := range items.Items {
		itemList[i] = ns.GetName()
	}
	return itemList, watchChanges, nil
}

//GetServices returns services
func (k *K8s) GetServices() ([]string, watch.Interface, error) {
	items, err := k.API.CoreV1().Services("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	watchChanges, err := k.API.CoreV1().Services("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	num := len(items.Items)
	itemList := make([]string, num, num)
	for i, ns := range items.Items {
		itemList[i] = ns.GetName()
	}
	return itemList, watchChanges, nil
}

/*
	nss, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
	//lista := nss.List(metav1.ListOptions{})
	for i, ns := range nss.Items {
		fmt.Println(i, ns.GetName(), ns.GetAnnotations())

		//cm, _ := clientset.CoreV1().ConfigMaps(ns.GetName()).List(metav1.ListOptions{})
		//fmt.Println("CONFIG MAP", ns.GetName())
		//for _, m := range cm.Items {
		//	fmt.Println(m)
		//}
		gledaj, _ := clientset.CoreV1().ConfigMaps(ns.GetName()).Watch(metav1.ListOptions{})
		go observeConfigMap(ns.GetName(), gledaj)

		ingressChan, _ := clientset.Extensions().Ingresses(ns.GetName()).Watch(metav1.ListOptions{})
		go observeIngress(ns.GetName(), ingressChan)
	}
*/
