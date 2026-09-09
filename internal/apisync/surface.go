// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/straddle-build/straddle-cli/internal/surface"
)

const schemaReferencePrefix = "#/components/schemas/"

type schemaNode struct {
	Ref                  string                     `json:"$ref"`
	Type                 json.RawMessage            `json:"type"`
	Description          string                     `json:"description"`
	Format               string                     `json:"format"`
	Properties           map[string]json.RawMessage `json:"properties"`
	Required             []string                   `json:"required"`
	Items                json.RawMessage            `json:"items"`
	AdditionalProperties json.RawMessage            `json:"additionalProperties"`
	AllOf                []json.RawMessage          `json:"allOf"`
	OneOf                []json.RawMessage          `json:"oneOf"`
	AnyOf                []json.RawMessage          `json:"anyOf"`
	Enum                 []json.RawMessage          `json:"enum"`
	Default              json.RawMessage            `json:"default"`
}

type surfaceDeriver struct {
	doc *parsedDocument
}

func DeriveSurfaces(specPath string) ([]surface.Surface, []UnsupportedOperation, error) {
	doc, err := loadParsedDocument(specPath)
	if err != nil {
		return nil, nil, err
	}
	surfaces, unsupported := deriveSurfaces(doc)
	return surfaces, unsupported, nil
}

func deriveSurfaces(doc *parsedDocument) ([]surface.Surface, []UnsupportedOperation) {
	surfaces := make([]surface.Surface, 0, len(doc.operations))
	unsupported := make([]UnsupportedOperation, 0)
	for _, op := range doc.operations {
		derived, reasons := SurfaceFromOperation(op, doc)
		surfaces = append(surfaces, derived)
		if len(reasons) > 0 {
			unsupported = append(unsupported, UnsupportedOperation{Operation: op, Reasons: reasons})
		}
	}
	sortSurfaces(surfaces)
	sort.Slice(unsupported, func(i, j int) bool {
		return unsupported[i].Operation.Key < unsupported[j].Operation.Key
	})
	return surfaces, unsupported
}

func SurfaceFromOperation(op Operation, doc *parsedDocument) (surface.Surface, []string) {
	derived := surface.Surface{
		Endpoint:    op.Endpoint,
		OperationID: op.OperationID,
		Method:      op.Method,
		Path:        op.Path,
		PathParams:  pathParameters(op.Path),
		ReadOnly:    op.Method == "GET" || op.Method == "HEAD",
	}
	reasons := append([]string(nil), UnsupportedReasons(op)...)
	parsed, ok := doc.rawOperations[op.Key]
	if !ok {
		return derived, append(reasons, "operation is missing from parsed document")
	}

	deriver := surfaceDeriver{doc: doc}
	for _, parameter := range parsed.parameters {
		if parameter.In != "query" && parameter.In != "header" {
			continue
		}
		if parameter.In == "header" && strings.EqualFold(parameter.Name, "Straddle-Account-Id") {
			derived.AcceptsAccountHeader = true
			continue
		}
		flag, parameterReasons := deriver.parameterFlag(parameter)
		reasons = append(reasons, parameterReasons...)
		if len(parameterReasons) == 0 {
			derived.Flags = append(derived.Flags, flag)
		}
	}

	if parsed.operation.RequestBody != nil {
		derived.HasBody = true
		derived.BodyRequired = parsed.operation.RequestBody.Required
		if schema, ok := requestBodySchema(parsed.operation.RequestBody); ok {
			root, rootReasons := deriver.resolveSchema(schema, "/", map[string]bool{})
			reasons = append(reasons, rootReasons...)
			if len(rootReasons) == 0 {
				stack := stringSet(schemaReferences(schema))
				bodyFlags, bodyReasons := deriver.flattenNode(root, nil, nil, derived.BodyRequired, stack)
				derived.Flags = append(derived.Flags, bodyFlags...)
				reasons = append(reasons, bodyReasons...)
			}
		} else if len(parsed.operation.RequestBody.Content) == 0 {
			reasons = append(reasons, "request body has no declared media type")
		} else {
			reasons = append(reasons, "request body lacks application/json content")
		}
	}

	reasons = append(reasons, surfaceUnsupportedReasons(derived)...)
	sortSurfaceFlags(derived.Flags)
	return derived, uniqueSortedStrings(reasons)
}

