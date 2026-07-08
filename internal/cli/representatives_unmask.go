// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newRepresentativesUnmaskCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unmask",
		Short: "Manage unmask",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newRepresentativesUnmaskGetCmd(flags))
	return cmd
}
