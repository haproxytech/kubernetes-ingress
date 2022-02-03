package ingress

import (
	"fmt"
	"net"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type AccessControl struct {
	name      string
	rules     *rules.Rules
	maps      maps.MapFiles
	whitelist bool
}

func NewBlackList(n string, r *rules.Rules, m maps.MapFiles) *AccessControl {
	return &AccessControl{name: n, rules: r, maps: m}
}

func NewWhiteList(n string, r *rules.Rules, m maps.MapFiles) *AccessControl {
	return &AccessControl{name: n, rules: r, maps: m, whitelist: true}
}

func (a *AccessControl) GetName() string {
	return a.name
}

func (a *AccessControl) Process(k store.K8s, annotations ...map[string]string) (err error) {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		return
	}

	if strings.HasPrefix(input, "patterns/") {
		a.rules.Add(&rules.ReqDeny{
			SrcIPsMap: maps.Path(input),
			Whitelist: a.whitelist,
		})

		return
	}

	var mapName maps.Name
	if a.whitelist {
		mapName = maps.Name("whitelist-" + utils.Hash([]byte(input)))
	} else {
		mapName = maps.Name("blacklist-" + utils.Hash([]byte(input)))
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
		SrcIPsMap: maps.GetPath(mapName),
		Whitelist: a.whitelist,
	})
	return
}
