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
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Handle HAProxy daemon via Master process
func (c *HAProxyController) haproxyService(action string) (err error) {
	if c.OSArgs.Test {
		logger.Infof("HAProxy would be %sed now", action)
		return nil
	}
	var cmd *exec.Cmd
	// if processErr is nil, process variable will automatically
	// hold information about a running Master HAproxy process
	process, processErr := c.haproxyProcess()

	switch action {
	case "start":
		if processErr == nil {
			logger.Error(fmt.Errorf("haproxy is already running"))
			return nil
		}
		//nolint:gosec //checks on HAProxyBinary should be done in configuration module.
		cmd = exec.Command(c.Cfg.Env.HAProxyBinary, "-W", "-f", c.Cfg.Env.MainCFGFile, "-p", c.Cfg.Env.PIDFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	case "stop":
		if processErr != nil {
			logger.Error(fmt.Errorf("haproxy  already stopped"))
			return processErr
		}
		if err = process.Signal(syscall.SIGUSR1); err != nil {
			return err
		}
		_, err = process.Wait()
		return err
	case "reload":
		logger.Error(c.saveServerState())
		if processErr != nil {
			logger.Errorf("haproxy is not running, trying to start it")
			return c.haproxyService("start")
		}
		return process.Signal(syscall.SIGUSR2)
	case "restart":
		logger.Error(c.saveServerState())
		if processErr != nil {
			logger.Errorf("haproxy is not running, trying to start it")
			return c.haproxyService("start")
		}
		pid := strconv.Itoa(process.Pid)
		//nolint:gosec //checks on HAProxyBinary should be done in configuration module.
		cmd = exec.Command(c.Cfg.Env.HAProxyBinary, "-W", "-f", c.Cfg.Env.MainCFGFile, "-p", c.Cfg.Env.PIDFile, "-sf", pid)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	default:
		return fmt.Errorf("unknown command '%s'", action)
	}
}

// Return HAProxy master process if it exists.
func (c *HAProxyController) haproxyProcess() (*os.Process, error) {
	file, err := os.Open(c.Cfg.Env.PIDFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	pid, err := strconv.Atoi(scanner.Text())
	if err != nil {
		return nil, err
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}
	err = process.Signal(syscall.Signal(0))
	return process, err
}

// Saves HAProxy servers state so it is retrieved after reload.
func (c *HAProxyController) saveServerState() error {
	result, err := c.Client.ExecuteRaw("show servers state")
	if err != nil {
		return err
	}
	var f *os.File
	if f, err = os.Create(c.Cfg.Env.StateDir + "global"); err != nil {
		logger.Error(err)
		return err
	}
	defer f.Close()
	if _, err = f.Write([]byte(result[0])); err != nil {
		logger.Error(err)
		return err
	}
	if err = f.Sync(); err != nil {
		logger.Error(err)
		return err
	}
	if err = f.Close(); err != nil {
		logger.Error(err)
		return err
	}
	return nil
}

func (c *HAProxyController) haproxyStartup() {
	//nolint:gosec //checks on HAProxyBinary should be done in configuration module.
	cmd := exec.Command(c.Cfg.Env.HAProxyBinary, "-v")
	haproxyInfo, err := cmd.Output()
	if err == nil {
		haproxyInfo := strings.Split(string(haproxyInfo), "\n")
		logger.Printf("Running with %s", haproxyInfo[0])
	} else {
		logger.Error(err)
	}
	logger.Printf("Starting HAProxy with %s", c.Cfg.Env.MainCFGFile)
	logger.Panic(c.haproxyService("start"))
}
