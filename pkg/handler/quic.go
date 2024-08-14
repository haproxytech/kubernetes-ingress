package handler

import (
	"fmt"

	"github.com/haproxytech/client-native/v5/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

const (
	QUIC4BIND = "quicv4"
	QUIC6BIND = "quicv6"
)

type Quic struct {
	AddrIPv4         string
	AddrIPv6         string
	CertDir          string
	MaxAge           string
	QuicAnnouncePort int64
	QuicBindPort     int64
	IPv4             bool
	IPv6             bool
}

func (q *Quic) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	var errs utils.Errors
	defer func() {
		err = errs.Result()
	}()
	var bindv4Present, bindv6Present bool
	binds, errBindsGet := h.FrontendBindsGet(h.FrontHTTPS)
	if errBindsGet != nil {
		errs.Add(errBindsGet)
		return
	}

	for _, bind := range binds {
		bindv4Present = bindv4Present || bind.Name == QUIC4BIND
		bindv6Present = bindv6Present || bind.Name == QUIC6BIND
	}

	ipv4Func := func() {
		if bindv4Present {
			return
		}

		errFrontendBindCreate := h.FrontendBindCreate(h.FrontHTTPS, models.Bind{
			Address: func() (addr string) {
				addr = "quic4@" + q.AddrIPv4
				return
			}(),
			Port: utils.PtrInt64(q.QuicBindPort),
			BindParams: models.BindParams{
				Name:           QUIC4BIND,
				Ssl:            true,
				SslCertificate: q.CertDir,
				Alpn:           "h3",
			},
		})
		errs.Add(errFrontendBindCreate)
		instance.ReloadIf(errFrontendBindCreate == nil, "quic binding v4 created")
	}

	ipv6Func := func() {
		if bindv6Present {
			return
		}
		errFrontendBindCreate := h.FrontendBindCreate(h.FrontHTTPS, models.Bind{
			Address: func() (addr string) {
				addr = "quic6@" + q.AddrIPv6
				return
			}(),
			Port: utils.PtrInt64(q.QuicBindPort),
			BindParams: models.BindParams{
				Name:           QUIC6BIND,
				Ssl:            true,
				SslCertificate: q.CertDir,
				Alpn:           "h3",
			},
		})
		errs.Add(errFrontendBindCreate)
		instance.ReloadIf(errFrontendBindCreate == nil, "quic binding v6 created")
	}

	ipv4DeleteFunc := func() {
		if !bindv4Present {
			return
		}
		errFrontendBindDelete := h.FrontendBindDelete(h.FrontHTTPS, QUIC4BIND)
		errs.Add(errFrontendBindDelete)
		instance.ReloadIf(errFrontendBindDelete == nil, "quic binding v4 removed")
	}

	ipv6DeleteFunc := func() {
		if !bindv6Present {
			return
		}
		errFrontendBindDelete := h.FrontendBindDelete(h.FrontHTTPS, QUIC6BIND)
		errs.Add(errFrontendBindDelete)
		instance.ReloadIf(errFrontendBindDelete == nil, "quic binding v6 removed")
	}

	maxAge := common.GetValue("quic-alt-svc-max-age", k.ConfigMaps.Main.Annotations)
	updatedMaxAge := maxAge != q.MaxAge
	if updatedMaxAge {
		instance.Reload("quic max age updated from %s to %s", q.MaxAge, maxAge)
		q.MaxAge = maxAge
	}

	nsSslCertificateAnn, nameSslCertificateAnn, err := common.GetK8sPath("ssl-certificate", k.ConfigMaps.Main.Annotations)
	if err != nil || (nameSslCertificateAnn == "") {
		errs.Add(err)
		ipv4Func = ipv4DeleteFunc
		ipv6Func = ipv6DeleteFunc
	} else {
		namespaceSslCertificate := k.Namespaces[nsSslCertificateAnn]
		var sslSecret *store.Secret
		if namespaceSslCertificate != nil {
			sslSecret = namespaceSslCertificate.Secret[nameSslCertificateAnn]
		}

		if sslSecret == nil || sslSecret.Status == store.DELETED {
			ipv4Func = ipv4DeleteFunc
			ipv6Func = ipv6DeleteFunc
		} else {
			logger.Debug("quic redirect rule to be created")
			errs.Add(h.AddRule(h.FrontHTTPS, rules.RequestRedirectQuic{}, false))
			logger.Debug("quic set header rule to be created")
			errs.Add(h.AddRule(h.FrontHTTPS, rules.SetHdr{
				HdrName:   "alt-svc",
				Response:  true,
				HdrFormat: fmt.Sprintf("\"h3=\\\":%d\\\";ma="+maxAge+";\"", q.QuicAnnouncePort),
			}, false))
		}
	}

	if q.IPv4 {
		ipv4Func()
	}

	if q.IPv6 {
		ipv6Func()
	}

	return
}
