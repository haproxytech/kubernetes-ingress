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
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

func main() {
	cmd := exec.Command("bin/golangci-lint", "linters")
	result, err := cmd.CombinedOutput()
	if err != nil {
		log.Panic(err)
	}
	if _, err := os.Stat(".golangci.yml"); err == nil {
		data, err := os.ReadFile(".golangci.yml")
		if err != nil {
			log.Panic(err)
		}
		var config map[string]interface{}
		err = yaml.Unmarshal(data, &config)
		if err != nil {
			log.Panic(err)
		}
		delete(config, "linters")

		err = os.Rename(".golangci.yml", ".golangci.yml.tmp")
		if err != nil {
			log.Panic(err)
		}

		yamlData, err := yaml.Marshal(config)
		if err != nil {
			log.Panic(err)
		}
		err = os.WriteFile(".golangci.yml", yamlData, 0o600)
		if err != nil {
			log.Panic(err)
		}
	}
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, os.Interrupt)

	go func() {
		<-signalCh
		fmt.Println("ctrl-c received, terminating linters...") //nolint:forbidigo
		os.Remove(".golangci.yml")
		err = os.Rename(".golangci.yml.tmp", ".golangci.yml")
		if err != nil {
			log.Panic(err)
		}
		os.Exit(1)
	}()

	// fmt.Println(string(result))
	lines := strings.Split(string(result), "\n")
	exitCode := 0
	for _, line := range lines {
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Disabled by your configuration linters") {
			break
		}
		if strings.HasPrefix(line, "Enabled by your configuration linters:") {
			continue
		}
		parts := strings.Split(line, ":")
		fmt.Print(parts[0]) //nolint:forbidigo
		timeStart := time.Now()
		args := []string{"--timeout", "20m", "--max-issues-per-linter", "0", "--max-same-issues", "0", "run", "-E"}

		cmd := exec.Command("bin/golangci-lint", append(args, parts[0])...) //nolint:gosec
		result, err := cmd.CombinedOutput()
		duration := time.Since(timeStart)
		fmt.Printf(" %.1fs %s\n", duration.Seconds(), string(result)) //nolint:forbidigo
		if err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				if exitError.Exited() {
					if exitError.ExitCode() != 0 {
						exitCode = 1
					}
				}
			} else {
				fmt.Println(err) //nolint:forbidigo
			}
		}
	}

	os.Remove(".golangci.yml")
	err = os.Rename(".golangci.yml.tmp", ".golangci.yml")
	if err != nil {
		log.Panic(err)
	}
	os.Exit(exitCode)
}
