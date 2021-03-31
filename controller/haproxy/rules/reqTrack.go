package rules

import (
	"fmt"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type ReqTrack struct {
	TableName   string
	TablePeriod *int64
	TableSize   *int64
	TrackKey    string
}

func (r ReqTrack) GetType() haproxy.RuleType {
	return haproxy.REQ_TRACK
}

func (r ReqTrack) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return fmt.Errorf("request Track cannot be configured in TCP mode")
	}
	// Create tracking table.
	if _, err := client.BackendGet(r.TableName); err != nil {
		err = client.BackendCreate(models.Backend{
			Name: r.TableName,
			StickTable: &models.BackendStickTable{
				Peers: "localinstance",
				Type:  "ip",
				Size:  r.TableSize,
				Store: fmt.Sprintf("http_req_rate(%d)", *r.TablePeriod),
			},
		})
		if err != nil {
			return err
		}
	}
	// Create rule
	httpRule := models.HTTPRequestRule{
		Index:         utils.PtrInt64(0),
		Type:          "track-sc0",
		TrackSc0Key:   r.TrackKey,
		TrackSc0Table: r.TableName,
	}
	return client.FrontendHTTPRequestRuleCreate(frontend.Name, httpRule, ingressACL)
}
