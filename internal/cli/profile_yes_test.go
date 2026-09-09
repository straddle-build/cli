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
