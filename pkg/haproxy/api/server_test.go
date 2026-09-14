package api

import (
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/require"
)

// Only servers the controller itself put in maintenance are candidates for removal.
func TestMaintServersSelectsOnlyDisabledOnes(t *testing.T) {
	client := &clientNative{
		backends: map[string]Backend{
			"be1": {Backend: models.Backend{
				BackendBase: models.BackendBase{Name: "be1"},
				Servers: map[string]models.Server{
					"s01": {Name: "s01", ServerParams: models.ServerParams{Maintenance: "enabled"}},
					"s02": {Name: "s02", ServerParams: models.ServerParams{Maintenance: "disabled"}},
					"s03": {Name: "s03"},
				},
			}},
			"be2": {Backend: models.Backend{
				BackendBase: models.BackendBase{Name: "be2"},
				Servers: map[string]models.Server{
					"s04": {Name: "s04", ServerParams: models.ServerParams{Maintenance: "enabled"}},
				},
			}},
			"be3": {Backend: models.Backend{BackendBase: models.BackendBase{Name: "be3"}}},
		},
	}

	got := client.maintServers()

	require.Equal(t, map[string][]string{"be1": {"s01"}, "be2": {"s04"}}, got)
}
