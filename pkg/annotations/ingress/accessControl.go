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
	input := a.GetAnnotationValue(annotations...)
	if input == "" {
		return err
	}

	if strings.HasPrefix(input, "patterns/") {
		a.rules.Add(&rules.ReqDeny{
			// SrcIPsMap: "/etc/haproxy/" + maps.Path(input),
			SrcIPsMap: maps.Path(input),
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

	if !a.maps.MapExists(mapName) {
		for _, address := range strings.Split(input, ",") {
			address = strings.TrimSpace(address)
			if ip := net.ParseIP(address); ip == nil {
				if _, _, err := net.ParseCIDR(address); err != nil {
					return fmt.Errorf("incorrect address '%s' in %s annotation", address, a.name)
				}
			}
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
	value := common.GetValue(a.name, annotations...)
	if value == "" { // fallback to deprecated annotation name
		value = common.GetValue(a.deprecatedName, annotations...)
		if value != "" {
			logger.Warningf("annotation %q is deprecated and will be removed in a future version. Please use %q instead.", a.deprecatedName, a.name)
		}
	}
	return value
}
