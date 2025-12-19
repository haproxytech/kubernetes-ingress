package rutracker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

//revive:disable:function-length
func TestRuntimeUpdateTracker_TrackCommandForServer(t *testing.T) {
	backendName := "test-backend"
	serverName := "test-server"
	key := getTrackingKeyForServer(backendName, serverName)

	testCases := []struct {
		name              string
		ruType            RuntimeCommandType
		statusData        StatusData
		initialState      *TrackingData
		expectedStatus    Status
		expectedCmdStatus Status
	}{
		{
			name:              "Creation - First command is Success",
			ruType:            TypeCreation,
			statusData:        StatusData{Status: StatusSuccess},
			expectedStatus:    StatusSuccess,
			expectedCmdStatus: StatusSuccess,
		},
		{
			name:              "Creation - First command is Failure",
			ruType:            TypeCreation,
			statusData:        StatusData{Status: StatusFailure, Error: errors.New("creation failed")},
			expectedStatus:    StatusFailure,
			expectedCmdStatus: StatusFailure,
		},
		{
			name:       "Creation - Success after Failure",
			ruType:     TypeCreation,
			statusData: StatusData{Status: StatusSuccess},
			initialState: &TrackingData{
				GlobalStatus: StatusFailure,
				CommandsStatus: map[RuntimeCommandType]StatusData{
					TypeCreation: {Status: StatusFailure},
				},
			},
			expectedStatus:    StatusFailure,
			expectedCmdStatus: StatusSuccess,
		},
		{
			name:       "Creation - Failure after Success",
			ruType:     TypeCreation,
			statusData: StatusData{Status: StatusFailure, Error: errors.New("creation failed after success")},
			initialState: &TrackingData{
				GlobalStatus: StatusSuccess,
				CommandsStatus: map[RuntimeCommandType]StatusData{
					TypeCreation: {Status: StatusSuccess},
				},
			},
			expectedStatus:    StatusFailure,
			expectedCmdStatus: StatusFailure,
		},
		{
			name:       "Creation - Success after Success",
			ruType:     TypeCreation,
			statusData: StatusData{Status: StatusSuccess},
			initialState: &TrackingData{
				GlobalStatus: StatusSuccess,
				CommandsStatus: map[RuntimeCommandType]StatusData{
					TypeCreation: {Status: StatusSuccess},
				},
			},
			expectedStatus:    StatusSuccess,
			expectedCmdStatus: StatusSuccess,
		},
		{
			name:              "Update - First command is Success",
			ruType:            TypeUpdate,
			statusData:        StatusData{Status: StatusSuccess},
			expectedStatus:    StatusSuccess,
			expectedCmdStatus: StatusSuccess,
		},
		{
			name:              "Update - First command is Failure",
			ruType:            TypeUpdate,
			statusData:        StatusData{Status: StatusFailure, Error: errors.New("update failed")},
			expectedStatus:    StatusFailure,
			expectedCmdStatus: StatusFailure,
		},
		{
			name:       "Update - Failure after Success",
			ruType:     TypeUpdate,
			statusData: StatusData{Status: StatusFailure, Error: errors.New("update failed again")},
			initialState: &TrackingData{
				GlobalStatus: StatusSuccess,
				CommandsStatus: map[RuntimeCommandType]StatusData{
					TypeUpdate: {Status: StatusSuccess},
				},
			},
			expectedStatus:    StatusFailure,
			expectedCmdStatus: StatusFailure,
		},
		{
			name:              "Deletion - Success must not mark the server as created at runtime",
			ruType:            TypeDeletion,
			statusData:        StatusData{Status: StatusSuccess},
			expectedStatus:    StatusUnknown,
			expectedCmdStatus: StatusSuccess,
		},
		{
			name:       "Deletion - Success after a failed creation keeps Failure",
			ruType:     TypeDeletion,
			statusData: StatusData{Status: StatusSuccess},
			initialState: &TrackingData{
				GlobalStatus: StatusFailure,
				CommandsStatus: map[RuntimeCommandType]StatusData{
					TypeCreation: {Status: StatusFailure},
				},
			},
			expectedStatus:    StatusFailure,
			expectedCmdStatus: StatusSuccess,
		},
		{
			name:              "Deletion - Failure",
			ruType:            TypeDeletion,
			statusData:        StatusData{Status: StatusFailure, Error: errors.New("deletion failed")},
			expectedStatus:    StatusUnknown, // Deletion does not affect global status
			expectedCmdStatus: StatusFailure,
		},
		{
			name:       "Deletion - Failure with existing success status",
			ruType:     TypeDeletion,
			statusData: StatusData{Status: StatusFailure, Error: errors.New("deletion failed")},
			initialState: &TrackingData{
				GlobalStatus: StatusSuccess,
				CommandsStatus: map[RuntimeCommandType]StatusData{
					TypeCreation: {Status: StatusSuccess},
				},
			},
			expectedStatus:    StatusSuccess, // Global status should not change
			expectedCmdStatus: StatusFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			tracker := GetRuntimeUpdateTracker()
			tracker.Reset()

			if tc.initialState != nil {
				// Deep copy to avoid race conditions if tests were parallel
				initial := TrackingData{
					GlobalStatus:   tc.initialState.GlobalStatus,
					CommandsStatus: make(map[RuntimeCommandType]StatusData),
				}
				for k, v := range tc.initialState.CommandsStatus {
					initial.CommandsStatus[k] = v
				}
				tracker.server[key] = initial
			}

			// Execute
			tracker.TrackCommandForServer(backendName, serverName, tc.ruType, tc.statusData)

			// Verify
			trackedData, ok := tracker.server[key]
			assert.True(t, ok, "server tracking data should exist")

			// Check Global Status
			assert.Equal(t, tc.expectedStatus, trackedData.GlobalStatus, "GlobalStatus should be correct")

			// Check Command Status
			cmdStatus, ok := trackedData.CommandsStatus[tc.ruType]
			assert.True(t, ok, "command status for the type should exist")
			assert.Equal(t, tc.expectedCmdStatus, cmdStatus.Status, "Command status should be correct")
			if tc.statusData.Status == StatusFailure {
				assert.Error(t, cmdStatus.Error, "Error should be set on failure")
				assert.Equal(t, tc.statusData.Error, cmdStatus.Error)
			} else {
				assert.NoError(t, cmdStatus.Error, "Error should not be set on success")
			}
		})
	}
}
