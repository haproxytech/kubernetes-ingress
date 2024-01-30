package definition

import _ "embed"

//go:embed ingress.v1.haproxy.org_defaults.yaml
var Defaults []byte

//go:embed ingress.v1.haproxy.org_globals.yaml
var Globals []byte

//go:embed ingress.v1.haproxy.org_backends.yaml
var Backends []byte

func GetCRDs() map[string][]byte {
	return map[string][]byte{
		"ingress.v1.haproxy.org_defaults.yaml": Defaults,
		"ingress.v1.haproxy.org_globals.yaml":  Globals,
		"ingress.v1.haproxy.org_backends.yaml": Backends,
	}
}
