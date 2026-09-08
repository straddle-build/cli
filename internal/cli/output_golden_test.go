// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var updateOutputGoldens = flag.Bool("update", false, "rewrite CLI output golden files")

const goldenUUID = "550e8400-e29b-41d4-a716-446655440000"

type goldenEndpoint struct {
	commandPath []string
	endpoint    string
	id          string
	method      string
	apiPath     string
	internal    bool
}

type goldenMode struct {
	name     string
	args     []string
	terminal bool
}

type goldenRun struct {
	stdout     []byte
	stderr     []byte
	err        error
	exitCode   int
	errorClass string
}

var goldenModes = []goldenMode{
	{name: "agent", args: []string{"--agent"}},
	{name: "json", args: []string{"--json"}},
	{name: "quiet", args: []string{"--quiet"}},
	{name: "plain", args: []string{"--plain"}},
	{name: "csv", args: []string{"--csv"}},
	{name: "dry-run-agent", args: []string{"--dry-run", "--agent"}},
	{name: "human", terminal: true},
}

var goldenInvocations = map[string][]string{
	"account-settings.get-settings": {goldenUUID},
	"accounts.create": {
		"--access-level=standard",
		"--account-type=business",
		"--business-profile-address-city=Denver",
		"--business-profile-address-line1=123 Main St",
		"--business-profile-address-postal-code=80202",
		"--business-profile-address-state=CO",
		"--business-profile-name=Golden Account",
		"--business-profile-website=https://example.com",
		"--organization-id=" + goldenUUID,
	},
	"accounts.get": {goldenUUID},
	"accounts.update": {
		goldenUUID,
		"--business-profile-address-city=Denver",
		"--business-profile-address-line1=123 Main St",
		"--business-profile-address-postal-code=80202",
		"--business-profile-address-state=CO",
		"--business-profile-name=Golden Account",
		"--business-profile-website=https://example.com",
	},
	"bridge.create": {
		"--customer-id=" + goldenUUID,
		"--quiltt-token=quiltt-golden-token",
	},
	"bridge.create-bank-account-paykey": {
		"--account-number=1234567890",
		"--account-type=checking",
		"--customer-id=" + goldenUUID,
		"--routing-number=110000000",
	},
	"bridge.create-plaid-paykey": {
		"--customer-id=" + goldenUUID,
		"--plaid-token=plaid-golden-token",
	},
	"bridge.create-speedchex": {
		"--customer-id=" + goldenUUID,
		"--speedchex-token=speedchex-golden-token",
	},
	"bridge.create-tan": {
		"--account-type=checking",
		"--customer-id=" + goldenUUID,
		"--routing-number=110000000",
		"--tan=tan-golden-token",
	},
	"bridge.create-token": {"--customer-id=" + goldenUUID},
	"cancel.charge":       {goldenUUID},
	"cancel.payout": {
		goldenUUID,
		"--reason=Golden cancellation",
	},
	"cancel.update": {goldenUUID},
	"capability-requests.create": {goldenUUID,
		"--businesses-enable=true",
		"--charges-daily-amount=10000",
		"--charges-enable=true",
		"--charges-max-amount=5000",
		"--charges-monthly-amount=100000",
		"--charges-monthly-count=100",
		"--individuals-enable=true",
		"--internet-enable=true",
		"--payouts-daily-amount=10000",
		"--payouts-enable=true",
		"--payouts-max-amount=5000",
		"--payouts-monthly-amount=100000",
		"--payouts-monthly-count=100",
		"--signed-agreement-enable=true",
	},
	"capability-requests.list": {goldenUUID},
	"charges.create": {
		"--amount=1250",
		"--config-balance-check=required",
		"--consent-type=internet",
		"--description=Golden charge",
		"--device-ip-address=192.0.2.1",
		"--external-id=golden-charge",
		"--paykey=" + goldenUUID,
		"--payment-date=2026-01-15",
	},
	"charges.get":    {goldenUUID},
	"charges.refund": {goldenUUID},
	"charges.update": {goldenUUID,
		"--amount=1250",
		"--description=Updated golden charge",
		"--payment-date=2026-01-16",
	},
	"customers.create": {
		"--address-address1=123 Main St",
		"--address-city=Denver",
		"--address-state=CO",
		"--address-zip=80202",
		"--device-ip-address=192.0.2.1",
		"--email=golden@example.com",
		"--name=Golden Customer",
		"--phone=+15555550100",
		"--type=business",
	},
	"customers.delete": {goldenUUID},
	"customers.get":    {goldenUUID},
	"customers.update": {goldenUUID,
		"--address-address1=123 Main St",
		"--address-city=Denver",
		"--address-state=CO",
		"--address-zip=80202",
		"--device-ip-address=192.0.2.1",
		"--email=golden@example.com",
		"--name=Golden Customer",
		"--phone=+15555550100",
	},
	"funding-event-payments.get": {goldenUUID},
	"funding-events.create":      {"--funding-event-job-type=charges"},
	"funding-events.get":         {goldenUUID},
	"hold.charge":                {goldenUUID},
	"hold.payout": {goldenUUID,
		"--reason=Golden hold",
	},
	"linked-bank-accounts.create": {
		"--account-id=" + goldenUUID,
		"--bank-account-account-holder=Golden Account",
		"--bank-account-account-number=1234567890",
		"--bank-account-routing-number=110000000",
	},
	"linked-bank-accounts.get": {goldenUUID},
	"linked-bank-accounts.update": {goldenUUID,
		"--bank-account-account-holder=Golden Account",
		"--bank-account-account-number=1234567890",
		"--bank-account-routing-number=110000000",
	},
	"onboard.account": {goldenUUID,
		"--terms-of-service-accepted-date=2026-01-15T12:00:00Z",
		"--terms-of-service-agreement-type=embedded",
		"--terms-of-service-agreement-url=https://example.com/terms",
	},
	"organizations.create":    {"--name=Golden Organization"},
	"organizations.get-by-id": {goldenUUID},
	"paykeys.get":             {goldenUUID},
	"payouts.create": {
		"--amount=1250",
		"--description=Golden payout",
		"--device-ip-address=192.0.2.1",
		"--external-id=golden-payout",
		"--paykey=" + goldenUUID,
		"--payment-date=2026-01-15",
	},
	"payouts.get": {goldenUUID},
	"payouts.update": {goldenUUID,
		"--amount=1250",
		"--description=Updated golden payout",
		"--payment-date=2026-01-16",
	},
	"refresh-balance.update": {goldenUUID},
	"refresh-review.update":  {goldenUUID},
	"release.charge":         {goldenUUID},
	"release.payout": {goldenUUID,
		"--reason=Golden release",
	},
	"representatives.create": {
		"--account-id=" + goldenUUID,
		"--dob=1990-01-15",
		"--email=representative@example.com",
		"--first-name=Golden",
		"--last-name=Representative",
		"--mobile-number=+15555550100",
		"--relationship-control=true",
		"--relationship-owner=false",
		"--relationship-primary=true",
		"--ssn-last4=1234",
	},
	"representatives.get": {goldenUUID},
	"representatives.update": {goldenUUID,
		"--dob=1990-01-15",
		"--email=representative@example.com",
		"--first-name=Golden",
		"--last-name=Representative",
		"--mobile-number=+15555550100",
		"--relationship-control=true",
		"--relationship-owner=false",
		"--relationship-primary=true",
		"--ssn-last4=1234",
	},
	"resubmit.create":        {goldenUUID},
	"reveal.get":             {goldenUUID},
	"review.get":             {goldenUUID},
	"review.get-customer":    {goldenUUID},
	"review.update":          {goldenUUID, "--status=active"},
	"review.update-customer": {goldenUUID},
	"simulate.create":        {goldenUUID},
	"unblock.update":         {goldenUUID},
	"unmask.charges-v1-get":  {goldenUUID},
	"unmask.get":             {goldenUUID},
	"unmask.get-linked-bank-account-unmasked": {goldenUUID},
	"unmask.payouts-v1-get":                   {goldenUUID},
	"unmasked.get-customer":                   {goldenUUID},
	"unmasked.get-paykey":                     {goldenUUID},
}

