// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	clientnative "github.com/haproxytech/client-native"
	"github.com/haproxytech/client-native/configuration"
	"github.com/haproxytech/client-native/misc"
	"github.com/haproxytech/client-native/runtime"
	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/models"
	"k8s.io/apimachinery/pkg/watch"
)

// HAProxyController is ingress controller
type HAProxyController struct {
	k8s                         *K8s
	cfg                         Configuration
	osArgs                      OSArgs
	NativeAPI                   *clientnative.HAProxyClient
	NativeParser                parser.Parser
	ActiveTransaction           string
	ActiveTransactionHasChanges bool
	UseHTTPS                    BoolW
	eventChan                   chan SyncDataEvent
	serverlessPods              map[string]int
}

// Start initialize and run HAProxyController
func (c *HAProxyController) Start(osArgs OSArgs) {

	c.osArgs = osArgs

	c.HAProxyInitialize()

	var k8s *K8s
	var err error

	if osArgs.OutOfCluster {
		k8s, err = GetRemoteKubernetesClient(osArgs)
	} else {
		k8s, err = GetKubernetesClient()
	}
	if err != nil {
		log.Panic(err)
	}
	c.k8s = k8s

	x := k8s.API.Discovery()
	if k8sVersion, err := x.ServerVersion(); err != nil {
		log.Fatalf("Unable to get Kubernetes version: %v\n", err)
	} else {
		log.Printf("Running on Kubernetes version: %s %s", k8sVersion.String(), k8sVersion.Platform)
	}

	c.serverlessPods = map[string]int{}
	c.eventChan = make(chan SyncDataEvent, watch.DefaultChanSize*6)
	go c.monitorChanges()
}

//HAProxyInitialize runs HAProxy for the first time so native client can have access to it
func (c *HAProxyController) HAProxyInitialize() {
	//cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	err := os.MkdirAll(HAProxyCertDir, 0755)
	if err != nil {
		log.Panic(err.Error())
	}
	err = os.MkdirAll(HAProxyStateDir, 0755)
	if err != nil {
		log.Panic(err.Error())
	}

	cmd := exec.Command("sh", "-c", "haproxy -v")
	haproxyInfo, err := cmd.Output()
	log.Println("Running with ", string(haproxyInfo))

	log.Println("Starting HAProxy with", HAProxyCFG)
	if !c.osArgs.Test {
		cmd := exec.Command("service", "haproxy", "start")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Start()
		if err != nil {
			log.Println(err)
		}
	}

	c.NativeParser = parser.Parser{}
	err = c.NativeParser.LoadData(HAProxyGlobalCFG)
	if err != nil {
		log.Panic(err)
	}

	hostname, err := os.Hostname()
	LogErr(err)
	log.Println("Running on", hostname)

	runtimeClient := runtime.Client{}
	err = runtimeClient.Init([]string{"/var/run/haproxy-runtime-api.sock"}, "", 0)
	if err != nil {
		log.Panicln(err)
	}

	confClient := configuration.Client{}
	err = confClient.Init(configuration.ClientParams{
		ConfigurationFile:      HAProxyCFG,
		PersistentTransactions: false,
		Haproxy:                "haproxy",
	})
	if err != nil {
		log.Panicln(err)
	}

	c.NativeAPI = &clientnative.HAProxyClient{
		Configuration: &confClient,
		Runtime:       &runtimeClient,
	}

	err = c.apiStartTransaction()
	LogErr(err)
	defer c.apiDisposeTransaction()
	c.initHTTPS()
	err = c.apiCommitTransaction()
	LogErr(err)
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
	if !c.osArgs.Test {
		cmd := exec.Command("service", "haproxy", "reload")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Start()
	} else {
		err = nil
		log.Println("HAProxy would be reloaded now")
	}
	return err
}

func (c *HAProxyController) handlePath(index int, namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath) (needsReload bool, err error) {
	needsReload = false
	backendName, service, reload, err := c.handleService(index, namespace, ingress, rule, path)
	needsReload = needsReload || reload
	if err != nil {
		return needsReload, err
	}

	endpoints, ok := namespace.Endpoints[service.Name]
	if !ok {
		log.Printf("Endpoint for service %s does not exists", service.Name)
		return needsReload, nil // not an end of world scenario, just log this
	}
	endpoints.BackendName = backendName

	for _, ip := range *endpoints.Addresses {
		reload := c.handleEndpointIP(namespace, ingress, rule, path, service, backendName, endpoints, ip)
		needsReload = needsReload || reload
	}
	return needsReload, nil
}

