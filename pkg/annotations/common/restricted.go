package common

import "regexp"

// restrictedKeyword finds fetches and converters that reach outside the request.
// They read the environment, read local files or change controller variables.
var restrictedKeyword = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.])((?:env|map(?:_[a-z]+)*|set-var|unset-var)\(|dump_all_vars)`)

// RestrictedKeyword returns the keyword of expr that tenants cannot use.
// It returns an empty string when expr holds none.
func RestrictedKeyword(expr string) string {
	match := restrictedKeyword.FindStringSubmatch(expr)
	if match == nil {
		return ""
	}
	keyword := match[1]
	if keyword[len(keyword)-1] == '(' {
		keyword = keyword[:len(keyword)-1]
	}
	return keyword
}

// SetByTenant reports whether the value of name comes from a tenant resource.
// The ConfigMap is always the last map, and its values belong to the administrator.
func SetByTenant(name string, annotations ...map[string]string) bool {
	for i, a := range annotations {
		if _, ok := a[name]; ok {
			return i < len(annotations)-1
		}
	}
	return false
}

// TenantRestrictedKeyword returns the restricted keyword of a tenant value.
// It returns an empty string for ConfigMap values and for allowed ones.
func TenantRestrictedKeyword(name, value string, annotations ...map[string]string) string {
	if !SetByTenant(name, annotations...) {
		return ""
	}
	return RestrictedKeyword(value)
}