var goldenSkips = map[string]string{}

var legacyGoldenEndpointIDs = map[string]string{
	"account-settings.get":                                  "account-settings.get-settings",
	"accounts.onboard":                                      "onboard.account",
	"accounts.simulate-account-onboarding":                  "simulate.create",
	"bridge.create-bridge-token":                            "bridge.create-token",
	"bridge.create-quiltt-paykey":                           "bridge.create",
	"charges.cancel":                                        "cancel.charge",
	"charges.get-unmasked-charge":                           "unmask.charges-v1-get",
	"charges.hold":                                          "hold.charge",
	"charges.release":                                       "release.charge",
	"charges.resubmit":                                      "resubmit.create",
	"customers.get-customer-review":                         "review.get-customer",
	"customers.get-unmasked-customer":                       "unmasked.get-customer",
	"customers.refresh-customer-review":                     "refresh-review.update",
	"customers.set-customer-verification-decision":          "review.update-customer",
	"funding-events.list-funding-event-payments":            "funding-event-payments.get",
	"funding-events.simulate":                               "funding-events.create",
	"linked-bank-accounts.cancel":                           "cancel.update",
	"linked-bank-accounts.get-unmasked-linked-bank-account": "unmask.get-linked-bank-account-unmasked",
	"organizations.get":                                     "organizations.get-by-id",
	"paykeys.cancel":                                        "cancel.update",
	"paykeys.get-paykey-review":                             "review.get",
	"paykeys.get-unmasked-paykey":                           "unmasked.get-paykey",
	"paykeys.refresh-paykey-balance":                        "refresh-balance.update",
	"paykeys.refresh-paykey-review":                         "refresh-review.update",
	"paykeys.reveal":                                        "reveal.get",
	"paykeys.set-paykey-verification-decision":              "review.update",
	"paykeys.unblock-paykey":                                "unblock.update",
	"payouts.cancel":                                        "cancel.payout",
	"payouts.get-unmasked-payout":                           "unmask.payouts-v1-get",
	"payouts.hold":                                          "hold.payout",
	"payouts.release":                                       "release.payout",
	"payouts.resubmit":                                      "resubmit.create",
	"representatives.get-unmasked-representative":           "unmask.get",
}

