// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newChargesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "charges",
		Short:  "Charges represent attempts to debit money from a customer's bank account using a Paykey. Each charge includes...",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newChargesCreateCmd(flags))
	cmd.AddCommand(newChargesGetCmd(flags))
	cmd.AddCommand(newChargesUpdateCmd(flags))
	cmd.AddCommand(newChargesCancelCmd(flags))
	cmd.AddCommand(newChargesHoldCmd(flags))
	cmd.AddCommand(newChargesReleaseCmd(flags))
	cmd.AddCommand(newChargesResubmitCmd(flags))
	cmd.AddCommand(newChargesUnmaskCmd(flags))
	return cmd
}
