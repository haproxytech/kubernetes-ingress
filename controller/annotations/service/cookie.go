package service

import (
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type Cookie struct {
	name    string
	backend *models.Backend
	server  *models.Server
}

func NewCookie(n string, b *models.Backend, s *models.Server) *Cookie {
	return &Cookie{name: n, backend: b, server: s}
}

func (a *Cookie) GetName() string {
	return a.name
}

func (a *Cookie) Process(input string) error {
	params := strings.Fields(input)
	if len(params) == 0 {
		switch {
		case a.backend != nil:
			a.backend.Cookie = nil
		case a.server != nil:
			a.server.Cookie = ""
		}
		return nil
	}
	switch {
	case a.backend != nil:
		cookieName := params[0]
		cookie := models.Cookie{
			Name:     &cookieName,
			Type:     "insert",
			Nocache:  true,
			Indirect: true,
		}
		a.backend.Cookie = &cookie
	case a.server != nil:
		a.server.Cookie = a.server.Name
	}
	return nil
}
