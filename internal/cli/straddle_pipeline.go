// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: payment pipeline + cancel window. Groups synced payments by
// lifecycle status and flags which are still cancelable (created/scheduled/
// on_hold) versus locked once they reach pending. Hand-authored; survives regen.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type pipelineEntry struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Amount     int64  `json:"amount"`
	Cancelable bool   `json:"cancelable"`
}

type pipelineStatusGroup struct {
	Status     string `json:"status"`
	Cancelable bool   `json:"cancelable"`
	Count      int    `json:"count"`
	Total      int64  `json:"total"`
}

type pipelineResult struct {
	ByStatus        []pipelineStatusGroup `json:"by_status"`
	CancelableCount int                   `json:"cancelable_count"`
	CancelableTotal int64                 `json:"cancelable_total"`
}

func newPipelineCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var cancelableOnly bool

	cmd := &cobra.Command{
		Use:         "pipeline",
		Short:       "Group synced payments by status; flag which are still cancelable",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: "Group synced charges and payouts by lifecycle status. Payments in\n" +
			"created, scheduled, or on_hold can still be held/released/cancelled;\n" +
			"once a payment reaches pending it is locked. Use --cancelable to list\n" +
			"only the payments you can still act on before a cutoff.",
		Example: "  straddle-pp-cli pipeline --json\n" +
			"  straddle-pp-cli pipeline --cancelable --json --select id,type,status,amount",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStraddleStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			payments, err := loadStraddlePayments(cmd.Context(), db)
			if err != nil {
				return fmt.Errorf("loading payments: %w", err)
			}

			if cancelableOnly {
				entries := make([]pipelineEntry, 0)
				for _, p := range payments {
					if isCancelableStatus(p.Status) {
						entries = append(entries, pipelineEntry{ID: p.ID, Type: p.PaymentType, Status: p.Status, Amount: p.Amount, Cancelable: true})
					}
				}
				sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
				if straddleWantsJSON(cmd, flags) {
					return flags.printJSON(cmd, entries)
				}
				w := cmd.OutOrStdout()
				if len(entries) == 0 {
					fmt.Fprintln(w, "No cancelable payments. (Payments are locked once they reach pending.)")
					return nil
				}
				for _, e := range entries {
					fmt.Fprintf(w, "%-28s %-8s %-10s %s\n", e.ID, e.Type, e.Status, dollars(e.Amount))
				}
				return nil
			}

			byStatus := map[string]*pipelineStatusGroup{}
			var cancelableCount int
			var cancelableTotal int64
			for _, p := range payments {
				g := byStatus[p.Status]
				if g == nil {
					g = &pipelineStatusGroup{Status: p.Status, Cancelable: isCancelableStatus(p.Status)}
					byStatus[p.Status] = g
				}
				g.Count++
				g.Total += p.Amount
				if g.Cancelable {
					cancelableCount++
					cancelableTotal += p.Amount
				}
			}
			statuses := make([]string, 0, len(byStatus))
			for s := range byStatus {
				statuses = append(statuses, s)
			}
			sort.Strings(statuses)
			result := pipelineResult{CancelableCount: cancelableCount, CancelableTotal: cancelableTotal}
			for _, s := range statuses {
				result.ByStatus = append(result.ByStatus, *byStatus[s])
			}
			if result.ByStatus == nil {
				result.ByStatus = []pipelineStatusGroup{}
			}
			if straddleWantsJSON(cmd, flags) {
				return flags.printJSON(cmd, result)
			}
			w := cmd.OutOrStdout()
			if len(result.ByStatus) == 0 {
				fmt.Fprintln(w, "No payments synced. Run 'straddle-pp-cli sync' first.")
				return nil
			}
			fmt.Fprintf(w, "%-12s %-11s %6s  %s\n", "STATUS", "CANCELABLE", "COUNT", "TOTAL")
			for _, g := range result.ByStatus {
				fmt.Fprintf(w, "%-12s %-11t %6d  %s\n", g.Status, g.Cancelable, g.Count, dollars(g.Total))
			}
			fmt.Fprintf(w, "\nCancelable now: %d payments, %s\n", result.CancelableCount, dollars(result.CancelableTotal))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&cancelableOnly, "cancelable", false, "List only payments that can still be cancelled/held/released")
	return cmd
}
