// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newAPICmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api [interface]|<method> <path>",
		Short: "Browse API endpoints or call a raw API path",
		Long: `Browse and call any API endpoint using the raw interface names.

The friendly top-level commands cover the most common operations.
This command provides access to ALL endpoints for power users and
agents that need full API coverage.

Run 'api' with no arguments to list all interfaces.
Run 'api <interface>' to see that interface's methods.
Run 'api <method> <path>' to call a raw API path when <method> is GET,
POST, PUT, PATCH, or DELETE.`,
		Example: `  # List all available interfaces
  straddle api

  # Show methods for a specific interface
  straddle api <interface-name>

  # Call a raw API path
  straddle api get /v1/charges --param limit=10
  echo '{}' | straddle api post /v1/charges --stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			if len(args) > 0 {
				if method, ok := normalizeRawAPIMethod(args[0]); ok {
					return runAPIPassthrough(cmd, flags, method, args[1:])
				}
			}

			if len(args) > 0 {
				target := strings.ToLower(args[0])
				for _, child := range root.Commands() {
					if child.Hidden && strings.ToLower(child.Name()) == target {
						methods := child.Commands()
						// JSON envelope: {interface, short, methods: [{name, short}, ...]}.
						if flags.asJSON {
							methodList := make([]map[string]any, 0, len(methods))
							for _, method := range methods {
								methodList = append(methodList, map[string]any{
									"name":  method.Name(),
									"short": method.Short,
								})
							}
							return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
								"interface": child.Name(),
								"short":     child.Short,
								"methods":   methodList,
							}, flags)
						}
						if len(methods) == 0 {
							return child.Help()
						}
						fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n\nMethods:\n", child.Name(), child.Short)
						for _, method := range methods {
							fmt.Fprintf(cmd.OutOrStdout(), "  %-50s %s\n", child.Name()+" "+method.Name(), method.Short)
						}
						fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'straddle %s <method> --help' for details.\n", child.Name())
						return nil
					}
				}
				return fmt.Errorf("interface %q not found. Run 'straddle api' to list all interfaces", args[0])
			}

			// Pre-formatting human strings ahead of time would block the JSON
			// path from emitting clean field values; build the typed slice and
			// derive human format on print.
			type ifaceEntry struct {
				Name  string `json:"name"`
				Short string `json:"short"`
			}
			var ifaces []ifaceEntry
			for _, child := range root.Commands() {
				if child.Hidden {
					ifaces = append(ifaces, ifaceEntry{Name: child.Name(), Short: child.Short})
				}
			}
			sort.Slice(ifaces, func(i, j int) bool { return ifaces[i].Name < ifaces[j].Name })

			// JSON envelope: {interfaces: [...], note?: "..."}.
			if flags.asJSON {
				out := map[string]any{"interfaces": ifaces}
				if len(ifaces) == 0 {
					out["interfaces"] = []ifaceEntry{}
					out["note"] = "No hidden API interfaces found."
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			if len(ifaces) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hidden API interfaces found.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Available API interfaces (%d):\n\n", len(ifaces))
			for _, e := range ifaces {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-45s %s\n", e.Name, e.Short)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'straddle api <interface>' to see methods.\n")
			return nil
		},
	}
	cmd.Flags().StringArray("param", nil, "Add query parameter for raw API calls (repeatable key=value)")
	cmd.Flags().StringArray("header", nil, "Add request header for raw API calls (repeatable key=value)")
	cmd.Flags().Bool("stdin", false, "Read raw API request body as JSON from stdin")

	return cmd
}
