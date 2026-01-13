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
	v3models "github.com/haproxytech/client-native/v3/models"
	"github.com/haproxytech/client-native/v5/models"
	corev1alpha2 "github.com/haproxytech/kubernetes-ingress/crs/api/core/v1alpha2"
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

//revive:disable-next-line:cyclomatic,function-length,cognitive-complexity
func DeepConvertGlobalSpecA2toV1(o corev1alpha2.GlobalSpec) v1.GlobalSpec {
	var cp v1.GlobalSpec
	if o.Config != nil {
		cp.Config = new(models.Global)

		if o.Config.CPUMaps != nil {
			cp.Config.CPUMaps = make([]*models.CPUMap, len(o.Config.CPUMaps))
			for i4 := range o.Config.CPUMaps {
				if o.Config.CPUMaps[i4] != nil {
					cp.Config.CPUMaps[i4] = new(models.CPUMap)

					if o.Config.CPUMaps[i4].CPUSet != nil {
						cp.Config.CPUMaps[i4].CPUSet = new(string)

						cp.Config.CPUMaps[i4].CPUSet = o.Config.CPUMaps[i4].CPUSet
					}
					if o.Config.CPUMaps[i4].Process != nil {
						cp.Config.CPUMaps[i4].Process = new(string)

						cp.Config.CPUMaps[i4].Process = o.Config.CPUMaps[i4].Process
					}
				}
			}
		}
		if o.Config.H1CaseAdjusts != nil {
			cp.Config.H1CaseAdjusts = make([]*models.H1CaseAdjust, len(o.Config.H1CaseAdjusts))
			for i4 := range o.Config.H1CaseAdjusts {
				if o.Config.H1CaseAdjusts[i4] != nil {
					cp.Config.H1CaseAdjusts[i4] = new(models.H1CaseAdjust)

					if o.Config.H1CaseAdjusts[i4].From != nil {
						cp.Config.H1CaseAdjusts[i4].From = new(string)

						cp.Config.H1CaseAdjusts[i4].From = o.Config.H1CaseAdjusts[i4].From
					}
					if o.Config.H1CaseAdjusts[i4].To != nil {
						cp.Config.H1CaseAdjusts[i4].To = new(string)

						cp.Config.H1CaseAdjusts[i4].To = o.Config.H1CaseAdjusts[i4].To
					}
				}
			}
		}
		if o.Config.RuntimeAPIs != nil {
			cp.Config.RuntimeAPIs = make([]*models.RuntimeAPI, len(o.Config.RuntimeAPIs))
			for i4 := range o.Config.RuntimeAPIs {
				if o.Config.RuntimeAPIs[i4] != nil {
					cp.Config.RuntimeAPIs[i4] = new(models.RuntimeAPI)

					if o.Config.RuntimeAPIs[i4].Address != nil {
						cp.Config.RuntimeAPIs[i4].Address = new(string)

						cp.Config.RuntimeAPIs[i4].Address = o.Config.RuntimeAPIs[i4].Address
					}
					cp.Config.RuntimeAPIs[i4].ExposeFdListeners = o.Config.RuntimeAPIs[i4].ExposeFdListeners
					cp.Config.RuntimeAPIs[i4].Level = o.Config.RuntimeAPIs[i4].Level
					cp.Config.RuntimeAPIs[i4].Mode = o.Config.RuntimeAPIs[i4].Mode
					cp.Config.RuntimeAPIs[i4].Process = o.Config.RuntimeAPIs[i4].Process
				}
			}
		}
		cp.Config.Chroot = o.Config.Chroot
		cp.Config.Daemon = o.Config.Daemon
		cp.Config.ExternalCheck = o.Config.ExternalCheck
		cp.Config.Group = o.Config.Group
		cp.Config.H1CaseAdjustFile = o.Config.H1CaseAdjustFile
		if o.Config.HardStopAfter != nil {
			cp.Config.HardStopAfter = new(int64)

			cp.Config.HardStopAfter = o.Config.HardStopAfter
		}
		cp.Config.Localpeer = o.Config.Localpeer
		if o.Config.LogSendHostname != nil {
			cp.Config.LogSendHostname = new(models.GlobalLogSendHostname)

			if o.Config.LogSendHostname.Enabled != nil {
				cp.Config.LogSendHostname.Enabled = new(string)

				cp.Config.LogSendHostname.Enabled = o.Config.LogSendHostname.Enabled
			}
			cp.Config.LogSendHostname.Param = o.Config.LogSendHostname.Param
		}
		if o.Config.LuaLoads != nil {
			cp.Config.LuaLoads = make([]*models.LuaLoad, len(o.Config.LuaLoads))
			for i4 := range o.Config.LuaLoads {
				if o.Config.LuaLoads[i4] != nil {
					cp.Config.LuaLoads[i4] = new(models.LuaLoad)

					if o.Config.LuaLoads[i4].File != nil {
						cp.Config.LuaLoads[i4].File = new(string)

						cp.Config.LuaLoads[i4].File = o.Config.LuaLoads[i4].File
					}
				}
			}
		}
		if o.Config.LuaPrependPath != nil {
			cp.Config.LuaPrependPath = make([]*models.LuaPrependPath, len(o.Config.LuaPrependPath))
			for i4 := range o.Config.LuaPrependPath {
				if o.Config.LuaPrependPath[i4] != nil {
					cp.Config.LuaPrependPath[i4] = new(models.LuaPrependPath)

					if o.Config.LuaPrependPath[i4].Path != nil {
						cp.Config.LuaPrependPath[i4].Path = new(string)

						cp.Config.LuaPrependPath[i4].Path = o.Config.LuaPrependPath[i4].Path
					}
					cp.Config.LuaPrependPath[i4].Type = o.Config.LuaPrependPath[i4].Type
				}
			}
		}
		cp.Config.MasterWorker = o.Config.MasterWorker
		cp.Config.Maxconn = o.Config.Maxconn
		cp.Config.Nbproc = o.Config.Nbproc
		cp.Config.Nbthread = o.Config.Nbthread
		cp.Config.Pidfile = o.Config.Pidfile
		cp.Config.ServerStateBase = o.Config.ServerStateBase
		cp.Config.ServerStateFile = o.Config.ServerStateFile
		cp.Config.SslDefaultBindCiphers = o.Config.SslDefaultBindCiphers
		cp.Config.SslDefaultBindCiphersuites = o.Config.SslDefaultBindCiphersuites
		cp.Config.SslDefaultBindOptions = o.Config.SslDefaultBindOptions
		cp.Config.SslDefaultServerCiphers = o.Config.SslDefaultServerCiphers
		cp.Config.SslDefaultServerCiphersuites = o.Config.SslDefaultServerCiphersuites
		cp.Config.SslDefaultServerOptions = o.Config.SslDefaultServerOptions
		cp.Config.SslModeAsync = o.Config.SslModeAsync
		if o.Config.StatsTimeout != nil {
			cp.Config.StatsTimeout = new(int64)
			cp.Config.StatsTimeout = o.Config.StatsTimeout
		}
		cp.Config.TuneSslDefaultDhParam = o.Config.TuneSslDefaultDhParam
		cp.Config.User = o.Config.User
		cp.Config.TuneOptions = Convert(o.Config.TuneOptions)
	}
	if o.LogTargets != nil {
		cp.LogTargets = make([]*models.LogTarget, len(o.LogTargets))
		for i2 := range o.LogTargets {
			if o.LogTargets[i2] != nil {
				cp.LogTargets[i2] = new(models.LogTarget)

				cp.LogTargets[i2].Address = o.LogTargets[i2].Address
				cp.LogTargets[i2].Facility = o.LogTargets[i2].Facility
				cp.LogTargets[i2].Format = o.LogTargets[i2].Format
				cp.LogTargets[i2].Global = o.LogTargets[i2].Global
				if o.LogTargets[i2].Index != nil {
					cp.LogTargets[i2].Index = new(int64)

					cp.LogTargets[i2].Index = o.LogTargets[i2].Index
				}
				cp.LogTargets[i2].Length = o.LogTargets[i2].Length
				cp.LogTargets[i2].Level = o.LogTargets[i2].Level
				cp.LogTargets[i2].Minlevel = o.LogTargets[i2].Minlevel
				cp.LogTargets[i2].Nolog = o.LogTargets[i2].Nolog
			}
		}
	}
	return cp
}

