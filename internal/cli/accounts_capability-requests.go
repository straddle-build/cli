// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newAccountsCapabilityRequestsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capability-requests",
		Short: "Capabilities enable specific features and services for an Account. Use capability requests to unlock higher...",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newAccountsCapabilityRequestsCreateCmd(flags))
	cmd.AddCommand(newAccountsCapabilityRequestsListCmd(flags))
	return cmd
}
