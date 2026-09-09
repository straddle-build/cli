// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/straddle-build/straddle-cli/internal/surface"
)

// Overlays can require a deliberate input where a contract default would change
// business state. Stdin replaces body flags, so its value must stand on its own.
func (b *surfaceFlagBinding) validateExplicitInput(cmd *cobra.Command, body any, stdin bool) error {
	flag := cmd.Flags().Lookup(b.definition.Name)
	if flag == nil || len(flag.Annotations["straddle:explicit"]) == 0 {
		return nil
	}
	if !stdin || b.definition.In != surface.InBody {
		if !flag.Changed {
			return fmt.Errorf("required flag %q must be set explicitly", b.definition.Name)
		}
		return nil
	}
	value := body
	for _, part := range strings.Split(strings.TrimPrefix(b.definition.Key, "/"), "/") {
		object, ok := value.(map[string]any)
		if !ok {
			value = nil
			break
		}
		key := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		value = object[key]
	}
	if value == nil {
		return fmt.Errorf("stdin field %q must be set explicitly", b.definition.Name)
	}
	if b.definition.Kind == surface.KindString {
		text, ok := value.(string)
		if !ok || text == "" {
			return fmt.Errorf("stdin field %q must be a nonempty string", b.definition.Name)
		}
		return validateSurfaceEnum(cmd, b.definition, []string{text})
	}
	return nil
}
