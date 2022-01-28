package controller

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/config"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/process"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type Builder struct {
	osArgs         utils.OSArgs
	haproxyEnv     config.Env
	haproxyProcess process.Process
	haproxyCfgFile []byte
	store          store.K8s
	publishService *utils.NamespaceValue
	eventChan      chan k8s.SyncDataEvent
	ingressChan    chan ingress.Sync
}

var defaultEnv = config.Env{
	Binary:      "/usr/local/sbin/haproxy",
	MainCFGFile: "/etc/haproxy/haproxy.cfg",
	CfgDir:      "/etc/haproxy/",
	RuntimeDir:  "/var/run",
	StateDir:    "/var/state/haproxy/",
	Proxies: config.Proxies{
		FrontHTTP:  "http",
		FrontHTTPS: "https",
		FrontSSL:   "ssl",
		BackSSL:    "ssl",
	},
}

func NewBuilder() *Builder {
	return &Builder{haproxyEnv: defaultEnv}
}

func (builder *Builder) WithHAProxyProcess(process process.Process) *Builder {
	builder.haproxyProcess = process
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

func (builder *Builder) WithHaproxyEnv(env config.Env) *Builder {
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
	if builder.osArgs.PromotheusPort != 0 {
		http.Handle("/metrics", promhttp.Handler())
		go func() {
			logger.Error(http.ListenAndServe(fmt.Sprintf(":%d", builder.osArgs.PromotheusPort), nil))
		}()
	}

	if builder.osArgs.PprofEnabled {
		logger.Warning("pprof endpoint exposed over https")
		go func() {
			logger.Error(http.ListenAndServe("127.0.0.1:6060", nil))
		}()
	}

	haproxy, err := haproxy.New(builder.osArgs, builder.haproxyEnv, builder.haproxyCfgFile, builder.haproxyProcess)
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
	}
}
