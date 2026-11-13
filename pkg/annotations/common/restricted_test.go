package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRestrictedKeyword(t *testing.T) {
	restricted := map[string]string{
		"%[env(HOME)]":                       "env",
		"prefix-%[env(HOME),upper]":          "env",
		"%{+Q}[env(HOME)]":                   "env",
		"env(HOME) -m beg /root":             "env",
		"x-env(HOME)":                        "env",
		"%[str(user),map(/etc/passwd)]":      "map",
		"path,map_beg(/etc/passwd) -m found": "map_beg",
		"%[src,map_ip_int(/etc/hosts)]":      "map_ip_int",
		"%[str(a),set-var(txn.host)]":        "set-var",
		"%[str(a),unset-var(txn.host)]":      "unset-var",
		"%[dump_all_vars]":                   "dump_all_vars",
		"%[dump_all_vars(txn)]":              "dump_all_vars",
	}
	for expr, keyword := range restricted {
		require.Equal(t, keyword, RestrictedKeyword(expr), expr)
	}
	for _, expr := range []string{
		"",
		"plain value",
		"%[req.hdr(X-Env)]",
		"%[var(txn.host)]",
		"path_beg /env /map /set-var",
		"cookie(env) -m found",
		"hdr(map) -i env",
		"%[src]",
		"sitemap(1)",
		"%[ENV(HOME)]",
	} {
		require.Empty(t, RestrictedKeyword(expr), expr)
	}
}

func TestSetByTenant(t *testing.T) {
	ingress := map[string]string{"a": "1"}
	configMap := map[string]string{"a": "2", "b": "3"}
	require.True(t, SetByTenant("a", ingress, configMap))
	require.False(t, SetByTenant("b", ingress, configMap))
	require.False(t, SetByTenant("c", ingress, configMap))
	require.False(t, SetByTenant("a", configMap))
	require.True(t, SetByTenant("a", ingress, map[string]string{}, configMap))

	require.Equal(t, "env", TenantRestrictedKeyword("a", "%[env(X)]", ingress, configMap))
	require.Empty(t, TenantRestrictedKeyword("b", "%[env(X)]", ingress, configMap))
	require.Empty(t, TenantRestrictedKeyword("a", "%[src]", ingress, configMap))
}
