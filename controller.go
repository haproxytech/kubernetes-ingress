package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/watch"

	clientnative "github.com/haproxytech/client-native"
	"github.com/haproxytech/client-native/configuration"
	"github.com/haproxytech/client-native/stats"
	"github.com/haproxytech/models"
)

//HAProxyController is ingress controller
type HAProxyController struct {
	k8s       *K8s
	Namespace map[string]*Namespace
	ConfigMap map[string]*ConfigMap
	Secret    map[string]*Secret
	NativeAPI *clientnative.HAProxyClient
}

// Start initialize and run HAProxyController
func (c *HAProxyController) Start() {

	go c.ReloadHAProxy()

	k8s, err := GetKubernetesClient()
	if err != nil {
		log.Panic(err)
	}
	c.k8s = k8s

	x := k8s.API.Discovery()
	k8sVersion, _ := x.ServerVersion()
	log.Printf("Running on Kubernetes version: %s %s", k8sVersion.String(), k8sVersion.Platform)

	_, nsWatch, err := k8s.GetNamespaces()
	if err != nil {
		log.Panic(err)
	}

	_, svcWatch, err := k8s.GetServices()
	if err != nil {
		log.Panic(err)
	}

	_, podWatch, err := k8s.GetPods()
	if err != nil {
		log.Panic(err)
	}

	_, ingressWatch, err := k8s.GetIngresses()
	if err != nil {
		log.Panic(err)
	}

	_, configMapWatch, err := k8s.GetConfigMap()
	if err != nil {
		log.Panic(err)
	}

	_, secretsWatch, err := k8s.GetSecrets()
	if err != nil {
		log.Panic(err)
	}

	confClient := configuration.NewLBCTLClient(HAProxyCFG, "/usr/sbin/lbctl", "")
	statsClient := stats.NewStatsClient(HAProxySocket)
	//client := client_native.New(confClient, statsClient)
	c.NativeAPI = clientnative.New(confClient, statsClient)

	go c.watchChanges(nsWatch, svcWatch, podWatch, ingressWatch, configMapWatch, secretsWatch)
}

//InitializeHAProxy runs HAProxy for the first time so native client can have access to it
func (c *HAProxyController) ReloadHAProxy() {
	//cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	log.Println("Starting HAProxy with", HAProxyCFG)
	cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	err := cmd.Run()
	if err != nil {
		log.Println(err)
	}
}

func (c *HAProxyController) watchChanges(namespaces watch.Interface, services watch.Interface, pods watch.Interface,
	ingresses watch.Interface, configMapWatch watch.Interface, secretsWatch watch.Interface) {
	syncEveryNSeconds := 5
	eventChan := make(chan SyncDataEvent, watch.DefaultChanSize)
	go c.SyncData(eventChan)
	for {
		select {
		case msg := <-namespaces.ResultChan():
			obj := msg.Object.(*corev1.Namespace)
			namespace := Namespace{
				Name:  obj.GetName(),
				Watch: obj.GetName() == "default",
				//Annotations
				Pods:      make(map[string]*Pod),
				Services:  make(map[string]*Service),
				Ingresses: make(map[string]*Ingress),
			}
			eventChan <- SyncDataEvent{SyncType: NAMESPACE, EventType: msg.Type, Namespace: &namespace}
		case msg := <-services.ResultChan():
			obj := msg.Object.(*corev1.Service)
			svc := Service{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
				//ClusterIP:  "string",
				//ExternalIP: "string",
				Ports: obj.Spec.Ports,

				Annotations: obj.ObjectMeta.Annotations,
				Selector:    obj.Spec.Selector,
			}
			eventChan <- SyncDataEvent{SyncType: SERVICE, EventType: msg.Type, Service: &svc}
		case msg := <-pods.ResultChan():
			obj := msg.Object.(*corev1.Pod)
			//LogWatchEvent(msg.Type, POD, obj)
			pod := Pod{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
				Labels:    obj.Labels,
				IP:        obj.Status.PodIP,
				Status:    obj.Status.Phase,
				//Port:      obj.Status. ? yes no, check
			}
			eventChan <- SyncDataEvent{SyncType: POD, EventType: msg.Type, Pod: &pod}
		case msg := <-ingresses.ResultChan():
			obj := msg.Object.(*extensionsv1beta1.Ingress)
			ingress := Ingress{
				Name:        obj.GetName(),
				Namespace:   obj.GetNamespace(),
				Annotations: obj.ObjectMeta.Annotations,
				Rules:       obj.Spec.Rules,
			}
			eventChan <- SyncDataEvent{SyncType: INGRESS, EventType: msg.Type, Ingress: &ingress}
		case msg := <-configMapWatch.ResultChan():
			obj := msg.Object.(*corev1.ConfigMap)
			//only config with name=haproxy-configmap is interesting
			if obj.ObjectMeta.GetName() == "haproxy-configmap" {
				configMap := ConfigMap{
					Name: obj.GetName(),
					Data: obj.Data,
				}
				eventChan <- SyncDataEvent{SyncType: CONFIGMAP, EventType: msg.Type, ConfigMap: &configMap}
			}
		case msg := <-secretsWatch.ResultChan():
			obj := msg.Object.(*corev1.Secret)
			secret := Secret{
				Name: obj.ObjectMeta.GetName(),
				Data: obj.Data,
			}
			eventChan <- SyncDataEvent{SyncType: SECRET, EventType: msg.Type, Secret: &secret}
		case <-time.After(time.Duration(syncEveryNSeconds) * time.Second):
			//TODO syncEveryNSeconds sec is hardcoded, change that (annotation?)
			//do sync of data every syncEveryNSeconds sec
			eventChan <- SyncDataEvent{SyncType: COMMAND, EventType: watch.Added}
		}
	}
}

