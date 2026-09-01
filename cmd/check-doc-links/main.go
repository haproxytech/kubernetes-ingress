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

// check-doc-links rejects markdown links to repository files in documentation
// yaml fields that the documentation site renders as page-relative links.
// Such links break the site build; only web URLs and page anchors are allowed.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type entry struct {
	Title    string   `yaml:"title"`
	Argument string   `yaml:"argument"`
	Tip      []string `yaml:"tip"`
	Values   []string `yaml:"values"`
}

type doc struct {
	ImageArguments []entry `yaml:"image_arguments"`
	Annotations    []entry `yaml:"annotations"`
}

var mdLink = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: check-doc-links <doc.yaml> [<doc.yaml> ...]")
		os.Exit(2)
	}
	failed := false
	for _, file := range os.Args[1:] {
		failed = checkFile(file) || failed
	}
	if failed {
		os.Exit(1)
	}
}

func checkFile(file string) (failed bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Println(err)
		return true
	}
	var d doc
	if err := yaml.Unmarshal(data, &d); err != nil {
		fmt.Printf("%s: %v\n", file, err)
		return true
	}
	for _, e := range append(d.ImageArguments, d.Annotations...) {
		name := e.Title
		if name == "" {
			name = e.Argument
		}
		failed = checkLines(file, name, "tip", e.Tip) || failed
		failed = checkLines(file, name, "values", e.Values) || failed
	}
	return failed
}

func checkLines(file, name, field string, lines []string) (failed bool) {
	for _, line := range lines {
		for _, m := range mdLink.FindAllStringSubmatch(line, -1) {
			target := m[1]
			if strings.HasPrefix(target, "http://") ||
				strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "#") {
				continue
			}
			failed = true
			fmt.Printf("%s: %s: %s: link target %q is a repository file; use a web URL, a #anchor, or plain text\n",
				file, name, field, target)
		}
	}
	return failed
}
