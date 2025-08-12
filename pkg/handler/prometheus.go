package handler

import (
	"fmt"
	"strings"
	"sync"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type PrometheusEndpoint struct {
	EventChan chan k8ssync.SyncDataEvent
	PodNs     string
}

var (
	prometheusUsers       map[string]prometheusAuthUser
	prometheusUsersActive bool
	prometheusMu          sync.RWMutex
)

//nolint:golint, stylecheck
const (
	PROMETHEUS_URL_PATH     = "/metrics"
	PROMETHEUS_SERVICE_NAME = "prometheus"
)

type prometheusAuthUser struct {
	Password string
	Salt     string
}

func PrometheusAuthUsers() map[string]prometheusAuthUser {
	prometheusMu.RLock()
	defer prometheusMu.RUnlock()
	users := make(map[string]prometheusAuthUser, len(prometheusUsers))
	for user, passwordData := range prometheusUsers {
		users[user] = prometheusAuthUser{
			Password: passwordData.Password,
			Salt:     passwordData.Salt,
		}
	}
	return users
}

func PrometheusAuthActive() bool {
	prometheusMu.RLock()
	defer prometheusMu.RUnlock()
	return prometheusUsersActive
}

func (handler PrometheusEndpoint) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	if handler.PodNs == "" {
		return nil
	}

	var secret *store.Secret

	annSecret := annotations.String("prometheus-endpoint-auth-secret", k.ConfigMaps.Main.Annotations)
	prometheusMu.RLock()
	prometheusUsersActiveLocal := prometheusUsersActive
	prometheusMu.RUnlock()

	if annSecret != "" && !prometheusUsersActiveLocal {
		prometheusMu.Lock()
		prometheusUsersActive = true
		prometheusMu.Unlock()
	} else if annSecret == "" && prometheusUsersActiveLocal {
		prometheusMu.Lock()
		prometheusUsersActive = false
		prometheusUsers = nil
		prometheusMu.Unlock()
	}
	if annSecret == "" {
		return nil
	}

	var secretExists bool
	// Does the secret exist in store ? ...
	if annSecret != "" {
		secretFQN := strings.Split(annSecret, "/")
		if len(secretFQN) == 2 {
			ns := k.Namespaces[secretFQN[0]]
			if ns != nil {
				secret = ns.Secret[secretFQN[1]]
				secretExists = secret != nil && secret.Status != store.DELETED
			}
		}
	}

	if secretExists {
		// first see if we need to do something
		prometheusMu.RLock()
		// prometheusUsers != nil, not len, there is a diff in logic
		if secret.Status == store.EMPTY && prometheusUsers != nil {
			prometheusMu.RUnlock()
			return nil
		}
		prometheusMu.RUnlock()

		// then fill users if needed
		prometheusMu.Lock()
		prometheusUsers = make(map[string]prometheusAuthUser)
		for user, password := range secret.Data {
			partsPass := strings.Split(string(password), "$")
			salt := fmt.Sprintf("$%s$%s$", partsPass[1], partsPass[2])
			prometheusUsers[user] = prometheusAuthUser{
				Password: string(password),
				Salt:     salt,
			}
			logger.Debugf("Adding prometheus user '%s' from secret '%s'", user, annSecret)
		}
		prometheusUsersActive = true
		prometheusMu.Unlock()
	}
	return nil
}
