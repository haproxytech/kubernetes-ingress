package rules

import (
	"errors"
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/controller/constants"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ReqTrack struct {
	TableName      string
	TablePeriod    *int64
	TableSize      *int64
	TrackKey       string
	ExcludePathEnd []string // Path suffixes excluded from request tracking
}

const (
	defaultPeriod    = "1s"
	defaultTableSize = "100k"
)

func (r ReqTrack) GetType() Type {
	return REQ_TRACK
}

func (r ReqTrack) Create(client api.HAProxyClient, frontend *models.Frontend, ingressACL string) error {
	if frontend.Mode == "tcp" {
		return errors.New("request Track cannot be configured in TCP mode")
	}
	err := r.applyDefaults()
	if err != nil {
		return err
	}

	// Create tracking table.
	if !client.BackendUsed(r.TableName) {
		backend := models.BackendBase{
			From: constants.DefaultsSectionName,
			Name: r.TableName,
			StickTable: &models.ConfigStickTable{
				Peers: "localinstance",
				Type:  "ip",
				Size:  r.TableSize,
				Store: fmt.Sprintf("http_req_rate(%d)", *r.TablePeriod),
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
	if condTest := r.condTest(); condTest != "" {
		httpRule.Cond = "if"
		httpRule.CondTest = condTest
	}
	return client.FrontendHTTPRequestRuleCreate(0, frontend.Name, httpRule, ingressACL)
}

// condTest builds the HAProxy condition of the track-sc rule. Requests
// whose path ends with an excluded suffix are not tracked at all, so they
// never increment the http_req_rate counter the deny rule reads.
func (r ReqTrack) condTest() string {
	if len(r.ExcludePathEnd) == 0 {
		return ""
	}
	return fmt.Sprintf("!{ path_end %s }", strings.Join(r.ExcludePathEnd, " "))
}

func (r *ReqTrack) applyDefaults() error {
	if r.TablePeriod == nil {
		period, err := utils.ParseTime(defaultPeriod)
		if err != nil {
			return err
		}
		r.TablePeriod = utils.PtrInt64(*period)
	}
	if r.TableSize == nil {
		size, err := utils.ParseSize(defaultTableSize)
		if err != nil {
			return err
		}
		r.TableSize = size
	}
	return nil
}
