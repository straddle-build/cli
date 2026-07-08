// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

type GeneratedFile struct {
	Path      string    `json:"path"`
	Operation Operation `json:"operation"`
	Content   string    `json:"-"`
}

type GenerateResult struct {
	Generated             []string               `json:"generated"`
	SkippedExisting       []string               `json:"skipped_existing"`
	UnsupportedOperations []UnsupportedOperation `json:"unsupported_operations"`
	DryRun                bool                   `json:"dry_run"`
}

func MissingSupportedOperations(ops []Operation, inv Inventory) ([]Operation, []UnsupportedOperation) {
	covered := make(map[string]bool, len(inv.Annotations))
	for _, annotation := range inv.Annotations {
		covered[OperationKey(annotation.Method, annotation.Path)] = true
	}
	supported := []Operation{}
	unsupported := []UnsupportedOperation{}
	for _, op := range ops {
		if covered[op.Key] {
			continue
		}
		if reasons := UnsupportedReasons(op); len(reasons) > 0 {
			unsupported = append(unsupported, UnsupportedOperation{Operation: op, Reasons: reasons})
			continue
		}
		supported = append(supported, op)
	}
	SortOperations(supported)
	sort.Slice(unsupported, func(i, j int) bool { return unsupported[i].Operation.Key < unsupported[j].Operation.Key })
	return supported, unsupported
}

func GenerateEndpointFile(op Operation, outDir string) (GeneratedFile, error) {
	if reasons := UnsupportedReasons(op); len(reasons) > 0 {
		return GeneratedFile{}, fmt.Errorf("unsupported operation %s: %s", op.Key, strings.Join(reasons, "; "))
	}
	data := fileTemplateData{
		FuncName:       functionName(op),
		CommandUse:     commandUse(op),
		Short:          firstSentence(op),
		Example:        example(op),
		Endpoint:       op.Endpoint,
		Method:         op.Method,
		Path:           op.Path,
		ReadOnly:       op.ReadOnly,
		PathParams:     op.PathParameters,
		QueryParams:    op.QueryParameters,
		HeaderParams:   generatedHeaderParameters(op.HeaderParameters),
		HasRequestBody: op.RequestBodyRequired || len(op.RequestBodyMediaTypes) > 0,
		NeedsBody:      op.Method == "POST" || op.Method == "PUT" || op.Method == "PATCH",
	}
	var buf bytes.Buffer
	if err := endpointTemplate.Execute(&buf, data); err != nil {
		return GeneratedFile{}, err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return GeneratedFile{}, fmt.Errorf("formatting generated %s: %w", op.Key, err)
	}
	return GeneratedFile{
		Path:      filepath.Join(outDir, fileName(op)),
		Operation: op,
		Content:   string(formatted),
	}, nil
}

func WriteGeneratedFiles(files []GeneratedFile, overwrite bool) (GenerateResult, error) {
	result := GenerateResult{
		Generated:             []string{},
		SkippedExisting:       []string{},
		UnsupportedOperations: []UnsupportedOperation{},
	}
	for _, file := range files {
		if _, err := os.Stat(file.Path); err == nil && !overwrite {
			result.SkippedExisting = append(result.SkippedExisting, file.Path)
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return result, fmt.Errorf("stat %s: %w", file.Path, err)
		}
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o750); err != nil {
			return result, err
		}
		if err := os.WriteFile(file.Path, []byte(file.Content), 0o644); err != nil { // #nosec G306 -- generated source files are intended repo artifacts.
			return result, fmt.Errorf("writing %s: %w", file.Path, err)
		}
		result.Generated = append(result.Generated, file.Path)
	}
	return result, nil
}

type fileTemplateData struct {
	FuncName       string
	CommandUse     string
	Short          string
	Example        string
	Endpoint       string
	Method         string
	Path           string
	ReadOnly       bool
	PathParams     []Parameter
	QueryParams    []Parameter
	HeaderParams   []Parameter
	HasRequestBody bool
	NeedsBody      bool
}

