package ingress

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

const CONTROLLER = "haproxy.org/ingress-controller"

var logger = utils.GetLogger()

type Sync struct {
	Service *corev1.Service
	Ingress *store.Ingress
}
