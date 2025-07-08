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
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

//go:embed ascii.txt
var hello string

//nolint:forbidigo
func main() {
	fmt.Println(hello)
	// Check if we are in a merge request context
	mrIID := os.Getenv("CI_MERGE_REQUEST_IID")
	if mrIID == "" {
		fmt.Println("Not a merge request. Exiting.")
		os.Exit(0)
	}

	// Get necessary environment variables
	gitlabAPIURL := os.Getenv("CI_API_V4_URL")
	projectID := os.Getenv("CI_PROJECT_ID")
	sourceProjectID := os.Getenv("CI_MERGE_REQUEST_SOURCE_PROJECT_ID")
	gitlabToken := os.Getenv("GITLAB_TOKEN")

	if gitlabAPIURL == "" || projectID == "" || sourceProjectID == "" {
		fmt.Println("Missing required GitLab CI/CD environment variables.")
		os.Exit(1)
	}

	if gitlabToken == "" {
		fmt.Print("GitLab token not found in environment variable.\n")
		os.Exit(1)
	}

	// 1. Get all old pipelines for this Merge Request
	pipelinesToCancel, err := getOldMergeRequestPipelines(gitlabAPIURL, projectID, mrIID, gitlabToken)
	if err != nil {
		fmt.Printf("Error getting merge request pipelines: %v\n", err)
		os.Exit(1)
	}

	if len(pipelinesToCancel) == 0 {
		fmt.Println("No old, running pipelines found for this merge request.")
		os.Exit(0)
	}

	fmt.Printf("Found %d old pipelines to cancel.\n", len(pipelinesToCancel))

	// 2. Cancel all found pipelines
	for _, p := range pipelinesToCancel {
		fmt.Printf("Canceling pipeline ID %d on project ID %d\n", p.ID, p.ProjectID)
		err = cancelPipeline(gitlabAPIURL, strconv.Itoa(p.ProjectID), p.ID, gitlabToken)
		if err != nil {
			// Log error but continue trying to cancel others
			fmt.Printf("Failed to cancel pipeline %d: %v\n", p.ID, err)
		} else {
			fmt.Printf("Successfully requested cancellation for pipeline %d\n", p.ID)
		}
	}
}

type pipelineInfo struct {
	ID        int    `json:"id"`
	ProjectID int    `json:"project_id"`
	Status    string `json:"status"`
}

func getOldMergeRequestPipelines(apiURL, projectID, mrIID, token string) ([]pipelineInfo, error) {
	// Get the current pipeline ID to avoid canceling ourselves
	currentPipelineIDStr := os.Getenv("CI_PIPELINE_ID")
	var currentPipelineID int
	if currentPipelineIDStr != "" {
		// a non-integer value will result in 0, which is fine since pipeline IDs are positive
		currentPipelineID, _ = strconv.Atoi(currentPipelineIDStr)
	}

	url := fmt.Sprintf("%s/projects/%s/merge_requests/%s/pipelines", apiURL, projectID, mrIID)
	req, err := http.NewRequest("GET", url, nil) //nolint:noctx,usestdlibvars
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token) //nolint:canonicalheader

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list merge request pipelines: status %d, body: %s", resp.StatusCode, string(body))
	}

	var pipelines []pipelineInfo
	if err := json.NewDecoder(resp.Body).Decode(&pipelines); err != nil {
		return nil, err
	}

	var pipelinesToCancel []pipelineInfo
	for _, p := range pipelines {
		// Cancel pipelines that are running or pending, and are not the current pipeline
		if (p.Status == "running" || p.Status == "pending") && p.ID != currentPipelineID {
			pipelinesToCancel = append(pipelinesToCancel, p)
		}
	}

	return pipelinesToCancel, nil
}

func cancelPipeline(apiURL, projectID string, pipelineID int, token string) error {
	url := fmt.Sprintf("%s/projects/%s/pipelines/%d/cancel", apiURL, projectID, pipelineID)
	req, err := http.NewRequest("POST", url, nil) //nolint:noctx,usestdlibvars
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", token) //nolint:canonicalheader

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// It's possible the pipeline is already finished.
		if strings.Contains(string(body), "Cannot cancel a pipeline that is not pending or running") {
			fmt.Println("Pipeline already finished, nothing to do.") //nolint:forbidigo
			return nil
		}
		return fmt.Errorf("failed to cancel pipeline: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
