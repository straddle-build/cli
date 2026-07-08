// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newPaykeysUnmaskedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unmasked",
		Short: "Manage unmasked",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newPaykeysUnmaskedGetPaykeyCmd(flags))
	return cmd
}
