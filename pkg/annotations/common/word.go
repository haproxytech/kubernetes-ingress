package common

import (
	"strconv"
	"strings"
)

// UnquotedWord decodes s the way HAProxy reads one unquoted word.
// It fails when s would split, join or abort the configuration line.
func UnquotedWord(s string) (string, bool) {
	word := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= ' ' || c == 0x7f || c == '"' || c == '\'' || c == '#' {
			return "", false
		}
		if c != '\\' {
			word = append(word, c)
			continue
		}
		i++
		switch {
		case i == len(s):
			// A trailing backslash escapes the separator after the word.
			return "", false
		case s[i] == 'x':
			if i+2 >= len(s) {
				return "", false
			}
			value, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
			if err != nil || value < ' ' || value == 0x7f {
				return "", false
			}
			word = append(word, byte(value))
			i += 2
		case s[i] == '\\' || s[i] == '$':
			word = append(word, s[i])
		case s[i] <= ' ' || s[i] == 0x7f || strings.IndexByte(`"'#rnt`, s[i]) >= 0:
			return "", false
		default:
			// HAProxy keeps unknown escapes, which regexes rely on.
			word = append(word, '\\', s[i])
		}
	}
	return string(word), s != ""
}
