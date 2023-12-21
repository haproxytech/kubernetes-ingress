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

package env

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/renameio"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// Env contains Directories and files required by haproxy
type Env struct {
	Certs certs.Env
	Proxies
	CfgDir         string
	RuntimeSocket  string
	PIDFile        string
	AuxCFGFile     string
	RuntimeDir     string
	StateDir       string
	PatternDir     string
	ErrFileDir     string
	MapsDir        string
	Binary         string
	MainCFGFile    string
	MainCFGRaw     []byte
	ControllerPort int
}

// Proxies contains names of the main proxies of haproxy config
type Proxies struct {
	FrontHTTP  string
	FrontHTTPS string
	FrontSSL   string
	BackSSL    string
}

// Init initializes HAProxy Environment
func (env *Env) Init(osArgs utils.OSArgs) (err error) {
	if osArgs.External {
		if err = env.external(osArgs); err != nil {
			return fmt.Errorf("unable to configure environment for external mode: %w", err)
		}
	}
	for _, dir := range []string{env.CfgDir, env.RuntimeDir, env.StateDir} {
		if dir == "" {
			return fmt.Errorf("failed to init controller config: missing config directories")
		}
	}
	// Binary and main files
	env.AuxCFGFile = filepath.Join(env.CfgDir, "haproxy-aux.cfg")
	env.PIDFile = filepath.Join(env.RuntimeDir, "haproxy.pid")
	env.RuntimeSocket = filepath.Join(env.RuntimeDir, "haproxy-runtime-api.sock")
	if osArgs.Test {
		env.Binary = "echo"
		env.RuntimeSocket = ""
	} else if _, err = os.Stat(env.Binary); err != nil {
		return err
	}
	err = renameio.WriteFile(env.MainCFGFile, env.MainCFGRaw, 0o755)
	if err != nil {
		return err
	}
	// Directories
	env.Certs.MainDir = filepath.Join(env.CfgDir, "certs")
	env.Certs.FrontendDir = filepath.Join(env.Certs.MainDir, "frontend")
	env.Certs.BackendDir = filepath.Join(env.Certs.MainDir, "backend")
	env.Certs.CaDir = filepath.Join(env.Certs.MainDir, "ca")
	env.MapsDir = filepath.Join(env.CfgDir, "maps")
	env.PatternDir = filepath.Join(env.CfgDir, "patterns")
	env.ErrFileDir = filepath.Join(env.CfgDir, "errorfiles")
	env.ControllerPort = osArgs.ControllerPort
	for _, d := range []string{
		env.Certs.MainDir,
		env.Certs.FrontendDir,
		env.Certs.BackendDir,
		env.Certs.CaDir,
		env.MapsDir,
		env.ErrFileDir,
		env.StateDir,
		env.PatternDir,
	} {
		err = os.MkdirAll(d, 0o755)
		if err != nil {
			return err
		}
	}
	return
}

// When controller is not running on a containerized
// environment (out of Kubernetes)
func (env *Env) external(osArgs utils.OSArgs) (err error) {
	env.Binary = "/usr/local/sbin/haproxy"
	env.MainCFGFile = "/tmp/haproxy-ingress/etc/haproxy.cfg"
	env.CfgDir = "/tmp/haproxy-ingress/etc"
	env.RuntimeDir = "/tmp/haproxy-ingress/run"
	env.StateDir = "/tmp/haproxy-ingress/state/"
	if osArgs.Program != "" {
		env.Binary = osArgs.Program
	}
	if osArgs.CfgDir != "" {
		env.CfgDir = osArgs.CfgDir
		env.MainCFGFile = filepath.Join(env.CfgDir, "haproxy.cfg")
	}
	if osArgs.RuntimeDir != "" {
		env.RuntimeDir = osArgs.RuntimeDir
	}
	for _, dir := range []string{env.CfgDir, env.RuntimeDir, env.StateDir} {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return err
}
