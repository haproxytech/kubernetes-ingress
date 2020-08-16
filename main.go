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
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	//nolint
	_ "net/http/pprof"

	c "github.com/haproxytech/kubernetes-ingress/controller"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/jessevdk/go-flags"
)

var cfgDir string

func main() {

	var osArgs utils.OSArgs
	var parser = flags.NewParser(&osArgs, flags.IgnoreUnknown)
	_, err := parser.Parse()
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()
	if err != nil {
		fmt.Println(err)
		exitCode = 1
		return
	}
	logger := utils.GetLogger()
	logger.SetLevel(osArgs.LogLevel.LogLevel)

	defaultBackendSvc := fmt.Sprintf("%s/%s", osArgs.DefaultBackendService.Namespace, osArgs.DefaultBackendService.Name)
	defaultCertificate := fmt.Sprintf("%s/%s", osArgs.DefaultCertificate.Namespace, osArgs.DefaultCertificate.Name)
	c.SetDefaultAnnotation("default-backend-service", defaultBackendSvc)
	c.SetDefaultAnnotation("ssl-certificate", defaultCertificate)
	c.SetDefaultAnnotation("sync-period", osArgs.SyncPeriod.String())
	c.SetDefaultAnnotation("cache-resync-period", osArgs.CacheResyncPeriod.String())

	if len(osArgs.Version) > 0 {
		fmt.Printf("HAProxy Ingress Controller %s %s%s\n\n", GitTag, GitCommit, GitDirty)
		fmt.Printf("Build from: %s\n", GitRepo)
		fmt.Printf("Build date: %s\n\n", BuildTime)
		if len(osArgs.Version) > 1 {
			fmt.Printf("ConfigMap: %s/%s\n", osArgs.ConfigMap.Namespace, osArgs.ConfigMap.Name)
			fmt.Printf("Ingress class: %s\n", osArgs.IngressClass)
		}
		return
	}

	if len(osArgs.Help) > 0 && osArgs.Help[0] {
		parser.WriteHelp(os.Stdout)
		return
	}

	logger.FileName = false
	logger.Print(IngressControllerInfo)
	logger.Printf("HAProxy Ingress Controller %s %s%s\n", GitTag, GitCommit, GitDirty)
	logger.Printf("Build from: %s", GitRepo)
	logger.Printf("Build date: %s\n", BuildTime)
	if osArgs.PprofEnabled {
		logger.Warning("pprof endpoint exposed over https")
		go func() {
			logger.Error(http.ListenAndServe("127.0.0.1:6060", nil))
		}()
	}
	logger.Printf("ConfigMap: %s/%s", osArgs.ConfigMap.Namespace, osArgs.ConfigMap.Name)
	logger.Printf("Ingress class: %s", osArgs.IngressClass)
	logger.Printf("Publish service: %s", osArgs.PublishService)
	logger.Printf("Default backend service: %s", defaultBackendSvc)
	logger.Printf("Default ssl certificate: %s", defaultCertificate)
	logger.Printf("Controller sync period: %s", osArgs.SyncPeriod.String())
	logger.Debugf("Kubernetes Informers resync period: %s", osArgs.CacheResyncPeriod.String())

	if osArgs.ConfigMapTCPServices.Name != "" {
		logger.Printf("TCP Services defined in %s/%s", osArgs.ConfigMapTCPServices.Namespace, osArgs.ConfigMapTCPServices.Name)
	}
	if osArgs.ConfigMapErrorfiles.Name != "" {
		logger.Printf("Errofile pages defined in %s/%s", osArgs.ConfigMapErrorfiles.Namespace, osArgs.ConfigMapErrorfiles.Name)
	}
	logger.FileName = true

	ctx, cancel := context.WithCancel(context.Background())
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1)
	go func() {
		<-signalC
		cancel()
	}()

	cfgDir = "/etc/haproxy/"
	if osArgs.Test {
		setupTestEnv()
	}

	controller := c.HAProxyController{
		HAProxyCfgDir: cfgDir,
		Logger:        logger,
	}
	controller.Start(ctx, osArgs)
}
