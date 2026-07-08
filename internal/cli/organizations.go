// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newOrganizationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "organizations",
		Short:  "Organizations are a powerful feature in Straddle that allow you to manage multiple accounts under a single umbrella....",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newOrganizationsCreateCmd(flags))
	cmd.AddCommand(newOrganizationsGetByIdCmd(flags))
	cmd.AddCommand(newOrganizationsListCmd(flags))
	return cmd
}
