// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var endpointOverlays = map[string]func(*cobra.Command){}

type flagOverlay struct {
	name            string
	usage           string
	defaultSet      bool
	defaultVal      string
	requireExplicit bool
	enumSet         bool
	enum            []string
}

type commandOverlay struct {
	short     string
	long      string
	example   string
	aliases   []string
	flags     []flagOverlay
	paginated bool
	body      bool
	resource  string
	action    string
	unwrap    bool
}

func registerCommandOverlay(endpoint string, overlay commandOverlay) {
	if _, exists := endpointOverlays[endpoint]; exists {
		panic(fmt.Sprintf("duplicate endpoint overlay %q", endpoint))
	}
	endpointOverlays[endpoint] = func(cmd *cobra.Command) {
		cmd.Short = overlay.short
		cmd.Long = overlay.long
		cmd.Example = overlay.example
		cmd.Aliases = append([]string(nil), overlay.aliases...)
		for _, change := range overlay.flags {
			flag := cmd.Flags().Lookup(change.name)
			if flag == nil {
				panic(fmt.Sprintf("endpoint overlay %q references missing flag --%s", endpoint, change.name))
			}
			flag.Usage = change.usage
			if change.defaultSet {
				if err := flag.Value.Set(change.defaultVal); err != nil {
					panic(fmt.Sprintf("endpoint overlay %q sets invalid default for --%s: %v", endpoint, change.name, err))
				}
				flag.DefValue = change.defaultVal
			}
			if change.requireExplicit {
				if flag.Annotations == nil {
					flag.Annotations = map[string][]string{}
				}
				flag.Annotations["straddle:explicit"] = []string{"true"}
			}
			if change.enumSet {
				if len(change.enum) == 0 {
					panic(fmt.Sprintf("endpoint overlay %q sets enumSet for --%s without enum values", endpoint, change.name))
				}
				if flag.Annotations == nil {
					flag.Annotations = map[string][]string{}
				}
				flag.Annotations["straddle:enum"] = append([]string(nil), change.enum...)
			}
		}
		if overlay.paginated {
			cmd.Flags().Bool("all", false, "Fetch all pages")
		}
		if overlay.body {
			cmd.Flags().Bool("stdin", false, "Read request body as JSON from stdin")
			cmd.Annotations["straddle:body"] = "true"
		}
		cmd.Annotations["straddle:resource"] = overlay.resource
		if overlay.action != "" {
			cmd.Annotations["straddle:action"] = overlay.action
		}
		if overlay.unwrap {
			cmd.Annotations["straddle:unwrap-response"] = "true"
		}
	}
}

func applyOverlay(endpoint string, cmd *cobra.Command) {
	if f, ok := endpointOverlays[endpoint]; ok {
		f(cmd)
	}
}
