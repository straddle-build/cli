// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestValidateReadOnlySQL(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{name: "select", query: "SELECT 1"},
		{name: "with select", query: "WITH rows AS (SELECT 1) SELECT * FROM rows"},
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
