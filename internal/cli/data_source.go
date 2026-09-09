// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/straddle-build/straddle-cli/internal/client"
	"github.com/straddle-build/straddle-cli/internal/store"
)

// isNetworkError returns true for errors caused by network connectivity issues
// (DNS, connection refused, timeout). HTTP 4xx/5xx errors are NOT network errors.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var urlErr *url.Error
	if As(err, &urlErr) {
		// url.Error wraps the underlying network error
		err = urlErr.Err
	}
	var netErr *net.OpError
	if As(err, &netErr) {
		return true
	}
	var dnsErr *net.DNSError
	if As(err, &dnsErr) {
		return true
	}
	// Check for common network error strings
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "TLS handshake timeout")
}

// openStoreForRead opens the local SQLite store for reading.
// Returns nil, nil if the database file does not exist (no sync has been run).
func openStoreForRead(ctx context.Context, cliName string) (*store.Store, error) {
	dbPath := defaultDBPath(cliName)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}
	return store.OpenWithContext(ctx, dbPath)
}

// localProvenance builds a DataProvenance for local data reads.
func localProvenance(db *store.Store, resourceType, reason string) DataProvenance {
	prov := DataProvenance{
		Source:       "local",
		Reason:       reason,
		ResourceType: resourceType,
	}
	_, lastSynced, _, err := db.GetSyncState(resourceType)
	if err == nil && !lastSynced.IsZero() {
		prov.SyncedAt = &lastSynced
	}
	return prov
}

func attachFreshness(prov DataProvenance, flags *rootFlags) DataProvenance {
	if flags != nil {
		prov.Freshness = flags.freshnessMeta
	}
	return prov
}

func resolveReadWithValues(ctx context.Context, c *client.Client, flags *rootFlags, resourceType string, isList bool, path string, params url.Values, headers map[string]string) (json.RawMessage, DataProvenance, error) {
	return resolveReadRequest(ctx, flags, resourceType, isList, path, firstQueryValues(params), func() (json.RawMessage, error) {
		if queryHasRepeatedValues(params) {
			return c.GetWithValues(path, params, headers)
		}
		return c.GetWithHeaders(path, firstQueryValues(params), headers)
	})
}
func resolveReadRequest(ctx context.Context, flags *rootFlags, resourceType string, isList bool, path string, localParams map[string]string, liveRead func() (json.RawMessage, error)) (json.RawMessage, DataProvenance, error) {
	switch flags.dataSource {
	case "local":
		data, prov, err := resolveLocal(ctx, resourceType, isList, path, localParams, "user_requested")
		return data, attachFreshness(prov, flags), err
	case "live":
		data, err := liveRead()
		if err != nil {
			return nil, DataProvenance{}, err
		}
		return data, attachFreshness(DataProvenance{Source: "live"}, flags), nil
	default:
		data, err := liveRead()
		if err == nil {
			writeThroughCache(ctx, resourceType, data)
			return data, attachFreshness(DataProvenance{Source: "live"}, flags), nil
		}
		if !isNetworkError(err) {
			return nil, DataProvenance{}, err
		}
		fallbackData, fallbackProv, fallbackErr := resolveLocal(ctx, resourceType, isList, path, localParams, "api_unreachable")
		if fallbackErr != nil {
			return nil, DataProvenance{}, fmt.Errorf("API unreachable and no local data. Run 'straddle sync' to enable offline access.\n\nOriginal error: %w", err)
		}
		return fallbackData, attachFreshness(fallbackProv, flags), nil
	}
}

func resolvePaginatedReadWithValues(ctx context.Context, c *client.Client, flags *rootFlags, resourceType string, path string, params url.Values, headers map[string]string, fetchAll bool, cursorParam, nextCursorPath, hasMoreField string) (json.RawMessage, DataProvenance, error) {
	return resolvePaginatedReadRequest(ctx, flags, resourceType, path, firstQueryValues(params), func() (json.RawMessage, error) {
		return paginatedGetWithValues(c, path, params, headers, fetchAll, cursorParam, nextCursorPath, hasMoreField)
	})
}

