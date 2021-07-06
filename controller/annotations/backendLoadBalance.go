package annotations

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"
)

type BackendLoadBalance struct {
	name    string
	params  *models.Balance
	backend *models.Backend
}

func NewBackendLoadBalance(n string, b *models.Backend) *BackendLoadBalance {
	return &BackendLoadBalance{name: n, backend: b}
}

func (a *BackendLoadBalance) GetName() string {
	return a.name
}

func (a *BackendLoadBalance) Parse(input string) error {
	params := &models.Balance{
		Algorithm: &input,
	}
	if err := params.Validate(nil); err != nil {
		return fmt.Errorf("load-balance: %w", err)
	}
	a.params = params
	return nil
}

func (a *BackendLoadBalance) Update() error {
	a.backend.Balance = a.params
	return nil
}
