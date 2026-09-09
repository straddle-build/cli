// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Regression coverage for the profile-supplied `--account` honoring contract:
// internal/cli/root.go's PersistentPreRunE applies a saved profile before
// calling resolveStraddleAccount "so a profile-set --account is honored", and
// internal/straddleacct/policy.go's Resolve promises "The per-call flag
// overrides the sticky account whenever the flag was supplied." A profile
// overlays the flag via flag.Value.Set, which (unlike FlagSet.Set) does not
// mark the flag Changed; resolveStraddleAccount feeds cmd.Flags().Changed(
// "account") into Resolve as the flagChanged signal, so a profile-supplied
// account was silently dropped (Require aborted; Allow misattributed to the
// sticky). These tests pin the fixed behavior.
package cli

import (
	"path/filepath"
	"testing"

	"github.com/straddle-build/straddle-cli/internal/config"
	"github.com/straddle-build/straddle-cli/internal/straddleacct"
)

func TestProfileAccountHonoredByResolve(t *testing.T) {
	// Case 1: Require decision, no sticky account. A profile supplying
	// --account must be honored so a SaaS charge create resolves to the
	// profile account rather than aborting with "an account id is required".
	// Before the fix, Changed("account") stayed false after profile
	// application, Resolve fell back to the empty sticky, and the command
	// hard-errored despite the value being present on the bound field.
	t.Run("require_no_sticky_uses_profile_account", func(t *testing.T) {
		t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
		if err := straddleacct.SaveContext(straddleacct.Context{IntegrationType: straddleacct.TypeSaaS}); err != nil {
			t.Fatal(err)
		}
		cmd, f := newAccountTestCmd("/v1/charges", "POST")
		prof := &Profile{Name: "p", Values: map[string]string{"account": "acct_profile"}}

		if err := ApplyProfileToFlags(cmd, prof); err != nil {
			t.Fatalf("ApplyProfileToFlags: %v", err)
		}
		if !cmd.Flags().Changed("account") {
			t.Fatal("profile-set --account must mark the flag Changed so Resolve treats it as supplied")
		}
		if f.straddleAccount != "acct_profile" {
			t.Fatalf("bound account = %q, want acct_profile", f.straddleAccount)
		}
		if err := resolveStraddleAccount(cmd, f, nil); err != nil {
			t.Fatalf("Require: profile-set --account dropped (Changed(account)=false): %v", err)
		}
		if f.straddleAccountResolved != "acct_profile" {
			t.Fatalf("Require: resolved = %q, want acct_profile", f.straddleAccountResolved)
		}
	})

	// Case 2: Allow decision with a differing sticky account. The profile
	// account must override the sticky so the Straddle-Account-Id header is
	// attributed to acct_profile, not the sticky acct_sticky. Before the fix,
	// Resolve returned the sticky and applyStraddleAccount injected it onto
	// the wire — silent misattribution to the wrong embedded account.
	t.Run("allow_with_sticky_profile_overrides_sticky", func(t *testing.T) {
		t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
		if err := straddleacct.SaveContext(straddleacct.Context{
			IntegrationType: straddleacct.TypeSaaS,
			CurrentAccount:  "acct_sticky",
		}); err != nil {
			t.Fatal(err)
		}
		cmd, f := newAccountTestCmd("/v1/charges/{id}", "GET")
		prof := &Profile{Name: "p", Values: map[string]string{"account": "acct_profile"}}

		if err := ApplyProfileToFlags(cmd, prof); err != nil {
			t.Fatalf("ApplyProfileToFlags: %v", err)
		}
		if err := resolveStraddleAccount(cmd, f, nil); err != nil {
			t.Fatalf("Allow: resolveStraddleAccount: %v", err)
		}
		if f.straddleAccountResolved != "acct_profile" {
			t.Fatalf("Allow: misattribution resolved=%q, want acct_profile (sticky leaked)", f.straddleAccountResolved)
		}
		cfg := &config.Config{Headers: map[string]string{}}
		applyStraddleAccount(cfg, f)
		if got := cfg.Headers[straddleAccountHeader]; got != "acct_profile" {
			t.Fatalf("Allow: header = %q, want acct_profile (no sticky leak)", got)
		}
	})

	// Case 3: The fix is scoped to `account`. A profile-supplied value for any
	// other Changed-gated flag must NOT be marked Changed, so existing
	// Changed-gated consumers keep their current behavior — most notably the
	// --agent default block in root.go (json/compact/no-input/yes/no-color),
	// which intentionally overrides profile values because Changed stays false.
	// This locks in the narrow Option A fix and guards against a broad sweep
	// (Option B) that would change the documented agent-mode workflow.
	t.Run("scoped_to_account_other_flags_not_marked_changed", func(t *testing.T) {
		var jsonOut bool
		cmd, _ := newAccountTestCmd("/v1/charges", "POST")
		cmd.Flags().BoolVar(&jsonOut, "json", false, "")
		prof := &Profile{Name: "p", Values: map[string]string{
			"account": "acct_profile",
			"json":    "true",
		}}

		if err := ApplyProfileToFlags(cmd, prof); err != nil {
			t.Fatalf("ApplyProfileToFlags: %v", err)
		}
		if !cmd.Flags().Changed("account") {
			t.Error("account must be marked Changed by the profile fix")
		}
		if cmd.Flags().Changed("json") {
			t.Error("json must NOT be marked Changed (fix scoped to account; --agent block unchanged)")
		}
		if !jsonOut {
			t.Error("bound json value should still be populated from the profile via Value.Set")
		}
	})
}
