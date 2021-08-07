package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/global"
	"github.com/haproxytech/kubernetes-ingress/controller/annotations/service"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Annotation interface {
	GetName() string
	Process(value string) error
}

func GetGlobalAnnotations(client api.HAProxyClient, g *models.Global) []Annotation {
	return []Annotation{
		global.NewFrontendCfgSnippet("frontend-config-snippet", client, []string{"http", "https"}),
		global.NewFrontendCfgSnippet("stats-config-snippet", client, []string{"stats"}),
		global.NewCfgSnippet("global-config-snippet", client),
		global.NewSyslogServers("syslog-server", client, g),
		global.NewNbthread("nbthread", g),
		global.NewMaxconn("maxconn", g),
		global.NewHardStopAfter("hard-stop-after", g),
	}
}

func GetDefaultsAnnotations(d *models.Defaults) []Annotation {
	return []Annotation{
		global.NewOption("http-server-close", d),
		global.NewOption("http-keep-alive", d),
		global.NewOption("dontlognull", d),
		global.NewOption("logasap", d),
		global.NewTimeout("timeout-http-request", d),
		global.NewTimeout("timeout-connect", d),
		global.NewTimeout("timeout-client", d),
		global.NewTimeout("timeout-client-fin", d),
		global.NewTimeout("timeout-queue", d),
		global.NewTimeout("timeout-server", d),
		global.NewTimeout("timeout-server-fin", d),
		global.NewTimeout("timeout-tunnel", d),
		global.NewTimeout("timeout-http-keep-alive", d),
		global.NewLogFormat("log-format", d),
	}
}

func GetBackendAnnotations(client api.HAProxyClient, b *models.Backend) []Annotation {
	annotations := []Annotation{
		service.NewCfgSnippet("backend-config-snippet", client, b),
		service.NewAbortOnClose("abortonclose", b),
		service.NewTimeoutCheck("timeout-check", b),
		service.NewLoadBalance("load-balance", b),
		service.NewCookie("cookie-persistence", b, nil),
	}
	if b.Mode == "http" {
		annotations = append(annotations,
			service.NewCheckHTTP("check-http", b),
			service.NewForwardedFor("forwarded-for", b),
		)
	}
	return annotations
}

func GetServerAnnotations(s *models.Server, k8sStore store.K8s, certs *haproxy.Certificates) []Annotation {
	return []Annotation{
		service.NewCheck("check", s),
		service.NewCheckInter("check-interval", s),
		service.NewCookie("cookie-persistence", nil, s),
		service.NewMaxconn("pod-maxconn", s),
		service.NewSendProxy("send-proxy-protocol", s),
		// Order is important for ssl annotations so they don't conflict
		service.NewSSL("server-ssl", s),
		service.NewCrt("server-crt", k8sStore, certs, s),
		service.NewCA("server-ca", k8sStore, certs, s),
		service.NewProto("server-proto", s),
	}
}
