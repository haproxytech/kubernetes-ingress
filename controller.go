package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/haproxytech/config-parser/parsers/global"
	"github.com/haproxytech/config-parser/parsers/simple"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/watch"

	clientnative "github.com/haproxytech/client-native"
	"github.com/haproxytech/client-native/configuration"
	"github.com/haproxytech/client-native/runtime"
	"github.com/haproxytech/config-parser"
	"github.com/haproxytech/models"
)

// HAProxyController is ingress controller
type HAProxyController struct {
	k8s          *K8s
	cfg          Configuration
	osArgs       OSArgs
	NativeAPI    *clientnative.HAProxyClient
	NativeParser parser.Parser
}

// Start initialize and run HAProxyController
func (c *HAProxyController) Start(osArgs OSArgs) {

	c.osArgs = osArgs

	c.HAProxyInitialize()

	k8s, err := GetKubernetesClient()
	if err != nil {
		log.Panic(err)
	}
	c.k8s = k8s

	x := k8s.API.Discovery()
	k8sVersion, _ := x.ServerVersion()
	log.Printf("Running on Kubernetes version: %s %s", k8sVersion.String(), k8sVersion.Platform)

	nsWatch, svcWatch, podWatch, ingressWatch, configMapWatch, secretsWatch := k8s.GetAll()

	go c.watchChanges(nsWatch, svcWatch, podWatch, ingressWatch, configMapWatch, secretsWatch)
}

//HAProxyInitialize runs HAProxy for the first time so native client can have access to it
func (c *HAProxyController) HAProxyInitialize() {
	//cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	err := os.MkdirAll(HAProxyCertDir, 0644)
	if err != nil {
		log.Panic(err.Error())
	}

	log.Println("Starting HAProxy with", HAProxyCFG)
	cmd := exec.Command("service", "haproxy", "start")
	err = cmd.Run()
	if err != nil {
		log.Println(err)
	}

	c.NativeParser = parser.Parser{}
	err = c.NativeParser.LoadData(HAProxyGlobalCFG)
	if err != nil {
		log.Panic(err)
	}

	runtimeClient := runtime.Client{}
	err = runtimeClient.Init([]string{"/var/run/haproxy-runtime-api.sock"})
	if err != nil {
		log.Println(err)
	}

	confClient := configuration.LBCTLClient{}
	confClient.Init(HAProxyCFG, HAProxyGlobalCFG, "haproxy", "/usr/sbin/lbctl", "")

	c.NativeAPI = &clientnative.HAProxyClient{
		Configuration: &confClient,
		Runtime:       &runtimeClient,
	}
}

//HAProxyReload reloads HAProxy
func (c *HAProxyController) HAProxyReload() error {
	c.NativeParser.Save(HAProxyGlobalCFG)
	//cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	cmd := exec.Command("service", "haproxy", "reload")
	err := cmd.Run()
	return err
}

func (c *HAProxyController) watchChanges(namespaces watch.Interface, services watch.Interface, pods watch.Interface,
	ingresses watch.Interface, configMapWatch watch.Interface, secretsWatch watch.Interface) {
	syncEveryNSeconds := 5
	eventChan := make(chan SyncDataEvent, watch.DefaultChanSize*6)
	configMapReceivedAndProccesed := make(chan bool)
	//initOver := true
	eventsIngress := []SyncDataEvent{}
	eventsServices := []SyncDataEvent{}
	eventsPods := []SyncDataEvent{}

	go c.SyncData(eventChan, configMapReceivedAndProccesed)
	configMapOk := false

	for {
		select {
		case _ = <-configMapReceivedAndProccesed:
			for _, event := range eventsIngress {
				eventChan <- event
			}
			for _, event := range eventsServices {
				eventChan <- event
			}
			for _, event := range eventsPods {
				eventChan <- event
			}
			time.Sleep(1 * time.Second)
			eventsIngress = []SyncDataEvent{}
			eventsServices = []SyncDataEvent{}
			eventsPods = []SyncDataEvent{}
			configMapOk = true
		case msg, ok := <-namespaces.ResultChan():
			if !ok || msg.Object == nil {
				namespaces, services, pods, ingresses, configMapWatch, secretsWatch = c.k8s.GetAll()
				goruntime.GC()
				continue
			}
			obj := msg.Object.(*corev1.Namespace)
			namespace := &Namespace{
				Name:     obj.GetName(),
				Relevant: obj.GetName() == "default",
				//Annotations
				Pods:      make(map[string]*Pod),
				PodNames:  make(map[string]bool),
				Services:  make(map[string]*Service),
				Ingresses: make(map[string]*Ingress),
				Secret:    make(map[string]*Secret),
				Status:    msg.Type,
			}
			eventChan <- SyncDataEvent{SyncType: NAMESPACE, Namespace: obj.GetName(), Data: namespace}
		case msg, ok := <-services.ResultChan():
			if !ok || msg.Object == nil {
				namespaces, services, pods, ingresses, configMapWatch, secretsWatch = c.k8s.GetAll()
				goruntime.GC()
				continue
			}
			obj := msg.Object.(*corev1.Service)
			svc := &Service{
				Name: obj.GetName(),
				//ClusterIP:  "string",
				//ExternalIP: "string",
				//Ports: obj.Spec.Ports,

				Annotations: ConvertToMapStringW(obj.ObjectMeta.Annotations),
				Selector:    ConvertToMapStringW(obj.Spec.Selector),
				Status:      msg.Type,
			}
			event := SyncDataEvent{SyncType: SERVICE, Namespace: obj.GetNamespace(), Data: svc}
			if configMapOk {
				eventChan <- event
			} else {
				eventsServices = append(eventsServices, event)
			}
		case msg, ok := <-pods.ResultChan():
			if !ok || msg.Object == nil {
				namespaces, services, pods, ingresses, configMapWatch, secretsWatch = c.k8s.GetAll()
				goruntime.GC()
				continue
			}
			obj := msg.Object.(*corev1.Pod)
			status := msg.Type
			if obj.ObjectMeta.GetDeletionTimestamp() != nil {
				//detetct pods that are in terminating state
				status = watch.Deleted
			}
			pod := &Pod{
				Name:   obj.GetName(),
				Labels: ConvertToMapStringW(obj.Labels),
				IP:     obj.Status.PodIP,
				Status: status,
			}
			event := SyncDataEvent{SyncType: POD, Namespace: obj.GetNamespace(), Data: pod}
			if configMapOk {
				eventChan <- event
			} else {
				eventsPods = append(eventsPods, event)
			}
		case msg, ok := <-ingresses.ResultChan():
			if !ok || msg.Object == nil {
				namespaces, services, pods, ingresses, configMapWatch, secretsWatch = c.k8s.GetAll()
				goruntime.GC()
				continue
			}
			obj := msg.Object.(*extensionsv1beta1.Ingress)
			ingress := &Ingress{
				Name:        obj.GetName(),
				Annotations: ConvertToMapStringW(obj.ObjectMeta.Annotations),
				Rules:       ConvertIngressRules(obj.Spec.Rules),
				Status:      msg.Type,
			}
			event := SyncDataEvent{SyncType: INGRESS, Namespace: obj.GetNamespace(), Data: ingress}
			if configMapOk {
				eventChan <- event
			} else {
				eventsIngress = append(eventsIngress, event)
			}
		case msg, ok := <-configMapWatch.ResultChan():
			if !ok || msg.Object == nil {
				namespaces, services, pods, ingresses, configMapWatch, secretsWatch = c.k8s.GetAll()
				goruntime.GC()
				continue
			}
			obj := msg.Object.(*corev1.ConfigMap)
			//only config with name=haproxy-configmap is interesting
			if obj.ObjectMeta.GetName() == "haproxy-configmap" {
				configMap := &ConfigMap{
					Name:        obj.GetName(),
					Annotations: ConvertToMapStringW(obj.Data),
					Status:      msg.Type,
				}
				eventChan <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: obj.GetNamespace(), Data: configMap}
			}
		case msg, ok := <-secretsWatch.ResultChan():
			if !ok || msg.Object == nil {
				namespaces, services, pods, ingresses, configMapWatch, secretsWatch = c.k8s.GetAll()
				goruntime.GC()
				continue
			}
			obj := msg.Object.(*corev1.Secret)
			secret := &Secret{
				Name:   obj.ObjectMeta.GetName(),
				Data:   obj.Data,
				Status: msg.Type,
			}
			eventChan <- SyncDataEvent{SyncType: SECRET, Namespace: obj.GetNamespace(), Data: secret}
		case <-time.After(time.Duration(syncEveryNSeconds) * time.Second):
			//TODO syncEveryNSeconds sec is hardcoded, change that (annotation?)
			//do sync of data every syncEveryNSeconds sec
			if configMapOk && len(eventsIngress) == 0 && len(eventsServices) == 0 && len(eventsPods) == 0 {
				eventChan <- SyncDataEvent{SyncType: COMMAND}
			}
		}
	}
}

