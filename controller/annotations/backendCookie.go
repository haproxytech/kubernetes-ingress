package annotations

import (
	"fmt"
	"strings"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type BackendCookie struct {
	name       string
	cookieName string
	backend    *models.Backend
}

func NewBackendCookie(n string, b *models.Backend) *BackendCookie {
	return &BackendCookie{name: n, backend: b}
}

func (a *BackendCookie) GetName() string {
	return a.name
}

func (a *BackendCookie) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	if len(strings.Fields(input.Value)) != 1 {
		return fmt.Errorf("cookie-persistence: Incorrect input %s", input.Value)
	}
	a.cookieName = input.Value
	return nil
}

func (a *BackendCookie) Update() error {
	if a.cookieName == "" {
		a.backend.Cookie = nil
		return nil
	}
	cookie := models.Cookie{
		Name:     &a.cookieName,
		Type:     "insert",
		Nocache:  true,
		Indirect: true,
	}
	a.backend.Cookie = &cookie
	return nil
}
