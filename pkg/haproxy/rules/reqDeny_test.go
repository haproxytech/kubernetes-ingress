package rules

import (
	"errors"
	"testing"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
)

type denyTestClient struct {
	api.HAProxyClient
	httpRule models.HTTPRequestRule
	tcpRule  models.TCPRequestRule
	acl      string
	err      error
}

func (c *denyTestClient) FrontendHTTPRequestRuleCreate(_ int64, _ string, rule models.HTTPRequestRule, acl string) error {
	c.httpRule = rule
	c.acl = acl
	return c.err
}

func (c *denyTestClient) FrontendTCPRequestRuleCreate(_ int64, _ string, rule models.TCPRequestRule, acl string) error {
	c.tcpRule = rule
	c.acl = acl
	return c.err
}

func TestReqDenyCreate(t *testing.T) {
	tests := []struct {
		name     string
		rule     ReqDeny
		cond     string
		condTest string
	}{
		{name: "deny without a file", rule: ReqDeny{}},
		{name: "empty allow-list", rule: ReqDeny{AllowList: true}},
		{
			name:     "allow-list",
			rule:     ReqDeny{SrcIPsMap: "patterns/ips", AllowList: true},
			cond:     "if",
			condTest: "!{ src -f patterns/ips }",
		},
		{
			name:     "deny-list",
			rule:     ReqDeny{SrcIPsMap: "patterns/ips"},
			cond:     "if",
			condTest: "{ src -f patterns/ips }",
		},
	}
	for _, mode := range []string{"http", "tcp"} {
		for _, tt := range tests {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				client := &denyTestClient{}
				frontend := &models.Frontend{FrontendBase: models.FrontendBase{Name: "test", Mode: mode}}
				const acl = "{ var(txn.path_match) -m dom protected-rule }"
				require.NoError(t, tt.rule.Create(client, frontend, acl))
				assert.Equal(t, acl, client.acl)
				if mode == "tcp" {
					assert.Equal(t, "content", client.tcpRule.Type)
					assert.Equal(t, "reject", client.tcpRule.Action)
					assert.Equal(t, tt.cond, client.tcpRule.Cond)
					assert.Equal(t, tt.condTest, client.tcpRule.CondTest)
				} else {
					assert.Equal(t, "deny", client.httpRule.Type)
					require.NotNil(t, client.httpRule.DenyStatus)
					assert.Equal(t, int64(403), *client.httpRule.DenyStatus)
					assert.Equal(t, tt.cond, client.httpRule.Cond)
					assert.Equal(t, tt.condTest, client.httpRule.CondTest)
				}
				client.err = errors.New("cannot create rule")
				assert.ErrorIs(t, tt.rule.Create(client, frontend, acl), client.err)
			})
		}
	}
}

func TestGlobalDenyRemainsGlobalWithInheritedDeclarations(t *testing.T) {
	for _, declarations := range [][]bool{{false, true}, {true, false}, {true, true, false}} {
		rs := *New()
		rule := ReqDeny{}
		frontend := &models.Frontend{FrontendBase: models.FrontendBase{Name: "test", Mode: "http"}}
		for _, ingress := range declarations {
			require.NoError(t, rs.AddRule(frontend.Name, rule, ingress))
		}
		client := &denyTestClient{}
		rs.refreshRule(client, REQ_DENY, 0, frontend)
		assert.Empty(t, client.acl)
		assert.Empty(t, client.httpRule.Cond)
		assert.Empty(t, client.httpRule.CondTest)
		assert.Equal(t, "deny", client.httpRule.Type)
	}
	instance.Reset()
}

func TestDenyScopeRecomputedEachReconciliation(t *testing.T) {
	instance.Reset()
	t.Cleanup(instance.Reset)
	rs := *New()
	rule := ReqDeny{}
	frontend := &models.Frontend{FrontendBase: models.FrontendBase{Name: "test", Mode: "http"}}
	steps := []struct {
		declarations []bool
		ingress      bool
		reload       bool
	}{
		{declarations: []bool{false, true}, reload: true},
		{declarations: []bool{true}, ingress: true, reload: true},
		{declarations: []bool{true, false}, reload: true},
		{declarations: []bool{true, false}},
		{declarations: []bool{false, true}},
		{declarations: []bool{true}, ingress: true, reload: true},
		{declarations: []bool{true}, ingress: true},
	}
	for _, step := range steps {
		rs.CleanRules()
		instance.Reset()
		for _, ingress := range step.declarations {
			require.NoError(t, rs.AddRule(frontend.Name, rule, ingress))
		}
		client := &denyTestClient{}
		rs.refreshRule(client, REQ_DENY, 0, frontend)
		assert.Equal(t, step.ingress, rs[frontend.Name].meta[GetID(rule)].ingress)
		if step.ingress {
			assert.Equal(t, "{ var("+HTTPACLVar+") -m dom "+string(GetID(rule))+" }", client.acl)
		} else {
			assert.Empty(t, client.acl)
		}
		assert.Equal(t, step.reload, instance.NeedReload())
	}
}
