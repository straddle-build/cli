// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPaykeysRefreshBalanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh-balance",
		Short: "Manage refresh balance",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newPaykeysRefreshBalanceUpdateCmd(flags))
	return cmd
}
