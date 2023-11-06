package utils

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// NamespaceValue used to automatically distinct namespace/name string
type NamespaceValue struct {
	Namespace, Name string
}

// UnmarshalFlag Unmarshal flag
func (n *NamespaceValue) UnmarshalFlag(value string) error {
	parts := strings.Split(value, "/")

	if len(parts) != 2 {
		return errors.New("expected two strings separated by a /")
	}
	n.Namespace = parts[0]
	n.Name = parts[1]
	return nil
}

// MarshalFlag Marshals flag
func (n NamespaceValue) MarshalFlag() (string, error) {
	return fmt.Sprintf("%s/%s", n.Namespace, n.Name), nil
}

func (n NamespaceValue) String() string {
	if n.Namespace == "" || n.Name == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", n.Namespace, n.Name)
}

// LogLevel used to automatically distinct namespace/name string
type LogLevelValue struct {
	LogLevel LogLevel
}

// UnmarshalFlag Unmarshal flag
func (n *LogLevelValue) UnmarshalFlag(value string) error {
	switch value {
	case "trace":
		n.LogLevel = Trace
		return nil
	case "debug":
		n.LogLevel = Debug
		return nil
	case "info":
		n.LogLevel = Info
		return nil
	case "warning":
		n.LogLevel = Warning
		return nil
	case "error":
		n.LogLevel = Error
		return nil
	}

	return fmt.Errorf("value %s not permitted", value)
}

// OSArgs contains arguments that can be sent to controller
type OSArgs struct {
	ConfigMapPatternFiles      NamespaceValue `long:"configmap-patternfiles" description:"configmap used to provide a list of pattern files to use in haproxy configuration " default:""`
	ConfigMapTCPServices       NamespaceValue `long:"configmap-tcp-services" description:"configmap used to define tcp services" default:""`
	DefaultBackendService      NamespaceValue `long:"default-backend-service" default:"" description:"default service to serve 404 page. If not specified HAProxy serves http 400"`
	ConfigMapErrorFiles        NamespaceValue `long:"configmap-errorfiles" description:"configmap used to define custom error pages associated to HTTP error codes" default:""`
	DefaultCertificate         NamespaceValue `long:"default-ssl-certificate" default:"" description:"secret name of the certificate"`
	ConfigMap                  NamespaceValue `long:"configmap" description:"configmap designated for HAProxy" default:""`
	IPV6BindAddr               string         `long:"ipv6-bind-address" default:"::" description:"IPv6 address the Ingress Controller listens on (if enabled)"`
	GatewayControllerName      string         `long:"gateway-controller-name" description:"sets the controller name of gatewayclass managed by the controller"`
	IPV4BindAddr               string         `long:"ipv4-bind-address" default:"0.0.0.0" description:"IPv4 address the Ingress Controller listens on (if enabled)"`
	RuntimeDir                 string         `long:"runtime-dir" description:"path to HAProxy runtime directory. NOTE: works only in External mode"`
	IngressClass               string         `long:"ingress.class" default:"" description:"ingress.class to monitor in multiple controllers environment"`
	PublishService             string         `long:"publish-service" default:"" description:"Takes the form namespace/name. The controller mirrors the address of this service's endpoints to the load-balancer status of all Ingress objects it satisfies"`
	CfgDir                     string         `long:"config-dir" description:"path to HAProxy configuration directory. NOTE: works only in External mode"`
	Program                    string         `long:"program" description:"path to HAProxy program. NOTE: works only with External mode"`
	KubeConfig                 string         `long:"kubeconfig" default:"" description:"combined with -e. location of kube config file"`
	Version                    []bool         `short:"v" long:"version" description:"version"`
	NamespaceWhitelist         []string       `long:"namespace-whitelist" description:"whitelisted namespaces"`
	NamespaceBlacklist         []string       `long:"namespace-blacklist" description:"blacklisted namespaces"`
	Help                       []bool         `short:"h" long:"help" description:"show this help message"`
	LocalPeerPort              int64          `long:"localpeer-port" default:"10000" description:"port to listen on for local peer"`
	StatsBindPort              int64          `long:"stats-bind-port" default:"1024" description:"port to listen on for stats page"`
	DefaultBackendPort         int            `long:"default-backend-port" description:"port to use for default service" default:"6061"`
	ChannelSize                int64          `long:"channel-size" description:"sets the size of controller buffers used to receive and send k8s events.NOTE: increase the value to accommodate large number of resources "`
	ControllerPort             int            `long:"controller-port" description:"port to listen on for controller data: prometheus, pprof"`
	HTTPBindPort               int64          `long:"http-bind-port" default:"80" description:"port to listen on for HTTP traffic"`
	HTTPSBindPort              int64          `long:"https-bind-port" default:"443" description:"port to listen on for HTTPS traffic"`
	SyncPeriod                 time.Duration  `long:"sync-period" default:"5s" description:"Sets the period at which the controller syncs HAProxy configuration file"`
	CacheResyncPeriod          time.Duration  `long:"cache-resync-period" default:"10m" description:"Sets the underlying Shared Informer resync period: resyncing controller with informers cache"`
	HealthzBindPort            int64          `long:"healthz-bind-port" default:"1042" description:"port to listen on for probes"`
	LogLevel                   LogLevelValue  `long:"log" default:"info" description:"level of log messages you can see"`
	DisableIPV4                bool           `long:"disable-ipv4" description:"toggle to disable the IPv4 protocol from all frontends"`
	External                   bool           `short:"e" long:"external" description:"use as external Ingress Controller (out of k8s cluster)"`
	Test                       bool           `short:"t" description:"simulate running HAProxy"`
	EmptyIngressClass          bool           `long:"empty-ingress-class" description:"empty-ingress-class manages the behavior in case an ingress has no explicit ingress class annotation. true: to process, false: to skip"`
	DisableServiceExternalName bool           `long:"disable-service-external-name" description:"disable forwarding to ExternalName Services due to CVE-2021-25740"`
	UseWiths6Overlay           bool           `long:"with-s6-overlay" description:"use s6 overlay to start/stpop/reload HAProxy"`
	DisableHTTPS               bool           `long:"disable-https" description:"toggle to disable the HTTPs frontend"`
	PprofEnabled               bool           `long:"pprof" short:"p" description:"enable pprof"`
	PrometheusEnabled          bool           `long:"prometheus" description:"enable prometheus of IC data"`
	DisableHTTP                bool           `long:"disable-http" description:"toggle to disable the HTTP frontend"`
	DisableIPV6                bool           `long:"disable-ipv6" description:"toggle to disable the IPv6 protocol from all frontends"`
	DisableConfigSnippets      string         `long:"disable-config-snippets" description:"Allow to disable config snippets. List of comma separated values (possible values: all/global/backend/frontend)"`
	UseWithPebble              bool           `long:"with-pebble" description:"use pebble to start/stop/reload HAProxy"`
	JobCheckCRD                bool           `long:"job-check-crd" description:"does not execute IC, but adds/updates CRDs"`
}
