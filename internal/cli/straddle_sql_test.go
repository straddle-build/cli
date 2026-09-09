// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/straddle-build/straddle-cli/internal/store"
)

func TestValidateReadOnlySQL(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{name: "select", query: "SELECT 1"},
		{name: "with select", query: "WITH rows AS (SELECT 1) SELECT * FROM rows"},
		{name: "recursive with select", query: "WITH RECURSIVE rows(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM rows WHERE n < 2) SELECT * FROM rows"},
		{name: "keyword-like CTE name with select", query: "WITH update1 AS (SELECT 1 AS n) SELECT n FROM update1"},
		{name: "unicode CTE name with select", query: "WITH updateé AS (SELECT 1 AS n) SELECT n FROM updateé"},
		{name: "with values", query: "WITH rows AS (SELECT 1) VALUES (1)"},
		{name: "semicolon in string", query: "SELECT 'note; urgent'"},
		{name: "semicolon in json path string", query: "SELECT json_extract(data, '$.note; urgent') FROM resources"},
		{name: "semicolon in line comment", query: "SELECT 1 -- ignored; semicolon\n"},
		{name: "semicolon in block comment", query: "SELECT 1 /* ignored; semicolon */"},
		{name: "trailing semicolon", query: "SELECT 1;"},
		{name: "trailing comment after semicolon", query: "SELECT 1; -- comment\n"},
		{name: "trailing attach rejected", query: "SELECT 1; ATTACH DATABASE 'fixture.db' AS other", wantErr: true},
		{name: "trailing attach after comment rejected", query: "SELECT 1; /* comment; */ ATTACH DATABASE 'fixture.db' AS other", wantErr: true},
		{name: "trailing update rejected", query: "SELECT 1; UPDATE resources SET data = '{}'", wantErr: true},
		{name: "trailing pragma rejected", query: "SELECT 1; PRAGMA query_only = OFF", wantErr: true},
		{name: "trailing create rejected", query: "SELECT 1; CREATE TABLE leaked (id TEXT)", wantErr: true},
		{name: "insert rejected", query: "INSERT INTO resources VALUES ('id', 'type', '{}')", wantErr: true},
		{name: "unterminated trailing block comment", query: "SELECT 1; /* trailing comment"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateReadOnlySQL(tc.query)
			if tc.wantErr && err == nil {
				t.Fatalf("validateReadOnlySQL(%q) = nil, want error", tc.query)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateReadOnlySQL(%q) = %v, want nil", tc.query, err)
			}
		})
	}
}

func TestSQLCommandRejectsMutationsWithoutOpeningOrChangingStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dbPath := filepath.Join(t.TempDir(), "data.db")
	seed, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open seed store: %v", err)
	}
	if _, err := seed.DB().Exec(`INSERT INTO resources (id, resource_type, data) VALUES ('seed', 'thing', '{}')`); err != nil {
		seed.Close()
		t.Fatalf("seed resource: %v", err)
	}
	seed.Close()

	attachedPath := filepath.Join(t.TempDir(), "attached.db")
	queries := []string{
		fmt.Sprintf("SELECT 1; ATTACH DATABASE '%s' AS other", attachedPath),
		"SELECT 1; CREATE TABLE leaked (id TEXT)",
		"SELECT 1; INSERT INTO resources (id, resource_type, data) VALUES ('leaked', 'thing', '{}')",
		"WITH doomed AS (SELECT 1) DELETE FROM resources",
		"WITH doomed AS (SELECT 1) INSERT INTO resources (id, resource_type, data) SELECT 'leaked', 'thing', '{}' FROM doomed",
		"WITH doomed AS (SELECT 1) UPDATE resources SET data = '{}'",
	}
	for _, query := range queries {
		t.Run(strings.Fields(query)[2], func(t *testing.T) {
			root := RootCmd()
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs([]string{"--json", "sql", query, "--db", dbPath})
			if err := root.Execute(); err == nil {
				t.Fatalf("sql command succeeded for mutation query %q", query)
			}
		})
	}
	if _, err := os.Stat(attachedPath); !os.IsNotExist(err) {
		t.Fatalf("ATTACH created %q, stat error = %v", attachedPath, err)
	}

	check, err := store.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("open read-only store: %v", err)
	}
	defer check.Close()
	var count int
	if err := check.DB().QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&count); err != nil {
		t.Fatalf("count resources: %v", err)
	}
	if count != 1 {
		t.Fatalf("resource count = %d, want 1", count)
	}
	var tables int
	if err := check.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'leaked'`).Scan(&tables); err != nil {
		t.Fatalf("check leaked table: %v", err)
	}
	if tables != 0 {
		t.Fatal("CREATE TABLE mutation changed the store")
	}
}

func TestSQLCommandReadsSelectAndCTE(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dbPath := filepath.Join(t.TempDir(), "data.db")
	seed, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open seed store: %v", err)
	}
	seed.Close()
	for _, query := range []string{
		"SELECT 'literal;value' AS value",
		"WITH rows AS (SELECT 1 AS value) SELECT value FROM rows",
		"WITH updateé AS (SELECT 1 AS value) SELECT value FROM updateé",
		"WITH rows AS (SELECT 1) VALUES (1)",
		"\fSELECT 1 AS value",
		"SELECT 1 AS value;\f",
		"\ufeffSELECT 1 AS value",
		"SELECT 1 AS value;\ufeff",
		"\f\ufeffSELECT 1 AS value",
		"\ufeff\fSELECT 1 AS value",
	} {
		t.Run(query, func(t *testing.T) {
			root := RootCmd()
			var stdout bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&bytes.Buffer{})
			root.SetArgs([]string{"--json", "sql", query, "--db", dbPath})
			if err := root.Execute(); err != nil {
				t.Fatalf("sql command: %v", err)
			}
			if stdout.Len() == 0 {
				t.Fatalf("output = %q, want query result", stdout.String())
			}
		})
	}
}

func TestCountSQLStatementsIgnoresQuotedAndCommentSemicolons(t *testing.T) {
	query := "SELECT '[;]' /* ; */ FROM resources -- ;\n"
	if got := countSQLStatements(query); got != 1 {
		t.Fatalf("countSQLStatements(%q) = %d, want 1", query, got)
	}
	query = "SELECT 1; -- comment\n SELECT 2"
	if got := countSQLStatements(query); got != 2 {
		t.Fatalf("countSQLStatements(%q) = %d, want 2", query, got)
	}
}
