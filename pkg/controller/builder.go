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
	"errors"
	"os"
	"strconv"

	"github.com/fasthttp/router"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"github.com/valyala/fasthttp/pprofhandler"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	gateway "github.com/haproxytech/kubernetes-ingress/pkg/gateways"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/env"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/process"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type Builder struct {
	osArgs                   utils.OSArgs
	haproxyClient            api.HAProxyClient
	haproxyEnv               env.Env
	haproxyProcess           process.Process
	haproxyRules             rules.Rules
	haproxyCfgFile           []byte
	annotations              annotations.Annotations
	store                    store.K8s
	publishService           *utils.NamespaceValue
	eventChan                chan k8s.SyncDataEvent
	updatePublishServiceFunc func(ingresses []*ingress.Ingress, publishServiceAddresses []string)
	clientSet                *kubernetes.Clientset
	restClientSet            client.Client
	gatewayManager           gateway.GatewayManager
}

var defaultEnv = env.Env{
	Binary:      "/usr/local/sbin/haproxy",
	MainCFGFile: "/etc/haproxy/haproxy.cfg",
	CfgDir:      "/etc/haproxy/",
	RuntimeDir:  "/var/run",
	StateDir:    "/var/state/haproxy/",
	Proxies: env.Proxies{
		FrontHTTP:  "http",
		FrontHTTPS: "https",
		FrontSSL:   "ssl",
		BackSSL:    "ssl",
	},
}

func NewBuilder() *Builder {
	return &Builder{
		haproxyEnv:   defaultEnv,
		annotations:  annotations.New(),
		haproxyRules: rules.New(),
	}
}

func (builder *Builder) WithHAProxyProcess(process process.Process) *Builder {
	builder.haproxyProcess = process
	return builder
}

func (builder *Builder) WithAnnotations(a annotations.Annotations) *Builder {
	builder.annotations = a
	return builder
}

func (builder *Builder) WithHAProxyRules(rules rules.Rules) *Builder {
	builder.haproxyRules = rules
	return builder
}

func (builder *Builder) WithHaproxyClient(haproxyClient api.HAProxyClient) *Builder {
	builder.haproxyClient = haproxyClient
	return builder
}

func (builder *Builder) WithEventChan(eventChan chan k8s.SyncDataEvent) *Builder {
	builder.eventChan = eventChan
	return builder
}

func (builder *Builder) WithStore(store store.K8s) *Builder {
	builder.store = store
	return builder
}

func (builder *Builder) WithHaproxyEnv(env env.Env) *Builder {
	builder.haproxyEnv = env
	return builder
}

func (builder *Builder) WithHaproxyCfgFile(cfgFile []byte) *Builder {
	builder.haproxyCfgFile = cfgFile
	return builder
}

func (builder *Builder) WithArgs(osArgs utils.OSArgs) *Builder {
	builder.osArgs = osArgs
	return builder
}

func (builder *Builder) WithPublishService(publishService *utils.NamespaceValue) *Builder {
	builder.publishService = publishService
	return builder
}

func (builder *Builder) WithUpdatePublishServiceFunc(updatePublishServiceFunc func(ingresses []*ingress.Ingress, publishServiceAddresses []string)) *Builder {
	builder.updatePublishServiceFunc = updatePublishServiceFunc
	return builder
}

func (builder *Builder) WithClientSet(clientSet *kubernetes.Clientset) *Builder {
	builder.clientSet = clientSet
	return builder
}

func (builder *Builder) WithRestClientSet(restClientSet client.Client) *Builder {
	builder.restClientSet = restClientSet
	return builder
}

func (builder *Builder) WithGatewayManager(gatewayManager gateway.GatewayManager) *Builder {
	builder.gatewayManager = gatewayManager
	return builder
}

func (builder *Builder) Build() *HAProxyController {
	if builder.haproxyCfgFile == nil {
		logger.Panic(errors.New("no HAProxy Config file provided"))
	}

	chShutdown := make(chan struct{})

	if builder.osArgs.ControllerPort != 0 {
		addControllerMetricData(builder, chShutdown)
	}

	if builder.osArgs.DefaultBackendService.String() == "" {
		addLocalDefaultService(builder, chShutdown)
	}

	haproxy, err := haproxy.New(builder.osArgs, builder.haproxyEnv, builder.haproxyCfgFile, builder.haproxyProcess, builder.haproxyClient, builder.haproxyRules)
	logger.Panic(err)

	prefix, errPrefix := utils.GetPodPrefix(os.Getenv("POD_NAME"))
	logger.Error(errPrefix)

	builder.store.UpdateStatusFunc = ingress.NewStatusIngressUpdater(builder.clientSet, builder.store, builder.osArgs.IngressClass, builder.osArgs.EmptyIngressClass, builder.annotations)
	builder.store.GatewayControllerName = builder.osArgs.GatewayControllerName
	gatewayManager := builder.gatewayManager
	if gatewayManager == nil {
		gatewayManager = gateway.New(builder.store, haproxy.HAProxyClient, builder.osArgs, builder.restClientSet)
	}
	return &HAProxyController{
		osArgs:                   builder.osArgs,
		haproxy:                  haproxy,
		podNamespace:             os.Getenv("POD_NAMESPACE"),
		podPrefix:                prefix,
		store:                    builder.store,
		eventChan:                builder.eventChan,
		publishService:           builder.publishService,
		annotations:              builder.annotations,
		chShutdown:               chShutdown,
		updatePublishServiceFunc: builder.updatePublishServiceFunc,
		gatewayManager:           gatewayManager,
	}
}

func addControllerMetricData(builder *Builder, chShutdown chan struct{}) {
	rtr := router.New()
	var runningServices string
	if builder.osArgs.PprofEnabled {
		rtr.GET("/debug/pprof/{profile:*}", pprofhandler.PprofHandler)
		runningServices += " pprof"
	}
	if builder.osArgs.PrometheusEnabled {
		rtr.GET("/metrics", fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler()))
		runningServices += ", prometheus"
	}
	rtr.GET("/healtz", requestHandler)
	// all others will be 404
	go func() {
		server := fasthttp.Server{
			Handler:               rtr.Handler,
			NoDefaultServerHeader: true,
		}
		go func() {
			<-chShutdown
			if err := server.Shutdown(); err != nil {
				logger.Errorf("Could not gracefully shutdown controller data server: %v\n", err)
			} else {
				logger.Errorf("Gracefully shuting down controller data server")
			}
		}()
		logger.Infof("running controller data server on :%d, running%s", builder.osArgs.ControllerPort, runningServices)
		err := server.ListenAndServe(":" + strconv.Itoa(builder.osArgs.ControllerPort))
		logger.Error(err)
	}()
}

func addLocalDefaultService(builder *Builder, chShutdown chan struct{}) {
	rtr := router.New()
	rtr.GET("/healtz", requestHandler)
	// all others will be 404
	go func() {
		server := fasthttp.Server{
			Handler:               rtr.Handler,
			NoDefaultServerHeader: true,
		}
		go func() {
			<-chShutdown
			if err := server.Shutdown(); err != nil {
				logger.Errorf("Could not gracefully shutdown controller data server: %v\n", err)
			} else {
				logger.Errorf("Gracefully shuting down controller data server")
			}
		}()
		logger.Infof("running default backend server on :%d", builder.osArgs.DefaultBackendPort)
		err := server.ListenAndServe(":" + strconv.Itoa(builder.osArgs.DefaultBackendPort))
		logger.Error(err)
	}()
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.Header.Set("X-HAProxy-Ingress-Controller", "healtz")
}
