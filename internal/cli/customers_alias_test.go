// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestCustomersAliasOverlaysNoDuplicate scans every installed
// customers.* endpoint command and asserts that none of its aliases equals
// its own canonical Name(). This is the generalized regression guard for the
// endpointUses/overlay-alias collision class: cobra's NameAndAliases() prepends
// Name() to Aliases without dedup, so an overlay alias matching the
// endpointUses-assigned command name renders the canonical name twice in
// --help. Any future customer overlay (hand-edited or refreshed by api-sync)
// that reintroduces such a collision is caught here. Byte-locked golden help
// fixtures render the expected output but cannot express this semantic
// invariant, so a new colliding alias locked in via 'go test -update' would
// pass the golden suite while still being wrong.
//
// wantNA also spot-checks the known-good NameAndAliases() strings, including
// the control case customers.refresh-customer-review (renamed via endpointUses
// to "update" but declaring no overlay aliases), which must keep producing
// NameAndAliases()=="update" with no duplicate.
func TestCustomersAliasOverlaysNoDuplicate(t *testing.T) {
	setGoldenEnvironment(t)
	root := RootCmd()
	customers, _, err := root.Find([]string{"customers"})
	if err != nil || customers == nil {
		t.Fatalf("customers command not found: %v", err)
	}

	wantNA := map[string]string{
		"customers.set-customer-verification-decision": "update-customer, update",
		"customers.get-customer-review":                "get-customer, get",
		"customers.get-unmasked-customer":              "get-customer, get",
		"customers.refresh-customer-review":            "update",
	}

	var audited int
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		endpoint := cmd.Annotations["straddle:endpoint"]
		if strings.HasPrefix(endpoint, "customers.") {
			audited++
			if want, ok := wantNA[endpoint]; ok && cmd.NameAndAliases() != want {
				t.Errorf("endpoint %s: NameAndAliases() = %q, want %q", endpoint, cmd.NameAndAliases(), want)
			}
			for _, a := range cmd.Aliases {
				if a == cmd.Name() {
					t.Errorf("endpoint %s: alias %q duplicates canonical name %q (cobra would render Name() twice in --help)",
						endpoint, a, cmd.Name())
				}
			}
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(customers)

	if audited == 0 {
		t.Fatal("no customers.* endpoint commands were audited")
	}
}
