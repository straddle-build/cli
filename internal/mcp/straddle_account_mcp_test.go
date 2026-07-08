// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package mcp

import (
	"path/filepath"
	"testing"

	"github.com/straddle-build/cli/internal/straddleacct"
)

func TestApplyMCPAccount(t *testing.T) {
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
	if err := straddleacct.SaveContext(straddleacct.Context{
		IntegrationType: straddleacct.TypeSaaS, CurrentAccount: "acct_x",
	}); err != nil {
		t.Fatal(err)
	}

	// saas charge create -> header injected
	h, err := applyMCPAccount("POST", "/v1/charges", nil)
	if err != nil {
		t.Fatalf("charge create: %v", err)
	}
	if h[straddleacct.Header] != "acct_x" {
		t.Errorf("charge create header = %q, want acct_x", h[straddleacct.Header])
	}

	// account-management op -> never gets the header
	h, err = applyMCPAccount("POST", "/v1/accounts", nil)
	if err != nil {
		t.Fatalf("accounts create: %v", err)
	}
	if _, ok := h[straddleacct.Header]; ok {
		t.Errorf("accounts create unexpectedly got the header: %v", h)
	}

	// must not mutate a shared HeaderOverrides map
	shared := map[string]string{"X-Api-Version": "2026-01-01"}
	h, err = applyMCPAccount("POST", "/v1/charges", shared)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := shared[straddleacct.Header]; ok {
		t.Error("applyMCPAccount mutated the shared input map")
	}
	if h[straddleacct.Header] != "acct_x" || h["X-Api-Version"] != "2026-01-01" {
		t.Errorf("merged headers wrong: %v", h)
	}

	// saas with no account -> charge create is a tool error
	if err := straddleacct.SaveContext(straddleacct.Context{IntegrationType: straddleacct.TypeSaaS}); err != nil {
		t.Fatal(err)
	}
	if _, err := applyMCPAccount("POST", "/v1/charges", nil); err == nil {
		t.Error("charge create with no account should error")
	}
}