func (c *HAProxyController) handleEndpointIP(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath, service *Service, backendName string, endpoints *Endpoints, ip *EndpointIP) (needsReload bool) {
	needsReload = false
	annMaxconn, errMaxConn := GetValueFromAnnotations("pod-maxconn", service.Annotations)
	annCheck, _ := GetValueFromAnnotations("check", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annCheckInterval, errCheckInterval := GetValueFromAnnotations("check-interval", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)

	status := ip.Status
	port := path.ServicePortInt
	if port == 0 {
		portName := path.ServicePortString
		for _, p := range *endpoints.Ports {
			if p.Name == portName {
				port = p.TargetPort
				break
			}
		}
	}
	if port == 0 {
		for _, p := range *endpoints.Ports {
			port = p.TargetPort
		}
	}
	weight := int64(128)
	data := models.Server{
		Name:    ip.HAProxyName,
		Address: ip.IP,
		Port:    &port,
		Weight:  &weight,
	}
	if ip.Disabled {
		data.Maintenance = "enabled"
	}
	annnotationsActive := false
	if annMaxconn != nil {
		if annMaxconn.Status != DELETED && errMaxConn == nil {
			if maxconn, err := strconv.ParseInt(annMaxconn.Value, 10, 64); err == nil {
				data.Maxconn = &maxconn
			}
			if annMaxconn.Status != "" {
				annnotationsActive = true
			}
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
	if errCheckInterval == nil {
		data.Inter = misc.ParseTimeout(annCheckInterval.Value)
		if annCheckInterval.Status != EMPTY {
			annnotationsActive = true
		}
	} else {
		data.Inter = nil
	}
	if ip.Status == EMPTY && annnotationsActive {
		status = MODIFIED
	}
	if status == EMPTY && path.Status != ADDED && path.Status != EMPTY {
		status = ADDED
	}
	if status == EMPTY && service.Status != ADDED && service.Status != EMPTY {
		status = ADDED
	}
	if status == EMPTY && rule != nil && rule.Status == ADDED {
		status = ADDED
	}
	switch status {
	case ADDED:
		err := c.backendServerCreate(backendName, data)
		if err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				LogErr(err)
				needsReload = true
			}
		} else {
			needsReload = true
		}
	case MODIFIED:
		err := c.backendServerEdit(backendName, data)
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") {
				err1 := c.backendServerCreate(backendName, data)
				LogErr(err1)
				needsReload = true
			} else {
				LogErr(err)
			}
		}
		status := "ready"
		if ip.Disabled {
			status = "maint"
		}
		log.Printf("Modified: %s - %s - %v\n", backendName, ip.HAProxyName, status)
	case DELETED:
		err := c.backendServerDelete(backendName, data.Name)
		if err != nil && !strings.Contains(err.Error(), "does not exist") {
			LogErr(err)
		}
		needsReload = true
	}
	return needsReload
}

