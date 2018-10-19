package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
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
	cfg       Configuration
	osArgs    OSArgs
	NativeAPI *clientnative.HAProxyClient
}

// Start initialize and run HAProxyController
func (c *HAProxyController) Start(osArgs OSArgs) {

	c.osArgs = osArgs

	go c.InitializeHAProxy()

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
func (c *HAProxyController) InitializeHAProxy() {
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
			eventChan <- SyncDataEvent{SyncType: SERVICE, Namespace: obj.GetNamespace(), Data: svc}
		case msg := <-pods.ResultChan():
			obj := msg.Object.(*corev1.Pod)
			//LogWatchEvent(msg.Type, POD, obj)
			pod := &Pod{
				Name:     obj.GetName(),
				Labels:   ConvertToMapStringW(obj.Labels),
				IP:       obj.Status.PodIP,
				PodPhase: obj.Status.Phase,
				//Port:      obj.Status. ? yes no, check
				Status: msg.Type,
			}
			eventChan <- SyncDataEvent{SyncType: POD, Namespace: obj.GetNamespace(), Data: pod}
		case msg := <-ingresses.ResultChan():
			obj := msg.Object.(*extensionsv1beta1.Ingress)
			ingress := &Ingress{
				Name:        obj.GetName(),
				Annotations: ConvertToMapStringW(obj.ObjectMeta.Annotations),
				Rules:       ConvertIngressRules(obj.Spec.Rules),
				Status:      msg.Type,
			}
			eventChan <- SyncDataEvent{SyncType: INGRESS, Namespace: obj.GetNamespace(), Data: ingress}
		case msg := <-configMapWatch.ResultChan():
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
			eventChan <- SyncDataEvent{SyncType: COMMAND}
		}
	}
}

//SyncData gets all kubernetes changes, aggregates them and apply to HAProxy.
//All the changes must come through this function
//TODO this is not necessary, remove it later
func (c *HAProxyController) SyncData(jobChan <-chan SyncDataEvent) {
	hadChanges := false
	c.cfg.Init(c.NativeAPI)
	for job := range jobChan {
		ns := c.cfg.GetNamespace(job.Namespace)
		switch job.SyncType {
		case COMMAND:
			if hadChanges {
				log.Println("job processing", job.SyncType)
				c.updateHAProxy()
				hadChanges = false
			}
		case NAMESPACE:
			hadChanges = c.eventNamespace(ns, job.Data.(*Namespace))
		case INGRESS:
			hadChanges = c.eventIngress(ns, job.Data.(*Ingress))
		case SERVICE:
			hadChanges = c.eventService(ns, job.Data.(*Service))
		case POD:
			hadChanges = c.eventPod(ns, job.Data.(*Pod))
		case CONFIGMAP:
			hadChanges = c.eventConfigMap(ns, job.Data.(*ConfigMap))
		case SECRET:
			hadChanges = c.eventSecret(ns, job.Data.(*Secret))
		}
	}
}

func (c *HAProxyController) eventNamespace(ns *Namespace, data *Namespace) bool {
	updateRequired := false
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
	return updateRequired
}

