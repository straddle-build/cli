// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/straddle-build/straddle-cli/internal/straddleacct"
	"github.com/straddle-build/straddle-cli/internal/surface"
)

const accountHeaderTestID = "550e8400-e29b-41d4-a716-446655440000"

type accountHeaderRequestRecorder struct {
	mu       sync.Mutex
	requests []http.Header
}

func (r *accountHeaderRequestRecorder) record(header http.Header) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, header.Clone())
}

func (r *accountHeaderRequestRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = nil
}

func (r *accountHeaderRequestRecorder) take() []http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	requests := append([]http.Header(nil), r.requests...)
	r.requests = nil
	return requests
}

func TestAccountHeaderMatrix(t *testing.T) {
	surfaces := registeredSurfaces()
	if len(surfaces) == 0 {
		t.Fatal("registeredSurfaces() is empty")
	}
	arguments := accountHeaderArgumentTable(surfaces)
	commands := accountHeaderCommands(t, RootCmd())
	recorder := &accountHeaderRequestRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		recorder.record(request.Header)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	isolateSurfaceConfig(t, server.URL)

	integrationTypes := [...]string{
		straddleacct.TypeAccount,
		straddleacct.TypeSaaS,
		straddleacct.TypeMarketplace,
	}
	decisionCounts := map[straddleacct.Decision]int{}

	for _, registered := range surfaces {
		key := accountHeaderOperationKey(registered.Method, registered.Path)
		argumentSet, ok := arguments[key]
		if !ok {
			t.Fatalf("%s has no contract argument set", key)
		}
		command, ok := commands[key]
		if !ok {
			t.Fatalf("%s has no annotated production command", key)
		}
		prefix := accountHeaderCommandPrefix(command)

		for _, integrationType := range integrationTypes {
			decision := straddleacct.Classify(registered.Path, registered.Method, integrationType, registered.AcceptsAccountHeader)
			passed := t.Run(registered.Endpoint+"/"+integrationType, func(t *testing.T) {
				if err := straddleacct.SaveContext(straddleacct.Context{IntegrationType: integrationType}); err != nil {
					t.Fatalf("save platform context: %v", err)
				}

				withAccount := decision == straddleacct.Require || decision == straddleacct.Allow
				args := accountHeaderMatrixArgs(prefix, argumentSet, withAccount)
				recorder.reset()
				body := "{}"
				if registered.Endpoint == "customers.update" {
					body = `{"status":"review"}`
				}
				_, stderr, err := runRootForAPITest(t, args, body)
				if err != nil {
					t.Fatalf("execute %s: %v\nstderr: %s", strings.Join(args, " "), err, stderr)
				}
				requests := recorder.take()
				if len(requests) != 1 {
					t.Fatalf("HTTP requests = %d, want 1", len(requests))
				}
				gotValues, headerPresent := requests[0][straddleacct.Header]
				if withAccount && (!headerPresent || len(gotValues) != 1 || gotValues[0] != accountHeaderTestID) {
					t.Fatalf("%s values = %q, want [%q]", straddleacct.Header, gotValues, accountHeaderTestID)
				}
				if !withAccount && headerPresent {
					t.Fatalf("%s values = %q, want absent", straddleacct.Header, gotValues)
				}

				if decision != straddleacct.Forbid {
					return
				}
				recorder.reset()
				rejectedArgs := accountHeaderMatrixArgs(prefix, argumentSet, true)
				_, _, err = runRootForAPITest(t, rejectedArgs, "{}")
				if err == nil {
					t.Fatal("forbidden --account returned nil error")
				}
				if !strings.Contains(err.Error(), "remove --account") {
					t.Fatalf("forbidden --account error = %q, want policy guidance", err)
				}
				if got := ExitCode(err); got != 2 {
					t.Fatalf("ExitCode(forbidden --account) = %d, want 2", got)
				}
				if requests := recorder.take(); len(requests) != 0 {
					t.Fatalf("forbidden --account sent %d HTTP requests, want 0", len(requests))
				}
			})
			if passed {
				decisionCounts[decision]++
			}
		}
	}

	for _, decision := range []straddleacct.Decision{straddleacct.Forbid, straddleacct.Require, straddleacct.Allow} {
		if decisionCounts[decision] == 0 {
			t.Errorf("decision %d was not exercised", decision)
		}
	}
	t.Logf(
		"exercised %d surface/integration cells: forbid=%d require=%d allow=%d",
		len(surfaces)*len(integrationTypes),
		decisionCounts[straddleacct.Forbid],
		decisionCounts[straddleacct.Require],
		decisionCounts[straddleacct.Allow],
	)
}

func accountHeaderMatrixArgs(prefix, required []string, withAccount bool) []string {
	args := []string{"--json", "--data-source", "live", "--no-cache", "--yes", "--no-input"}
	if withAccount {
		args = append(args, "--account", accountHeaderTestID)
	}
	args = append(args, prefix...)
	return append(args, required...)
}

func accountHeaderArgumentTable(surfaces []surface.Surface) map[string][]string {
	table := make(map[string][]string, len(surfaces))
	for _, registered := range surfaces {
		args := make([]string, 0, len(registered.PathParams)+1)
		for range registered.PathParams {
			args = append(args, accountHeaderTestID)
		}
		if registered.HasBody {
			args = append(args, "--stdin")
		} else {
			for _, flag := range registered.Flags {
				if flag.Required {
					args = append(args, "--"+flag.Name, accountHeaderFlagValue(flag))
				}
			}
		}
		table[accountHeaderOperationKey(registered.Method, registered.Path)] = args
	}
	return table
}

func accountHeaderFlagValue(flag surface.Flag) string {
	if len(flag.Enum) > 0 {
		return flag.Enum[0]
	}
	switch flag.Kind {
	case surface.KindString:
		return "x"
	case surface.KindInteger, surface.KindNumber:
		return "1"
	case surface.KindBoolean:
		return "true"
	case surface.KindJSON:
		return "{}"
	default:
		panic("unsupported account header test flag kind: " + string(flag.Kind))
	}
}
func accountHeaderOperationKey(method, path string) string {
	return method + " " + path
}

func accountHeaderCommands(t *testing.T, root *cobra.Command) map[string]*cobra.Command {
	t.Helper()
	commands := map[string]*cobra.Command{}
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		method := cmd.Annotations["straddle:method"]
		path := cmd.Annotations["straddle:path"]
		if method != "" && path != "" {
			key := accountHeaderOperationKey(method, path)
			if prior, exists := commands[key]; exists {
				t.Fatalf("%s is declared by both %q and %q", key, prior.CommandPath(), cmd.CommandPath())
			}
			commands[key] = cmd
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(root)
	return commands
}

func accountHeaderCommandPrefix(cmd *cobra.Command) []string {
	var reversed []string
	for current := cmd; current.Parent() != nil; current = current.Parent() {
		reversed = append(reversed, current.Name())
	}
	prefix := make([]string, len(reversed))
	for i := range reversed {
		prefix[len(reversed)-1-i] = reversed[i]
	}
	return prefix
}
