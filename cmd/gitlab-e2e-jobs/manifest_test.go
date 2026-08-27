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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureManifest = `
jobs:
  - name: e2e_k8s
    stage: e2e_k8s
    when: mr_or_push
    splits:
      - tag: e2e_parallel
        shards: 1
      - tag: e2e_sequential
        shards: 2
    variables:
      K8S_VERSION: v1.34.0
  - name: e2e_dormant
    stage: e2e_k8s_sch_1
    when: never
    splits:
      - tag: e2e_parallel
        shards: 1
    variables:
      K8S_VERSION: v1.31.9
  - name: e2e_crd_v1
    stage: e2e_crd_versions
    when: schedule_weekly_wednesday
    allow_failure: true
    splits:
      - tag: e2e_parallel
        shards: 1
    variables:
      CRD_VERSION: v1
`

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "e2e-jobs.yml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func names(jobs []JobSet) []string {
	found := make([]string, 0, len(jobs))
	for _, job := range jobs {
		found = append(found, job.Name)
	}
	return found
}

// TestLoadManifestReadsTheRepositoryManifest keeps the checked-in manifest
// honest: a typo in .gitlab/e2e-jobs.yml fails the unit tests.
func TestLoadManifestReadsTheRepositoryManifest(t *testing.T) {
	manifest, err := loadManifest(filepath.Join("..", "..", ".gitlab", "e2e-jobs.yml"))
	require.NoError(t, err)
	require.NotEmpty(t, manifest.Jobs)
	assert.Equal(t, "e2e_k8s", manifest.Jobs[0].Name)
	assert.Equal(t, []Split{
		{Tag: "e2e_parallel", Mode: "parallel", Shards: 1},
		{Tag: "e2e_https", Mode: "parallel", Shards: 1},
		{Tag: "e2e_sequential", Mode: "sequential", Shards: 4},
	}, manifest.Jobs[0].Splits)
	assert.Equal(t, "v1.34.0", manifest.Jobs[0].Variables["K8S_VERSION"])
}

func TestLoadManifestRejectsAnUnknownWhenValue(t *testing.T) {
	path := writeManifest(t, "jobs:\n  - name: bad\n    stage: s\n    when: on_tuesdays\n"+
		"    splits:\n      - tag: e2e_parallel\n        shards: 1\n")
	_, err := loadManifest(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "on_tuesdays")
}

func TestLoadManifestRejectsAJobWithoutSplits(t *testing.T) {
	path := writeManifest(t, "jobs:\n  - name: bad\n    stage: s\n    when: never\n")
	_, err := loadManifest(path)
	require.Error(t, err)
}

func TestLoadManifestRejectsASplitWithoutATag(t *testing.T) {
	path := writeManifest(t, "jobs:\n  - name: bad\n    stage: s\n    when: never\n    splits:\n      - shards: 2\n")
	_, err := loadManifest(path)
	require.Error(t, err)
}

func TestSplitLabelKeepsSingleShardNamesUnnumbered(t *testing.T) {
	split := Split{Tag: "e2e_parallel", Shards: 1}
	assert.Equal(t, "parallel", split.Label(0, 1))
	assert.Equal(t, "sequential-3", Split{Tag: "e2e_sequential"}.Label(2, 4))
}

func TestSelectedJobsForAMergeRequest(t *testing.T) {
	manifest, err := loadManifest(writeManifest(t, fixtureManifest))
	require.NoError(t, err)
	selected := manifest.selectedJobs(Trigger{Source: "merge_request_event"})
	assert.Equal(t, []string{"e2e_k8s"}, names(selected))
}

func TestSelectedJobsForAPushOutsideTheProtectedNamespace(t *testing.T) {
	manifest, err := loadManifest(writeManifest(t, fixtureManifest))
	require.NoError(t, err)
	selected := manifest.selectedJobs(Trigger{Source: "push", Namespace: "someone"})
	assert.Equal(t, []string{"e2e_k8s"}, names(selected))
}

func TestSelectedJobsSkipsAPushInsideTheProtectedNamespace(t *testing.T) {
	manifest, err := loadManifest(writeManifest(t, fixtureManifest))
	require.NoError(t, err)
	selected := manifest.selectedJobs(Trigger{Source: "push", Namespace: "haproxy-controller"})
	assert.Empty(t, selected)
}

func TestSelectedJobsForTheWeeklyWednesdaySchedule(t *testing.T) {
	manifest, err := loadManifest(writeManifest(t, fixtureManifest))
	require.NoError(t, err)
	selected := manifest.selectedJobs(Trigger{Source: "schedule", ScheduleType: "weekly", ScheduleDay: "wednesday"})
	assert.Equal(t, []string{"e2e_crd_v1"}, names(selected))
	assert.True(t, selected[0].AllowFailure)
}

func TestSelectedJobsIsEmptyOnAScheduleThatMatchesNothing(t *testing.T) {
	manifest, err := loadManifest(writeManifest(t, fixtureManifest))
	require.NoError(t, err)
	selected := manifest.selectedJobs(Trigger{Source: "schedule", ScheduleType: "weekly", ScheduleDay: "thursday"})
	assert.Empty(t, selected)
}

func TestTriggerFromEnvReadsTheGitLabVariables(t *testing.T) {
	t.Setenv("CI_PIPELINE_SOURCE", "schedule")
	t.Setenv("CI_PROJECT_NAMESPACE", "haproxy-controller")
	t.Setenv("SCHEDULE_TYPE", "weekly")
	t.Setenv("SCHEDULE_DAY", "wednesday")

	assert.Equal(t, Trigger{
		Source:       "schedule",
		Namespace:    "haproxy-controller",
		ScheduleType: "weekly",
		ScheduleDay:  "wednesday",
	}, triggerFromEnv())
}

func TestRunModeDefaultsToParallel(t *testing.T) {
	assert.Equal(t, "parallel", Split{Tag: "e2e_parallel"}.RunMode())
	assert.Equal(t, "sequential", Split{Tag: "e2e_sequential", Mode: "sequential"}.RunMode())
}

func TestLoadManifestRejectsAnUnknownSplitMode(t *testing.T) {
	path := writeManifest(t, "jobs:\n  - name: bad\n    stage: s\n    when: never\n"+
		"    splits:\n      - tag: e2e_parallel\n        shards: 1\n        mode: whenever\n")
	_, err := loadManifest(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "whenever")
}
