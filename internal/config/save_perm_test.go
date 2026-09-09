// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFileLoose creates path with the given (loose) mode, simulating an
// operator placing a config file out-of-band (touch / cp / scp / a dotfile
// templating tool) before the CLI ever writes to it. The CLI itself never
// produces a file looser than 0600 — this helper is the only route to that
// precondition.
func writeFileLoose(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("pre-placing loose config: %v", err)
	}
	// WriteFile applies mode&^umask on creation; Chmod sets it exactly so the
	// test's precondition is deterministic regardless of the runner's umask.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod pre-placed config to %o: %v", mode, err)
	}
}

// assertMode0600 fails the test if path's permission bits are not exactly
// 0600 (owner-only). It reports the actual mode for triage.
func assertMode0600(t *testing.T, path, step string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s: stat config: %v", step, err)
	}
	got := info.Mode().Perm()
	if got != 0o600 {
		t.Fatalf("%s: config perms = %o (world-readable=%v, group-readable=%v), want 0600",
			step, got, got&0o004 != 0, got&0o040 != 0)
	}
}

// TestSave_TightensPreExistingLooseFile is the unit-level pin for the
// security hardening added after a8545d0: os.WriteFile's perm argument is
// silently dropped when the target file already exists (it truncates
// instead of creating), so a pre-existing 0644 config file placed
// out-of-band at the config path keeps those permissions after a save.
// The config file carries the API token, so save() must Chmod it to 0600
// explicitly. Mirrors the Chmod defense for the SQLite store in store.go.
func TestSave_TightensPreExistingLooseFile(t *testing.T) {
	cases := []struct {
		name string
		mode os.FileMode
		op   func(*Config) error
	}{
		{
			name: "SaveTokens on 0644 file",
			mode: 0o644,
			op: func(c *Config) error {
				return c.SaveTokens("client_id", "secret", "sk_live_test_token", "rt", time.Unix(1e9, 0))
			},
		},
		{
			name: "ClearTokens on 0644 file",
			mode: 0o644,
			op: func(c *Config) error {
				c.AccessToken = "sk_live_pretoken"
				c.StraddleApiKey = "sk_live_prekey"
				return c.ClearTokens()
			},
		},
		{
			name: "SaveTokens on 0666 file",
			mode: 0o666,
			op: func(c *Config) error {
				return c.SaveTokens("", "", "sk_live_test_token", "", time.Time{})
			},
		},
		{
			name: "SaveTokens on 0640 file (group-readable)",
			mode: 0o640,
			op: func(c *Config) error {
				return c.SaveTokens("", "", "sk_live_test_token", "", time.Time{})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			writeFileLoose(t, path, `base_url = "https://sandbox.straddle.com"`+"\n", tc.mode)

			cfg := &Config{Path: path}
			if err := tc.op(cfg); err != nil {
				t.Fatalf("op returned error: %v", err)
			}
			assertMode0600(t, path, "after op")
		})
	}
}

// TestSave_FreshCreateIs0600 guards the default happy path (regression
// guard): when the config file does not already exist, save() creates it
// with 0600 via WriteFile's perm argument as before, and the new Chmod is
// a no-op on the same mode. The first-save path must not regress.
func TestSave_FreshCreateIs0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.toml")
	cfg := &Config{Path: path}
	if err := cfg.SaveTokens("", "", "sk_live_test_token", "", time.Time{}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	assertMode0600(t, path, "fresh create")
}

// TestSave_Already0600Stays0600 guards the idempotent re-save path: a file
// already at 0600 (the CLI's own first-save output) stays 0600 after a
// subsequent save. This is the steady-state for the default onboarding
// flow and confirms the Chmod does not loosen a correctly-permissioned
// file.
func TestSave_Already0600Stays0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFileLoose(t, path, "", 0o600)
	cfg := &Config{Path: path}
	if err := cfg.SaveTokens("", "", "sk_live_test_token", "", time.Time{}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	assertMode0600(t, path, "re-save")
}

