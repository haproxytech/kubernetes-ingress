package utils

import (
	"strings"
	"testing"

	"github.com/jessevdk/go-flags"
)

func TestOSArgs_NamespaceLabelSelectorActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args OSArgs
		want bool
	}{
		{name: "unset", args: OSArgs{}, want: false},
		{name: "selector only", args: OSArgs{NamespaceLabelSelector: "watch=true"}, want: true},
		{name: "whitespace selector is not active", args: OSArgs{NamespaceLabelSelector: "  "}, want: false},
		{
			name: "whitelist wins",
			args: OSArgs{NamespaceLabelSelector: "watch=true", NamespaceWhitelist: []string{"app"}},
			want: false,
		},
		{
			name: "blacklist wins",
			args: OSArgs{NamespaceLabelSelector: "watch=true", NamespaceBlacklist: []string{"kube-system"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.args.NamespaceLabelSelectorActive(); got != tt.want {
				t.Errorf("NamespaceLabelSelectorActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOSArgs_ValidateNamespaceLabelSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    OSArgs
		wantErr bool
	}{
		{name: "empty is ok", args: OSArgs{}},
		{name: "equality selector", args: OSArgs{NamespaceLabelSelector: "watch=true"}},
		{name: "set-based selector", args: OSArgs{NamespaceLabelSelector: "env in (prod,staging)"}},
		{name: "invalid selector", args: OSArgs{NamespaceLabelSelector: "watch==="}, wantErr: true},
		{name: "whitespace only", args: OSArgs{NamespaceLabelSelector: "   "}, wantErr: true},
		{
			name: "invalid ignored when whitelist set",
			args: OSArgs{NamespaceLabelSelector: "watch===", NamespaceWhitelist: []string{"app"}},
		},
		{
			name: "invalid ignored when blacklist set",
			args: OSArgs{NamespaceLabelSelector: "   ", NamespaceBlacklist: []string{"kube-system"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.args.ValidateNamespaceLabelSelector()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNamespaceLabelSelector() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr && !strings.Contains(err.Error(), "--namespace-label-selector") {
				t.Errorf("error %q should name --namespace-label-selector", err)
			}
		})
	}
}

func TestOSArgs_NamespaceLabelSelectorCanonicalAndIgnored(t *testing.T) {
	t.Parallel()

	active := OSArgs{NamespaceLabelSelector: "  watch=true  "}
	if got := active.NamespaceLabelSelectorCanonical(); got != "watch=true" {
		t.Errorf("Canonical() = %q, want watch=true", got)
	}
	if active.NamespaceLabelSelectorIgnored() {
		t.Fatal("selector-only must not be reported as ignored")
	}

	ignored := OSArgs{NamespaceLabelSelector: "watch=true", NamespaceWhitelist: []string{"app"}}
	if ignored.NamespaceLabelSelectorCanonical() != "" {
		t.Errorf("Canonical() with whitelist must be empty")
	}
	if !ignored.NamespaceLabelSelectorIgnored() {
		t.Fatal("whitelist + selector must be ignored")
	}
}

func TestOSArgs_ParseNamespaceLabelSelectorFlag(t *testing.T) {
	t.Parallel()

	var args OSArgs
	parser := flags.NewParser(&args, flags.IgnoreUnknown)
	_, err := parser.ParseArgs([]string{"--namespace-label-selector=watch=true"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if !args.NamespaceLabelSelectorActive() {
		t.Fatal("parsed --namespace-label-selector should be active")
	}
	if err := args.ValidateNamespaceLabelSelector(); err != nil {
		t.Fatalf("ValidateNamespaceLabelSelector: %v", err)
	}

	var withWhitelist OSArgs
	parser = flags.NewParser(&withWhitelist, flags.IgnoreUnknown)
	_, err = parser.ParseArgs([]string{
		"--namespace-label-selector=watch=true",
		"--namespace-whitelist=app",
	})
	if err != nil {
		t.Fatalf("ParseArgs whitelist: %v", err)
	}
	if withWhitelist.NamespaceLabelSelectorActive() {
		t.Fatal("whitelist must take precedence over selector")
	}
	if !withWhitelist.NamespaceLabelSelectorIgnored() {
		t.Fatal("selector present with whitelist must be ignored")
	}
}
