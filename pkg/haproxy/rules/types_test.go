package rules

import "testing"

func TestRuleTypeOrder_RedirectBeforeAuth(t *testing.T) {
	if REQ_REDIRECT >= REQ_AUTH {
		t.Fatalf("expected REQ_REDIRECT (%d) to be evaluated before REQ_AUTH (%d)", REQ_REDIRECT, REQ_AUTH)
	}
}