func (d surfaceDeriver) parameterFlag(parameter rawParameter) (surface.Flag, []string) {
	location := surface.In(parameter.In)
	style := surface.StyleForm
	if location == surface.InHeader {
		style = surface.StyleSimple
	}
	if parameter.Style != "" {
		style = surface.Style(parameter.Style)
	}
	explode := style == surface.StyleForm
	if parameter.Explode != nil {
		explode = *parameter.Explode
	}

	flag := surface.Flag{
		Name:        kebab(parameter.Name),
		In:          location,
		Key:         parameter.Name,
		Style:       style,
		Explode:     explode,
		Required:    parameter.Required,
		Description: surfaceDescription(parameter.Description),
	}
	var reasons []string
	if location == surface.InQuery && style != surface.StyleForm {
		reasons = append(reasons, fmt.Sprintf("query parameter %q uses unsupported style %s", parameter.Name, style))
	}
	if location == surface.InHeader && style != surface.StyleSimple {
		reasons = append(reasons, fmt.Sprintf("header parameter %q uses unsupported style %s", parameter.Name, style))
	}

	node, schemaReasons := d.resolveSchema(parameter.Schema, parameter.In+" parameter "+parameter.Name, map[string]bool{})
	reasons = append(reasons, schemaReasons...)
	if len(schemaReasons) > 0 {
		return flag, reasons
	}
	if flag.Description == "" {
		flag.Description = surfaceDescription(node.Description)
	}
	flag.Default = renderSchemaValue(node.Default)
	flag.Format = node.Format

	schemaType, ok := nodeType(node)
	if !ok {
		return flag, append(reasons, fmt.Sprintf("%s parameter %q has no representable schema kind", parameter.In, parameter.Name))
	}
	if schemaType != "array" {
		kind, scalar := scalarKind(schemaType)
		if !scalar {
			return flag, append(reasons, fmt.Sprintf("%s parameter %q uses unsupported schema type %s", parameter.In, parameter.Name, schemaType))
		}
		flag.Kind = kind
		flag.Enum = renderEnum(node.Enum)
		return flag, reasons
	}
	if location != surface.InQuery {
		return flag, append(reasons, fmt.Sprintf("header parameter %q uses unsupported array schema", parameter.Name))
	}
	if style != surface.StyleForm || !explode {
		return flag, append(reasons, fmt.Sprintf("query array parameter %q requires style form and explode true", parameter.Name))
	}
	if len(bytes.TrimSpace(node.Items)) == 0 {
		return flag, append(reasons, fmt.Sprintf("query array parameter %q has no item schema", parameter.Name))
	}
	items, itemReasons := d.resolveSchema(node.Items, parameter.In+" parameter "+parameter.Name+" items", map[string]bool{})
	reasons = append(reasons, itemReasons...)
	itemType, itemOK := nodeType(items)
	kind, repeatable := repeatableKind(itemType)
	if !itemOK || !repeatable {
		return flag, append(reasons, fmt.Sprintf("query array parameter %q has unsupported item schema", parameter.Name))
	}
	flag.Kind = kind
	flag.Array = true
	flag.Format = items.Format
	flag.Enum = renderEnum(items.Enum)
	return flag, reasons
}

// The binder registers repeatable flags only for string and integer elements;
// other element kinds fall back to a JSON flag or an unsupported reason.
func repeatableKind(itemType string) (surface.Kind, bool) {
	kind, scalar := scalarKind(itemType)
	if !scalar || (kind != surface.KindString && kind != surface.KindInteger) {
		return "", false
	}
	return kind, true
}

func requestBodySchema(body *rawRequestBody) (json.RawMessage, bool) {
	if media, ok := body.Content["application/json"]; ok {
		return media.Schema, true
	}
	mediaTypes := make([]string, 0, len(body.Content))
	for mediaType := range body.Content {
		mediaTypes = append(mediaTypes, mediaType)
	}
	sort.Strings(mediaTypes)
	for _, mediaType := range mediaTypes {
		base := strings.ToLower(strings.TrimSpace(strings.SplitN(mediaType, ";", 2)[0]))
		if base == "application/json" || strings.HasSuffix(base, "+json") {
			return body.Content[mediaType].Schema, true
		}
	}
	return nil, false
}

