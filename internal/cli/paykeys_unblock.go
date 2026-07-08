// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPaykeysUnblockCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unblock",
		Short: "Manage unblock",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newPaykeysUnblockUpdateCmd(flags))
	return cmd
}
