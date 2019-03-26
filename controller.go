package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

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

	go c.monitorChanges()
}

//HAProxyInitialize runs HAProxy for the first time so native client can have access to it
func (c *HAProxyController) HAProxyInitialize() {
	//cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	err := os.MkdirAll(HAProxyCertDir, 0644)
	if err != nil {
		log.Panic(err.Error())
	}
	err = os.MkdirAll(HAProxyStateDir, 0644)
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
		log.Panicln(err)
	}

	confClient := configuration.Client{}
	err = confClient.Init(configuration.ClientParams{
		ConfigurationFile: HAProxyCFG,
		//GlobalConfigurationFile: HAProxyGlobalCFG,
		Haproxy: "haproxy",
		//LBCTLPath:               "/usr/sbin/lbctl",
	})
	if err != nil {
		log.Panicln(err)
	}

	c.NativeAPI = &clientnative.HAProxyClient{
		Configuration: &confClient,
		Runtime:       &runtimeClient,
	}
}

func (c *HAProxyController) saveServerState() error {
	result, err := c.NativeAPI.Runtime.ExecuteRaw("show servers state")
	if err != nil {
		return err
	}
	var f *os.File
	if f, err = os.Create(HAProxyStateDir + "global"); err != nil {
		log.Println(err)
		return err
	}
	defer f.Close()
	if _, err = f.Write([]byte(result[0])); err != nil {
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

func (c *HAProxyController) HAProxyReload() error {
	err := c.NativeParser.Save(HAProxyGlobalCFG)
	if err != nil {
		return err
	}
	err = c.saveServerState()
	LogErr(err)
	//cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	cmd := exec.Command("service", "haproxy", "reload")
	err = cmd.Run()
	return err
}

func (c *HAProxyController) monitorChanges() {

	configMapReceivedAndProcessed := make(chan bool)
	syncEveryNSeconds := 5
	eventChan := make(chan SyncDataEvent, watch.DefaultChanSize*6)
	go c.SyncData(eventChan, configMapReceivedAndProcessed)

	stop := make(chan struct{})

	podChan := make(chan *Pod, 100)
	c.k8s.EventsPods(podChan, stop)

	svcChan := make(chan *Service, 100)
	c.k8s.EventsServices(svcChan, stop)

	nsChan := make(chan *Namespace, 10)
	c.k8s.EventsNamespaces(nsChan, stop)

	ingChan := make(chan *Ingress, 10)
	c.k8s.EventsIngresses(ingChan, stop)

	cfgChan := make(chan *ConfigMap, 10)
	c.k8s.EventsConfigfMaps(cfgChan, stop)

	secretChan := make(chan *Secret, 10)
	c.k8s.EventsSecrets(secretChan, stop)

	eventsIngress := []SyncDataEvent{}
	eventsServices := []SyncDataEvent{}
	eventsPods := []SyncDataEvent{}
	configMapOk := false

	for {
		select {
		case _ = <-configMapReceivedAndProcessed:
			for _, event := range eventsIngress {
				eventChan <- event
			}
			for _, event := range eventsServices {
				eventChan <- event
			}
			for _, event := range eventsPods {
				eventChan <- event
			}
			eventsIngress = []SyncDataEvent{}
			eventsServices = []SyncDataEvent{}
			eventsPods = []SyncDataEvent{}
			configMapOk = true
			time.Sleep(1 * time.Millisecond)
		case item := <-cfgChan:
			eventChan <- SyncDataEvent{SyncType: CONFIGMAP, Namespace: item.Namespace, Data: item}
		case item := <-nsChan:
			event := SyncDataEvent{SyncType: NAMESPACE, Namespace: item.Name, Data: item}
			eventChan <- event
		case item := <-podChan:
			event := SyncDataEvent{SyncType: POD, Namespace: item.Namespace, Data: item}
			if configMapOk {
				eventChan <- event
			} else {
				eventsPods = append(eventsPods, event)
			}
		case item := <-svcChan:
			event := SyncDataEvent{SyncType: SERVICE, Namespace: item.Namespace, Data: item}
			if configMapOk {
				eventChan <- event
			} else {
				eventsServices = append(eventsServices, event)
			}
		case item := <-ingChan:
			event := SyncDataEvent{SyncType: INGRESS, Namespace: item.Namespace, Data: item}
			if configMapOk {
				eventChan <- event
			} else {
				eventsIngress = append(eventsIngress, event)
			}
		case item := <-secretChan:
			event := SyncDataEvent{SyncType: SECRET, Namespace: item.Namespace, Data: item}
			eventChan <- event
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
func (c *HAProxyController) SyncData(jobChan <-chan SyncDataEvent, chConfigMapReceivedAndProcessed chan bool) {
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
			change, reload = c.eventConfigMap(ns, job.Data.(*ConfigMap), chConfigMapReceivedAndProcessed)
		case SECRET:
			change, reload = c.eventSecret(ns, job.Data.(*Secret))
		}
		hadChanges = hadChanges || change
		needsReload = needsReload || reload
	}
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
				if annMaxconn.Status != DELETED {
					if maxconn, err := strconv.ParseInt(annMaxconn.Value, 10, 64); err == nil {
						data.Maxconn = &maxconn
					}
				}
				if annMaxconn.Status != "" {
					annnotationsActive = true
				}
			}
			if annCheck != nil {
				if annCheck.Status != DELETED {
					if annCheck.Value == "enabled" {
						data.Check = "enabled"
						//see if we have port and interval defined
					}
				}
				if annCheck.Status != "" {
					annnotationsActive = true
				}
			}
			if pod.Status == EMPTY && annnotationsActive {
				pod.Status = MODIFIED
			}
			switch pod.Status {
			case ADDED:
				if err := nativeAPI.Configuration.CreateServer(backendName, data, transaction.ID, 0); err != nil {
					return err
				}
			case MODIFIED:
				if err := nativeAPI.Configuration.EditServer(data.Name, backendName, data, transaction.ID, 0); err != nil {
					return err
				}
			case DELETED:
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
	//TODO BackendBalance proper usage
	balanceAlg := &models.BackendBalance{
		Algorithm: annBalanceAlg.Value,
	}
	if err != nil {
		log.Printf("%s, using %s \n", err, balanceAlg)
	}

	switch service.Status {
	case ADDED:
		if numberOfTimesBackendUsed := backendsUsed[backendName]; numberOfTimesBackendUsed < 2 {
			backend := &models.Backend{
				Balance: balanceAlg,
				Name:    backendName,
				Mode:    "http",
			}
			if annForwardedFor.Value == "enabled" { //disabled with anything else is ok
				forwardfor := "enabled"
				backend.Forwardfor = &models.BackendForwardfor{
					Enabled: &forwardfor,
				}
			}
			if err := nativeAPI.Configuration.CreateBackend(backend, transaction.ID, 0); err != nil {
				msg := err.Error()
				if !strings.Contains(msg, "Farm already exists") {
					return "", nil, nil, err
				}
			}
		}
		id := int64(0)
		backendSwitchingRule := &models.BackendSwitchingRule{
			Cond:     "if",
			CondTest: condTest,
			Name:     backendName,
			ID:       &id,
		}
		if err := nativeAPI.Configuration.CreateBackendSwitchingRule(FrontendHTTP, backendSwitchingRule, transaction.ID, 0); err != nil {
			log.Println("CreateBackendSwitchingRule http", err)
			return "", nil, nil, err
		}
		if err := nativeAPI.Configuration.CreateBackendSwitchingRule(FrontendHTTPS, backendSwitchingRule, transaction.ID, 0); err != nil {
			log.Println("CreateBackendSwitchingRule https", err)
			return "", nil, nil, err
		}
	//case MODIFIED:
	//nothing to do for now
	case DELETED:
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
