// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
//
// Straddle-Account-Id gating for the in-process MCP paths (code-orchestration
// execute and the endpoint-mirror handler). Mirrors the CLI pre-run so an agent
// gets identical scoping: it sets the acting account once via the use-account
// tool, and every subsequent endpoint call is scoped per the integration type.
// There is no per-call flag on the MCP surface, so the value comes only from
// the sticky platform context. Hand-authored; survives regen.
package mcp

import (
	"errors"
	"fmt"

	"straddle-pp-cli/internal/straddleacct"
)

// applyMCPAccount returns request headers with Straddle-Account-Id injected
// when the integration type and operation call for it. It never mutates the
// passed map (which may be a shared endpoint HeaderOverrides); it copies before
// adding. A required-but-unset account is returned as an error for the tool.
func applyMCPAccount(method, pathTemplate string, headers map[string]string) (map[string]string, error) {
	ctx, err := straddleacct.LoadContext()
	if err != nil {
		return headers, err
	}
	decision := straddleacct.Classify(pathTemplate, method, ctx.IntegrationType)
	value, send, rerr := straddleacct.Resolve(decision, "", false, ctx.CurrentAccount)
	if rerr != nil {
		var pe *straddleacct.PolicyError
		if errors.As(rerr, &pe) && pe.Reason == "required" {
			return headers, fmt.Errorf("%s; set one with the use-account tool", pe.Message)
		}
		return headers, rerr
	}
	if !send {
		return headers, nil
	}
	merged := make(map[string]string, len(headers)+1)
	for k, v := range headers {
		merged[k] = v
	}
	merged[straddleacct.Header] = value
	return merged, nil
}