var endpointTemplate = template.Must(template.New("endpoint").Funcs(template.FuncMap{
	"flagName": flagName,
	"varName":  paramVarName,
}).Parse(`// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
{{- if .HasRequestBody }}
	"encoding/json"
	"fmt"
	"io"
	"os"
{{- end }}

	"github.com/spf13/cobra"
)

func init() {
	registerGeneratedEndpoint({{ printf "%q" .Endpoint }}, {{ .FuncName }})
}

	func {{ .FuncName }}(flags *rootFlags) *cobra.Command {
	{{- range .QueryParams }}
		var {{ varName . }} string
	{{- end }}
	{{- range .HeaderParams }}
		var {{ varName . }}Header string
	{{- end }}
	{{- if .HasRequestBody }}
		var stdinBody bool
	{{- end }}

	cmd := &cobra.Command{
		Use:     {{ printf "%q" .CommandUse }},
		Short:   {{ printf "%q" .Short }},
		Example: {{ printf "%q" .Example }},
		Annotations: map[string]string{"straddle:endpoint": {{ printf "%q" .Endpoint }}, "straddle:method": {{ printf "%q" .Method }}, "straddle:path": {{ printf "%q" .Path }}{{ if .ReadOnly }}, "mcp:read-only": "true"{{ end }}},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < {{ len .PathParams }} {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := {{ printf "%q" .Path }}
{{- range $i, $param := .PathParams }}
			path = replacePathParam(path, {{ printf "%q" $param.Name }}, args[{{ $i }}])
{{- end }}
			params := map[string]string{}
	{{- range .QueryParams }}
				if cmd.Flags().Changed({{ printf "%q" (flagName .) }}) {
					params[{{ printf "%q" .Name }}] = {{ varName . }}
				}
	{{- end }}
				headers := map[string]string{}
	{{- range .HeaderParams }}
				if cmd.Flags().Changed({{ printf "%q" (flagName .) }}) {
					headers[{{ printf "%q" .Name }}] = {{ varName . }}Header
				}
	{{- end }}
	{{- if .NeedsBody }}
				var body map[string]any
{{- if .HasRequestBody }}
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if err := json.Unmarshal(stdinData, &body); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
			} else {
				body = map[string]any{}
			}
{{- end }}
{{- end }}

	{{- if eq .Method "GET" }}
				data, err := c.GetWithHeaders(path, params, headers)
				if err != nil {
					return classifyAPIError(err, flags)
				}
	{{- else if eq .Method "DELETE" }}
				data, _, err := c.DeleteWithParamsAndHeaders(path, params, headers)
				if err != nil {
					return classifyAPIError(err, flags)
				}
	{{- else if eq .Method "POST" }}
				data, _, err := c.PostWithParamsAndHeaders(path, params, body, headers)
				if err != nil {
					return classifyAPIError(err, flags)
				}
	{{- else if eq .Method "PUT" }}
				data, _, err := c.PutWithParamsAndHeaders(path, params, body, headers)
				if err != nil {
					return classifyAPIError(err, flags)
				}
	{{- else if eq .Method "PATCH" }}
				data, _, err := c.PatchWithParamsAndHeaders(path, params, body, headers)
				if err != nil {
					return classifyAPIError(err, flags)
				}
{{- end }}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	{{- range .QueryParams }}
		cmd.Flags().StringVar(&{{ varName . }}, {{ printf "%q" (flagName .) }}, "", {{ printf "%q" .Description }})
	{{- end }}
	{{- range .HeaderParams }}
		cmd.Flags().StringVar(&{{ varName . }}Header, {{ printf "%q" (flagName .) }}, "", {{ printf "%q" .Description }})
	{{- end }}
	{{- if .HasRequestBody }}
		cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read JSON request body from stdin")
	{{- end }}
	return cmd
}
`))

func fileName(op Operation) string {
	name := op.Endpoint
	if name == "" {
		name = strings.ToLower(op.Method) + "-" + strings.Trim(op.Path, "/")
	}
	name = strings.NewReplacer(".", "_", "/", "_", "{", "", "}", "", " ", "_", "-", "-").Replace(name)
	return name + ".go"
}

func functionName(op Operation) string {
	base := op.Endpoint
	if base == "" {
		base = strings.ToLower(op.Method) + " " + op.Path
	}
	parts := strings.FieldsFunc(base, func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == '/' || r == ' ' || r == '{' || r == '}'
	})
	var b strings.Builder
	b.WriteString("new")
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	b.WriteString("Cmd")
	return b.String()
}

func commandUse(op Operation) string {
	name := op.Endpoint
	if dot := strings.LastIndex(name, "."); dot >= 0 && dot < len(name)-1 {
		name = name[dot+1:]
	}
	if name == "" {
		name = strings.ToLower(op.Method)
	}
	for _, param := range op.PathParameters {
		name += " <" + param.Name + ">"
	}
	return name
}

func firstSentence(op Operation) string {
	text := strings.TrimSpace(op.Summary)
	if text == "" {
		text = strings.TrimSpace(op.Description)
	}
	if text == "" {
		return op.Method + " " + op.Path
	}
	text = strings.ReplaceAll(text, "\n", " ")
	if len(text) > 120 {
		text = strings.TrimSpace(text[:117]) + "..."
	}
	return text
}

func example(op Operation) string {
	return "  straddle " + strings.ReplaceAll(op.Endpoint, ".", " ")
}

func flagName(param Parameter) string {
	return strings.ToLower(strings.ReplaceAll(param.Name, "_", "-"))
}

func generatedHeaderParameters(params []Parameter) []Parameter {
	headers := make([]Parameter, 0, len(params))
	for _, param := range params {
		if strings.EqualFold(param.Name, "Straddle-Account-Id") {
			continue
		}
		headers = append(headers, param)
	}
	return headers
}

func paramVarName(param Parameter) string {
	parts := strings.FieldsFunc(param.Name, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	if len(parts) == 0 {
		return "flagValue"
	}
	var b strings.Builder
	b.WriteString("flag")
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	return b.String()
}
