package configuration

import (
	"fmt"
	"runtime/debug"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

var DefaultConfigurationManager = NewConfigurationManager()
var Reload = DefaultConfigurationManager.SetReload
var Restart = DefaultConfigurationManager.SetRestart
var ReloadIf = DefaultConfigurationManager.SetReloadIf
var RestartIf = DefaultConfigurationManager.SetRestartIf
var GetReload = DefaultConfigurationManager.GetReload
var GetRestart = DefaultConfigurationManager.GetRestart
var Reset = DefaultConfigurationManager.Reset

type ConfigurationManager interface {
	SetReload(reason string)
	SetRestart(reason string)
	Reset()
	GetReload() bool
	GetRestart() bool
	SetReloadIf(reload bool, reason string)
}

type configurationManagerImpl struct {
	reload, restart bool
}

func NewConfigurationManager() *configurationManagerImpl {
	return &configurationManagerImpl{}
}

func (cmi *configurationManagerImpl) SetReload(reason string, args ...string) {
	cmi.reload = true
	if reason == "" {
		logger.InfoDelegated("empty reason for reload")
		logger.Debug(debug.Stack())
		return
	}
	logger.InfoDelegated("reload required : " + fmt.Sprintf(reason, args))
}
func (cmi *configurationManagerImpl) SetRestart(reason string, args ...string) {
	cmi.restart = true
	if reason == "" {
		logger.InfoDelegated("empty reason for restart")
		logger.Debug(debug.Stack())
		return
	}
	logger.InfoDelegated("restart required : " + fmt.Sprintf(reason, args))
}

func (cmi *configurationManagerImpl) Reset() {
	cmi.reload = false
	cmi.restart = false
}

func (cmi *configurationManagerImpl) GetReload() bool {
	return cmi.reload
}
func (cmi *configurationManagerImpl) GetRestart() bool {
	return cmi.restart
}

func (cmi *configurationManagerImpl) SetReloadIf(reload bool, reason string, args ...string) {
	cmi.reload = cmi.reload || reload
	if reload {
		if reason == "" {
			logger.InfoDelegated("empty reason for reload")
			logger.Debug(debug.Stack())
			return
		}
		logger.InfoDelegated("reload required : " + fmt.Sprintf(reason, args))
	}
}

func (cmi *configurationManagerImpl) SetRestartIf(restart bool, reason string, args ...string) {
	cmi.restart = cmi.restart || restart
	if restart {
		if reason == "" {
			logger.InfoDelegated("empty reason for restart")
			logger.Debug(debug.Stack())
			return
		}
		logger.InfoDelegated("restart required : " + fmt.Sprintf(reason, args))
	}
}
