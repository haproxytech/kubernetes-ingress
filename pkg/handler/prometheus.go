package handler

import (
	"fmt"
	"strings"
	"sync"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
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
	errs := utils.Errors{}

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

	var secret *store.Secret
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
			salt, ok := cryptSalt(string(password))
			if !ok {
				// Skipping the user leaves the endpoint closed for them, which is the only
				// safe outcome: a value that is not a SHA-256 crypt hash can never match
				// what crypt.Generate computes, so it would authenticate nobody either way.
				errs.Add(fmt.Errorf("prometheus user '%s' in secret '%s': password is not a SHA-256 crypt hash (expected '%s...', as produced by `mkpasswd -m SHA-256`), user skipped", user, annSecret, sha256CryptPrefix))
				continue
			}
			prometheusUsers[user] = prometheusAuthUser{
				Password: string(password),
				Salt:     salt,
			}
			logger.Debugf("Adding prometheus user '%s' from secret '%s'", user, annSecret)
		}
		prometheusUsersActive = true
		prometheusMu.Unlock()
	}
	return errs.Result()
}

// sha256CryptPrefix identifies a SHA-256 crypt hash. It is the only algorithm the
// endpoint can verify, since prometheusHandler hashes with crypt.SHA256, so a hash
// carrying any other identifier is refused here rather than at the first request.
const sha256CryptPrefix = "$5$"

// cryptSalt returns the salt that crypt.Generate expects from a hash produced by
// `mkpasswd -m SHA-256`: the hash with its last '$' and everything after it removed.
//
// Cutting at the last '$' rather than at a fixed field index is what keeps an optional
// "rounds=" parameter inside the salt, where crypt needs it. The trailing '$' has to go:
// crypt tolerates it on a plain "$5$salt$" but folds it into the salt of a
// "$5$rounds=N$salt$", which then hashes to something the stored hash never matches.
//
// Requiring the identifier, and not just a leading '$', is what separates a hash from a
// plaintext password that happens to contain '$' - "$ecret$pass$word" is shaped like a
// hash and would otherwise be kept with a salt read out of it, locking the user out with
// nothing in the logs to say why.
func cryptSalt(password string) (salt string, ok bool) {
	if !strings.HasPrefix(password, sha256CryptPrefix) || strings.Count(password, "$") < 3 {
		return "", false
	}
	return password[:strings.LastIndex(password, "$")], true
}
