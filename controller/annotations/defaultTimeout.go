package annotations

import (
	"errors"
	"strings"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type DefaultTimeout struct {
	name     string
	defaults *models.Defaults
	timeout  *int64
}

func NewDefaultTimeout(n string, d *models.Defaults) *DefaultTimeout {
	return &DefaultTimeout{name: n, defaults: d}
}

func (a *DefaultTimeout) GetName() string {
	return a.name
}

func (a *DefaultTimeout) Parse(input string) error {
	var err error
	a.timeout, err = utils.ParseTime(input)
	return err
}

func (a *DefaultTimeout) Update() error {
	timeout := strings.TrimPrefix(a.name, "timeout-")
	if a.timeout == nil {
		logger.Infof("Removing default timeout-%s", timeout)
	} else {
		logger.Infof("Setting default timeout-%s to %ds", timeout, *a.timeout)
	}
	switch a.name {
	case "timeout-client":
		a.defaults.ClientTimeout = a.timeout
	case "timeout-client-fin":
		a.defaults.ClientFinTimeout = a.timeout
	case "timeout-connect":
		a.defaults.ConnectTimeout = a.timeout
	case "timeout-http-keep-alive":
		a.defaults.HTTPKeepAliveTimeout = a.timeout
	case "timeout-http-request":
		a.defaults.HTTPRequestTimeout = a.timeout
	case "timeout-queue":
		a.defaults.QueueTimeout = a.timeout
	case "timeout-server":
		a.defaults.ServerTimeout = a.timeout
	case "timeout-server-fin":
		a.defaults.ServerFinTimeout = a.timeout
	case "timeout-tunnel":
		a.defaults.TunnelTimeout = a.timeout
	default:
		return errors.New("unknown param")
	}
	return nil
}
