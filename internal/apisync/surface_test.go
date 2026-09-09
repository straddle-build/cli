// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/straddle-build/straddle-cli/internal/surface"
)

func TestDeriveSurfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec string
		want func(*testing.T, []surface.Surface, []UnsupportedOperation)
	}{
		{
			name: "query array uses item enum and OpenAPI defaults",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets:
    get:
      operationId: listWidgets
      tags: [widgets]
      parameters:
        - name: status
          in: query
          schema:
            type: array
            items:
              type: string
              enum: [open, closed]
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				got := requireSingleSupportedSurface(t, surfaces, unsupported)
				requireFlag(t, got, surface.Flag{
					Name:    "status",
					In:      surface.InQuery,
					Key:     "status",
					Kind:    surface.KindString,
					Array:   true,
					Style:   surface.StyleForm,
					Explode: true,
					Enum:    []string{"open", "closed"},
				})
			},
		},
		{
			name: "scalar formats are preserved",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets:
    post:
      operationId: createWidget
      tags: [widgets]
      parameters:
        - name: customer_id
          in: query
          schema:
            type: string
            format: uuid
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                starts_on:
                  type: string
                  format: date
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				got := requireSingleSupportedSurface(t, surfaces, unsupported)
				requireFlag(t, got, surface.Flag{
					Name:    "customer-id",
					In:      surface.InQuery,
					Key:     "customer_id",
					Kind:    surface.KindString,
					Style:   surface.StyleForm,
					Explode: true,
					Format:  "uuid",
				})
				requireFlag(t, got, surface.Flag{
					Name:   "starts-on",
					In:     surface.InBody,
					Key:    "/starts_on",
					Kind:   surface.KindString,
					Format: "date",
				})
			},
		},
		{
			name: "nested required property has required ancestors",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets:
    post:
      operationId: createWidget
      tags: [widgets]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [config]
              properties:
                config:
                  type: object
                  required: [auto_hold]
                  properties:
                    auto_hold:
                      type: boolean
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				got := requireSingleSupportedSurface(t, surfaces, unsupported)
				requireFlag(t, got, surface.Flag{
					Name:     "config-auto-hold",
					In:       surface.InBody,
					Key:      "/config/auto_hold",
					Kind:     surface.KindBoolean,
					Required: true,
				})
			},
		},
		{
			name: "optional request body never yields required flags",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets/{id}/hold:
    put:
      operationId: holdWidget
      tags: [widgets]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [reason]
              properties:
                reason:
                  type: string
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				got := requireSingleSupportedSurface(t, surfaces, unsupported)
				if got.BodyRequired {
					t.Fatalf("BodyRequired = true, want false")
				}
				requireFlag(t, got, surface.Flag{
					Name: "reason",
					In:   surface.InBody,
					Key:  "/reason",
					Kind: surface.KindString,
				})
			},
		},
		{
			name: "nested required property has optional ancestor",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets:
    post:
      operationId: createWidget
      tags: [widgets]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                config:
                  type: object
                  required: [auto_hold]
                  properties:
                    auto_hold:
                      type: boolean
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				got := requireSingleSupportedSurface(t, surfaces, unsupported)
				requireFlag(t, got, surface.Flag{
					Name: "config-auto-hold",
					In:   surface.InBody,
					Key:  "/config/auto_hold",
					Kind: surface.KindBoolean,
				})
			},
		},
		{
			name: "metadata is one JSON flag",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets:
    post:
      operationId: createWidget
      tags: [widgets]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                metadata:
                  type: object
                  additionalProperties:
                    type: string
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				got := requireSingleSupportedSurface(t, surfaces, unsupported)
				requireFlag(t, got, surface.Flag{
					Name: "metadata",
					In:   surface.InBody,
					Key:  "/metadata",
					Kind: surface.KindJSON,
				})
			},
		},
		{
			name: "account header is represented by the surface bit",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets:
    get:
      operationId: listWidgets
      tags: [widgets]
      parameters:
        - name: Straddle-Account-Id
          in: header
          schema:
            type: string
        - name: Request-Id
          in: header
          schema:
            type: string
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				got := requireSingleSupportedSurface(t, surfaces, unsupported)
				if !got.AcceptsAccountHeader {
					t.Fatal("AcceptsAccountHeader = false, want true")
				}
				if flagByName(got.Flags, "straddle-account-id") != nil {
					t.Fatalf("Flags = %#v, want no straddle-account-id flag", got.Flags)
				}
				requireFlag(t, got, surface.Flag{
					Name:  "request-id",
					In:    surface.InHeader,
					Key:   "Request-Id",
					Kind:  surface.KindString,
					Style: surface.StyleSimple,
				})
			},
		},
		{
			name: "delete retains its request body",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets/{id}:
    delete:
      operationId: deleteWidget
      tags: [widgets]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [reason]
              properties:
                reason:
                  type: string
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				got := requireSingleSupportedSurface(t, surfaces, unsupported)
				if !got.HasBody || !got.BodyRequired {
					t.Fatalf("HasBody = %t, BodyRequired = %t, want both true", got.HasBody, got.BodyRequired)
				}
			},
		},
		{
			name: "unrepresentable schema identifies its pointer",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets:
    post:
      operationId: createWidget
      tags: [widgets]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                config:
                  type: object
                  properties:
                    mystery: {}
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				if len(surfaces) != 1 {
					t.Fatalf("surfaces = %#v, want the partial surface retained", surfaces)
				}
				if len(unsupported) != 1 {
					t.Fatalf("unsupported = %#v, want one operation", unsupported)
				}
				if !surfaceReasonContains(unsupported[0].Reasons, "/config/mystery") {
					t.Fatalf("reasons = %#v, want JSON pointer", unsupported[0].Reasons)
				}
			},
		},
		{
			name: "array path parameter is unsupported",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets/{tags}:
    get:
      operationId: listWidgetsByTags
      tags: [widgets]
      parameters:
        - name: tags
          in: path
          required: true
          style: simple
          explode: false
          schema:
            type: array
            items:
              type: string
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				if len(unsupported) != 1 {
					t.Fatalf("unsupported = %#v, want one array-path operation", unsupported)
				}
				if unsupported[0].Operation.Key != "GET /v1/widgets/{tags}" {
					t.Fatalf("unsupported key = %q, want GET /v1/widgets/{tags}", unsupported[0].Operation.Key)
				}
				if !surfaceReasonContains(unsupported[0].Reasons, `path parameter "tags" uses unsupported schema type array`) {
					t.Fatalf("reasons = %#v, want array path schema-type reason", unsupported[0].Reasons)
				}
			},
		},
		{
			name: "array query parameter remains supported end to end",
			spec: `
openapi: 3.1.0
paths:
  /v1/widgets:
    get:
      operationId: listWidgets
      tags: [widgets]
      parameters:
        - name: status
          in: query
          schema:
            type: array
            items:
              type: string
`,
			want: func(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) {
				t.Helper()
				got := requireSingleSupportedSurface(t, surfaces, unsupported)
				requireFlag(t, got, surface.Flag{
					Name:    "status",
					In:      surface.InQuery,
					Key:     "status",
					Kind:    surface.KindString,
					Array:   true,
					Style:   surface.StyleForm,
					Explode: true,
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			surfaces, unsupported, err := DeriveSurfaces(writeSurfaceSpec(t, test.spec))
			if err != nil {
				t.Fatalf("DeriveSurfaces: %v", err)
			}
			test.want(t, surfaces, unsupported)
		})
	}
}

