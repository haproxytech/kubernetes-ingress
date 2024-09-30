package rules

import (
	"errors"
	"fmt"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqTrack struct {
	TableName   string
	TablePeriod *int64
	TableSize   *int64
	TrackKey    string
}

func (r ReqTrack) GetType() Type {
	return REQ_TRACK
}

func (r ReqTrack) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return errors.New("request Track cannot be configured in TCP mode")
	}

	// Create tracking table.
	if _, err := client.BackendGet(r.TableName); err != nil {
		err = client.BackendCreate(models.Backend{
			Name: r.TableName,
			StickTable: &models.ConfigStickTable{
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
