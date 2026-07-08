// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/straddle-build/cli/internal/client"
)

func normalizeRawAPIMethod(method string) (string, bool) {
	switch strings.ToUpper(method) {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return strings.ToUpper(method), true
	default:
		return "", false
	}
}

func runAPIPassthrough(cmd *cobra.Command, flags *rootFlags, method string, args []string) error {
	if len(args) != 1 {
		return usageErr(fmt.Errorf("straddle api %s requires exactly one path argument", strings.ToLower(method)))
	}
	path := args[0]
	if !strings.HasPrefix(path, "/") {
		return usageErr(fmt.Errorf("api path %q must start with /", path))
	}

	paramTokens, err := cmd.Flags().GetStringArray("param")
	if err != nil {
		return err
	}
	params, err := parseAPIKeyValueTokens("param", paramTokens)
	if err != nil {
		return err
	}
	headerTokens, err := cmd.Flags().GetStringArray("header")
	if err != nil {
		return err
	}
	headers, err := parseAPIKeyValueTokens("header", headerTokens)
	if err != nil {
		return err
	}
	if err := validateAPIPassthroughHeaders(headers); err != nil {
		return err
	}
	stdinBody, err := cmd.Flags().GetBool("stdin")
	if err != nil {
		return err
	}

	var body any
	if stdinBody {
		if method == "GET" || method == "DELETE" {
			return usageErr(fmt.Errorf("--stdin is only supported with POST, PUT, and PATCH raw API calls"))
		}
		body, err = readAPIPassthroughBody(cmd)
		if err != nil {
			return err
		}
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}

	data, status, err := callRawAPI(c, method, path, params, headers, body)
	if err != nil {
		if method == "DELETE" {
			return classifyDeleteError(err, flags)
		}
		return classifyAPIError(err, flags)
	}

	var partialFailure *partialFailureReport
	if method != "GET" && !flags.dryRun && status >= 200 && status < 300 {
		partialFailure = detectPartialFailure(data)
	}

	if shouldPrintAPIPassthroughEnvelope(cmd, flags) {
		if flags.quiet {
			return nil
		}
		if err := printAPIPassthroughEnvelope(cmd, flags, method, path, status, data, partialFailure); err != nil {
			return err
		}
		if partialFailure != nil && !flags.allowPartialFailure {
			return partialFailureErr(fmt.Errorf("partial failure in raw API response: %s", partialFailure.Message))
		}
		return nil
	}

	if err := printOutputWithFlags(cmd.OutOrStdout(), data, flags); err != nil {
		return err
	}
	if partialFailure != nil && !flags.allowPartialFailure {
		return partialFailureErr(fmt.Errorf("partial failure in raw API response: %s", partialFailure.Message))
	}
	return nil
}

func parseAPIKeyValueTokens(flagName string, tokens []string) (map[string]string, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	values := make(map[string]string, len(tokens))
	for _, token := range tokens {
		key, value, ok := strings.Cut(token, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, usageErr(fmt.Errorf("invalid --%s %q: expected key=value", flagName, token))
		}
		values[key] = value
	}
	return values, nil
}

func validateAPIPassthroughHeaders(headers map[string]string) error {
	for key := range headers {
		if strings.EqualFold(strings.TrimSpace(key), straddleAccountHeader) {
			return usageErr(fmt.Errorf("raw --header %s is reserved for account scoping; use --account <acct_id>", straddleAccountHeader))
		}
	}
	return nil
}

func readAPIPassthroughBody(cmd *cobra.Command) (json.RawMessage, error) {
	stdinData, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	var parsed any
	if err := json.Unmarshal(stdinData, &parsed); err != nil {
		return nil, usageErr(fmt.Errorf("parsing stdin JSON: %w", err))
	}
	return json.RawMessage(stdinData), nil
}

func callRawAPI(c *client.Client, method, path string, params, headers map[string]string, body any) (json.RawMessage, int, error) {
	switch method {
	case "GET":
		data, err := c.GetWithHeaders(path, params, headers)
		return data, 0, err
	case "POST":
		return c.PostWithParamsAndHeaders(path, params, body, headers)
	case "PUT":
		return c.PutWithParamsAndHeaders(path, params, body, headers)
	case "PATCH":
		return c.PatchWithParamsAndHeaders(path, params, body, headers)
	case "DELETE":
		return c.DeleteWithParamsAndHeaders(path, params, headers)
	default:
		return nil, 0, usageErr(fmt.Errorf("unsupported API method %q", method))
	}
}

func shouldPrintAPIPassthroughEnvelope(cmd *cobra.Command, flags *rootFlags) bool {
	return flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain)
}

func printAPIPassthroughEnvelope(cmd *cobra.Command, flags *rootFlags, method, path string, status int, data json.RawMessage, partialFailure *partialFailureReport) error {
	envelope := map[string]any{
		"method":  method,
		"path":    path,
		"success": status == 0 || (status >= 200 && status < 300 && (partialFailure == nil || flags.allowPartialFailure)),
	}
	if status != 0 {
		envelope["status"] = status
	}
	if partialFailure != nil {
		envelope["partial_failure"] = partialFailure
	}
	if flags.dryRun {
		envelope["dry_run"] = true
		envelope["success"] = false
	}
	if verifyNoop := isVerifyNoopBody(data); verifyNoop {
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

func isVerifyNoopBody(data json.RawMessage) bool {
	if len(data) == 0 {
		return false
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return false
	}
	verifyNoop, _ := parsed["__straddle_verify_synthetic__"].(bool)
	return verifyNoop
}
