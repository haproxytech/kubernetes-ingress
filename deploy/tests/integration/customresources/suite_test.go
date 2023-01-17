// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwarehaproxyConfig
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package customresources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haproxytech/client-native/v3/models"
	corev1alpha1 "github.com/haproxytech/kubernetes-ingress/crs/api/core/v1alpha1"
	c "github.com/haproxytech/kubernetes-ingress/pkg/controller"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type CustomResourceSuite struct {
	suite.Suite
	test Test
}

func TestCustomResource(t *testing.T) {
	suite.Run(t, new(CustomResourceSuite))
}

type Test struct {
	TempDir    string
	Controller *c.HAProxyController
}

func (suite *CustomResourceSuite) BeforeTest(suiteName, testName string) {
	tempDir, err := os.MkdirTemp("", "ut-"+testName+"-*")
	if err != nil {
		suite.T().Fatalf("Suite '%s': Test '%s' : error : %s", suiteName, testName, err)
	}
	suite.test.TempDir = tempDir
}

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
bind 127.0.0.1:8080 name v4
http-request set-var(txn.base) base
use_backend %[var(txn.path_match),field(1,.)]

frontend http
mode http
bind 127.0.0.1:4443 name v4
http-request set-var(txn.base) base
use_backend %[var(txn.path_match),field(1,.)]

frontend healthz
bind 127.0.0.1:1042 name v4
mode http
monitor-uri /healthz
option dontlog-normal

frontend stats
 mode http
 bind 0:0:0:0:1024
 http-request set-var(txn.base) base
 http-request use-service prometheus-exporter if { path /metrics }
 stats enable
 stats uri /
 stats refresh 10s
 `

func (suite *CustomResourceSuite) GlobalCRFixture() (eventChan chan k8s.SyncDataEvent, s store.K8s, globalCREvt k8s.SyncDataEvent) {
	var osArgs utils.OSArgs
	parser := flags.NewParser(&osArgs, flags.IgnoreUnknown)
	_, _ = parser.Parse()
	osArgs.Test = true
	eventChan = make(chan k8s.SyncDataEvent, watch.DefaultChanSize*6)
	s = store.NewK8sStore(osArgs)
	globalCREvt = k8s.SyncDataEvent{
		SyncType: k8s.CR_GLOBAL,
		Data: &corev1alpha1.Global{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fake",
			},
			Spec: corev1alpha1.GlobalSpec{
				Config:     &models.Global{},
				LogTargets: models.LogTargets{},
			},
		},
		Name: "globalcrjob",
	}

	haproxyEnv := env.Env{
		Binary:      "/usr/local/sbin/haproxy",
		MainCFGFile: filepath.Join(suite.test.TempDir, "haproxy.cfg"),
		CfgDir:      suite.test.TempDir,
		RuntimeDir:  filepath.Join(suite.test.TempDir, "run"),
		StateDir:    filepath.Join(suite.test.TempDir, "state/haproxy/"),
		Proxies: env.Proxies{
			FrontHTTP:  "http",
			FrontHTTPS: "https",
			FrontSSL:   "ssl",
			BackSSL:    "ssl",
		},
	}

	controller := c.NewBuilder().
		WithHaproxyCfgFile([]byte(haproxyConfig)).
		WithEventChan(eventChan).
		WithStore(s).
		WithHaproxyEnv(haproxyEnv).
		WithArgs(osArgs).Build()

	go controller.Start()
	return
}
