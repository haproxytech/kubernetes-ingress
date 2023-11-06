package definition

import _ "embed"

//go:embed defaults.core.haproxy.org.yaml
var DefaultsV1alpha2 []byte

//go:embed globals.core.haproxy.org.yaml
var GlobalsV1alpha2 []byte

//go:embed backends.core.haproxy.org.yaml
var BackendsV1alpha2 []byte

//go:embed upgrade/defaults.core.haproxy.org.yaml
var DefaultsV1alpha1V1alpha2 []byte

//go:embed upgrade/globals.core.haproxy.org.yaml
var GlobalsV1alpha1V1alpha2 []byte

//go:embed upgrade/backends.core.haproxy.org.yaml
var BackendsV1alpha1V1alpha2 []byte

func GetCRDs() map[string][]byte {
	return map[string][]byte{
		"defaults.core.haproxy.org": DefaultsV1alpha2,
		"globals.core.haproxy.org":  GlobalsV1alpha2,
		"backends.core.haproxy.org": BackendsV1alpha2,
	}
}

func GetCRDsUpgrade() map[string][]byte {
	return map[string][]byte{
		"defaults.core.haproxy.org": DefaultsV1alpha1V1alpha2,
		"globals.core.haproxy.org":  GlobalsV1alpha1V1alpha2,
		"backends.core.haproxy.org": BackendsV1alpha1V1alpha2,
	}
}
