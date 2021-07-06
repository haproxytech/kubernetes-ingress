package annotations

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type BackendForwardedFor struct {
	name    string
	params  *models.Forwardfor
	backend *models.Backend
}

func NewBackendForwardedFor(n string, b *models.Backend) *BackendForwardedFor {
	return &BackendForwardedFor{name: n, backend: b}
}

func (a *BackendForwardedFor) GetName() string {
	return a.name
}

func (a *BackendForwardedFor) Parse(input string) error {
	enabled, err := utils.GetBoolValue(input, "forwarded-for")
	if enabled {
		params := &models.Forwardfor{
			Enabled: utils.PtrString("enabled"),
		}
		if err = params.Validate(nil); err != nil {
			return fmt.Errorf("forwarded-for: %w", err)
		}
		a.params = params
	}
	return err
}

func (a *BackendForwardedFor) Update() error {
	a.backend.Forwardfor = a.params
	return nil
}
