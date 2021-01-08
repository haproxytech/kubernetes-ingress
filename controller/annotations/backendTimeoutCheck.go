package annotations

import (
	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type BackendTimeoutCheck struct {
	name    string
	timeout *int64
	backend *models.Backend
}

func NewBackendTimeoutCheck(n string, b *models.Backend) *BackendTimeoutCheck {
	return &BackendTimeoutCheck{name: n, backend: b}
}

func (a *BackendTimeoutCheck) GetName() string {
	return a.name
}

func (a *BackendTimeoutCheck) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	var err error
	a.timeout, err = utils.ParseTime(input.Value)
	return err
}

func (a *BackendTimeoutCheck) Update() error {
	a.backend.CheckTimeout = a.timeout
	return nil
}
