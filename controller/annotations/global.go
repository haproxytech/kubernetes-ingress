package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func HandleGlobalAnnotations(k8sStore store.K8s, client api.HAProxyClient, global *models.Global, defaults *models.Defaults, forcePase bool, annotations store.MapStringW) (restart bool, reload bool) {
	annList := GetGlobalAnnotations(client, global, defaults)
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
