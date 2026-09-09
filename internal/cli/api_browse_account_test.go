// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Tests for the `straddle api` browse forms and account-scope policy.
//
// The `api` command has two local-only browse forms — `straddle api` (list all
// hidden interfaces) and `straddle api <interface>` (list an interface's
// methods) — that print from the in-memory cobra tree and make no network
// call. Account-scope policy has nothing to gate on them, so an explicit
// --account flag must be silently ignored (matching sticky/saved-profile
// behavior) rather than raised as a hard usage error. See straddle_account.go
// resolveStraddleAccount and api_discovery.go's RunE.
package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/straddle-build/straddle-cli/internal/straddleacct"
)

// newAccountAPITestCmd builds a bare `api` cobra command (no straddle:path /
// straddle:method annotations) bound to the account flag, mirroring the real
// newAPICmd shape that resolveStraddleAccount sees.
func newAccountAPITestCmd() (*cobra.Command, *rootFlags) {
	f := &rootFlags{}
	cmd := &cobra.Command{
		Use: "api",
		RunE: func(*cobra.Command, []string) error {
			return nil
		},
	}
	cmd.Flags().StringVar(&f.straddleAccount, "account", "", "embedded account scope")
	return cmd, f
}

// setAccountFlag sets --account on the command and marks it changed, exactly as
// cobra does when the user passes it on the command line.
func setAccountFlag(t *testing.T, cmd *cobra.Command, value string) {
	t.Helper()
	if err := cmd.Flags().Set("account", value); err != nil {
		t.Fatalf("set --account: %v", err)
	}
}

func isolateAccountPlatform(t *testing.T, ctx straddleacct.Context) {
	t.Helper()
	isolateAPIConfig(t)
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(t.TempDir(), "platform.toml"))
	if err := straddleacct.SaveContext(ctx); err != nil {
		t.Fatalf("save platform context: %v", err)
	}
}

// TestResolveStraddleAccountAPIBrowseSkipsPolicy exercises resolveStraddleAccount
// directly: the browse forms must bypass Classify/Resolve entirely so an
// explicit --account is silently dropped, leaving straddleAccountResolved empty.
func TestResolveStraddleAccountAPIBrowseSkipsPolicy(t *testing.T) {
	isolateAccountPlatform(t, straddleacct.Context{
		IntegrationType: straddleacct.TypeSaaS,
	})

	browseArgs := [][]string{
		nil,                // `straddle api`
		{"charges"},        // `straddle api <interface>`
		{"get"},            // method with no path → runAPIPassthrough usage error, but local for policy
		{"bogus"},          // non-method single arg → browse "interface not found", local for policy
		{"foobar", "x"},    // 2 args, non-method first → local for policy
		{"get", "charges"}, // 2 args, method + non-path → runAPIPassthrough usage error, local for policy
	}
	for _, args := range browseArgs {
		cmd, f := newAccountAPITestCmd()
		setAccountFlag(t, cmd, "acct_flag")
		if err := resolveStraddleAccount(cmd, f, args); err != nil {
			t.Errorf("browse args=%v: resolveStraddleAccount error = %v, want nil (policy must be skipped)", args, err)
			continue
		}
		if f.straddleAccountResolved != "" {
			t.Errorf("browse args=%v: straddleAccountResolved = %q, want \"\" (no header on a no-network browse)", args, f.straddleAccountResolved)
		}
	}
}

// TestAPIBrowseAcceptsAccountFlag is the primary regression test for the bug
// report: the browse forms must print the listing (no network call) when an
// explicit non-empty --account is supplied, across every integration type.
func TestAPIBrowseAcceptsAccountFlag(t *testing.T) {
	for _, integration := range []string{straddleacct.TypeSaaS, straddleacct.TypeMarketplace, straddleacct.TypeAccount} {
		t.Run(integration, func(t *testing.T) {
			isolateAccountPlatform(t, straddleacct.Context{IntegrationType: integration})

			// `straddle --account acct_x api` → list all interfaces.
			stdout, _, err := runRootForAPITest(t, []string{"--account", "acct_x", "api"}, "")
			if err != nil {
				t.Fatalf("api browse with --account returned error: %v", err)
			}
			if !strings.Contains(stdout, "Available API interfaces") || !strings.Contains(stdout, "charges") {
				t.Fatalf("api browse output missing expected listing; got: %s", stdout)
			}

			// `straddle --account acct_x api charges` → list interface methods.
			stdout, _, err = runRootForAPITest(t, []string{"--account", "acct_x", "api", "charges"}, "")
			if err != nil {
				t.Fatalf("api <interface> browse with --account returned error: %v", err)
			}
			if !strings.Contains(stdout, "Methods:") || !strings.Contains(stdout, "charges create") {
				t.Fatalf("api <interface> output missing methods; got: %s", stdout)
			}
		})
	}
}

// TestAPIBrowseAccountFlagEqualAcrossMechanisms proves the asymmetry is fixed:
// the identical listing is produced whether the account is supplied via an
// explicit --account flag, via the sticky current_account, or omitted
// entirely. The listing is a pure function of the cobra tree and must not
// depend on how (or whether) an account was named.
func TestAPIBrowseAccountFlagEqualAcrossMechanisms(t *testing.T) {
	isolateAccountPlatform(t, straddleacct.Context{
		IntegrationType: straddleacct.TypeSaaS,
		CurrentAccount:  "acct_sticky",
	})

	// Sticky account in platform config, no --account flag.
	stickyStdout, _, err := runRootForAPITest(t, []string{"api", "charges"}, "")
	if err != nil {
		t.Fatalf("sticky api charges: %v", err)
	}

	// Explicit --account flag naming the same logical account.
	flagStdout, _, err := runRootForAPITest(t, []string{"--account", "acct_flag", "api", "charges"}, "")
	if err != nil {
		t.Fatalf("flag api charges: %v", err)
	}

	// No account at all.
	noneStdout, _, err := runRootForAPITest(t, []string{"api", "charges"}, "")
	if err != nil {
		t.Fatalf("no-account api charges: %v", err)
	}

	if stickyStdout != flagStdout {
		t.Fatalf("sticky vs flag output differ.\nsticky:\n%s\nflag:\n%s", stickyStdout, flagStdout)
	}
	if stickyStdout != noneStdout {
		t.Fatalf("sticky vs no-account output differ.\nsticky:\n%s\nnone:\n%s", stickyStdout, noneStdout)
	}
}

// TestAPIRawCallForbiddenAccountFlagStillErrors is the over-broad-fix guard:
// the raw-API form (`api <HTTP_METHOD> /path`) is NOT a browse and must keep
// enforcing policy. Under the account integration type, /v1/charges is Forbid,
// so an explicit --account must still be a hard usage error with exit code 2.
func TestAPIRawCallForbiddenAccountFlagStillErrors(t *testing.T) {
	isolateAccountPlatform(t, straddleacct.Context{IntegrationType: straddleacct.TypeAccount})

	_, _, err := runRootForAPITest(t, []string{"--account", "acct_x", "api", "get", "/v1/charges"}, "")
	if err == nil {
		t.Fatal("raw api get /v1/charges with --account under account type: error = nil, want forbidden error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode = %d, want 2", got)
	}
	if !strings.Contains(err.Error(), "remove --account") {
		t.Fatalf("error = %q, want it to contain \"remove --account\"", err.Error())
	}
}
