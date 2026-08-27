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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"
)

// job is the part of a GitLab job the fetcher needs.
type job struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	ID     int    `json:"id"`
}

// config is everything the fetcher reads from the environment.
type config struct {
	APIURL       string
	ProjectID    string
	PipelineID   string
	Token        string
	JobName      string
	ArtifactPath string
	Output       string
	Timeout      time.Duration
	Interval     time.Duration
}

const (
	defaultJobName      = "docker-build"
	defaultArtifactPath = "tar/k8sIC.tar"
	defaultTimeout      = 15 * time.Minute
	defaultInterval     = 5 * time.Second
)

// deadStatuses are the states from which the build will never publish.
var deadStatuses = []string{"failed", "canceled", "skipped"}

var (
	errNoJob    = errors.New("job not found in the parent pipeline")
	errBuild    = errors.New("build job will not publish an artifact")
	errTimedOut = errors.New("timed out waiting for the build artifact")
)

// findBuildJob picks the named job out of a GitLab jobs listing.
func findBuildJob(body []byte, name string) (job, error) {
	var jobs []job
	if err := json.Unmarshal(body, &jobs); err != nil {
		return job{}, err
	}
	for _, candidate := range jobs {
		if candidate.Name == name {
			return candidate, nil
		}
	}
	return job{}, fmt.Errorf("%w: %s", errNoJob, name)
}

func (c config) get(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.Token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// waitForArtifact polls the build job and writes its artifact once it lands.
// The e2e job runs this after creating its cluster, so the wait overlaps the
// build rather than following it.
func waitForArtifact(client *http.Client, cfg config) error {
	jobsURL := fmt.Sprintf("%s/projects/%s/pipelines/%s/jobs?per_page=100",
		cfg.APIURL, cfg.ProjectID, cfg.PipelineID)
	deadline := time.Now().Add(cfg.Timeout)

	for {
		body, err := cfg.get(client, jobsURL)
		if err != nil {
			log.Printf("cannot list the parent pipeline jobs: %v", err)
		} else {
			build, findErr := findBuildJob(body, cfg.JobName)
			switch {
			case findErr != nil:
				log.Printf("%v", findErr)
			case build.Status == "success":
				return cfg.download(client, build.ID)
			case slices.Contains(deadStatuses, build.Status):
				return fmt.Errorf("%w: %s is %s", errBuild, cfg.JobName, build.Status)
			default:
				log.Printf("%s is %s, waiting", cfg.JobName, build.Status)
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w after %s", errTimedOut, cfg.Timeout)
		}
		time.Sleep(cfg.Interval)
	}
}

// download writes the single artifact file, so nothing has to unzip an archive.
func (c config) download(client *http.Client, jobID int) error {
	url := fmt.Sprintf("%s/projects/%s/jobs/%d/artifacts/%s",
		c.APIURL, c.ProjectID, jobID, c.ArtifactPath)
	body, err := c.get(client, url)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(c.Output), 0o750); err != nil {
		return err
	}
	log.Printf("writing %s (%d bytes) from job %d", c.Output, len(body), jobID)
	return os.WriteFile(c.Output, body, 0o600)
}

// configFromEnv reads the GitLab CI environment the e2e job runs in.
func configFromEnv() (config, error) {
	cfg := config{
		APIURL:       os.Getenv("CI_API_V4_URL"),
		ProjectID:    os.Getenv("CI_PROJECT_ID"),
		PipelineID:   os.Getenv("PARENT_PIPELINE_ID"),
		Token:        os.Getenv("GITLAB_TOKEN"),
		JobName:      envOr("E2E_BUILD_JOB", defaultJobName),
		ArtifactPath: envOr("E2E_ARTIFACT_PATH", defaultArtifactPath),
		Timeout:      defaultTimeout,
		Interval:     defaultInterval,
	}
	cfg.Output = envOr("E2E_ARTIFACT_OUTPUT", cfg.ArtifactPath)

	if cfg.Token == "" {
		return cfg, errors.New("GITLAB_TOKEN is empty, the e2e job cannot fetch the build artifact")
	}
	if cfg.APIURL == "" || cfg.ProjectID == "" || cfg.PipelineID == "" {
		return cfg, errors.New("CI_API_V4_URL, CI_PROJECT_ID and PARENT_PIPELINE_ID must all be set")
	}
	if raw := os.Getenv("E2E_ARTIFACT_TIMEOUT"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil {
			return cfg, fmt.Errorf("E2E_ARTIFACT_TIMEOUT %q is not a number of seconds", raw)
		}
		cfg.Timeout = time.Duration(seconds) * time.Second
	}
	return cfg, nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
