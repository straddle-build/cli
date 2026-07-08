// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: settlement reconciliation. Joins synced payments to their
// funding events in the local store so you can see what settled and what is
// still outstanding. Hand-authored; survives regen.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type reconcilePayment struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Amount int64  `json:"amount"`
}

type reconcileGroup struct {
	FundingEventID string             `json:"funding_event_id"`
	Status         string             `json:"status,omitempty"`
	Direction      string             `json:"direction,omitempty"`
	FundingAmount  int64              `json:"funding_amount"`
	PaymentCount   int                `json:"payment_count"`
	PaymentTotal   int64              `json:"payment_total"`
	Payments       []reconcilePayment `json:"payments"`
}

type reconcileBucket struct {
	PaymentCount int                `json:"payment_count"`
	PaymentTotal int64              `json:"payment_total"`
	Payments     []reconcilePayment `json:"payments"`
}

type reconcileResult struct {
	FundingEvents []reconcileGroup `json:"funding_events"`
	Outstanding   reconcileBucket  `json:"outstanding"`
}

func toReconcilePayment(p straddlePayment) reconcilePayment {
	return reconcilePayment{ID: p.ID, Type: p.PaymentType, Status: p.Status, Amount: p.Amount}
}

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var fundingEvent string
	var outstanding bool

	cmd := &cobra.Command{
		Use:         "reconcile",
		Short:       "Match synced payments to funding events; show settled vs outstanding",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: "Match synced charges and payouts to their funding events from the local\n" +
			"store. With no flags, groups payments under each funding event and lists\n" +
			"what has not settled yet. Requires 'sync' to have populated the store.",
		Example: "  straddle reconcile --outstanding --json\n" +
			"  straddle reconcile --funding-event fe_123 --json",
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
			events, err := loadStraddleFundingEvents(cmd.Context(), db)
			if err != nil {
				return fmt.Errorf("loading funding events: %w", err)
			}
			eventByID := map[string]straddleFundingEvent{}
			for _, e := range events {
				eventByID[e.ID] = e
			}

			groups := map[string]*reconcileGroup{}
			var bucket reconcileBucket
			for _, p := range payments {
				refs := p.fundingRefs()
				if len(refs) == 0 {
					bucket.Payments = append(bucket.Payments, toReconcilePayment(p))
					bucket.PaymentCount++
					bucket.PaymentTotal += p.Amount
					continue
				}
				for _, fid := range refs {
					g := groups[fid]
					if g == nil {
						g = &reconcileGroup{FundingEventID: fid}
						if e, ok := eventByID[fid]; ok {
							g.Status = e.Status
							g.Direction = e.Direction
							g.FundingAmount = e.Amount
						}
						groups[fid] = g
					}
					g.Payments = append(g.Payments, toReconcilePayment(p))
					g.PaymentCount++
					g.PaymentTotal += p.Amount
				}
			}

			// Single funding-event view.
			if fundingEvent != "" {
				g := groups[fundingEvent]
				if g == nil {
					g = &reconcileGroup{FundingEventID: fundingEvent, Payments: []reconcilePayment{}}
					if e, ok := eventByID[fundingEvent]; ok {
						g.Status, g.Direction, g.FundingAmount = e.Status, e.Direction, e.Amount
					}
				}
				if straddleWantsJSON(cmd, flags) {
					return flags.printJSON(cmd, g)
				}
				printReconcileGroup(cmd, *g)
				return nil
			}

			// Outstanding-only view.
			if outstanding {
				if bucket.Payments == nil {
					bucket.Payments = []reconcilePayment{}
				}
				if straddleWantsJSON(cmd, flags) {
					return flags.printJSON(cmd, bucket)
				}
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "Outstanding (not yet settled): %d payments, %s\n\n", bucket.PaymentCount, dollars(bucket.PaymentTotal))
				printReconcilePaymentRows(cmd, bucket.Payments)
				return nil
			}

			// Full view.
			result := reconcileResult{Outstanding: bucket}
			if result.Outstanding.Payments == nil {
				result.Outstanding.Payments = []reconcilePayment{}
			}
			ids := make([]string, 0, len(groups))
			for id := range groups {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			for _, id := range ids {
				result.FundingEvents = append(result.FundingEvents, *groups[id])
			}
			if result.FundingEvents == nil {
				result.FundingEvents = []reconcileGroup{}
			}
			if straddleWantsJSON(cmd, flags) {
				return flags.printJSON(cmd, result)
			}
			w := cmd.OutOrStdout()
			if len(result.FundingEvents) == 0 && result.Outstanding.PaymentCount == 0 {
				fmt.Fprintln(w, "No payments synced. Run 'straddle sync' first.")
				return nil
			}
			for _, g := range result.FundingEvents {
				printReconcileGroup(cmd, g)
			}
			fmt.Fprintf(w, "\nOutstanding (not yet settled): %d payments, %s\n", result.Outstanding.PaymentCount, dollars(result.Outstanding.PaymentTotal))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&fundingEvent, "funding-event", "", "Show only payments tied to this funding event id")
	cmd.Flags().BoolVar(&outstanding, "outstanding", false, "Show only payments not yet tied to a funding event")
	return cmd
}

func printReconcileGroup(cmd *cobra.Command, g reconcileGroup) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s  [%s %s]  funding %s  payments %d (%s)\n", g.FundingEventID, g.Direction, g.Status, dollars(g.FundingAmount), g.PaymentCount, dollars(g.PaymentTotal))
	printReconcilePaymentRows(cmd, g.Payments)
}

func printReconcilePaymentRows(cmd *cobra.Command, ps []reconcilePayment) {
	w := cmd.OutOrStdout()
	for _, p := range ps {
		fmt.Fprintf(w, "  %-28s %-8s %-10s %s\n", p.ID, p.Type, p.Status, dollars(p.Amount))
	}
}