func (d surfaceDeriver) resolveSchema(raw json.RawMessage, pointer string, stack map[string]bool) (schemaNode, []string) {
	var node schemaNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return node, []string{fmt.Sprintf("invalid schema at %s: %v", pointer, err)}
	}

	var reasons []string
	resolved := schemaNode{}
	if node.Ref != "" {
		if stack[node.Ref] {
			return node, []string{fmt.Sprintf("recursive schema reference %s at %s", node.Ref, pointer)}
		}
		if !strings.HasPrefix(node.Ref, schemaReferencePrefix) {
			return node, []string{fmt.Sprintf("unsupported schema reference %s at %s", node.Ref, pointer)}
		}
		componentName := decodeJSONPointerToken(strings.TrimPrefix(node.Ref, schemaReferencePrefix))
		component, ok := d.doc.raw.Components.Schemas[componentName]
		if !ok {
			return node, []string{fmt.Sprintf("missing schema reference %s at %s", node.Ref, pointer)}
		}
		stack[node.Ref] = true
		resolved, reasons = d.resolveSchema(component, pointer, stack)
		delete(stack, node.Ref)
	}

	allOf := node.AllOf
	node.Ref = ""
	node.AllOf = nil
	resolved, mergeReasons := mergeSchemaNodes(resolved, node, pointer)
	reasons = append(reasons, mergeReasons...)
	for _, member := range allOf {
		part, partReasons := d.resolveSchema(member, pointer, stack)
		reasons = append(reasons, partReasons...)
		resolved, mergeReasons = mergeSchemaNodes(resolved, part, pointer)
		reasons = append(reasons, mergeReasons...)
	}
	return resolved, reasons
}

func mergeSchemaNodes(base, overlay schemaNode, pointer string) (schemaNode, []string) {
	var reasons []string
	baseType, baseHasType := nodeType(base)
	overlayType, overlayHasType := nodeType(overlay)
	if baseHasType && overlayHasType && baseType != overlayType {
		reasons = append(reasons, fmt.Sprintf("conflicting allOf schema kinds at %s: %s and %s", pointer, baseType, overlayType))
	}
	if len(overlay.Type) > 0 {
		base.Type = append(json.RawMessage(nil), overlay.Type...)
	}
	if overlay.Description != "" {
		base.Description = overlay.Description
	}
	if overlay.Format != "" {
		base.Format = overlay.Format
	}
	if base.Properties == nil && len(overlay.Properties) > 0 {
		base.Properties = make(map[string]json.RawMessage, len(overlay.Properties))
	}
	for name, property := range overlay.Properties {
		if existing, ok := base.Properties[name]; ok {
			base.Properties[name] = combineAllOfSchemas(existing, property)
			continue
		}
		base.Properties[name] = append(json.RawMessage(nil), property...)
	}
	base.Required = appendUnique(base.Required, overlay.Required...)
	if len(overlay.Items) > 0 {
		base.Items = append(json.RawMessage(nil), overlay.Items...)
	}
	if len(overlay.AdditionalProperties) > 0 {
		base.AdditionalProperties = append(json.RawMessage(nil), overlay.AdditionalProperties...)
	}
	base.OneOf = append(base.OneOf, overlay.OneOf...)
	base.AnyOf = append(base.AnyOf, overlay.AnyOf...)
	if overlay.Enum != nil {
		base.Enum = append([]json.RawMessage(nil), overlay.Enum...)
	}
	if len(overlay.Default) > 0 {
		base.Default = append(json.RawMessage(nil), overlay.Default...)
	}
	return base, reasons
}

func combineAllOfSchemas(base, overlay json.RawMessage) json.RawMessage {
	combined := make(json.RawMessage, 0, len(base)+len(overlay)+14)
	combined = append(combined, `{"allOf":[`...)
	combined = append(combined, base...)
	combined = append(combined, ',')
	combined = append(combined, overlay...)
	return append(combined, ']', '}')
}

