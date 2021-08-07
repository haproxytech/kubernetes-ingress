package annotations

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type BackendTimeoutCheck struct {
	name    string
	backend *models.Backend
}

func NewBackendTimeoutCheck(n string, b *models.Backend) *BackendTimeoutCheck {
	return &BackendTimeoutCheck{name: n, backend: b}
}

func (a *BackendTimeoutCheck) GetName() string {
	return a.name
}

func (a *BackendTimeoutCheck) Process(input string) error {
	if input == "" {
		a.backend.CheckTimeout = nil
		return nil
	}
	timeout, err := utils.ParseTime(input)
	if err != nil {
		return err
	}
	a.backend.CheckTimeout = timeout
	return nil
}
