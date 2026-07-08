// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func printGeneratedMutationOutput(cmd *cobra.Command, flags *rootFlags, method, endpoint, path string, status int, data json.RawMessage) error {
	partialFailure := generatedMutationPartialFailure(flags, status, data)
	if shouldPrintMutationEnvelope(cmd, flags) {
		if flags.quiet {
			return generatedMutationPartialFailureErr(flags, endpoint, partialFailure)
		}
		if err := printGeneratedMutationEnvelope(cmd, flags, method, endpoint, path, status, data, partialFailure); err != nil {
			return err
		}
		return generatedMutationPartialFailureErr(flags, endpoint, partialFailure)
	}
	if err := printOutputWithFlags(cmd.OutOrStdout(), data, flags); err != nil {
		return err
	}
	return generatedMutationPartialFailureErr(flags, endpoint, partialFailure)
}

func shouldPrintMutationEnvelope(cmd *cobra.Command, flags *rootFlags) bool {
	return flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain)
}

func generatedMutationPartialFailure(flags *rootFlags, status int, data json.RawMessage) *partialFailureReport {
	if flags.dryRun || status < 200 || status >= 300 {
		return nil
	}
	return detectPartialFailure(data)
}

func generatedMutationPartialFailureErr(flags *rootFlags, endpoint string, partialFailure *partialFailureReport) error {
	if partialFailure == nil || flags.allowPartialFailure {
		return nil
	}
	return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", generatedMutationLabel(endpoint), partialFailure.Message))
}

func printGeneratedMutationEnvelope(cmd *cobra.Command, flags *rootFlags, method, endpoint, path string, status int, data json.RawMessage, partialFailure *partialFailureReport) error {
	action, resource := generatedMutationActionResource(method, endpoint)
	envelope := map[string]any{
		"action":   action,
		"resource": resource,
		"path":     path,
		"status":   status,
		"success":  status >= 200 && status < 300 && (partialFailure == nil || flags.allowPartialFailure),
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
	endpoint = strings.TrimSpace(endpoint)
	action := strings.ToLower(method)
	resource := "endpoint"
	if endpoint == "" {
		return action, resource
	}
	if dot := strings.LastIndex(endpoint, "."); dot >= 0 {
		if left := strings.TrimSpace(endpoint[:dot]); left != "" {
			resource = left
		}
		if right := strings.TrimSpace(endpoint[dot+1:]); right != "" {
			action = right
		}
		return action, resource
	}
	return action, endpoint
}

func generatedMutationLabel(endpoint string) string {
	_, resource := generatedMutationActionResource("", endpoint)
	return resource
}
