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

package controller

import (
	"os"
	"os/exec"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/process"
)

// Handle HAProxy daemon via Master process
func (c *HAProxyController) haproxyService(action string) (err error) {
	return c.haproxyProcess.HaproxyService(action)
}

func (c *HAProxyController) haproxyStartup() {
	//nolint:gosec //checks on HAProxyBinary should be done in configuration module.
	cmd := exec.Command(c.cfg.Env.HAProxyBinary, "-v")
	haproxyInfo, err := cmd.Output()
	if err == nil {
		haproxyInfo := strings.Split(string(haproxyInfo), "\n")
		logger.Printf("Running with %s", haproxyInfo[0])
	} else {
		logger.Error(err)
	}
	var msgAuxConfigFile string
	if c.osArgs.UseWiths6Overlay {
		c.haproxyProcess = process.NewControlOverS6(c.cfg.Env, c.osArgs, c.client)
	} else {
		c.haproxyProcess = process.NewDirectControl(c.cfg.Env, c.osArgs, c.client)
		if _, err := os.Stat(c.cfg.Env.AuxCFGFile); err == nil {
			c.haproxyProcess.UseAuxFile(true)
			msgAuxConfigFile = "and aux config file " + c.cfg.Env.AuxCFGFile
		}
	}
	logger.Printf("Starting HAProxy with %s %s", c.cfg.Env.MainCFGFile, msgAuxConfigFile)
	logger.Panic(c.haproxyService("start"))
}