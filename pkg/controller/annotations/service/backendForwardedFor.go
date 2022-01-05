package service

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ForwardedFor struct {
	name    string
	backend *models.Backend
}

func NewForwardedFor(n string, b *models.Backend) *ForwardedFor {
	return &ForwardedFor{name: n, backend: b}
}

func (a *ForwardedFor) GetName() string {
	return a.name
}

func (a *ForwardedFor) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		a.backend.Forwardfor = nil
		return nil
	}
	var params *models.Forwardfor
	enabled, err := utils.GetBoolValue(input, "forwarded-for")
	if err != nil {
		return err
	}
	if enabled {
		params = &models.Forwardfor{
			Enabled: utils.PtrString("enabled"),
		}
		if err = params.Validate(nil); err != nil {
			return fmt.Errorf("forwarded-for: %w", err)
		}
	}
	a.backend.Forwardfor = params
	return nil
}
