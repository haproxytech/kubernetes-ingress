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

	// Try to copy original file if current directory is project directory
	// Otherwise check if haproxy.cfg is already in config-dir
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		logger.Panic(err)
	}
	logger.Debug(dir)
	cfgFile := path.Join(c.HAProxyCfgDir, "haproxy.cfg")
	origin := path.Join(dir, "fs/etc/haproxy/haproxy.cfg")
	_, err = os.Stat(origin)
	if err != nil {
		if _, err = os.Stat(cfgFile); err != nil {
			logger.Panicf("%s not found", cfgFile)
		}
	} else if err = copyFile(origin, cfgFile); err != nil {
		logger.Panic(err)
	}
}

func copyFile(src, dst string) (err error) {
	content, err := ioutil.ReadFile(src)
	if err != nil {
		return
	}
	return renameio.WriteFile(dst, content, 0640)
}
