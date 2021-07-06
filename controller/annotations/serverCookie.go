package annotations

import (
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type ServerCookie struct {
	name    string
	enabled bool
	server  *models.Server
}

func NewServerCookie(n string, s *models.Server) *ServerCookie {
	return &ServerCookie{name: n, server: s}
}

func (a *ServerCookie) GetName() string {
	return a.name
}

func (a *ServerCookie) Parse(input string) error {
	if len(strings.Fields(input)) != 1 {
		// Error should already be reported in BackendCookie
		return nil
	}
	a.enabled = true
	return nil
}

func (a *ServerCookie) Update() error {
	if a.enabled {
		a.server.Cookie = a.server.Name
	} else {
		a.server.Cookie = ""
	}
	return nil
}