func (d surfaceDeriver) flattenNode(node schemaNode, nameParts, pointerParts []string, required bool, stack map[string]bool) ([]surface.Flag, []string) {
	pointer := jsonPointer(pointerParts)
	if len(nameParts) > 0 && strings.EqualFold(nameParts[len(nameParts)-1], "metadata") {
		return []surface.Flag{bodyFlag(node, nameParts, pointer, required, surface.KindJSON, false)}, nil
	}
	if len(node.OneOf) > 0 || len(node.AnyOf) > 0 || allowsAdditionalProperties(node.AdditionalProperties) {
		return []surface.Flag{bodyFlag(node, nameParts, pointer, required, surface.KindJSON, false)}, nil
	}

	schemaType, hasType := nodeType(node)
	if schemaType == "object" || (!hasType && len(node.Properties) > 0) {
		if len(node.Properties) == 0 {
			return nil, []string{"schema node at " + pointer + " has no representable kind"}
		}
		propertyNames := make([]string, 0, len(node.Properties))
		for name := range node.Properties {
			propertyNames = append(propertyNames, name)
		}
		sort.Strings(propertyNames)
		requiredProperties := stringSet(node.Required)
		var flags []surface.Flag
		var reasons []string
		for _, name := range propertyNames {
			propertyPointer := appendPath(pointerParts, name)
			propertyName := appendPath(nameParts, name)
			propertySchema := node.Properties[name]
			propertyNode, propertyReasons := d.resolveSchema(propertySchema, jsonPointer(propertyPointer), stack)
			reasons = append(reasons, propertyReasons...)
			if len(propertyReasons) > 0 {
				continue
			}
			references := schemaReferences(propertySchema)
			recursive := false
			for _, reference := range references {
				if stack[reference] {
					reasons = append(reasons, fmt.Sprintf("recursive schema reference %s at %s", reference, jsonPointer(propertyPointer)))
					recursive = true
					break
				}
			}
			if recursive {
				continue
			}
			for _, reference := range references {
				stack[reference] = true
			}
			propertyFlags, nestedReasons := d.flattenNode(propertyNode, propertyName, propertyPointer, required && requiredProperties[name], stack)
			for _, reference := range references {
				delete(stack, reference)
			}
			flags = append(flags, propertyFlags...)
			reasons = append(reasons, nestedReasons...)
		}
		return flags, reasons
	}
	if schemaType == "array" {
		if len(bytes.TrimSpace(node.Items)) == 0 {
			return []surface.Flag{bodyFlag(node, nameParts, pointer, required, surface.KindJSON, false)}, nil
		}
		items, reasons := d.resolveSchema(node.Items, pointer, stack)
		itemType, ok := nodeType(items)
		kind, repeatable := repeatableKind(itemType)
		if len(reasons) == 0 && ok && repeatable {
			flag := bodyFlag(node, nameParts, pointer, required, kind, true)
			flag.Format = items.Format
			flag.Enum = renderEnum(items.Enum)
			return []surface.Flag{flag}, nil
		}
		if len(reasons) > 0 {
			return nil, reasons
		}
		return []surface.Flag{bodyFlag(node, nameParts, pointer, required, surface.KindJSON, false)}, nil
	}
	if kind, scalar := scalarKind(schemaType); hasType && scalar {
		flag := bodyFlag(node, nameParts, pointer, required, kind, false)
		flag.Format = node.Format
		flag.Enum = renderEnum(node.Enum)
		return []surface.Flag{flag}, nil
	}
	return nil, []string{"schema node at " + pointer + " has no representable kind"}
}

func bodyFlag(node schemaNode, nameParts []string, pointer string, required bool, kind surface.Kind, array bool) surface.Flag {
	kebabParts := make([]string, 0, len(nameParts))
	for _, part := range nameParts {
		kebabParts = append(kebabParts, kebab(part))
	}
	return surface.Flag{
		Name:        strings.Join(kebabParts, "-"),
		In:          surface.InBody,
		Key:         pointer,
		Kind:        kind,
		Array:       array,
		Required:    required,
		Description: surfaceDescription(node.Description),
		Default:     renderSchemaValue(node.Default),
	}
}

