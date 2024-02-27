package handler

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type PrometheusEndpoint struct {
	EventChan chan k8s.SyncDataEvent
	PodNs     string
}

//nolint:golint, stylecheck
const (
	PROMETHEUS_BACKEND_NAME = "haproxy-controller_prometheus_http"
	PROMETHEUS_URL_PATH     = "/metrics"
)

func (handler PrometheusEndpoint) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	if handler.PodNs == "" {
		return
	}

	status := store.EMPTY
	var secret *store.Secret
	_, errBackend := h.BackendGet(PROMETHEUS_BACKEND_NAME)
	backendExists := errBackend == nil

	annSecret := annotations.String("prometheus-endpoint-auth-secret", k.ConfigMaps.Main.Annotations)
	var secretExists, secretChanged, userListChanged bool
	userListExists, _ := h.UserListExistsByGroup("haproxy-controller-prometheus")

	// Does the secret exist in store ? ...
	if annSecret != "" {
		secretFQN := strings.Split(annSecret, "/")
		if len(secretFQN) == 2 {
			ns := k.Namespaces[secretFQN[0]]
			if ns != nil {
				secret = ns.Secret[secretFQN[1]]
				secretExists = secret != nil && secret.Status != store.DELETED
				secretChanged = secret.Status == store.MODIFIED
			}
		}
		userListChanged = secretChanged || !userListExists
	} else {
		userListChanged = userListExists
	}

	if !backendExists {
		status = store.ADDED
	}

	if !userListChanged && status == store.EMPTY && (!secretExists || (secretExists && secret.Status == store.EMPTY)) {
		return
	}

	prometheusSvcName := "prometheus"
	svc := &store.Service{
		Namespace:   handler.PodNs,
		Name:        prometheusSvcName,
		Status:      status,
		Annotations: k.ConfigMaps.Main.Annotations,
		Ports: []store.ServicePort{
			{
				Name:     "http",
				Protocol: "http",
				Port:     8765,
				Status:   status,
			},
		},
		Faked: true,
	}
	endpoints := &store.Endpoints{
		Namespace: handler.PodNs,
		Service:   prometheusSvcName,
		SliceName: prometheusSvcName,
		Status:    status,
		Ports: map[string]*store.PortEndpoints{
			"http": {
				Port:      int64(h.Env.ControllerPort),
				Addresses: map[string]struct{}{"127.0.0.1": {}},
			},
		},
	}

	if status != store.EMPTY {
		k.EventService(k.GetNamespace(svc.Namespace), svc)
		k.EventEndpoints(k.GetNamespace(endpoints.Namespace), endpoints, func(*store.RuntimeBackend, bool) error { return nil })
	}

	ing := &store.Ingress{
		Status: status,
		Faked:  true,
		IngressCore: store.IngressCore{
			Namespace: handler.PodNs,
			Name:      "prometheus",
			Rules: map[string]*store.IngressRule{"": {
				Paths: map[string]*store.IngressPath{
					PROMETHEUS_URL_PATH: {
						SvcNamespace:  svc.Namespace,
						SvcName:       svc.Name,
						Path:          PROMETHEUS_URL_PATH,
						SvcPortString: "http",
						PathTypeMatch: store.PATH_TYPE_IMPLEMENTATION_SPECIFIC,
					},
				},
			}},
		},
	}

	if secretExists {
		ing.Annotations = map[string]string{
			"auth-type":   "basic-auth",
			"auth-secret": annSecret,
		}
	}

	if userListChanged || status != store.EMPTY || secretExists && secret.Status != store.EMPTY {
		k.EventIngress(k.GetNamespace(ing.Namespace), ing)
	}

	instance.ReloadIf(status != store.EMPTY, "creation/modification of prometheus endpoint")

	return nil
}
