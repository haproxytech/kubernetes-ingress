package api

import (
	"fmt"
	"strconv"
	"strings"

	"dario.cat/mergo"
	cp_params "github.com/haproxytech/client-native/v6/config-parser/params"
	cn_options "github.com/haproxytech/client-native/v6/configuration/options"

	"github.com/haproxytech/client-native/v6/configuration"
	"github.com/haproxytech/client-native/v6/models"
	rutracker "github.com/haproxytech/kubernetes-ingress/pkg/runtime-update-tracker"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type addServerSpec struct {
	Args         string
	EnableHealth bool
	EnableAgent  bool
}

// buildAddServerSpec merges section and backend default-server, then serializes the runtime line.
func buildAddServerSpec(server models.Server, backend *models.Backend, sectionDefaults *models.DefaultServer) (addServerSpec, error) {
	merged := &models.DefaultServer{}
	if sectionDefaults != nil {
		if err := mergo.MergeWithOverwrite(merged, sectionDefaults); err != nil {
			return addServerSpec{}, fmt.Errorf("merging defaults section default-server: %w", err)
		}
	}
	if backend != nil && backend.DefaultServer != nil {
		if err := mergo.MergeWithOverwrite(merged, backend.DefaultServer); err != nil {
			return addServerSpec{}, fmt.Errorf("merging backend default-server: %w", err)
		}
	}

	var args strings.Builder
	args.WriteString(server.Address)
	if server.Port != nil {
		args.WriteString(":")
		args.WriteString(strconv.FormatInt(*server.Port, 10))
	}
	if server.Maintenance == "enabled" {
		args.WriteString(" disabled")
	}
	if options := serializeDefaultServerOptions(merged); options != "" {
		args.WriteString(" ")
		args.WriteString(options)
	}
	// Static cookie persistence pins clients by server name; the config line gets the same value.
	if backend != nil && backend.Cookie != nil && !backend.Cookie.Dynamic {
		args.WriteString(" cookie ")
		args.WriteString(server.Name)
	}

	return addServerSpec{
		Args:         args.String(),
		EnableHealth: merged.Check == "enabled",
		EnableAgent:  merged.AgentCheck == "enabled",
	}, nil
}

// BackendServerRuntimeCreate adds a server through the runtime socket and records the outcome in the tracker.
// Options the runtime rejects make `add server` fail; the caller then falls back to a config write and reload.
func (c *clientNative) BackendServerRuntimeCreate(backend *models.Backend, server models.Server, sectionDefaults *models.DefaultServer) error {
	backendName := backend.Name
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	tracker := rutracker.GetRuntimeUpdateTracker()
	fail := func(err error) error {
		tracker.TrackCommandForServer(backendName, server.Name, rutracker.TypeCreation, rutracker.StatusData{Status: rutracker.StatusFailure, Error: err})
		return err
	}

	spec, err := buildAddServerSpec(server, backend, sectionDefaults)
	if err != nil {
		return fail(err)
	}
	utils.GetLogger().Infof("[RUNTIME] [BACKEND] [SERVER] [CREATE] add server %s/%s %s", backendName, server.Name, spec.Args)
	if err := runtime.AddServer(backendName, server.Name, spec.Args); err != nil {
		return fail(err)
	}
	// Dynamic servers start with checks off; enable them explicitly.
	if spec.EnableHealth {
		if err := runtime.EnableServerHealth(backendName, server.Name); err != nil {
			return fail(err)
		}
	}
	if spec.EnableAgent {
		if err := runtime.EnableAgentCheck(backendName, server.Name); err != nil {
			return fail(err)
		}
	}
	if server.Maintenance != "enabled" {
		if err := runtime.EnableServer(backendName, server.Name); err != nil {
			return fail(err)
		}
	}
	tracker.TrackCommandForServer(backendName, server.Name, rutracker.TypeCreation, rutracker.StatusData{Status: rutracker.StatusSuccess})
	return nil
}

func serializeDefaultServerOptions(s *models.DefaultServer) string {
	serverParams := configuration.SerializeServerParams(s.ServerParams, &cn_options.ConfigurationOptions{
		PreferredTimeSuffix: "d",
	})
	return strings.TrimSpace(cp_params.ServerOptionsString(serverParams))
}

// BackendServerRuntimeDelete will use the Runtime socket to delete a Backend server
// The runtime commands status are stored in the RuntimeUpdateTracker
func (c *clientNative) BackendServerRuntimeDelete(backendName, serverName string) (err error) {
	// runtime socket
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}

	// Command status tracker
	tracker := rutracker.GetRuntimeUpdateTracker()

	err = runtime.DeleteServer(backendName, serverName)
	utils.GetLogger().Debugf("[RUNTIME] [BACKEND] [SERVER] [DEL] del server %s/%s", backendName, serverName)
	if err != nil && !strings.Contains(err.Error(), "No such server") && !strings.Contains(err.Error(), "No such backend") {
		tracker.TrackCommandForServer(backendName, serverName, rutracker.TypeDeletion, rutracker.StatusData{
			Status: rutracker.StatusFailure,
			Error:  err,
		})
		return err
	}
	tracker.TrackCommandForServer(backendName, serverName, rutracker.TypeDeletion, rutracker.StatusData{
		Status: rutracker.StatusSuccess,
		Error:  err,
	})

	return nil
}
