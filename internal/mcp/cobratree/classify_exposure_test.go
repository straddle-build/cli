// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package cobratree

import (
	"testing"

	"github.com/spf13/cobra"
)

// The platform-setup commands must reach agents: the walker registers
// commandNovel commands as MCP tools, so an agent can set the acting account
// once (use-account) and have every later endpoint call scoped to it.
func TestClassifyExposesSetupCommands(t *testing.T) {
	for _, name := range []string{"use-account", "setup"} {
		if got := classify(&cobra.Command{Use: name}); got != commandNovel {
			t.Errorf("classify(%q) = %v, want commandNovel (must be agent-exposed)", name, got)
		}
	}
	if got := classify(&cobra.Command{Use: "doctor"}); got != commandFramework {
		t.Errorf("classify(doctor) = %v, want commandFramework", got)
	}
	ep := &cobra.Command{Use: "create", Annotations: map[string]string{EndpointAnnotation: "charges.create"}}
	if got := classify(ep); got != commandEndpoint {
		t.Errorf("classify(endpoint) = %v, want commandEndpoint", got)
	}
}
