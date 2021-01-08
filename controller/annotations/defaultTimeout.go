package annotations

import (
	"strings"
	"time"

	"github.com/haproxytech/config-parser/v3/types"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type DefaultTimeout struct {
	name   string
	data   *types.SimpleTimeout
	client api.HAProxyClient
}

func NewDefaultTimeout(n string, c api.HAProxyClient) *DefaultTimeout {
	return &DefaultTimeout{name: n, client: c}
}

func (a *DefaultTimeout) GetName() string {
	return a.name
}

func (a *DefaultTimeout) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	timeout, err := time.ParseDuration(input.Value)
	if err != nil {
		return err
	}
	s := timeout.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	a.data = &types.SimpleTimeout{Value: s}
	return nil
}

func (a *DefaultTimeout) Update() error {
	timeout := strings.TrimPrefix(a.name, "timeout-")
	if a.data == nil {
		logger.Infof("Removing default timeout-%s ", timeout)
		return a.client.DefaultTimeout(timeout, nil)
	}
	logger.Infof("Setting default timeout-%s to %s", timeout, a.data.Value)
	return a.client.DefaultTimeout(timeout, a.data)
}
