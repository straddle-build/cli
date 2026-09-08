// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// writeFileLoose places path with the given (loose) mode, simulating an
// operator placing a config file out-of-band (touch / cp / scp / a dotfile
// templating tool) before the CLI writes to it. The CLI's own first save
// produces 0600; this helper is the only route to a 0644 precondition.
func writeFileLoose(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("pre-placing loose config: %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod pre-placed config to %o: %v", mode, err)
	}
}

// assertConfigMode0600 fails the test if path's permission bits are not
// exactly 0600 (owner-only), reporting the actual mode for triage.
func assertConfigMode0600(t *testing.T, path, step string) {
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

// TestSetTokenWorldReadableConfigFile pins the CLI-level security
// boundary that a8545d0 raised for other credential-bearing artifacts:
// `auth set-token <token>` and `auth logout` persist credentials (or
// clear them) through Config.save(), which calls os.WriteFile with 0600.
// WriteFile's perm argument is only applied on file *creation*; on
// truncation of a pre-existing file the mode is silently retained, so a
// 0644 config file placed out-of-band at a custom path (via STRADDLE_CONFIG
// or --config) inside a world-traversable directory stays 0644 after
// set-token, leaking the saved API token to every local user.
//
// Pre-fix this test fails: save() left the pre-existing 0644 mode in place.
// Post-fix save() Chmods the file to 0600 after WriteFile, so both the
// credential-write (set-token) and credential-clear (logout) paths tighten
// the file regardless of how it was originally permissioned.
func TestSetTokenWorldReadableConfigFile(t *testing.T) {
	t.Setenv("STRADDLE_API_KEY", "")
	t.Setenv("STRADDLE_BASE_URL", "")
	t.Setenv("STRADDLE_CONFIG", "")

	t.Run("set-token tightens pre-existing 0644 file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "team-config.toml")
		// Operator pre-placed the config file out-of-band at a custom path
		// (the only route to a 0644 config — the CLI's own first save is 0600).
		writeFileLoose(t, path, `base_url = "https://sandbox.straddle.com"`+"\n", 0o644)

		cmd := newAuthSetTokenCmd(&rootFlags{configPath: path})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"sk_live_rotated_key"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("auth set-token returned error: %v", err)
		}
		assertConfigMode0600(t, path, "after set-token")

		// The token must actually have been persisted (feature regression
		// guard): the permission hardening must not have broken the write.
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading saved config: %v", err)
		}
		if !bytes.Contains(data, []byte("sk_live_rotated_key")) {
			t.Fatalf("saved config does not contain the API token; content:\n%s", string(data))
		}
	})

	t.Run("logout tightens pre-existing 0644 file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "team-config.toml")
		// Pre-place a placeholder: an operator templated the config (0644),
		// then later runs `auth logout` to clear stale credentials. logout
		// re-writes through the same save() path as set-token.
		writeFileLoose(t, path,
			`base_url = "https://sandbox.straddle.com"`+"\n"+
				`access_token = "sk_live_stale"`+"\n",
			0o644)

		cmd := newAuthLogoutCmd(&rootFlags{configPath: path})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(nil)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("auth logout returned error: %v", err)
		}
		assertConfigMode0600(t, path, "after logout")

		// The credential must actually have been cleared (feature regression
		// guard) — and the cleared content must not remain world-readable.
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading cleared config: %v", err)
		}
		if bytes.Contains(data, []byte("sk_live_stale")) {
			t.Fatalf("cleared config still contains the stale API token; content:\n%s", string(data))
		}
	})

	t.Run("set-token then re-loosened then logout tightens again", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "team-config.toml")
		writeFileLoose(t, path, `base_url = "https://sandbox.straddle.com"`+"\n", 0o644)

		setCmd := newAuthSetTokenCmd(&rootFlags{configPath: path})
		setCmd.SetOut(&bytes.Buffer{})
		setCmd.SetErr(&bytes.Buffer{})
		setCmd.SetArgs([]string{"sk_live_key"})
		if err := setCmd.Execute(); err != nil {
			t.Fatalf("auth set-token: %v", err)
		}
		assertConfigMode0600(t, path, "after set-token")

		// Out-of-band re-loosen between commands (e.g. a second templating
		// pass). Confirms logout tightens independently of set-token.
		writeFileLoose(t, path, `base_url = "https://sandbox.straddle.com"`+"\n", 0o644)

		outCmd := newAuthLogoutCmd(&rootFlags{configPath: path})
		outCmd.SetOut(&bytes.Buffer{})
		outCmd.SetErr(&bytes.Buffer{})
		if err := outCmd.Execute(); err != nil {
			t.Fatalf("auth logout: %v", err)
		}
		assertConfigMode0600(t, path, "after logout on re-loosened file")
	})

	t.Run("fresh default-path create is 0600 (regression guard)", func(t *testing.T) {
		// The CLI's own first save into a non-existent path must still
		// produce 0600 — the new Chmod must not loosen the default-path
		// behavior, and WriteFile's perm=0600 must remain in effect.
		dir := t.TempDir()
		path := filepath.Join(dir, "nested", "deep", "config.toml")
		cmd := newAuthSetTokenCmd(&rootFlags{configPath: path})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"sk_live_first"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("auth set-token on fresh path: %v", err)
		}
		assertConfigMode0600(t, path, "fresh create")
	})
}

// TestSetTokenJSONOutputStillWorks guards the JSON envelope emitted by
// `auth set-token --json` so the permission-tightening change does not
// regress the structured-output agent contract.
func TestSetTokenJSONOutputStillWorks(t *testing.T) {
	t.Setenv("STRADDLE_API_KEY", "")
	t.Setenv("STRADDLE_BASE_URL", "")
	t.Setenv("STRADDLE_CONFIG", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFileLoose(t, path, "", 0o644)

	var out bytes.Buffer
	cmd := newAuthSetTokenCmd(&rootFlags{configPath: path, asJSON: true})
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"sk_live_json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth set-token --json: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"saved": true`)) {
		t.Fatalf("JSON envelope missing saved:true; output: %s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"config_path"`)) {
		t.Fatalf("JSON envelope missing config_path; output: %s", out.String())
	}
	assertConfigMode0600(t, path, "after set-token --json")
}
