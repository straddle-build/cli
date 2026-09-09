// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/straddle-build/straddle-cli/internal/store"
)

func newWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Compound workflows that combine multiple API operations",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWorkflowArchiveCmd(flags))
	cmd.AddCommand(newWorkflowStatusCmd(flags))

	return cmd
}
func newWorkflowArchiveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var full bool

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Sync all resources to local store for offline access and search",
		Long: `Archive fetches all syncable resources from the API and stores them in a
local SQLite database. Supports incremental sync (only new data since last run)
and full resync. After archiving, use 'search' for instant full-text search.
Resource failures retain successfully archived data and the summary, but return
a nonzero exit status. Access warnings remain nonfatal.`,
		Example: `  # Archive all resources
  straddle workflow archive

  # Full re-archive (ignore previous sync state)
  straddle workflow archive --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			if dbPath == "" {
				dbPath = defaultDBPath("straddle")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			resources := []string{"accounts", "customers", "funding-events", "linked-bank-accounts", "organizations", "paykeys", "payments", "representatives"}
			totalSynced := 0
			resourcesSynced := 0
			var resourceErrors []error

			// --full clears the cursor here because syncResource reads
			// existingCursor unconditionally; its full param only gates the
			// since filter, not cursor reset. Mirrors newSyncCmd's pattern.
			if full {
				for _, resource := range resources {
					_ = s.SaveSyncState(resource, "", 0)
				}
			}

			// syncResource treats !humanFriendly as machine mode and emits NDJSON
			// progress events straight to os.Stdout, but humanFriendly defaults to
			// false for color-safety and only --human-friendly sets it. Archive's
			// own output is a summary (human text or a single JSON object), not an
			// event stream, so a bare `workflow archive` would leak machine events
			// onto stdout and collide with the --agent JSON summary. Force
			// humanFriendly on for the duration of the sync loop so progress and
			// warnings route to stderr as readable prose, keeping archive's stdout
			// clean for its own summary. `sync` is unaffected (separate RunE).
			prevHumanFriendly := humanFriendly
			humanFriendly = true
			defer func() { humanFriendly = prevHumanFriendly }()

			for _, resource := range resources {
				res := syncResource(c, s, resource, "", full, 100, false, nil)
				totalSynced += res.Count
				if res.Err != nil {
					resourceErrors = append(resourceErrors, res.Err)
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: error: %v\n", resource, res.Err)
					continue
				}
				if res.Warn != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: warning: %v\n", resource, res.Warn)
					continue
				}
				resourcesSynced++
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %d synced\n", resource, res.Count)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(map[string]any{
					"resources_synced": resourcesSynced,
					"total_items":      totalSynced,
					"store_path":       dbPath,
					"timestamp":        time.Now().UTC().Format(time.RFC3339),
				}); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Archived %d items across %d resources to %s\n", totalSynced, resourcesSynced, dbPath)
			}

			if len(resourceErrors) > 0 {
				return fmt.Errorf("archive failed for %d of %d resources: %w", len(resourceErrors), len(resources), errors.Join(resourceErrors...))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/straddle/data.db)")
	cmd.Flags().BoolVar(&full, "full", false, "Full re-archive (ignore previous sync state)")

	return cmd
}

func newWorkflowStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show local archive status and sync state for all resources",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Show archive status
  straddle workflow status

  # Show status as JSON
  straddle workflow status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("straddle")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			status, err := s.Status()
			if err != nil {
				return err
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			}

			if len(status) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No archived data. Run 'workflow archive' to sync.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Archive Status:")
			total := 0
			for resource, count := range status {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d items\n", resource, count)
				total += count
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Total: %d items\n", total)
			fmt.Fprintf(cmd.OutOrStdout(), "  Store: %s\n", dbPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

// defaultDBPath is defined in helpers.go
