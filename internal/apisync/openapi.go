// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

var methodOrder = map[string]int{
	"GET":     0,
	"POST":    1,
	"PUT":     2,
	"PATCH":   3,
	"DELETE":  4,
	"HEAD":    5,
	"OPTIONS": 6,
	"TRACE":   7,
}

type Operation struct {
	Key                   string      `json:"key"`
	OperationID           string      `json:"operation_id"`
	Endpoint              string      `json:"endpoint,omitempty"`
	Method                string      `json:"method"`
	Path                  string      `json:"path"`
	Summary               string      `json:"summary,omitempty"`
	Description           string      `json:"description,omitempty"`
	Tags                  []string    `json:"tags,omitempty"`
	PathParameters        []Parameter `json:"path_parameters,omitempty"`
	QueryParameters       []Parameter `json:"query_parameters,omitempty"`
	HeaderParameters      []Parameter `json:"header_parameters,omitempty"`
	RequestBodyRequired   bool        `json:"request_body_required,omitempty"`
	RequestBodyMediaTypes []string    `json:"request_body_media_types,omitempty"`
	ReadOnly              bool        `json:"read_only"`
	Fingerprint           string      `json:"fingerprint"`
}

type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
}

type rawDocument struct {
	OpenAPI string                                `json:"openapi"`
	Paths   map[string]map[string]json.RawMessage `json:"paths"`
}

type rawOperation struct {
	Tags        []string        `json:"tags"`
	Summary     string          `json:"summary"`
	Description string          `json:"description"`
	OperationID string          `json:"operationId"`
	Parameters  []rawParameter  `json:"parameters"`
	RequestBody *rawRequestBody `json:"requestBody"`
}

type rawParameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type rawRequestBody struct {
	Required bool                       `json:"required"`
	Content  map[string]json.RawMessage `json:"content"`
}

func LoadSpec(path string) ([]Operation, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- spec paths are explicit local CLI/workflow inputs.
	if err != nil {
		return nil, fmt.Errorf("reading spec %s: %w", path, err)
	}
	return ParseSpec(data)
}

func ParseSpec(data []byte) ([]Operation, error) {
	var doc rawDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing OpenAPI JSON: %w", err)
	}
	if strings.TrimSpace(doc.OpenAPI) == "" {
		return nil, fmt.Errorf("missing openapi version")
	}
	if len(doc.Paths) == 0 {
		return nil, fmt.Errorf("missing paths")
	}

	ops := make([]Operation, 0)
	for path, item := range doc.Paths {
		pathParams, err := parseRawParameters(item["parameters"], path)
		if err != nil {
			return nil, err
		}
		for method, raw := range item {
			method = strings.ToUpper(method)
			if _, ok := methodOrder[method]; !ok {
				continue
			}
			var ro rawOperation
			if err := json.Unmarshal(raw, &ro); err != nil {
				return nil, fmt.Errorf("parsing %s %s operation: %w", method, path, err)
			}
			op := Operation{
				Key:         OperationKey(method, path),
				OperationID: ro.OperationID,
				Endpoint:    deriveEndpoint(ro.OperationID, ro.Tags),
				Method:      method,
				Path:        path,
				Summary:     ro.Summary,
				Description: ro.Description,
				Tags:        append([]string(nil), ro.Tags...),
				ReadOnly:    method == "GET" || method == "HEAD",
				Fingerprint: fingerprintOperation(method, path, raw, item["parameters"]),
			}
			for _, p := range append(pathParams, ro.Parameters...) {
				param := Parameter(p)
				switch p.In {
				case "path":
					op.PathParameters = append(op.PathParameters, param)
				case "query":
					op.QueryParameters = append(op.QueryParameters, param)
				case "header":
					op.HeaderParameters = append(op.HeaderParameters, param)
				default:
					// Preserve unsupported parameter locations through support classification.
					op.QueryParameters = append(op.QueryParameters, param)
				}
			}
			if ro.RequestBody != nil {
				op.RequestBodyRequired = ro.RequestBody.Required
				for mediaType := range ro.RequestBody.Content {
					op.RequestBodyMediaTypes = append(op.RequestBodyMediaTypes, mediaType)
				}
				sort.Strings(op.RequestBodyMediaTypes)
			}
			ops = append(ops, op)
		}
	}
	SortOperations(ops)
	return ops, nil
}

func parseRawParameters(raw json.RawMessage, context string) ([]rawParameter, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var params []rawParameter
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("parsing %s parameters: %w", context, err)
	}
	return params, nil
}

func OperationKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

func SortOperations(ops []Operation) {
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		if methodOrder[ops[i].Method] != methodOrder[ops[j].Method] {
			return methodOrder[ops[i].Method] < methodOrder[ops[j].Method]
		}
		return ops[i].OperationID < ops[j].OperationID
	})
}

func fingerprintOperation(method, path string, raw, pathParameters json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		v = string(raw)
	}
	var params any
	if len(pathParameters) > 0 {
		if err := json.Unmarshal(pathParameters, &params); err != nil {
			params = string(pathParameters)
		}
	}
	canonical, _ := json.Marshal(map[string]any{
		"method":          strings.ToUpper(method),
		"path":            path,
		"path_parameters": params,
		"op":              v,
	})
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func deriveEndpoint(operationID string, tags []string) string {
	if operationID == "" {
		return ""
	}
	resource := "endpoint"
	if len(tags) > 0 && strings.TrimSpace(tags[0]) != "" {
		resource = kebab(tags[0])
		switch resource {
		case "charge":
			resource = "charges"
		case "payout":
			resource = "payouts"
		}
	}
	action, rest := splitAction(operationID)
	if action == "" {
		return resource + "." + kebab(operationID)
	}
	restKebab := kebab(rest)
	if restKebab == "" || restMatchesResource(restKebab, resource) {
		return resource + "." + action
	}
	return resource + "." + action + "-" + restKebab
}

func splitAction(operationID string) (string, string) {
	prefixes := []string{"Create", "Update", "Delete", "List", "Get", "Hold", "Release", "Cancel", "Resubmit", "Onboard", "Refresh", "Reveal", "Unmask", "Simulate"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(operationID, prefix) && len(operationID) > len(prefix) {
			return strings.ToLower(prefix), operationID[len(prefix):]
		}
	}
	return "", operationID
}

func restMatchesResource(rest, resource string) bool {
	trimmed := strings.TrimSuffix(resource, "s")
	return rest == resource || rest == trimmed || rest+"s" == resource
}

func kebab(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	var prevDash bool
	for i, r := range s {
		if r == '_' || r == '-' || r == ' ' || r == '.' || r == '/' {
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
			continue
		}
		if i > 0 && isUpperASCII(r) && !prevDash {
			b.WriteByte('-')
		}
		b.WriteRune(toLowerASCII(r))
		prevDash = false
	}
	return strings.Trim(b.String(), "-")
}

func isUpperASCII(r rune) bool { return r >= 'A' && r <= 'Z' }
func toLowerASCII(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
