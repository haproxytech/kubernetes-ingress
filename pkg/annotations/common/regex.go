package common

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// RegexUse names the directive a regex ends up in; each compiles differently.
type RegexUse int

const (
	// RegexACL is an ACL pattern, compiled without capture groups.
	RegexACL RegexUse = iota
	// RegexReplacePath is the match part of a path rewrite.
	RegexReplacePath
)

const (
	regexCheckTimeout = 5 * time.Second
	regexCacheSize    = 4096
	// regexProbe must compile on every HAProxy build.
	regexProbe = "^/probe$"
)

var regexChecker struct {
	sync.Mutex
	binary string
	cache  map[string]bool
}

// SetRegexBinary makes ValidRegex ask the given HAProxy binary.
// An empty binary turns the check off.
func SetRegexBinary(binary string) {
	regexChecker.Lock()
	regexChecker.binary = binary
	regexChecker.cache = map[string]bool{}
	regexChecker.Unlock()
	if binary == "" {
		return
	}
	// A binary that refuses a known good regex cannot judge other ones.
	if !ValidRegex(RegexACL, regexProbe) {
		logger.Warningf("regex validation disabled: '%s' does not accept the test configuration", binary)
		regexChecker.Lock()
		regexChecker.binary = ""
		regexChecker.Unlock()
	}
}

// ValidRegex reports whether HAProxy compiles word for the given use.
// Only HAProxy knows its regex dialect, so the binary gives the verdict.
func ValidRegex(use RegexUse, word string) bool {
	// The word is written into a configuration line and must stay one word.
	if _, ok := UnquotedWord(word); !ok {
		return false
	}
	var rule string
	switch use {
	case RegexACL:
		// A flag or a brace would change the condition instead of being a pattern.
		if word == "{" || word == "}" || strings.HasPrefix(word, "-") {
			return false
		}
		rule = "http-request deny if { path -m reg " + word + " }"
	case RegexReplacePath:
		rule = "http-request replace-path " + word + " /"
	default:
		return false
	}

	regexChecker.Lock()
	defer regexChecker.Unlock()
	if regexChecker.binary == "" {
		return true
	}
	if valid, found := regexChecker.cache[rule]; found {
		return valid
	}
	valid := haproxyAcceptsRule(regexChecker.binary, rule)
	if len(regexChecker.cache) >= regexCacheSize {
		regexChecker.cache = map[string]bool{}
	}
	regexChecker.cache[rule] = valid
	return valid
}

// haproxyAcceptsRule runs a configuration check on a frontend holding only rule.
func haproxyAcceptsRule(binary, rule string) bool {
	config := "defaults\n  mode http\n  timeout client 5s\n  timeout server 5s\n  timeout connect 5s\n" +
		"frontend regex-check\n  bind 127.0.0.1:1\n  " + rule + "\n"
	ctx, cancel := context.WithTimeout(context.Background(), regexCheckTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-c", "-f", "/dev/stdin")
	cmd.Stdin = strings.NewReader(config)
	err := cmd.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false
	}
	// The binary did not run; the full configuration check still guards the reload.
	logger.Errorf("regex validation: unable to run '%s': %s", binary, err)
	return true
}
