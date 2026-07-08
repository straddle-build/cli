// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/straddle-build/cli/internal/apisync"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: gen-endpoint <check|drift|generate> [flags]")
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "drift":
		return runDrift(args[1:], stdout, stderr)
	case "generate":
		return runGenerate(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func runCheck(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	specPath := fs.String("spec", "spec.json", "OpenAPI spec lockfile")
	repo := fs.String("repo", ".", "repository root")
	agent := fs.Bool("agent", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := apisync.CheckSpecAgainstRepo(*specPath, *repo)
	if err != nil {
		return err
	}
	if *agent {
		if err := writeJSON(stdout, result); err != nil {
			return err
		}
	} else {
		writeCheckSummary(stdout, result)
	}
	if !result.OK {
		return fmt.Errorf("endpoint coverage check failed: %d missing, %d extra, %d duplicate groups, %d invalid annotations", len(result.Missing), len(result.Extra), len(result.DuplicateAnnotations), len(result.InvalidAnnotations))
	}
	return nil
}

func runDrift(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("drift", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "", "base OpenAPI spec")
	head := fs.String("head", "", "head OpenAPI spec")
	target := fs.String("target", "", "alias for --head")
	repo := fs.String("repo", ".", "repository root, reserved for workflow callers")
	agent := fs.Bool("agent", false, "emit JSON")
	outPath := fs.String("out", "", "write JSON drift result to this file")
	summaryPath := fs.String("summary", "", "write text summary to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = repo
	if *head == "" && *target != "" {
		*head = *target
	}
	if *base == "" || *head == "" {
		return errors.New("drift requires --base and --head")
	}
	result, err := apisync.DriftSpecs(*base, *head)
	if err != nil {
		return err
	}
	if *outPath != "" {
		if err := writeJSONFile(*outPath, result); err != nil {
			return err
		}
	}
	if *summaryPath != "" {
		if err := os.WriteFile(*summaryPath, []byte(driftSummary(result)), 0o600); err != nil {
			return err
		}
	}
	if *agent {
		return writeJSON(stdout, result)
	}
	_, err = io.WriteString(stdout, driftSummary(result))
	return err
}

func runGenerate(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	specPath := fs.String("spec", "spec.json", "OpenAPI spec to generate from")
	repo := fs.String("repo", ".", "repository root")
	outDir := fs.String("out-dir", "", "directory for generated command files, defaults to internal/cli under --repo")
	driftPath := fs.String("drift", "", "optional drift JSON produced by the drift subcommand")
	only := fs.String("only", "missing", "operation selection: missing, supported-additions, or all")
	supportedAdditions := fs.Bool("supported-additions", false, "alias for --only supported-additions")
	dryRun := fs.Bool("dry-run", false, "show files that would be written without writing them")
	overwrite := fs.Bool("overwrite", false, "overwrite generated files that already exist")
	agent := fs.Bool("agent", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *supportedAdditions {
		*only = "supported-additions"
	}
	ops, err := apisync.LoadSpec(*specPath)
	if err != nil {
		return err
	}
	inventory, err := apisync.InventoryRepo(*repo)
	if err != nil {
		return err
	}
	selection, unsupported, err := selectOperations(*only, *driftPath, ops, inventory)
	if err != nil {
		return err
	}
	resolvedOut := *outDir
	if resolvedOut == "" {
		resolvedOut = filepath.Join(*repo, "internal", "cli")
	} else if !filepath.IsAbs(resolvedOut) {
		resolvedOut = filepath.Join(*repo, resolvedOut)
	}
	files := make([]apisync.GeneratedFile, 0, len(selection))
	for _, op := range selection {
		file, err := apisync.GenerateEndpointFile(op, resolvedOut)
		if err != nil {
			return fmt.Errorf("generating %s: %w", op.Key, err)
		}
		files = append(files, file)
	}
	if unsupported == nil {
		unsupported = []apisync.UnsupportedOperation{}
	}
	result := apisync.GenerateResult{
		Generated:             []string{},
		SkippedExisting:       []string{},
		UnsupportedOperations: unsupported,
		DryRun:                *dryRun,
	}
	if *dryRun {
		for _, file := range files {
			result.Generated = append(result.Generated, file.Path)
		}
	} else {
		written, err := apisync.WriteGeneratedFiles(files, *overwrite)
		if err != nil {
			return err
		}
		result.Generated = written.Generated
		result.SkippedExisting = written.SkippedExisting
		if *only == "supported-additions" {
			if err := requireSupportedAdditionsCovered(selection, *repo, resolvedOut, result.SkippedExisting); err != nil {
				return err
			}
		}
	}
	if *agent {
		return writeJSON(stdout, result)
	}
	writeGenerateSummary(stdout, result)
	return nil
}

func requireSupportedAdditionsCovered(selection []apisync.Operation, repo, outDir string, skipped []string) error {
	var missing []string
	if samePath(outDir, filepath.Join(repo, "internal", "cli")) {
		inventory, err := apisync.InventoryRepo(repo)
		if err != nil {
			return err
		}
		covered := make(map[string]bool, len(inventory.Annotations))
		for _, annotation := range inventory.Annotations {
			covered[apisync.OperationKey(annotation.Method, annotation.Path)] = true
		}
		for _, op := range selection {
			if !covered[op.Key] {
				missing = append(missing, op.Key)
			}
		}
	}
	if len(missing) == 0 && len(skipped) == 0 {
		return nil
	}
	sort.Strings(missing)
	sort.Strings(skipped)
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "missing annotations: "+strings.Join(missing, ", "))
	}
	if len(skipped) > 0 {
		parts = append(parts, "skipped existing files: "+strings.Join(skipped, ", "))
	}
	return fmt.Errorf("supported endpoint generation incomplete: %s", strings.Join(parts, "; "))
}

func samePath(a, b string) bool {
	aAbs, aErr := filepath.Abs(a)
	bAbs, bErr := filepath.Abs(b)
	if aErr == nil && bErr == nil {
		return filepath.Clean(aAbs) == filepath.Clean(bAbs)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func selectOperations(only, driftPath string, ops []apisync.Operation, inventory apisync.Inventory) ([]apisync.Operation, []apisync.UnsupportedOperation, error) {
	switch only {
	case "missing":
		selection, unsupported := apisync.MissingSupportedOperations(ops, inventory)
		return selection, unsupported, nil
	case "supported-additions":
		if driftPath != "" {
			drift, err := apisync.ReadDrift(driftPath)
			if err != nil {
				return nil, nil, err
			}
			return drift.SupportedAdditions, drift.UnsupportedOperations, nil
		}
		selection, unsupported := apisync.MissingSupportedOperations(ops, inventory)
		return selection, unsupported, nil
	case "all":
		var selection []apisync.Operation
		var unsupported []apisync.UnsupportedOperation
		for _, op := range ops {
			if reasons := apisync.UnsupportedReasons(op); len(reasons) > 0 {
				unsupported = append(unsupported, apisync.UnsupportedOperation{Operation: op, Reasons: reasons})
				continue
			}
			selection = append(selection, op)
		}
		return selection, unsupported, nil
	default:
		return nil, nil, fmt.Errorf("unsupported --only value %q", only)
	}
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil && filepath.Dir(path) != "." {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func writeCheckSummary(w io.Writer, result apisync.CheckResult) {
	status := "ok"
	if !result.OK {
		status = "failed"
	}
	fmt.Fprintf(w, "endpoint coverage: %s\n", status)
	fmt.Fprintf(w, "spec_operations: %d\n", result.SpecOperations)
	fmt.Fprintf(w, "annotated_endpoints: %d\n", result.AnnotatedEndpoints)
	fmt.Fprintf(w, "missing: %d\n", len(result.Missing))
	fmt.Fprintf(w, "extra: %d\n", len(result.Extra))
	fmt.Fprintf(w, "duplicate_annotations: %d\n", len(result.DuplicateAnnotations))
	fmt.Fprintf(w, "invalid_annotations: %d\n", len(result.InvalidAnnotations))
}

func driftSummary(result apisync.DriftResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "api drift summary\n")
	fmt.Fprintf(&b, "base_operations: %d\n", result.BaseOperations)
	fmt.Fprintf(&b, "head_operations: %d\n", result.HeadOperations)
	fmt.Fprintf(&b, "supported_additions: %d\n", len(result.SupportedAdditions))
	fmt.Fprintf(&b, "changes: %d\n", len(result.Changes))
	fmt.Fprintf(&b, "removals: %d\n", len(result.Removals))
	fmt.Fprintf(&b, "unsupported_operations: %d\n", len(result.UnsupportedOperations))
	fmt.Fprintf(&b, "no_drift: %t\n", result.NoDrift)
	return b.String()
}

func writeGenerateSummary(w io.Writer, result apisync.GenerateResult) {
	fmt.Fprintf(w, "generated: %d\n", len(result.Generated))
	fmt.Fprintf(w, "skipped_existing: %d\n", len(result.SkippedExisting))
	fmt.Fprintf(w, "unsupported_operations: %d\n", len(result.UnsupportedOperations))
	fmt.Fprintf(w, "dry_run: %t\n", result.DryRun)
}
