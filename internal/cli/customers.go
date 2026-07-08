// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newCustomersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "customers",
		Short:  "Customers represent the end users who send or receive payments through your integration. Each customer undergoes...",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newCustomersCreateCmd(flags))
	cmd.AddCommand(newCustomersDeleteCmd(flags))
	cmd.AddCommand(newCustomersGetCmd(flags))
	cmd.AddCommand(newCustomersListCmd(flags))
	cmd.AddCommand(newCustomersUpdateCmd(flags))
	cmd.AddCommand(newCustomersRefreshReviewCmd(flags))
	cmd.AddCommand(newCustomersReviewCmd(flags))
	cmd.AddCommand(newCustomersUnmaskedCmd(flags))
	return cmd
}