func Convert(globalTuneOptions *v3models.GlobalTuneOptions) *models.GlobalTuneOptions {
	if globalTuneOptions == nil {
		return nil
	}

	globalTuneOptionsV5 := &models.GlobalTuneOptions{}

	globalTuneOptionsV5.BuffersLimit = globalTuneOptions.BuffersLimit
	globalTuneOptionsV5.BuffersReserve = globalTuneOptions.BuffersReserve
	globalTuneOptionsV5.Bufsize = globalTuneOptions.Bufsize
	globalTuneOptionsV5.CompMaxlevel = globalTuneOptions.CompMaxlevel
	globalTuneOptionsV5.FailAlloc = globalTuneOptions.FailAlloc
	globalTuneOptionsV5.H2FeMaxConcurrentStreams = globalTuneOptions.H2MaxConcurrentStreams
	globalTuneOptionsV5.H2BeMaxConcurrentStreams = globalTuneOptions.H2MaxConcurrentStreams
	globalTuneOptionsV5.H2FeInitialWindowSize = utils.PointerDefaultValueIfNil(globalTuneOptions.H2InitialWindowSize)
	globalTuneOptionsV5.H2BeInitialWindowSize = utils.PointerDefaultValueIfNil(globalTuneOptions.H2InitialWindowSize)
	globalTuneOptionsV5.H2HeaderTableSize = globalTuneOptions.H2HeaderTableSize
	globalTuneOptionsV5.H2InitialWindowSize = globalTuneOptions.H2InitialWindowSize
	globalTuneOptionsV5.H2MaxConcurrentStreams = globalTuneOptions.H2MaxConcurrentStreams
	globalTuneOptionsV5.H2MaxFrameSize = globalTuneOptions.H2MaxFrameSize
	globalTuneOptionsV5.HTTPCookielen = globalTuneOptions.HTTPCookielen
	globalTuneOptionsV5.HTTPLogurilen = globalTuneOptions.HTTPLogurilen
	globalTuneOptionsV5.HTTPMaxhdr = globalTuneOptions.HTTPMaxhdr
	globalTuneOptionsV5.IdlePoolShared = globalTuneOptions.IdlePoolShared
	globalTuneOptionsV5.Idletimer = globalTuneOptions.Idletimer
	globalTuneOptionsV5.ListenerMultiQueue = globalTuneOptions.ListenerMultiQueue
	globalTuneOptionsV5.LuaForcedYield = globalTuneOptions.LuaForcedYield
	globalTuneOptionsV5.LuaMaxmem = globalTuneOptions.LuaMaxmem
	globalTuneOptionsV5.LuaServiceTimeout = globalTuneOptions.LuaServiceTimeout
	globalTuneOptionsV5.LuaSessionTimeout = globalTuneOptions.LuaSessionTimeout
	globalTuneOptionsV5.LuaTaskTimeout = globalTuneOptions.LuaTaskTimeout
	globalTuneOptionsV5.Maxaccept = globalTuneOptions.Maxaccept
	globalTuneOptionsV5.Maxpollevents = globalTuneOptions.Maxpollevents
	globalTuneOptionsV5.Maxrewrite = globalTuneOptions.Maxrewrite
	globalTuneOptionsV5.PatternCacheSize = globalTuneOptions.PatternCacheSize
	globalTuneOptionsV5.Pipesize = globalTuneOptions.Pipesize
	globalTuneOptionsV5.PoolHighFdRatio = globalTuneOptions.PoolHighFdRatio
	globalTuneOptionsV5.PoolLowFdRatio = globalTuneOptions.PoolLowFdRatio
	globalTuneOptionsV5.RcvbufClient = globalTuneOptions.RcvbufClient
	globalTuneOptionsV5.RcvbufServer = globalTuneOptions.RcvbufServer
	globalTuneOptionsV5.RecvEnough = globalTuneOptions.RecvEnough
	globalTuneOptionsV5.RunqueueDepth = globalTuneOptions.RunqueueDepth
	globalTuneOptionsV5.SchedLowLatency = globalTuneOptions.SchedLowLatency
	globalTuneOptionsV5.SndbufClient = globalTuneOptions.SndbufClient
	globalTuneOptionsV5.SndbufServer = globalTuneOptions.SndbufServer
	globalTuneOptionsV5.SslCachesize = globalTuneOptions.SslCachesize
	globalTuneOptionsV5.SslCaptureBufferSize = globalTuneOptions.SslCaptureBufferSize
	globalTuneOptionsV5.SslCtxCacheSize = globalTuneOptions.SslCtxCacheSize
	globalTuneOptionsV5.SslDefaultDhParam = globalTuneOptions.SslDefaultDhParam
	globalTuneOptionsV5.SslForcePrivateCache = globalTuneOptions.SslForcePrivateCache
	globalTuneOptionsV5.SslKeylog = globalTuneOptions.SslKeylog
	globalTuneOptionsV5.SslLifetime = globalTuneOptions.SslLifetime
	globalTuneOptionsV5.SslMaxrecord = globalTuneOptions.SslMaxrecord
	globalTuneOptionsV5.VarsGlobalMaxSize = globalTuneOptions.VarsGlobalMaxSize
	globalTuneOptionsV5.VarsProcMaxSize = globalTuneOptions.VarsProcMaxSize
	globalTuneOptionsV5.VarsReqresMaxSize = globalTuneOptions.VarsReqresMaxSize
	globalTuneOptionsV5.VarsSessMaxSize = globalTuneOptions.VarsSessMaxSize
	globalTuneOptionsV5.VarsTxnMaxSize = globalTuneOptions.VarsTxnMaxSize
	globalTuneOptionsV5.ZlibMemlevel = globalTuneOptions.ZlibMemlevel
	globalTuneOptionsV5.ZlibWindowsize = globalTuneOptions.ZlibWindowsize

	return globalTuneOptionsV5
}
