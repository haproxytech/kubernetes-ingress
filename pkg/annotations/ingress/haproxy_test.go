package ingress

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// TestValuesRefusedByHAProxy needs a real HAProxy, as only it knows what it refuses.
func TestValuesRefusedByHAProxy(t *testing.T) {
	binary, err := exec.LookPath("haproxy")
	if err != nil {
		t.Skip("haproxy is not installed")
	}
	t.Cleanup(func() { common.SetHAProxyBinary("") })
	common.SetHAProxyBinary(binary)

	// process runs the named annotations in order and returns the last error.
	type step struct{ name, value string }
	process := func(steps ...step) error {
		var list rules.List
		values := map[string]string{}
		for _, s := range steps {
			values[s.name] = s.value
		}
		capture := NewReqCapture(&list)
		hostRedirect := NewHostRedirect(&list)
		sslRedirect := NewHTTPSRedirect(&list, nil)
		rateLimit := NewReqRateLimit(&list, nil)
		cors := NewResSetCORS(&list)
		var last error
		for _, s := range steps {
			switch s.name {
			case "request-capture", "request-capture-len":
				last = capture.NewAnnotation(s.name).Process(store.K8s{}, values)
			case "request-set-header":
				last = NewReqSetHdr(s.name, &list).Process(store.K8s{}, values)
			case "response-set-header":
				last = NewResSetHdr(s.name, &list).Process(store.K8s{}, values)
			case "set-host":
				last = NewReqSetHost(s.name, &list).Process(store.K8s{}, values)
			case "request-redirect", "request-redirect-code":
				last = hostRedirect.NewAnnotation(s.name).Process(store.K8s{}, values)
			case "ssl-redirect", "ssl-redirect-code", "ssl-redirect-port":
				last = sslRedirect.NewAnnotation(s.name).Process(store.K8s{}, values)
			case "cors-enable", "cors-allow-headers":
				last = cors.NewAnnotation(s.name).Process(store.K8s{}, values)
			default:
				last = rateLimit.NewAnnotation(s.name).Process(store.K8s{}, values)
			}
		}
		return last
	}

	tests := []struct {
		name  string
		steps []step
		valid bool
	}{
		{"capture", []step{{"request-capture", "req.hdr(Host)"}}, true},
		{"capture with converter", []step{{"request-capture", "req.hdr(Host),lower"}}, true},
		{"capture unknown fetch", []step{{"request-capture", "bogus_fetch"}}, false},
		{"capture unknown converter", []step{{"request-capture", "req.hdr(Host),bogus"}}, false},
		{"capture response fetch", []step{{"request-capture", "res.hdr(Host)"}}, false},
		{"capture length", []step{{"request-capture", "req.hdr(Host)"}, {"request-capture-len", "64"}}, true},
		{"capture zero length", []step{{"request-capture", "req.hdr(Host)"}, {"request-capture-len", "0"}}, false},
		{"capture negative length", []step{{"request-capture", "req.hdr(Host)"}, {"request-capture-len", "-1"}}, false},

		{"request header", []step{{"request-set-header", "X-A plain"}}, true},
		{"request header expression", []step{{"request-set-header", "X-A %[req.hdr(a)]"}}, true},
		{"request header unknown fetch", []step{{"request-set-header", "X-A %[bogus]"}}, false},
		{"request header unknown alias", []step{{"request-set-header", "X-A 100%Z"}}, false},
		{"request header response fetch", []step{{"request-set-header", "X-A %[res.hdr(a)]"}}, false},
		{"response header expression", []step{{"response-set-header", "X-A %[res.hdr(a)]"}}, true},
		{"response header request fetch", []step{{"response-set-header", "X-A %[req.hdr(a)]"}}, false},

		{"host", []step{{"set-host", "example.com"}}, true},
		{"host expression", []step{{"set-host", "%[req.hdr(x-host)]"}}, true},
		{"host unknown fetch", []step{{"set-host", "%[bogus]"}}, false},

		{"redirect", []step{{"request-redirect", "example.com"}, {"request-redirect-code", "308"}}, true},
		{"redirect unknown alias", []step{{"request-redirect", "exa%mple.com"}}, false},
		{"redirect unknown fetch", []step{{"request-redirect", "https://%[bogus]"}}, false},
		{"redirect code", []step{{"request-redirect", "example.com"}, {"request-redirect-code", "999"}}, false},
		{"ssl redirect code", []step{{"ssl-redirect", "true"}, {"ssl-redirect-code", "301"}}, true},
		{"ssl redirect bad code", []step{{"ssl-redirect", "true"}, {"ssl-redirect-code", "300"}}, false},
		{"ssl redirect port", []step{{"ssl-redirect", "true"}, {"ssl-redirect-port", "8443"}}, true},
		{"ssl redirect bad port", []step{{"ssl-redirect", "true"}, {"ssl-redirect-port", "0"}}, false},

		{"cors headers", []step{{"cors-enable", "true"}, {"cors-allow-headers", "X-A, X-B"}}, true},
		{"cors headers alias", []step{{"cors-enable", "true"}, {"cors-allow-headers", "X-A, X-%Z"}}, false},

		{"rate limit", []step{{"rate-limit-requests", "10"}, {"rate-limit-period", "10s"}, {"rate-limit-size", "100k"}, {"rate-limit-status-code", "429"}}, true},
		{"rate limit negative", []step{{"rate-limit-requests", "-1"}}, false},
		{"rate limit negative period", []step{{"rate-limit-requests", "10"}, {"rate-limit-period", "-5s"}}, false},
		{"rate limit long period", []step{{"rate-limit-requests", "10"}, {"rate-limit-period", "99999999999s"}}, false},
		{"rate limit empty table", []step{{"rate-limit-requests", "10"}, {"rate-limit-size", "0"}}, false},
		{"rate limit status", []step{{"rate-limit-requests", "10"}, {"rate-limit-status-code", "418"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := process(tt.steps...)
			if tt.valid {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
		})
	}
}
