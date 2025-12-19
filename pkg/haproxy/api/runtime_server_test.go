package api

import (
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"github.com/stretchr/testify/require"
)

func addServerFixture() models.Server {
	return models.Server{
		Name:         "s01",
		Address:      "10.0.0.1",
		Port:         utils.PtrInt64(8080),
		ServerParams: models.ServerParams{Maintenance: "disabled"},
	}
}

func TestBuildAddServerSpecUsesDefaultsSectionCheck(t *testing.T) {
	// Plain Ingress without the `check` annotation leaves the backend default-server nil.
	backend := &models.Backend{BackendBase: models.BackendBase{Name: "be"}}
	sectionDefaults := &models.DefaultServer{ServerParams: models.ServerParams{Check: "enabled", Inter: utils.PtrInt64(2000)}}

	spec, err := buildAddServerSpec(addServerFixture(), backend, sectionDefaults)

	require.NoError(t, err)
	require.True(t, spec.EnableHealth)
	require.Contains(t, spec.Args, "10.0.0.1:8080")
	require.Contains(t, spec.Args, " check")
	require.Contains(t, spec.Args, "inter 2s")
}

func TestBuildAddServerSpecBackendOverridesSection(t *testing.T) {
	backend := &models.Backend{
		BackendBase: models.BackendBase{
			Name:          "be",
			DefaultServer: &models.DefaultServer{ServerParams: models.ServerParams{Inter: utils.PtrInt64(150)}},
		},
	}
	sectionDefaults := &models.DefaultServer{ServerParams: models.ServerParams{Inter: utils.PtrInt64(2000)}}

	spec, err := buildAddServerSpec(addServerFixture(), backend, sectionDefaults)

	require.NoError(t, err)
	require.Contains(t, spec.Args, "inter 150")
	require.NotContains(t, spec.Args, "inter 2s")
}

func TestBuildAddServerSpecAddsCookieForStaticPersistence(t *testing.T) {
	backend := &models.Backend{BackendBase: models.BackendBase{
		Name:   "be",
		Cookie: &models.Cookie{Name: utils.PtrString("mycookie"), Dynamic: false},
	}}

	spec, err := buildAddServerSpec(addServerFixture(), backend, nil)

	require.NoError(t, err)
	require.Contains(t, spec.Args, " cookie s01")
}

func TestBuildAddServerSpecNoCookieForDynamicPersistence(t *testing.T) {
	backend := &models.Backend{BackendBase: models.BackendBase{
		Name:   "be",
		Cookie: &models.Cookie{Name: utils.PtrString("mycookie"), Dynamic: true},
	}}

	spec, err := buildAddServerSpec(addServerFixture(), backend, nil)

	require.NoError(t, err)
	require.NotContains(t, spec.Args, "cookie")
}

func TestBuildAddServerSpecDisabledServer(t *testing.T) {
	server := addServerFixture()
	server.Maintenance = "enabled"
	backend := &models.Backend{BackendBase: models.BackendBase{Name: "be"}}

	spec, err := buildAddServerSpec(server, backend, nil)

	require.NoError(t, err)
	require.Contains(t, spec.Args, " disabled")
	require.False(t, spec.EnableHealth)
}
