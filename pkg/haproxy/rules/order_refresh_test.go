package rules

import (
	"testing"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
)

// orderingTestClient overrides only methods touched by SectionRules.RefreshRules and rule Create methods.
// The embedded interface satisfies the full api.HAProxyClient method set for compilation.
type orderingTestClient struct {
	api.HAProxyClient
	frontendRules map[string][]models.HTTPRequestRule
}

func (c *orderingTestClient) UserListDeleteAll() error {
	return nil
}

func (c *orderingTestClient) FrontendGet(frontendName string) (models.Frontend, error) {
	return models.Frontend{
		FrontendBase: models.FrontendBase{
			Name: frontendName,
			Mode: "http",
		},
	}, nil
}

func (c *orderingTestClient) FrontendRuleDeleteAll(frontend string) {
	if c.frontendRules == nil {
		c.frontendRules = map[string][]models.HTTPRequestRule{}
	}
	c.frontendRules[frontend] = nil
}

func (c *orderingTestClient) UserListExistsByGroup(group string) (bool, error) {
	return true, nil
}

func (c *orderingTestClient) UserListCreateByGroup(group string, userPasswordMap map[string][]byte) error {
	return nil
}

func (c *orderingTestClient) FrontendHTTPRequestRuleCreate(id int64, frontend string, rule models.HTTPRequestRule, ingressACL string) error {
	if c.frontendRules == nil {
		c.frontendRules = map[string][]models.HTTPRequestRule{}
	}

	rules := c.frontendRules[frontend]
	if id == 0 {
		// HAProxy client treats index 0 insertion as rule prepending.
		rules = append([]models.HTTPRequestRule{rule}, rules...)
	} else {
		rules = append(rules, rule)
	}
	c.frontendRules[frontend] = rules
	return nil
}

func TestRefreshRules_RedirectBeforeAuthInFinalHTTPRules(t *testing.T) {
	rs := New()
	frontend := "http"

	err := rs.AddRule(frontend, RequestRedirect{
		SSLRedirect:  true,
		RedirectPort: 8443,
		RedirectCode: 302,
	}, false)
	if err != nil {
		t.Fatalf("failed to add redirect rule: %v", err)
	}

	err = rs.AddRule(frontend, ReqBasicAuth{
		AuthGroup: "site.example.com",
		AuthRealm: "Authentication-Required",
	}, false)
	if err != nil {
		t.Fatalf("failed to add auth rule: %v", err)
	}

	client := &orderingTestClient{}
	rs.RefreshRules(client)

	created := client.frontendRules[frontend]
	if len(created) < 2 {
		t.Fatalf("expected at least 2 http-request rules, got %d", len(created))
	}

	if created[0].Type != "redirect" {
		t.Fatalf("expected first rule to be redirect, got %q", created[0].Type)
	}
	if created[1].Type != "auth" {
		t.Fatalf("expected second rule to be auth, got %q", created[1].Type)
	}
}