//SyncData gets all kubernetes changes, aggregates them and apply to HAProxy.
//All the changes must come through this function
func (c *HAProxyController) SyncData(jobChan <-chan SyncDataEvent) {
	hadChanges := false
	c.Namespace = make(map[string]*Namespace)
	c.ConfigMap = make(map[string]*ConfigMap)
	c.Secret = make(map[string]*Secret)
	for job := range jobChan {
		switch job.SyncType {
		case COMMAND:
			if hadChanges {
				log.Println("job processing", job.SyncType)
				c.UpdateHAProxy()
				hadChanges = false
			}
		case NAMESPACE:
			hadChanges = c.eventNamespace(job.EventType, job.Namespace)
		case INGRESS:
			hadChanges = c.eventIngress(job.EventType, job.Ingress)
		case SERVICE:
			hadChanges = c.eventService(job.EventType, job.Service)
		case POD:
			hadChanges = c.eventPod(job.EventType, job.Pod)
		case CONFIGMAP:
			hadChanges = c.eventConfigMap(job.EventType, job.ConfigMap)
		case SECRET:
			hadChanges = c.eventSecret(job.EventType, job.Secret)
		}
	}
}

func (c *HAProxyController) eventNamespace(eventType watch.EventType, data *Namespace) bool {
	updateRequired := false
	switch eventType {
	case watch.Added:
		ns := c.GetNamespace(data.Name)
		log.Println("Namespace added", ns.Name)
		updateRequired = true
	case watch.Deleted:
		_, ok := c.Namespace[data.Name]
		if ok {
			delete(c.Namespace, data.Name)
			log.Println("Namespace deleted", data.Name)
			updateRequired = true
			c.UpdateHAProxy()
		} else {
			log.Println("Namespace not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (c *HAProxyController) eventIngress(eventType watch.EventType, data *Ingress) bool {
	updateRequired := false
	switch eventType {
	case watch.Modified:
		newIngress := data
		ns := c.GetNamespace(data.Namespace)
		oldIngress, ok := ns.Ingresses[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Ingress not registered with controller, cannot modify !", data.Name)
		}
		ns.Ingresses[data.Name] = newIngress
		diffStr := cmp.Diff(oldIngress, newIngress)
		log.Println("Ingress modified", data.Name, "\n", diffStr)
		diff := cmp.Equal(oldIngress, newIngress)
		log.Println(diff)
		updateRequired = true
	case watch.Added:
		ns := c.GetNamespace(data.Namespace)
		ns.Ingresses[data.Name] = data
		log.Println("Ingress added", data.Name)
		updateRequired = true
	case watch.Deleted:
		ns := c.GetNamespace(data.Namespace)
		_, ok := ns.Ingresses[data.Name]
		if ok {
			delete(ns.Ingresses, data.Name)
			log.Println("Ingress deleted", data.Name)
			//update immediately
			c.UpdateHAProxy()
		} else {
			log.Println("Ingress not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (c *HAProxyController) eventService(eventType watch.EventType, data *Service) bool {
	updateRequired := false
	switch eventType {
	case watch.Modified:
		newService := data
		ns := c.GetNamespace(data.Namespace)
		oldService, ok := ns.Services[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Service not registered with controller, cannot modify !", data.Name)
		}
		ns.Services[data.Name] = newService
		result := cmp.Diff(oldService, newService)
		log.Println("Service modified", data.Name, "\n", result)
		updateRequired = true
	case watch.Added:
		ns := c.GetNamespace(data.Namespace)
		ns.Services[data.Name] = data
		log.Println("Service added", data.Name)
		updateRequired = true
	case watch.Deleted:
		ns := c.GetNamespace(data.Namespace)
		_, ok := ns.Services[data.Name]
		if ok {
			delete(ns.Services, data.Name)
			log.Println("Service deleted", data.Name)
			c.UpdateHAProxy()
		} else {
			log.Println("Service not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (c *HAProxyController) eventPod(eventType watch.EventType, data *Pod) bool {
	updateRequired := false
	switch eventType {
	case watch.Modified:
		newPod := data
		ns := c.GetNamespace(data.Namespace)
		oldPod, ok := ns.Pods[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Pod not registered with controller, cannot modify !", data.Name)
		}
		ns.Pods[data.Name] = newPod
		result := cmp.Diff(oldPod, newPod)
		log.Println("Pod modified", data.Name, oldPod.Status, "\n", result)
		updateRequired = true
	case watch.Added:
		ns := c.GetNamespace(data.Namespace)
		ns.Pods[data.Name] = data
		log.Println("Pod added", data.Name)
		updateRequired = true
	case watch.Deleted:
		ns := c.GetNamespace(data.Namespace)
		_, ok := ns.Pods[data.Name]
		if ok {
			delete(ns.Pods, data.Name)
			log.Println("Pod deleted", data.Name)
			//update immediately
			c.UpdateHAProxy()
		} else {
			log.Println("Pod not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (c *HAProxyController) eventConfigMap(eventType watch.EventType, data *ConfigMap) bool {
	updateRequired := false
	switch eventType {
	case watch.Modified:
		newConfigMap := data
		oldConfigMap, ok := c.ConfigMap[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("ConfigMap not registered with controller, cannot modify !", data.Name)
		}
		c.ConfigMap[data.Name] = newConfigMap
		result := cmp.Diff(oldConfigMap, newConfigMap)
		log.Println("ConfigMap modified", data.Name, "\n", result)
		updateRequired = true
	case watch.Added:
		c.ConfigMap[data.Name] = data
		log.Println("ConfigMap added", data.Name)
		updateRequired = true
	case watch.Deleted:
		_, ok := c.ConfigMap[data.Name]
		if ok {
			delete(c.ConfigMap, data.Name)
			log.Println("ConfigMap deleted", data.Name)
			c.UpdateHAProxy()
		} else {
			log.Println("ConfigMap not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}
func (c *HAProxyController) eventSecret(eventType watch.EventType, data *Secret) bool {
	updateRequired := false
	switch eventType {
	case watch.Modified:
		newSecret := data
		oldSecret, ok := c.Secret[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Secret not registered with controller, cannot modify !", data.Name)
		}
		c.Secret[data.Name] = newSecret
		result := cmp.Diff(oldSecret, newSecret)
		log.Println("Secret modified", data.Name, "\n", result)
		updateRequired = true
	case watch.Added:
		c.Secret[data.Name] = data
		log.Println("Secret added", data.Name)
		updateRequired = true
	case watch.Deleted:
		_, ok := c.Secret[data.Name]
		if ok {
			delete(c.Secret, data.Name)
			log.Println("Secret deleted", data.Name)
			c.UpdateHAProxy()
		} else {
			log.Println("Secret not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (c *HAProxyController) GetNamespace(name string) *Namespace {
	namespace, ok := c.Namespace[name]
	if ok {
		return namespace
	}
	newNamespace := c.NewNamespace(name)
	c.Namespace[name] = newNamespace
	return newNamespace
}

func (c *HAProxyController) NewNamespace(name string) *Namespace {
	namespace := Namespace{
		Name:  name,
		Watch: name == "default",
		//Annotations
		Pods:      make(map[string]*Pod),
		Services:  make(map[string]*Service),
		Ingresses: make(map[string]*Ingress),
	}
	return &namespace
}

func (c *HAProxyController) NewUpdateHAProxy() {
	nativeAPI := c.NativeAPI
	//backend, err := nativeAPI.Configuration.GetBackend("default-http-svc-8080")
	//log.Println(backend, err)
	version, err := nativeAPI.Configuration.GetVersion()
	if err != nil {
		//silently fallback to 1
		version = 1
	}
	transaction, err := nativeAPI.Configuration.StartTransaction(version)
	var frontendHTTP *models.Frontend
	frontendHTTPGet, err := nativeAPI.Configuration.GetFrontend("http")
	if err != nil {
		log.Println("frontend http does not exists, creating one")
		frontendHTTP = &models.Frontend{
			Name: "http",
			//Mode: "http",
			Protocol: "http",
		}
		nativeAPI.Configuration.CreateFrontend(frontendHTTP, transaction.ID, transaction.Version)
	} else {
		frontendHTTP = frontendHTTPGet.Data
	}
	for _, namespace := range c.Namespace {
		if !namespace.Watch {
			continue
		}
		for _, ingress := range namespace.Ingresses {
			for _, rule := range ingress.Rules {
				/*var acl *models.ACL
				//TODO get fetch system
				acl = &models.ACL{
					Criterion: "var(txn.hdr_host) -i",
					//ID int64 `json:"id"`
					Name:  "acl host-" + rule.Host,
					Value: rule.Host,
				}
				//WriteBufferedString(&acls, "    acl host-", rule.Host, " var(txn.hdr_host) -i ", rule.Host)
				log.Println(acl)*/
				//checkACL, err := nativeAPI.Configuration.
				for _, path := range rule.HTTP.Paths {
					log.Println(path)
					/*
						service, ok := namespace.Services[path.Backend.ServiceName]
						if !ok {
							log.Println("service", path.Backend.ServiceName, "does not exists")
							continue
						}
						WriteBufferedString(&useBackend,
							"    use_backend ", namespace.Name, "-", path.Backend.ServiceName, "-", path.Backend.ServicePort.String(),
							" if host-", rule.Host, " { var(txn.path) -m beg ", path.Path, " }\n")

						selector := service.Selector
						if len(selector) == 0 {
							log.Println("service", service.Name, "no selector")
							continue
						}
						backendName := namespace.Name + "-" + service.Name + "-" + path.Backend.ServicePort.String()
						_, ok = createdBackends[backendName]
						if !ok {
							WriteBufferedString(&backends,
								"backend ", backendName, "\n",
								"    mode http\n",
								"    balance leastconn\n")
							index := 0
							for _, pod := range namespace.Pods {
								//TODO what about state unknown, should we do something about it?
								if pod.Status == corev1.PodRunning && hasSelectors(selector, pod.Labels) {
									WriteBufferedString(&backends,
										"    server server000", strconv.Itoa(index), " ", pod.IP, ":", path.Backend.ServicePort.String(),
										" weight 1 check port ", path.Backend.ServicePort.String(), "\n")
									index++
								}
							}
							createdBackends[backendName] = true
						}
						backends.WriteString("\n")
						/* */
				}
			}
		}
	}
	//err = nativeAPI.Configuration.CommitTransaction(transaction.ID)
	if err != nil {
		log.Println(err)
	}
}

//SimpleUpdateHAProxy this need to generate API call/calls for HAProxy API
//currently it only generates direct cfg file for checking
func (c *HAProxyController) UpdateHAProxy() {
	c.NewUpdateHAProxy()

	var frontend strings.Builder
	var acls strings.Builder
	var useBackend strings.Builder
	var backends strings.Builder
	createdBackends := make(map[string]bool)
	WriteBufferedString(&frontend, "frontend http\n", "    mode http\n    bind *:80\n")
	for _, namespace := range c.Namespace {
		if !namespace.Watch {
			continue
		}
		for _, ingress := range namespace.Ingresses {
			for _, rule := range ingress.Rules {
				WriteBufferedString(&acls, "    acl host-", rule.Host, " var(txn.hdr_host) -i ", rule.Host)
				for _, path := range rule.HTTP.Paths {
					service, ok := namespace.Services[path.Backend.ServiceName]
					if !ok {
						log.Println("service", path.Backend.ServiceName, "does not exists")
						continue
					}
					//WriteBufferedString(&acls, " ", rule.Host, ":", "80", " ", "\n")

					//acls.WriteString("    acl host-foo.bar var(txn.hdr_host) -i foo.bar foo.bar:80 foo.bar:443\n")
					//"    acl host-foo.bar var(txn.hdr_host) -i foo.bar foo.bar:80 foo.bar:443\n")
					WriteBufferedString(&useBackend,
						"    use_backend ", namespace.Name, "-", path.Backend.ServiceName, "-", path.Backend.ServicePort.String(),
						" if host-", rule.Host, " { var(txn.path) -m beg ", path.Path, " }\n")
					//use_backend default-web-5858 if host-foo.bar { var(txn.path) -m beg /web }

					selector := service.Selector
					if len(selector) == 0 {
						log.Println("service", service.Name, "no selector")
						continue
					}
					backendName := namespace.Name + "-" + service.Name + "-" + path.Backend.ServicePort.String()
					_, ok = createdBackends[backendName]
					if !ok {
						WriteBufferedString(&backends,
							"backend ", backendName, "\n",
							"    mode http\n",
							"    balance leastconn\n")
						index := 0
						for _, pod := range namespace.Pods {
							//TODO what about state unknown, should we do something about it?
							if pod.Status == corev1.PodRunning && hasSelectors(selector, pod.Labels) {
								WriteBufferedString(&backends,
									"    server server000", strconv.Itoa(index), " ", pod.IP, ":", path.Backend.ServicePort.String(),
									" weight 1 check port ", path.Backend.ServicePort.String(), "\n")
								index++
							}
						}
						createdBackends[backendName] = true
					}
					backends.WriteString("\n")
				}
			}
		}
	}
	var config strings.Builder
	WriteBufferedString(&config, "# _version=1\n\n", getGlobal(), "\n\n", getDefault(), "\n\n", frontend.String(), "\n", acls.String(), "\n", useBackend.String(), "\n\n", backends.String())
	cfg := config.String()
	//fmt.Println(cfg)
	os.MkdirAll("/etc/haproxy/", 0644)

	tmpfile, err := ioutil.TempFile("", "haproxy-*.cfg")
	if err != nil {
		log.Fatal(err)
	}

	if _, err := tmpfile.WriteString(cfg); err != nil {
		log.Println(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Println(err)
	}
	cfgPath := tmpfile.Name()

	log.Println("The file path : ", cfgPath)
	cmd := exec.Command("haproxy", "-c", "-f", cfgPath)
	log.Printf("Running command and waiting for it to finish...")
	log.Println("haproxy", "-c", "-f", cfgPath)
	err = cmd.Run()
	if err != nil {
		//there is no point of looking what because this controller will communicate with api
		log.Println("Command finished with error: %v", err)
	} else {
		log.Println("it looks as everything is ok with config")
	}
	err = os.Remove(HAProxyCFG)
	if err != nil {
		log.Println(err)
	}
	err = os.Rename(cfgPath, HAProxyCFG)
	if err != nil {
		log.Println(err)
	}
	log.Println("Config changed. reloading")
	c.ReloadHAProxy()
	//defer os.Remove(tmpfile.Name()) // clean up
}
