package common

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHAProxy refuses any configuration holding the word REFUSED and counts its runs.
func fakeHAProxy(t *testing.T) (binary, counter string) {
	t.Helper()
	dir := t.TempDir()
	binary = filepath.Join(dir, "haproxy")
	counter = filepath.Join(dir, "runs")
	script := "#!/bin/sh\necho run >> " + counter + "\nif grep -q REFUSED; then exit 1; fi\nexit 0\n"
	require.NoError(t, os.WriteFile(binary, []byte(script), 0o700)) //nolint:gosec
	t.Cleanup(func() { SetHAProxyBinary("") })
	return binary, counter
}

func runCount(t *testing.T, counter string) int {
	t.Helper()
	data, err := os.ReadFile(counter)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return len(data) / len("run\n")
}

func TestValidRegexWithoutBinary(t *testing.T) {
	SetHAProxyBinary("")
	assert.True(t, ValidRegex(RegexACL, "("))
	assert.False(t, ValidRegex(RegexACL, "two words"), "a value that is not one word is never valid")
}

func TestValidRegexAsksTheBinary(t *testing.T) {
	binary, counter := fakeHAProxy(t)
	SetHAProxyBinary(binary)
	probeRuns := runCount(t, counter)
	require.Equal(t, 1, probeRuns)

	assert.True(t, ValidRegex(RegexACL, "^/ok$"))
	assert.False(t, ValidRegex(RegexACL, "REFUSED"))
	assert.True(t, ValidRegex(RegexReplacePath, "^/ok$"))
	assert.False(t, ValidRegex(RegexReplacePath, "REFUSED"))
	assert.Equal(t, probeRuns+4, runCount(t, counter))

	assert.False(t, ValidRegex(RegexACL, "REFUSED"))
	assert.True(t, ValidRegex(RegexACL, "^/ok$"))
	assert.Equal(t, probeRuns+4, runCount(t, counter), "verdicts are cached")
}

func TestValidRegexNeverRunsUnsafeWords(t *testing.T) {
	binary, counter := fakeHAProxy(t)
	SetHAProxyBinary(binary)
	runs := runCount(t, counter)
	for _, word := range []string{"a b", "a\nb", "a#b", `a"b`, `a\`, "-u", "{", "}"} {
		assert.False(t, ValidRegex(RegexACL, word), word)
	}
	assert.False(t, ValidRegex(RegexReplacePath, "a\n  http-request deny"))
	assert.Equal(t, runs, runCount(t, counter))
}

func TestValidRegexDisabledWhenProbeFails(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "haproxy")
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nexit 1\n"), 0o700)) //nolint:gosec
	t.Cleanup(func() { SetHAProxyBinary("") })
	SetHAProxyBinary(binary)
	assert.True(t, ValidRegex(RegexACL, "("))
}

func TestValidRegexMissingBinaryAcceptsValues(t *testing.T) {
	t.Cleanup(func() { SetHAProxyBinary("") })
	SetHAProxyBinary(filepath.Join(t.TempDir(), "missing"))
	assert.True(t, ValidRegex(RegexACL, "("))
}

func TestValidRegexWithHAProxy(t *testing.T) {
	binary, err := exec.LookPath("haproxy")
	if err != nil {
		t.Skip("haproxy is not installed")
	}
	t.Cleanup(func() { SetHAProxyBinary("") })
	SetHAProxyBinary(binary)
	tests := []struct {
		use   RegexUse
		word  string
		valid bool
	}{
		{RegexACL, `^https://[a-z]+\.example\.com$`, true},
		{RegexACL, "^/api($|/)", true},
		{RegexACL, "(?=a)b", true},
		{RegexACL, "(", false},
		{RegexACL, "[z-a]", false},
		{RegexACL, "^/a(($|/)", false},
		{RegexACL, `(a)\1`, false},
		{RegexReplacePath, "^/foo/(.*)$", true},
		{RegexReplacePath, `(a)\1`, true},
		{RegexReplacePath, "[", false},
		{RegexReplacePath, "a{2,1}", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.valid, ValidRegex(tt.use, tt.word), tt.word)
	}
}
