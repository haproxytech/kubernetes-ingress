// Copyright 2023 HAProxy Technologies LLC
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

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	c "github.com/haproxytech/kubernetes-ingress/pkg/controller"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/watch"
)

var haproxyConfig = `global
daemon
master-worker
pidfile /var/run/haproxy.pid
stats socket /var/run/haproxy-runtime-api.sock level admin expose-fd listeners
default-path config

peers localinstance
 peer local 127.0.0.1:10000

frontend https
mode http
http-request set-var(txn.base) base
use_backend %[var(txn.path_match),field(1,.)]

frontend http
mode http
http-request set-var(txn.base) base
use_backend %[var(txn.path_match),field(1,.)]

frontend healthz
mode http
monitor-uri /healthz
option dontlog-normal

frontend stats
  mode http
  stats enable
  stats uri /
  stats refresh 10s
  http-request set-var(txn.base) base
  http-request use-service prometheus-exporter if { path /metrics }
 `

type updateStatusManager struct{}

func (m *updateStatusManager) AddIngress(ingress *ingress.Ingress) {}
func (m *updateStatusManager) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	return
}

type TestController struct {
	TempDir    string
	Controller *c.HAProxyController
	Store      store.K8s
	EventChan  chan k8ssync.SyncDataEvent
	OSArgs     utils.OSArgs
}

type BaseSuite struct {
	suite.Suite
	TestControllers map[string]*TestController
}

// BeforeTest:
// - create a tempDir for haproxy config + maps + ....
// - prepares and sets some common default start parameters for haproxy controller :
//   - "-e", "-t" and "--config-dir"
//
// To customize the controller start parameters, refer (as example) to
//
//	func (suite *DisableConfigSnippetSuite) BeforeTest(suiteName, testName string) {
func (suite *BaseSuite) BeforeTest(suiteName, testName string) {
	tempDir, err := os.MkdirTemp("", "tnr-"+testName+"-*")
	if err != nil {
		suite.T().Fatalf("Suite '%s': Test '%s' : error : %s", suiteName, testName, err)
	}
	if suite.TestControllers == nil {
		suite.TestControllers = make(map[string]*TestController)
	}

	// Some common default arguments
	var osArgs utils.OSArgs
	os.Args = []string{os.Args[0], "-e", "-t", "--config-dir=" + tempDir}
	parser := flags.NewParser(&osArgs, flags.IgnoreUnknown)
	_, errParsing := parser.Parse() //nolint:ifshort
	if errParsing != nil {
		suite.T().Fatal(errParsing)
	}

	suite.TestControllers[suite.T().Name()] = &TestController{
		TempDir: tempDir,
		OSArgs:  osArgs,
	}
	suite.T().Logf("temporary configuration dir %s", tempDir)
}

// AfterTest stops the controllers and delete the entry from the test map
func (suite *BaseSuite) AfterTest(suiteName, testName string) {
	delete(suite.TestControllers, suite.T().Name())
}

// StartController starts a controller
// It is not started in BeforeTest() or SetupSubTest() but must be called when desired in the test/subtest.
func (suite *BaseSuite) StartController() {
	testController := suite.TestControllers[suite.T().Name()]

	testController.Store = store.NewK8sStore(testController.OSArgs)

	haproxyEnv := env.Env{
		Proxies: env.Proxies{
			FrontHTTP:  "http",
			FrontHTTPS: "https",
			FrontSSL:   "ssl",
			BackSSL:    "ssl",
		},
	}

	testController.EventChan = make(chan k8ssync.SyncDataEvent, watch.DefaultChanSize*6)
	testController.Controller = c.NewBuilder().
		WithHaproxyCfgFile([]byte(haproxyConfig)).
		WithEventChan(testController.EventChan).
		WithStore(testController.Store).
		WithHaproxyEnv(haproxyEnv).
		WithUpdateStatusManager(&updateStatusManager{}).
		WithArgs(testController.OSArgs).Build()

	annotations.InitCfgSnippet()
	annotations.DisableConfigSnippets(testController.OSArgs.DisableConfigSnippets)

	go testController.Controller.Start()
}

func (suite *BaseSuite) StopController() {
	testController := suite.TestControllers[suite.T().Name()]
	testController.Controller.Stop()
	testController.Store.Clean()
	close(testController.EventChan)
}

func (suite *BaseSuite) ExpectHaproxyConfigContains(s string, count int) {
	testController := suite.TestControllers[suite.T().Name()]

	content, err := os.ReadFile(filepath.Join(testController.TempDir, "haproxy.cfg"))
	if err != nil {
		suite.T().Error(err.Error())
	}
	c := strings.Count(string(content), s)
	suite.Exactly(count, c, fmt.Sprintf("%s is repeated %d times but expected %d", s, c, count))
}
