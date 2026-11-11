package api

import (
	"strings"
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrontendDenyWithoutSourceCondition(t *testing.T) {
	for _, mode := range []string{"http", "tcp"} {
		for _, acl := range []string{"", "{ var(txn.path_match) -m dom protected-rule }"} {
			t.Run(mode+"/"+acl, func(t *testing.T) {
				frontend := &Frontend{Frontend: models.Frontend{FrontendBase: models.FrontendBase{Name: "test", Mode: mode}}}
				other := &Frontend{}
				client := &clientNative{frontends: map[string]*Frontend{"test": frontend, "other": other}}
				var cond, condTest string
				if mode == "tcp" {
					err := client.FrontendTCPRequestRuleCreate(0, "test", models.TCPRequestRule{
						Type: "content", Action: "reject",
					}, acl)
					require.NoError(t, err)
					require.Len(t, frontend.TCPRequestRuleList, 1)
					cond = frontend.TCPRequestRuleList[0].Cond
					condTest = frontend.TCPRequestRuleList[0].CondTest
				} else {
					err := client.FrontendHTTPRequestRuleCreate(0, "test", models.HTTPRequestRule{Type: "deny"}, acl)
					require.NoError(t, err)
					require.Len(t, frontend.HTTPRequestRuleList, 1)
					cond = frontend.HTTPRequestRuleList[0].Cond
					condTest = frontend.HTTPRequestRuleList[0].CondTest
				}
				if acl == "" {
					assert.Empty(t, cond)
				} else {
					assert.Equal(t, "if", cond)
				}
				assert.Equal(t, acl, strings.TrimSpace(condTest))
				assert.Empty(t, other.HTTPRequestRuleList)
				assert.Empty(t, other.TCPRequestRuleList)
			})
		}
	}
}
