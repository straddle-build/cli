// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestVersion_DefaultsToDev verifies local test binaries with no ldflags
// stamp do not report Go's buildinfo placeholder "(devel)" to users.
func TestVersion_DefaultsToDev(t *testing.T) {
	old := version
	version = ""
	t.Cleanup(func() { version = old })

	if got := Version(); got != "dev" {
		t.Fatalf("Version() = %q, want %q", got, "dev")
	}
}

// TestVersion_UsesInjectedValue verifies release builds prefer the ldflags
// stamp over buildinfo fallback, which is what GoReleaser sets for archives.
func TestVersion_UsesInjectedValue(t *testing.T) {
	old := version
	version = "v1.2.3-test"
	t.Cleanup(func() { version = old })

	if got := Version(); got != "v1.2.3-test" {
		t.Fatalf("Version() = %q, want %q", got, "v1.2.3-test")
	}
}
