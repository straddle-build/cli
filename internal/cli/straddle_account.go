// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Per-call Straddle-Account-Id header for Embed platform scoping.
//
// A platform call must name the embedded account it acts on behalf of. The
// generated client applies cfg.Headers to every request, so injecting the
// header through newClient reaches every endpoint command without editing any
// generated per-command file. resolveStraddleAccount runs in the root
// PersistentPreRunE: it decides — from the command's pp:path/pp:method and the
// configured integration type — whether the header is required, forbidden, or
// optional, then resolves the value from --account (per-call override) or the
// sticky current account. See https://docs.straddle.com/guides/embed/api-headers
// and internal/straddleacct. Hand-authored; survives regen.
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/straddle-build/cli/internal/config"
	"github.com/straddle-build/cli/internal/straddleacct"
)

// straddleAccountHeader scopes a platform API call to one embedded account.
const straddleAccountHeader = straddleacct.Header

// resolveStraddleAccount classifies the running command against the configured
// integration type and resolves the Straddle-Account-Id value to send. It
// stashes the result on flags for newClient to apply, or returns an actionable
// usage error when the account is required-but-missing or forbidden-but-given.
func resolveStraddleAccount(cmd *cobra.Command, f *rootFlags) error {
	ctx, err := straddleacct.LoadContext()
	if err != nil {
		return err
	}
	decision := straddleacct.Classify(
		cmd.Annotations["pp:path"],
		cmd.Annotations["pp:method"],
		ctx.IntegrationType,
	)
	value, _, rerr := straddleacct.Resolve(
		decision,
		f.straddleAccount,
		cmd.Flags().Changed("account"),
		ctx.CurrentAccount,
	)
	if rerr != nil {
		return accountPolicyErr(cmd, rerr)
	}
	f.straddleAccountResolved = value
	return nil
}

// accountPolicyErr turns a straddleacct.PolicyError into a usage error with a
// concrete next step. Non-policy errors pass through unchanged.
func accountPolicyErr(cmd *cobra.Command, err error) error {
	var pe *straddleacct.PolicyError
	if !errors.As(err, &pe) {
		return err
	}
	if pe.Reason == "required" {
		name := cmd.Root().Name()
		return usageErr(fmt.Errorf("%s\nset one with '%s use-account <acct_id>', pass --account <acct_id>, or run '%s setup' if you are not a platform",
			pe.Message, name, name))
	}
	return usageErr(err)
}

// applyStraddleAccount injects the resolved Straddle-Account-Id onto the
// request headers. No-op when nothing was resolved.
func applyStraddleAccount(cfg *config.Config, f *rootFlags) {
	if f == nil || f.straddleAccountResolved == "" {
		return
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}
	cfg.Headers[straddleAccountHeader] = f.straddleAccountResolved
}
