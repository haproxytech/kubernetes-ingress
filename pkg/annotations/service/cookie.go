package service

import (
	"strings"

	"github.com/haproxytech/client-native/v3/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type Cookie struct {
	backend *models.Backend
	name    string
}

func NewCookie(n string, b *models.Backend) *Cookie {
	return &Cookie{name: n, backend: b}
}

func (a *Cookie) GetName() string {
	return a.name
}

func (a *Cookie) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	params := strings.Fields(input)
	if len(params) == 0 {
		a.backend.Cookie = nil
		return nil
	}
	cookieName := params[0]
	a.backend.Cookie = &models.Cookie{
		Name:     &cookieName,
		Type:     "insert",
		Nocache:  true,
		Indirect: true,
		Dynamic:  true,
		Domains:  []*models.Domain{},
	}
	return nil
}
