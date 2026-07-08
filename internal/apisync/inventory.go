// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Annotation struct {
	Endpoint string `json:"endpoint"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	File     string `json:"file"`
	ReadOnly bool   `json:"read_only"`
}

type AnnotationIssue struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

type Inventory struct {
	Annotations []Annotation      `json:"annotations"`
	Issues      []AnnotationIssue `json:"issues,omitempty"`
}

func InventoryRepo(repo string) (Inventory, error) {
	cliDir := filepath.Join(repo, "internal", "cli")
	return InventoryDir(cliDir, repo)
}

func InventoryDir(dir, repoRoot string) (Inventory, error) {
	var inv Inventory
	if _, err := os.Stat(dir); err != nil {
		return inv, fmt.Errorf("stat %s: %w", dir, err)
	}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		annotations, issues, err := inventoryFile(path, repoRoot)
		if err != nil {
			return err
		}
		inv.Annotations = append(inv.Annotations, annotations...)
		inv.Issues = append(inv.Issues, issues...)
		return nil
	})
	if err != nil {
		return inv, err
	}
	sort.Slice(inv.Annotations, func(i, j int) bool {
		if inv.Annotations[i].Method != inv.Annotations[j].Method {
			return inv.Annotations[i].Method < inv.Annotations[j].Method
		}
		if inv.Annotations[i].Path != inv.Annotations[j].Path {
			return inv.Annotations[i].Path < inv.Annotations[j].Path
		}
		return inv.Annotations[i].File < inv.Annotations[j].File
	})
	sort.Slice(inv.Issues, func(i, j int) bool {
		if inv.Issues[i].File != inv.Issues[j].File {
			return inv.Issues[i].File < inv.Issues[j].File
		}
		return inv.Issues[i].Message < inv.Issues[j].Message
	})
	return inv, nil
}

func inventoryFile(path, repoRoot string) ([]Annotation, []AnnotationIssue, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	rel := path
	if repoRoot != "" {
		if r, err := filepath.Rel(repoRoot, path); err == nil {
			rel = r
		}
	}
	var annotations []Annotation
	var issues []AnnotationIssue
	ast.Inspect(file, func(node ast.Node) bool {
		lit, ok := node.(*ast.CompositeLit)
		if !ok || !isStringMapLiteral(lit) {
			return true
		}
		values := stringMapValues(lit)
		if len(values) == 0 {
			return true
		}
		_, hasEndpoint := values["straddle:endpoint"]
		_, hasMethod := values["straddle:method"]
		_, hasPath := values["straddle:path"]
		if !hasEndpoint && !hasMethod && !hasPath {
			return true
		}
		if !hasEndpoint || !hasMethod || !hasPath {
			issues = append(issues, AnnotationIssue{File: rel, Message: "incomplete straddle annotation"})
			return true
		}
		method := strings.ToUpper(strings.TrimSpace(values["straddle:method"]))
		pathValue := strings.TrimSpace(values["straddle:path"])
		annotations = append(annotations, Annotation{
			Endpoint: strings.TrimSpace(values["straddle:endpoint"]),
			Method:   method,
			Path:     pathValue,
			File:     rel,
			ReadOnly: strings.EqualFold(values["mcp:read-only"], "true"),
		})
		return true
	})
	return annotations, issues, nil
}

func isStringMapLiteral(lit *ast.CompositeLit) bool {
	mt, ok := lit.Type.(*ast.MapType)
	if !ok {
		return false
	}
	key, ok := mt.Key.(*ast.Ident)
	if !ok || key.Name != "string" {
		return false
	}
	value, ok := mt.Value.(*ast.Ident)
	return ok && value.Name == "string"
}

func stringMapValues(lit *ast.CompositeLit) map[string]string {
	values := make(map[string]string)
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := stringLiteral(kv.Key)
		if !ok {
			continue
		}
		value, ok := stringLiteral(kv.Value)
		if !ok {
			continue
		}
		values[key] = value
	}
	return values
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
