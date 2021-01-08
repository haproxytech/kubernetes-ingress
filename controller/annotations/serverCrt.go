package annotations

import (
	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type ServerCrt struct {
	name         string
	haproxyCerts *haproxy.Certificates
	k8sStore     store.K8s
	crtFile      string
	server       *models.Server
}

func NewServerCrt(n string, k store.K8s, c *haproxy.Certificates, s *models.Server) *ServerCrt {
	return &ServerCrt{
		name:         n,
		k8sStore:     k,
		haproxyCerts: c,
		server:       s,
	}
}

func (a *ServerCrt) GetName() string {
	return a.name
}

func (a *ServerCrt) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.DELETED {
		return nil
	}
	crtFile, updated, err := a.haproxyCerts.HandleTLSSecret(a.k8sStore, haproxy.SecretCtx{
		DefaultNS:  a.server.Namespace,
		SecretPath: input.Value,
		SecretType: haproxy.BD_CERT,
	})
	if err != nil && err != haproxy.ErrCertNotFound {
		return err
	}
	if input.Status == store.EMPTY && !updated && !forceParse {
		return ErrEmptyStatus
	}
	a.crtFile = crtFile
	return nil
}

func (a *ServerCrt) Update() error {
	if a.crtFile == "" {
		a.server.SslCertificate = ""
		return nil
	}
	a.server.Ssl = "enabled"
	a.server.Alpn = "h2,http/1.1"
	a.server.Verify = "none"
	a.server.SslCertificate = a.crtFile
	return nil
}
