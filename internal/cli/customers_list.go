// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCustomersListCmd(flags *rootFlags) *cobra.Command {
	var flagPageNumber int
	var flagPageSize int
	var flagSortBy string
	var flagSortOrder string
	var flagCreatedFrom string
	var flagCreatedTo string
	var flagName string
	var flagExternalId string
	var flagEmail string
	var flagStatus string
	var flagSearchText string
	var flagTypes string
	var flagAll bool

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Lists or searches customers connected to your account. All supported query parameters are optional. If none are...",
		Example:     "  straddle customers list",
		Annotations: map[string]string{"straddle:endpoint": "customers.list", "straddle:method": "GET", "straddle:path": "/v1/customers", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("sort-by") {
				allowedSortBy := []string{"name", "created_at"}
				validSortBy := false
				for _, v := range allowedSortBy {
					if flagSortBy == v {
						validSortBy = true
						break
					}
				}
				if !validSortBy {
					fmt.Fprintf(os.Stderr, "warning: --%s %q not in allowed set %v\n", "sort-by", flagSortBy, allowedSortBy)
				}
			}
			if cmd.Flags().Changed("sort-order") {
				allowedSortOrder := []string{"asc", "desc"}
				validSortOrder := false
				for _, v := range allowedSortOrder {
					if flagSortOrder == v {
						validSortOrder = true
						break
					}
				}
				if !validSortOrder {
					fmt.Fprintf(os.Stderr, "warning: --%s %q not in allowed set %v\n", "sort-order", flagSortOrder, allowedSortOrder)
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/v1/customers"
			data, prov, err := resolvePaginatedRead(cmd.Context(), c, flags, "customers", path, map[string]string{
				"page_number":  fmt.Sprintf("%v", flagPageNumber),
				"page_size":    fmt.Sprintf("%v", flagPageSize),
				"sort_by":      fmt.Sprintf("%v", flagSortBy),
				"sort_order":   fmt.Sprintf("%v", flagSortOrder),
				"created_from": fmt.Sprintf("%v", flagCreatedFrom),
				"created_to":   fmt.Sprintf("%v", flagCreatedTo),
				"name":         fmt.Sprintf("%v", flagName),
				"external_id":  fmt.Sprintf("%v", flagExternalId),
				"email":        fmt.Sprintf("%v", flagEmail),
				"status":       fmt.Sprintf("%v", flagStatus),
				"search_text":  fmt.Sprintf("%v", flagSearchText),
				"types":        fmt.Sprintf("%v", flagTypes),
			}, nil, flagAll, "page_number", "", "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Print provenance to stderr for human-facing output only.
			// Machine-format flags (--json, --csv, --compact, --quiet, --plain,
			// --select) and piped stdout suppress this line; the JSON envelope
			// already carries meta.source for those consumers.
			// SYNC: keep this gate aligned with command_promoted.go.tmpl.
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				if json.Unmarshal(data, &countItems) != nil {
					// Single object, not an array
					countItems = []json.RawMessage{data}
				}
				printProvenance(cmd, len(countItems), prov)
			}
			// For JSON output, wrap with provenance envelope before passing through flags.
			// --select wins over --compact when both are set; --compact only runs when
			// no explicit fields were requested. Explicit format flags (--csv, --quiet,
			// --plain) opt out of the auto-JSON path so piped consumers that asked for
			// a non-JSON format reach the standard pipeline below.
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			// For all other output modes (table, csv, plain, quiet), use the standard pipeline
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&flagPageNumber, "page-number", 0, "Page number for paginated results. Starts at 1.")
	cmd.Flags().IntVar(&flagPageSize, "page-size", 0, "Number of results per page. Maximum: 1000.")
	cmd.Flags().StringVar(&flagSortBy, "sort-by", "", "Sort by (one of: name, created_at)")
	cmd.Flags().StringVar(&flagSortOrder, "sort-order", "asc", "Sort order (one of: asc, desc)")
	cmd.Flags().StringVar(&flagCreatedFrom, "created-from", "", "Start date for filtering by `created_at` date.")
	cmd.Flags().StringVar(&flagCreatedTo, "created-to", "", "End date for filtering by `created_at` date.")
	cmd.Flags().StringVar(&flagName, "name", "", "Filter customers by `name` (partial match).")
	cmd.Flags().StringVar(&flagExternalId, "external-id", "", "Filter by your system's `external_id`.")
	cmd.Flags().StringVar(&flagEmail, "email", "", "Filter customers by `email` address.")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Filter customers by their current `status`.")
	cmd.Flags().StringVar(&flagSearchText, "search-text", "", "General search term to filter customers.")
	cmd.Flags().StringVar(&flagTypes, "types", "", "Filter by customer type `individual` or `business`.")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Fetch all pages")

	return cmd
}
