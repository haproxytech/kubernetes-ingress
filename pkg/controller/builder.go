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

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
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
	osArgs         utils.OSArgs
	haproxyClient  api.HAProxyClient
	haproxyEnv     env.Env
	haproxyProcess process.Process
	haproxyRules   rules.Rules
	haproxyCfgFile []byte
	annotations    annotations.Annotations
	store          store.K8s
	publishService *utils.NamespaceValue
	eventChan      chan k8s.SyncDataEvent
	ingressChan    chan ingress.Sync
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

func (builder *Builder) WithIngressChan(ingressChan chan ingress.Sync) *Builder {
	builder.ingressChan = ingressChan
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

func (builder *Builder) Build() *HAProxyController {
	if builder.haproxyCfgFile == nil {
		logger.Panic(errors.New("no HAProxy Config file provided"))
	}

	chShutdown := make(chan struct{})
	rtr := router.New()
	if builder.osArgs.ControllerPort != 0 {
		var runningServices string
		if builder.osArgs.PprofEnabled {
			rtr.GET("/debug/pprof/{profile:*}", pprofhandler.PprofHandler)
			runningServices += " pprof,"
		}
		if builder.osArgs.PrometheusEnabled {
			rtr.GET("/metrics", fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler()))
			runningServices += " prometheus,"
		}
		runningServices += " default service"
		rtr.GET("/healtz", requestHandler)
		// all others will be 404
		go func() {
			server := fasthttp.Server{
				Handler: rtr.Handler,
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

	haproxy, err := haproxy.New(builder.osArgs, builder.haproxyEnv, builder.haproxyCfgFile, builder.haproxyProcess, builder.haproxyClient, builder.haproxyRules)
	logger.Panic(err)

	prefix, errPrefix := utils.GetPodPrefix(os.Getenv("POD_NAME"))
	logger.Error(errPrefix)
	return &HAProxyController{
		osArgs:         builder.osArgs,
		haproxy:        haproxy,
		podNamespace:   os.Getenv("POD_NAMESPACE"),
		podPrefix:      prefix,
		store:          builder.store,
		eventChan:      builder.eventChan,
		ingressChan:    builder.ingressChan,
		publishService: builder.publishService,
		annotations:    builder.annotations,
		chShutdown:     chShutdown,
	}
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.Header.Set("X-HAProxy-Ingress-Controller", "healtz")
}
