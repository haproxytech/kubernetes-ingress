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

// RuleSection names the configuration section a rule is checked in.
type RuleSection int

const (
	FrontendRule RuleSection = iota
	BackendRule
)

// CheckBackend is a backend that frontend rules under check can refer to.
const CheckBackend = "rule-check"

const (
	checkTimeout   = 5 * time.Second
	checkCacheSize = 4096
	// checkProbe must be valid on every HAProxy build.
	checkProbe = "http-request deny if { path -m reg ^/probe$ }"
)

var checker struct {
	sync.Mutex
	binary string
	cache  map[string]bool
}

// SetHAProxyBinary makes ValidRule and ValidRegex ask the given HAProxy binary.
// An empty binary turns the checks off.
func SetHAProxyBinary(binary string) {
	checker.Lock()
	checker.binary = binary
	checker.cache = map[string]bool{}
	checker.Unlock()
	if binary == "" {
		return
	}
	// A binary that refuses a known good rule cannot judge other ones.
	if !ValidRule(FrontendRule, checkProbe) {
		logger.Warningf("annotation validation by HAProxy disabled: '%s' does not accept the test configuration", binary)
		checker.Lock()
		checker.binary = ""
		checker.Unlock()
	}
}

// ValidRegex reports whether HAProxy compiles word for the given use.
// Only HAProxy knows its regex dialect, so the binary gives the verdict.
func ValidRegex(use RegexUse, word string) bool {
	// The word is written into a configuration line and must stay one word.
	if _, ok := UnquotedWord(word); !ok {
		return false
	}
	switch use {
	case RegexACL:
		// A flag or a brace would change the condition instead of being a pattern.
		if word == "{" || word == "}" || strings.HasPrefix(word, "-") {
			return false
		}
		return ValidRule(FrontendRule, "http-request deny if { path -m reg "+word+" }")
	case RegexReplacePath:
		return ValidRule(FrontendRule, "http-request replace-path "+word+" /")
	}
	return false
}

// ValidRule reports whether HAProxy accepts rule as one line of the given section.
// The rule is judged alone, so it cannot name backends, tables or userlists of the real configuration.
func ValidRule(section RuleSection, rule string) bool {
	// A control character would add lines to the test configuration.
	if rule == "" || HasControlChar(rule) {
		return false
	}
	key := "f:" + rule
	if section == BackendRule {
		key = "b:" + rule
	}

	checker.Lock()
	defer checker.Unlock()
	if checker.binary == "" {
		return true
	}
	if valid, found := checker.cache[key]; found {
		return valid
	}
	valid := haproxyAcceptsRule(checker.binary, section, rule)
	if len(checker.cache) >= checkCacheSize {
		checker.cache = map[string]bool{}
	}
	checker.cache[key] = valid
	return valid
}

// haproxyAcceptsRule runs a configuration check on a minimal configuration holding only rule.
func haproxyAcceptsRule(binary string, section RuleSection, rule string) bool {
	frontendRule, backendRule := "", ""
	if section == BackendRule {
		backendRule = "  " + rule + "\n"
	} else {
		frontendRule = "  " + rule + "\n"
	}
	config := "defaults\n  mode http\n  timeout client 5s\n  timeout server 5s\n  timeout connect 5s\n" +
		"frontend rule-check-frontend\n  bind 127.0.0.1:1\n" + frontendRule +
		"backend " + CheckBackend + "\n" + backendRule
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
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
	logger.Errorf("annotation validation: unable to run '%s': %s", binary, err)
	return true
}
