// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Sticky platform context: the integration type and current acting account.
// Stored in its own file rather than the generated config.toml, whose save()
// marshals a fixed struct and would drop these keys. Hand-authored; survives
// regen.
package straddleacct

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Context is the persisted platform scoping state.
type Context struct {
	IntegrationType string `toml:"integration_type,omitempty"`
	CurrentAccount  string `toml:"current_account,omitempty"`
}

// ValidIntegrationType reports whether s is a recognized integration type.
func ValidIntegrationType(s string) bool {
	switch s {
	case TypeAccount, TypeSaaS, TypeMarketplace:
		return true
	default:
		return false
	}
}

// ContextPath resolves the platform context file. STRADDLE_PLATFORM_CONFIG
// overrides it directly; otherwise it sits beside the main config file (next
// to STRADDLE_CONFIG when set, else ~/.config/straddle/platform.toml),
// so it travels with the rest of the CLI's configuration.
func ContextPath() string {
	if p := os.Getenv("STRADDLE_PLATFORM_CONFIG"); p != "" {
		return p
	}
	if c := os.Getenv("STRADDLE_CONFIG"); c != "" {
		return filepath.Join(filepath.Dir(c), "platform.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "straddle", "platform.toml")
}

// LoadContext reads the platform context. A missing file yields an empty
// Context with no error.
func LoadContext() (Context, error) {
	var ctx Context
	data, err := os.ReadFile(ContextPath())
	if err != nil {
		if os.IsNotExist(err) {
			return ctx, nil
		}
		return ctx, fmt.Errorf("reading platform context: %w", err)
	}
	if err := toml.Unmarshal(data, &ctx); err != nil {
		return Context{}, fmt.Errorf("parsing platform context %s: %w", ContextPath(), err)
	}
	return ctx, nil
}

// SaveContext writes the platform context. An empty integration type is
// allowed (unset); any non-empty value must be valid.
func SaveContext(ctx Context) error {
	if ctx.IntegrationType != "" && !ValidIntegrationType(ctx.IntegrationType) {
		return fmt.Errorf("invalid integration type %q: must be %s, %s, or %s",
			ctx.IntegrationType, TypeAccount, TypeSaaS, TypeMarketplace)
	}
	path := ContextPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := toml.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("marshaling platform context: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}
