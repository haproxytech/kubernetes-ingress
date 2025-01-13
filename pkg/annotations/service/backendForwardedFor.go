package service

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type ForwardedFor struct {
	backend *models.Backend
	name    string
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
