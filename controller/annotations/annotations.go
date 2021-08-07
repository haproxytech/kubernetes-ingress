package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Annotation interface {
	GetName() string
	Process(value string) error
}

func GetGlobalAnnotations(client api.HAProxyClient, global *models.Global, defaults *models.Defaults) []Annotation {
	return []Annotation{
		NewFrontendCfgSnippet("frontend-config-snippet", client, []string{"http", "https"}),
		NewFrontendCfgSnippet("stats-config-snippet", client, []string{"stats"}),
		NewGlobalCfgSnippet("global-config-snippet", client),
		NewGlobalSyslogServers("syslog-server", client, global),
		NewGlobalNbthread("nbthread", global),
		NewGlobalMaxconn("maxconn", global),
		NewGlobalHardStopAfter("hard-stop-after", global),
		NewDefaultOption("http-server-close", defaults),
		NewDefaultOption("http-keep-alive", defaults),
		NewDefaultOption("dontlognull", defaults),
		NewDefaultOption("logasap", defaults),
		NewDefaultTimeout("timeout-http-request", defaults),
		NewDefaultTimeout("timeout-connect", defaults),
		NewDefaultTimeout("timeout-client", defaults),
		NewDefaultTimeout("timeout-client-fin", defaults),
		NewDefaultTimeout("timeout-queue", defaults),
		NewDefaultTimeout("timeout-server", defaults),
		NewDefaultTimeout("timeout-server-fin", defaults),
		NewDefaultTimeout("timeout-tunnel", defaults),
		NewDefaultTimeout("timeout-http-keep-alive", defaults),
		NewDefaultLogFormat("log-format", defaults),
	}
}

func GetBackendAnnotations(client api.HAProxyClient, b *models.Backend) []Annotation {
	annotations := []Annotation{
		NewBackendCfgSnippet("backend-config-snippet", client, b),
		NewBackendAbortOnClose("abortonclose", b),
		NewBackendTimeoutCheck("timeout-check", b),
		NewBackendLoadBalance("load-balance", b),
		NewBackendCookie("cookie-persistence", b),
	}
	if b.Mode == "http" {
		annotations = append(annotations,
			NewBackendCheckHTTP("check-http", b),
			NewBackendForwardedFor("forwarded-for", b),
		)
	}
	return annotations
}

func GetServerAnnotations(s *models.Server, k8sStore store.K8s, certs *haproxy.Certificates) []Annotation {
	return []Annotation{
		NewServerCheck("check", s),
		NewServerCheckInter("check-interval", s),
		NewServerCookie("cookie-persistence", s),
		NewServerMaxconn("pod-maxconn", s),
		NewServerSendProxy("send-proxy-protocol", s),
		// Order is important for ssl annotations so they don't conflict
		NewServerSSL("server-ssl", s),
		NewServerCrt("server-crt", k8sStore, certs, s),
		NewServerCA("server-ca", k8sStore, certs, s),
		NewServerProto("server-proto", s),
	}
}
