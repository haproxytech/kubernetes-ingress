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
	"time"
)

func main() {

	fmt.Println(`
 _   _    _    ____                                                               
| | | |  / \  |  _ \ _ __ _____  ___   _                                          
| |_| | / _ \ | |_) | '__/ _ \ \/ / | | |                                         
|  _  |/ ___ \|  __/| | | (_) >  <| |_| |                                         
|_| |_/_/   \_\_|   |_|  \___/_/\_\\__, |                                         
 ___                               |___/__            _             _ _           
|_ _|_ __   __ _ _ __ ___  ___ ___   / ___|___  _ __ | |_ _ __ ___ | | | ___ _ __ 
 | || '_ \ / _` + "`" + ` | '__/ _ \/ __/ __| | |   / _ \| '_ \| __| '__/ _ \| | |/ _ \ '__|
 | || | | | (_| | | |  __/\__ \__ \ | |__| (_) | | | | |_| | | (_) | | |  __/ |   
|___|_| |_|\__, |_|  \___||___/___/  \____\___/|_| |_|\__|_|  \___/|_|_|\___|_|   
           |___/ 
`)
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
