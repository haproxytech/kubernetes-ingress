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
	"strings"

	clientnative "github.com/haproxytech/client-native"
	"github.com/haproxytech/client-native/configuration"
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

	c.cfg.Init(c.osArgs, c.NativeAPI)

	err = c.apiStartTransaction()
	PanicErr(err)
	defer c.apiDisposeTransaction()
	c.initHTTPS()

	err = c.apiCommitTransaction()
	PanicErr(err)
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

func (c *HAProxyController) handlePath(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath) (needReload bool, err error) {
	needReload = false
	service, ok := namespace.Services[path.ServiceName]
	if !ok {
		log.Println("service", path.ServiceName, "does not exists")
		return needReload, fmt.Errorf("service %s does not exists", path.ServiceName)
	}

	backendName, newBackend, reload, err := c.handleService(namespace, ingress, rule, path, service)
	needReload = needReload || reload
	if err != nil {
		return needReload, err
	}

	endpoints, ok := namespace.Endpoints[service.Name]
	if !ok {
		log.Printf("No Endpoints found for service %s ", service.Name)
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
	weight := int64(128)
	server := models.Server{
		Name:    ip.HAProxyName,
		Address: ip.IP,
		Port:    &path.TargetPort,
		Weight:  &weight,
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
				LogErr(err)
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
				LogErr(err1)
				needReload = true
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
		err := c.backendServerDelete(backendName, server.Name)
		if err != nil && !strings.Contains(err.Error(), "does not exist") {
			LogErr(err)
		}
		return true
	}
	return needReload
}

// handleService processes the IngressPath related service and makes corresponding backend configuration in HAProxy
func (c *HAProxyController) handleService(namespace *Namespace, ingress *Ingress, rule *IngressRule, path *IngressPath, service *Service) (backendName string, newBackend bool, needReload bool, err error) {
	needReload = false

	c.handleRateLimitingAnnotations(ingress, service, path)

	if path.ServicePortInt == 0 {
		backendName = fmt.Sprintf("%s-%s-%s", namespace.Name, service.Name, path.ServicePortString)
	} else {
		backendName = fmt.Sprintf("%s-%s-%d", namespace.Name, service.Name, path.ServicePortInt)
	}
	// Default backend
	if path.IsDefaultPath && service.Status != EMPTY {
		var http models.Frontend
		var https models.Frontend
		http, err = c.frontendGet(FrontendHTTP)
		LogErr(err)
		http.DefaultBackend = backendName
		err = c.frontendEdit(http)
		LogErr(err)
		https, err = c.frontendGet(FrontendHTTPS)
		LogErr(err)
		https.DefaultBackend = backendName
		err = c.frontendEdit(https)
		LogErr(err)
		needReload = true
	}
	if path.IsTCPPath {
		c.cfg.TCPBackends[backendName] = path.ServicePortInt
	}

	// get Backend status
	status := service.Status
	if status == EMPTY {
		if rule != nil && rule.Status == ADDED {
			//TODO: check why an ADEED rule.Status
			// is not reflected for the path.Status
			status = ADDED
		} else {
			status = path.Status
		}
	}

	// Backend creation
	newBackend = false
	switch status {
	case ADDED, MODIFIED:
		if _, err = c.backendGet(backendName); err != nil {
			mode := "http"
			backend := models.Backend{
				Name: backendName,
				Mode: mode,
			}
			if path.IsTCPPath {
				backend.Mode = string(ModeTCP)
			}
			if err = c.backendCreate(backend); err != nil {
				msg := err.Error()
				if !strings.Contains(msg, "Farm already exists") {
					return "", true, needReload, err
				}
			} else {
				newBackend = true
				needReload = true
			}
		}
		// Update usebackend rule
		if rule != nil && !path.IsTCPPath && !path.IsDefaultPath {
			key := fmt.Sprintf("R%s%s%s%0006d", namespace.Name, ingress.Name, rule.Host, path.Path)
			old, ok := c.cfg.UseBackendRules[key]
			if ok {
				// Check if the existing use_backend rule refers to the right backend
				if old.Backend != backendName {
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
		}
	case DELETED:
		// Backend will be cleaned up with useBackendRuleRefresh
		// when no more UseBackendRules points to it.
		return "", newBackend, needReload, nil
	}

	reload, err := c.handleBackendAnnotations(ingress, service, backendName, newBackend)
	needReload = needReload || reload
	if err != nil {
		return "", newBackend, needReload, err
	}

	return backendName, newBackend, needReload, nil
}

// Looks for the targetPort (Endpoint port) corresponding to the servicePort of the IngressPath
func (c *HAProxyController) setTargetPort(path *IngressPath, service *Service, endpoints *Endpoints) error {
	for _, sp := range service.Ports {
		// Find corresponding servicePort
		if sp.Name == path.ServicePortString || sp.ServicePort == path.ServicePortInt {
			// Find the corresponding targetPort in Endpoints ports
			if endpoints != nil {
				for _, epPort := range *endpoints.Ports {
					if epPort.Name == sp.Name {
						if path.TargetPort != epPort.Port {
							for _, EndpointIP := range *endpoints.Addresses {
								if err := c.cfg.NativeAPI.Runtime.SetServerAddr(endpoints.BackendName, EndpointIP.HAProxyName, EndpointIP.IP, int(epPort.Port)); err != nil {
									log.Println(err)
								}
							}
							if path.TargetPort != 0 {
								log.Printf("TargetPort for backend %s changed to %d", endpoints.BackendName, epPort.Port)
							}
							path.TargetPort = epPort.Port
						}
						return nil
					}
				}
				log.Printf("Targetport %s not found for service %s", sp.TargetPortStr, service.Name)
			} // Return nil even if corresponding target port was not found.
			return nil
		}
	}
	return fmt.Errorf("servicePort(Str: %s, Int: %d) for serviceName %s not found", path.ServicePortString, path.ServicePortInt, service.Name)
}
