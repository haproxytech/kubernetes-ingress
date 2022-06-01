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
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	//nolint:gosec
	_ "net/http/pprof"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/jessevdk/go-flags"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/controller"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

//go:embed fs/usr/local/etc/haproxy/haproxy.cfg
var haproxyConf []byte

func main() {
	exitCode := 0
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Error : ", r)
		}
		os.Exit(exitCode)
	}()

	// Parse Controller Args
	var osArgs utils.OSArgs
	var err error
	parser := flags.NewParser(&osArgs, flags.IgnoreUnknown)
	if _, err = parser.Parse(); err != nil {
		fmt.Println(err)
		exitCode = 1
		return
	}

	// Set Logger
	logger := utils.GetLogger()
	logger.SetLevel(osArgs.LogLevel.LogLevel)
	if len(osArgs.Help) > 0 && osArgs.Help[0] {
		parser.WriteHelp(os.Stdout)
		return
	}
	logger.ShowFilename(false)
	logInfo(logger, osArgs)
	logger.ShowFilename(true)

	// backwards compatibility with 1.7
	if osArgs.PprofEnabled && osArgs.ControllerPort == 0 {
		osArgs.ControllerPort = 6060
	}
	if osArgs.PrometheusEnabled && osArgs.ControllerPort == 0 {
		osArgs.ControllerPort = 6060
	}

	// Default annotations
	defaultBackendSvc := fmt.Sprint(osArgs.DefaultBackendService)
	defaultCertificate := fmt.Sprint(osArgs.DefaultCertificate)
	annotations.SetDefaultValue("default-backend-service", defaultBackendSvc)
	annotations.SetDefaultValue("default-backend-port", strconv.Itoa(osArgs.DefaultBackendPort))
	annotations.SetDefaultValue("ssl-certificate", defaultCertificate)

	// Start Controller
	var chanSize int64 = int64(watch.DefaultChanSize * 6)
	if osArgs.ChannelSize > 0 {
		chanSize = osArgs.ChannelSize
	}
	eventChan := make(chan k8s.SyncDataEvent, chanSize)
	ingressChan := make(chan ingress.Sync, chanSize)
	stop := make(chan struct{})

	publishService := getNamespaceValue(osArgs.PublishService)

	s := store.NewK8sStore(osArgs)

	c := controller.NewBuilder().
		WithHaproxyCfgFile(haproxyConf).
		WithEventChan(eventChan).
		WithIngressChan(ingressChan).
		WithStore(s).
		WithPublishService(publishService).
		WithArgs(osArgs).Build()

	k := k8s.New(
		osArgs,
		s.NamespacesAccess.Whitelist,
		publishService,
	)

	go k.MonitorChanges(eventChan, ingressChan, stop)
	go c.Start()
	if publishService != nil {
		go ingress.UpdateStatus(k.GetClientset(), s, osArgs.IngressClass, osArgs.EmptyIngressClass, ingressChan, annotations.New())
	}

	// Catch QUIT signals
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1)
	<-signalC
	c.Stop()
	close(stop)
}

func logInfo(logger utils.Logger, osArgs utils.OSArgs) {
	if len(osArgs.Version) > 0 {
		fmt.Printf("HAProxy Ingress Controller %s %s%s", GitTag, GitCommit, GitDirty)
		fmt.Printf("Build from: %s", GitRepo)
		fmt.Printf("Build date: %s\n", BuildTime)
		if len(osArgs.Version) > 1 {
			fmt.Printf("ConfigMap: %s", osArgs.ConfigMap)
			fmt.Printf("Ingress class: %s", osArgs.IngressClass)
			fmt.Printf("Empty Ingress class: %t", osArgs.EmptyIngressClass)
		}
		return
	}

	logger.Print(IngressControllerInfo)
	logger.Printf("HAProxy Ingress Controller %s %s%s", GitTag, GitCommit, GitDirty)
	logger.Printf("Build from: %s", GitRepo)
	logger.Printf("Build date: %s\n", BuildTime)
	logger.Printf("ConfigMap: %s", osArgs.ConfigMap)
	logger.Printf("Ingress class: %s", osArgs.IngressClass)
	logger.Printf("Empty Ingress class: %t", osArgs.EmptyIngressClass)
	logger.Printf("Publish service: %s", osArgs.PublishService)
	if osArgs.DefaultBackendService.String() != "" {
		logger.Printf("Default backend service: %s", osArgs.DefaultBackendService)
	} else {
		logger.Printf("Using local backend service on port: %s", osArgs.DefaultBackendPort)
	}
	logger.Printf("Default ssl certificate: %s", osArgs.DefaultCertificate)
	if !osArgs.DisableHTTP {
		logger.Printf("Frontend HTTP listening on: %s:%d", osArgs.IPV4BindAddr, osArgs.HTTPBindPort)
	}
	if !osArgs.DisableHTTPS {
		logger.Printf("Frontend HTTPS listening on: %s:%d", osArgs.IPV4BindAddr, osArgs.HTTPSBindPort)
	}
	if osArgs.DisableHTTP {
		logger.Printf("Disabling HTTP frontend")
	}
	if osArgs.DisableHTTPS {
		logger.Printf("Disabling HTTPS frontend")
	}
	if osArgs.DisableIPV4 {
		logger.Printf("Disabling IPv4 support")
	}
	if osArgs.DisableIPV6 {
		logger.Printf("Disabling IPv6 support")
	}
	if osArgs.ConfigMapTCPServices.Name != "" {
		logger.Printf("TCP Services provided in '%s'", osArgs.ConfigMapTCPServices)
	}
	if osArgs.ConfigMapErrorFiles.Name != "" {
		logger.Printf("Errorfiles provided in '%s'", osArgs.ConfigMapErrorFiles)
	}
	if osArgs.ConfigMapPatternFiles.Name != "" {
		logger.Printf("Pattern files provided in '%s'", osArgs.ConfigMapPatternFiles)
	}
	logger.Debugf("Kubernetes Informers resync period: %s", osArgs.CacheResyncPeriod.String())
	logger.Printf("Controller sync period: %s\n", osArgs.SyncPeriod.String())

	hostname, err := os.Hostname()
	logger.Error(err)
	logger.Printf("Running on %s", hostname)
}

func getNamespaceValue(name string) *utils.NamespaceValue {
	parts := strings.Split(name, "/")
	var result *utils.NamespaceValue
	if len(parts) == 2 {
		result = &utils.NamespaceValue{
			Namespace: parts[0],
			Name:      parts[1],
		}
	}
	return result
}
