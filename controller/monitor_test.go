package controller

import (
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMonitor_emptySyncPeriodInArgs(t *testing.T) {
	h := &HAProxyController{}
	period := h.syncPeriod()
	assert.Equal(t, "5s", period.String())
}

func TestMonitor_valuedSyncPeriodInArgs(t *testing.T) {
	h := &HAProxyController{
		osArgs: utils.OSArgs{
			SyncPeriod: 4 * time.Second,
		},
	}
	SetDefaultAnnotation("sync-period", h.osArgs.SyncPeriod.String())
	period := h.syncPeriod()
	assert.Equal(t, "4s", period.String())
}
