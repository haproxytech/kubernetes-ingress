// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const twoRunnersJSON = `[
  {"id":110,"description":"ded8116-golang","status":"online","online":true,"paused":false,"tag_list":["go","go-r1","go-g1"]},
  {"id":111,"description":"ded8117-golang","status":"online","online":true,"paused":false,"tag_list":["go","go-g1","go-r2"]}
]`

const oneRunnerJSON = `{"id":155,"description":"ded6697-golang","status":"online","online":true,"paused":false,"tag_list":["go","go-g2","go-r8"]}`

// TestDecodeRunnersAcceptsArrayResponse covers the /next?count=N shape.
func TestDecodeRunnersAcceptsArrayResponse(t *testing.T) {
	runners, err := decodeRunners([]byte(twoRunnersJSON))
	require.NoError(t, err)
	require.Len(t, runners, 2)
	assert.Equal(t, 110, runners[0].ID)
	assert.Equal(t, []string{"go", "go-g1", "go-r2"}, runners[1].TagList)
}

// TestDecodeRunnersAcceptsSingleObjectResponse covers /next?count=1, which
// returns a bare object rather than an array.
func TestDecodeRunnersAcceptsSingleObjectResponse(t *testing.T) {
	runners, err := decodeRunners([]byte(oneRunnerJSON))
	require.NoError(t, err)
	require.Len(t, runners, 1)
	assert.Equal(t, 155, runners[0].ID)
	assert.Equal(t, "ded6697-golang", runners[0].Description)
}

func TestFetchRunnersRequestsTheAskedCount(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		_, _ = w.Write([]byte(twoRunnersJSON))
	}))
	defer srv.Close()

	runners, err := fetchRunners(srv.Client(), srv.URL, 2)
	require.NoError(t, err)
	assert.Equal(t, "/next?count=2", gotURI)
	assert.Len(t, runners, 2)
}

func TestFetchRunnersRejectsANonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := fetchRunners(srv.Client(), srv.URL, 2)
	require.Error(t, err)
}

func TestFetchRunnersWithRetryRecoversFromATransientFailure(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(twoRunnersJSON))
	}))
	defer srv.Close()

	runners, err := fetchRunnersWithRetry(srv.Client(), srv.URL, 2, 3, time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
	assert.Len(t, runners, 2)
}

func TestFetchRunnersWithRetryGivesUpAfterAllAttempts(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchRunnersWithRetry(srv.Client(), srv.URL, 4, 3, time.Millisecond)
	require.Error(t, err)
	assert.Equal(t, 3, calls)
}

// TestPinTagsPicksTheGoRTagWhereverItSits guards against reading tag_list by
// index: the go-r* tag is second on one runner and third on another.
func TestPinTagsPicksTheGoRTagWhereverItSits(t *testing.T) {
	runners := []Runner{
		{ID: 110, Online: true, TagList: []string{"go", "go-r1", "go-g1"}},
		{ID: 111, Online: true, TagList: []string{"go", "go-g1", "go-r2"}},
	}
	assert.Equal(t, []string{"go-r1", "go-r2"}, pinTags(runners, defaultTagPattern))
}

func TestPinTagsSkipsRunnersWithoutAGoRTag(t *testing.T) {
	runners := []Runner{
		{ID: 227, Online: true, TagList: []string{"go"}},
		{ID: 126, Online: true, TagList: []string{"go", "go-r3", "go-g2"}},
	}
	assert.Equal(t, []string{"go-r3"}, pinTags(runners, defaultTagPattern))
}

func TestPinTagsSkipsOfflineAndPausedRunners(t *testing.T) {
	runners := []Runner{
		{ID: 110, Online: false, TagList: []string{"go", "go-r1"}},
		{ID: 111, Online: true, Paused: true, TagList: []string{"go", "go-r2"}},
		{ID: 126, Online: true, TagList: []string{"go", "go-r3"}},
	}
	assert.Equal(t, []string{"go-r3"}, pinTags(runners, defaultTagPattern))
}

func TestPinTagsDropsDuplicates(t *testing.T) {
	runners := []Runner{
		{ID: 110, Online: true, TagList: []string{"go", "go-r1"}},
		{ID: 110, Online: true, TagList: []string{"go", "go-r1"}},
	}
	assert.Equal(t, []string{"go-r1"}, pinTags(runners, defaultTagPattern))
}

func TestPinTagsHonoursACustomPattern(t *testing.T) {
	runners := []Runner{{ID: 110, Online: true, TagList: []string{"go", "go-r1", "go-g1"}}}
	assert.Equal(t, []string{"go-g1"}, pinTags(runners, regexp.MustCompile(`^go-g[0-9]+$`)))
}

func TestAssignTagsWrapsWhenLegsOutnumberRunners(t *testing.T) {
	assert.Equal(t, []string{"go-r1", "go-r2", "go-r1", "go-r2"}, assignTags(4, []string{"go-r1", "go-r2"}))
}

func TestAssignTagsLeavesLegsUnpinnedWithoutRunners(t *testing.T) {
	assert.Equal(t, []string{"", "", "", ""}, assignTags(4, nil))
}
