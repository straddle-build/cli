// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import (
	"fmt"
	"strings"
)

type UnsupportedOperation struct {
	Operation Operation `json:"operation"`
	Reasons   []string  `json:"reasons"`
}

func UnsupportedReasons(op Operation) []string {
	var reasons []string
	if strings.TrimSpace(op.OperationID) == "" {
		reasons = append(reasons, "missing operationId")
	}
	if !strings.HasPrefix(op.Path, "/") {
		reasons = append(reasons, "path must start with /")
	}
	switch op.Method {
	case "GET", "DELETE", "POST", "PUT", "PATCH":
	default:
		reasons = append(reasons, "unsupported HTTP method "+op.Method)
	}
	for _, param := range append(append([]Parameter{}, op.PathParameters...), append(op.QueryParameters, op.HeaderParameters...)...) {
		switch param.In {
		case "path", "query", "header":
		default:
			reasons = append(reasons, "unsupported parameter location "+param.In+" for "+param.Name)
		}
	}
	reasons = append(reasons, generatedParameterUnsupportedReasons(op)...)
	if strings.TrimSpace(op.RequestBodyRef) != "" {
		reasons = append(reasons, "request body $ref is not supported: "+op.RequestBodyRef)
	}
	if (op.Method == "GET" || op.Method == "DELETE") && (op.RequestBodyRequired || len(op.RequestBodyMediaTypes) > 0) {
		reasons = append(reasons, "request body is not supported for "+op.Method+" operations")
	}
	if op.RequestBodyRequired || len(op.RequestBodyMediaTypes) > 0 {
		if len(op.RequestBodyMediaTypes) == 0 {
			reasons = append(reasons, "request body has no declared media type")
		} else if !hasJSONMediaType(op.RequestBodyMediaTypes) {
			reasons = append(reasons, "request body lacks application/json content")
		}
	}
	return reasons
}

func IsSupported(op Operation) bool {
	return len(UnsupportedReasons(op)) == 0
}

func hasJSONMediaType(mediaTypes []string) bool {
	for _, mediaType := range mediaTypes {
		mediaType = strings.ToLower(strings.TrimSpace(mediaType))
		if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
			return true
		}
	}
	return false
}

func generatedParameterUnsupportedReasons(op Operation) []string {
	var reasons []string
	for _, param := range append(append([]Parameter{}, op.PathParameters...), append(op.QueryParameters, op.HeaderParameters...)...) {
		if !isSupportedGeneratedParameterName(param.Name) {
			reasons = append(reasons, fmt.Sprintf("unsupported parameter name %q: must start with a letter and contain only letters, numbers, hyphens, or underscores", param.Name))
		}
	}

	flagOwners := generatedReservedFlagOwners()
	if op.RequestBodyRequired || len(op.RequestBodyMediaTypes) > 0 {
		flagOwners["stdin"] = "request body stdin flag"
	}
	varOwners := map[string]string{}
	for _, param := range append(append([]Parameter{}, op.QueryParameters...), generatedHeaderParameters(op.HeaderParameters)...) {
		flag := flagName(param)
		owner := parameterOwner(param)
		if previous, ok := flagOwners[flag]; ok {
			reasons = append(reasons, fmt.Sprintf("parameter flag name collision %q for %s and %s", flag, previous, owner))
		} else {
			flagOwners[flag] = owner
		}

		variable := paramVarName(param)
		if param.In == "header" {
			variable += "Header"
		}
		if previous, ok := varOwners[variable]; ok {
			reasons = append(reasons, fmt.Sprintf("parameter variable name collision %q for %s and %s", variable, previous, owner))
		} else {
			varOwners[variable] = owner
		}
	}
	return reasons
}

func generatedReservedFlagOwners() map[string]string {
	return map[string]string{
		"account":               "inherited --account flag",
		"agent":                 "inherited --agent flag",
		"allow-partial-failure": "inherited --allow-partial-failure flag",
		"compact":               "inherited --compact flag",
		"config":                "inherited --config flag",
		"csv":                   "inherited --csv flag",
		"data-source":           "inherited --data-source flag",
		"deliver":               "inherited --deliver flag",
		"dry-run":               "inherited --dry-run flag",
		"help":                  "Cobra --help flag",
		"human-friendly":        "inherited --human-friendly flag",
		"idempotent":            "inherited --idempotent flag",
		"ignore-missing":        "inherited --ignore-missing flag",
		"json":                  "inherited --json flag",
		"no-cache":              "inherited --no-cache flag",
		"no-color":              "inherited --no-color flag",
		"no-input":              "inherited --no-input flag",
		"plain":                 "inherited --plain flag",
		"profile":               "inherited --profile flag",
		"quiet":                 "inherited --quiet flag",
		"rate-limit":            "inherited --rate-limit flag",
		"select":                "inherited --select flag",
		"timeout":               "inherited --timeout flag",
		"version":               "Cobra --version flag",
		"yes":                   "inherited --yes flag",
	}
}

func isSupportedGeneratedParameterName(name string) bool {
	if name == "" || strings.TrimSpace(name) != name {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !isASCIILetter(r) {
				return false
			}
			continue
		}
		if isASCIILetter(r) || isASCIIDigit(r) {
			continue
		}
		if r != '-' && r != '_' {
			return false
		}
		if i == len(name)-1 {
			return false
		}
	}
	return true
}

func parameterOwner(param Parameter) string {
	return fmt.Sprintf("%s parameter %q", param.In, param.Name)
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
