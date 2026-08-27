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
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
)

const (
	defaultAPI       = "https://api-ci-go-runners-cycle.gtm.int.haproxy.com"
	defaultManifest  = ".gitlab/e2e-jobs.yml"
	defaultTestsRoot = "deploy/tests/e2e"
	defaultOutput    = "generated-e2e.yml"
	fetchAttempts    = 3
	fetchBackoff     = 2 * time.Second
	fetchTimeout     = 10 * time.Second
)

func main() {
	log.SetFlags(0)
	manifestPath := envOr("E2E_JOBS_MANIFEST", defaultManifest)
	outputPath := envOr("E2E_JOBS_OUTPUT", defaultOutput)

	manifest, err := loadManifest(manifestPath)
	if err != nil {
		log.Fatalf("cannot read %s: %v", manifestPath, err)
	}

	tests, err := discoverTests(envOr("E2E_TESTS_ROOT", defaultTestsRoot))
	if err != nil {
		log.Fatalf("cannot discover the e2e tests: %v", err)
	}

	jobs := manifest.selectedJobs(triggerFromEnv())
	legCount := 0
	for _, job := range jobs {
		for _, split := range job.Splits {
			legCount += len(shardTests(tests, split.Tag, split.Shards))
		}
	}
	log.Printf("%d test(s) discovered, %d job set(s) selected, %d leg(s) to place", len(tests), len(jobs), legCount)

	for _, job := range jobs {
		for _, split := range job.Splits {
			if split.RunMode() != modeParallel {
				continue
			}
			for _, shard := range shardTests(tests, split.Tag, split.Shards) {
				if clashes := duplicateNames(shard); len(clashes) > 0 {
					log.Fatalf("job %s split %s: %v appear in more than one package of a parallel shard",
						job.Name, split.Tag, clashes)
				}
			}
		}
	}

	content, err := render(buildPipeline(jobs, tests, runnerTags(legCount)))
	if err != nil {
		log.Fatalf("cannot render the child pipeline: %v", err)
	}
	if err = os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		log.Fatalf("cannot write %s: %v", outputPath, err)
	}
	log.Printf("wrote %s:\n%s", outputPath, content)
}

// runnerTags returns the pin tags for legCount legs. Anything that goes wrong
// leaves the legs unpinned rather than failing the pipeline.
func runnerTags(legCount int) []string {
	if legCount == 0 {
		return nil
	}
	if os.Getenv("E2E_PIN_RUNNERS") == "false" {
		log.Print("E2E_PIN_RUNNERS=false, leaving every leg unpinned")
		return nil
	}

	pattern := defaultTagPattern
	if raw := os.Getenv("E2E_RUNNER_TAG_PATTERN"); raw != "" {
		compiled, err := regexp.Compile(raw)
		if err != nil {
			log.Printf("E2E_RUNNER_TAG_PATTERN %q does not compile, leaving legs unpinned: %v", raw, err)
			return nil
		}
		pattern = compiled
	}

	client := &http.Client{Timeout: fetchTimeout}
	runners, err := fetchRunnersWithRetry(client, envOr("E2E_RUNNERS_API", defaultAPI), legCount, fetchAttempts, fetchBackoff)
	if err != nil {
		log.Printf("falling back to unpinned jobs: %v", err)
		return nil
	}

	tags := pinTags(runners, pattern)
	if len(tags) == 0 {
		log.Print("no runner carries a pin tag, leaving every leg unpinned")
		return nil
	}
	log.Printf("pinning %d leg(s) across runners %v", legCount, tags)
	return tags
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
