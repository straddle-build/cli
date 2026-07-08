// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newLinkedBankAccountsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "linked-bank-accounts",
		Short:  "Linked bank accounts connect your platform users' external bank accounts to Straddle for settlements and payment...",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newLinkedBankAccountsCreateCmd(flags))
	cmd.AddCommand(newLinkedBankAccountsGetCmd(flags))
	cmd.AddCommand(newLinkedBankAccountsListCmd(flags))
	cmd.AddCommand(newLinkedBankAccountsUpdateCmd(flags))
	cmd.AddCommand(newLinkedBankAccountsCancelCmd(flags))
	cmd.AddCommand(newLinkedBankAccountsUnmaskCmd(flags))
	return cmd
}
