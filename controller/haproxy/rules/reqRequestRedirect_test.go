package rules

import (
	"fmt"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/models/v2"
)

type haproxyClientHTTP struct {
	api.HAProxyClient
}

func (client haproxyClientHTTP) FrontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule) error {
	if *rule.Index != 0 {
		return fmt.Errorf("value Index should be 0 but is %d", *rule.Index)
	}
	if rule.Type != "redirect" {
		return fmt.Errorf("value Type should be 'redirect' but is %s", rule.Type)
	}
	if *rule.RedirCode != 302 {
		return fmt.Errorf("value RedirCode should be 302 but is %d", *rule.RedirCode)
	}
	if rule.RedirValue != "http://example.com%[capture.req.uri]" {
		return fmt.Errorf("value RedirValue should be 'http://example.com%%[capture.req.uri]' but is %s", rule.RedirValue)
	}
	if rule.RedirType != "location" {
		return fmt.Errorf("value RedirType should be 'location' but is %s", rule.RedirType)
	}
	return nil
}

type haproxyClientHTTPS struct {
	api.HAProxyClient
}

func (client haproxyClientHTTPS) FrontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule) error {
	if *rule.Index != 0 {
		return fmt.Errorf("value Index should be 0 but is %d", *rule.Index)
	}
	if rule.Type != "redirect" {
		return fmt.Errorf("value Type should be 'redirect' but is %s", rule.Type)
	}
	if *rule.RedirCode != 302 {
		return fmt.Errorf("value RedirCode should be 302 but is %d", *rule.RedirCode)
	}
	if rule.RedirValue != "https://example.com%[capture.req.uri]" {
		return fmt.Errorf("value RedirValue should be 'https://example.com%%[capture.req.uri]' but is %s", rule.RedirValue)
	}
	if rule.RedirType != "location" {
		return fmt.Errorf("value RedirType should be 'location' but is %s", rule.RedirType)
	}
	return nil
}

func TestReqRequestRedirect(t *testing.T) {

	var redirectCode int64 = 302
	domain := "example.com"
	requestRedirection := RequestRedirect{
		RedirectCode: redirectCode,
		Host:         domain,
	}
	f := models.Frontend{
		Name: "http",
	}

	if err := requestRedirection.Create(haproxyClientHTTP{}, &f); err != nil {
		t.Fatal(err)
	}
	requestRedirection.SSLRequest = true
	if err := requestRedirection.Create(haproxyClientHTTPS{}, &f); err != nil {
		t.Fatal(err)
	}
}
