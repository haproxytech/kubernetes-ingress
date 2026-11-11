package ingress

import (
	"fmt"
	"net"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

type AccessControl struct {
	maps  maps.Maps
	rules *rules.List

	// name of the annotation managing the access control (either allow-list or deny-list)
	name string

	// deprecatedName is the deprecated annotation's name managing the access control (either whitelist or blacklist).
	// It will be removed in a future version in favor name.
	deprecatedName string
	allowList      bool
}

func NewDenyList(n string, r *rules.List, m maps.Maps) *AccessControl {
	return &AccessControl{name: n, deprecatedName: "blacklist", rules: r, maps: m}
}

func NewAllowList(n string, r *rules.List, m maps.Maps) *AccessControl {
	return &AccessControl{name: n, deprecatedName: "whitelist", rules: r, maps: m, allowList: true}
}

func (a *AccessControl) GetName() string {
	return a.name
}

func (a *AccessControl) Process(k store.K8s, annotations ...map[string]string) (err error) {
	if common.IsSet(a.name, annotations...) {
		utils.Once("deprecation-"+a.name, func() {
			logger.Warningf("DEPRECATED: annotation %q will be removed in a future version", a.name)
		})
	}
	input := a.GetAnnotationValue(annotations...)
	if input == "" {
		return err
	}
	defer func() {
		if err != nil {
			// Invalid access control must not leave the protected route open.
			a.rules.Add(&rules.ReqDeny{})
			err = fmt.Errorf("%w; denying access", err)
		}
	}()
	if strings.ContainsAny(input, "\r\n#") {
		return fmt.Errorf("%s annotation contains a newline or comment marker", a.name)
	}
	input = strings.TrimSpace(input)

	if strings.HasPrefix(input, "patterns/") {
		pattern, errPat := patternPath(k, a.name, input)
		if errPat != nil {
			return errPat
		}
		a.rules.Add(&rules.ReqDeny{
			SrcIPsMap: pattern,
			AllowList: a.allowList,
		})

		return err
	}

	var mapName maps.Name
	if a.allowList {
		mapName = maps.Name("allowlist-" + utils.Hash([]byte(input)))
	} else {
		mapName = maps.Name("denylist-" + utils.Hash([]byte(input)))
	}

	var addresses []string
	for _, address := range strings.Split(input, ",") {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		if ip := net.ParseIP(address); ip == nil {
			if _, _, err := net.ParseCIDR(address); err != nil {
				return fmt.Errorf("incorrect address '%s' in %s annotation", address, a.name)
			}
		}
		addresses = append(addresses, address)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("no valid address found in %s annotation", a.name)
	}
	if !a.maps.MapExists(mapName) {
		for _, address := range addresses {
			a.maps.MapAppend(mapName, address)
		}
	}
	a.rules.Add(&rules.ReqDeny{
		SrcIPsMap: maps.GetPath(mapName),
		AllowList: a.allowList,
	})
	return err
}

// GetAnnotationValue returns the annotation value of the AccessControl. If the annotation is not defined, it returns an empty string.
// If the "new" annotation's name is not defined, it falls back to the deprecated name and logs a warning.
// Deprecated: remove this function when the deprecated annotation name will not be supported anymore.
func (a *AccessControl) GetAnnotationValue(annotations ...map[string]string) string {
	// Keep invalid values visible so Process can deny access instead of dropping protection.
	for _, name := range []string{a.name, a.deprecatedName} {
		for _, annotation := range annotations {
			value, found := annotation[name]
			if !found {
				continue
			}
			if value == "" {
				break
			}
			if name == a.deprecatedName {
				utils.Once("deprecation-"+a.deprecatedName, func() {
					logger.Warningf("DEPRECATED: annotation %q will be removed in a future version, use %q instead", a.deprecatedName, a.name)
				})
			}
			return value
		}
	}
	return ""
}

// patternPath rejects any reference that could escape or miss the pattern files ConfigMap.
func patternPath(k store.K8s, annotation, value string) (maps.Path, error) {
	file := strings.TrimPrefix(value, "patterns/")
	if file == "" || file == "." || file == ".." {
		return "", fmt.Errorf("%s: invalid pattern file name", annotation)
	}
	for _, r := range file {
		if r <= ' ' || r == 0x7f || r == '#' || r == '"' || r == '/' || r == '\\' {
			return "", fmt.Errorf("%s: invalid character %q in pattern file reference '%s'", annotation, r, value)
		}
	}
	if k.ConfigMaps.PatternFiles == nil {
		return "", fmt.Errorf("%s: pattern file '%s' requires a pattern files ConfigMap", annotation, file)
	}
	content, declared := k.ConfigMaps.PatternFiles.Annotations[file]
	if !declared {
		return "", fmt.Errorf("%s: pattern file '%s' is not declared in the pattern files ConfigMap", annotation, file)
	}
	for index, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		// HAProxy ignores leading spaces, but trailing spaces invalidate IP patterns.
		line = strings.TrimSuffix(strings.TrimLeft(line, " \t"), "\r")
		if line == "" || net.ParseIP(line) != nil {
			continue
		}
		ip, mask, masked := strings.Cut(line, "/")
		if masked && !strings.Contains(line, ":") && net.ParseIP(ip).To4() != nil && net.ParseIP(mask).To4() != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(line); err != nil {
			return "", fmt.Errorf("%s: pattern file '%s' has an invalid IP or CIDR at line %d", annotation, file, index+1)
		}
	}
	return maps.Path(value), nil
}
