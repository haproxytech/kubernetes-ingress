package annotations

import (
	"errors"
	"strings"

	"github.com/haproxytech/config-parser/v3/types"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type DefaultLogFormat struct {
	name   string
	data   *types.StringC
	client api.HAProxyClient
}

func NewDefaultLogFormat(n string, c api.HAProxyClient) *DefaultLogFormat {
	return &DefaultLogFormat{name: n, client: c}
}

func (a *DefaultLogFormat) GetName() string {
	return a.name
}

func (a *DefaultLogFormat) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	v := strings.TrimSpace(input.Value)
	if v == "" {
		return errors.New("unable to parse log-format: empty input")
	}
	a.data = &types.StringC{Value: "'" + v + "'"}
	return nil
}

func (a *DefaultLogFormat) Update() error {
	if a.data == nil {
		logger.Infof("Removing default log-format")
		return a.client.DefaultLogFormat(nil)
	}
	logger.Infof("Setting default log-format to: %s", a.data.Value)
	return a.client.DefaultLogFormat(a.data)
}
