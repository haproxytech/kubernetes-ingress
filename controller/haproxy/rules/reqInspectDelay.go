package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqInspectDelay struct {
	Timeout *int64
}

func (r ReqInspectDelay) GetType() haproxy.RuleType {
	return haproxy.REQ_INSPECT_DELAY
}

func (r ReqInspectDelay) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	if frontend.Mode == "http" {
		return fmt.Errorf("tcp inspect-delay rule is only available in TCP frontends")
	}
	tcpRule := models.TCPRequestRule{
		Type:    "inspect-delay",
		Index:   utils.PtrInt64(0),
		Timeout: r.Timeout,
	}
	return client.FrontendTCPRequestRuleCreate(frontend.Name, tcpRule)
}
