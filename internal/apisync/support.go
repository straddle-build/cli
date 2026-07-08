// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import "strings"

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
