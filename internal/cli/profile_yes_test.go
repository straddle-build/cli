// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestProfileYesCannotBeCapturedOrApplied(t *testing.T) {
	for _, reservedFlag := range []string{"--yes", "--agent"} {
		t.Run("profile save excludes "+reservedFlag, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			root := RootCmd()
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs([]string{"profile", "save", "p", reservedFlag})
			if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "no non-default flags") {
				t.Fatalf("profile save error = %v, want no non-default flags", err)
			}
			profile, err := GetProfile("p")
			if err != nil {
				t.Fatalf("GetProfile: %v", err)
			}
			if profile != nil {
				t.Fatalf("profile was created from confirmation flag: %#v", profile.Values)
			}
		})
	}

	t.Run("profile yes does not bypass delete confirmation", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
			"p": {Name: "p", Values: map[string]string{"yes": "true"}},
		}}); err != nil {
			t.Fatalf("saveProfileStore: %v", err)
		}

		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"--profile", "p", "profile", "delete", "p"})
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "confirmation required") {
			t.Fatalf("delete error = %v, want confirmation required", err)
		}
		if _, err := os.Stat(func() string {
			path, pathErr := profileStorePath()
			if pathErr != nil {
				t.Fatal(pathErr)
			}
			return path
		}()); err != nil {
			t.Fatalf("profile store was removed: %v", err)
		}
		if !strings.Contains(stderr.String(), "without --yes") {
			t.Fatalf("stderr = %q, want confirmation guidance", stderr.String())
		}
	})

	t.Run("profile agent does not bypass delete confirmation", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
			"p": {Name: "p", Values: map[string]string{"agent": "true"}},
		}}); err != nil {
			t.Fatalf("saveProfileStore: %v", err)
		}

		root := RootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"--profile", "p", "profile", "delete", "p"})
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "confirmation required") {
			t.Fatalf("delete error = %v, want confirmation required", err)
		}
		profile, getErr := GetProfile("p")
		if getErr != nil {
			t.Fatalf("GetProfile: %v", getErr)
		}
		if profile == nil {
			t.Fatal("profile was deleted without explicit confirmation")
		}
	})

	t.Run("explicit yes still confirms delete", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
			"p": {Name: "p", Values: map[string]string{"yes": "true"}},
		}}); err != nil {
			t.Fatalf("saveProfileStore: %v", err)
		}
		root := RootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"--profile", "p", "profile", "delete", "p", "--yes"})
		if err := root.Execute(); err != nil {
			t.Fatalf("delete with explicit --yes: %v", err)
		}
		profile, err := GetProfile("p")
		if err != nil {
			t.Fatalf("GetProfile: %v", err)
		}
		if profile != nil {
			t.Fatalf("profile still exists after explicit confirmation: %#v", profile)
		}
	})
}

// TestProfileDeleteConfirmationProvenance verifies the confirmation gate
// checks flag provenance (Changed) rather than the underlying bool value.
// isReservedProfileFlag already blocks profiles from capturing/applying
// --yes; these tests guard the deeper root cause: even if some overlay path
// writes flags.yes=true without recording provenance (exactly what the old
// flag.Value.Set overlay did), the gate must still refuse.
func TestProfileDeleteConfirmationProvenance(t *testing.T) {
	t.Run("agent flag on command line confirms delete", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
			"p": {Name: "p", Values: map[string]string{"json": "true"}},
		}}); err != nil {
			t.Fatalf("saveProfileStore: %v", err)
		}
		root := RootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"--agent", "profile", "delete", "p"})
		if err := root.Execute(); err != nil {
			t.Fatalf("delete with --agent: %v", err)
		}
		profile, err := GetProfile("p")
		if err != nil {
			t.Fatalf("GetProfile: %v", err)
		}
		if profile != nil {
			t.Fatalf("profile still exists after --agent confirmation: %#v", profile)
		}
	})

	t.Run("injected yes value without Changed is refused", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
			"p": {Name: "p", Values: map[string]string{"json": "true"}},
		}}); err != nil {
			t.Fatalf("saveProfileStore: %v", err)
		}
		root := RootCmd()
		// Simulate an overlay path (profile or otherwise) that writes via
		// flag.Value.Set: the backing bool becomes true but Changed stays
		// false. The old gate (if !flags.yes) would pass; the hardened gate
		// checks Changed and must still refuse.
		yesFlag := root.PersistentFlags().Lookup("yes")
		if yesFlag == nil {
			t.Fatal("could not find --yes flag on root")
		}
		if err := yesFlag.Value.Set("true"); err != nil {
			t.Fatalf("set yes value: %v", err)
		}
		if yesFlag.Changed {
			t.Fatal("precondition: yesFlag.Changed must be false after Value.Set (no provenance)")
		}
		root.SetOut(&bytes.Buffer{})
		var stderr bytes.Buffer
		root.SetErr(&stderr)
		root.SetArgs([]string{"profile", "delete", "p"})
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "confirmation required") {
			t.Fatalf("expected confirmation required, got err=%v", err)
		}
		if !strings.Contains(stderr.String(), "without --yes") {
			t.Fatalf("stderr = %q, want confirmation guidance", stderr.String())
		}
		profile, getErr := GetProfile("p")
		if getErr != nil {
			t.Fatalf("GetProfile: %v", getErr)
		}
		if profile == nil {
			t.Fatal("profile was deleted despite injected yes=true having no provenance")
		}
	})

	t.Run("injected agent value without Changed is refused", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
			"p": {Name: "p", Values: map[string]string{"json": "true"}},
		}}); err != nil {
			t.Fatalf("saveProfileStore: %v", err)
		}
		root := RootCmd()
		// An overlay that sets agent=true via Value.Set would, with the old
		// gate, flip flags.yes through the --agent block. The hardened gate
		// checks agent.Changed so an unprovenanced agent value cannot confirm.
		agentFlag := root.PersistentFlags().Lookup("agent")
		if agentFlag == nil {
			t.Fatal("could not find --agent flag on root")
		}
		if err := agentFlag.Value.Set("true"); err != nil {
			t.Fatalf("set agent value: %v", err)
		}
		if agentFlag.Changed {
			t.Fatal("precondition: agentFlag.Changed must be false after Value.Set (no provenance)")
		}
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"profile", "delete", "p"})
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "confirmation required") {
			t.Fatalf("expected confirmation required, got err=%v", err)
		}
		profile, getErr := GetProfile("p")
		if getErr != nil {
			t.Fatalf("GetProfile: %v", getErr)
		}
		if profile == nil {
			t.Fatal("profile was deleted despite injected agent=true having no provenance")
		}
	})
}
