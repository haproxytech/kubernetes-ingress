package configuration

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// SetGlobal will set default values for Global section config.
func SetGlobal(global *models.Global, env Env) {
	// Enforced values
	global.MasterWorker = true
	global.Pidfile = env.PIDFile
	global.Localpeer = "local"
	global.ServerStateBase = env.StateDir
	global.ServerStateFile = "global"
	global.RuntimeAPIs = append(global.RuntimeAPIs, &models.RuntimeAPI{
		Address:           &env.RuntimeSocket,
		ExposeFdListeners: true,
		Level:             "admin",
	})
	// Default values
	if global.Daemon == "" {
		global.Daemon = "enabled"
	}
	if global.StatsTimeout == nil {
		global.StatsTimeout = utils.PtrInt64(36000)
	}
	if global.TuneSslDefaultDhParam == 0 {
		global.TuneSslDefaultDhParam = 2048
	}
	if global.SslDefaultBindCiphers == "" {
		global.SslDefaultBindCiphers = "ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-DSS-AES128-GCM-SHA256:kEDH+AESGCM:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA384:ECDHE-RSA-AES256-SHA:ECDHE-ECDSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-DSS-AES128-SHA256:DHE-RSA-AES256-SHA256:DHE-DSS-AES256-SHA:DHE-RSA-AES256-SHA:!aNULL:!eNULL:!EXPORT:!DES:!RC4:!3DES:!MD5:!PSK"
	}
	if global.SslDefaultBindOptions == "" {
		global.SslDefaultBindOptions = "no-sslv3 no-tls-tickets no-tlsv10"
	}
}

// SetDefaults will set default values for Defaults section config.
func SetDefaults(defaults *models.Defaults) {
	enabled := "enabled"
	if defaults.Redispatch == nil {
		defaults.Redispatch = &models.Redispatch{Enabled: &enabled}
	}
	if defaults.Dontlognull == "" {
		defaults.Dontlognull = "enabled"
	}
	if defaults.HTTPConnectionMode == "" {
		defaults.HTTPConnectionMode = "http-keep-alive"
	}
	if defaults.HTTPRequestTimeout == nil {
		defaults.HTTPRequestTimeout = utils.PtrInt64(5000) // 5s
	}
	if defaults.ConnectTimeout == nil {
		defaults.ConnectTimeout = utils.PtrInt64(5000) // 5s
	}
	if defaults.ConnectTimeout == nil {
		defaults.QueueTimeout = utils.PtrInt64(5000) // 5s
	}
	if defaults.ClientTimeout == nil {
		defaults.ClientTimeout = utils.PtrInt64(50000) // 50s
	}
	if defaults.ServerTimeout == nil {
		defaults.ServerTimeout = utils.PtrInt64(50000) // 50s
	}
	if defaults.TunnelTimeout == nil {
		defaults.TunnelTimeout = utils.PtrInt64(3600000) // 1h
	}
	if defaults.HTTPKeepAliveTimeout == nil {
		defaults.HTTPKeepAliveTimeout = utils.PtrInt64(60000) // 1m
	}
	if defaults.LogFormat == "" {
		defaults.LogFormat = "'%ci:%cp [%tr] %ft %b/%s %TR/%Tw/%Tc/%Tr/%Ta %ST %B %CC %CS %tsc %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs \"%HM %[var(txn.base)] %HV\"'"
	}
}
