// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newChargesReleaseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Manage release",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newChargesReleaseChargeCmd(flags))
	return cmd
}
