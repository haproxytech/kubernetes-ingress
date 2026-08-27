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
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// JobSet is one e2e job definition, fanned out over its test parts.
type JobSet struct {
	Variables    map[string]string `yaml:"variables"`
	Name         string            `yaml:"name"`
	Stage        string            `yaml:"stage"`
	When         string            `yaml:"when"`
	Parts        []string          `yaml:"parts"`
	AllowFailure bool              `yaml:"allow_failure"`
}

// Manifest is the parsed .gitlab/e2e-jobs.yml.
type Manifest struct {
	Jobs []JobSet `yaml:"jobs"`
}

// Trigger describes the pipeline the generator runs in.
type Trigger struct {
	Source       string
	Namespace    string
	ScheduleType string
	ScheduleDay  string
}

const (
	whenNever          = "never"
	whenMROrPush       = "mr_or_push"
	whenSchedulePrefix = "schedule_weekly_"
	protectedNamespace = "haproxy-controller"
	sourceMR           = "merge_request_event"
	sourcePush         = "push"
	sourceSchedule     = "schedule"
	scheduleWeekly     = "weekly"
)

var errUnknownWhen = errors.New("unknown when value")

// loadManifest reads and validates the job manifest at path.
func loadManifest(path string) (Manifest, error) {
	var manifest Manifest
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	if err = yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	for _, job := range manifest.Jobs {
		if err = job.validate(); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

func (j JobSet) validate() error {
	if j.Name == "" || j.Stage == "" || len(j.Parts) == 0 {
		return fmt.Errorf("job %q needs a name, a stage and at least one part", j.Name)
	}
	if j.When == whenNever || j.When == whenMROrPush || strings.HasPrefix(j.When, whenSchedulePrefix) {
		return nil
	}
	return fmt.Errorf("%w %q for job %q", errUnknownWhen, j.When, j.Name)
}

// selected reports whether this job set runs in the given pipeline.
func (j JobSet) selected(t Trigger) bool {
	if j.When == whenMROrPush {
		return t.Source == sourceMR || (t.Source == sourcePush && t.Namespace != protectedNamespace)
	}
	if strings.HasPrefix(j.When, whenSchedulePrefix) {
		day := strings.TrimPrefix(j.When, whenSchedulePrefix)
		return t.Source == sourceSchedule && t.ScheduleType == scheduleWeekly && t.ScheduleDay == day
	}
	return false
}

// selectedJobs returns the job sets that run in the given pipeline.
func (m Manifest) selectedJobs(t Trigger) []JobSet {
	jobs := make([]JobSet, 0, len(m.Jobs))
	for _, job := range m.Jobs {
		if job.selected(t) {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// triggerFromEnv reads the pipeline context from the GitLab CI environment.
func triggerFromEnv() Trigger {
	return Trigger{
		Source:       os.Getenv("CI_PIPELINE_SOURCE"),
		Namespace:    os.Getenv("CI_PROJECT_NAMESPACE"),
		ScheduleType: os.Getenv("SCHEDULE_TYPE"),
		ScheduleDay:  os.Getenv("SCHEDULE_DAY"),
	}
}