func (c *HAProxyController) eventIngress(ns *Namespace, data *Ingress) bool {
	updateRequired := false
	switch data.Status {
	case watch.Modified:
		newIngress := data
		oldIngress, ok := ns.Ingresses[data.Name]
		newIngress.Annotations.SetStatus(oldIngress.Annotations)
		if !ok {
			//intentionally do not add it. TODO see if idea of only watching is ok
			log.Println("Ingress not registered with controller, cannot modify !", data.Name)
			return false
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
		diff := cmp.Equal(oldIngress, newIngress)
		log.Println(diff)
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
	return updateRequired
}

func (c *HAProxyController) eventService(ns *Namespace, data *Service) bool {
	updateRequired := false
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
	return updateRequired
}

func (c *HAProxyController) eventPod(ns *Namespace, data *Pod) bool {
	updateRequired := false
	switch data.Status {
	case watch.Modified:
		newPod := data
		var oldPod *Pod
		oldPod, ok := ns.Pods[data.Name]
		if !ok {
			//intentionally do not add it. TODO see if our idea of only watching is ok
			log.Println("Pod not registered with controller, cannot modify !", data.Name)
			return updateRequired
		}
		newPod.HAProxyName = oldPod.HAProxyName
		if oldPod.Status == watch.Added {
			newPod.Status = watch.Added
		}
		ns.Pods[data.Name] = newPod
		//result := cmp.Diff(oldPod, newPod)
		//log.Println("Pod modified", data.Name, oldPod.Status, "\n", newPod.HAProxyName, oldPod.HAProxyName, "/n", result)
		updateRequired = true
	case watch.Added:
		//first see if we have spare place in servers
		//INFO if same pod used in multiple services, this will not work
		createNew := true
		if service, err := ns.GetServiceForPod(data.Labels); err == nil {
			pods := ns.GetPodsForSelector(service.Selector)
			//now see if we have some free place where we can place pod
			for _, pod := range pods {
				if pod.Maintenance {
					log.Println("found pod in maintenace mode", pod.Name, pod.HAProxyName, service.Name, hasSelectors(service.Selector, pod.Labels), service.Selector, pod.Labels)
					createNew = false
					data.Maintenance = false
					data.Status = watch.Modified
					data.HAProxyName = pod.HAProxyName
					ns.Pods[data.Name] = data
					delete(ns.Pods, pod.Name)
					updateRequired = true
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
		}
	case watch.Deleted:
		oldPod, ok := ns.Pods[data.Name]
		if ok {
			oldPod.IP = "127.0.0.1"
			oldPod.Status = watch.Modified //we replace it with disabled one
			oldPod.Maintenance = true
			//delete(ns.Pods, data.Name)
			oldPod, _ = ns.Pods[data.Name]
			//log.Println("Pod set for deletion", oldPod)
			//update immediately
			updateRequired = true
		} else {
			log.Println("Pod not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (c *HAProxyController) eventConfigMap(ns *Namespace, data *ConfigMap) bool {
	updateRequired := false
	if ns.Name != c.osArgs.ConfigMap.Namespace ||
		data.Name != c.osArgs.ConfigMap.Name {
		return updateRequired
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
		c.cfg.ConfigMap = data
		updateRequired = true
	case watch.Deleted:
		c.cfg.ConfigMap.Annotations.SetStatusState(watch.Deleted)
		c.cfg.ConfigMap.Status = watch.Deleted
	}
	return updateRequired
}
func (c *HAProxyController) eventSecret(ns *Namespace, data *Secret) bool {
	updateRequired := false
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
	return updateRequired
}

func (c *HAProxyController) updateHAProxy() error {
	nativeAPI := c.NativeAPI
	//backend, err := nativeAPI.Configuration.GetBackend("default-http-svc-8080")
	//log.Println(backend, err)
	version, err := nativeAPI.Configuration.GetVersion()
	if err != nil || version < 1 {
		//silently fallback to 1
		version = 1
	}
	log.Println("Config version:", version)
	transaction, err := nativeAPI.Configuration.StartTransaction(version)
	if err != nil {
		log.Println(err)
		return err
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
					c.handlePath(frontendHTTP, namespace, rule, path, transaction, backendsUsed)
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
	log.Println("CommitTransaction ...")
	err = nativeAPI.Configuration.CommitTransaction(transaction.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println("Transaction successfull")
	c.cfg.Clean()

	log.Println("updateHAProxy ended")
	return nil
}

func (c *HAProxyController) handlePath(frontendHTTP string, namespace *Namespace, rule *IngressRule, path *IngressPath,
	transaction *models.Transaction, backendsUsed map[string]int) {
	nativeAPI := c.NativeAPI
	//log.Println("PATH", path)
	backendName, selector, err := c.handleService(frontendHTTP, namespace, rule, path, backendsUsed, transaction)
	if err != nil {
		return
	}

	if numberOfTimesBackendUsed := backendsUsed[backendName]; numberOfTimesBackendUsed > 1 {
		return //we have already went through pods
	}
	for _, pod := range namespace.Pods {
		if hasSelectors(selector, pod.Labels) {
			port := int64(path.ServicePort)
			weight := int64(1)
			data := &models.Server{
				Name:    pod.HAProxyName,
				Address: pod.IP,
				Check:   "enabled",
				Port:    &port,
				Weight:  &weight,
			}
			if pod.Maintenance {
				data.Maintenance = "enabled"
			}
			/*if pod.Sorry != "" {
				data.Sorry = pod.Sorry
			}*/
			switch pod.Status {
			case watch.Added:
				if err := nativeAPI.Configuration.CreateServer(backendName, data, transaction.ID, 0); err != nil {
					log.Println("CreateServer", err)
				}
			case watch.Modified:
				if err := nativeAPI.Configuration.EditServer(data.Name, backendName, data, transaction.ID, 0); err != nil {
					log.Println("EditServer", err)
				}
			case watch.Deleted:
				log.Printf("Unsupported state for POD [%s]", pod.Name)
			}
		} //if pod.Status...
	} //for pod
}

func (c *HAProxyController) handleService(frontendHTTP string, namespace *Namespace, rule *IngressRule, path *IngressPath,
	backendsUsed map[string]int, transaction *models.Transaction) (backendName string, selector MapStringW, err error) {
	nativeAPI := c.NativeAPI

	service, ok := namespace.Services[path.ServiceName]
	if !ok {
		log.Println("service", path.ServiceName, "does not exists")
		return "", nil, nil
	}
	selector = service.Selector
	if len(selector) == 0 {
		log.Println("service", service.Name, "no selector")
		return "", nil, nil
	}

	backendName = fmt.Sprintf("%s-%s-%d", namespace.Name, service.Name, path.ServicePort)
	backendsUsed[backendName]++
	condTest := "{ req.hdr(host) -i " + rule.Host + " } { var(txn.path) -m beg " + path.Path + " } "
	balanceAlg, _ := GetValueFromAnnotations("load-balance", service.Annotations, c.cfg.ConfigMap.Annotations)

	switch service.Status {
	case watch.Added:
		if numberOfTimesBackendUsed := backendsUsed[backendName]; numberOfTimesBackendUsed < 2 {
			backend := &models.Backend{
				Balance:  balanceAlg.Value,
				Name:     backendName,
				Protocol: "http",
			}
			if err := nativeAPI.Configuration.CreateBackend(backend, transaction.ID, 0); err != nil {
				log.Println("CreateBackend", err)
				return "", nil, err
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
			return "", nil, err
		}
	//case watch.Modified:
	//nothing to do for now
	case watch.Deleted:
		backendsUsed[backendName]--
		if err := nativeAPI.Configuration.DeleteBackend(backendName, transaction.ID, 0); err != nil {
			log.Println("DeleteBackend", err)
			return "", nil, err
		}
		return "", nil, nil
	}

	log.Println(backendName, "load-balance", balanceAlg)
	if balanceAlg.Status != "" {
		// don't worry about deleted state, load-balance has default value
		backend := &models.Backend{
			Balance:  balanceAlg.Value,
			Name:     backendName,
			Protocol: "http",
		}
		if err := nativeAPI.Configuration.EditBackend(backend.Name, backend, transaction.ID, 0); err != nil {
			log.Println("EditBackend", err)
			return "", nil, err
		}
	}
	return backendName, selector, nil
}
