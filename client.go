package main

import (
	"log"

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

func (k *K8s) GetAll() (ns, svc, pod, ingress, config, secrets watch.Interface) {
	nsWatch, err := k.GetNamespaces()
	if err != nil {
		log.Panic(err)
	}

	svcWatch, err := k.GetServices()
	if err != nil {
		log.Panic(err)
	}

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
	watchChanges, err := k.API.CoreV1().Pods("").Watch(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return watchChanges, nil
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
