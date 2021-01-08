package annotations

import (
	"strings"
	"time"

	"github.com/haproxytech/config-parser/v3/types"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type GlobalHardStopAfter struct {
	name   string
	data   *types.StringC
	client api.HAProxyClient
}

func NewGlobalHardStopAfter(n string, c api.HAProxyClient) *GlobalHardStopAfter {
	return &GlobalHardStopAfter{name: n, client: c}
}

func (a *GlobalHardStopAfter) GetName() string {
	return a.name
}

func (a *GlobalHardStopAfter) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	after, err := time.ParseDuration(input.Value)
	if err != nil {
		return err
	}
	duration := after.String()
	if strings.HasSuffix(duration, "m0s") {
		duration = duration[:len(duration)-2]
	}
	if strings.HasSuffix(duration, "h0m") {
		duration = duration[:len(duration)-2]
	}

	if err != nil {
		return err
	}

	a.data = &types.StringC{Value: duration}
	return nil
}

func (a *GlobalHardStopAfter) Delete() error {
	return a.client.GlobalHardStopAfter(nil)
}

func (a *GlobalHardStopAfter) Update() error {
	if a.data == nil {
		logger.Infof("Removing hard-stop-after timeout")
		return a.client.GlobalHardStopAfter(nil)
	}
	logger.Infof("Setting hard-stop-after to %s", a.data.Value)
	return a.client.GlobalHardStopAfter(a.data)
}
