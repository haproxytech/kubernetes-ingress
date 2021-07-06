package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type BackendAbortOnClose struct {
	name    string
	enabled bool
	backend *models.Backend
}

func NewBackendAbortOnClose(n string, b *models.Backend) *BackendAbortOnClose {
	return &BackendAbortOnClose{name: n, backend: b}
}

func (a *BackendAbortOnClose) GetName() string {
	return a.name
}

func (a *BackendAbortOnClose) Parse(input string) error {
	var err error
	a.enabled, err = utils.GetBoolValue(input, "abortonclose")
	return err
}

func (a *BackendAbortOnClose) Update() error {
	if a.enabled {
		a.backend.Abortonclose = "enabled"
	} else {
		a.backend.Abortonclose = "disabled"
	}
	return nil
}
