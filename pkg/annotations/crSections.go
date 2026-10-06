package annotations

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Section kinds a custom resource can produce in the rendered configuration.
const (
	SectionGlobal   = "global"
	SectionDefaults = "defaults"
	SectionFrontend = "frontend"
	SectionBackend  = "backend"
)

// Fences client-native writes around config-snippet directive content; the
// lines inside belong to custom annotations, not to a custom resource.
const (
	COMMENT_CFG_SNIPPET_END   = "  ###_config-snippet_### END"
	COMMENT_CFG_SNIPPET_BEGIN = "  ###_config-snippet_### BEGIN"
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
// Lines inside custom annotation content belong to no resource.
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
// Not ok when the directive sits in custom annotation content or in a section no resource produces.
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

func getErrorLineNumberAndFileName(msg string) (lineNumber int, file string, err error) {
	lineNumber = -1
	openSquareBracket := strings.Index(msg, "[")
	if openSquareBracket == -1 {
		return lineNumber, file, err
	}
	closeSquareBracket := strings.Index(msg, "]")
	if closeSquareBracket == -1 {
		return lineNumber, file, err
	}
	location := msg[openSquareBracket+1 : closeSquareBracket]
	fileLineNumber := strings.Split(location, ":")
	// The error line number and file name of configuration file is in format [file:line number] in the reporting error line
	if len(fileLineNumber) == 2 {
		file = fileLineNumber[0]
		lineNumber, err = strconv.Atoi(fileLineNumber[1])
		if err != nil {
			return lineNumber, file, err
		}
	} else {
		err = fmt.Errorf("invalid configuration error location: '%s'", location)
	}
	return lineNumber, file, err
}

func processConfigurationError(configErr error) (file string, lineNumbers []int, err error) {
	// Break error message into lines
	msgs := strings.Split(configErr.Error(), "\n")
	// storage of errors lines numbers
	lineNumbers = []int{}
	// Parse error message line to get lines in error in configuration file.
	for _, msg := range msgs {
		var lineNumber int
		var fileInError string
		if lineNumber, fileInError, err = getErrorLineNumberAndFileName(msg); err == nil && lineNumber >= 0 {
			if file == "" && fileInError != "" {
				file = fileInError
			}
			lineNumbers = append(lineNumbers, lineNumber)
		} else if err != nil {
			return file, lineNumbers, err
		}
	}
	return file, lineNumbers, err
}
