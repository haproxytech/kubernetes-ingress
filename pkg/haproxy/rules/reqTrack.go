package rules

import (
	"errors"
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/controller/constants"
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
	if !client.BackendUsed(r.TableName) {
		backend := models.Backend{
			BackendBase: models.BackendBase{
				From: constants.DefaultsSectionName,
				Name: r.TableName,
				StickTable: &models.ConfigStickTable{
					Peers: "localinstance",
					Type:  "ip",
					Size:  r.TableSize,
					Store: fmt.Sprintf("http_req_rate(%d)", *r.TablePeriod),
				},
			},
		}
		// Create tracking table.
		client.BackendCreateOrUpdate(backend)
	}

	// Create rule
	httpRule := models.HTTPRequestRule{
		Type:                "track-sc",
		TrackScStickCounter: utils.PtrInt64(0),
		TrackScKey:          r.TrackKey,
		TrackScTable:        r.TableName,
	}
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}
