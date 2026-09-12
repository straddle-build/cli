// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"testing"

	"github.com/straddle-build/straddle-cli/internal/config"
)

// TestCacheKey_TemplateVarsPartition verifies that two clients whose configs
// differ only in a non-environment template variable used in BaseURL produce
// distinct cache keys. On the original hard-coded {"environment"} allowlist,
// both keys would be identical — the variable that buildURL resolves would be
// invisible to cacheKey, causing cache collisions and stale responses from the
// wrong resolved URL within the TTL.
func TestCacheKey_TemplateVarsPartition(t *testing.T) {
	t.Parallel()

	const baseURL = "https://{shop}.api.example.com"

	alice := &Client{
		BaseURL: baseURL,
		Config: &config.Config{
			TemplateVars: map[string]string{"environment": "sandbox", "shop": "alice"},
		},
	}
	bob := &Client{
		BaseURL: baseURL,
		Config: &config.Config{
			TemplateVars: map[string]string{"environment": "sandbox", "shop": "bob"},
		},
	}

	keyAlice := alice.cacheKey("/v1/accounts", nil, nil)
	keyBob := bob.cacheKey("/v1/accounts", nil, nil)

	if keyAlice == keyBob {
		t.Fatalf("cache key collision: alice and bob share key %q — shop should partition the cache", keyAlice)
	}
}

// TestCacheKey_DefaultSpecUnchangedByFix pins the cache key for the shipped
// default spec (BaseURL "https://{environment}.straddle.com", environment
// "sandbox", no config path, no auth, no headers). An unrelated future change
// to cacheKey must not silently rotate on-disk caches for default-spec users.
func TestCacheKey_DefaultSpecUnchangedByFix(t *testing.T) {
	t.Parallel()

	c := &Client{
		BaseURL: "https://{environment}.straddle.com",
		Config: &config.Config{
			TemplateVars: map[string]string{"environment": "sandbox"},
		},
	}

	const want = "abb55fc83e895710"
	if got := c.cacheKey("/v1/accounts", nil, nil); got != want {
		t.Fatalf("cacheKey() = %q, want %q (default-spec key changed)", got, want)
	}
}

// TestCacheKey_FlatBaseURLPreservesEnvironment pins the cache key for a flat
// BaseURL with environment set — the shape produced by the documented
// STRADDLE_BASE_URL env-var override — so the fix does not rotate caches for
// override users (verify mode, mock servers).
func TestCacheKey_FlatBaseURLPreservesEnvironment(t *testing.T) {
	t.Parallel()

	c := &Client{
		BaseURL: "https://staging.example.com",
		Config: &config.Config{
			TemplateVars: map[string]string{"environment": "sandbox"},
		},
	}

	const want = "03911574544a772b"
	if got := c.cacheKey("/v1/accounts", nil, nil); got != want {
		t.Fatalf("cacheKey() = %q, want %q (flat-BaseURL key changed)", got, want)
	}
}

// TestCacheKey_UnresolvedPlaceholderDistinctFromResolved verifies that a
// placeholder present in BaseURL but absent from TemplateVars produces a cache
// key distinct from the same placeholder resolved to any concrete value. On
// the original allowlist both keys would be identical, so a second request with
// the variable un-set could receive the first request's cached body instead of
// surfacing buildURL's TemplateVarError.
func TestCacheKey_UnresolvedPlaceholderDistinctFromResolved(t *testing.T) {
	t.Parallel()

	const baseURL = "https://{shop}.api.example.com"

	resolved := &Client{
		BaseURL: baseURL,
		Config: &config.Config{
			TemplateVars: map[string]string{"environment": "sandbox", "shop": "alice"},
		},
	}
	unresolved := &Client{
		BaseURL: baseURL,
		Config: &config.Config{
			TemplateVars: map[string]string{"environment": "sandbox"},
		},
	}

	keyResolved := resolved.cacheKey("/v1/accounts", nil, nil)
	keyUnresolved := unresolved.cacheKey("/v1/accounts", nil, nil)

	if keyResolved == keyUnresolved {
		t.Fatalf("cache key collision: resolved=%q unresolved=%q — an unresolved placeholder must not collide with a resolved one", keyResolved, keyUnresolved)
	}
}

// TestCacheKey_IncludesAllTemplateVars verifies that every key in
// TemplateVars—not just "environment"—contributes to the cache key. Two
// configs with the same "environment" but a different secondary var must
// produce distinct keys even when that secondary var does not appear in
// BaseURL (over-conservative partitioning is safe; under-partitioning is not).
func TestCacheKey_IncludesAllTemplateVars(t *testing.T) {
	t.Parallel()

	const baseURL = "https://api.example.com"

	a := &Client{
		BaseURL: baseURL,
		Config: &config.Config{
			TemplateVars: map[string]string{"environment": "sandbox", "tenant": "acme"},
		},
	}
	b := &Client{
		BaseURL: baseURL,
		Config: &config.Config{
			TemplateVars: map[string]string{"environment": "sandbox", "tenant": "beta"},
		},
	}

	keyA := a.cacheKey("/v1/accounts", nil, nil)
	keyB := b.cacheKey("/v1/accounts", nil, nil)

	if keyA == keyB {
		t.Fatalf("cache key collision: two distinct tenant values share key %q", keyA)
	}
}
