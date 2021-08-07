package annotations

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type BackendForwardedFor struct {
	name    string
	backend *models.Backend
}

func NewBackendForwardedFor(n string, b *models.Backend) *BackendForwardedFor {
	return &BackendForwardedFor{name: n, backend: b}
}

func (a *BackendForwardedFor) GetName() string {
	return a.name
}

func (a *BackendForwardedFor) Process(input string) error {
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
