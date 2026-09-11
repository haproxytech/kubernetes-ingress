package store

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func (gw *Gateway) IsValid() error {
	if len(gw.Listeners) == 0 {
		return fmt.Errorf("Gateway '%s/%s' has no listeners", gw.Namespace, gw.Name)
	}
	err := utils.Errors{}
	combinations := map[string]struct{}{}
	for _, listener := range gw.Listeners {
		hostname := ""
		if listener.Hostname != nil {
			hostname = *listener.Hostname
		}
		key := fmt.Sprintf("%s/%d/%s", hostname, listener.Port, listener.Protocol)
		if _, found := combinations[key]; found {
			err.Add(fmt.Errorf("duplicate combination hostname/port/protocol '%s' in listeners from gateway '%s/%s", key, gw.Namespace, gw.Name))
		}
		combinations[key] = struct{}{}
	}
	return err.Result()
}

// CompareTCPRoutes orders routes by creation time, then by namespaced name.
func CompareTCPRoutes(a, b TCPRoute) int {
	if c := a.CreationTime.Compare(b.CreationTime); c != 0 {
		return c
	}
	return strings.Compare(a.Namespace+a.Name, b.Namespace+b.Name)
}