func TestOutputGolden(t *testing.T) {
	setGoldenEnvironment(t)
	root := RootCmd()
	endpoints := collectGoldenEndpoints(root)
	if len(endpoints) == 0 {
		t.Fatal("no commands with straddle:endpoint annotations found")
	}
	validateGoldenEndpoints(t, endpoints)
	validateGoldenSkips(t, endpoints)

	t.Run("tree", func(t *testing.T) {
		checkOutputGolden(t, filepath.Join("testdata", "golden", "tree.txt"), renderGoldenTree(root))
	})

	for _, endpoint := range endpoints {
		endpoint := endpoint
		if reason, skipped := goldenSkips[endpoint.id]; skipped {
			t.Run("skip/"+endpoint.id, func(t *testing.T) {
				t.Skip(reason)
			})
			continue
		}
		t.Run("help/"+endpoint.id, func(t *testing.T) {
			checkOutputGolden(t, filepath.Join("testdata", "golden", "help", endpoint.id+".txt"), renderGoldenHelp(t, endpoint))
		})
		response := readGoldenResponse(t, endpoint)
		for _, mode := range goldenModes {
			mode := mode
			t.Run(mode.name+"/"+endpoint.id, func(t *testing.T) {
				got := exerciseGoldenMode(t, endpoint, mode, response)
				checkOutputGolden(t, filepath.Join("testdata", "golden", mode.name, endpoint.id+".txt"), got)
			})
		}
	}
}