func resolvePaginatedReadRequest(ctx context.Context, flags *rootFlags, resourceType, path string, localParams map[string]string, liveRead func() (json.RawMessage, error)) (json.RawMessage, DataProvenance, error) {
	switch flags.dataSource {
	case "local":
		data, prov, err := resolveLocal(ctx, resourceType, true, path, localParams, "user_requested")
		return data, attachFreshness(prov, flags), err
	case "live":
		data, err := liveRead()
		if err != nil {
			return nil, DataProvenance{}, err
		}
		return data, attachFreshness(DataProvenance{Source: "live"}, flags), nil
	default:
		data, err := liveRead()
		if err == nil {
			writeThroughCache(ctx, resourceType, data)
			return data, attachFreshness(DataProvenance{Source: "live"}, flags), nil
		}
		if !isNetworkError(err) {
			return nil, DataProvenance{}, err
		}
		fallbackData, fallbackProv, fallbackErr := resolveLocal(ctx, resourceType, true, path, localParams, "api_unreachable")
		if fallbackErr != nil {
			return nil, DataProvenance{}, fmt.Errorf("API unreachable and no local data. Run 'straddle sync' to enable offline access.\n\nOriginal error: %w", err)
		}
		return fallbackData, attachFreshness(fallbackProv, flags), nil
	}
}

func firstQueryValues(values url.Values) map[string]string {
	params := make(map[string]string, len(values))
	for key, entries := range values {
		if len(entries) != 0 {
			params[key] = entries[0]
		}
	}
	return params
}

func queryHasRepeatedValues(values url.Values) bool {
	for _, entries := range values {
		if len(entries) > 1 {
			return true
		}
	}
	return false
}

// listEnvelopeMetadataKeys are top-level keys that, when accompanying a
// list-wrapper array, suggest the response is a paginated list envelope
// rather than a detail object. Used by writeThroughCache to decide
// whether to upsert a single-object body. Any envelope key NOT in this
// set (and not a list wrapper itself) signals real per-row data, and
// the envelope is treated as a detail object even when one of its
// wrapper-named fields happens to be an empty array.
var listEnvelopeMetadataKeys = map[string]bool{
	// list wrappers themselves
	"results": true, "data": true, "items": true,
	// pagination cursors / tokens
	"next_cursor": true, "nextCursor": true,
	"next_page_token": true, "nextPageToken": true,
	"page_token": true, "pageToken": true,
	"end_cursor": true, "endCursor": true,
	"start_cursor": true, "startCursor": true,
	"cursor": true, "after": true, "before": true,
	// has-more flags and page numbers
	"has_more": true, "hasMore": true, "has_next": true, "hasNext": true,
	"next_page": true, "previous_page": true,
	"page": true, "page_size": true, "per_page": true,
	// counts / totals
	"total": true, "count": true, "size": true, "total_count": true, "totalCount": true,
	// wrapper objects
	"links": true, "meta": true, "pagination": true,
	"response_metadata": true, "paging": true,
	// links shape
	"next": true, "prev": true, "previous": true, "first": true, "last": true,
}

