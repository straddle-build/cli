// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newAccountsOnboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Manage onboard",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newAccountsOnboardAccountCmd(flags))
	return cmd
}
