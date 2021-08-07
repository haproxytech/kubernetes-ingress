package annotations

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type BackendCheckHTTP struct {
	name    string
	backend *models.Backend
}

func NewBackendCheckHTTP(n string, b *models.Backend) *BackendCheckHTTP {
	return &BackendCheckHTTP{name: n, backend: b}
}

func (a *BackendCheckHTTP) GetName() string {
	return a.name
}

func (a *BackendCheckHTTP) Process(input string) error {
	if input == "" {
		a.backend.AdvCheck = ""
		a.backend.HttpchkParams = nil
		return nil
	}
	var params *models.HttpchkParams
	checkHTTPParams := strings.Fields(strings.TrimSpace(input))
	switch len(checkHTTPParams) {
	case 0:
		return fmt.Errorf("httpchk option: incorrect number of params")
	case 1:
		params = &models.HttpchkParams{
			URI: checkHTTPParams[0],
		}
	case 2:
		params = &models.HttpchkParams{
			Method: checkHTTPParams[0],
			URI:    checkHTTPParams[1],
		}
	default:
		params = &models.HttpchkParams{
			Method:  checkHTTPParams[0],
			URI:     checkHTTPParams[1],
			Version: strings.Join(checkHTTPParams[2:], " "),
		}
	}

	a.backend.AdvCheck = "httpchk"
	a.backend.HttpchkParams = params
	return nil
}
