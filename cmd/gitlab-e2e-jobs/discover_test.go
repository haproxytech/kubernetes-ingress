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

func writeTestFile(t *testing.T, root, dir, name, tag string, funcs ...string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o750))
	body := "//go:build " + tag + "\n\npackage e2e\n\nimport \"testing\"\n"
	for _, fn := range funcs {
		body += "\nfunc " + fn + "(t *testing.T) {}\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, dir, name), []byte(body), 0o600))
}

func TestDiscoverTestsPairsEachTestWithItsTagAndPackage(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "cors", "suite_test.go", "e2e_sequential", "TestCorsSuite")
	writeTestFile(t, root, "admin-port", "suite_test.go", "e2e_parallel", "TestAdminPortSuite", "TestPprof")

	found, err := discoverTests(root)
	require.NoError(t, err)
	require.Len(t, found, 3)
	assert.Equal(t, TestCase{Tag: "e2e_parallel", Package: "./" + root + "/admin-port", Name: "TestAdminPortSuite"}, found[0])
	assert.Equal(t, "TestPprof", found[1].Name)
	assert.Equal(t, "e2e_sequential", found[2].Tag)
}

func TestDiscoverTestsIgnoresFilesWithoutABuildTag(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plain"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "plain", "unit_test.go"),
		[]byte("package e2e\n\nimport \"testing\"\n\nfunc TestUnit(t *testing.T) {}\n"), 0o600))

	found, err := discoverTests(root)
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestDiscoverTestsIgnoresHelpersAndBenchmarks(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "cors", "suite_test.go", "e2e_sequential", "TestCorsSuite")
	require.NoError(t, os.WriteFile(filepath.Join(root, "cors", "helper_test.go"),
		[]byte("//go:build e2e_sequential\n\npackage e2e\n\nimport \"testing\"\n\n"+
			"func BenchmarkThing(b *testing.B) {}\n\nfunc testHelper(t *testing.T) {}\n"), 0o600))

	found, err := discoverTests(root)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "TestCorsSuite", found[0].Name)
}

// TestDiscoverTestsMatchesTheRepository is the guard against the discovery
// drifting from the real tree; the counts come from the e2e suites in git.
func TestDiscoverTestsMatchesTheRepository(t *testing.T) {
	found, err := discoverTests(filepath.Join("..", "..", "deploy", "tests", "e2e"))
	require.NoError(t, err)

	perTag := map[string]int{}
	for _, tc := range found {
		perTag[tc.Tag]++
	}
	assert.Equal(t, 14, perTag["e2e_parallel"])
	assert.Equal(t, 4, perTag["e2e_https"])
	assert.Equal(t, 18, perTag["e2e_sequential"])
}

// TestDiscoverTestsKeepsDuplicateNamesApart covers TestHTTPSSuite, which exists
// in two packages under two different tags.
func TestDiscoverTestsKeepsDuplicateNamesApart(t *testing.T) {
	found, err := discoverTests(filepath.Join("..", "..", "deploy", "tests", "e2e"))
	require.NoError(t, err)

	packages := []string{}
	for _, tc := range found {
		if tc.Name == "TestHTTPSSuite" {
			packages = append(packages, tc.Package)
		}
	}
	assert.Len(t, packages, 2)
	assert.NotEqual(t, packages[0], packages[1])
}

func TestShardTestsDealsRoundRobinAndKeepsEveryTest(t *testing.T) {
	tests := []TestCase{}
	for _, name := range []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8", "T9"} {
		tests = append(tests, TestCase{Tag: "e2e_sequential", Package: "./pkg", Name: name})
	}
	tests = append(tests, TestCase{Tag: "e2e_parallel", Package: "./other", Name: "TOther"})

	shards := shardTests(tests, "e2e_sequential", 4)
	require.Len(t, shards, 4)
	assert.Equal(t, []int{3, 2, 2, 2}, []int{len(shards[0]), len(shards[1]), len(shards[2]), len(shards[3])})

	seen := map[string]bool{}
	for _, shard := range shards {
		for _, tc := range shard {
			assert.Equal(t, "e2e_sequential", tc.Tag)
			assert.False(t, seen[tc.Name], "%s appears twice", tc.Name)
			seen[tc.Name] = true
		}
	}
	assert.Len(t, seen, 9)
}

func TestShardTestsDropsEmptyShards(t *testing.T) {
	tests := []TestCase{{Tag: "e2e_https", Package: "./pkg", Name: "TOnly"}}
	shards := shardTests(tests, "e2e_https", 4)
	require.Len(t, shards, 1)
	assert.Equal(t, "TOnly", shards[0][0].Name)
}

func TestShardTestsReturnsNothingForAnUnknownTag(t *testing.T) {
	assert.Empty(t, shardTests([]TestCase{{Tag: "e2e_https", Name: "T"}}, "e2e_absent", 2))
}