func TestDriftSpecsReportsReferencedSchemaFieldChanges(t *testing.T) {
	t.Parallel()

	base := writeSurfaceSpec(t, `
openapi: 3.1.0
paths:
  /v1/widgets:
    post:
      operationId: createWidget
      tags: [widgets]
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateWidget'
components:
  schemas:
    CreateWidget:
      type: object
      properties:
        status:
          type: string
          enum: [open, closed]
`)
	head := writeSurfaceSpec(t, `
openapi: 3.1.0
paths:
  /v1/widgets:
    post:
      operationId: createWidget
      tags: [widgets]
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateWidget'
components:
  schemas:
    CreateWidget:
      type: object
      required: [amount]
      properties:
        amount:
          type: integer
        status:
          type: string
          enum: [open]
`)

	result, err := DriftSpecs(base, head)
	if err != nil {
		t.Fatalf("DriftSpecs: %v", err)
	}
	if len(result.Changes) != 1 {
		t.Fatalf("Changes = %#v, want one operation change", result.Changes)
	}
	fields := result.Changes[0].Fields
	if len(fields) != 2 {
		t.Fatalf("Fields = %#v, want two field changes", fields)
	}
	if fields[0].Flag != "amount" || fields[0].Kind != "added" {
		t.Fatalf("Fields[0] = %#v, want amount added", fields[0])
	}
	if fields[1].Flag != "status" || fields[1].Kind != "changed" {
		t.Fatalf("Fields[1] = %#v, want status changed", fields[1])
	}
}

func writeSurfaceSpec(t *testing.T, spec string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(spec)+"\n"), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return path
}

func requireSingleSupportedSurface(t *testing.T, surfaces []surface.Surface, unsupported []UnsupportedOperation) surface.Surface {
	t.Helper()
	if len(unsupported) != 0 {
		t.Fatalf("unsupported = %#v, want none", unsupported)
	}
	if len(surfaces) != 1 {
		t.Fatalf("surfaces = %#v, want one", surfaces)
	}
	return surfaces[0]
}

func requireFlag(t *testing.T, got surface.Surface, want surface.Flag) {
	t.Helper()
	flag := flagByName(got.Flags, want.Name)
	if flag == nil {
		t.Fatalf("Flags = %#v, want %q", got.Flags, want.Name)
	}
	if flag.In != want.In || flag.Key != want.Key || flag.Kind != want.Kind || flag.Array != want.Array || flag.Style != want.Style || flag.Explode != want.Explode || flag.Required != want.Required || flag.Format != want.Format {
		t.Fatalf("flag %q = %#v, want %#v", want.Name, *flag, want)
	}
	if strings.Join(flag.Enum, ",") != strings.Join(want.Enum, ",") {
		t.Fatalf("flag %q enum = %#v, want %#v", want.Name, flag.Enum, want.Enum)
	}
}

func flagByName(flags []surface.Flag, name string) *surface.Flag {
	for i := range flags {
		if flags[i].Name == name {
			return &flags[i]
		}
	}
	return nil
}

func surfaceReasonContains(reasons []string, want string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, want) {
			return true
		}
	}
	return false
}

func TestDeriveSurfacesAcceptsParameterizedJSONBody(t *testing.T) {
	path := writeSurfaceSpec(t, `
openapi: 3.1.0
paths:
  /v1/widgets:
    post:
      operationId: createWidget
      tags: [widgets]
      requestBody:
        required: true
        content:
          application/json; charset=utf-8:
            schema:
              type: object
              properties:
                name:
                  type: string
`)
	surfaces, unsupported, err := DeriveSurfaces(path)
	if err != nil {
		t.Fatal(err)
	}
	got := requireSingleSupportedSurface(t, surfaces, unsupported)
	if !got.HasBody || !got.BodyRequired {
		t.Fatalf("body = (%t, %t), want required body", got.HasBody, got.BodyRequired)
	}
	requireFlag(t, got, surface.Flag{Name: "name", In: surface.InBody, Key: "/name", Kind: surface.KindString})
}
