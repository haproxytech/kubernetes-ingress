package annotations

import (
	"errors"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type Annotation interface {
	Parse(value store.StringW, forceParse bool) error
	GetName() string
	Update() error
}

var ErrEmptyStatus = errors.New("emptyST")
var logger = utils.GetLogger()

func HandleGlobalAnnotations(k8sStore store.K8s, client api.HAProxyClient, forcePase bool, annotations store.MapStringW) (restart bool, reload bool) {
	annList := GetGlobalAnnotations(client)
	for _, a := range annList {
		annValue, _ := k8sStore.GetValueFromAnnotations(a.GetName(), annotations)
		if annValue == nil {
			continue
		}
		reload = HandleAnnotation(a, *annValue, forcePase) || reload
	}
	// Check syslog-server annotation for a restart (stdout logging)
	if a, ok := annList[1].(*GlobalSyslogServers); ok {
		restart = a.Restart()
	}
	return restart, reload
}

func HandleBackendAnnotations(k8sStore store.K8s, client api.HAProxyClient, backend *models.Backend, forceParse bool, annotations ...store.MapStringW) (reload bool) {
	for _, a := range GetBackendAnnotations(client, backend) {
		annValue, _ := k8sStore.GetValueFromAnnotations(a.GetName(), annotations...)
		if annValue == nil {
			continue
		}
		reload = HandleAnnotation(a, *annValue, forceParse) || reload
	}
	return reload
}

func HandleServerAnnotations(k8sStore store.K8s, client api.HAProxyClient, haproxyCerts *haproxy.Certificates, server *models.Server, forceParse bool, annotations ...store.MapStringW) (reload bool) {
	for _, a := range GetServerAnnotations(server, k8sStore, haproxyCerts) {
		annValue, _ := k8sStore.GetValueFromAnnotations(a.GetName(), annotations...)
		if annValue == nil {
			continue
		}
		reload = HandleAnnotation(a, *annValue, forceParse) || reload
	}
	return reload
}

func HandleAnnotation(a Annotation, value store.StringW, forceParse bool) (updated bool) {
	err := a.Parse(value, forceParse)
	if err != nil {
		if err != ErrEmptyStatus {
			logger.Error(err)
		}
		return false
	}
	err = a.Update()
	if err != nil {
		logger.Error(err)
		return false
	}
	return true
}

func GetGlobalAnnotations(client api.HAProxyClient) []Annotation {
	return []Annotation{
		NewGlobalCfgSnippet("global-config-snippet", client),
		NewGlobalSyslogServers("syslog-server", client),
		NewGlobalNbthread("nbthread", client),
		NewGlobalMaxconn("maxconn", client),
		NewGlobalHardStopAfter("hard-stop-after", client),
		NewDefaultOption("http-server-close", client),
		NewDefaultOption("http-keep-alive", client),
		NewDefaultOption("dontlognull", client),
		NewDefaultOption("logasap", client),
		NewDefaultTimeout("timeout-http-request", client),
		NewDefaultTimeout("timeout-connect", client),
		NewDefaultTimeout("timeout-client", client),
		NewDefaultTimeout("timeout-client-fin", client),
		NewDefaultTimeout("timeout-queue", client),
		NewDefaultTimeout("timeout-server", client),
		NewDefaultTimeout("timeout-server-fin", client),
		NewDefaultTimeout("timeout-tunnel", client),
		NewDefaultTimeout("timeout-http-keep-alive", client),
		NewDefaultLogFormat("log-format", client),
	}
}

func GetBackendAnnotations(client api.HAProxyClient, b *models.Backend) []Annotation {
	annotations := []Annotation{
		NewBackendCfgSnippet("backend-config-snippet", client, b),
		NewBackendAbortOnClose("abortonclose", b),
		NewBackendTimeoutCheck("check-timeout", b),
		NewBackendLoadBalance("load-balance", b),
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
