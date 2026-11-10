package instance

import (
	"fmt"
	"runtime"
	"sync/atomic"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var DefaultConfigurationManager = NewConfigurationManager()

func Reload(reason string, args ...any) {
	DefaultConfigurationManager.SetReload(reason, args...)
}

func ReloadIf(reload bool, reason string, args ...any) {
	DefaultConfigurationManager.SetReloadIf(reload, reason, args...)
}

func NeedReload() bool {
	return DefaultConfigurationManager.NeedReload()
}

// ConsumeReload takes the pending request before a reload attempt. Requests
// arriving during the attempt remain pending; a failed attempt must re-arm it.
func ConsumeReload() bool {
	return DefaultConfigurationManager.ConsumeReload()
}

func Reset() {
	DefaultConfigurationManager.Reset()
}

type configurationManagerImpl struct {
	logger utils.Logger
	// Map and certificate files are written by parallel goroutines that each
	// mark the configuration as needing a reload, so the flag must be atomic.
	reload atomic.Bool
}

func NewConfigurationManager() *configurationManagerImpl {
	return &configurationManagerImpl{
		logger: utils.GetLogger(),
	}
}

func (cmi *configurationManagerImpl) SetReload(reason string, args ...any) {
	cmi.reload.Store(true)
	if !cmi.validReason(reason) {
		return
	}
	cmi.logger.InfoSkipCallerf("reload required : "+reason, args...)
}

func (cmi *configurationManagerImpl) Reset() {
	cmi.reload.Store(false)
}

func (cmi *configurationManagerImpl) NeedReload() bool {
	return cmi.reload.Load()
}

func (cmi *configurationManagerImpl) ConsumeReload() bool {
	return cmi.reload.Swap(false)
}

func (cmi *configurationManagerImpl) SetReloadIf(reload bool, reason string, args ...any) {
	if !reload {
		return
	}
	cmi.reload.Store(true)
	if !cmi.validReason(reason) {
		return
	}
	cmi.logger.InfoSkipCallerf("reload required : "+reason, args...)
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
