package definition

import _ "embed"

//go:embed ingress.v3.haproxy.org_defaults.yaml
var Defaults []byte

//go:embed ingress.v3.haproxy.org_globals.yaml
var Globals []byte

//go:embed ingress.v3.haproxy.org_backends.yaml
var Backends []byte

//go:embed ingress.v3.haproxy.org_tcps.yaml
var TCPs []byte

func GetCRDs() map[string][]byte {
	return map[string][]byte{
		"defaults.ingress.v3.haproxy.org": Defaults,
		"globals.ingress.v3.haproxy.org":  Globals,
		"backends.ingress.v3.haproxy.org": Backends,
		"tcps.ingress.v3.haproxy.org":     TCPs,
	}
}
