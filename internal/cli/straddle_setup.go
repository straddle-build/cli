// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Platform setup commands: declare the integration type once (`setup`) and
// switch the current acting account (`use-account`). Both persist to the
// straddleacct platform context that drives Straddle-Account-Id gating. The CLI
// never prompts (see Execute); the conversational walkthrough lives in the
// companion skill, which calls `setup --type ...`. Hand-authored; survives
// regen.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"straddle-pp-cli/internal/straddleacct"
)

func newSetupCmd(flags *rootFlags) *cobra.Command {
	var typ string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set your Straddle integration type (account, saas, or marketplace)",
		Long: `Declare how your integration uses Straddle. This determines when the
Straddle-Account-Id header is sent on platform calls.

  account      A single business sending/collecting its own payments.
  saas         Software with embedded payments; your clients own their customers.
  marketplace  A platform connecting buyers with sellers; the platform owns customers.

Run without --type to print the current setting. Platforms then pick an acting
account with 'use-account <acct_id>'.`,
		Example: "  straddle-pp-cli setup --type saas",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := straddleacct.LoadContext()
			if err != nil {
				return err
			}
			if typ == "" {
				return printPlatformContext(cmd, flags, ctx)
			}
			if !straddleacct.ValidIntegrationType(typ) {
				return usageErr(fmt.Errorf("invalid --type %q: must be %s, %s, or %s",
					typ, straddleacct.TypeAccount, straddleacct.TypeSaaS, straddleacct.TypeMarketplace))
			}
			ctx.IntegrationType = typ
			if err := straddleacct.SaveContext(ctx); err != nil {
				return err
			}
			if !flags.asJSON && typ != straddleacct.TypeAccount && ctx.CurrentAccount == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "next: pick an acting account with 'use-account <acct_id>'")
			}
			return printPlatformContext(cmd, flags, ctx)
		},
	}
	cmd.Flags().StringVar(&typ, "type", "", "Integration type: account, saas, or marketplace")
	return cmd
}

func newUseAccountCmd(flags *rootFlags) *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "use-account [acct_id]",
		Short: "Set the current embedded account for platform calls (sticky until changed)",
		Long: `Set the embedded account that platform calls act on behalf of, sent as the
Straddle-Account-Id header. It sticks until you change it; --account on any
command overrides it for that one call.

Run without an id to show the current account. Use --clear to unset it.`,
		Example: "  straddle-pp-cli use-account acct_01h...",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := straddleacct.LoadContext()
			if err != nil {
				return err
			}
			switch {
			case clear:
				ctx.CurrentAccount = ""
			case len(args) == 1:
				ctx.CurrentAccount = args[0]
			default:
				return printPlatformContext(cmd, flags, ctx)
			}
			if err := straddleacct.SaveContext(ctx); err != nil {
				return err
			}
			return printPlatformContext(cmd, flags, ctx)
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear the current account")
	return cmd
}

// printPlatformContext renders the current integration type and acting account,
// as JSON under --json or a short human summary otherwise.
func printPlatformContext(cmd *cobra.Command, f *rootFlags, ctx straddleacct.Context) error {
	if f.asJSON {
		return f.printJSON(cmd, map[string]any{
			"integration_type": ctx.IntegrationType,
			"current_account":  ctx.CurrentAccount,
		})
	}
	it := ctx.IntegrationType
	if it == "" {
		it = "(not set — run 'setup --type account|saas|marketplace')"
	}
	acct := ctx.CurrentAccount
	if acct == "" {
		acct = "(none)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "integration type: %s\ncurrent account:  %s\n", it, acct)
	return nil
}
