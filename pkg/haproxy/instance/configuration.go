package instance

import (
	"fmt"
	"runtime"

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
	logger          utils.Logger
	reload, restart bool
}

func NewConfigurationManager() *configurationManagerImpl {
	return &configurationManagerImpl{
		logger: utils.GetLogger(),
	}
}

func (cmi *configurationManagerImpl) SetReload(reason string, args ...any) {
	cmi.reload = true
	if !cmi.validReason(reason) {
		return
	}
	cmi.logger.InfoSkipCallerf("reload required : "+reason, args...)
}

func (cmi *configurationManagerImpl) SetRestart(reason string, args ...any) {
	cmi.restart = true
	if !cmi.validReason(reason) {
		return
	}
	cmi.logger.InfoSkipCallerf("restart required : "+reason, args...)
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
	if !cmi.validReason(reason) {
		return
	}
	cmi.logger.InfoSkipCallerf("reload required : "+reason, args...)
}

func (cmi *configurationManagerImpl) SetRestartIf(restart bool, reason string, args ...any) {
	if !restart {
		return
	}
	cmi.restart = true
	if !cmi.validReason(reason) {
		return
	}
	cmi.logger.InfoSkipCallerf("restart required : "+reason, args...)
}

func (cmi *configurationManagerImpl) NeedAction() bool {
	return cmi.NeedReload() || cmi.NeedRestart()
}

func (cmi *configurationManagerImpl) validReason(reason string) bool {
	if reason == "" {
		errMsg := "empty reason for reload"
		_, file, line, ok := runtime.Caller(3)
		if ok {
			errMsg += fmt.Sprintf(" from %s:%d", file, line)
		}
		cmi.logger.Error(errMsg)
		return false
	}
	return true
}
