package service

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

//nolint:stylecheck
const (
	SUFFIX_NO_DYNAMIC = "no-dynamic"
)

type Cookie struct {
	backend       *models.Backend
	name          string
	nameNoDynamic string
}

func NewCookie(n string, b *models.Backend) *Cookie {
	nameNoDynamic := n + "-" + SUFFIX_NO_DYNAMIC
	return &Cookie{name: n, nameNoDynamic: nameNoDynamic, backend: b}
}

func (a *Cookie) GetName() string {
	return a.name
}

func (a *Cookie) Process(k store.K8s, annotations ...map[string]string) error {
	// Cookie dynamic annotation ?
	input := common.GetValue(a.GetName(), annotations...)
	params := strings.Fields(input)

	// Is there a "no-dynamic" annotation
	inputNoDynamic := common.GetValue(a.nameNoDynamic, annotations...)
	paramsNoDynamic := strings.Fields(inputNoDynamic)

	if len(paramsNoDynamic) > 0 && len(params) > 0 {
		return fmt.Errorf("cookie: cannot use both %s and %s annotations", a.GetName(), a.nameNoDynamic)
	}

	if len(params) == 0 && len(paramsNoDynamic) == 0 {
		a.backend.Cookie = nil
		return nil
	}

	cookieName := ""
	isdynamicCookie := true
	if len(params) > 0 {
		cookieName = params[0]
	} else {
		cookieName = paramsNoDynamic[0]
		isdynamicCookie = false
	}

	a.backend.Cookie = &models.Cookie{
		Name:     &cookieName,
		Type:     "insert",
		Nocache:  true,
		Indirect: true,
		Dynamic:  isdynamicCookie,
		Domains:  []*models.Domain{},
	}
	return nil
}
