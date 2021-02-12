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
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/google/renameio"
	c "github.com/haproxytech/kubernetes-ingress/controller"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// When controller is not running on a containerized
// environment (out of Kubernetes)
func setupHAProxyEnv(osArgs utils.OSArgs) {
	logger := utils.GetLogger()
	logger.Print("Running Controller out of K8s cluster")
	logger.FileName = true
	c.HAProxyCfgDir = "/tmp/haproxy-ingress/etc"
	runtimeDir := "/tmp/haproxy-ingress/run"
	if osArgs.CfgDir != "" {
		c.HAProxyCfgDir = osArgs.CfgDir
	}
	if osArgs.RuntimeDir != "" {
		runtimeDir = osArgs.RuntimeDir
	}
	if err := os.MkdirAll(c.HAProxyCfgDir, 0755); err != nil {
		logger.Panic(err)
	}
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		logger.Panic(err)
	}
	c.TransactionDir = path.Join(c.HAProxyCfgDir, "transactions")
	c.HAProxyStateDir = runtimeDir
	c.HAProxyRuntimeSocket = path.Join(runtimeDir, "haproxy-runtime-api.sock")
	c.HAProxyPIDFile = path.Join(runtimeDir, "haproxy.pid")
	time.Sleep(2 * time.Second)

	pwd, err := os.Getwd()
	if err != nil {
		logger.Panicf("Couldn't get current directory %s\n", err.Error())
	}
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		logger.Panic(err)
	}
	logger.Debug(dir)
	if err = copyFile(path.Join(dir, "fs/etc/haproxy/haproxy.cfg"), path.Join(c.HAProxyCfgDir, "haproxy.cfg")); err != nil {
		logger.Panic(err)
	}
	logger.Debug(pwd)
}

func copyFile(src, dst string) (err error) {
	content, err := ioutil.ReadFile(src)
	if err != nil {
		return
	}
	return renameio.WriteFile(dst, content, 0640)
}
