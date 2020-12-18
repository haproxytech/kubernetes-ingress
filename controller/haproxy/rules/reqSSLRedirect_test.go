package rules

import (
	"fmt"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/models/v2"
)

type haproxyClient struct {
	api.HAProxyClient
}

func (client haproxyClient) FrontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule) error {
	if *rule.Index != 0 {
		return fmt.Errorf("value Index should be 0 but is %d", *rule.Index)
	}
	if rule.Type != "redirect" {
		return fmt.Errorf("value Type should be 'redirect' but is %s", rule.Type)
	}
	if *rule.RedirCode != 302 {
		return fmt.Errorf("value RedirCode should be 302 but is %d", *rule.RedirCode)
	}
	if rule.RedirValue != "https://%[hdr(host),field(1,:)]:8443%[capture.req.uri]" {
		return fmt.Errorf("value RedirValue should be 'https://%%[hdr(host),field(1,:)]:8443%%[capture.req.uri]' but is %s", rule.RedirValue)
	}
	if rule.RedirType != "location" {
		return fmt.Errorf("value RedirType should be 'location' but is %s", rule.RedirType)
	}
	return nil
}

func TestReqSSLRedirect(t *testing.T) {

	var sslRedirectCode int64 = 302
	sslRedirectPort := 8443
	reqSSLRedirect := ReqSSLRedirect{
		RedirectCode: sslRedirectCode,
		RedirectPort: sslRedirectPort,
	}
	f := models.Frontend{
		Name: "http",
	}

	if err := reqSSLRedirect.Create(haproxyClient{}, &f); err != nil {
		t.Fatal(err)
	}
}
