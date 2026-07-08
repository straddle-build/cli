// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPaykeysReviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Manage review",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newPaykeysReviewGetCmd(flags))
	cmd.AddCommand(newPaykeysReviewUpdateCmd(flags))
	return cmd
}
