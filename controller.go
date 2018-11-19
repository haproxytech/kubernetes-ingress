package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

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

	go c.HAProxyInitialize()

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

	go c.watchChanges(nsWatch, svcWatch, podWatch, ingressWatch, configMapWatch, secretsWatch)
}

//HAProxyInitialize runs HAProxy for the first time so native client can have access to it
func (c *HAProxyController) HAProxyInitialize() {
	//cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	log.Println("Starting HAProxy with", HAProxyCFG)
	cmd := exec.Command("service", "haproxy", "start")
	err := cmd.Run()
	if err != nil {
		log.Println(err)
	}

	c.NativeParser = parser.Parser{}
	err = c.NativeParser.LoadData(HAProxyGlobalCFG)
	if err != nil {
		log.Panic(err)
	}

	runtimeClient := runtime.Client{}
	err = runtimeClient.Init([]string{"/var/run/haproxy-runtime-api.sock"}, true)
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
	//cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	cmd := exec.Command("service", "haproxy", "reload")
	err := cmd.Run()
	return err
}

func (c *HAProxyController) watchChanges(namespaces watch.Interface, services watch.Interface, pods watch.Interface,
	ingresses watch.Interface, configMapWatch watch.Interface, secretsWatch watch.Interface) {
	syncEveryNSeconds := 5
	eventChan := make(chan SyncDataEvent, watch.DefaultChanSize)
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
		case msg := <-namespaces.ResultChan():
			if msg.Object == nil {
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
		case msg := <-services.ResultChan():
			if msg.Object == nil {
				continue
			}
			obj := msg.Object.(*corev1.Service)
			svc := &Service{
				Name: obj.GetName(),
				//ClusterIP:  "string",
				//ExternalIP: "string",
				Ports: obj.Spec.Ports,

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
		case msg := <-pods.ResultChan():
			if msg.Object == nil {
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
		case msg := <-ingresses.ResultChan():
			if msg.Object == nil {
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
		case msg := <-configMapWatch.ResultChan():
			if msg.Object == nil {
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
		case msg := <-secretsWatch.ResultChan():
			if msg.Object == nil {
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
				//log.Println("job processing", job.SyncType, hadChanges, needsReload)
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
		newIngress.Annotations.SetStatus(oldIngress.Annotations)
		if !ok {
			//intentionally do not add it. TODO see if idea of only watching is ok
			log.Println("Ingress not registered with controller, cannot modify !", data.Name)
			return false, false
		}
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
		newService.Annotations.SetStatus(oldService.Annotations)
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Service not registered with controller, cannot modify !", data.Name)
		}
		ns.Services[data.Name] = newService
		//result := cmp.Diff(oldService, newService)
		//log.Println("Service modified", data.Name, "\n", result)
		updateRequired = true
	case watch.Added:
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
						log.Println(backendName, newPod.HAProxyName, newPod.IP, err)
						needsReload = true
					}
					err = runtimeClient.SetServerState(backendName, newPod.HAProxyName, "ready")
					if err != nil {
						log.Println(backendName, newPod.HAProxyName, err)
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
		}
		c.cfg.ConfigMap = data
		updateRequired = true
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
		//oldSecret, ok := c.cfg.Secret[data.Name]
		_, ok := ns.Secret[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Secret not registered with controller, cannot modify !", data.Name)
		}
		ns.Secret[data.Name] = newSecret
		//result := cmp.Diff(oldSecret, newSecret)
		//log.Println("Secret modified", data.Name, "\n", result)
		updateRequired = true
	case watch.Added:
		ns.Secret[data.Name] = data
		//log.Println("Secret added", data.Name)
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

	if err := c.checkHealthzStatus(transaction); err != nil {
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

	frontendHTTP := "http"
	for _, namespace := range c.cfg.Namespace {
		if !namespace.Relevant {
			continue
		}
		if c.osArgs.DefaultCertificate.Name != "" {
			if secret, ok := namespace.Secret[c.osArgs.DefaultCertificate.Name]; ok {
				key, ok := secret.Data["tls.key"]
				if !ok {
					log.Println("missing tls.key")
					return errors.New("missing tls.key")
				}
				crt, ok := secret.Data["tls.crt"]
				if !ok {
					log.Println("missing tls.crt")
					return errors.New("missing tls.crt")
				}
				var f *os.File
				if f, err = os.Create(HAProxyCERT); err != nil {
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
				port := int64(443)
				listener := &models.Listener{
					Address:        "0.0.0.0",
					Name:           "https",
					Port:           &port,
					Ssl:            "enabled",
					SslCertificate: HAProxyCERT,
				}
				switch secret.Status {
				case watch.Added:
					if err = nativeAPI.Configuration.CreateListener(frontendHTTP, listener, transaction.ID, 0); err != nil {
						return err
					}
				case watch.Modified:
					if err = nativeAPI.Configuration.EditListener(listener.Name, frontendHTTP, listener, transaction.ID, 0); err != nil {
						return err
					}
				case watch.Deleted:
					if err = nativeAPI.Configuration.DeleteListener(listener.Name, frontendHTTP, transaction.ID, 0); err != nil {
						return err
					}
				}

				//see if we need to add redirect to https redirect scheme https if !{ ssl_fc }
				// no need for error checking, we have default value
				sslRedirect, _ := GetValueFromAnnotations("ssl-redirect", c.cfg.ConfigMap.Annotations)
				switch sslRedirect.Status {
				case watch.Added:
				case watch.Modified:
				case watch.Deleted:
				case "":
				}
			}
		}
		//TODO, do not just go through them, sort them to handle /web,/ option maybe?
		for _, ingress := range namespace.Ingresses {
			//no need for switch/case for now
			backendsUsed := map[string]int{}
			for _, rule := range ingress.Rules {
				//nothing to switch/case for now
				for _, path := range rule.Paths {
					c.handlePath(frontendHTTP, namespace, ingress, rule, path, transaction, backendsUsed)
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

func (c *HAProxyController) handlePath(frontendHTTP string, namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath,
	transaction *models.Transaction, backendsUsed map[string]int) error {
	nativeAPI := c.NativeAPI
	//log.Println("PATH", path)
	backendName, selector, service, err := c.handleService(frontendHTTP, namespace, ingress, rule, path, backendsUsed, transaction)
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

func (c *HAProxyController) handleService(frontendHTTP string, namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath,
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
	balanceAlg, _ := GetValueFromAnnotations("load-balance", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	forwardedFor, _ := GetValueFromAnnotations("forwarded-for", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)

	switch service.Status {
	case watch.Added:
		if numberOfTimesBackendUsed := backendsUsed[backendName]; numberOfTimesBackendUsed < 2 {
			backend := &models.Backend{
				Balance:  balanceAlg.Value,
				Name:     backendName,
				Protocol: "http",
			}
			if forwardedFor.Value == "enabled" { //disabled with anything else is ok
				backend.HTTPXffHeaderInsert = "enabled"
			}
			if err := nativeAPI.Configuration.CreateBackend(backend, transaction.ID, 0); err != nil {
				log.Println("CreateBackend", err)
				return "", nil, nil, err
			}
		}
		//log.Println("use_backend", condTest)
		backendSwitchingRule := &models.BackendSwitchingRule{
			Cond:       "if",
			CondTest:   condTest,
			TargetFarm: backendName,
			ID:         1,
		}
		if err := nativeAPI.Configuration.CreateBackendSwitchingRule(frontendHTTP, backendSwitchingRule, transaction.ID, 0); err != nil {
			log.Println("CreateBackendSwitchingRule", err)
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

	if balanceAlg.Status != "" || forwardedFor.Status != "" {
		if err = c.handleBackendAnnotations(balanceAlg, forwardedFor, backendName, transaction); err != nil {
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

func (c *HAProxyController) handleBackendAnnotations(balanceAlg, forwardedFor *StringW,
	backendName string, transaction *models.Transaction) error {
	backend := &models.Backend{
		Balance:  balanceAlg.Value,
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

func (c *HAProxyController) checkHealthzStatus(transaction *models.Transaction) error {
	cfg := c.NativeAPI.Configuration
	frontendName := "healthz"
	if annHealthz, err := GetValueFromAnnotations("healthz", c.cfg.ConfigMap.Annotations); err == nil {
		enabled := annHealthz.Value == "enabled"
		enabledOld := annHealthz.OldValue == "enabled"
		port := int64(1042) //only default if user inputs invalid data

		annHealthzPort, _ := GetValueFromAnnotations("healthz-port", c.cfg.ConfigMap.Annotations)
		if annPort, err := strconv.ParseInt(annHealthzPort.Value, 10, 64); err == nil {
			port = annPort
		}
		listener := &models.Listener{
			Address: "0.0.0.0",
			Name:    "health-bind-name",
			Port:    &port,
		}
		switch annHealthz.Status {
		case watch.Added:
			if !enabled {
				return nil
			}
			if err := cfg.CreateListener(frontendName, listener, transaction.ID, 0); err != nil {
				return err
			}
		case watch.Modified:
			if enabled {
				if !enabledOld {
					if err := cfg.CreateListener(frontendName, listener, transaction.ID, 0); err != nil {
						return err
					}
				} else {
					if err := cfg.EditListener(listener.Name, frontendName, listener, transaction.ID, 0); err != nil {
						return err
					}
				}
			} else {
				if enabledOld {
					if err := cfg.DeleteListener(listener.Name, frontendName, transaction.ID, 0); err != nil {
						return err
					}
					lst, err := cfg.GetListener(listener.Name, frontendName, transaction.ID)
					log.Println(lst, err)
				}
			}
		case watch.Deleted:
			if err := cfg.DeleteListener(listener.Name, frontendName, transaction.ID, 0); err != nil {
				return err
			}
		case "":
			if annHealthzPort.Status != "" {
				if err := cfg.EditListener(listener.Name, frontendName, listener, transaction.ID, 0); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
