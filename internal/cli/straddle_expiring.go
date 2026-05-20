// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: paykey expiry and block monitor. Lists paykeys approaching
// expires_at and blocked paykeys that are unblock-eligible, so recurring
// payments do not fail on stale tokens. Hand-authored; survives regen.
package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type expiringPaykey struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	CustomerID      string `json:"customer_id,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	DaysToExpiry    int    `json:"days_to_expiry"` // negative = already expired
	UnblockEligible bool   `json:"unblock_eligible"`
	Label           string `json:"label,omitempty"`
	Reason          string `json:"reason"` // expired | expiring | blocked_recoverable
}

type expiringResult struct {
	WindowDays int              `json:"window_days"`
	Count      int              `json:"count"`
	Paykeys    []expiringPaykey `json:"paykeys"`
}

var expiringReasonRank = map[string]int{"expired": 0, "expiring": 1, "blocked_recoverable": 2}

func newExpiringCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int

	cmd := &cobra.Command{
		Use:         "expiring",
		Short:       "Paykeys near expiry or blocked-but-unblockable, so payments don't fail",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: "List synced paykeys approaching expires_at (within --days), already\n" +
			"expired, or blocked but unblock-eligible. These are the tokens to refresh\n" +
			"or unblock before recurring charges fail against them.",
		Example: "  straddle-pp-cli expiring --days 14 --json\n" +
			"  straddle-pp-cli expiring --days 30 --json --select id,reason,days_to_expiry",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if days <= 0 {
				days = 14
			}
			db, err := openStraddleStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			paykeys, err := loadStraddlePaykeys(cmd.Context(), db)
			if err != nil {
				return fmt.Errorf("loading paykeys: %w", err)
			}

			now := time.Now()
			out := make([]expiringPaykey, 0)
			for _, p := range paykeys {
				item := expiringPaykey{
					ID: p.ID, Status: p.Status, CustomerID: p.CustomerID,
					ExpiresAt: p.ExpiresAt, UnblockEligible: p.UnblockEligible, Label: p.Label,
				}
				included := false

				if d, ok := daysUntil(p.ExpiresAt, now); ok {
					item.DaysToExpiry = d
					if d < 0 {
						item.Reason = "expired"
						included = true
					} else if d <= days {
						item.Reason = "expiring"
						included = true
					}
				}

				// Blocked-but-recoverable paykeys also need action.
				if !included && p.Status == "blocked" && p.UnblockEligible {
					item.Reason = "blocked_recoverable"
					included = true
				}

				if included {
					out = append(out, item)
				}
			}

			sort.SliceStable(out, func(i, j int) bool {
				ri, rj := expiringReasonRank[out[i].Reason], expiringReasonRank[out[j].Reason]
				if ri != rj {
					return ri < rj
				}
				return out[i].DaysToExpiry < out[j].DaysToExpiry
			})

			result := expiringResult{WindowDays: days, Count: len(out), Paykeys: out}
			if straddleWantsJSON(cmd, flags) {
				return flags.printJSON(cmd, result)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No expiring, expired, or recoverable-blocked paykeys.")
				return nil
			}
			fmt.Fprintf(w, "%-28s %-19s %7s  %s\n", "ID", "REASON", "DAYS", "EXPIRES_AT")
			for _, it := range out {
				dayStr := "-"
				if it.Reason != "blocked_recoverable" {
					dayStr = fmt.Sprintf("%d", it.DaysToExpiry)
				}
				fmt.Fprintf(w, "%-28s %-19s %7s  %s\n", it.ID, it.Reason, dayStr, it.ExpiresAt)
			}
			fmt.Fprintf(w, "\n%d paykeys need attention\n", result.Count)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 14, "Flag paykeys expiring within this many days")
	return cmd
}
