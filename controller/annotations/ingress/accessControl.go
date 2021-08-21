package ingress

import (
	"fmt"
	"net"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type AccessControl struct {
	name      string
	rules     *haproxy.Rules
	maps      haproxy.Maps
	whitelist bool
}

func NewBlackList(n string, rules *haproxy.Rules, m haproxy.Maps) *AccessControl {
	return &AccessControl{name: n, rules: rules, maps: m}
}

func NewWhiteList(n string, rules *haproxy.Rules, m haproxy.Maps) *AccessControl {
	return &AccessControl{name: n, rules: rules, maps: m, whitelist: true}
}

func (a *AccessControl) GetName() string {
	return a.name
}

func (a *AccessControl) Process(input string) (err error) {
	if input == "" {
		return
	}
	var mapName string
	var whitelist bool
	if a.whitelist {
		mapName = "whitelist-" + utils.Hash([]byte(input))
		whitelist = true
	} else {
		mapName = "blacklist-" + utils.Hash([]byte(input))
	}
	if !a.maps.Exists(mapName) {
		for _, address := range strings.Split(input, ",") {
			address = strings.TrimSpace(address)
			if ip := net.ParseIP(address); ip == nil {
				if _, _, err := net.ParseCIDR(address); err != nil {
					return fmt.Errorf("incorrect address '%s' in blacklist annotation'", address)
				}
			}
			a.maps.AppendRow(mapName, address)
		}
	}
	a.rules.Add(&rules.ReqDeny{
		SrcIPsMap: mapName,
		Whitelist: whitelist,
	})
	return
}
