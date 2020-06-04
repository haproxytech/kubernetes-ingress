package utils

import (
	"os"
	"testing"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/assert"
)

func TestFlags_defaultSyncPeriod(t *testing.T) {
	var osArgs OSArgs
	var parser = flags.NewParser(&osArgs, flags.IgnoreUnknown)
	os.Args = []string{"nothing", "--sync-period=5s"}
	_, err := parser.Parse()
	if assert.Nil(t, err) {
		assert.Equal(t, 5*time.Second, osArgs.SyncPeriod)
	}
}
