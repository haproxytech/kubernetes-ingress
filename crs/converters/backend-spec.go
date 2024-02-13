// Copyright 2022 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package converters

import (
	"github.com/haproxytech/client-native/v5/models"

	corev1alpha2 "github.com/haproxytech/kubernetes-ingress/crs/api/core/v1alpha2"
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func DeepConvertBackendSpecA2toV1(o corev1alpha2.BackendSpec) v1.BackendSpec { //nolint:cyclop,maintidx
	var cp v1.BackendSpec
	if o.Config != nil {
		cp.Config = new(models.Backend)
		cp.Config.Abortonclose = o.Config.Abortonclose
		cp.Config.AcceptInvalidHTTPResponse = o.Config.AcceptInvalidHTTPResponse
		cp.Config.AdvCheck = o.Config.AdvCheck
		cp.Config.Allbackups = o.Config.Allbackups
		if o.Config.Balance != nil {
			cp.Config.Balance = new(models.Balance)
			if o.Config.Balance.Algorithm != nil {
				cp.Config.Balance.Algorithm = new(string)
				cp.Config.Balance.Algorithm = o.Config.Balance.Algorithm
			}
			cp.Config.Balance.HdrName = o.Config.Balance.HdrName
			cp.Config.Balance.HdrUseDomainOnly = o.Config.Balance.HdrUseDomainOnly
			cp.Config.Balance.RandomDraws = o.Config.Balance.RandomDraws
			cp.Config.Balance.RdpCookieName = o.Config.Balance.RdpCookieName
			cp.Config.Balance.URIDepth = o.Config.Balance.URIDepth
			cp.Config.Balance.URILen = o.Config.Balance.URILen
			cp.Config.Balance.URIPathOnly = o.Config.Balance.URIPathOnly
			cp.Config.Balance.URIWhole = o.Config.Balance.URIWhole
			cp.Config.Balance.URLParam = o.Config.Balance.URLParam
			cp.Config.Balance.URLParamCheckPost = o.Config.Balance.URLParamCheckPost
			cp.Config.Balance.URLParamMaxWait = o.Config.Balance.URLParamMaxWait
		}
		cp.Config.BindProcess = o.Config.BindProcess
		if o.Config.CheckTimeout != nil {
			cp.Config.CheckTimeout = new(int64)
			cp.Config.CheckTimeout = o.Config.CheckTimeout
		}
		if o.Config.Compression != nil {
			cp.Config.Compression = new(models.Compression)
			if o.Config.Compression.Algorithms != nil {
				cp.Config.Compression.Algorithms = make([]string, len(o.Config.Compression.Algorithms))
				copy(cp.Config.Compression.Algorithms, o.Config.Compression.Algorithms)
			}
			cp.Config.Compression.Offload = o.Config.Compression.Offload
			if o.Config.Compression.Types != nil {
				cp.Config.Compression.Types = make([]string, len(o.Config.Compression.Types))
				copy(cp.Config.Compression.Types, o.Config.Compression.Types)
			}
		}
		if o.Config.ConnectTimeout != nil {
			cp.Config.ConnectTimeout = new(int64)
			cp.Config.ConnectTimeout = o.Config.ConnectTimeout
		}
		if o.Config.Cookie != nil {
			cp.Config.Cookie = new(models.Cookie)
			if o.Config.Cookie.Domains != nil {
				cp.Config.Cookie.Domains = make([]*models.Domain, len(o.Config.Cookie.Domains))
				for i6 := range o.Config.Cookie.Domains {
					if o.Config.Cookie.Domains[i6] != nil {
						cp.Config.Cookie.Domains[i6] = new(models.Domain)
						cp.Config.Cookie.Domains[i6].Value = o.Config.Cookie.Domains[i6].Value
					}
				}
			}
			cp.Config.Cookie.Dynamic = o.Config.Cookie.Dynamic
			cp.Config.Cookie.Httponly = o.Config.Cookie.Httponly
			cp.Config.Cookie.Indirect = o.Config.Cookie.Indirect
			cp.Config.Cookie.Maxidle = o.Config.Cookie.Maxidle
			cp.Config.Cookie.Maxlife = o.Config.Cookie.Maxlife
			if o.Config.Cookie.Name != nil {
				cp.Config.Cookie.Name = new(string)
				cp.Config.Cookie.Name = o.Config.Cookie.Name
			}
			cp.Config.Cookie.Nocache = o.Config.Cookie.Nocache
			cp.Config.Cookie.Postonly = o.Config.Cookie.Postonly
			cp.Config.Cookie.Preserve = o.Config.Cookie.Preserve
			cp.Config.Cookie.Secure = o.Config.Cookie.Secure
			cp.Config.Cookie.Type = o.Config.Cookie.Type
		}
		if o.Config.DefaultServer != nil {
			cp.Config.DefaultServer = new(models.DefaultServer)
			// cp.Config.DefaultServer.Address = o.Config.DefaultServer.Address // bug in alpha2
			cp.Config.DefaultServer.AgentAddr = o.Config.DefaultServer.AgentAddr
			cp.Config.DefaultServer.AgentCheck = o.Config.DefaultServer.AgentCheck
			if o.Config.DefaultServer.AgentInter != nil {
				cp.Config.DefaultServer.AgentInter = new(int64)
				cp.Config.DefaultServer.AgentInter = o.Config.DefaultServer.AgentInter
			}
			if o.Config.DefaultServer.AgentPort != nil {
				cp.Config.DefaultServer.AgentPort = new(int64)
				cp.Config.DefaultServer.AgentPort = o.Config.DefaultServer.AgentPort
			}
			cp.Config.DefaultServer.AgentSend = o.Config.DefaultServer.AgentSend
			cp.Config.DefaultServer.Allow0rtt = o.Config.DefaultServer.Allow0rtt
			cp.Config.DefaultServer.Alpn = o.Config.DefaultServer.Alpn
			cp.Config.DefaultServer.Backup = o.Config.DefaultServer.Backup
			cp.Config.DefaultServer.SslCafile = o.Config.DefaultServer.CaFile
			cp.Config.DefaultServer.Check = o.Config.DefaultServer.Check
			cp.Config.DefaultServer.CheckSendProxy = o.Config.DefaultServer.CheckSendProxy
			cp.Config.DefaultServer.CheckSni = o.Config.DefaultServer.CheckSni
			cp.Config.DefaultServer.CheckSsl = o.Config.DefaultServer.CheckSsl
			cp.Config.DefaultServer.CheckAlpn = o.Config.DefaultServer.CheckAlpn
			cp.Config.DefaultServer.CheckProto = o.Config.DefaultServer.CheckProto
			cp.Config.DefaultServer.CheckViaSocks4 = o.Config.DefaultServer.CheckViaSocks4
			cp.Config.DefaultServer.Ciphers = o.Config.DefaultServer.Ciphers
			cp.Config.DefaultServer.Ciphersuites = o.Config.DefaultServer.Ciphersuites
			cp.Config.DefaultServer.Cookie = o.Config.DefaultServer.Cookie
			cp.Config.DefaultServer.CrlFile = o.Config.DefaultServer.CrlFile
			if o.Config.DefaultServer.Disabled == "enabled" {
				cp.Config.DefaultServer.Maintenance = "disabled"
			}
			if o.Config.DefaultServer.Enabled == "disabled" {
				cp.Config.DefaultServer.Maintenance = "enabled"
			}

			if o.Config.DefaultServer.Downinter != nil {
				cp.Config.DefaultServer.Downinter = new(int64)
				cp.Config.DefaultServer.Downinter = o.Config.DefaultServer.Downinter
			}
			cp.Config.DefaultServer.ErrorLimit = o.Config.DefaultServer.ErrorLimit
			if o.Config.DefaultServer.Fall != nil {
				cp.Config.DefaultServer.Fall = new(int64)
				cp.Config.DefaultServer.Fall = o.Config.DefaultServer.Fall
			}
			if o.Config.DefaultServer.Fastinter != nil {
				cp.Config.DefaultServer.Fastinter = new(int64)
				cp.Config.DefaultServer.Fastinter = o.Config.DefaultServer.Fastinter
			}
			cp.Config.DefaultServer.ForceSslv3 = o.Config.DefaultServer.ForceSslv3
			cp.Config.DefaultServer.ForceTlsv10 = o.Config.DefaultServer.ForceTlsv10
			cp.Config.DefaultServer.ForceTlsv11 = o.Config.DefaultServer.ForceTlsv11
			cp.Config.DefaultServer.ForceTlsv12 = o.Config.DefaultServer.ForceTlsv12
			cp.Config.DefaultServer.ForceTlsv13 = o.Config.DefaultServer.ForceTlsv13
			if o.Config.DefaultServer.HealthCheckPort != nil {
				cp.Config.DefaultServer.HealthCheckPort = new(int64)
				cp.Config.DefaultServer.HealthCheckPort = o.Config.DefaultServer.HealthCheckPort
			}
			cp.Config.DefaultServer.InitAddr = utils.PointerIfNotDefault(o.Config.DefaultServer.InitAddr)
			if o.Config.DefaultServer.Inter != nil {
				cp.Config.DefaultServer.Inter = new(int64)
				cp.Config.DefaultServer.Inter = o.Config.DefaultServer.Inter
			}
			cp.Config.DefaultServer.LogProto = o.Config.DefaultServer.LogProto
			if o.Config.DefaultServer.MaxReuse != nil {
				cp.Config.DefaultServer.MaxReuse = new(int64)
				cp.Config.DefaultServer.MaxReuse = o.Config.DefaultServer.MaxReuse
			}
			if o.Config.DefaultServer.Maxconn != nil {
				cp.Config.DefaultServer.Maxconn = new(int64)
				cp.Config.DefaultServer.Maxconn = o.Config.DefaultServer.Maxconn
			}
			if o.Config.DefaultServer.Maxqueue != nil {
				cp.Config.DefaultServer.Maxqueue = new(int64)
				cp.Config.DefaultServer.Maxqueue = o.Config.DefaultServer.Maxqueue
			}
			if o.Config.DefaultServer.Minconn != nil {
				cp.Config.DefaultServer.Minconn = new(int64)
				cp.Config.DefaultServer.Minconn = o.Config.DefaultServer.Minconn
			}
			// cp.Config.DefaultServer.Name = o.Config.DefaultServer.Name // bug in alpha2
			cp.Config.DefaultServer.Namespace = o.Config.DefaultServer.Namespace
			cp.Config.DefaultServer.NoSslv3 = o.Config.DefaultServer.NoSslv3
			cp.Config.DefaultServer.NoTlsv10 = o.Config.DefaultServer.NoTlsv10
			cp.Config.DefaultServer.NoTlsv11 = o.Config.DefaultServer.NoTlsv11
			cp.Config.DefaultServer.NoTlsv12 = o.Config.DefaultServer.NoTlsv12
			cp.Config.DefaultServer.NoTlsv13 = o.Config.DefaultServer.NoTlsv13
			cp.Config.DefaultServer.NoVerifyhost = o.Config.DefaultServer.NoVerifyhost
			cp.Config.DefaultServer.Npn = o.Config.DefaultServer.Npn
			cp.Config.DefaultServer.Observe = o.Config.DefaultServer.Observe
			cp.Config.DefaultServer.OnError = o.Config.DefaultServer.OnError
			cp.Config.DefaultServer.OnMarkedDown = o.Config.DefaultServer.OnMarkedDown
			cp.Config.DefaultServer.OnMarkedUp = o.Config.DefaultServer.OnMarkedUp
			if o.Config.DefaultServer.PoolLowConn != nil {
				cp.Config.DefaultServer.PoolLowConn = new(int64)
				cp.Config.DefaultServer.PoolLowConn = o.Config.DefaultServer.PoolLowConn
			}
			if o.Config.DefaultServer.PoolMaxConn != nil {
				cp.Config.DefaultServer.PoolMaxConn = new(int64)
				cp.Config.DefaultServer.PoolMaxConn = o.Config.DefaultServer.PoolMaxConn
			}
			if o.Config.DefaultServer.PoolPurgeDelay != nil {
				cp.Config.DefaultServer.PoolPurgeDelay = new(int64)
				cp.Config.DefaultServer.PoolPurgeDelay = o.Config.DefaultServer.PoolPurgeDelay
			}
			// if o.Config.DefaultServer.Port != nil { // bug in alpha2
			// 	cp.Config.DefaultServer.Port = new(int64)
			// 	cp.Config.DefaultServer.Port = o.Config.DefaultServer.Port
			// }
			cp.Config.DefaultServer.Proto = o.Config.DefaultServer.Proto
			if o.Config.DefaultServer.ProxyV2Options != nil {
				cp.Config.DefaultServer.ProxyV2Options = make([]string, len(o.Config.DefaultServer.ProxyV2Options))
				copy(cp.Config.DefaultServer.ProxyV2Options, o.Config.DefaultServer.ProxyV2Options)
			}
			cp.Config.DefaultServer.Redir = o.Config.DefaultServer.Redir
			cp.Config.DefaultServer.ResolveNet = o.Config.DefaultServer.ResolveNet
			cp.Config.DefaultServer.ResolvePrefer = o.Config.DefaultServer.ResolvePrefer
			cp.Config.DefaultServer.ResolveOpts = o.Config.DefaultServer.ResolveOpts
			cp.Config.DefaultServer.Resolvers = o.Config.DefaultServer.Resolvers
			if o.Config.DefaultServer.Rise != nil {
				cp.Config.DefaultServer.Rise = new(int64)
				cp.Config.DefaultServer.Rise = o.Config.DefaultServer.Rise
			}
			cp.Config.DefaultServer.SendProxy = o.Config.DefaultServer.SendProxy
			cp.Config.DefaultServer.SendProxyV2 = o.Config.DefaultServer.SendProxyV2
			cp.Config.DefaultServer.SendProxyV2Ssl = o.Config.DefaultServer.SendProxyV2Ssl
			cp.Config.DefaultServer.SendProxyV2SslCn = o.Config.DefaultServer.SendProxyV2SslCn
			if o.Config.DefaultServer.Slowstart != nil {
				cp.Config.DefaultServer.Slowstart = new(int64)
				cp.Config.DefaultServer.Slowstart = o.Config.DefaultServer.Slowstart
			}
			cp.Config.DefaultServer.Sni = o.Config.DefaultServer.Sni
			cp.Config.DefaultServer.Socks4 = o.Config.DefaultServer.Socks4
			cp.Config.DefaultServer.Source = o.Config.DefaultServer.Source
			cp.Config.DefaultServer.Ssl = o.Config.DefaultServer.Ssl
			cp.Config.DefaultServer.SslCertificate = o.Config.DefaultServer.SslCertificate
			cp.Config.DefaultServer.SslMaxVer = o.Config.DefaultServer.SslMaxVer
			cp.Config.DefaultServer.SslMinVer = o.Config.DefaultServer.SslMinVer
			cp.Config.DefaultServer.SslReuse = o.Config.DefaultServer.SslReuse
			cp.Config.DefaultServer.Stick = o.Config.DefaultServer.Stick
			cp.Config.DefaultServer.TCPUt = utils.PointerIfNotDefault(o.Config.DefaultServer.TCPUt)
			cp.Config.DefaultServer.Tfo = o.Config.DefaultServer.Tfo
			cp.Config.DefaultServer.TLSTickets = o.Config.DefaultServer.TLSTickets
			cp.Config.DefaultServer.Track = o.Config.DefaultServer.Track
			cp.Config.DefaultServer.Verify = o.Config.DefaultServer.Verify
			cp.Config.DefaultServer.Verifyhost = o.Config.DefaultServer.Verifyhost
			if o.Config.DefaultServer.Weight != nil {
				cp.Config.DefaultServer.Weight = new(int64)
				cp.Config.DefaultServer.Weight = o.Config.DefaultServer.Weight
			}
		}
		cp.Config.DynamicCookieKey = o.Config.DynamicCookieKey
		cp.Config.ExternalCheck = o.Config.ExternalCheck
		cp.Config.ExternalCheckCommand = o.Config.ExternalCheckCommand
		cp.Config.ExternalCheckPath = o.Config.ExternalCheckPath
		if o.Config.Forwardfor != nil {
			cp.Config.Forwardfor = new(models.Forwardfor)
			if o.Config.Forwardfor.Enabled != nil {
				cp.Config.Forwardfor.Enabled = new(string)
				cp.Config.Forwardfor.Enabled = o.Config.Forwardfor.Enabled
			}
			cp.Config.Forwardfor.Except = o.Config.Forwardfor.Except
			cp.Config.Forwardfor.Header = o.Config.Forwardfor.Header
			cp.Config.Forwardfor.Ifnone = o.Config.Forwardfor.Ifnone
		}
		cp.Config.H1CaseAdjustBogusServer = o.Config.H1CaseAdjustBogusServer
		if o.Config.HashType != nil {
			cp.Config.HashType = new(models.HashType)
			cp.Config.HashType.Function = o.Config.HashType.Function
			cp.Config.HashType.Method = o.Config.HashType.Method
			cp.Config.HashType.Modifier = o.Config.HashType.Modifier
		}
		cp.Config.HTTPBufferRequest = o.Config.HTTPBufferRequest
		if o.Config.HTTPCheck != nil {
			cp.Config.HTTPCheck = new(models.HTTPCheck)
			cp.Config.HTTPCheck.ExclamationMark = o.Config.HTTPCheck.ExclamationMark
			cp.Config.HTTPCheck.Match = o.Config.HTTPCheck.Match
			cp.Config.HTTPCheck.Pattern = o.Config.HTTPCheck.Pattern
			if o.Config.HTTPCheck != nil {
				cp.Config.HTTPCheck.Type = o.Config.HTTPCheck.Type
			}
		}
		cp.Config.HTTPUseHtx = o.Config.HTTPUseHtx
		cp.Config.HTTPConnectionMode = o.Config.HTTPConnectionMode
		if o.Config.HTTPKeepAliveTimeout != nil {
			cp.Config.HTTPKeepAliveTimeout = new(int64)
			cp.Config.HTTPKeepAliveTimeout = o.Config.HTTPKeepAliveTimeout
		}
		cp.Config.HTTPPretendKeepalive = o.Config.HTTPPretendKeepalive
		if o.Config.HTTPRequestTimeout != nil {
			cp.Config.HTTPRequestTimeout = new(int64)
			cp.Config.HTTPRequestTimeout = o.Config.HTTPRequestTimeout
		}
		cp.Config.HTTPReuse = o.Config.HTTPReuse
		if o.Config.HttpchkParams != nil {
			cp.Config.HttpchkParams = new(models.HttpchkParams)
			cp.Config.HttpchkParams.Method = o.Config.HttpchkParams.Method
			cp.Config.HttpchkParams.URI = o.Config.HttpchkParams.URI
			cp.Config.HttpchkParams.Version = o.Config.HttpchkParams.Version
		}
		cp.Config.LogTag = o.Config.LogTag
		cp.Config.Mode = o.Config.Mode
		if o.Config.MysqlCheckParams != nil {
			cp.Config.MysqlCheckParams = new(models.MysqlCheckParams)
			cp.Config.MysqlCheckParams.ClientVersion = o.Config.MysqlCheckParams.ClientVersion
			cp.Config.MysqlCheckParams.Username = o.Config.MysqlCheckParams.Username
		}
		cp.Config.Name = o.Config.Name
		if o.Config.PgsqlCheckParams != nil {
			cp.Config.PgsqlCheckParams = new(models.PgsqlCheckParams)
			cp.Config.PgsqlCheckParams.Username = o.Config.PgsqlCheckParams.Username
		}
		if o.Config.QueueTimeout != nil {
			cp.Config.QueueTimeout = new(int64)
			cp.Config.QueueTimeout = o.Config.QueueTimeout
		}
		if o.Config.Redispatch != nil {
			cp.Config.Redispatch = new(models.Redispatch)
			if o.Config.Redispatch.Enabled != nil {
				cp.Config.Redispatch.Enabled = new(string)
				cp.Config.Redispatch.Enabled = o.Config.Redispatch.Enabled
			}
			cp.Config.Redispatch.Interval = o.Config.Redispatch.Interval
		}
		if o.Config.Retries != nil {
			cp.Config.Retries = new(int64)
			cp.Config.Retries = o.Config.Retries
		}
		if o.Config.ServerTimeout != nil {
			cp.Config.ServerTimeout = new(int64)
			cp.Config.ServerTimeout = o.Config.ServerTimeout
		}
		if o.Config.SmtpchkParams != nil {
			cp.Config.SmtpchkParams = new(models.SmtpchkParams)
			cp.Config.SmtpchkParams.Domain = o.Config.SmtpchkParams.Domain
			cp.Config.SmtpchkParams.Hello = o.Config.SmtpchkParams.Hello
		}
		if o.Config.StatsOptions != nil {
			cp.Config.StatsOptions = new(models.StatsOptions)
			cp.Config.StatsOptions.StatsEnable = o.Config.StatsOptions.StatsEnable
			cp.Config.StatsOptions.StatsHideVersion = o.Config.StatsOptions.StatsHideVersion
			cp.Config.StatsOptions.StatsMaxconn = o.Config.StatsOptions.StatsMaxconn
			if o.Config.StatsOptions.StatsRefreshDelay != nil {
				cp.Config.StatsOptions.StatsRefreshDelay = new(int64)
				cp.Config.StatsOptions.StatsRefreshDelay = o.Config.StatsOptions.StatsRefreshDelay
			}
			if o.Config.StatsOptions.StatsShowDesc != nil {
				cp.Config.StatsOptions.StatsShowDesc = new(string)
				cp.Config.StatsOptions.StatsShowDesc = o.Config.StatsOptions.StatsShowDesc
			}
			cp.Config.StatsOptions.StatsShowLegends = o.Config.StatsOptions.StatsShowLegends
			if o.Config.StatsOptions.StatsShowNodeName != nil {
				cp.Config.StatsOptions.StatsShowNodeName = new(string)
				cp.Config.StatsOptions.StatsShowNodeName = o.Config.StatsOptions.StatsShowNodeName
			}
			cp.Config.StatsOptions.StatsURIPrefix = o.Config.StatsOptions.StatsURIPrefix
		}
		if o.Config.StickTable != nil {
			cp.Config.StickTable = new(models.ConfigStickTable)
			if o.Config.StickTable.Expire != nil {
				cp.Config.StickTable.Expire = new(int64)
				cp.Config.StickTable.Expire = o.Config.StickTable.Expire
			}
			if o.Config.StickTable.Keylen != nil {
				cp.Config.StickTable.Keylen = new(int64)
				cp.Config.StickTable.Keylen = o.Config.StickTable.Keylen
			}
			cp.Config.StickTable.Nopurge = o.Config.StickTable.Nopurge
			cp.Config.StickTable.Peers = o.Config.StickTable.Peers
			if o.Config.StickTable.Size != nil {
				cp.Config.StickTable.Size = new(int64)
				cp.Config.StickTable.Size = o.Config.StickTable.Size
			}
			cp.Config.StickTable.Store = o.Config.StickTable.Store
			cp.Config.StickTable.Type = o.Config.StickTable.Type
		}
		if o.Config.TunnelTimeout != nil {
			cp.Config.TunnelTimeout = new(int64)
			cp.Config.TunnelTimeout = o.Config.TunnelTimeout
		}
	}
	return cp
}
