package rules

import (
	"errors"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
)

type ReqInspectDelay struct {
	Timeout *int64
}

func (r ReqInspectDelay) GetType() Type {
	return REQ_INSPECT_DELAY
}

func (r ReqInspectDelay) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "http" {
		return errors.New("tcp inspect-delay rule is only available in TCP frontends")
	}
	tcpRule := models.TCPRequestRule{
		Type:    "inspect-delay",
		Timeout: r.Timeout,
	}
	return client.FrontendTCPRequestRuleCreate(0, frontend.Name, tcpRule, ingressACL)
}
