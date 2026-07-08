// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/straddle-build/cli/internal/straddleacct"
)

func TestSetupSetsIntegrationType(t *testing.T) {
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))

	cmd := newSetupCmd(&rootFlags{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--type", "saas"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("setup --type saas: %v", err)
	}
	ctx, _ := straddleacct.LoadContext()
	if ctx.IntegrationType != straddleacct.TypeSaaS {
		t.Errorf("integration type = %q, want saas", ctx.IntegrationType)
	}
}

func TestSetupRejectsInvalidType(t *testing.T) {
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
	cmd := newSetupCmd(&rootFlags{})
	cmd.SetArgs([]string{"--type", "enterprise"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("setup --type enterprise should error")
	}
}

func TestSetupCommandsRegistered(t *testing.T) {
	root := RootCmd()
	found := map[string]bool{"use-account": false, "setup": false}
	for _, c := range root.Commands() {
		if _, ok := found[c.Name()]; ok {
			found[c.Name()] = true
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("RootCmd is missing the %q command", name)
		}
	}
}

func TestUseAccountSetClearShow(t *testing.T) {
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))

	// set
	set := newUseAccountCmd(&rootFlags{})
	set.SetArgs([]string{"acct_xyz"})
	var b bytes.Buffer
	set.SetOut(&b)
	if err := set.Execute(); err != nil {
		t.Fatalf("use-account set: %v", err)
	}
	if ctx, _ := straddleacct.LoadContext(); ctx.CurrentAccount != "acct_xyz" {
		t.Errorf("current account = %q, want acct_xyz", ctx.CurrentAccount)
	}

	// clear
	clr := newUseAccountCmd(&rootFlags{})
	clr.SetArgs([]string{"--clear"})
	clr.SetOut(&bytes.Buffer{})
	if err := clr.Execute(); err != nil {
		t.Fatalf("use-account --clear: %v", err)
	}
	if ctx, _ := straddleacct.LoadContext(); ctx.CurrentAccount != "" {
		t.Errorf("current account after clear = %q, want empty", ctx.CurrentAccount)
	}
}