func (c *HAProxyController) handleService(index int, namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath) (backendName string, service *Service, needReload bool, err error) {
	needReload = false

	service, ok := namespace.Services[path.ServiceName]
	if !ok {
		log.Println("service", path.ServiceName, "does not exists")
		return "", nil, needReload, fmt.Errorf("service %s does not exists", path.ServiceName)
	}

	if path.ServicePortInt == 0 {
		for _, p := range service.Ports {
			if p.Name == path.ServicePortString || path.ServicePortString == "" {
				path.ServicePortInt = p.TargetPort
				break
			}
		}
	} else {
		//check if user defined service port and not target port
		for _, servicePort := range service.Ports {
			if path.ServicePortInt == servicePort.ServicePort {
				path.ServicePortInt = servicePort.TargetPort
				break
			}
		}
	}

	backendName = fmt.Sprintf("%s-%s-%d", namespace.Name, service.Name, path.ServicePortInt)
	//Annotations with default values don't need error checking.
	annWhitelist, _ := GetValueFromAnnotations("whitelist", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annWhitelistRL, _ := GetValueFromAnnotations("whitelist-with-rate-limit", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	allowRateLimiting := annWhitelistRL.Value != "" && annWhitelistRL.Value != "OFF"
	status := annWhitelist.Status
	if status == EMPTY {
		if annWhitelistRL.Status != EMPTY {
			data, ok := c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", index)]
			if ok && len(data) > 0 {
				status = MODIFIED
			}
		}
		if annWhitelistRL.Value != "" && path.Status == ADDED {
			status = MODIFIED
		}
	}
	switch status {
	case ADDED, MODIFIED:
		if annWhitelist.Value != "" {
			ID := int64(0)
			httpRequest1 := &models.HTTPRequestRule{
				ID:       &ID,
				Type:     "allow",
				Cond:     "if",
				CondTest: fmt.Sprintf("{ path_beg %s } { src %s }", path.Path, strings.Replace(annWhitelist.Value, ",", " ", -1)),
			}
			httpRequest2 := &models.HTTPRequestRule{
				ID:       &ID,
				Type:     "deny",
				Cond:     "if",
				CondTest: fmt.Sprintf("{ path_beg %s }", path.Path),
			}
			if allowRateLimiting {
				c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", index)] = []models.HTTPRequestRule{
					*httpRequest1,
				}
			} else {
				c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", index)] = []models.HTTPRequestRule{
					*httpRequest2, //reverse order
					*httpRequest1,
				}
			}
		} else {
			c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", index)] = []models.HTTPRequestRule{}
		}
		c.cfg.HTTPRequestsStatus = MODIFIED
	case DELETED:
		c.cfg.HTTPRequests[fmt.Sprintf("WHT-%0006d", index)] = []models.HTTPRequestRule{}
	}

	// Update usebackend rules
	if rule != nil {
		key := fmt.Sprintf("R%s%s%s%0006d", namespace.Name, ingress.Name, rule.Host, index)
		old, ok := c.cfg.UseBackendRules[key]
		if ok {
			if old.Backend != backendName || old.Host != rule.Host || old.Path != path.Path {
				c.cfg.UseBackendRules[key] = BackendSwitchingRule{
					Host:      rule.Host,
					Path:      path.Path,
					Backend:   backendName,
					Namespace: namespace.Name,
				}
				c.cfg.UseBackendRulesStatus = MODIFIED
			}
		} else {
			c.cfg.UseBackendRules[key] = BackendSwitchingRule{
				Host:      rule.Host,
				Path:      path.Path,
				Backend:   backendName,
				Namespace: namespace.Name,
			}
			c.cfg.UseBackendRulesStatus = MODIFIED
		}
	} else if service.Status != EMPTY {
		http, err := c.frontendGet(FrontendHTTP)
		LogErr(err)
		http.DefaultBackend = backendName
		err = c.frontendEdit(http)
		LogErr(err)
		https, err := c.frontendGet(FrontendHTTPS)
		LogErr(err)
		https.DefaultBackend = backendName
		err = c.frontendEdit(https)
		LogErr(err)
		needReload = true
	}

	status = service.Status
	newImportantPath := false
	if status == EMPTY && path.Status == ADDED {
		status = ADDED
		newImportantPath = true
	}
	if status == EMPTY && rule != nil && rule.Status == ADDED {
		status = ADDED
		//in this case nothing is new except rule,
		//we need also to populate
		//newly created backend with servers
	}

	// Backend creation/deletion
	newBackend := false
	switch status {
	case ADDED, MODIFIED:
		_, err := c.backendGet(backendName)
		if err != nil {
			backend := models.Backend{
				Name: backendName,
				Mode: "http",
			}
			if err := c.backendCreate(backend); err != nil {
				msg := err.Error()
				if !strings.Contains(msg, "Farm already exists") {
					if !newImportantPath {
						return "", nil, needReload, err
					}
				}
			} else {
				newBackend = true
			}
			needReload = true
		}
	case DELETED:
		delete(c.cfg.UseBackendRules, fmt.Sprintf("R%0006d", index))
		c.cfg.UseBackendRulesStatus = MODIFIED
		return "", service, needReload, nil
	}

	reload, err := c.handleBackendAnnotations(ingress, service, backendName, newBackend)
	needReload = needReload || reload
	if err != nil {
		return "", nil, needReload, err
	}

	return backendName, service, needReload, nil
}
