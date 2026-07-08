// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newLinkedBankAccountsCancelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Manage cancel",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newLinkedBankAccountsCancelUpdateCmd(flags))
	return cmd
}
