// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Local-SQLite power feature: run read-only SQL against the synced store.
// The MCP server already exposes a typed `sql` tool (handleSQL) and the
// cobratree framework set reserves the name `sql`, but the generator does not
// emit a human-facing `sql` Cobra command — so this fills that gap. The
// cobratree walker skips `sql` (framework set), so this does not double up with
// the typed MCP tool. Hand-authored; survives regen.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"straddle-pp-cli/internal/store"
)

// stripLeadingSQLNoiseCLI drops leading whitespace, line/block comments, and
// statement separators so the read-only gate matches what SQLite actually
// parses as the first keyword. Mirrors the MCP server's stripLeadingSQLNoise.
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

// validateReadOnlySQL allows only SELECT / WITH queries. Mirrors the MCP
// server's validateReadOnlyQuery so the CLI and agent surfaces enforce the
// same boundary.
func validateReadOnlySQL(query string) error {
	upper := strings.ToUpper(stripLeadingSQLNoiseCLI(query))
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("only read-only SELECT/WITH queries are allowed")
	}
	return nil
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
		Example: "  straddle-pp-cli sql \"SELECT json_extract(data,'\\$.status') AS status, COUNT(*) n FROM payments GROUP BY status\" --json\n" +
			"  straddle-pp-cli sql \"SELECT id, json_extract(data,'\\$.amount') AS amount FROM payments ORDER BY amount DESC LIMIT 10\"",
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
				dbPath = defaultDBPath("straddle-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'straddle-pp-cli sync' first.", err)
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
				fmt.Fprintln(cmd.OutOrStdout(), "0 rows. (Run 'straddle-pp-cli sync' if the store is empty.)")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), results)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