//SyncData gets all kubernetes changes, aggregates them and apply to HAProxy.
//All the changes must come through this function
//TODO this is not necessary, remove it later
func (c *HAProxyController) SyncData(jobChan <-chan SyncDataEvent, chConfigMapReceivedAndProccesed chan bool) {
	hadChanges := false
	needsReload := false
	c.cfg.Init(c.NativeAPI)
	for job := range jobChan {
		ns := c.cfg.GetNamespace(job.Namespace)
		change := false
		reload := false
		switch job.SyncType {
		case COMMAND:
			if hadChanges {
				if err := c.updateHAProxy(needsReload); err != nil {
					log.Println(err)
				}
				hadChanges = false
				needsReload = false
				continue
			}
		case NAMESPACE:
			change, reload = c.eventNamespace(ns, job.Data.(*Namespace))
		case INGRESS:
			change, reload = c.eventIngress(ns, job.Data.(*Ingress))
		case SERVICE:
			change, reload = c.eventService(ns, job.Data.(*Service))
		case POD:
			change, reload = c.eventPod(ns, job.Data.(*Pod))
		case CONFIGMAP:
			change, reload = c.eventConfigMap(ns, job.Data.(*ConfigMap), chConfigMapReceivedAndProccesed)
		case SECRET:
			change, reload = c.eventSecret(ns, job.Data.(*Secret))
		}
		hadChanges = hadChanges || change
		needsReload = needsReload || reload
	}
}

