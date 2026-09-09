// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/straddle-build/straddle-cli/internal/store"
)

func TestSearchLocalCustomTypeFiltersResults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		resourceType string
		id           string
	}{
		{resourceType: "transactions", id: "transaction-1"},
		{resourceType: "charges", id: "charge-1"},
	} {
		data, marshalErr := json.Marshal(map[string]string{"id": item.id, "name": "shared search term"})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if err := db.Upsert(item.resourceType, item.id, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	flags := rootFlags{}
	cmd := newRootCmd(&flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json", "--data-source", "local", "search", "shared", "--type", " Transactions ", "--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var output struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode search output: %v\noutput: %s", err, stdout.String())
	}
	if len(output.Data) != 1 || output.Data[0].ID != "transaction-1" {
		t.Fatalf("search results = %+v, want only transaction-1\noutput: %s", output.Data, stdout.String())
	}
}
