// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPayoutsResubmitCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resubmit",
		Short: "Manage resubmit",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newPayoutsResubmitCreateCmd(flags))
	return cmd
}
