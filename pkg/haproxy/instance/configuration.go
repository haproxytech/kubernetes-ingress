package instance

import (
	"fmt"
	"runtime/debug"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var DefaultConfigurationManager = NewConfigurationManager()

func Reload(reason string, args ...any) {
	DefaultConfigurationManager.SetReload(reason, args...)
}

func Restart(reason string, args ...any) {
	DefaultConfigurationManager.SetRestart(reason, args...)
}

func ReloadIf(reload bool, reason string, args ...any) {
	DefaultConfigurationManager.SetReloadIf(reload, reason, args...)
}

func RestartIf(restart bool, reason string, args ...any) {
	DefaultConfigurationManager.SetRestartIf(restart, reason, args...)
}

func NeedReload() bool {
	return DefaultConfigurationManager.NeedReload()
}

func NeedRestart() bool {
	return DefaultConfigurationManager.NeedRestart()
}

func Reset() {
	DefaultConfigurationManager.Reset()
}

func NeedAction() bool {
	return DefaultConfigurationManager.NeedAction()
}

type configurationManagerImpl struct {
	reload, restart bool
	logger          utils.Logger
}

func NewConfigurationManager() *configurationManagerImpl {
	return &configurationManagerImpl{
		logger: utils.GetLogger(),
	}
}

func (cmi *configurationManagerImpl) SetReload(reason string, args ...any) {
	cmi.reload = true
	if reason == "" {
		cmi.logger.Error("empty reason for reload")
		cmi.logger.Debug(debug.Stack())
		return
	}
	cmi.logger.Info("reload required : " + fmt.Sprintf(reason, args...))
}

func (cmi *configurationManagerImpl) SetRestart(reason string, args ...any) {
	cmi.restart = true
	if reason == "" {
		cmi.logger.Error("empty reason for restart")
		cmi.logger.Debug(debug.Stack())
		return
	}
	cmi.logger.Info("restart required : " + fmt.Sprintf(reason, args...))
}

func (cmi *configurationManagerImpl) Reset() {
	cmi.reload = false
	cmi.restart = false
}

func (cmi *configurationManagerImpl) NeedReload() bool {
	return cmi.reload
}

func (cmi *configurationManagerImpl) NeedRestart() bool {
	return cmi.restart
}

func (cmi *configurationManagerImpl) SetReloadIf(reload bool, reason string, args ...any) {
	if !reload {
		return
	}
	cmi.reload = true
	if reason == "" {
		cmi.logger.Error("empty reason for reload")
		cmi.logger.Debug(debug.Stack())
		return
	}
	cmi.logger.Info("reload required : " + fmt.Sprintf(reason, args...))
}

func (cmi *configurationManagerImpl) SetRestartIf(restart bool, reason string, args ...any) {
	if !restart {
		return
	}
	cmi.restart = true
	if reason == "" {
		cmi.logger.Error("empty reason for restart")
		cmi.logger.Debug(debug.Stack())
		return
	}
	cmi.logger.Info("restart required : " + fmt.Sprintf(reason, args...))
}

func (cmi *configurationManagerImpl) NeedAction() bool {
	return cmi.NeedReload() || cmi.NeedRestart()
}
