package annotations

import (
	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func HandleServerAnnotations(k8sStore store.K8s, client api.HAProxyClient, haproxyCerts *haproxy.Certificates, server *models.Server, forceParse bool, annotations ...store.MapStringW) (reload bool) {
	for _, a := range GetServerAnnotations(server, k8sStore, haproxyCerts) {
		annValue, _ := k8sStore.GetValueFromAnnotations(a.GetName(), annotations...)
		if annValue == nil {
			continue
		}
		reload = HandleAnnotation(a, *annValue, forceParse) || reload
	}
	return reload
}

func GetServerAnnotations(s *models.Server, k8sStore store.K8s, certs *haproxy.Certificates) []Annotation {
	return []Annotation{
		NewServerCheck("check", s),
		NewServerCheckInter("check-interval", s),
		NewServerCookie("cookie-persistence", s),
		NewServerMaxconn("pod-maxconn", s),
		NewServerSendProxy("send-proxy-protocol", s),
		// Order is important for ssl annotations so they don't conflict
		NewServerSSL("server-ssl", s),
		NewServerCrt("server-crt", k8sStore, certs, s),
		NewServerCA("server-ca", k8sStore, certs, s),
		NewServerProto("server-proto", s),
	}
}
