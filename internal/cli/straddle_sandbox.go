// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: sandbox outcome reference. Prints Straddle's deterministic
// sandbox_outcome values and test bank values so test scenarios are scriptable
// without scraping the docs mid-task. Curated static reference, read-only.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Source: https://docs.straddle.com/guides/resources/sandbox-paybybank
// These are Straddle's published sandbox simulation contract. Pass the value on
// a create call's config.sandbox_outcome (e.g. charges/payouts create
// --config-sandbox-outcome <value>) to force a deterministic result in sandbox.
type sandboxOutcome struct {
	Value       string `json:"value"`
	Description string `json:"description"`
	Code        string `json:"code,omitempty"`
}

type sandboxReference struct {
	Customers      []sandboxOutcome  `json:"customers"`
	Paykeys        []sandboxOutcome  `json:"paykeys"`
	ChargesPayouts []sandboxOutcome  `json:"charges_payouts"`
	TestBank       map[string]string `json:"test_bank"`
	Notes          []string          `json:"notes"`
}

func straddleSandboxReference() sandboxReference {
	return sandboxReference{
		Customers: []sandboxOutcome{
			{Value: "standard", Description: "Customer undergoes normal review process"},
			{Value: "verified", Description: "Customer automatically becomes verified"},
			{Value: "rejected", Description: "Customer automatically gets rejected"},
			{Value: "review", Description: "Customer enters manual review status"},
		},
		Paykeys: []sandboxOutcome{
			{Value: "standard", Description: "Paykey follows normal review process"},
			{Value: "active", Description: "Paykey becomes immediately active"},
			{Value: "rejected", Description: "Paykey gets rejected"},
			{Value: "review", Description: "Paykey requires manual review"},
		},
		ChargesPayouts: []sandboxOutcome{
			{Value: "standard", Description: "Normal processing through standard windows"},
			{Value: "paid", Description: "Transitions to paid within minutes"},
			{Value: "on_hold_daily_limit", Description: "Held due to daily limits; can be released"},
			{Value: "cancelled_for_fraud_risk", Description: "Cancelled for fraud detection (terminal)"},
			{Value: "cancelled_for_balance_check", Description: "Cancelled due to balance check (charges only, terminal)"},
			{Value: "failed_insufficient_funds", Description: "Fails before funding due to NSF", Code: "R01"},
			{Value: "failed_customer_dispute", Description: "Fails before funding due to dispute", Code: "R05"},
			{Value: "failed_closed_bank_account", Description: "Fails before funding due to closed account", Code: "R02"},
			{Value: "reversed_insufficient_funds", Description: "Paid then reversed for NSF", Code: "R01"},
			{Value: "reversed_customer_dispute", Description: "Paid then reversed for dispute", Code: "R05"},
			{Value: "reversed_closed_bank_account", Description: "Paid then reversed for closed account", Code: "R02"},
		},
		TestBank: map[string]string{
			"routing_number": "021000021",
			"account_number": "123456789",
			"account_type":   "checking",
		},
		Notes: []string{
			"Set config.sandbox_outcome on the create call (charges/payouts: --config-sandbox-outcome).",
			"Sandbox processes simulated payments roughly every minute, preserving async state transitions.",
			"Reversal outcomes are ideal for testing reconciliation (paid then returned).",
			"Sandbox keys only work against sandbox.straddle.com; set --environment sandbox (the default).",
		},
	}
}

func newSandboxCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sandbox [topic]",
		Short:       "Reference: deterministic sandbox_outcome values and test bank",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: "Print Straddle's sandbox simulation contract: the config.sandbox_outcome\n" +
			"values for customers, paykeys, charges, and payouts, plus the sandbox test\n" +
			"bank values. Optional topic: outcomes | bank | all (default all).",
		Example: "  straddle sandbox outcomes --json\n" +
			"  straddle sandbox bank",
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := "all"
			if len(args) > 0 {
				topic = args[0]
			}
			switch topic {
			case "all", "outcomes", "bank":
			default:
				return usageErr(fmt.Errorf("topic must be one of: all, outcomes, bank (got %q)", topic))
			}
			ref := straddleSandboxReference()

			if straddleWantsJSON(cmd, flags) {
				switch topic {
				case "bank":
					return flags.printJSON(cmd, map[string]any{"test_bank": ref.TestBank})
				case "outcomes":
					return flags.printJSON(cmd, map[string]any{
						"customers": ref.Customers, "paykeys": ref.Paykeys, "charges_payouts": ref.ChargesPayouts,
					})
				default:
					return flags.printJSON(cmd, ref)
				}
			}

			w := cmd.OutOrStdout()
			if topic == "all" || topic == "outcomes" {
				printOutcomeTable(cmd, "Customers (config.sandbox_outcome)", ref.Customers)
				printOutcomeTable(cmd, "Paykeys (config.sandbox_outcome)", ref.Paykeys)
				printOutcomeTable(cmd, "Charges & Payouts (config.sandbox_outcome)", ref.ChargesPayouts)
			}
			if topic == "all" || topic == "bank" {
				fmt.Fprintln(w, "\nSandbox test bank:")
				fmt.Fprintf(w, "  routing_number  %s\n", ref.TestBank["routing_number"])
				fmt.Fprintf(w, "  account_number  %s\n", ref.TestBank["account_number"])
				fmt.Fprintf(w, "  account_type    %s\n", ref.TestBank["account_type"])
			}
			if topic == "all" {
				fmt.Fprintln(w, "\nNotes:")
				for _, n := range ref.Notes {
					fmt.Fprintf(w, "  - %s\n", n)
				}
			}
			return nil
		},
	}
	return cmd
}

func printOutcomeTable(cmd *cobra.Command, title string, outcomes []sandboxOutcome) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "\n%s:\n", title)
	for _, o := range outcomes {
		if o.Code != "" {
			fmt.Fprintf(w, "  %-30s %-4s %s\n", o.Value, o.Code, o.Description)
		} else {
			fmt.Fprintf(w, "  %-30s %-4s %s\n", o.Value, "", o.Description)
		}
	}
}
