package annotations

import (
	"fmt"
	"strconv"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/global"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/ingress"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

type Annotation interface {
	GetName() string
	Process(k store.K8s, annotations ...map[string]string) error
}

type Annotations interface {
	GlobalCfgSnipp() []Annotation
	Global(g *models.Global, l *models.LogTargets) []Annotation
	Defaults(d *models.Defaults) []Annotation
	Backend(b *models.Backend, s store.K8s, c certs.Certificates) []Annotation
	Frontend(i *store.Ingress, r *rules.List, m maps.Maps) []Annotation
	Secret(name, defaultNs string, k store.K8s, annotations ...map[string]string) (secret *store.Secret, err error)
	Timeout(name string, annotations ...map[string]string) (out *int64, err error)
	String(name string, annotations ...map[string]string) string
}

type annImpl struct{}

func New() Annotations { //nolint:ireturn
	return annImpl{}
}

func (a annImpl) String(name string, annotations ...map[string]string) string {
	return String(name, annotations...)
}

func (a annImpl) Secret(name, defaultNs string, k store.K8s, annotations ...map[string]string) (secret *store.Secret, err error) {
	return Secret(name, defaultNs, k, annotations...)
}

func (a annImpl) Timeout(name string, annotations ...map[string]string) (out *int64, err error) {
	return Timeout(name, annotations...)
}

func (a annImpl) GlobalCfgSnipp() []Annotation {
	return []Annotation{
		// global
		NewCfgSnippet(ConfigSnippetOptions{Name: "global-config-snippet"}),
		// frontend
		NewCfgSnippet(ConfigSnippetOptions{
			Name:     "frontend-config-snippet",
			Frontend: utils.Ptr("http"),
		},
		),
		NewCfgSnippet(ConfigSnippetOptions{
			Name:     "frontend-config-snippet",
			Frontend: utils.Ptr("https"),
		},
		),
		NewCfgSnippet(ConfigSnippetOptions{
			Name:     "stats-config-snippet",
			Frontend: utils.Ptr("stats"),
		},
		),
	}
}

func (a annImpl) Global(g *models.Global, l *models.LogTargets) []Annotation {
	return []Annotation{
		global.NewSyslogServers("syslog-server", l),
		global.NewNbthread("nbthread", g),
		global.NewMaxconn("maxconn", g),
		global.NewHardStopAfter("hard-stop-after", g),
	}
}

func (a annImpl) Defaults(d *models.Defaults) []Annotation {
	return []Annotation{
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
		global.NewHTTPConnectionMode("http-connection-mode", d),
	}
}

func (a annImpl) Frontend(i *store.Ingress, r *rules.List, m maps.Maps) []Annotation {
	reqRateLimit := ingress.NewReqRateLimit(r, m)
	httpsRedirect := ingress.NewHTTPSRedirect(r, i)
	hostRedirect := ingress.NewHostRedirect(r)
	reqAuth := ingress.NewReqAuth(r, i)
	reqCapture := ingress.NewReqCapture(r)
	resSetCORS := ingress.NewResSetCORS(r)
	return []Annotation{
		// Simple annoations
		ingress.NewDenyList("deny-list", r, m),
		ingress.NewAllowList("allow-list", r, m),
		ingress.NewSrcIPHdr("src-ip-header", r),
		ingress.NewReqSetHost("set-host", r),
		ingress.NewReqPathRewrite("path-rewrite", r),
		ingress.NewReqSetHdr("request-set-header", r),
		ingress.NewResSetHdr("response-set-header", r),
		// Annotation factory for related annotations
		httpsRedirect.NewAnnotation("ssl-redirect"),
		httpsRedirect.NewAnnotation("ssl-redirect-port"),
		httpsRedirect.NewAnnotation("ssl-redirect-code"),
		hostRedirect.NewAnnotation("request-redirect"),
		hostRedirect.NewAnnotation("request-redirect-code"),
		reqRateLimit.NewAnnotation("rate-limit-requests"),
		reqRateLimit.NewAnnotation("rate-limit-period"),
		reqRateLimit.NewAnnotation("rate-limit-size"),
		reqRateLimit.NewAnnotation("rate-limit-status-code"),
		reqRateLimit.NewAnnotation("rate-limit-whitelist"),
		reqAuth.NewAnnotation("auth-type"),
		reqAuth.NewAnnotation("auth-realm"),
		reqAuth.NewAnnotation("auth-secret"),
		reqCapture.NewAnnotation("request-capture"),
		reqCapture.NewAnnotation("request-capture-len"),
		// always put cors-enable annotation before any oth
		resSetCORS.NewAnnotation("cors-enable"),
		resSetCORS.NewAnnotation("cors-allow-origin"),
		resSetCORS.NewAnnotation("cors-allow-methods"),
		resSetCORS.NewAnnotation("cors-allow-headers"),
		resSetCORS.NewAnnotation("cors-max-age"),
		resSetCORS.NewAnnotation("cors-allow-credentials"),
		resSetCORS.NewAnnotation("cors-respond-to-options"),
	}
}

func (a annImpl) Backend(b *models.Backend, s store.K8s, c certs.Certificates) []Annotation {
	annotations := []Annotation{
		service.NewAbortOnClose("abortonclose", b),
		service.NewTimeoutCheck("timeout-check", b),
		service.NewTimeoutServer("timeout-server", b),
		service.NewLoadBalance("load-balance", b),
		service.NewCheck("check", b),
		service.NewCheckInter("check-interval", b),
		service.NewCookie("cookie-persistence", b),
		service.NewMaxconn("pod-maxconn", b),
		service.NewSendProxy("send-proxy-protocol", b),
		// Order is important for ssl annotations so they don't conflict
		service.NewSSL("server-ssl", b),
		service.NewCrt("server-crt", c, b),
		service.NewCA("server-ca", c, b),
		service.NewProto("server-proto", b),
	}
	if b.Mode == "http" {
		annotations = append(annotations,
			service.NewCheckHTTP("check-http", b),
			service.NewForwardedFor("forwarded-for", b),
		)
	}
	return annotations
}

func SetDefaultValue(annotation, value string) {
	common.DefaultValues[annotation] = value
}

func Bool(name string, annotations ...map[string]string) (out bool, err error) {
	boolean, err := ParseBool(name, annotations...)
	out = boolean == "true"
	return out, err
}

func ParseBool(name string, annotations ...map[string]string) (out string, err error) {
	input := common.GetValue(name, annotations...)
	if input == "" {
		return out, err
	}
	_, err = utils.GetBoolValue(input, name)
	if err != nil {
		err = fmt.Errorf("%s annotation: %w", name, err)
		return out, err
	}
	out = input
	return out, err
}

func Int(name string, annotations ...map[string]string) (out int, err error) {
	input := common.GetValue(name, annotations...)
	if input == "" {
		return out, err
	}
	out, err = strconv.Atoi(input)
	if err != nil {
		err = fmt.Errorf("annotation '%s': %w", name, err)
		return out, err
	}
	return out, err
}

func Secret(name, defaultNs string, k store.K8s, annotations ...map[string]string) (secret *store.Secret, err error) {
	var secNs, secName string
	secNs, secName, err = common.GetK8sPath(name, annotations...)
	if err != nil {
		err = fmt.Errorf("annotation '%s': %w", name, err)
		return secret, err
	}
	if secName == "" {
		return secret, err
	}
	if secNs == "" {
		secNs = defaultNs
	}
	secret, err = k.GetSecret(secNs, secName)
	if err != nil {
		err = fmt.Errorf("annotation '%s': %w", name, err)
		return secret, err
	}
	return secret, err
}

func String(name string, annotations ...map[string]string) string {
	return common.GetValue(name, annotations...)
}

func Timeout(name string, annotations ...map[string]string) (out *int64, err error) {
	input := common.GetValue(name, annotations...)
	if input == "" {
		return out, err
	}
	out, err = utils.ParseTime(input)
	if err != nil {
		err = fmt.Errorf("annotation '%s': %w", name, err)
		return out, err
	}
	return out, err
}

// SpecificAnnotations is a set of annotations that uses rules to produce specific configuration with rule ID in configuration file.
// These annotations in an ingress can't be merged with other ingresses annotations when these ingresses point to the same service because specific paths must be treated specifically.
var SpecificAnnotations = map[string]struct{}{
	"backend-config-snippet":  {},
	"deny-list":               {},
	"blacklist":               {},
	"allow-list":              {},
	"whitelist":               {},
	"src-ip-header":           {},
	"auth-type":               {},
	"auth-realm":              {},
	"auth-secret":             {},
	"ssl-redirect":            {},
	"ssl-redirect-port":       {},
	"ssl-redirect-code":       {},
	"request-redirect":        {},
	"request-redirect-code":   {},
	"request-capture":         {},
	"request-capture-len":     {},
	"path-rewrite":            {},
	"rate-limit-requests":     {},
	"rate-limit-period":       {},
	"rate-limit-size":         {},
	"rate-limit-status-code":  {},
	"rate-limit-whitelist":    {},
	"request-set-header":      {},
	"response-set-header":     {},
	"set-host":                {},
	"cors-enable":             {},
	"cors-allow-origin":       {},
	"cors-allow-methods":      {},
	"cors-allow-headers":      {},
	"cors-max-age":            {},
	"cors-allow-credentials":  {},
	"cors-respond-to-options": {},
}