func collectGoldenEndpoints(root *cobra.Command) []goldenEndpoint {
	var endpoints []goldenEndpoint
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if endpoint := cmd.Annotations["straddle:endpoint"]; endpoint != "" {
			if legacyEndpoint, ok := legacyGoldenEndpointIDs[endpoint]; ok {
				endpoint = legacyEndpoint
			}
			endpoints = append(endpoints, goldenEndpoint{
				commandPath: strings.Fields(strings.TrimPrefix(cmd.CommandPath(), root.Name()+" ")),
				endpoint:    endpoint,
				method:      cmd.Annotations["straddle:method"],
				apiPath:     cmd.Annotations["straddle:path"],
				internal:    cmd.Annotations["straddle:contract"] == "internal",
			})
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)

	counts := make(map[string]int, len(endpoints))
	for _, endpoint := range endpoints {
		counts[endpoint.endpoint]++
	}
	for i := range endpoints {
		endpoints[i].id = goldenFileID(endpoints[i].endpoint)
		if counts[endpoints[i].endpoint] > 1 {
			endpoints[i].id += "--" + goldenFileID(strings.Join(endpoints[i].commandPath, "."))
		}
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return strings.Join(endpoints[i].commandPath, " ") < strings.Join(endpoints[j].commandPath, " ")
	})
	return endpoints
}

func validateGoldenEndpoints(t *testing.T, endpoints []goldenEndpoint) {
	t.Helper()
	byID := make(map[string]string, len(endpoints))
	knownEndpoints := make(map[string]bool, len(endpoints))
	for _, endpoint := range endpoints {
		commandPath := strings.Join(endpoint.commandPath, " ")
		if endpoint.method == "" || endpoint.apiPath == "" {
			t.Errorf("%s is missing method or path annotations", commandPath)
		}
		if previous, exists := byID[endpoint.id]; exists {
			t.Errorf("golden id %q is shared by %q and %q", endpoint.id, previous, commandPath)
		}
		byID[endpoint.id] = commandPath
		knownEndpoints[endpoint.endpoint] = true
	}
	for endpoint := range goldenInvocations {
		if !knownEndpoints[endpoint] {
			t.Errorf("golden invocation %q has no endpoint command", endpoint)
		}
	}
}

func goldenFileID(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func validateGoldenSkips(t *testing.T, endpoints []goldenEndpoint) {
	t.Helper()
	known := make(map[string]bool, len(endpoints))
	for _, endpoint := range endpoints {
		known[endpoint.id] = true
	}
	for id, reason := range goldenSkips {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("golden skip %q has no reason", id)
		}
		if !known[id] {
			t.Errorf("golden skip %q does not match an endpoint command", id)
		}
	}
}

func renderGoldenTree(root *cobra.Command) []byte {
	var commands []*cobra.Command
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		commands = append(commands, cmd)
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].CommandPath() < commands[j].CommandPath()
	})

	var out strings.Builder
	for _, cmd := range commands {
		cmd.InitDefaultHelpFlag()
		fmt.Fprintf(&out, "command %s\n", cmd.CommandPath())
		flags := goldenCommandFlags(cmd)
		if len(flags) == 0 {
			out.WriteString("  flags: none\n")
		}
		for _, commandFlag := range flags {
			fmt.Fprintf(&out, "  --%s type=%s default=%q usage=%q\n", commandFlag.Name, commandFlag.Value.Type(), commandFlag.DefValue, commandFlag.Usage)
		}
	}
	return []byte(out.String())
}

func goldenCommandFlags(cmd *cobra.Command) []*pflag.Flag {
	byName := map[string]*pflag.Flag{}
	cmd.LocalNonPersistentFlags().VisitAll(func(commandFlag *pflag.Flag) {
		byName[commandFlag.Name] = commandFlag
	})
	cmd.PersistentFlags().VisitAll(func(commandFlag *pflag.Flag) {
		byName[commandFlag.Name] = commandFlag
	})
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	flags := make([]*pflag.Flag, 0, len(names))
	for _, name := range names {
		flags = append(flags, byName[name])
	}
	return flags
}

