// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newRepresentativesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "representatives",
		Short:  "Representatives are individuals who have legal authority or significant responsibility within a business entity...",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newRepresentativesCreateCmd(flags))
	cmd.AddCommand(newRepresentativesGetCmd(flags))
	cmd.AddCommand(newRepresentativesListCmd(flags))
	cmd.AddCommand(newRepresentativesUpdateCmd(flags))
	cmd.AddCommand(newRepresentativesUnmaskCmd(flags))
	return cmd
}
