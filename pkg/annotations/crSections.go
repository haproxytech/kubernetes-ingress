package annotations

import (
	"os"
	"path/filepath"
	"strings"
)

// Section kinds a custom resource can produce in the rendered configuration.
const (
	SectionGlobal   = "global"
	SectionDefaults = "defaults"
	SectionFrontend = "frontend"
	SectionBackend  = "backend"
)

// Section identifies a section of the rendered configuration.
type Section struct {
	Kind string
	Name string // empty for the global section
}

// Key is the section identifier used to record which resource produced it.
func (s Section) Key() string {
	if s.Name == "" {
		return s.Kind
	}
	return s.Kind + "/" + s.Name
}

// SectionsInError lists the sections whose own directives HAProxy rejected.
// dir holds the rejected configuration file named in configErr.
// Lines inside a config snippet are left to the snippet handling.
func SectionsInError(configErr error, dir string) (sections []Section, err error) {
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
	return sectionsInError(string(contents), lineNumbers), nil
}

func sectionsInError(contents string, lineNumbers []int) (sections []Section) {
	lines := strings.Split(contents, "\n")
	seen := map[Section]struct{}{}
	for _, lineNumber := range lineNumbers {
		if lineNumber < 1 || lineNumber > len(lines) {
			continue
		}
		section, ok := sectionOwningLine(lines, lineNumber-1)
		if !ok {
			continue
		}
		if _, dup := seen[section]; dup {
			continue
		}
		seen[section] = struct{}{}
		sections = append(sections, section)
	}
	return sections
}

// sectionOwningLine walks up from a directive to its section header.
// Not ok when the directive sits in a config snippet or in a section no resource produces.
func sectionOwningLine(lines []string, index int) (section Section, ok bool) {
	for i := index; i >= 0; i-- {
		line := lines[i]
		if line == COMMENT_CFG_SNIPPET_BEGIN {
			return section, false
		}
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		// Reached a section header: "<kind> [<name>] [from <defaults>]".
		fields := strings.Fields(line)
		switch fields[0] {
		case SectionGlobal:
			return Section{Kind: SectionGlobal}, true
		case SectionDefaults, SectionFrontend, SectionBackend:
			if len(fields) >= 2 {
				return Section{Kind: fields[0], Name: fields[1]}, true
			}
		}
		return section, false
	}
	return section, false
}
