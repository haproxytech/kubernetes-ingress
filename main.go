/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

func main() {

	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "-vv") {
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
	//log.SetFlags(log.LstdFlags | log.Llongfile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	hAProxyController := HAProxyController{}
	hAProxyController.Start()

	//TODO wait channel
	for {
		//TODO don't do that
		time.Sleep(60 * time.Hour)
		//log.Println("sleeping")
	}
}
