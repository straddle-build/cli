// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package straddleacct

import (
	"path/filepath"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))

	want := Context{IntegrationType: TypeSaaS, CurrentAccount: "acct_123"}
	if err := SaveContext(want); err != nil {
		t.Fatalf("SaveContext: %v", err)
	}
	got, err := LoadContext()
	if err != nil {
		t.Fatalf("LoadContext: %v", err)
	}
	if got != want {
		t.Errorf("round-trip = %+v, want %+v", got, want)
	}
}

func TestLoadContextAbsentFile(t *testing.T) {
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "does-not-exist.toml"))
	got, err := LoadContext()
	if err != nil {
		t.Fatalf("LoadContext on absent file should not error: %v", err)
	}
	if got != (Context{}) {
		t.Errorf("absent file = %+v, want empty Context", got)
	}
}

func TestSaveContextRejectsInvalidType(t *testing.T) {
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
	if err := SaveContext(Context{IntegrationType: "enterprise"}); err == nil {
		t.Fatal("SaveContext should reject an invalid integration type")
	}
}

func TestSaveContextAllowsEmptyType(t *testing.T) {
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
	// Setting only the account before declaring a type is valid (unset type).
	if err := SaveContext(Context{CurrentAccount: "acct_x"}); err != nil {
		t.Fatalf("SaveContext with empty type should be allowed: %v", err)
	}
}

func TestValidIntegrationType(t *testing.T) {
	for _, ok := range []string{TypeAccount, TypeSaaS, TypeMarketplace} {
		if !ValidIntegrationType(ok) {
			t.Errorf("ValidIntegrationType(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "enterprise", "Account", "SAAS"} {
		if ValidIntegrationType(bad) {
			t.Errorf("ValidIntegrationType(%q) = true, want false", bad)
		}
	}
}
