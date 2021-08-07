package annotations

import (
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type ServerCookie struct {
	name   string
	server *models.Server
}

func NewServerCookie(n string, s *models.Server) *ServerCookie {
	return &ServerCookie{name: n, server: s}
}

func (a *ServerCookie) GetName() string {
	return a.name
}

func (a *ServerCookie) Process(input string) error {
	// Cookie is handled also at the Backend
	if len(strings.Fields(input)) != 1 {
		a.server.Cookie = ""
	} else {
		a.server.Cookie = a.server.Name
	}
	return nil
}
