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
	Version            []bool         `short:"v" long:"version" description:"Verbose output"`
	DefaultService     NamespaceValue `long:"default-backend-service" description:"default service to serve 404 page. If not specified HAProxy serves http 400" default:""`
	DefaultCertificate NamespaceValue `long:"default-ssl-certificate" description:"secret name of the certificate" default:"default/tls-secret"`
	ConfigMap          NamespaceValue `long:"configmap" description:"configmap designated for HAProxy" default:"default/haproxy-configmap"`
}
