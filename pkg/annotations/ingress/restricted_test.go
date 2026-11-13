package ingress

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// TestTenantRestrictedKeywords checks that only ConfigMap values reach outside the request.
func TestTenantRestrictedKeywords(t *testing.T) {
	process := func(name string, annotations ...map[string]string) error {
		var list rules.List
		switch name {
		case "request-set-header":
			return NewReqSetHdr(name, &list).Process(store.K8s{}, annotations...)
		case "response-set-header":
			return NewResSetHdr(name, &list).Process(store.K8s{}, annotations...)
		case "set-host":
			return NewReqSetHost(name, &list).Process(store.K8s{}, annotations...)
		case "request-redirect":
			return NewHostRedirect(&list).NewAnnotation(name).Process(store.K8s{}, annotations...)
		case "request-capture":
			return NewReqCapture(&list).NewAnnotation(name).Process(store.K8s{}, annotations...)
		default:
			return NewReqPathRewrite(name, &list).Process(store.K8s{}, annotations...)
		}
	}

	tests := []struct{ name, value string }{
		{"request-set-header", "X-Leak %[env(SECRET)]"},
		{"request-set-header", "X-Leak %[str(user),map(/etc/haproxy/haproxy.cfg)]"},
		{"request-set-header", "X-Leak %[str(other),set-var(txn.host)]"},
		{"request-set-header", "X-Ok fine\nX-Leak %[env(SECRET)]"},
		{"response-set-header", "X-Leak %[env(SECRET)]"},
		{"response-set-header", "X-Leak %[dump_all_vars]"},
		{"set-host", "%[env(SECRET)].example.com"},
		{"request-redirect", "%[env(SECRET)].example.com"},
		{"request-capture", "env(SECRET)"},
		{"request-capture", "path,map_beg(/etc/passwd)"},
		{"path-rewrite", "/%[env(SECRET)]"},
		{"path-rewrite", "(.*) /%[path,map(/etc/passwd)]"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" "+tt.value, func(t *testing.T) {
			tenant := map[string]string{tt.name: tt.value}
			configMap := map[string]string{}
			require.ErrorContains(t, process(tt.name, tenant, configMap), "is not allowed")
			// The administrator owns the ConfigMap and keeps these keywords.
			require.NoError(t, process(tt.name, tenant))
			require.NoError(t, process(tt.name, map[string]string{}, tenant))
		})
	}

	allowed := map[string]string{"request-set-header": "X-Client %[src]", "request-capture": "cookie(env)"}
	for name, value := range allowed {
		require.NoError(t, process(name, map[string]string{name: value}, map[string]string{}), name)
	}
}
