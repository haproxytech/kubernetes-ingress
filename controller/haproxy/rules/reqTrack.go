package rules

import (
	"fmt"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type ReqTrack struct {
	Ingress   haproxy.MapID
	TableName string
	TrackKey  string
}

func (r ReqTrack) GetType() haproxy.RuleType {
	return haproxy.REQ_TRACK
}

func (r ReqTrack) Create(client api.HAProxyClient, frontend *models.Frontend) error {
	if frontend.Mode == "tcp" {
		//TODO: tcp request tracking
		return fmt.Errorf("request Track cannot be configured in TCP mode")
	}
	ingressMapFile := r.Ingress.Path()
	httpRule := models.HTTPRequestRule{
		Index:         utils.PtrInt64(0),
		Type:          "track-sc0",
		TrackSc0Key:   r.TrackKey,
		TrackSc0Table: r.TableName,
		Cond:          "if",
		CondTest:      makeACL("", ingressMapFile),
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule)
}