func renderGoldenHelp(t *testing.T, endpoint goldenEndpoint) []byte {
	t.Helper()
	cmd := RootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs(append(append([]string{}, endpoint.commandPath...), "--help"))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("render help for %s: %v\nstderr: %s", endpoint.id, err, stderr.String())
	}
	if stderr.Len() > 0 {
		t.Fatalf("render help for %s wrote stderr: %s", endpoint.id, stderr.String())
	}
	return stdout.Bytes()
}

func setGoldenEnvironment(t *testing.T) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("HOME", configDir)
	t.Setenv("STRADDLE_CONFIG", filepath.Join(configDir, "config.toml"))
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(configDir, "platform.toml"))
	t.Setenv("STRADDLE_API_KEY", "test_key")
	t.Setenv("STRADDLE_BASE_URL", "http://golden.invalid")
	t.Setenv("STRADDLE_VERIFY", "")
	t.Setenv("STRADDLE_VERIFY_LIVE_HTTP", "")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
}

func exerciseGoldenMode(t *testing.T, endpoint goldenEndpoint, mode goldenMode, response []byte) []byte {
	t.Helper()
	setGoldenEnvironment(t)

	var status atomic.Int64
	var requests atomic.Int64
	status.Store(http.StatusOK)
	expectedPath := goldenExpectedPath(endpoint.apiPath)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != endpoint.method || r.URL.Path != expectedPath {
			t.Errorf("%s request = %s %s, want %s %s", endpoint.id, r.Method, r.URL.Path, endpoint.method, expectedPath)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if status.Load() == http.StatusUnprocessableEntity {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response)
	}))
	defer server.Close()
	t.Setenv("STRADDLE_BASE_URL", server.URL)

	success := runGoldenCommand(t, endpoint, mode)
	if success.err != nil {
		t.Errorf("%s %s success error: %v", endpoint.id, mode.name, success.err)
	}
	status.Store(http.StatusUnprocessableEntity)
	unprocessable := runGoldenCommand(t, endpoint, mode)
	if mode.name == "dry-run-agent" {
		if unprocessable.err != nil {
			t.Errorf("%s %s second dry run error: %v", endpoint.id, mode.name, unprocessable.err)
		}
		if requests.Load() != 0 {
			t.Errorf("%s %s sent %d requests, want 0", endpoint.id, mode.name, requests.Load())
		}
	} else {
		if unprocessable.exitCode != 5 {
			t.Errorf("%s %s 422 exit code = %d, want 5", endpoint.id, mode.name, unprocessable.exitCode)
		}
		if requests.Load() != 2 {
			t.Errorf("%s %s sent %d requests, want 2", endpoint.id, mode.name, requests.Load())
		}
	}

	transcript := renderGoldenRuns(success, unprocessable)
	return bytes.ReplaceAll(transcript, []byte(server.URL), []byte("http://golden.test"))
}

func goldenExpectedPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			parts[i] = goldenUUID
		}
	}
	return strings.Join(parts, "/")
}

func runGoldenCommand(t *testing.T, endpoint goldenEndpoint, mode goldenMode) goldenRun {
	t.Helper()
	cmd := RootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("capture stdout: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		t.Fatalf("capture stderr: %v", err)
	}
	stdoutDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdout, stdoutReader)
		close(stdoutDone)
	}()
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stderr, stderrReader)
		close(stderrDone)
	}()

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalNoColor := noColor
	originalHumanFriendly := humanFriendly
	originalResource := currentResource
	terminal := mode.terminal
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	isTerminalOverride = &terminal
	noColor = false
	humanFriendly = false
	currentResource = ""

	cmd.SetOut(stdoutWriter)
	cmd.SetErr(stderrWriter)
	cmd.SetIn(strings.NewReader(""))
	args := append([]string{}, mode.args...)
	args = append(args, "--no-cache", "--data-source=live")
	args = append(args, endpoint.commandPath...)
	args = append(args, goldenInvocations[endpoint.endpoint]...)
	cmd.SetArgs(args)
	executeErr := cmd.Execute()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout = originalStdout
	os.Stderr = originalStderr
	isTerminalOverride = nil
	noColor = originalNoColor
	humanFriendly = originalHumanFriendly
	currentResource = originalResource
	<-stdoutDone
	<-stderrDone
	_ = stdoutReader.Close()
	_ = stderrReader.Close()

	exitCode := 0
	if executeErr != nil {
		exitCode = ExitCode(executeErr)
	}
	return goldenRun{
		stdout:     append([]byte(nil), stdout.Bytes()...),
		stderr:     append([]byte(nil), stderr.Bytes()...),
		err:        executeErr,
		exitCode:   exitCode,
		errorClass: goldenErrorClass(executeErr, exitCode),
	}
}

