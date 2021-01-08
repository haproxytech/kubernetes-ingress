package annotations

import (
	"fmt"
	"strings"

	"github.com/haproxytech/models/v2"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type BackendCheckHTTP struct {
	name    string
	params  *models.HttpchkParams
	backend *models.Backend
}

func NewBackendCheckHTTP(n string, b *models.Backend) *BackendCheckHTTP {
	return &BackendCheckHTTP{name: n, backend: b}
}

func (a *BackendCheckHTTP) GetName() string {
	return a.name
}

func (a *BackendCheckHTTP) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	checkHTTPParams := strings.Fields(strings.TrimSpace(input.Value))
	switch len(checkHTTPParams) {
	case 0:
		return fmt.Errorf("httpchk option: incorrect number of params")
	case 1:
		a.params = &models.HttpchkParams{
			URI: checkHTTPParams[0],
		}
	case 2:
		a.params = &models.HttpchkParams{
			Method: checkHTTPParams[0],
			URI:    checkHTTPParams[1],
		}
	default:
		a.params = &models.HttpchkParams{
			Method:  checkHTTPParams[0],
			URI:     checkHTTPParams[1],
			Version: strings.Join(checkHTTPParams[2:], " "),
		}
	}
	return nil
}

func (a *BackendCheckHTTP) Update() error {
	if a.params == nil {
		a.backend.AdvCheck = ""
		a.backend.HttpchkParams = nil
		return nil
	}
	a.backend.AdvCheck = "httpchk"
	a.backend.HttpchkParams = a.params
	return nil
}
