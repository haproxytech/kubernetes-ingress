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
	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// When controller is not running on a containerized
// environment (out of Kubernetes)
func setupHAProxyEnv(osArgs utils.OSArgs) config.ControllerCfg {
	logger := utils.GetLogger()
	logger.Print("Running Controller out of K8s cluster")
	logger.FileName = true
	cfg := config.ControllerCfg{
		Env: config.Env{
			HAProxyBinary: "/usr/local/sbin/haproxy",
			MainCFGFile:   "/tmp/haproxy-ingress/etc/haproxy.cfg",
			CfgDir:        "/tmp/haproxy-ingress/etc",
			RuntimeDir:    "/tmp/haproxy-ingress/run",
			StateDir:      "/tmp/haproxy-ingress/state",
		},
	}
	if osArgs.CfgDir != "" {
		cfg.Env.CfgDir = osArgs.CfgDir
	}
	if osArgs.RuntimeDir != "" {
		cfg.Env.RuntimeDir = osArgs.RuntimeDir
	}
	if err := os.MkdirAll(cfg.Env.CfgDir, 0755); err != nil {
		logger.Panic(err)
	}
	if err := os.MkdirAll(cfg.Env.RuntimeDir, 0755); err != nil {
		logger.Panic(err)
	}

	// Try to copy original file if current directory is project directory
	// Otherwise check if haproxy.cfg is already in config-dir
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		logger.Panic(err)
	}
	logger.Debug(dir)
	origin := path.Join(dir, "fs/usr/local/etc/haproxy/haproxy.cfg")
	_, err = os.Stat(origin)
	if err != nil {
		if _, err = os.Stat(cfg.Env.MainCFGFile); err != nil {
			logger.Panicf("%s not found", cfg.Env.MainCFGFile)
		}
	} else if err = copyFile(origin, cfg.Env.MainCFGFile); err != nil {
		logger.Panic(err)
	}
	return cfg
}

func copyFile(src, dst string) (err error) {
	content, err := ioutil.ReadFile(src)
	if err != nil {
		return
	}
	return renameio.WriteFile(dst, content, 0640)
}
