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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	jobsRunning = `[{"id":11,"name":"unit-tests","status":"success"},{"id":22,"name":"docker-build","status":"running"}]`
	jobsSuccess = `[{"id":11,"name":"unit-tests","status":"success"},{"id":22,"name":"docker-build","status":"success"}]`
	jobsFailed  = `[{"id":22,"name":"docker-build","status":"failed"}]`
)

func TestFindBuildJobPicksTheNamedJob(t *testing.T) {
	job, err := findBuildJob([]byte(jobsSuccess), "docker-build")
	require.NoError(t, err)
	assert.Equal(t, 22, job.ID)
	assert.Equal(t, "success", job.Status)
}

func TestFindBuildJobReportsAMissingJob(t *testing.T) {
	_, err := findBuildJob([]byte(jobsSuccess), "absent")
	require.Error(t, err)
}

// TestWaitForArtifactDownloadsOnceTheBuildSucceeds also covers the poll loop:
// the first look finds the build still running.
func TestWaitForArtifactDownloadsOnceTheBuildSucceeds(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/projects/9/pipelines/77/jobs":
			polls++
			if polls == 1 {
				_, _ = w.Write([]byte(jobsRunning))
				return
			}
			_, _ = w.Write([]byte(jobsSuccess))
		case r.URL.Path == "/projects/9/jobs/22/artifacts/tar/k8sIC.tar":
			assert.Equal(t, "secret", r.Header.Get("PRIVATE-TOKEN"))
			_, _ = w.Write([]byte("TARBALL"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "tar", "k8sIC.tar")
	cfg := config{
		APIURL: srv.URL, ProjectID: "9", PipelineID: "77", Token: "secret",
		JobName: "docker-build", ArtifactPath: "tar/k8sIC.tar", Output: out,
		Timeout: 5 * time.Second, Interval: time.Millisecond,
	}
	require.NoError(t, waitForArtifact(srv.Client(), cfg))

	body, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "TARBALL", string(body))
	assert.Equal(t, 2, polls)
}

// TestWaitForArtifactStopsWhenTheBuildFails is the whole point of polling the
// job status: six e2e jobs must not hold runners for a build that will never
// publish anything.
func TestWaitForArtifactStopsWhenTheBuildFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(jobsFailed))
	}))
	defer srv.Close()

	cfg := config{
		APIURL: srv.URL, ProjectID: "9", PipelineID: "77", Token: "secret",
		JobName: "docker-build", ArtifactPath: "tar/k8sIC.tar",
		Output:  filepath.Join(t.TempDir(), "k8sIC.tar"),
		Timeout: time.Minute, Interval: time.Millisecond,
	}
	err := waitForArtifact(srv.Client(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}

func TestWaitForArtifactGivesUpAtTheTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(jobsRunning))
	}))
	defer srv.Close()

	cfg := config{
		APIURL: srv.URL, ProjectID: "9", PipelineID: "77", Token: "secret",
		JobName: "docker-build", ArtifactPath: "tar/k8sIC.tar",
		Output:  filepath.Join(t.TempDir(), "k8sIC.tar"),
		Timeout: 30 * time.Millisecond, Interval: time.Millisecond,
	}
	err := waitForArtifact(srv.Client(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestConfigFromEnvRejectsAMissingToken(t *testing.T) {
	t.Setenv("CI_API_V4_URL", "https://gitlab.example/api/v4")
	t.Setenv("CI_PROJECT_ID", "9")
	t.Setenv("PARENT_PIPELINE_ID", "77")
	t.Setenv("GITLAB_TOKEN", "")

	_, err := configFromEnv()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITLAB_TOKEN")
}

func TestConfigFromEnvFillsTheDefaults(t *testing.T) {
	t.Setenv("CI_API_V4_URL", "https://gitlab.example/api/v4")
	t.Setenv("CI_PROJECT_ID", "9")
	t.Setenv("PARENT_PIPELINE_ID", "77")
	t.Setenv("GITLAB_TOKEN", "secret")

	cfg, err := configFromEnv()
	require.NoError(t, err)
	assert.Equal(t, "docker-build", cfg.JobName)
	assert.Equal(t, "tar/k8sIC.tar", cfg.ArtifactPath)
	assert.Equal(t, "tar/k8sIC.tar", cfg.Output)
	assert.Positive(t, cfg.Timeout)
}
