package service

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Crt struct {
	name         string
	haproxyCerts *haproxy.Certificates
	server       *models.Server
}

func NewCrt(n string, c *haproxy.Certificates, s *models.Server) *Crt {
	return &Crt{
		name:         n,
		haproxyCerts: c,
		server:       s,
	}
}

func (a *Crt) GetName() string {
	return a.name
}

func (a *Crt) Process(k store.K8s, annotations ...map[string]string) error {
	var secret *store.Secret
	var crtFile string
	ns, name, err := common.GetK8sPath(a.name, annotations...)
	if err != nil {
		return err
	}
	secret, _ = k.GetSecret(ns, name)
	if secret == nil {
		a.server.SslCertificate = ""
		// Other values from serverSSL annotation are kept
		return nil
	}
	crtFile, err = a.haproxyCerts.HandleTLSSecret(secret, haproxy.BD_CERT)
	if err != nil {
		return err
	}
	a.server.Ssl = "enabled"
	a.server.Alpn = "h2,http/1.1"
	a.server.Verify = "none"
	a.server.SslCertificate = crtFile
	return nil
}