func (c *HAProxyController) eventNamespace(ns *Namespace, data *Namespace) (updateRequired, needsReload bool) {
	updateRequired = false
	switch data.Status {
	case watch.Added:
		_ = c.cfg.GetNamespace(data.Name)
		//ns := c.cfg.GetNamespace(data.Name)
		//log.Println("Namespace added", ns.Name)
		updateRequired = true
	case watch.Deleted:
		_, ok := c.cfg.Namespace[data.Name]
		if ok {
			delete(c.cfg.Namespace, data.Name)
			//log.Println("Namespace deleted", data.Name)
			updateRequired = true
		} else {
			log.Println("Namespace not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired, true
}

func (c *HAProxyController) eventIngress(ns *Namespace, data *Ingress) (updateRequired, needsReload bool) {
	updateRequired = false
	switch data.Status {
	case watch.Modified:
		newIngress := data
		oldIngress, ok := ns.Ingresses[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if idea of only watching is ok
			log.Println("Ingress not registered with controller, cannot modify !", data.Name)
			return false, false
		}
		if oldIngress.Equal(data) {
			return false, false
		}
		newIngress.Annotations.SetStatus(oldIngress.Annotations)
		//so see what exactly has changed in there
		for _, newRule := range newIngress.Rules {
			if oldRule, ok := oldIngress.Rules[newRule.Host]; ok {
				//so we need to compare if anything is different
				for _, newPath := range newRule.Paths {
					if oldPath, ok := oldRule.Paths[newPath.Path]; ok {
						//compare path for differences
						if newPath.ServiceName != oldPath.ServiceName ||
							newPath.ServicePort != oldPath.ServicePort {
							newPath.Status = watch.Modified
							newRule.Status = watch.Modified
						}
					} else {
						newPath.Status = watch.Modified
						newRule.Status = watch.Modified
					}
				}
				for _, oldPath := range oldRule.Paths {
					if _, ok := newRule.Paths[oldPath.Path]; ok {
						oldPath.Status = watch.Deleted
						newRule.Paths[oldPath.Path] = oldPath
					}
				}
			} else {
				newRule.Status = watch.Added
			}
		}
		for _, oldRule := range oldIngress.Rules {
			if _, ok := newIngress.Rules[oldRule.Host]; !ok {
				oldRule.Status = watch.Deleted
				for _, path := range oldRule.Paths {
					path.Status = watch.Deleted
				}
				newIngress.Rules[oldRule.Host] = oldRule
			}
		}
		ns.Ingresses[data.Name] = newIngress
		//diffStr := cmp.Diff(oldIngress, newIngress)
		//log.Println("Ingress modified", data.Name, "\n", diffStr)
		updateRequired = true
	case watch.Added:
		if old, ok := ns.Ingresses[data.Name]; ok {
			data.Status = old.Status
			if !old.Equal(data) {
				data.Status = watch.Modified
				return c.eventIngress(ns, data)
			}
			return updateRequired, updateRequired
		}
		ns.Ingresses[data.Name] = data
		//log.Println("Ingress added", data.Name)
		updateRequired = true
	case watch.Deleted:
		ingress, ok := ns.Ingresses[data.Name]
		if ok {
			ingress.Status = watch.Deleted
			for _, rule := range ingress.Rules {
				rule.Status = watch.Deleted
				for _, path := range rule.Paths {
					path.Status = watch.Deleted
				}
			}
			ingress.Annotations.SetStatusState(watch.Deleted)
			//log.Println("Ingress deleted", data.Name)
			updateRequired = true
		} else {
			log.Println("Ingress not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired, updateRequired
}

func (c *HAProxyController) eventService(ns *Namespace, data *Service) (updateRequired, needsReload bool) {
	updateRequired = false
	switch data.Status {
	case watch.Modified:
		newService := data
		oldService, ok := ns.Services[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Service not registered with controller, cannot modify !", data.Name)
		}
		if oldService.Equal(newService) {
			return updateRequired, updateRequired
		}
		newService.Annotations.SetStatus(oldService.Annotations)
		ns.Services[data.Name] = newService
		//result := cmp.Diff(oldService, newService)
		//log.Println("Service modified", data.Name, "\n", result)
		updateRequired = true
	case watch.Added:
		if old, ok := ns.Services[data.Name]; ok {
			if !old.Equal(data) {
				data.Status = watch.Modified
				return c.eventService(ns, data)
			}
			return updateRequired, updateRequired
		}
		ns.Services[data.Name] = data
		//log.Println("Service added", data.Name)
		updateRequired = true
	case watch.Deleted:
		service, ok := ns.Services[data.Name]
		if ok {
			service.Status = watch.Deleted
			service.Annotations.SetStatusState(watch.Deleted)
			//log.Println("Service deleted", data.Name)
			updateRequired = true
		} else {
			log.Println("Service not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired, updateRequired
}

func (c *HAProxyController) eventPod(ns *Namespace, data *Pod) (updateRequired, needsReload bool) {
	updateRequired = false
	needsReload = false
	runtimeClient := c.cfg.NativeAPI.Runtime
	switch data.Status {
	case watch.Modified:
		newPod := data
		var oldPod *Pod
		oldPod, ok := ns.Pods[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Pod not registered with controller, cannot modify !", data.Name)
			return updateRequired, needsReload
		}
		if oldPod.Equal(data) {
			return updateRequired, needsReload
		}
		newPod.HAProxyName = oldPod.HAProxyName
		newPod.Backends = oldPod.Backends
		if oldPod.Status == watch.Added {
			newPod.Status = watch.Added
		} else {
			//so, old is not just added, see diff and if only ip is different
			// issue socket command to change ip ad set it to ready
			if newPod.IP != oldPod.IP && len(oldPod.Backends) > 0 {
				for backendName := range newPod.Backends {
					err := runtimeClient.SetServerAddr(backendName, newPod.HAProxyName, newPod.IP, 0)
					if err != nil {
						log.Println(err)
						needsReload = true
					} else {
						log.Printf("POD modified through runtime: %s\n", data.Name)
					}
					err = runtimeClient.SetServerState(backendName, newPod.HAProxyName, "ready")
					if err != nil {
						log.Println(err)
						needsReload = true
					}
				}
			}
		}
		ns.Pods[data.Name] = newPod
		//result := cmp.Diff(oldPod, newPod)
		//log.Println("Pod modified", data.Name, oldPod.Status, "\n", newPod.HAProxyName, oldPod.HAProxyName, "/n", result)
		updateRequired = true
	case watch.Added:
		if old, ok := ns.Pods[data.Name]; ok {
			data.HAProxyName = old.HAProxyName
			if old.Equal(data) {
				//so this is actually modified
				data.Status = watch.Modified
				return c.eventPod(ns, data)
			}
			return updateRequired, needsReload
		}
		//first see if we have spare place in servers
		//INFO if same pod used in multiple services, this will not work
		createNew := true
		var pods map[string]*Pod
		if services, err := ns.GetServicesForPod(data.Labels); err == nil {
			// we will see if we need to support behaviour where same pod is shared between services
			service := services[0]
			pods = ns.GetPodsForSelector(service.Selector)
			//now see if we have some free place where we can place pod
			for _, pod := range pods {
				if pod.Maintenance {
					createNew = false
					data.Maintenance = false
					if pod.Status == watch.Added {
						data.Status = watch.Added
					} else {
						data.Status = watch.Modified
					}
					data.HAProxyName = pod.HAProxyName
					data.Backends = pod.Backends
					ns.Pods[data.Name] = data
					delete(ns.Pods, pod.Name)
					updateRequired = true
					needsReload = false
					for backendName := range data.Backends {
						if data.IP != "" {
							err := runtimeClient.SetServerAddr(backendName, data.HAProxyName, data.IP, 0)
							if err != nil {
								log.Println(backendName, data.HAProxyName, data.IP, err)
								needsReload = true
							}
						}
						err := runtimeClient.SetServerState(backendName, data.HAProxyName, "ready")
						if err != nil {
							log.Println(backendName, data.HAProxyName, err)
							needsReload = true
						} else {
							log.Printf("POD added through runtime: %s\n", data.Name)
						}
					}
					break
				}
			}
		}
		if createNew {
			data.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
			for _, ok := ns.PodNames[data.HAProxyName]; ok; {
				data.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
			}
			ns.PodNames[data.HAProxyName] = true
			ns.Pods[data.Name] = data
			//log.Println("Pod added", data.Name)

			updateRequired = true
			needsReload = true

			annIncrement, _ := GetValueFromAnnotations("servers-increment", c.cfg.ConfigMap.Annotations)
			incrementSize := int64(128)
			if increment, err := strconv.ParseInt(annIncrement.Value, 10, 64); err == nil {
				incrementSize = increment
			}
			podsNumber := int64(len(pods) + 1)
			if podsNumber%incrementSize != 0 {
				for index := podsNumber % incrementSize; index < incrementSize; index++ {
					pod := &Pod{
						IP:          "127.0.0.1",
						Labels:      data.Labels.Clone(),
						Maintenance: true,
						Status:      watch.Added,
					}
					pod.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
					for _, ok := ns.PodNames[pod.HAProxyName]; ok; {
						pod.HAProxyName = fmt.Sprintf("SRV_%s", RandomString(5))
					}
					pod.Name = pod.HAProxyName
					ns.PodNames[pod.HAProxyName] = true
					ns.Pods[pod.Name] = pod
				}
			}
		}
	case watch.Deleted:
		oldPod, ok := ns.Pods[data.Name]
		if ok {
			if oldPod.Maintenance {
				//this occurres because we have a terminating signal (converted to delete)
				//and later we receive delete that is no longer relevant
				//log.Println("Pod already put to sleep !", data.Name)
				return updateRequired, needsReload
			}
			annIncrement, _ := GetValueFromAnnotations("servers-increment-max-disabled", c.cfg.ConfigMap.Annotations)
			maxDisabled := int64(8)
			if increment, err := strconv.ParseInt(annIncrement.Value, 10, 64); err == nil {
				maxDisabled = increment
			}
			var service *Service
			convertToMaintPod := true
			if services, err := ns.GetServicesForPod(data.Labels); err == nil {
				// we will see if we need to support behaviour where same pod is shared between services
				service = services[0]
				pods := ns.GetPodsForSelector(service.Selector)
				//first count number of disabled pods
				numDisabled := int64(0)
				for _, pod := range pods {
					if pod.Maintenance {
						numDisabled++
					}
				}
				if numDisabled >= maxDisabled {
					convertToMaintPod = false
					oldPod.Status = watch.Deleted
					needsReload = true
				}
			}
			if convertToMaintPod {
				oldPod.IP = "127.0.0.1"
				oldPod.Status = watch.Modified //we replace it with disabled one
				oldPod.Maintenance = true
				for backendName := range oldPod.Backends {
					err := runtimeClient.SetServerState(backendName, oldPod.HAProxyName, "maint")
					if err != nil {
						log.Println(backendName, oldPod.HAProxyName, err)
					} else {
						log.Printf("POD disabled through runtime: %s\n", oldPod.Name)
					}
				}
			}
			updateRequired = true
		}
	}
	return updateRequired, needsReload
}

func (c *HAProxyController) eventConfigMap(ns *Namespace, data *ConfigMap, chConfigMapReceivedAndProccesed chan bool) (updateRequired, needsReload bool) {
	updateRequired = false
	if ns.Name != c.osArgs.ConfigMap.Namespace ||
		data.Name != c.osArgs.ConfigMap.Name {
		return updateRequired, needsReload
	}
	switch data.Status {
	case watch.Modified:

		different := data.Annotations.SetStatus(c.cfg.ConfigMap.Annotations)
		c.cfg.ConfigMap = data
		if !different {
			data.Status = ""
		} else {
			updateRequired = true
		}
	case watch.Added:
		if c.cfg.ConfigMap == nil {
			chConfigMapReceivedAndProccesed <- true
			c.cfg.ConfigMap = data
			updateRequired = true
			return updateRequired, updateRequired
		}
		if !c.cfg.ConfigMap.Equal(data) {
			data.Status = watch.Modified
			return c.eventConfigMap(ns, data, chConfigMapReceivedAndProccesed)
		}
	case watch.Deleted:
		c.cfg.ConfigMap.Annotations.SetStatusState(watch.Deleted)
		c.cfg.ConfigMap.Status = watch.Deleted
	}
	return updateRequired, updateRequired
}
func (c *HAProxyController) eventSecret(ns *Namespace, data *Secret) (updateRequired, needsReload bool) {
	updateRequired = false
	switch data.Status {
	case watch.Modified:
		newSecret := data
		oldSecret, ok := ns.Secret[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Secret not registered with controller, cannot modify !", data.Name)
			return updateRequired, updateRequired
		}
		if oldSecret.Equal(data) {
			return updateRequired, updateRequired
		}
		ns.Secret[data.Name] = newSecret
		//result := cmp.Diff(oldSecret, newSecret)
		//log.Println("Secret modified", data.Name, "\n", result)
		updateRequired = true
	case watch.Added:
		if old, ok := ns.Secret[data.Name]; ok {
			if !old.Equal(data) {
				data.Status = watch.Modified
				return c.eventSecret(ns, data)
			}
			return updateRequired, updateRequired
		}
		ns.Secret[data.Name] = data
		updateRequired = true
	case watch.Deleted:
		_, ok := ns.Secret[data.Name]
		if ok {
			//log.Println("Secret set for deletion", data.Name)
			updateRequired = true
		} else {
			log.Println("Secret not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired, updateRequired
}

func (c *HAProxyController) updateHAProxy(reloadRequired bool) error {
	nativeAPI := c.NativeAPI

	c.handleGlobalTimeouts()
	version, err := nativeAPI.Configuration.GetVersion()
	if err != nil || version < 1 {
		//silently fallback to 1
		version = 1
	}
	//log.Println("Config version:", version)
	transaction, err := nativeAPI.Configuration.StartTransaction(version)
	if err != nil {
		log.Println(err)
		return err
	}

	if maxconnAnn, err := GetValueFromAnnotations("maxconn", c.cfg.ConfigMap.Annotations); err == nil {
		if maxconn, err := strconv.ParseInt(maxconnAnn.Value, 10, 64); err == nil {
			if maxconnAnn.Status == watch.Deleted {
				maxconnAnn, _ = GetValueFromAnnotations("maxconn", c.cfg.ConfigMap.Annotations) // has default
				maxconn, _ = strconv.ParseInt(maxconnAnn.Value, 10, 64)
			}
			if maxconnAnn.Status != "" {
				if frontend, err := nativeAPI.Configuration.GetFrontend("http", transaction.ID); err == nil {
					frontend.Data.MaxConnections = &maxconn
					nativeAPI.Configuration.EditFrontend("http", frontend.Data, transaction.ID, 0)
				} else {
					return err
				}
			}
		}
	}

	maxProcs, maxThreads, reload, err := c.handleGlobalAnnotations(transaction)
	reloadRequired = reloadRequired || reload

	for _, namespace := range c.cfg.Namespace {
		if !namespace.Relevant {
			continue
		}
		var usingHTTPS bool
		reload, usingHTTPS, err = c.handleHTTPS(namespace, maxProcs, maxThreads, transaction)
		if err != nil {
			return err
		}
		numProcs, _ := strconv.Atoi(maxProcs.Value)
		numThreads, _ := strconv.Atoi(maxThreads.Value)
		port := int64(80)
		listener := &models.Listener{
			Name:    "http_1",
			Address: "0.0.0.0",
			Port:    &port,
			Process: "1/1",
		}
		if !usingHTTPS {
			if numProcs > 1 {
				listener.Process = "all"
			}
			if numThreads > 1 {
				listener.Process = "all"
			}
		}
		if listener.Process != c.cfg.HTTPBindProcess {
			if err = nativeAPI.Configuration.EditListener(listener.Name, FrontendHTTP, listener, transaction.ID, 0); err != nil {
				return err
			}
			c.cfg.HTTPBindProcess = listener.Process
		}
		reloadRequired = reloadRequired || reload
		reload, err = c.handleHTTPRedirect(usingHTTPS, transaction)
		if err != nil {
			return err
		}
		reloadRequired = reloadRequired || reload
		//TODO, do not just go through them, sort them to handle /web,/ maybe?
		for _, ingress := range namespace.Ingresses {
			//no need for switch/case for now
			backendsUsed := map[string]int{}
			for _, rule := range ingress.Rules {
				//nothing to switch/case for now
				for _, path := range rule.Paths {
					c.handlePath(namespace, ingress, rule, path, transaction, backendsUsed)
				}
			}
			for backendName, numberOfTimesBackendUsed := range backendsUsed {
				if numberOfTimesBackendUsed < 1 {
					if err := nativeAPI.Configuration.DeleteBackend(backendName, transaction.ID, 0); err != nil {
						log.Println("Cannot delete backend", err)
					}
				}
			}
		}
	}
	err = nativeAPI.Configuration.CommitTransaction(transaction.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	c.cfg.Clean()
	if reloadRequired {
		if err := c.HAProxyReload(); err != nil {
			log.Println(err)
		} else {
			log.Println("HAProxy reloaded")
		}
	} else {
		log.Println("HAProxy updated without reload")
	}
	return nil
}

func (c *HAProxyController) writeCert(filename string, key, crt []byte) error {
	var f *os.File
	var err error
	if f, err = os.Create(filename); err != nil {
		log.Println(err)
		return err
	}
	defer f.Close()
	if _, err = f.Write(key); err != nil {
		log.Println(err)
		return err
	}
	if _, err = f.Write(crt); err != nil {
		log.Println(err)
		return err
	}
	if err = f.Sync(); err != nil {
		log.Println(err)
		return err
	}
	if err = f.Close(); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (c *HAProxyController) handleGlobalAnnotations(transaction *models.Transaction) (maxProcsStat *StringW, maxThreadsStat *StringW, reloadRequired bool, err error) {
	reloadRequired = false
	maxProcs := goruntime.GOMAXPROCS(0)
	numThreads := maxProcs
	annNumProc, _ := GetValueFromAnnotations("ssl-numproc", c.cfg.ConfigMap.Annotations)
	annNbthread, _ := GetValueFromAnnotations("nbthread", c.cfg.ConfigMap.Annotations)
	maxProcsStat = &StringW{}
	maxThreadsStat = &StringW{}
	if numthr, err := strconv.Atoi(annNbthread.Value); err == nil {
		if numthr < maxProcs {
			numThreads = numthr
		}
	}
	if numproc, err := strconv.Atoi(annNumProc.Value); err == nil {
		if numproc < maxProcs {
			maxProcs = numproc
		}
	}

	//see global config
	p := c.NativeParser
	var nbproc *global.NbProc
	data, err := p.GetGlobalAttr("nbproc")
	if err == nil {
		nbproc = data.(*global.NbProc)
		if nbproc.Value != int64(maxProcs) {
			reloadRequired = true
			nbproc.Value = int64(maxProcs)
			maxProcsStat.Status = watch.Modified
		}
	} else {
		nbproc = &global.NbProc{
			Enabled: true,
			Value:   int64(maxProcs),
		}
		p.NewGlobalAttr(nbproc)
		maxProcsStat.Status = watch.Added
		reloadRequired = true
	}
	if maxProcs > 1 {
		numThreads = 1
	}

	var nbthread *global.NbThread
	data, err = p.GetGlobalAttr("nbthread")
	if err == nil {
		nbthread = data.(*global.NbThread)
		if nbthread.Value != int64(numThreads) {
			reloadRequired = true
			nbthread.Value = int64(numThreads)
			maxThreadsStat.Status = watch.Modified
		}
	} else {
		nbthread = &global.NbThread{
			Enabled: true,
			Value:   int64(numThreads),
		}
		p.NewGlobalAttr(nbthread)
		maxThreadsStat.Status = watch.Added
		reloadRequired = true
	}

	data, err = p.GetGlobalAttr("cpu-map")
	numCPUMap := numThreads
	namePrefix := "1/"
	if nbthread.Value < 2 {
		numCPUMap = maxProcs
		namePrefix = ""
	}
	cpuMap := make([]*global.CpuMap, numCPUMap)
	for index := 0; index < numCPUMap; index++ {
		cpuMap[index] = &global.CpuMap{
			Name:  fmt.Sprintf("%s%d", namePrefix, index+1),
			Value: strconv.Itoa(index),
		}
	}
	cpuMaps := &global.CpuMapLines{CpuMapLines: cpuMap}
	if err == nil {
		mapLines := data.(*global.CpuMapLines)
		if !mapLines.Equal(cpuMaps) {
			reloadRequired = true
			mapLines.CpuMapLines = cpuMaps.CpuMapLines
		}
	} else {
		reloadRequired = true
		p.NewGlobalAttr(cpuMaps)
	}
	maxProcsStat.Value = strconv.Itoa(maxProcs)
	maxThreadsStat.Value = strconv.Itoa(numThreads)
	return maxProcsStat, maxThreadsStat, reloadRequired, err
}

func (c *HAProxyController) removeHTTPSListeners(transaction *models.Transaction) (err error) {
	listeners := *c.cfg.HTTPSListeners
	for index, data := range listeners {
		data.Status = watch.Deleted
		listenerName := "https_" + strconv.Itoa(index+1)
		if err = c.NativeAPI.Configuration.DeleteListener(listenerName, FrontendHTTPS, transaction.ID, 0); err != nil {
			return err
		}
	}
	return nil
}

func (c *HAProxyController) handleHTTPRedirect(usingHTTPS bool, transaction *models.Transaction) (reloadRequired bool, err error) {
	//see if we need to add redirect to https redirect scheme https if !{ ssl_fc }
	// no need for error checking, we have default value,
	//if not defined as OFF, we always do redirect
	reloadRequired = false
	sslRedirect, _ := GetValueFromAnnotations("ssl-redirect", c.cfg.ConfigMap.Annotations)
	useSSLRedirect := sslRedirect.Value != "OFF"
	if !usingHTTPS {
		useSSLRedirect = false
	}
	var state watch.EventType
	if useSSLRedirect {
		if c.cfg.SSLRedirect == "" {
			c.cfg.SSLRedirect = "ON"
			state = watch.Added
		} else if c.cfg.SSLRedirect == "OFF" {
			c.cfg.SSLRedirect = "ON"
			state = watch.Added
		}
	} else {
		if c.cfg.SSLRedirect == "" {
			c.cfg.SSLRedirect = "OFF"
			state = ""
		} else if c.cfg.SSLRedirect != "OFF" {
			c.cfg.SSLRedirect = "OFF"
			state = watch.Deleted
		}
	}
	redirectCode := int64(302)
	annRedirectCode, _ := GetValueFromAnnotations("ssl-redirect-code", c.cfg.ConfigMap.Annotations)
	if value, err := strconv.ParseInt(annRedirectCode.Value, 10, 64); err == nil {
		redirectCode = value
	}
	if state == "" && annRedirectCode.Status != "" {
		state = watch.Modified
	}
	rule := &models.HTTPRequestRule{
		ID:        1,
		Type:      "redirect",
		RedirCode: redirectCode,
		RedirTo:   "https",
		RedirType: "scheme",
		Cond:      "if",
		CondTest:  "!{ ssl_fc }",
	}
	switch state {
	case watch.Added:
		if err = c.NativeAPI.Configuration.CreateHTTPRequestRule("frontend", "http", rule, transaction.ID, 0); err != nil {
			return reloadRequired, err
		}
		c.cfg.SSLRedirect = "ON"
		reloadRequired = true
	case watch.Modified:
		if err = c.NativeAPI.Configuration.EditHTTPRequestRule(rule.ID, "frontend", "http", rule, transaction.ID, 0); err != nil {
			return reloadRequired, err
		}
		reloadRequired = true
	case watch.Deleted:
		if err = c.NativeAPI.Configuration.DeleteHTTPRequestRule(rule.ID, "frontend", "http", transaction.ID, 0); err != nil {
			return reloadRequired, err
		}
		c.cfg.SSLRedirect = "OFF"
		reloadRequired = true
	}
	return reloadRequired, nil
}

func (c *HAProxyController) handleHTTPS(namespace *Namespace, maxProcsStatus, numThreadsStat *StringW, transaction *models.Transaction) (reloadRequired bool, usingHTTPS bool, err error) {
	usingHTTPS = false
	nativeAPI := c.NativeAPI
	reloadRequired = false
	if c.osArgs.DefaultCertificate.Name == "" {
		err := c.removeHTTPSListeners(transaction)
		return reloadRequired, usingHTTPS, err
	}
	secretName, err := GetValueFromAnnotations("ssl-certificate", c.cfg.ConfigMap.Annotations)
	minProc := 1
	maxProcs, _ := strconv.Atoi(maxProcsStatus.Value) // always number
	numThreads, _ := strconv.Atoi(numThreadsStat.Value)
	if maxProcs < 2 {
		if numThreads < 2 {
			minProc = 0
		}
	}

	if secretName.Status != "" || maxProcsStatus.Status != "" {
		if err != nil {
			log.Println("no ssl-certificate defined, using default secret:", c.osArgs.DefaultCertificate.Name)
			secretName = &StringW{Value: c.osArgs.DefaultCertificate.Name}
		}
		secret, ok := namespace.Secret[secretName.Value]
		if !ok {
			log.Println("secret not found", secretName.Value)
			err := c.removeHTTPSListeners(transaction)
			return reloadRequired, usingHTTPS, err
		}
		//two options are allowed, tls, rsa+ecdsa
		rsaKey, rsaKeyOK := secret.Data["rsa.key"]
		rsaCrt, rsaCrtOK := secret.Data["rsa.crt"]
		ecdsaKey, ecdsaKeyOK := secret.Data["ecdsa.key"]
		ecdsaCrt, ecdsaCrtOK := secret.Data["ecdsa.crt"]
		haveCert := false
		if rsaKeyOK && rsaCrtOK || ecdsaKeyOK && ecdsaCrtOK {
			if rsaKeyOK && rsaCrtOK {
				err := c.writeCert(HAProxyCertDir+"cert.pem.rsa", rsaKey, rsaCrt)
				if err != nil {
					c.removeHTTPSListeners(transaction)
					return reloadRequired, usingHTTPS, err
				}
				haveCert = true
			}
			if ecdsaKeyOK && ecdsaCrtOK {
				err := c.writeCert(HAProxyCertDir+"cert.pem.ecdsa", ecdsaKey, ecdsaCrt)
				if err != nil {
					c.removeHTTPSListeners(transaction)
					return reloadRequired, usingHTTPS, err
				}
				haveCert = true
			}
		} else {
			tlsKey, tlsKeyOK := secret.Data["tls.key"]
			tlsCrt, tlsCrtOK := secret.Data["tls.crt"]
			if tlsKeyOK && tlsCrtOK {
				err := c.writeCert(HAProxyCertDir+"cert.pem", tlsKey, tlsCrt)
				if err != nil {
					c.removeHTTPSListeners(transaction)
					return reloadRequired, usingHTTPS, err
				}
				haveCert = true
			}
		}
		if !haveCert {
			c.removeHTTPSListeners(transaction)
			return reloadRequired, usingHTTPS, fmt.Errorf("no certificate")
		}

		port := int64(443)
		listener := &models.Listener{
			Address:        "0.0.0.0",
			Port:           &port,
			Ssl:            "enabled",
			SslCertificate: HAProxyCertDir,
		}
		maxIndex := maxProcs
		if maxProcs < 2 {
			maxIndex = numThreads
		}
		listeners := *c.cfg.HTTPSListeners
		if len(listeners) > maxIndex {
			maxIndex = len(listeners)
		}
		usingHTTPS = true
		for index := minProc; index < maxIndex; index++ {
			data, ok := listeners[index]
			if !ok {
				data = &IntW{
					Status: watch.Added,
				}
				listeners[index] = data
			} else {
				if secret.Status != "" {
					data.Status = secret.Status
				} else if maxProcsStatus.Status != "" {
					data.Status = maxProcsStatus.Status
				}
			}
			if index >= maxProcs && index >= numThreads {
				data.Status = watch.Deleted
			}
			if numThreads < 2 {
				listener.Process = strconv.Itoa(index + 1)
			} else {
				listener.Process = fmt.Sprintf("1/%d", index+1)
			}
			listener.Name = "https_" + strconv.Itoa(index+1)
			switch data.Status {
			case watch.Added:
				if err = nativeAPI.Configuration.CreateListener(FrontendHTTPS, listener, transaction.ID, 0); err != nil {
					if strings.Contains(err.Error(), "already exists") {
						if err = nativeAPI.Configuration.EditListener(listener.Name, FrontendHTTPS, listener, transaction.ID, 0); err != nil {
							return reloadRequired, usingHTTPS, err
						}
					} else {
						return reloadRequired, usingHTTPS, err
					}
				}
			case watch.Modified:
				if err = nativeAPI.Configuration.EditListener(listener.Name, FrontendHTTPS, listener, transaction.ID, 0); err != nil {
					return reloadRequired, usingHTTPS, err
				}
			case watch.Deleted:
				if err = nativeAPI.Configuration.DeleteListener(listener.Name, FrontendHTTPS, transaction.ID, 0); err != nil {
					return reloadRequired, usingHTTPS, err
				}
			}
		}
	}

	listeners := *c.cfg.HTTPSListeners
	for _, listener := range listeners {
		if listener.Status != watch.Deleted {
			return reloadRequired, true, nil
		}
	}
	return reloadRequired, usingHTTPS, nil
}

func (c *HAProxyController) handlePath(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath,
	transaction *models.Transaction, backendsUsed map[string]int) error {
	nativeAPI := c.NativeAPI
	//log.Println("PATH", path)
	backendName, selector, service, err := c.handleService(namespace, ingress, rule, path, backendsUsed, transaction)
	if err != nil {
		return err
	}
	if numberOfTimesBackendUsed := backendsUsed[backendName]; numberOfTimesBackendUsed > 1 {
		return nil
	}
	annMaxconn, _ := GetValueFromAnnotations("pod-maxconn", service.Annotations)
	annCheck, _ := GetValueFromAnnotations("check", service.Annotations, c.cfg.ConfigMap.Annotations)

	for _, pod := range namespace.Pods {
		if hasSelectors(selector, pod.Labels) {
			if pod.Backends == nil {
				pod.Backends = map[string]struct{}{}
			}
			pod.Backends[backendName] = struct{}{}
			port := int64(path.ServicePort)
			weight := int64(128)
			data := &models.Server{
				Name:    pod.HAProxyName,
				Address: pod.IP,
				Port:    &port,
				Weight:  &weight,
			}
			if pod.Maintenance {
				data.Maintenance = "enabled"
			}
			/*if pod.Sorry != "" {
				data.Sorry = pod.Sorry
			}*/
			annnotationsActive := false
			if annMaxconn != nil {
				if annMaxconn.Status != watch.Deleted {
					if maxconn, err := strconv.ParseInt(annMaxconn.Value, 10, 64); err == nil {
						data.MaxConnections = &maxconn
					}
				}
				if annMaxconn.Status != "" {
					annnotationsActive = true
				}
			}
			if annCheck != nil {
				if annCheck.Status != watch.Deleted {
					if annCheck.Value == "enabled" {
						data.Check = "enabled"
						//see if we have port and interval defined
					}
				}
				if annCheck.Status != "" {
					annnotationsActive = true
				}
			}
			if pod.Status == "" && annnotationsActive {
				pod.Status = watch.Modified
			}
			switch pod.Status {
			case watch.Added:
				if err := nativeAPI.Configuration.CreateServer(backendName, data, transaction.ID, 0); err != nil {
					return err
				}
			case watch.Modified:
				if err := nativeAPI.Configuration.EditServer(data.Name, backendName, data, transaction.ID, 0); err != nil {
					return err
				}
			case watch.Deleted:
				if err := nativeAPI.Configuration.DeleteServer(data.Name, backendName, transaction.ID, 0); err != nil {
					return err
				}
			}
		} //if pod.Status...
	} //for pod
	return nil
}

func (c *HAProxyController) handleService(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath,
	backendsUsed map[string]int, transaction *models.Transaction) (backendName string, selector MapStringW, service *Service, err error) {
	nativeAPI := c.NativeAPI

	service, ok := namespace.Services[path.ServiceName]
	if !ok {
		log.Println("service", path.ServiceName, "does not exists")
		return "", nil, nil, fmt.Errorf("service %s does not exists", path.ServiceName)
	}
	selector = service.Selector
	if len(selector) == 0 {
		return "", nil, nil, fmt.Errorf("service %s has no selector", service.Name)
	}

	backendName = fmt.Sprintf("%s-%s-%d", namespace.Name, service.Name, path.ServicePort)
	backendsUsed[backendName]++
	condTest := "{ req.hdr(host) -i " + rule.Host + " } { path_beg " + path.Path + " } "
	//both load-balance and forwarded-for have default values, so no need for error checking
	annBalanceAlg, _ := GetValueFromAnnotations("load-balance", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annForwardedFor, _ := GetValueFromAnnotations("forwarded-for", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	balanceAlg, err := ConvertLoadBalanceAlgorithm(annBalanceAlg.Value)
	if err != nil {
		log.Printf("%s, using %s \n", err, balanceAlg)
	}

	switch service.Status {
	case watch.Added:
		if numberOfTimesBackendUsed := backendsUsed[backendName]; numberOfTimesBackendUsed < 2 {
			backend := &models.Backend{
				Balance:  balanceAlg,
				Name:     backendName,
				Protocol: "http",
			}
			if annForwardedFor.Value == "enabled" { //disabled with anything else is ok
				backend.HTTPXffHeaderInsert = "enabled"
			}
			if err := nativeAPI.Configuration.CreateBackend(backend, transaction.ID, 0); err != nil {
				log.Println("CreateBackend", err)
				return "", nil, nil, err
			}
		}
		backendSwitchingRule := &models.BackendSwitchingRule{
			Cond:       "if",
			CondTest:   condTest,
			TargetFarm: backendName,
			ID:         1,
		}
		if err := nativeAPI.Configuration.CreateBackendSwitchingRule(FrontendHTTP, backendSwitchingRule, transaction.ID, 0); err != nil {
			log.Println("CreateBackendSwitchingRule http", err)
			return "", nil, nil, err
		}
		if err := nativeAPI.Configuration.CreateBackendSwitchingRule(FrontendHTTPS, backendSwitchingRule, transaction.ID, 0); err != nil {
			log.Println("CreateBackendSwitchingRule https", err)
			return "", nil, nil, err
		}
	//case watch.Modified:
	//nothing to do for now
	case watch.Deleted:
		backendsUsed[backendName]--
		if err := nativeAPI.Configuration.DeleteBackend(backendName, transaction.ID, 0); err != nil {
			log.Println("DeleteBackend", err)
			return "", nil, nil, err
		}
		return "", nil, service, nil
	}

	if annBalanceAlg.Status != "" || annForwardedFor.Status != "" {
		if err = c.handleBackendAnnotations(balanceAlg, annForwardedFor, backendName, transaction); err != nil {
			return "", nil, nil, err
		}
	}
	return backendName, selector, service, nil
}

func (c *HAProxyController) handleGlobalTimeouts() bool {
	hasChanges := false
	hasChanges = c.handleGlobalTimeout("http-request") || hasChanges
	hasChanges = c.handleGlobalTimeout("connect") || hasChanges
	hasChanges = c.handleGlobalTimeout("client") || hasChanges
	hasChanges = c.handleGlobalTimeout("queue") || hasChanges
	hasChanges = c.handleGlobalTimeout("server") || hasChanges
	hasChanges = c.handleGlobalTimeout("tunnel") || hasChanges
	hasChanges = c.handleGlobalTimeout("http-keep-alive") || hasChanges
	if hasChanges {
		c.NativeParser.Save(HAProxyGlobalCFG)
	}
	return hasChanges
}

func (c *HAProxyController) handleGlobalTimeout(timeout string) bool {
	client := c.NativeParser
	annTimeout, err := GetValueFromAnnotations(fmt.Sprintf("timeout-%s", timeout), c.cfg.ConfigMap.Annotations)
	if err != nil {
		log.Println(err)
		return false
	}
	if annTimeout.Status != "" {
		//log.Println(fmt.Sprintf("timeout [%s]", timeout), annTimeout.Value, annTimeout.OldValue, annTimeout.Status)
		data, err := client.GetDefaultsAttr(fmt.Sprintf("timeout %s", timeout))
		if err != nil {
			log.Println(err)
			return false
		}
		timeout := data.(*simple.SimpleTimeout)
		timeout.Value = annTimeout.Value
		return true
	}
	return false
}

func (c *HAProxyController) handleBackendAnnotations(balanceAlg string, forwardedFor *StringW,
	backendName string, transaction *models.Transaction) error {
	backend := &models.Backend{
		Balance:  balanceAlg,
		Name:     backendName,
		Protocol: "http",
	}
	if forwardedFor.Value == "enabled" { //disabled with anything else is ok
		backend.HTTPXffHeaderInsert = "enabled"
	}

	if err := c.NativeAPI.Configuration.EditBackend(backend.Name, backend, transaction.ID, 0); err != nil {
		return err
	}
	return nil
}
