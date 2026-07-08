// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPaykeysRefreshReviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh-review",
		Short: "Manage refresh review",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newPaykeysRefreshReviewUpdateCmd(flags))
	return cmd
}
