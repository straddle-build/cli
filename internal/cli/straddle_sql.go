// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Local-SQLite power feature: run read-only SQL against the synced store.
// The generator does not emit a human-facing `sql` Cobra command — this
// fills that gap.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/straddle-build/straddle-cli/internal/store"
)

// stripLeadingSQLNoiseCLI drops leading whitespace, line/block comments, and
// statement separators so the read-only gate matches what SQLite actually
// parses as the first keyword.
func stripLeadingSQLNoiseCLI(query string) string {
	for {
		query = strings.TrimLeft(query, " \t\r\n;")
		switch {
		case strings.HasPrefix(query, "--"):
			if idx := strings.IndexByte(query, '\n'); idx >= 0 {
				query = query[idx+1:]
				continue
			}
			return ""
		case strings.HasPrefix(query, "/*"):
			if idx := strings.Index(query[2:], "*/"); idx >= 0 {
				query = query[2+idx+2:]
				continue
			}
			return ""
		default:
			return query
		}
	}
}

// validateReadOnlySQL allows only SELECT / WITH queries, enforcing the
// CLI's read-only SQL boundary.
func validateReadOnlySQL(query string) error {
	if count := countSQLStatements(query); count != 1 {
		return fmt.Errorf("only one read-only SQL statement is allowed")
	}
	if readOnlySQLVerb(query) != "SELECT" {
		return fmt.Errorf("only read-only SELECT/WITH queries are allowed")
	}
	return nil
}

func readOnlySQLVerb(query string) string {
	query = stripLeadingSQLNoiseCLI(query)
	word, rest := nextSQLWord(query)
	if word != "WITH" {
		return word
	}

	depth := 0
	for len(rest) > 0 {
		switch rest[0] {
		case ' ', '\t', '\r', '\n', ',', ';':
			rest = rest[1:]
		case '(':
			depth++
			rest = rest[1:]
		case ')':
			depth--
			rest = rest[1:]
		case '-', '/':
			if strings.HasPrefix(rest, "--") {
				if idx := strings.IndexByte(rest, '\n'); idx >= 0 {
					rest = rest[idx+1:]
				} else {
					return ""
				}
				continue
			}
			if strings.HasPrefix(rest, "/*") {
				if idx := strings.Index(rest[2:], "*/"); idx >= 0 {
					rest = rest[2+idx+2:]
				} else {
					return ""
				}
				continue
			}
			rest = rest[1:]
		case '\'', '"', '`':
			rest = skipSQLQuoted(rest, rest[0])
		case '[':
			rest = skipSQLBracketIdentifier(rest)
		default:
			var next string
			word, next = nextSQLWord(rest)
			if next == rest {
				rest = rest[1:]
				continue
			}
			rest = next
			if depth == 0 && (word == "SELECT" || word == "INSERT" || word == "UPDATE" || word == "DELETE" || word == "REPLACE") {
				return word
			}
		}
	}
	return ""
}

func nextSQLWord(query string) (string, string) {
	query = strings.TrimLeft(query, " \t\r\n")
	i := 0
	for i < len(query) && ((query[i] >= 'a' && query[i] <= 'z') || (query[i] >= 'A' && query[i] <= 'Z') || (query[i] >= '0' && query[i] <= '9') || query[i] == '_' || query[i] == '$') {
		i++
	}
	if i == 0 {
		return "", query
	}
	return strings.ToUpper(query[:i]), query[i:]
}

func skipSQLQuoted(query string, quote byte) string {
	query = query[1:]
	for len(query) > 0 {
		if query[0] == quote {
			query = query[1:]
			if len(query) > 0 && query[0] == quote {
				query = query[1:]
				continue
			}
			return query
		}
		query = query[1:]
	}
	return query
}

func skipSQLBracketIdentifier(query string) string {
	query = query[1:]
	if idx := strings.IndexByte(query, ']'); idx >= 0 {
		return query[idx+1:]
	}
	return ""
}

func countSQLStatements(query string) int {
	count := 0
	hasToken := false
	for i := 0; i < len(query); {
		switch query[i] {
		case ' ', '\t', '\r', '\n', ';':
			if query[i] == ';' && hasToken {
				count++
				hasToken = false
			}
			i++
		case '-', '/':
			if i+1 < len(query) && query[i] == '-' && query[i+1] == '-' {
				i += 2
				for i < len(query) && query[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < len(query) && query[i] == '/' && query[i+1] == '*' {
				i += 2
				for i+1 < len(query) && (query[i] != '*' || query[i+1] != '/') {
					i++
				}
				if i+1 < len(query) {
					i += 2
				} else {
					i = len(query)
				}
				continue
			}
			hasToken = true
			i++
		case '\'', '"', '`':
			quote := query[i]
			hasToken = true
			i++
			for i < len(query) {
				if query[i] == quote {
					i++
					if i < len(query) && query[i] == quote {
						i++
						continue
					}
					break
				}
				i++
			}
		case '[':
			hasToken = true
			i++
			for i < len(query) {
				if query[i] == ']' {
					i++
					break
				}
				i++
			}
		default:
			hasToken = true
			i++
		}
	}
	if hasToken {
		count++
	}
	return count
}

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "sql [query]",
		Short:       "Run read-only SQL against the local synced SQLite store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: "Run an ad-hoc read-only SQL query (SELECT or WITH ... SELECT) against the\n" +
			"local SQLite store populated by sync. Tables match resource names:\n" +
			"payments, customers, paykeys, funding_events, accounts, organizations,\n" +
			"representatives, linked_bank_accounts. The JSON resource body is in the\n" +
			"`data` column (use json_extract(data, '$.field')). Read-only: only\n" +
			"SELECT/WITH are accepted.",
		Example: "  straddle sql \"SELECT json_extract(data,'\\$.status') AS status, COUNT(*) n FROM payments GROUP BY status\" --json\n" +
			"  straddle sql \"SELECT id, json_extract(data,'\\$.amount') AS amount FROM payments ORDER BY amount DESC LIMIT 10\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			if err := validateReadOnlySQL(query); err != nil {
				return usageErr(err)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("straddle")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'straddle sync' first.", err)
			}
			defer db.Close()

			rows, err := db.Query(query)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("reading columns: %w", err)
			}
			results := make([]map[string]any, 0)
			for rows.Next() {
				values := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range values {
					ptrs[i] = &values[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				row := make(map[string]any, len(cols))
				for i, col := range cols {
					// SQLite TEXT columns come back as []byte; convert so JSON
					// shows text, not base64.
					if b, ok := values[i].([]byte); ok {
						row[col] = string(b)
					} else {
						row[col] = values[i]
					}
				}
				results = append(results, row)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}

			if straddleWantsJSON(cmd, flags) {
				return flags.printJSON(cmd, results)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "0 rows. (Run 'straddle sync' if the store is empty.)")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), results)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
