package store

import (
	"fmt"

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

func (tcproutes TCPRoutes) Less(i, j int) bool {
	tcprouteI := tcproutes[i]
	tcprouteJ := tcproutes[j]
	if !tcprouteI.CreationTime.Equal(tcprouteJ.CreationTime) {
		return tcprouteI.CreationTime.Before(tcprouteJ.CreationTime)
	}
	return tcprouteI.Namespace+tcprouteI.Name < tcprouteJ.Namespace+tcprouteJ.Name
}
