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

package controller

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	clientnative "github.com/haproxytech/client-native"
	"github.com/haproxytech/client-native/configuration"
	"github.com/haproxytech/client-native/runtime"
	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
	"k8s.io/apimachinery/pkg/watch"
)

// HAProxyController is ingress controller
type HAProxyController struct {
	k8s                         *K8s
	cfg                         Configuration
	osArgs                      utils.OSArgs
	NativeAPI                   *clientnative.HAProxyClient
	ActiveTransaction           string
	ActiveTransactionHasChanges bool
	eventChan                   chan SyncDataEvent
	serverlessPods              map[string]int
}

// Start initialize and run HAProxyController
func (c *HAProxyController) Start(ctx context.Context, osArgs utils.OSArgs) {

	c.osArgs = osArgs

	c.HAProxyInitialize()

	var k8s *K8s
	var err error

	if osArgs.OutOfCluster {
		kubeconfig := filepath.Join(utils.HomeDir(), ".kube", "config")
		if osArgs.KubeConfig != "" {
			kubeconfig = osArgs.KubeConfig
		}
		k8s, err = GetRemoteKubernetesClient(kubeconfig)
	} else {
		k8s, err = GetKubernetesClient()
	}
	if err != nil {
		utils.PanicErr(err)
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
	<-ctx.Done()
}

//HAProxyInitialize runs HAProxy for the first time so native client can have access to it
func (c *HAProxyController) HAProxyInitialize() {
	//cmd := exec.Command("haproxy", "-f", HAProxyCFG)
	err := os.MkdirAll(HAProxyCertDir, 0755)
	if err != nil {
		utils.PanicErr(err)
	}
	err = os.MkdirAll(HAProxyStateDir, 0755)
	if err != nil {
		utils.PanicErr(err)
	}
	err = os.MkdirAll(HAProxyMapDir, 0755)
	if err != nil {
		utils.PanicErr(err)
	}

	cmd := exec.Command("sh", "-c", "haproxy -v")
	haproxyInfo, err := cmd.Output()
	if err == nil {
		log.Println("Running with ", strings.ReplaceAll(string(haproxyInfo), "\n", ""))
	} else {
		log.Println(err)
	}

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

	hostname, err := os.Hostname()
	utils.LogErr(err)
	log.Println("Running on", hostname)

	runtimeClient := runtime.Client{}
	err = runtimeClient.InitWithSockets(map[int]string{
		0: "/var/run/haproxy-runtime-api.sock",
	})
	if err != nil {
		utils.PanicErr(err)
	}

	confClient := configuration.Client{}
	err = confClient.Init(configuration.ClientParams{
		ConfigurationFile:      HAProxyCFG,
		PersistentTransactions: false,
		Haproxy:                "haproxy",
	})
	if err != nil {
		utils.PanicErr(err)
	}

	c.NativeAPI = &clientnative.HAProxyClient{
		Configuration: &confClient,
		Runtime:       &runtimeClient,
	}

	c.cfg.Init(c.osArgs, HAProxyMapDir)

}

func (c *HAProxyController) ActiveConfiguration() (*parser.Parser, error) {
	if c.ActiveTransaction == "" {
		return nil, fmt.Errorf("no active transaction")
	}
	return c.NativeAPI.Configuration.GetParser(c.ActiveTransaction)
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
	err := c.saveServerState()
	utils.LogErr(err)
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

func (c *HAProxyController) HAProxyRestart() error {
	err := c.saveServerState()
	utils.LogErr(err)
	if !c.osArgs.Test {
		cmd := exec.Command("service", "haproxy", "restart")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Start()
	} else {
		err = nil
		log.Println("HAProxy would be restarted now")
	}
	return err
}

func (c *HAProxyController) handlePath(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath) (needReload bool, err error) {
	needReload = false
	service, ok := namespace.Services[path.ServiceName]
	if !ok {
		log.Printf("service '%s' does not exist", path.ServiceName)
		return needReload, fmt.Errorf("service '%s' does not exist", path.ServiceName)
	}

	backendName, newBackend, reload, err := c.handleService(namespace, ingress, rule, path, service)
	needReload = needReload || reload
	if err != nil {
		return needReload, err
	}

	endpoints, ok := namespace.Endpoints[service.Name]
	if !ok {
		log.Printf("No Endpoints found for service '%s'", service.Name)
		return needReload, nil // not an end of world scenario, just log this
	}
	endpoints.BackendName = backendName
	if err := c.setTargetPort(path, service, endpoints); err != nil {
		return needReload, err
	}

	for _, ip := range *endpoints.Addresses {
		reload := c.handleEndpointIP(namespace, ingress, rule, path, service, backendName, newBackend, endpoints, ip)
		needReload = needReload || reload
	}
	return needReload, nil
}

// handleEndpointIP processes the IngressPath related endpoints and makes corresponding backend servers configuration in HAProxy
func (c *HAProxyController) handleEndpointIP(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath, service *Service, backendName string, newBackend bool, endpoints *Endpoints, ip *EndpointIP) (needReload bool) {
	needReload = false
	server := models.Server{
		Name:    ip.HAProxyName,
		Address: ip.IP,
		Port:    &path.TargetPort,
		Weight:  utils.PtrInt64(128),
	}
	if ip.Disabled {
		server.Maintenance = "enabled"
	}
	annotationsActive := c.handleServerAnnotations(ingress, service, &server)
	status := ip.Status
	if status == EMPTY {
		if newBackend {
			status = ADDED
		} else if annotationsActive {
			status = MODIFIED
		}
	}
	switch status {
	case ADDED:
		err := c.backendServerCreate(backendName, server)
		if err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				utils.LogErr(err)
				needReload = true
			}
		} else {
			needReload = true
		}
	case MODIFIED:
		err := c.backendServerEdit(backendName, server)
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") {
				err1 := c.backendServerCreate(backendName, server)
				utils.LogErr(err1)
				needReload = true
			} else {
				utils.LogErr(err)
			}
		}
		status := "ready"
		if ip.Disabled {
			status = "maint"
		}
		log.Printf("Modified: %s - %s - %v\n", backendName, ip.HAProxyName, status)
	case DELETED:
		err := c.backendServerDelete(backendName, server.Name)
		if err != nil && !strings.Contains(err.Error(), "does not exist") {
			utils.LogErr(err)
		}
		return true
	}
	return needReload
}

