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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// TestCase is one top-level test function, paired with the package holding it.
// The pair matters: a test name is not unique across packages.
type TestCase struct {
	Tag     string
	Package string
	Name    string
}

var (
	buildTagRe = regexp.MustCompile(`(?m)^//go:build[ \t]+(.+)$`)
	testFuncRe = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(t \*testing\.T\)`)
)

// discoverTests collects every tagged test function below root, sorted by
// package then name so the split is reproducible.
func discoverTests(root string) ([]TestCase, error) {
	tests := []TestCase{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tag := buildTagRe.FindSubmatch(source)
		if tag == nil {
			return nil
		}
		pkg := "./" + filepath.ToSlash(filepath.Dir(path))
		for _, match := range testFuncRe.FindAllSubmatch(source, -1) {
			tests = append(tests, TestCase{
				Tag:     strings.TrimSpace(string(tag[1])),
				Package: pkg,
				Name:    string(match[1]),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(tests, func(a, b TestCase) int {
		if pkg := strings.Compare(a.Package, b.Package); pkg != 0 {
			return pkg
		}
		return strings.Compare(a.Name, b.Name)
	})
	return tests, nil
}

// shardTests deals the tests carrying tag round robin across shards. Empty
// shards are dropped, so asking for more shards than tests is harmless.
func shardTests(tests []TestCase, tag string, shards int) [][]TestCase {
	if shards < 1 {
		return nil
	}
	dealt := make([][]TestCase, shards)
	index := 0
	for _, test := range tests {
		if test.Tag != tag {
			continue
		}
		dealt[index%shards] = append(dealt[index%shards], test)
		index++
	}
	filled := make([][]TestCase, 0, shards)
	for _, shard := range dealt {
		if len(shard) > 0 {
			filled = append(filled, shard)
		}
	}
	return filled
}
