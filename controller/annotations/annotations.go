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
		NewGlobalCfgSnippet("global-config-snippet"),
		NewFrontendCfgSnippet("frontend-config-snippet", "http"),
		NewFrontendCfgSnippet("frontend-config-snippet", "https"),
		NewFrontendCfgSnippet("stats-config-snippet", "stats"),
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

func GetBackendAnnotations(b *models.Backend) []Annotation {
	annotations := []Annotation{
		NewBackendCfgSnippet("backend-config-snippet", b.Name),
		service.NewAbortOnClose("abortonclose", b),
		service.NewTimeoutCheck("timeout-check", b),
		service.NewLoadBalance("load-balance", b),
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

// GetValue returns value by checking in multiple annotations.
func GetValue(annotationName string, annotations ...map[string]string) string {
	for _, a := range annotations {
		val, ok := a[annotationName]
		if ok {
			return val
		}
	}
	return defaultValues[annotationName]
}

func SetDefaultValue(annotation, value string) {
	defaultValues[annotation] = value
}

var defaultValues = map[string]string{
	"auth-realm":              "Protected Content",
	"check":                   "true",
	"cors-allow-origin":       "*",
	"cors-allow-methods":      "*",
	"cors-allow-headers":      "*",
	"cors-max-age":            "5s",
	"cookie-indirect":         "true",
	"cookie-nocache":          "true",
	"cookie-type":             "insert",
	"forwarded-for":           "true",
	"load-balance":            "roundrobin",
	"log-format":              "%ci:%cp [%tr] %ft %b/%s %TR/%Tw/%Tc/%Tr/%Ta %ST %B %CC %CS %tsc %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs \"%HM %[var(txn.base)] %HV\"",
	"rate-limit-size":         "100k",
	"rate-limit-period":       "1s",
	"rate-limit-status-code":  "403",
	"request-capture-len":     "128",
	"ssl-redirect-code":       "302",
	"request-redirect-code":   "302",
	"ssl-redirect-port":       "443",
	"ssl-passthrough":         "false",
	"server-ssl":              "false",
	"scale-server-slots":      "42",
	"syslog-server":           "address:127.0.0.1, facility: local0, level: notice",
	"timeout-http-request":    "5s",
	"timeout-connect":         "5s",
	"timeout-client":          "50s",
	"timeout-queue":           "5s",
	"timeout-server":          "50s",
	"timeout-tunnel":          "1h",
	"timeout-http-keep-alive": "1m",
	"hard-stop-after":         "1h",
	"client-crt-optional":     "false",
}
