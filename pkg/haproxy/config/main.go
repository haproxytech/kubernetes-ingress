package config

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
)

// Config holds the haroxy configuration state
type Config struct {
	maps.Maps
	rules.Rules
	certs.Certificates
	HTTPS          bool
	SSLPassthrough bool
	AuxCfgModTime  int64
}

// Init initializes HAProxy structs
func New(env Env, rules rules.Rules) (cfg *Config, err error) {
	cfg = &Config{}
	persistentMaps := []maps.Name{
		route.SNI,
		route.HOST,
		route.PATH_EXACT,
		route.PATH_PREFIX,
	}
	if cfg.Maps, err = maps.New(env.MapsDir, persistentMaps); err != nil {
		return
	}
	if cfg.Certificates, err = certs.New(env.Certs); err != nil {
		return
	}
	cfg.Rules = rules
	return
}

// Clean cleans all the statuses of various data that was changed
// deletes them completely or just resets them if needed
func (cfg *Config) Clean() {
	cfg.SSLPassthrough = false
	cfg.CleanMaps()
	cfg.CleanCerts()
	cfg.CleanRules()
}
