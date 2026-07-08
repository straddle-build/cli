// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newAccountsSimulateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "simulate",
		Short: "Manage simulate",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newAccountsSimulateCreateCmd(flags))
	return cmd
}
