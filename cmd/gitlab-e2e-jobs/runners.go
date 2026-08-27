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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"
)

// Runner is the subset of the runner cycle API payload the generator needs.
type Runner struct {
	Description string   `json:"description"`
	Status      string   `json:"status"`
	TagList     []string `json:"tag_list"`
	ID          int      `json:"id"`
	Online      bool     `json:"online"`
	Paused      bool     `json:"paused"`
}

// defaultTagPattern matches the tag that identifies a single runner, e.g. go-r1.
var defaultTagPattern = regexp.MustCompile(`^go-r[0-9]+$`)

var errRunnerAPI = errors.New("runner cycle API request failed")

// decodeRunners reads either a JSON array of runners or a single runner object.
func decodeRunners(body []byte) ([]Runner, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var runners []Runner
		err := json.Unmarshal(trimmed, &runners)
		return runners, err
	}
	var runner Runner
	if err := json.Unmarshal(trimmed, &runner); err != nil {
		return nil, err
	}
	return []Runner{runner}, nil
}

// fetchRunners asks the cycle API for the next count runners.
func fetchRunners(client *http.Client, baseURL string, count int) ([]Runner, error) {
	url := baseURL + "/next?count=" + strconv.Itoa(count)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s returned %d", errRunnerAPI, url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return decodeRunners(body)
}

// fetchRunnersWithRetry retries fetchRunners with a linear backoff.
func fetchRunnersWithRetry(client *http.Client, baseURL string, count, attempts int, backoff time.Duration) ([]Runner, error) {
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		var runners []Runner
		runners, err = fetchRunners(client, baseURL, count)
		if err == nil {
			return runners, nil
		}
		log.Printf("runner cycle API attempt %d/%d failed: %v", attempt, attempts, err)
		if attempt < attempts {
			time.Sleep(backoff * time.Duration(attempt))
		}
	}
	return nil, err
}

// pinTags returns one pin tag per usable runner, keeping the order the API
// returned. Unavailable runners and runners without a matching tag are skipped.
func pinTags(runners []Runner, pattern *regexp.Regexp) []string {
	tags := make([]string, 0, len(runners))
	for _, runner := range runners {
		if !runner.Online || runner.Paused {
			log.Printf("runner %d (%s) is not available, skipping", runner.ID, runner.Description)
			continue
		}
		matches := make([]string, 0, len(runner.TagList))
		for _, tag := range runner.TagList {
			if pattern.MatchString(tag) {
				matches = append(matches, tag)
			}
		}
		if len(matches) == 0 {
			log.Printf("runner %d (%s) has no tag matching %s, skipping", runner.ID, runner.Description, pattern)
			continue
		}
		slices.Sort(matches)
		if !slices.Contains(tags, matches[0]) {
			tags = append(tags, matches[0])
		}
	}
	return tags
}

// assignTags hands one tag to each leg, wrapping when legs outnumber tags.
// An empty tag leaves that leg unpinned.
func assignTags(legs int, tags []string) []string {
	assigned := make([]string, legs)
	if len(tags) == 0 {
		return assigned
	}
	for i := range assigned {
		assigned[i] = tags[i%len(tags)]
	}
	return assigned
}