func nodeType(node schemaNode) (string, bool) {
	var single string
	if err := json.Unmarshal(node.Type, &single); err == nil && single != "" {
		return single, true
	}
	var multiple []string
	if err := json.Unmarshal(node.Type, &multiple); err != nil {
		return "", false
	}
	for _, schemaType := range multiple {
		if schemaType == "null" {
			continue
		}
		if single != "" {
			return "", false
		}
		single = schemaType
	}
	return single, single != ""
}

func scalarKind(schemaType string) (surface.Kind, bool) {
	switch schemaType {
	case "string":
		return surface.KindString, true
	case "integer":
		return surface.KindInteger, true
	case "number":
		return surface.KindNumber, true
	case "boolean":
		return surface.KindBoolean, true
	default:
		return "", false
	}
}

func renderEnum(values []json.RawMessage) []string {
	if len(values) == 0 {
		return nil
	}
	rendered := make([]string, 0, len(values))
	for _, value := range values {
		if string(bytes.TrimSpace(value)) == "null" {
			continue
		}
		rendered = append(rendered, renderSchemaValue(value))
	}
	if len(rendered) == 0 {
		return nil
	}
	return rendered
}

func renderSchemaValue(value json.RawMessage) string {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return text
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, value); err == nil {
		return compact.String()
	}
	return string(value)
}

func surfaceDescription(description string) string {
	text := strings.Join(strings.Fields(description), " ")
	for index, r := range text {
		if r != '.' {
			continue
		}
		next := index + 1
		if next == len(text) || (next < len(text) && unicode.IsSpace(rune(text[next]))) {
			text = strings.TrimSpace(text[:next])
			break
		}
	}
	runes := []rune(text)
	if len(runes) > 120 {
		text = strings.TrimSpace(string(runes[:117])) + "..."
	}
	return text
}

func pathParameters(path string) []string {
	var parameters []string
	for offset := 0; offset < len(path); {
		start := strings.IndexByte(path[offset:], '{')
		if start < 0 {
			break
		}
		start += offset
		end := strings.IndexByte(path[start+1:], '}')
		if end < 0 {
			break
		}
		end += start + 1
		parameters = append(parameters, path[start+1:end])
		offset = end + 1
	}
	return parameters
}

func schemaReferences(raw json.RawMessage) []string {
	var node schemaNode
	if json.Unmarshal(raw, &node) != nil {
		return nil
	}
	references := make([]string, 0, 1+len(node.AllOf))
	if node.Ref != "" {
		references = append(references, node.Ref)
	}
	for _, member := range node.AllOf {
		references = append(references, schemaReferences(member)...)
	}
	return uniqueSortedStrings(references)
}

func decodeJSONPointerToken(token string) string {
	return strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
}

func allowsAdditionalProperties(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("false"))
}

func jsonPointer(parts []string) string {
	if len(parts) == 0 {
		return "/"
	}
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = strings.NewReplacer("~", "~0", "/", "~1").Replace(part)
	}
	return "/" + strings.Join(escaped, "/")
}

func appendPath(parts []string, part string) []string {
	appended := make([]string, len(parts)+1)
	copy(appended, parts)
	appended[len(parts)] = part
	return appended
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func appendUnique(values []string, additions ...string) []string {
	seen := stringSet(values)
	for _, value := range additions {
		if !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func sortSurfaces(surfaces []surface.Surface) {
	sort.Slice(surfaces, func(i, j int) bool {
		if surfaces[i].Path != surfaces[j].Path {
			return surfaces[i].Path < surfaces[j].Path
		}
		if methodOrder[surfaces[i].Method] != methodOrder[surfaces[j].Method] {
			return methodOrder[surfaces[i].Method] < methodOrder[surfaces[j].Method]
		}
		return surfaces[i].OperationID < surfaces[j].OperationID
	})
}

func sortSurfaceFlags(flags []surface.Flag) {
	rank := map[surface.In]int{
		surface.InQuery:  0,
		surface.InHeader: 1,
		surface.InBody:   2,
	}
	sort.Slice(flags, func(i, j int) bool {
		if rank[flags[i].In] != rank[flags[j].In] {
			return rank[flags[i].In] < rank[flags[j].In]
		}
		if flags[i].Name != flags[j].Name {
			return flags[i].Name < flags[j].Name
		}
		return flags[i].Key < flags[j].Key
	})
}
