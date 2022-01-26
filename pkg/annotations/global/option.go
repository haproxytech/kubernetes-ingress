package global

import (
	"errors"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type Option struct {
	name     string
	defaults *models.Defaults
}

func NewOption(n string, d *models.Defaults) *Option {
	return &Option{
		name:     n,
		defaults: d,
	}
}

func (a *Option) GetName() string {
	return a.name
}

func (a *Option) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
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
		return nil
	}

	enabled, err := utils.GetBoolValue(input, a.name)
	if err != nil {
		return err
	}
	if enabled {
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
