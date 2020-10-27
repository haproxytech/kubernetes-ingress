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
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	c "github.com/haproxytech/kubernetes-ingress/controller"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

const (
	TestFolderPath = "/tmp/haproxy-ingress/"
)

func setupTestEnv() {
	logger := utils.GetLogger()
	logger.Info("Running in test env")
	cfgDir = path.Join(TestFolderPath, cfgDir)
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		logger.Panic(err)
	}
	c.HAProxyStateDir = path.Join(TestFolderPath, "/var/state/haproxy/")
	if err := os.MkdirAll(c.HAProxyStateDir, 0755); err != nil {
		logger.Panic(err)
	}
	c.TransactionDir = path.Join(TestFolderPath, "transactions")
	time.Sleep(2 * time.Second)
	cmd := exec.Command("pwd")
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Panicf("cmd.Run() failed with %s\n", err.Error())
	}
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		logger.Panic(err)
	}
	logger.Debug(dir)
	copyFile(path.Join(dir, "fs/etc/haproxy/haproxy.cfg"), cfgDir)
	logger.Debug(string(out))
}

func copyFile(src, dst string) {
	logger := utils.GetLogger()
	cmd := fmt.Sprintf("cp %s %s", src, dst)
	logger.Debug(cmd)
	result := exec.Command("bash", "-c", cmd)
	_, err := result.CombinedOutput()
	logger.Debug(err)
}
