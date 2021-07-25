package status

import (
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	corev1 "k8s.io/api/core/v1"
)

var logger = utils.GetLogger()

type SyncIngress struct {
	Service *corev1.Service
	Ingress *store.Ingress
}
