// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: identity review queue. Lists customers and paykeys sitting in
// review status, oldest first, with age-in-queue so the KYC backlog is
// triageable. Hand-authored; survives regen.
package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type reviewItem struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"` // "customer" or "paykey"
	Status     string `json:"status"`
	Name       string `json:"name,omitempty"`
	CustomerID string `json:"customer_id,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	AgeDays    int    `json:"age_days"` // -1 when created_at is unknown
}

type reviewQueueResult struct {
	Count int          `json:"count"`
	Items []reviewItem `json:"items"`
}

func ageInDays(createdAt string, now time.Time) int {
	ts, ok := parseStraddleTime(createdAt)
	if !ok {
		return -1
	}
	d := int(now.Sub(ts).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

func newReviewQueueCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var kind string

	cmd := &cobra.Command{
		Use:         "review-queue",
		Short:       "Customers and paykeys awaiting a review decision, oldest first",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: "List synced customers and paykeys in review status, sorted oldest first\n" +
			"with age in queue. These are the identity items blocking downstream\n" +
			"charges and payouts from releasing. Use --type to narrow to one kind.",
		Example: "  straddle review-queue --json\n" +
			"  straddle review-queue --type customers --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			switch kind {
			case "", "all", "customers", "paykeys":
			default:
				return usageErr(fmt.Errorf("--type must be one of: all, customers, paykeys (got %q)", kind))
			}
			db, err := openStraddleStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			now := time.Now()
			items := make([]reviewItem, 0)

			if kind == "" || kind == "all" || kind == "customers" {
				customers, err := loadStraddleCustomers(cmd.Context(), db)
				if err != nil {
					return fmt.Errorf("loading customers: %w", err)
				}
				for _, c := range customers {
					if c.Status != "review" {
						continue
					}
					items = append(items, reviewItem{ID: c.ID, Kind: "customer", Status: c.Status, Name: c.Name, CreatedAt: c.CreatedAt, AgeDays: ageInDays(c.CreatedAt, now)})
				}
			}
			if kind == "" || kind == "all" || kind == "paykeys" {
				paykeys, err := loadStraddlePaykeys(cmd.Context(), db)
				if err != nil {
					return fmt.Errorf("loading paykeys: %w", err)
				}
				for _, p := range paykeys {
					if p.Status != "review" {
						continue
					}
					name := p.Label
					if name == "" {
						name = p.InstitutionName
					}
					items = append(items, reviewItem{ID: p.ID, Kind: "paykey", Status: p.Status, Name: name, CustomerID: p.CustomerID, CreatedAt: p.CreatedAt, AgeDays: ageInDays(p.CreatedAt, now)})
				}
			}

			// Oldest first: known ages descending, unknown ages last.
			sort.SliceStable(items, func(i, j int) bool {
				ai, aj := items[i].AgeDays, items[j].AgeDays
				if (ai < 0) != (aj < 0) {
					return aj < 0
				}
				return ai > aj
			})

			result := reviewQueueResult{Count: len(items), Items: items}
			if straddleWantsJSON(cmd, flags) {
				return flags.printJSON(cmd, result)
			}
			w := cmd.OutOrStdout()
			if len(items) == 0 {
				fmt.Fprintln(w, "Review queue empty. (Run 'straddle sync' if you expected items.)")
				return nil
			}
			fmt.Fprintf(w, "%-28s %-9s %6s  %s\n", "ID", "KIND", "AGE_D", "NAME")
			for _, it := range items {
				age := "?"
				if it.AgeDays >= 0 {
					age = fmt.Sprintf("%d", it.AgeDays)
				}
				fmt.Fprintf(w, "%-28s %-9s %6s  %s\n", it.ID, it.Kind, age, it.Name)
			}
			fmt.Fprintf(w, "\n%d items awaiting review\n", result.Count)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&kind, "type", "all", "Which items to show: all, customers, paykeys")
	return cmd
}
