package annotations

import (
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type annotation interface {
	Parse(input string) error
	Update(client api.HAProxyClient) Result
	Delete(client api.HAProxyClient) Result
}

type Result int8

const (
	NONE Result = 0 + iota
	RELOAD
	RESTART
)

// Global holds annotations related to the Global and Default HAProxy sections
var Global = map[string]annotation{
	"nbthread":                &nbthread{},
	"syslog-server":           &syslogServers{},
	"maxconn":                 &globalMaxconn{},
	"http-server-close":       &defaultOption{name: "http-server-close"},
	"http-keep-alive":         &defaultOption{name: "http-keep-alive"},
	"dontlognull":             &defaultOption{name: "dontlognull"},
	"logasap":                 &defaultOption{name: "logasap"},
	"timeout-http-request":    &defaultTimeout{name: "http-request"},
	"timeout-connect":         &defaultTimeout{name: "connect"},
	"timeout-client":          &defaultTimeout{name: "client"},
	"timeout-client-fin":      &defaultTimeout{name: "client-fin"},
	"timeout-queue":           &defaultTimeout{name: "queue"},
	"timeout-server":          &defaultTimeout{name: "server"},
	"timeout-server-fin":      &defaultTimeout{name: "server-fin"},
	"timeout-tunnel":          &defaultTimeout{name: "tunnel"},
	"timeout-http-keep-alive": &defaultTimeout{name: "http-keep-alive"},
	"log-format":              &defaultLogFormat{},
	"config-snippet":          &globalCfgSnippet{},
}

var logger = utils.GetLogger()
