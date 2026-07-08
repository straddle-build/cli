// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestGeneratedEndpointRegistryInstallsExistingAndHiddenParents(t *testing.T) {
	previousRegistrations := generatedEndpointRegistrations
	generatedEndpointRegistrations = nil
	t.Cleanup(func() { generatedEndpointRegistrations = previousRegistrations })

	registerGeneratedEndpoint("accounts.generated-test", func(flags *rootFlags) *cobra.Command {
		return &cobra.Command{Use: "generated-test", Short: "Generated test endpoint"}
	})
	registerGeneratedEndpoint("widgets.create", func(flags *rootFlags) *cobra.Command {
		return &cobra.Command{Use: "create", Short: "Create widget"}
	})

	root := RootCmd()

	accounts := generatedTestChild(root, "accounts")
	if accounts == nil {
		t.Fatal("accounts parent command not found")
	}
	if generatedTestChild(accounts, "generated-test") == nil {
		t.Fatal("generated endpoint was not installed under existing accounts parent")
	}

	widgets := generatedTestChild(root, "widgets")
	if widgets == nil {
		t.Fatal("generated widgets parent command not found")
	}
	if !widgets.Hidden {
		t.Fatal("generated widgets parent is not hidden")
	}
	if generatedTestChild(widgets, "create") == nil {
		t.Fatal("generated endpoint was not installed under generated widgets parent")
	}
}

func generatedTestChild(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}
