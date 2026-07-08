// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPayoutsHoldCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hold",
		Short: "Manage hold",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newPayoutsHoldPayoutCmd(flags))
	return cmd
}
