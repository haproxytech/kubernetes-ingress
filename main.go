package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jessevdk/go-flags"
)

// fixed paths to haproxy items
const (
	HAProxyCFG       = "/etc/haproxy/haproxy.cfg"
	HAProxyGlobalCFG = "/etc/haproxy/global.cfg"
	HAProxyStatCFG   = "/etc/haproxy/stats.cfg"
	HAProxyCertDir   = "/etc/haproxy/certs/"
	HAProxyStateDir  = "/var/state/haproxy/"
	HAProxySocket    = "/var/run/haproxy-runtime-api.sock"
	FrontendHTTP     = "http"
	FrontendHTTPS    = "https"
)

func main() {

	var osArgs OSArgs
	var parser = flags.NewParser(&osArgs, flags.Default)
	parser.Parse()
	if len(osArgs.Version) > 0 {
		fmt.Printf("HAProxy Ingress Controller %s %s%s\n\n", GitTag, GitCommit, GitDirty)
		fmt.Printf("Build from: %s\n", GitRepo)
		fmt.Printf("Build date: %s\n\n", BuildTime)
		return
	}

	log.Println(IngressControllerInfo)
	log.Printf("HAProxy Ingress Controller %s %s%s\n\n", GitTag, GitCommit, GitDirty)
	log.Printf("Build from: %s\n", GitRepo)
	log.Printf("Build date: %s\n\n", BuildTime)
	//TODO currently using default log, switch to something more convenient
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	hAProxyController := HAProxyController{}
	hAProxyController.Start(osArgs)

	//TODO wait channel
	for {
		//TODO don't do that
		time.Sleep(60 * time.Hour)
		//log.Println("sleeping")
	}
}
