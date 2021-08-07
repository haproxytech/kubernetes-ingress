package annotations

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"
)

type ServerProto struct {
	name   string
	server *models.Server
}

func NewServerProto(n string, s *models.Server) *ServerProto {
	return &ServerProto{name: n, server: s}
}

func (a *ServerProto) GetName() string {
	return a.name
}

func (a *ServerProto) Process(input string) error {
	switch input {
	case "":
		a.server.Proto = ""
	case "h1":
		// Forces H1 even when SSL is enabled
		a.server.Alpn = ""
		a.server.Proto = ""
	case "h2":
		a.server.Proto = "h2"
	default:
		return fmt.Errorf("unknown proto %s", input)
	}
	return nil
}
