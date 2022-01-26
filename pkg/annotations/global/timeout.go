package global

import (
	"errors"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type Timeout struct {
	name     string
	defaults *models.Defaults
}

func NewTimeout(n string, d *models.Defaults) *Timeout {
	return &Timeout{name: n, defaults: d}
}

func (a *Timeout) GetName() string {
	return a.name
}

func (a *Timeout) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	var timeout *int64
	var err error
	if input != "" {
		timeout, err = utils.ParseTime(input)
		if err != nil {
			return err
		}
	}

	switch a.name {
	case "timeout-client":
		a.defaults.ClientTimeout = timeout
	case "timeout-client-fin":
		a.defaults.ClientFinTimeout = timeout
	case "timeout-connect":
		a.defaults.ConnectTimeout = timeout
	case "timeout-http-keep-alive":
		a.defaults.HTTPKeepAliveTimeout = timeout
	case "timeout-http-request":
		a.defaults.HTTPRequestTimeout = timeout
	case "timeout-queue":
		a.defaults.QueueTimeout = timeout
	case "timeout-server":
		a.defaults.ServerTimeout = timeout
	case "timeout-server-fin":
		a.defaults.ServerFinTimeout = timeout
	case "timeout-tunnel":
		a.defaults.TunnelTimeout = timeout
	default:
		return errors.New("unknown param")
	}
	return nil
}
