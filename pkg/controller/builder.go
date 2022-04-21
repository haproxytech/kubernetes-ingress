package controller

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	config "github.com/haproxytech/kubernetes-ingress/pkg/configuration"
	"github.com/haproxytech/kubernetes-ingress/pkg/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type Builder struct {
	osArgs         utils.OSArgs
	cfg            config.ControllerCfg
	store          store.K8s
	publishService *utils.NamespaceValue
	eventChan      chan k8s.SyncDataEvent
	ingressChan    chan ingress.Sync
}

var defaultCfg = config.ControllerCfg{
	Env: config.Env{
		HAProxyBinary: "/usr/local/sbin/haproxy",
		MainCFGFile:   "/etc/haproxy/haproxy.cfg",
		CfgDir:        "/etc/haproxy/",
		RuntimeDir:    "/var/run",
		StateDir:      "/var/state/haproxy/",
	},
}

func NewBuilder() *Builder {
	return &Builder{
		cfg: defaultCfg,
	}
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

func (builder *Builder) WithConfiguration(cfg config.ControllerCfg) *Builder {
	builder.cfg = cfg
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
	if builder.osArgs.External {
		builder.cfg = setupExternalMode(builder.osArgs)
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

	prefix, errPrefix := utils.GetPodPrefix(os.Getenv("POD_NAME"))
	logger.Error(errPrefix)
	return &HAProxyController{
		cfg:            builder.cfg,
		osArgs:         builder.osArgs,
		podNamespace:   os.Getenv("POD_NAMESPACE"),
		podPrefix:      prefix,
		store:          builder.store,
		eventChan:      builder.eventChan,
		ingressChan:    builder.ingressChan,
		publishService: builder.publishService,
	}
}

// When controller is not running on a containerized
// environment (out of Kubernetes)
func setupExternalMode(osArgs utils.OSArgs) config.ControllerCfg {
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

	if osArgs.Program != "" {
		cfg.Env.HAProxyBinary = osArgs.Program
	}

	return cfg
}
