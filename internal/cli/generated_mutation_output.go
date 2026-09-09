// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func printGeneratedMutationOutput(cmd *cobra.Command, flags *rootFlags, method, endpoint, path string, status int, data json.RawMessage) error {
	resource := cmd.Annotations["straddle:resource"]
	if resource == "" {
		resource = generatedMutationLabel(endpoint)
	}
	partialFailure := generatedMutationPartialFailure(flags, status, data, resource)
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		var items []map[string]any
		if json.Unmarshal(data, &items) != nil || len(items) == 0 {
			var wrapped struct {
				Data []map[string]any `json:"data"`
			}
			if json.Unmarshal(data, &wrapped) == nil {
				items = wrapped.Data
			}
		}
		if len(items) != 0 {
			if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
				fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
			} else {
				return generatedMutationPartialFailureErr(flags, resource, partialFailure)
			}
		}
	}
	if shouldPrintMutationEnvelope(cmd, flags) {
		if flags.quiet {
			return generatedMutationPartialFailureErr(flags, resource, partialFailure)
		}
		if err := printGeneratedMutationEnvelope(cmd, flags, method, resource, path, status, data, partialFailure); err != nil {
			return err
		}
		return generatedMutationPartialFailureErr(flags, resource, partialFailure)
	}
	if err := printOutputWithFlags(cmd.OutOrStdout(), data, flags); err != nil {
		return err
	}
	return generatedMutationPartialFailureErr(flags, resource, partialFailure)
}

func shouldPrintMutationEnvelope(cmd *cobra.Command, flags *rootFlags) bool {
	return flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain)
}

func generatedMutationPartialFailure(flags *rootFlags, status int, data json.RawMessage, resource string) *partialFailureReport {
	if flags.dryRun || status < 200 || status >= 300 {
		return nil
	}
	pf := detectPartialFailure(data)
	if pf == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "warning: partial failure detected in %s response: %s\n", resource, pf.Message)
	if len(pf.ResourceNames) > 0 {
		fmt.Fprintf(os.Stderr, "         succeeded: %d operation(s)\n", len(pf.ResourceNames))
	}
	return pf
}

func generatedMutationPartialFailureErr(flags *rootFlags, resource string, partialFailure *partialFailureReport) error {
	if partialFailure == nil || flags.allowPartialFailure {
		return nil
	}
	return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", resource, partialFailure.Message))
}

func printGeneratedMutationEnvelope(cmd *cobra.Command, flags *rootFlags, method, resource, path string, status int, data json.RawMessage, partialFailure *partialFailureReport) error {
	action := cmd.Annotations["straddle:action"]
	if action == "" {
		action = strings.ToLower(method)
	}
	envelope := map[string]any{
		"action":   action,
		"resource": resource,
		"path":     path,
		"status":   status,
		"success":  status >= 200 && status < 300 && partialFailure == nil,
	}
	if partialFailure != nil {
		envelope["partial_failure"] = partialFailure
	}
	if flags.dryRun {
		envelope["dry_run"] = true
		envelope["status"] = 0
		envelope["success"] = false
	}
	if isVerifyNoopBody(data) {
		envelope["verify_noop"] = true
		envelope["success"] = false
	}

	filtered := data
	if flags.selectFields != "" {
		filtered = filterFields(filtered, flags.selectFields)
	} else if flags.compact {
		filtered = compactFields(filtered)
	}
	if len(filtered) > 0 {
		var parsed any
		if err := json.Unmarshal(filtered, &parsed); err == nil {
			envelope["data"] = parsed
		} else {
			envelope["data"] = string(filtered)
		}
	}

	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
}

func generatedMutationActionResource(method, endpoint string) (string, string) {
	action := strings.ToLower(method)
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return action, "endpoint"
	}
	if dot := strings.LastIndex(endpoint, "."); dot >= 0 {
		if resource := strings.TrimSpace(endpoint[:dot]); resource != "" {
			return action, resource
		}
	}
	return action, endpoint
}

func generatedMutationLabel(endpoint string) string {
	_, resource := generatedMutationActionResource("", endpoint)
	return resource
}
