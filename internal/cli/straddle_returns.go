// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: ACH return / NSF analysis. Surfaces failed and reversed
// payments with their reason codes and ranks repeat-offender paykeys/customers
// from the local store.
package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type returnEntry struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Amount     int64  `json:"amount"`
	Paykey     string `json:"paykey"`
	CustomerID string `json:"customer_id,omitempty"`
	Code       string `json:"code,omitempty"`
	Reason     string `json:"reason,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

type returnOffender struct {
	Paykey     string `json:"paykey"`
	CustomerID string `json:"customer_id,omitempty"`
	Returns    int    `json:"returns"`
	Total      int64  `json:"total"`
}

type returnsResult struct {
	WindowDays int              `json:"window_days"`
	Count      int              `json:"count"`
	Total      int64            `json:"total"`
	Returns    []returnEntry    `json:"returns,omitempty"`
	Offenders  []returnOffender `json:"offenders,omitempty"`
}

func returnReason(d straddleStatusDetails) string {
	switch {
	case d.Reason != "":
		return d.Reason
	case d.Message != "":
		return d.Message
	default:
		return ""
	}
}

func newReturnsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	var repeatOffenders bool

	cmd := &cobra.Command{
		Use:         "returns",
		Short:       "Failed/reversed payments with reason codes; rank repeat offenders",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: "Surface synced payments that failed before funding or reversed after\n" +
			"funding, with their ACH reason codes (R01 NSF, R02 closed, R05 dispute).\n" +
			"Use --repeat-offenders to rank the paykeys that keep bouncing.",
		Example: "  straddle returns --days 30 --json\n" +
			"  straddle returns --days 90 --repeat-offenders --json",
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

			var cutoff time.Time
			if days > 0 {
				cutoff = time.Now().AddDate(0, 0, -days)
			}

			entries := make([]returnEntry, 0)
			for _, p := range payments {
				if !isReturnStatus(p.Status) {
					continue
				}
				if days > 0 {
					if ts, ok := parseStraddleTime(p.CreatedAt); ok && ts.Before(cutoff) {
						continue
					}
				}
				entries = append(entries, returnEntry{
					ID: p.ID, Type: p.PaymentType, Status: p.Status, Amount: p.Amount,
					Paykey: p.Paykey, CustomerID: p.CustomerDetails.ID,
					Code: p.StatusDetails.Code, Reason: returnReason(p.StatusDetails), CreatedAt: p.CreatedAt,
				})
			}

			var total int64
			for _, e := range entries {
				total += e.Amount
			}
			result := returnsResult{WindowDays: days, Count: len(entries), Total: total}

			if repeatOffenders {
				byPaykey := map[string]*returnOffender{}
				for _, e := range entries {
					key := e.Paykey
					if key == "" {
						key = e.CustomerID
					}
					if key == "" {
						continue
					}
					o := byPaykey[key]
					if o == nil {
						o = &returnOffender{Paykey: e.Paykey, CustomerID: e.CustomerID}
						byPaykey[key] = o
					}
					o.Returns++
					o.Total += e.Amount
				}
				offenders := make([]returnOffender, 0, len(byPaykey))
				for _, o := range byPaykey {
					if o.Returns > 1 { // "repeat" = more than one return
						offenders = append(offenders, *o)
					}
				}
				sort.Slice(offenders, func(i, j int) bool {
					if offenders[i].Returns != offenders[j].Returns {
						return offenders[i].Returns > offenders[j].Returns
					}
					return offenders[i].Total > offenders[j].Total
				})
				result.Offenders = offenders
				if straddleWantsJSON(cmd, flags) {
					return flags.printJSON(cmd, result)
				}
				w := cmd.OutOrStdout()
				if len(offenders) == 0 {
					fmt.Fprintln(w, "No repeat-offender paykeys in window.")
					return nil
				}
				fmt.Fprintf(w, "%-28s %-28s %8s  %s\n", "PAYKEY", "CUSTOMER", "RETURNS", "TOTAL")
				for _, o := range offenders {
					pk := o.Paykey
					if pk == "" {
						pk = "-"
					}
					cust := o.CustomerID
					if cust == "" {
						cust = "-"
					}
					fmt.Fprintf(w, "%-28s %-28s %8d  %s\n", pk, cust, o.Returns, dollars(o.Total))
				}
				return nil
			}

			// Newest first, comparing parsed times (string compare misorders
			// mixed offsets and date-only values); unparseable timestamps sort last.
			sort.SliceStable(entries, func(i, j int) bool {
				ti, oki := parseStraddleTime(entries[i].CreatedAt)
				tj, okj := parseStraddleTime(entries[j].CreatedAt)
				if oki != okj {
					return oki
				}
				return ti.After(tj)
			})
			result.Returns = entries
			if straddleWantsJSON(cmd, flags) {
				return flags.printJSON(cmd, result)
			}
			w := cmd.OutOrStdout()
			if len(entries) == 0 {
				fmt.Fprintln(w, "No failed or reversed payments in window.")
				return nil
			}
			fmt.Fprintf(w, "%-28s %-8s %-9s %-8s %s\n", "ID", "TYPE", "STATUS", "CODE", "REASON")
			for _, e := range entries {
				fmt.Fprintf(w, "%-28s %-8s %-9s %-8s %s\n", e.ID, e.Type, e.Status, e.Code, e.Reason)
			}
			fmt.Fprintf(w, "\n%d returns, %s\n", result.Count, dollars(result.Total))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 30, "Only include returns created within this many days (0 = all)")
	cmd.Flags().BoolVar(&repeatOffenders, "repeat-offenders", false, "Rank paykeys/customers with more than one return")
	return cmd
}