// handleService processes the service related to the IngressPath and makes corresponding backend configuration in HAProxy
func (c *HAProxyController) handleService(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath, service *Service) (backendName string, newBackend bool, needReload bool, err error) {

	// Get Backend status
	status := service.Status
	if status == EMPTY {
		status = path.Status
	}

	// If status DELETED
	// remove use_backend rule and leave.
	// Backend will be deleted when no more use_backend
	// rules are left for the backend in question.
	// This is done via c.refreshBackendSwitching
	if status == DELETED {
		key := fmt.Sprintf("R%s%s%s%s", namespace.Name, ingress.Name, rule.Host, path.Path)
		switch {
		case path.IsSSLPassthrough:
			c.deleteUseBackendRule(key, FrontendSSL)
		case path.IsDefaultBackend:
			log.Printf("Removing default_backend %s from ingress \n", service.Name)
			utils.LogErr(c.setDefaultBackend(""))
			needReload = true
		default:
			c.deleteUseBackendRule(key, FrontendHTTP, FrontendHTTPS)
		}
		return "", false, needReload, nil
	}

	// Set backendName
	if path.ServicePortInt == 0 {
		backendName = fmt.Sprintf("%s-%s-%s", namespace.Name, service.Name, path.ServicePortString)
	} else {
		backendName = fmt.Sprintf("%s-%s-%d", namespace.Name, service.Name, path.ServicePortInt)
	}

	// Get/Create Backend
	newBackend = false
	needReload = false
	var backend models.Backend
	if backend, err = c.backendGet(backendName); err != nil {
		mode := "http"
		backend = models.Backend{
			Name: backendName,
			Mode: mode,
		}
		if path.IsTCPService || path.IsSSLPassthrough {
			backend.Mode = string(ModeTCP)
		}
		if err = c.backendCreate(backend); err != nil {
			return "", true, needReload, err
		}
		newBackend = true
		needReload = true
	}

	// handle Annotations
	activeSSLPassthrough := c.handleSSLPassthrough(ingress, service, path, &backend, newBackend)
	activeBackendAnn := c.handleBackendAnnotations(ingress, service, &backend, newBackend)
	if activeBackendAnn || activeSSLPassthrough {
		if err = c.backendEdit(backend); err != nil {
			return backendName, newBackend, needReload, err
		}
		needReload = true
	}

	// No need to update BackendSwitching
	if (status == EMPTY && !activeSSLPassthrough) || path.IsTCPService {
		return backendName, newBackend, needReload, nil
	}

	// Update backendSwitching
	key := fmt.Sprintf("R%s%s%s%s", namespace.Name, ingress.Name, rule.Host, path.Path)
	useBackendRule := UseBackendRule{
		Host:      rule.Host,
		Path:      path.Path,
		Backend:   backendName,
		Namespace: namespace.Name,
	}
	switch {
	case path.IsDefaultBackend:
		log.Printf("Confiugring default_backend %s from ingress %s\n", service.Name, ingress.Name)
		utils.LogErr(c.setDefaultBackend(backendName))
		needReload = true
	case path.IsSSLPassthrough:
		c.addUseBackendRule(key, useBackendRule, FrontendSSL)
		if activeSSLPassthrough {
			c.deleteUseBackendRule(key, FrontendHTTP, FrontendHTTPS)
		}
	default:
		c.addUseBackendRule(key, useBackendRule, FrontendHTTP, FrontendHTTPS)
		if activeSSLPassthrough {
			c.deleteUseBackendRule(key, FrontendSSL)
		}
	}

	if err != nil {
		return "", newBackend, needReload, err
	}

	return backendName, newBackend, needReload, nil
}

// Looks for the targetPort (Endpoint port) corresponding to the servicePort of the IngressPath
func (c *HAProxyController) setTargetPort(path *IngressPath, service *Service, endpoints *Endpoints) error {
	for _, sp := range service.Ports {
		// Find corresponding servicePort
		if sp.Name == path.ServicePortString || sp.Port == path.ServicePortInt {
			// Find the corresponding targetPort in Endpoints ports
			if endpoints != nil {
				for _, epPort := range *endpoints.Ports {
					if epPort.Name == sp.Name {
						// Dinamically update backend port
						if path.TargetPort != epPort.Port && path.TargetPort != 0 {
							for _, EndpointIP := range *endpoints.Addresses {
								if err := c.NativeAPI.Runtime.SetServerAddr(endpoints.BackendName, EndpointIP.HAProxyName, EndpointIP.IP, int(epPort.Port)); err != nil {
									log.Println(err)
								}
								log.Printf("TargetPort for backend %s changed to %d", endpoints.BackendName, epPort.Port)
							}
						}
						path.TargetPort = epPort.Port
						return nil
					}
				}
				log.Printf("Could not find Targetport of '%s' for service %s", sp.Name, service.Name)
			} // Return nil even if corresponding target port was not found.
			return nil
		}
	}
	return fmt.Errorf("servicePort(Str: %s, Int: %d) for serviceName '%s' not found", path.ServicePortString, path.ServicePortInt, service.Name)
}