func goldenErrorClass(err error, exitCode int) string {
	if err == nil {
		return "none"
	}
	switch exitCode {
	case 2:
		return "usage"
	case 3:
		return "not-found"
	case 4:
		return "authentication"
	case 5:
		return "api"
	case 6:
		return "partial-failure"
	case 7:
		return "rate-limit"
	case 10:
		return "configuration"
	default:
		return "other"
	}
}

func renderGoldenRuns(success, unprocessable goldenRun) []byte {
	var out bytes.Buffer
	writeGoldenRun(&out, "success", success)
	out.WriteByte('\n')
	writeGoldenRun(&out, "http-422", unprocessable)
	return out.Bytes()
}

func writeGoldenRun(out *bytes.Buffer, label string, run goldenRun) {
	fmt.Fprintf(out, "=== %s ===\n", label)
	fmt.Fprintf(out, "exit_code: %d\n", run.exitCode)
	fmt.Fprintf(out, "error_class: %s\n", run.errorClass)
	if run.err == nil {
		out.WriteString("error: <nil>\n")
	} else {
		fmt.Fprintf(out, "error: %q\n", run.err.Error())
	}
	writeGoldenStream(out, "stdout", run.stdout)
	writeGoldenStream(out, "stderr", run.stderr)
}

func writeGoldenStream(out *bytes.Buffer, name string, value []byte) {
	fmt.Fprintf(out, "%s_bytes: %d\n%s:\n<<<\n", name, len(value), name)
	out.Write(value)
	if len(value) == 0 || value[len(value)-1] != '\n' {
		out.WriteByte('\n')
	}
	out.WriteString(">>>\n")
}

func readGoldenResponse(t *testing.T, endpoint goldenEndpoint) []byte {
	t.Helper()
	path := filepath.Join("testdata", "responses", endpoint.id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read response fixture %s: %v", endpoint.id, err)
	}
	if len(data) >= 2*1024 {
		t.Fatalf("response fixture %s is %d bytes, want under 2048", endpoint.id, len(data))
	}
	if !json.Valid(data) {
		t.Fatalf("response fixture %s is not valid JSON", endpoint.id)
	}
	return data
}

func checkOutputGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *updateOutputGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden directory: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v; run go test ./internal/cli -run TestOutputGolden -update", path, err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("output differs from %s:\n%s", path, goldenUnifiedDiff(path, want, got))
	}
}

func goldenUnifiedDiff(path string, want, got []byte) string {
	wantLines := goldenDiffLines(want)
	gotLines := goldenDiffLines(got)
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ actual\n@@ -1,%d +1,%d @@\n", path, len(wantLines), len(gotLines))
	for _, line := range wantLines {
		writeGoldenDiffLine(&out, '-', line)
	}
	for _, line := range gotLines {
		writeGoldenDiffLine(&out, '+', line)
	}
	return out.String()
}

func goldenDiffLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func writeGoldenDiffLine(out *strings.Builder, prefix byte, line string) {
	out.WriteByte(prefix)
	out.WriteString(line)
	if !strings.HasSuffix(line, "\n") {
		out.WriteString("\n\\ No newline at end of file\n")
	}
}
