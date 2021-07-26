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
	"os"
	"path"
	"path/filepath"

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
		cfg.Env.MainCFGFile = path.Join(cfg.Env.CfgDir, "haproxy.cfg")
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

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		logger.Panic(err)
	}
	logger.Debug(dir)

	return cfg
}
