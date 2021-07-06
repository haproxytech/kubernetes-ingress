package annotations

import (
	"errors"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type ServerCA struct {
	name         string
	haproxyCerts *haproxy.Certificates
	k8sStore     store.K8s
	caFile       string
	server       *models.Server
}

func NewServerCA(n string, k store.K8s, c *haproxy.Certificates, s *models.Server) *ServerCA {
	return &ServerCA{
		name:         n,
		k8sStore:     k,
		haproxyCerts: c,
		server:       s,
	}
}

func (a *ServerCA) GetName() string {
	return a.name
}

func (a *ServerCA) Parse(input string) error {
	caFile, err := a.haproxyCerts.HandleTLSSecret(a.k8sStore, haproxy.SecretCtx{
		DefaultNS:  a.server.Namespace,
		SecretPath: input,
		SecretType: haproxy.CA_CERT,
	})
	if err != nil && !errors.Is(err, haproxy.ErrCertNotFound) {
		return err
	}
	a.caFile = caFile
	return nil
}

func (a *ServerCA) Update() error {
	if a.caFile == "" {
		a.server.SslCafile = ""
		return nil
	}
	a.server.Ssl = "enabled"
	a.server.Alpn = "h2,http/1.1"
	a.server.Verify = "required"
	a.server.SslCafile = a.caFile
	return nil
}
