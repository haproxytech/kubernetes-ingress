package annotations

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"
)

type ServerProto struct {
	name   string
	proto  string
	server *models.Server
}

func NewServerProto(n string, s *models.Server) *ServerProto {
	return &ServerProto{name: n, server: s}
}

func (a *ServerProto) GetName() string {
	return a.name
}

func (a *ServerProto) Parse(input string) error {
	switch input {
	case "h1":
		a.server.Alpn = ""
	case "h2":
		a.proto = "h2"
	default:
		return fmt.Errorf("unknown proto %s", input)
	}

	return nil
}

func (a *ServerProto) Update() error {
	// Exclusive with SSL (which sets ALPN to H1/H2)
	if a.server.Alpn != "" {
		return nil
	}
	a.server.Proto = a.proto
	return nil
}
