package service

import (
	"errors"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Crt struct {
	name         string
	haproxyCerts *haproxy.Certificates
	k8sStore     store.K8s
	server       *models.Server
}

func NewCrt(n string, k store.K8s, c *haproxy.Certificates, s *models.Server) *Crt {
	return &Crt{
		name:         n,
		k8sStore:     k,
		haproxyCerts: c,
		server:       s,
	}
}

func (a *Crt) GetName() string {
	return a.name
}

func (a *Crt) Process(input string) error {
	if input == "" {
		a.server.SslCertificate = ""
		// Other values from serverSSL annotation are kept
		return nil
	}
	crtFile, err := a.haproxyCerts.HandleTLSSecret(a.k8sStore, haproxy.SecretCtx{
		DefaultNS:  a.server.Namespace,
		SecretPath: input,
		SecretType: haproxy.BD_CERT,
	})
	if err != nil && !errors.Is(err, haproxy.ErrCertNotFound) {
		return err
	}
	a.server.Ssl = "enabled"
	a.server.Alpn = "h2,http/1.1"
	a.server.Verify = "none"
	a.server.SslCertificate = crtFile
	return nil
}
