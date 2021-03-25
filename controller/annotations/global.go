package annotations

import (
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

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
	if a, ok := annList[3].(*GlobalSyslogServers); ok {
		restart = a.Restart()
	}
	return restart, reload
}

func GetGlobalAnnotations(client api.HAProxyClient) []Annotation {
	return []Annotation{
		NewFrontendCfgSnippet("frontend-config-snippet", client, []string{"http", "https"}),
		NewFrontendCfgSnippet("stats-config-snippet", client, []string{"stats"}),
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