// writeThroughCache upserts live API results into the local SQLite store so
// FTS search covers everything the user has looked up — not just explicit syncs.
// Best-effort: failures are silently ignored (the live result already succeeded).
func writeThroughCache(ctx context.Context, resourceType string, data json.RawMessage) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("straddle"))
	if err != nil {
		return
	}
	defer db.Close()

	// Collect items to upsert from various response shapes
	var items []json.RawMessage

	// Try direct array first
	if json.Unmarshal(data, &items) != nil || len(items) == 0 {
		items = nil
		// Try object — check for common envelope patterns (results, data, items)
		var envelope map[string]json.RawMessage
		if json.Unmarshal(data, &envelope) == nil {
			for _, key := range []string{"results", "data", "items"} {
				if raw, ok := envelope[key]; ok {
					var arr []json.RawMessage
					if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
						items = arr
						break
					}
				}
			}
			// Single object detail response: let UpsertBatch's existing
			// resourceIDFieldOverrides mechanism resolve the primary key.
			// Guarding on envelope["id"] dropped any API whose PK is named
			// CertNo / sku / invoiceId / etc. on the floor (#1439).
			//
			// Treat the envelope as a list-shaped response only when EVERY
			// top-level key is either a list-wrapper (results/data/items)
			// holding a real array, or a known pagination-metadata key.
			// A detail object that happens to carry an empty wrapper-named
			// field alongside real data (e.g. {"id":"order","items":[],
			// "status":"pending"}) must still cache as a single row.
			if items == nil && len(envelope) > 0 {
				// Detail responses use the same meta/data envelope as lists, but
				// carry one object under data. Persist the resource object so its
				// identifier can be indexed for offline lookup.
				if raw, ok := envelope["data"]; ok {
					// Unmasked and reveal routes are deliberately excluded from
					// the local FTS mirror. Keep their sensitive detail payloads
					// out of this write-through path.
					if resourceType == "unmasked" || resourceType == "reveal" {
						return
					}
					var object map[string]json.RawMessage
					if json.Unmarshal(raw, &object) == nil && object != nil {
						_, _, _ = db.UpsertBatch(resourceType, []json.RawMessage{raw})
						return
					}
				}
				looksLikeListEnvelope := false
				hasListWrapperArray := false
				for _, key := range []string{"results", "data", "items"} {
					raw, ok := envelope[key]
					if !ok {
						continue
					}
					// json.Unmarshal("null", &arr) succeeds with arr=nil,
					// so require arr != nil to keep true empty arrays in
					// the skip branch while letting scalar/null values fall
					// through as regular field-name collisions.
					var arr []json.RawMessage
					if json.Unmarshal(raw, &arr) == nil && arr != nil {
						hasListWrapperArray = true
						break
					}
				}
				if hasListWrapperArray {
					looksLikeListEnvelope = true
					for k := range envelope {
						if !listEnvelopeMetadataKeys[k] {
							looksLikeListEnvelope = false
							break
						}
					}
				}
				if !looksLikeListEnvelope {
					_, _, _ = db.UpsertBatch(resourceType, []json.RawMessage{data})
					return
				}
			}
		}
	}

	if len(items) > 0 {
		_, _, _ = db.UpsertBatch(resourceType, items)
	}
}

// resolveLocal reads data from the local SQLite store.
// Note: local reads return ALL synced data for the resource type. Endpoint-specific
// filters (query params, path scoping like /teams/{id}/users) are NOT applied locally.
// The provenance metadata includes "unscoped":true when params were present but not applied.
func resolveLocal(ctx context.Context, resourceType string, isList bool, path string, params map[string]string, reason string) (json.RawMessage, DataProvenance, error) {
	db, err := openStoreForRead(ctx, "straddle")
	if err != nil {
		return nil, DataProvenance{}, fmt.Errorf("opening local database: %w\nRun 'straddle sync' first.", err)
	}
	if db == nil {
		return nil, DataProvenance{}, fmt.Errorf("no local data. Run 'straddle sync' first")
	}
	defer db.Close()

	prov := localProvenance(db, resourceType, reason)

	// Warn if endpoint had filters that local reads can't reproduce
	if len(params) > 0 {
		fmt.Fprintf(os.Stderr, "warning: local data is unfiltered — endpoint filters are not applied to cached data\n")
	}

	if isList {
		raw, err := db.List(resourceType, 0) // 0 = no limit, return all synced data
		if err != nil {
			return nil, DataProvenance{}, fmt.Errorf("querying local store: %w", err)
		}
		// Filter out empty/invalid records (empty arrays, null, whitespace-only)
		// that can end up in the store from pagination boundary artifacts.
		var items []json.RawMessage
		for _, r := range raw {
			trimmed := strings.TrimSpace(string(r))
			if trimmed == "" || trimmed == "null" || trimmed == "[]" || trimmed == "{}" {
				continue
			}
			items = append(items, r)
		}
		if len(items) == 0 {
			return nil, DataProvenance{}, fmt.Errorf("no local data for %q. Run 'straddle sync' first", resourceType)
		}
		// Marshal []json.RawMessage into a single JSON array
		data, err := json.Marshal(items)
		if err != nil {
			return nil, DataProvenance{}, fmt.Errorf("marshaling local data: %w", err)
		}
		return data, prov, nil
	}

	// Get by ID — extract the last path segment as the ID
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	id := parts[len(parts)-1]

	item, err := db.Get(resourceType, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, DataProvenance{}, fmt.Errorf("resource %q with ID %q not found in local store. Run 'straddle sync' first", resourceType, id)
		}
		return nil, DataProvenance{}, fmt.Errorf("querying local store: %w", err)
	}
	return item, prov, nil
}

// Ensure time import is used (compilation guard).
var _ = time.Now
