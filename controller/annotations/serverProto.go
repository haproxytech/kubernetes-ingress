package annotations

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
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

func (a *ServerProto) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.DELETED {
		return nil
	}
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}

	switch input.Value {
	case "h1":
		a.server.Alpn = ""
	case "h2":
		a.proto = "h2"
	default:
		return fmt.Errorf("unknown proto %s", input.Value)
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
