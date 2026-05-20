// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Tests for the per-call Straddle-Account-Id resolution and injection.
package cli

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"straddle-pp-cli/internal/config"
	"straddle-pp-cli/internal/straddleacct"
)

func TestApplyStraddleAccount(t *testing.T) {
	cases := []struct {
		name         string
		resolved     string
		startHeaders map[string]string
		wantHeader   string // value of Straddle-Account-Id; "" means absent
	}{
		{"unset adds no header", "", nil, ""},
		{"sets header on nil map", "acct_123", nil, "acct_123"},
		{"sets header on existing map", "acct_456", map[string]string{"X-Other": "y"}, "acct_456"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Headers: tc.startHeaders}
			f := &rootFlags{straddleAccountResolved: tc.resolved}
			applyStraddleAccount(cfg, f)

			if got := cfg.Headers[straddleAccountHeader]; got != tc.wantHeader {
				t.Errorf("%s = %q, want %q", straddleAccountHeader, got, tc.wantHeader)
			}
			if tc.startHeaders != nil && cfg.Headers["X-Other"] != "y" {
				t.Errorf("pre-existing header X-Other was dropped")
			}
		})
	}
}

func newAccountTestCmd(path, method string) (*cobra.Command, *rootFlags) {
	f := &rootFlags{}
	cmd := &cobra.Command{
		Use:         "x",
		Annotations: map[string]string{"pp:path": path, "pp:method": method},
	}
	cmd.Flags().StringVar(&f.straddleAccount, "account", "", "")
	return cmd, f
}

func TestResolveStraddleAccount(t *testing.T) {
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))

	// SaaS with a sticky account: a charge create resolves to the sticky value.
	if err := straddleacct.SaveContext(straddleacct.Context{
		IntegrationType: straddleacct.TypeSaaS, CurrentAccount: "acct_sticky",
	}); err != nil {
		t.Fatal(err)
	}
	cmd, f := newAccountTestCmd("/v1/charges", "POST")
	if err := resolveStraddleAccount(cmd, f); err != nil {
		t.Fatalf("charge create: %v", err)
	}
	if f.straddleAccountResolved != "acct_sticky" {
		t.Errorf("charge create resolved = %q, want acct_sticky", f.straddleAccountResolved)
	}

	// The sticky value must NOT leak onto an account-management call.
	cmd, f = newAccountTestCmd("/v1/accounts", "POST")
	if err := resolveStraddleAccount(cmd, f); err != nil {
		t.Fatalf("accounts create: %v", err)
	}
	if f.straddleAccountResolved != "" {
		t.Errorf("sticky leaked onto accounts create: %q", f.straddleAccountResolved)
	}

	// SaaS with no account set: a charge create is a hard error (no misattribution).
	if err := straddleacct.SaveContext(straddleacct.Context{IntegrationType: straddleacct.TypeSaaS}); err != nil {
		t.Fatal(err)
	}
	cmd, f = newAccountTestCmd("/v1/charges", "POST")
	if err := resolveStraddleAccount(cmd, f); err == nil {
		t.Error("charge create without an account should error under saas")
	}
}
