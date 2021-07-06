package annotations

import (
	"errors"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type DefaultOption struct {
	name     string
	defaults *models.Defaults
	enabled  *bool
}

func NewDefaultOption(n string, d *models.Defaults) *DefaultOption {
	return &DefaultOption{
		name:     n,
		defaults: d,
	}
}

func (a *DefaultOption) GetName() string {
	return a.name
}

func (a *DefaultOption) Parse(input string) error {
	enabled, err := utils.GetBoolValue(input, a.name)
	if err != nil {
		return err
	}
	a.enabled = &enabled
	return nil
}

func (a *DefaultOption) Update() error {
	if a.enabled == nil {
		logger.Infof("Removing option %s", a.name)
		switch a.name {
		case "http-server-close", "http-keep-alive":
			a.defaults.HTTPConnectionMode = ""
		case "dontlognull":
			a.defaults.Dontlognull = ""
		case "logasap":
			a.defaults.Logasap = ""
		default:
			return errors.New("unknown param")
		}
	}
	if *a.enabled {
		logger.Infof("enabling option %s", a.name)
		switch a.name {
		case "http-server-close":
			a.defaults.HTTPConnectionMode = "http-server-close"
		case "http-keep-alive":
			a.defaults.HTTPConnectionMode = "http-keep-alive"
		case "dontlognull":
			a.defaults.Dontlognull = "enabled"
		case "logasap":
			a.defaults.Logasap = "enabled"
		default:
			return errors.New("unknown param")
		}
	} else {
		logger.Infof("disabling option %s", a.name)
		switch a.name {
		case "http-server-close", "http-keep-alive":
			a.defaults.HTTPConnectionMode = ""
		case "dontlognull":
			a.defaults.Dontlognull = "disabled"
		case "logasap":
			a.defaults.Logasap = "disabled"
		default:
			return errors.New("unknown param")
		}
	}
	return nil
}
