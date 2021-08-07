package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type BackendAbortOnClose struct {
	name    string
	backend *models.Backend
}

func NewBackendAbortOnClose(n string, b *models.Backend) *BackendAbortOnClose {
	return &BackendAbortOnClose{name: n, backend: b}
}

func (a *BackendAbortOnClose) GetName() string {
	return a.name
}

func (a *BackendAbortOnClose) Process(input string) error {
	var enabled bool
	var err error
	if input != "" {
		enabled, err = utils.GetBoolValue(input, "abortonclose")
		if err != nil {
			return err
		}
	}
	if enabled {
		a.backend.Abortonclose = "enabled"
	} else {
		a.backend.Abortonclose = "disabled"
	}
	return nil
}
