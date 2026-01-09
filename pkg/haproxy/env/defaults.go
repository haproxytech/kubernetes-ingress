package env

import (
	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// SetGlobal will set default values for Global section config.
func SetGlobal(global *models.Global, logTargets *models.LogTargets, env Env) {
	// Enforced values
	global.MasterWorker = true
	global.Pidfile = env.PIDFile
	runtimeAPIs := []*models.RuntimeAPI{}
	if env.RuntimeSocket != "" {
		runtimeAPIs = append(runtimeAPIs, &models.RuntimeAPI{
			Address: &env.RuntimeSocket,
			BindParams: models.BindParams{
				ExposeFdListeners: true,
				Level:             "admin",
			},
		})
	}
	if len(global.RuntimeAPIs) == 0 {
		global.RuntimeAPIs = runtimeAPIs
	} else if *(global.RuntimeAPIs[0].Address) != env.RuntimeSocket {
		global.RuntimeAPIs = append(runtimeAPIs, global.RuntimeAPIs...)
	}
	// Default values
	if global.StatsTimeout == nil {
		global.StatsTimeout = utils.PtrInt64(36000)
	}

	// SSL options
	if global.SslOptions == nil {
		global.SslOptions = &models.SslOptions{}
	}
	if global.SslOptions.DefaultBindOptions == "" {
		global.SslOptions.DefaultBindOptions = "no-sslv3 no-tls-tickets no-tlsv10"
	}
	if global.SslOptions.DefaultBindCiphers == "" {
		global.SslOptions.DefaultBindCiphers = "ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-DSS-AES128-GCM-SHA256:kEDH+AESGCM:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA384:ECDHE-RSA-AES256-SHA:ECDHE-ECDSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-DSS-AES128-SHA256:DHE-RSA-AES256-SHA256:DHE-DSS-AES256-SHA:DHE-RSA-AES256-SHA:AES256-GCM-SHA384:AES128-SHA256:AES256-SHA256:AES128-SHA:AES256-SHA:AES:CAMELLIA:!aNULL:!eNULL:!EXPORT:!DES:!RC4:!MD5:!PSK:!aECDH:!EDH-DSS-DES-CBC3-SHA:!EDH-RSA-DES-CBC3-SHA:!KRB5-DES-CBC3-SHA:!3DES"
	}

	// Tune options
	if global.TuneOptions == nil {
		global.TuneOptions = &models.TuneOptions{}
	}
	if len(*logTargets) == 0 {
		*logTargets = []*models.LogTarget{{
			Address:  "127.0.0.1",
			Facility: "local0",
			Level:    "notice",
		}}
	} else {
		for _, v := range *logTargets {
			if v.Address == "stdout" {
				global.Daemon = false
				break
			}
		}
	}
	if global.HardStopAfter == nil {
		global.HardStopAfter = utils.PtrInt64(1800000) // 30m
	}
	global.DefaultPath = &models.GlobalDefaultPath{
		Type: "config",
	}
	global.LimitedQuic = true
}

// SetDefaults will set default values for Defaults section config.
func SetDefaults(defaults *models.Defaults) {
	// Enforced values
	// Logging is enforced in DefaultsPushConfiguration method

	// Default values
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
	if defaults.QueueTimeout == nil {
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
