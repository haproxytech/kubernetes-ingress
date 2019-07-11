package main

import (
	"errors"
	"fmt"
	"strings"
)

//NamespaceValue used to automatically distinct namespace/name string
type NamespaceValue struct {
	Namespace, Name string
}

//UnmarshalFlag Unmarshal flag
func (n *NamespaceValue) UnmarshalFlag(value string) error {
	parts := strings.Split(value, "/")

	if len(parts) != 2 {
		return errors.New("expected two strings separated by a /")
	}
	n.Namespace = parts[0]
	n.Name = parts[1]
	return nil
}

//MarshalFlag Marshals flag
func (n NamespaceValue) MarshalFlag() (string, error) {
	return fmt.Sprintf("%s/%s", n.Namespace, n.Name), nil
}

//OSArgs contains arguments that can be sent to controller
type OSArgs struct {
	Version               []bool         `short:"v" long:"version" description:"version"`
	DefaultBackendService NamespaceValue `long:"default-backend-service" default:"" description:"default service to serve 404 page. If not specified HAProxy serves http 400"`
	DefaultCertificate    NamespaceValue `long:"default-ssl-certificate" default:"" description:"secret name of the certificate"`
	ConfigMap             NamespaceValue `long:"configmap" description:"configmap designated for HAProxy" default:"default/haproxy-configmap"`
	KubeConfig            string         `long:"kubeconfig" default:"" description:"combined with -e. location of kube config file"`
	NamespaceWhitelist    []string       `long:"namespace-whitelist" description:"whitelisted namespaces"`
	NamespaceBlacklist    []string       `long:"namespace-blacklist" description:"blacklisted namespaces"`
	OutOfCluster          bool           `short:"e" description:"use as out of cluster controller NOTE: experimantal"`
	Test                  bool           `short:"t" description:"simulate running HAProxy"`
	Help                  []bool         `short:"h" long:"help" description:"show this help message"`
	IngressClass          string         `long:"ingress.class" default:"haproxy" description:"ingress.class to monitor in multiple controllers environment"`
}
