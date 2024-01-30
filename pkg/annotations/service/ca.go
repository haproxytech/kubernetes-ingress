package service

import (
	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type CA struct {
	haproxyCerts certs.Certificates
	backend      *models.Backend
	name         string
}

func NewCA(n string, c certs.Certificates, b *models.Backend) *CA {
	return &CA{
		name:         n,
		haproxyCerts: c,
		backend:      b,
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
		if a.backend.DefaultServer != nil {
			a.backend.DefaultServer.SslCafile = ""
			// Other values from serverSSL annotation are kept
		}
		return nil
	}
	caFile, err = a.haproxyCerts.AddSecret(secret, certs.CA_CERT)
	if err != nil {
		return err
	}
	if a.backend.DefaultServer == nil {
		a.backend.DefaultServer = &models.DefaultServer{}
	}
	a.backend.DefaultServer.Ssl = "enabled"
	a.backend.DefaultServer.Alpn = "h2,http/1.1"
	a.backend.DefaultServer.Verify = "required"
	a.backend.DefaultServer.SslCafile = caFile
	return nil
}
