package service

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type CA struct {
	name         string
	haproxyCerts *haproxy.Certificates
	server       *models.Server
}

func NewCA(n string, c *haproxy.Certificates, s *models.Server) *CA {
	return &CA{
		name:         n,
		haproxyCerts: c,
		server:       s,
	}
}

func (a *CA) GetName() string {
	return a.name
}

func (a *CA) Process(k store.K8s, annotations ...map[string]string) error {
	var secret *store.Secret
	var caFile string
	ns, name, err := common.GetK8sPath(a.name, annotations...)
	if err != nil {
		return err
	}
	secret, _ = k.GetSecret(ns, name)
	if secret == nil {
		a.server.SslCafile = ""
		// Other values from serverSSL annotation are kept
		return nil
	}
	caFile, err = a.haproxyCerts.HandleTLSSecret(secret, haproxy.CA_CERT)
	if err != nil {
		return err
	}
	a.server.Ssl = "enabled"
	a.server.Alpn = "h2,http/1.1"
	a.server.Verify = "required"
	a.server.SslCafile = caFile
	return nil
}
