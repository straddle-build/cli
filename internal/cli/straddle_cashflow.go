// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: cashflow analytics. Aggregates synced charge volume in versus
// payout volume out over a date window, including zero-activity days, with net
// flow per day or week. Hand-authored; survives regen.
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type cashflowBucket struct {
	Date        string `json:"date"`
	ChargeCount int    `json:"charge_count"`
	ChargeIn    int64  `json:"charge_in"`
	PayoutCount int    `json:"payout_count"`
	PayoutOut   int64  `json:"payout_out"`
	Net         int64  `json:"net"`
}

type cashflowResult struct {
	WindowDays  int              `json:"window_days"`
	Granularity string           `json:"granularity"`
	TotalIn     int64            `json:"total_in"`
	TotalOut    int64            `json:"total_out"`
	Net         int64            `json:"net"`
	Buckets     []cashflowBucket `json:"buckets"`
}

func newCashflowCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	var weekly bool

	cmd := &cobra.Command{
		Use:         "cashflow",
		Short:       "Charge volume in vs payout volume out over time, with net flow",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: "Aggregate synced charge volume (money in) against payout volume (money\n" +
			"out) per day, including days with zero activity, and report net flow.\n" +
			"Volume is bucketed by each payment's created date. Use --weekly to roll\n" +
			"days up into 7-day buckets.",
		Example: "  straddle-pp-cli cashflow --days 30 --json\n" +
			"  straddle-pp-cli cashflow --days 90 --weekly --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if days <= 0 {
				days = 30
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

			loc := time.Now().Location()
			today := truncDay(time.Now().In(loc))
			start := today.AddDate(0, 0, -(days - 1))

			// Zero-filled day buckets across the window.
			dayIndex := map[string]int{}
			dayBuckets := make([]cashflowBucket, 0, days)
			for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
				key := d.Format("2006-01-02")
				dayIndex[key] = len(dayBuckets)
				dayBuckets = append(dayBuckets, cashflowBucket{Date: key})
			}

			for _, p := range payments {
				ts, ok := parseStraddleTime(p.CreatedAt)
				if !ok {
					continue
				}
				// A date-only created_at is already a calendar day; bucket on the
				// literal date so a tz shift can't move it into an adjacent day or
				// out of the window. Full timestamps convert to the local day.
				var key string
				if len(p.CreatedAt) == 10 {
					key = p.CreatedAt
				} else {
					key = truncDay(ts.In(loc)).Format("2006-01-02")
				}
				idx, in := dayIndex[key]
				if !in {
					continue
				}
				b := &dayBuckets[idx]
				switch p.PaymentType {
				case "payout":
					b.PayoutCount++
					b.PayoutOut += p.Amount
				default: // treat charge (and unknown) as money in
					b.ChargeCount++
					b.ChargeIn += p.Amount
				}
				b.Net = b.ChargeIn - b.PayoutOut
			}

			buckets := dayBuckets
			granularity := "day"
			if weekly {
				granularity = "week"
				buckets = rollWeekly(dayBuckets)
			}

			var totalIn, totalOut int64
			for _, b := range buckets {
				totalIn += b.ChargeIn
				totalOut += b.PayoutOut
			}
			result := cashflowResult{
				WindowDays: days, Granularity: granularity,
				TotalIn: totalIn, TotalOut: totalOut, Net: totalIn - totalOut,
				Buckets: buckets,
			}
			if straddleWantsJSON(cmd, flags) {
				return flags.printJSON(cmd, result)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-12s %8s %14s %8s %14s %14s\n", "DATE", "CHARGES", "IN", "PAYOUTS", "OUT", "NET")
			for _, b := range buckets {
				fmt.Fprintf(w, "%-12s %8d %14s %8d %14s %14s\n", b.Date, b.ChargeCount, dollars(b.ChargeIn), b.PayoutCount, dollars(b.PayoutOut), dollars(b.Net))
			}
			fmt.Fprintf(w, "\nTotal in %s  |  total out %s  |  net %s\n", dollars(result.TotalIn), dollars(result.TotalOut), dollars(result.Net))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 30, "Number of days in the window")
	cmd.Flags().BoolVar(&weekly, "weekly", false, "Roll days up into 7-day buckets")
	return cmd
}

func truncDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// rollWeekly collapses ordered day buckets into 7-day groups, labeled by the
// first day of each group. Zero-activity weeks are preserved.
func rollWeekly(days []cashflowBucket) []cashflowBucket {
	var weeks []cashflowBucket
	for i := 0; i < len(days); i += 7 {
		end := i + 7
		if end > len(days) {
			end = len(days)
		}
		wb := cashflowBucket{Date: days[i].Date}
		for _, d := range days[i:end] {
			wb.ChargeCount += d.ChargeCount
			wb.ChargeIn += d.ChargeIn
			wb.PayoutCount += d.PayoutCount
			wb.PayoutOut += d.PayoutOut
		}
		wb.Net = wb.ChargeIn - wb.PayoutOut
		weeks = append(weeks, wb)
	}
	if weeks == nil {
		weeks = []cashflowBucket{}
	}
	return weeks
}
