// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/straddle-build/straddle-cli/internal/config"
)

// shopFromPath extracts the first path segment, which buildURL substitutes for
// {shop} in the BaseURL (server.URL + "/{shop}").
func shopFromPath(t *testing.T, path string) string {
	t.Helper()
	s := strings.TrimPrefix(path, "/")
	if idx := strings.IndexByte(s, '/'); idx >= 0 {
		s = s[:idx]
	}
	return s
}

// TestCacheKey_StaleResponseAcrossTemplateVars verifies end-to-end that two
// clients sharing a cache directory and differing only in a template variable
// used in BaseURL do not serve each other's cached responses. On buggy code,
// the second client receives the first's cached body and its URL is never
// dialed (server hit count stays at 1). With the fix, the second client dials
// its own resolved URL and receives its own response.
func TestCacheKey_StaleResponseAcrossTemplateVars(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		shop := shopFromPath(t, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"shop":%q}`, shop)
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	baseURL := server.URL + "/{shop}"

	alice := New(&config.Config{
		BaseURL:      baseURL,
		TemplateVars: map[string]string{"environment": "sandbox", "shop": "alice"},
	}, time.Second, 0)
	alice.cacheDir = cacheDir

	bob := New(&config.Config{
		BaseURL:      baseURL,
		TemplateVars: map[string]string{"environment": "sandbox", "shop": "bob"},
	}, time.Second, 0)
	bob.cacheDir = cacheDir

	first, err := alice.Get("/list", nil)
	if err != nil {
		t.Fatalf("alice Get: %v", err)
	}
	if !strings.Contains(string(first), `"shop":"alice"`) {
		t.Fatalf("alice response = %s, want shop alice", first)
	}

	second, err := bob.Get("/list", nil)
	if err != nil {
		t.Fatalf("bob Get: %v", err)
	}
	if !strings.Contains(string(second), `"shop":"bob"`) {
		t.Fatalf("bob received stale cache: %s, want shop bob", second)
	}
	if requests != 2 {
		t.Fatalf("server received %d requests, want 2 (bob should dial its own URL, not hit alice's cache)", requests)
	}
}

// TestCacheKey_NoCacheBypassesPopulatedCache verifies that a client with
// NoCache=true bypasses a populated cache and dials its own URL. This is the
// shipped path for sync --path-context, which sets NoCache=true before merging
// path context into TemplateVars. The fix is a no-op on this path: the
// !c.NoCache gate short-circuits before readCache/cacheKey is ever invoked.
func TestCacheKey_NoCacheBypassesPopulatedCache(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		shop := shopFromPath(t, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"shop":%q}`, shop)
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	baseURL := server.URL + "/{shop}"

	warm := New(&config.Config{
		BaseURL:      baseURL,
		TemplateVars: map[string]string{"environment": "sandbox", "shop": "alice"},
	}, time.Second, 0)
	warm.cacheDir = cacheDir

	if _, err := warm.Get("/list", nil); err != nil {
		t.Fatalf("warm Get: %v", err)
	}

	nocache := New(&config.Config{
		BaseURL:      baseURL,
		TemplateVars: map[string]string{"environment": "sandbox", "shop": "bob"},
	}, time.Second, 0)
	nocache.cacheDir = cacheDir
	nocache.NoCache = true

	got, err := nocache.Get("/list", nil)
	if err != nil {
		t.Fatalf("nocache Get: %v", err)
	}
	if !strings.Contains(string(got), `"shop":"bob"`) {
		t.Fatalf("nocache response = %s, want shop bob", got)
	}
	if requests != 2 {
		t.Fatalf("server received %d requests, want 2 (NoCache should bypass cache and dial)", requests)
	}
}

// TestCacheKey_UnresolvedDoesNotReceiveResolvedCachedBody verifies end-to-end
// that a request with an unresolved placeholder (absent from TemplateVars)
// does not receive a cached response from a resolved request. Instead it
// should surface buildURL's TemplateVarError naming the missing placeholder.
//
// On buggy code the cache keys collide (both include only "environment"),
// readCache hits, and the second client silently receives the first's cached
// body with no error. With the fix the keys differ, readCache misses, do()
// invokes buildURL, which returns *TemplateVarError{Names: ["shop"]}.
func TestCacheKey_UnresolvedDoesNotReceiveResolvedCachedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"shop":"alice"}`)
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	baseURL := server.URL + "/{shop}"

	resolved := New(&config.Config{
		BaseURL:      baseURL,
		TemplateVars: map[string]string{"environment": "sandbox", "shop": "alice"},
	}, time.Second, 0)
	resolved.cacheDir = cacheDir

	if _, err := resolved.Get("/list", nil); err != nil {
		t.Fatalf("resolved Get: %v", err)
	}

	unresolved := New(&config.Config{
		BaseURL:      baseURL,
		TemplateVars: map[string]string{"environment": "sandbox"},
	}, time.Second, 0)
	unresolved.cacheDir = cacheDir

	_, err := unresolved.Get("/list", nil)
	if err == nil {
		t.Fatal("unresolved Get: error = nil, want *TemplateVarError (should not receive cached body)")
	}
	var tve *TemplateVarError
	if !errors.As(err, &tve) {
		t.Fatalf("unresolved Get: error type = %T, want *TemplateVarError", err)
	}
	found := false
	for _, name := range tve.Names {
		if name == "shop" {
			found = true
		}
	}
	if !found {
		t.Fatalf("TemplateVarError names = %v, want to contain \"shop\"", tve.Names)
	}
}