func TestSave_DoesNotExposeRotatedTokenToExistingReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFileLoose(t, path, "api_key = 'old-secret'\n", 0o644)

	reader, err := os.Open(path)
	if err != nil {
		t.Fatalf("open existing config: %v", err)
	}
	defer reader.Close()

	cfg := &Config{Path: path}
	if err := cfg.SaveTokens("", "", "new-secret", "", time.Time{}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	if _, err := reader.Seek(0, 0); err != nil {
		t.Fatalf("rewind existing reader: %v", err)
	}
	oldView, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read existing reader: %v", err)
	}
	if strings.Contains(string(oldView), "new-secret") {
		t.Fatalf("existing reader observed rotated secret: %q", oldView)
	}
	if !strings.Contains(string(oldView), "old-secret") {
		t.Fatalf("existing reader no longer sees its original file: %q", oldView)
	}

	newView, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replacement config: %v", err)
	}
	if !strings.Contains(string(newView), "new-secret") {
		t.Fatalf("replacement config omitted rotated secret: %q", newView)
	}
	assertMode0600(t, path, "after atomic replacement")
}

func TestSave_PreservesExistingConfigSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.toml")
	path := filepath.Join(dir, "config.toml")
	writeFileLoose(t, target, "api_key = 'old-secret'\n", 0o644)
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("create config symlink: %v", err)
	}

	cfg := &Config{Path: path}
	if err := cfg.SaveTokens("", "", "new-secret", "", time.Time{}); err != nil {
		t.Fatalf("SaveTokens through symlink: %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("stat config symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config path was replaced instead of preserving symlink: mode=%v", info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config through symlink: %v", err)
	}
	if !strings.Contains(string(data), "new-secret") {
		t.Fatalf("symlink target omitted rotated secret: %q", data)
	}
	assertMode0600(t, target, "symlink target after replacement")
}

// TestSave_PersistsTokenContent confirms the permission hardening did not
// break the primary feature (writing the API token) or the credential-
// clear feature: the token reaches disk on SaveTokens and is wiped on
// ClearTokens.
func TestSave_PersistsTokenContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	// Pre-place a loose file to exercise the bug-relevant path.
	writeFileLoose(t, path, "", 0o644)

	cfg := &Config{Path: path}
	if err := cfg.SaveTokens("cid", "sec", "sk_live_test_token", "rt", time.Time{}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	assertMode0600(t, path, "after SaveTokens")

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after SaveTokens: %v", err)
	}
	if loaded.AccessToken != "sk_live_test_token" {
		t.Fatalf("AccessToken = %q, want sk_live_test_token", loaded.AccessToken)
	}
	if loaded.ClientID != "cid" {
		t.Fatalf("ClientID = %q, want cid", loaded.ClientID)
	}

	// ClearTokens must wipe the credential AND keep the file 0600.
	if err := loaded.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}
	assertMode0600(t, path, "after ClearTokens")
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after ClearTokens: %v", err)
	}
	if reloaded.AccessToken != "" {
		t.Fatalf("AccessToken after ClearTokens = %q, want empty", reloaded.AccessToken)
	}
	if reloaded.ClientID != "" {
		t.Fatalf("ClientID after ClearTokens = %q, want empty", reloaded.ClientID)
	}
}

// TestSave_TightensOnRepeatedLooseResets proves each save() caller
// independently tightens the file: even if an external actor re-loosens
// the file to 0644 between commands, the next save() restores 0600. This
// guards both SaveTokens and ClearTokens against a partial fix that only
// chmods on one path.
func TestSave_TightensOnRepeatedLooseResets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Path: path}

	// First save creates the file 0600.
	if err := cfg.SaveTokens("", "", "sk_live_one", "", time.Time{}); err != nil {
		t.Fatalf("SaveTokens #1: %v", err)
	}
	assertMode0600(t, path, "after first save")

	// Out-of-band re-loosen (e.g. another templating pass), then SaveTokens
	// again must re-tighten.
	writeFileLoose(t, path, "", 0o644)
	if err := cfg.SaveTokens("", "", "sk_live_two", "", time.Time{}); err != nil {
		t.Fatalf("SaveTokens #2: %v", err)
	}
	assertMode0600(t, path, "after second save on re-loosened file")

	// Out-of-band re-loosen, then ClearTokens must re-tighten too.
	writeFileLoose(t, path, "", 0o644)
	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}
	assertMode0600(t, path, "after ClearTokens on re-loosened file")
}
