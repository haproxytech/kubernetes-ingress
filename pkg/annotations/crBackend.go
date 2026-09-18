package annotations

import (
	"os"
	"path/filepath"
	"strings"
)

// BackendsInError names the backends whose own directives HAProxy rejected.
// dir holds the rejected configuration file named in configErr.
// Lines inside a config snippet are left to the snippet handling.
func BackendsInError(configErr error, dir string) (backends []string, err error) {
	if configErr == nil {
		return nil, nil
	}
	file, lineNumbers, err := processConfigurationError(configErr)
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(filepath.Join(dir, filepath.Base(file)))
	if err != nil {
		return nil, err
	}
	return backendsInError(string(contents), lineNumbers), nil
}

func backendsInError(contents string, lineNumbers []int) (backends []string) {
	lines := strings.Split(contents, "\n")
	seen := map[string]struct{}{}
	for _, lineNumber := range lineNumbers {
		if lineNumber < 1 || lineNumber > len(lines) {
			continue
		}
		name := backendOwningLine(lines, lineNumber-1)
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		backends = append(backends, name)
	}
	return backends
}

// backendOwningLine walks up from a directive to its section header.
// Empty when the directive sits in a config snippet or outside a backend.
func backendOwningLine(lines []string, index int) string {
	for i := index; i >= 0; i-- {
		line := lines[i]
		if line == COMMENT_CFG_SNIPPET_BEGIN {
			return ""
		}
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		// Reached a section header: "backend <name> [from <defaults>]".
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == BACKEND {
			return fields[1]
		}
		return ""
	}
	return ""
}
