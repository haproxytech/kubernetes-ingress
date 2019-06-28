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
	"time"

	"github.com/jessevdk/go-flags"
)

// fixed paths to haproxy items
const (
	FrontendHTTP   = "http"
	FrontendHTTPS  = "https"
	TestFolderPath = "/tmp/haproxy-ingress/"
	LogTypeShort   = log.LstdFlags
	LogType        = log.LstdFlags | log.Lshortfile
)

var (
	HAProxyCFG       = "/etc/haproxy/haproxy.cfg"
	HAProxyGlobalCFG = "/etc/haproxy/global.cfg"
	HAProxyCertDir   = "/etc/haproxy/certs/"
	HAProxyStateDir  = "/var/state/haproxy/"
)

func main() {

	var osArgs OSArgs
	var parser = flags.NewParser(&osArgs, flags.IgnoreUnknown)
	_, err := parser.Parse()

	defaultAnnotationValues["default-backend-service"] = &StringW{
		Value:  fmt.Sprintf("%s/%s", osArgs.DefaultBackendService.Namespace, osArgs.DefaultBackendService.Name),
		Status: ADDED,
	}
	defaultAnnotationValues["ssl-certificate"] = &StringW{
		Value:  fmt.Sprintf("%s/%s", osArgs.DefaultCertificate.Namespace, osArgs.DefaultCertificate.Name),
		Status: ADDED,
	}

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

	log.Println(IngressControllerInfo)
	log.Printf("HAProxy Ingress Controller %s %s%s\n\n", GitTag, GitCommit, GitDirty)
	log.Printf("Build from: %s\n", GitRepo)
	log.Printf("Build date: %s\n\n", BuildTime)
	log.Printf("ConfigMap: %s/%s\n", osArgs.ConfigMap.Namespace, osArgs.ConfigMap.Name)
	log.Printf("Ingress class: %s\n", osArgs.IngressClass)
	//TODO currently using default log, switch to something more convenient
	log.SetFlags(LogType)
	LogErr(err)

	log.Printf("Default backend service: %s\n", defaultAnnotationValues["default-backend-service"].Value)
	log.Printf("Default ssl certificate: %s\n", defaultAnnotationValues["ssl-certificate"].Value)

	if osArgs.Test {
		setupTestEnv()
	}

	hAProxyController := HAProxyController{}
	hAProxyController.Start(osArgs)

	//TODO wait channel
	for {
		//TODO don't do that
		time.Sleep(60 * time.Hour)
		//log.Println("sleeping")
	}
}
