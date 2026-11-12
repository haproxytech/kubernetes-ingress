package ingress

import (
	"errors"
	"strconv"
	"strings"
)

// Each limit keeps one hash near half a second of work per configuration check.
const (
	minBcryptCost = 4
	maxBcryptCost = 12
	minSHARounds  = 1000
	maxSHARounds  = 1000000
)

var errUnsupportedHash = errors.New("unsupported or too expensive password hash")

func isCryptChar(c byte) bool {
	return c == '.' || c == '/' || (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// isCryptWord reports whether s has minLen to maxLen characters of the crypt(3) alphabet.
func isCryptWord(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	for i := range len(s) {
		if !isCryptChar(s[i]) {
			return false
		}
	}
	return true
}

// passwordHashError rejects hashes that crypt(3) can refuse or that stall the configuration check.
func passwordHashError(hash string) error {
	var ok bool
	switch {
	case strings.HasPrefix(hash, "$1$"):
		ok = validSaltedHash(hash[3:], 8, 22)
	case strings.HasPrefix(hash, "$5$"):
		ok = validSHAHash(hash[3:], 43)
	case strings.HasPrefix(hash, "$6$"):
		ok = validSHAHash(hash[3:], 86)
	case strings.HasPrefix(hash, "$2a$"), strings.HasPrefix(hash, "$2b$"), strings.HasPrefix(hash, "$2y$"):
		ok = validBcryptHash(hash[4:])
	default:
		// Traditional DES; a leading '$' or '_' selects other schemes.
		ok = isCryptWord(hash, 13, 13)
	}
	if !ok {
		return errUnsupportedHash
	}
	return nil
}

// validSaltedHash checks "<salt>$<hash>" as MD5 and SHA crypt write it.
func validSaltedHash(s string, maxSalt, hashLen int) bool {
	salt, hash, found := strings.Cut(s, "$")
	return found && isCryptWord(salt, 1, maxSalt) && isCryptWord(hash, hashLen, hashLen)
}

func validSHAHash(s string, hashLen int) bool {
	if rest, found := strings.CutPrefix(s, "rounds="); found {
		digits, tail, hasTail := strings.Cut(rest, "$")
		// crypt(3) refuses a leading zero.
		if !hasTail || digits == "" || digits[0] == '0' || len(digits) > 7 {
			return false
		}
		rounds, err := strconv.Atoi(digits)
		if err != nil || rounds < minSHARounds || rounds > maxSHARounds {
			return false
		}
		s = tail
	}
	return validSaltedHash(s, 16, hashLen)
}

func validBcryptHash(s string) bool {
	if len(s) != 56 || s[2] != '$' || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return false
	}
	cost := int(s[0]-'0')*10 + int(s[1]-'0')
	return cost >= minBcryptCost && cost <= maxBcryptCost && isCryptWord(s[3:], 53, 53)
}
