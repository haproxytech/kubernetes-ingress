package rutracker

import (
	"strings"
	"sync"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var (
	runtimeUpdateTrackerSingleton *RuntimeUpdateTracker
	doOnce                        sync.Once
)

type RuntimeUpdateTracker struct {
	mu sync.RWMutex
	// server contains the tracking of all runtime commands performed on a given server
	// TrackingKey is the server name (backend/server) from
	//   func (rb *RuntimeBackend) ComputeServerName(address string) string {
	server map[trackingKey]TrackingData
}

type (
	trackingKey        string
	RuntimeCommandType string
	Status             string
)

const (
	StatusUnknown = "Unknown"
	StatusSuccess = "Success"
	StatusFailure = "Failure"
)

const (
	TypeCreation = "Create"
	TypeDeletion = "Delete"
	TypeUpdate   = "Update"
)

type TrackingData struct {
	// GlobalStatus is:
	// StatusSuccess if all DetailedStatus are StatusSuccess
	// StatusFailure if at least one DetailedStatus for 1 command failed
	GlobalStatus   Status
	CommandsStatus map[RuntimeCommandType]StatusData
}

type StatusData struct {
	Status Status
	Error  error // set if Status == StatusFailure
}

func GetRuntimeUpdateTracker() *RuntimeUpdateTracker {
	doOnce.Do(func() {
		runtimeUpdateTrackerSingleton = &RuntimeUpdateTracker{
			server: make(map[trackingKey]TrackingData),
		}
	})
	return runtimeUpdateTrackerSingleton
}

func getTrackingKeyForServer(backendName, serverName string) trackingKey {
	return trackingKey(backendName + "/" + serverName)
}

func (rut *RuntimeUpdateTracker) Reset() {
	rut.mu.Lock()
	defer rut.mu.Unlock()
	rut.server = make(map[trackingKey]TrackingData)
}

func (rut *RuntimeUpdateTracker) ensureServerRuntimeTracking(key trackingKey, ruType RuntimeCommandType) {
	if _, ok := rut.server[key]; !ok {
		rut.server[key] = TrackingData{
			CommandsStatus: make(map[RuntimeCommandType]StatusData),
			GlobalStatus:   StatusUnknown,
		}
	}
	if _, ok := rut.server[key].CommandsStatus[ruType]; !ok {
		rut.server[key].CommandsStatus[ruType] = StatusData{
			Status: StatusUnknown,
		}
	}
}

func (rut *RuntimeUpdateTracker) TrackCommandForServer(backendName, serverName string, ruType RuntimeCommandType, data StatusData) {
	rut.mu.Lock()
	defer rut.mu.Unlock()
	key := getTrackingKeyForServer(backendName, serverName)
	rut.ensureServerRuntimeTracking(key, ruType)

	switch data.Status {
	case StatusFailure:
		if data.Error != nil && strings.Contains(data.Error.Error(), "No such backend") {
			// New backend, not yet reloaded into haproxy: the reload creates the server anyway.
			utils.GetLogger().Debugf("[RUNTIME] [SERVER] [%s] - %s: backend not in running process yet", ruType, key)
		} else {
			utils.GetLogger().Infof("[RUNTIME] [SERVER] [%s] - failure - %s [trigger reload]. err: %v", ruType, key, data.Error)
		}
	case StatusSuccess:
		utils.GetLogger().Debugf("[RUNTIME] [SERVER] [%s] - success - %s ", ruType, key)
	}

	trackingDatas := rut.server[key]
	// Global Status
	switch ruType {
	case TypeCreation, TypeUpdate:
		if data.Status == StatusFailure {
			// Any failure => GlobalStatus is Failure
			trackingDatas.GlobalStatus = StatusFailure
		} else if data.Status == StatusSuccess && trackingDatas.GlobalStatus == StatusUnknown {
			// GlobalStatus still unknown and success => Success
			trackingDatas.GlobalStatus = StatusSuccess
		}
	case TypeDeletion:
		// A deletion says nothing about whether the server exists in the process.
	}

	trackingDatas.CommandsStatus[ruType] = data
	rut.server[key] = trackingDatas
}

func (rut *RuntimeUpdateTracker) GlobalStatusForServer(backendName, serverName string) Status {
	rut.mu.RLock()
	defer rut.mu.RUnlock()
	key := getTrackingKeyForServer(backendName, serverName)
	trackingDatas, ok := rut.server[key]
	if !ok {
		return StatusUnknown
	}
	status := trackingDatas.GlobalStatus
	return status
}
