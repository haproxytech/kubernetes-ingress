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
	if len(osArgs.Version) > 0 {
		fmt.Printf("HAProxy Ingress Controller %s %s%s\n\n", GitTag, GitCommit, GitDirty)
		fmt.Printf("Build from: %s\n", GitRepo)
		fmt.Printf("Build date: %s\n\n", BuildTime)
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
	log.Printf("ConfigMap: %s/%s", osArgs.ConfigMap.Namespace, osArgs.ConfigMap.Name)
	//TODO currently using default log, switch to something more convenient
	log.SetFlags(LogType)
	LogErr(err)

	defaultAnnotationValues["default-backend-service"] = &StringW{
		Value:  fmt.Sprintf("%s/%s", osArgs.DefaultBackendService.Namespace, osArgs.DefaultBackendService.Name),
		Status: ADDED,
	}
	defaultAnnotationValues["ssl-certificate"] = &StringW{
		Value:  fmt.Sprintf("%s/%s", osArgs.DefaultCertificate.Namespace, osArgs.DefaultCertificate.Name),
		Status: ADDED,
	}

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
