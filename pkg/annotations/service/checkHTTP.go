package service

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v3/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type CheckHTTP struct {
	name    string
	backend *models.Backend
}

func NewCheckHTTP(n string, b *models.Backend) *CheckHTTP {
	return &CheckHTTP{name: n, backend: b}
}

func (a *CheckHTTP) GetName() string {
	return a.name
}

func (a *CheckHTTP) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
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
