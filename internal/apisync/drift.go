// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/straddle-build/straddle-cli/internal/surface"
)

type DriftResult struct {
	BaseOperations        int                    `json:"base_operations"`
	HeadOperations        int                    `json:"head_operations"`
	SupportedAdditions    []Operation            `json:"supported_additions"`
	Changes               []OperationChange      `json:"changes"`
	Removals              []Operation            `json:"removals"`
	UnsupportedOperations []UnsupportedOperation `json:"unsupported_operations"`
	NoDrift               bool                   `json:"no_drift"`
}

type OperationChange struct {
	Key    string        `json:"key"`
	Base   Operation     `json:"base"`
	Head   Operation     `json:"head"`
	Reason string        `json:"reason"`
	Fields []FieldChange `json:"fields"`
}

type FieldChange struct {
	Flag   string `json:"flag"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

func DriftSpecs(basePath, headPath string) (DriftResult, error) {
	baseDoc, err := loadParsedDocument(basePath)
	if err != nil {
		return DriftResult{}, err
	}
	baseSurfaces, baseUnsupported := deriveSurfaces(baseDoc)
	headDoc, err := loadParsedDocument(headPath)
	if err != nil {
		return DriftResult{}, err
	}
	headSurfaces, headUnsupported := deriveSurfaces(headDoc)

	result := ClassifyDrift(baseSurfaces, headSurfaces)
	result.BaseOperations = len(baseSurfaces)
	result.HeadOperations = len(headSurfaces)
	baseUnsupportedByKey := unsupportedOperationMap(baseUnsupported)
	headUnsupportedByKey := unsupportedOperationMap(headUnsupported)
	headSurfaceByKey := surfaceMap(headSurfaces)
	for _, unsupported := range headUnsupported {
		base, existed := baseUnsupportedByKey[unsupported.Operation.Key]
		if !existed {
			result.SupportedAdditions = removeOperation(result.SupportedAdditions, unsupported.Operation.Key)
			result.Changes = removeOperationChange(result.Changes, unsupported.Operation.Key)
		}
		if !existed || !reflect.DeepEqual(base.Reasons, unsupported.Reasons) {
			result.UnsupportedOperations = append(result.UnsupportedOperations, unsupported)
		}
	}
	for _, unsupported := range baseUnsupported {
		if _, stillUnsupported := headUnsupportedByKey[unsupported.Operation.Key]; stillUnsupported {
			continue
		}
		if head, nowSupported := headSurfaceByKey[unsupported.Operation.Key]; nowSupported {
			result.SupportedAdditions = append(result.SupportedAdditions, operationFromSurface(head))
			result.Changes = removeOperationChange(result.Changes, unsupported.Operation.Key)
		}
	}
	hydrateDriftOperations(&result, baseDoc.operations, headDoc.operations)
	SortOperations(result.SupportedAdditions)
	sort.Slice(result.UnsupportedOperations, func(i, j int) bool {
		return result.UnsupportedOperations[i].Operation.Key < result.UnsupportedOperations[j].Operation.Key
	})
	result.NoDrift = len(result.SupportedAdditions) == 0 && len(result.Changes) == 0 && len(result.Removals) == 0 && len(result.UnsupportedOperations) == 0
	return result, nil
}

func ClassifyDrift(baseSurfaces, headSurfaces []surface.Surface) DriftResult {
	result := DriftResult{
		BaseOperations:        len(baseSurfaces),
		HeadOperations:        len(headSurfaces),
		SupportedAdditions:    []Operation{},
		Changes:               []OperationChange{},
		Removals:              []Operation{},
		UnsupportedOperations: []UnsupportedOperation{},
	}
	baseByKey := surfaceMap(baseSurfaces)
	headByKey := surfaceMap(headSurfaces)

	for _, head := range headSurfaces {
		base, ok := baseByKey[surfaceKey(head)]
		if !ok {
			result.SupportedAdditions = append(result.SupportedAdditions, operationFromSurface(head))
			continue
		}
		fields := changedSurfaceFields(base, head)
		if len(fields) > 0 {
			result.Changes = append(result.Changes, OperationChange{
				Key:    surfaceKey(head),
				Base:   operationFromSurface(base),
				Head:   operationFromSurface(head),
				Reason: fieldChangeReason(fields),
				Fields: fields,
			})
		}
	}
	for _, base := range baseSurfaces {
		if _, ok := headByKey[surfaceKey(base)]; !ok {
			result.Removals = append(result.Removals, operationFromSurface(base))
		}
	}
	SortOperations(result.SupportedAdditions)
	SortOperations(result.Removals)
	sort.Slice(result.Changes, func(i, j int) bool { return result.Changes[i].Key < result.Changes[j].Key })
	result.NoDrift = len(result.SupportedAdditions) == 0 && len(result.Changes) == 0 && len(result.Removals) == 0
	return result
}

func changedSurfaceFields(base, head surface.Surface) []FieldChange {
	var fields []FieldChange
	fields = appendSurfaceChange(fields, "endpoint", base.Endpoint, head.Endpoint)
	fields = appendSurfaceChange(fields, "operation-id", base.OperationID, head.OperationID)
	fields = appendSurfaceChange(fields, "path-params", base.PathParams, head.PathParams)
	fields = appendSurfaceChange(fields, "body", base.HasBody, head.HasBody)
	fields = appendSurfaceChange(fields, "body-required", base.BodyRequired, head.BodyRequired)
	fields = appendSurfaceChange(fields, "straddle-account-id", base.AcceptsAccountHeader, head.AcceptsAccountHeader)
	fields = appendSurfaceChange(fields, "read-only", base.ReadOnly, head.ReadOnly)

	baseFlags := flagMap(base.Flags)
	headFlags := flagMap(head.Flags)
	for _, flag := range head.Flags {
		previous, ok := baseFlags[flag.Name]
		switch {
		case !ok:
			fields = append(fields, FieldChange{Flag: flag.Name, Kind: "added", Detail: describeFlag(flag)})
		case !reflect.DeepEqual(previous, flag):
			fields = append(fields, FieldChange{Flag: flag.Name, Kind: "changed", Detail: describeFlagChanges(previous, flag)})
		}
	}
	for _, flag := range base.Flags {
		if _, ok := headFlags[flag.Name]; !ok {
			fields = append(fields, FieldChange{Flag: flag.Name, Kind: "removed", Detail: describeFlag(flag)})
		}
	}
	sort.Slice(fields, func(i, j int) bool {
		if fields[i].Flag != fields[j].Flag {
			return fields[i].Flag < fields[j].Flag
		}
		return fields[i].Kind < fields[j].Kind
	})
	return fields
}

func appendSurfaceChange(fields []FieldChange, name string, base, head any) []FieldChange {
	if reflect.DeepEqual(base, head) {
		return fields
	}
	return append(fields, FieldChange{
		Flag:   name,
		Kind:   "changed",
		Detail: fmt.Sprintf("%v -> %v", base, head),
	})
}

func fieldChangeReason(fields []FieldChange) string {
	changed := make([]string, len(fields))
	for i, field := range fields {
		changed[i] = fmt.Sprintf("%s (%s)", field.Flag, field.Kind)
	}
	return "surface fields changed: " + strings.Join(changed, ", ")
}

func describeFlag(flag surface.Flag) string {
	description := fmt.Sprintf("%s %s", flag.In, flag.Kind)
	if flag.Array {
		description += " array"
	}
	if flag.Required {
		description += ", required"
	}
	return description
}

func describeFlagChanges(base, head surface.Flag) string {
	var changes []string
	appendChange := func(name string, before, after any) {
		if !reflect.DeepEqual(before, after) {
			changes = append(changes, fmt.Sprintf("%s: %v -> %v", name, before, after))
		}
	}
	appendChange("in", base.In, head.In)
	appendChange("key", base.Key, head.Key)
	appendChange("kind", base.Kind, head.Kind)
	appendChange("array", base.Array, head.Array)
	appendChange("style", base.Style, head.Style)
	appendChange("explode", base.Explode, head.Explode)
	appendChange("required", base.Required, head.Required)
	appendChange("enum", base.Enum, head.Enum)
	appendChange("description", base.Description, head.Description)
	appendChange("default", base.Default, head.Default)
	return strings.Join(changes, "; ")
}

func operationFromSurface(derived surface.Surface) Operation {
	op := Operation{
		Key:                 surfaceKey(derived),
		OperationID:         derived.OperationID,
		Endpoint:            derived.Endpoint,
		Method:              derived.Method,
		Path:                derived.Path,
		RequestBodyRequired: derived.BodyRequired,
		ReadOnly:            derived.ReadOnly,
	}
	if derived.HasBody {
		op.RequestBodyMediaTypes = []string{"application/json"}
	}
	for _, name := range derived.PathParams {
		op.PathParameters = append(op.PathParameters, Parameter{Name: name, In: "path", Required: true, SchemaType: "string"})
	}
	for _, flag := range derived.Flags {
		if flag.In != surface.InQuery && flag.In != surface.InHeader {
			continue
		}
		schemaType := string(flag.Kind)
		if flag.Array {
			schemaType = "array"
		}
		parameter := Parameter{
			Name:        flag.Key,
			In:          string(flag.In),
			Required:    flag.Required,
			Description: flag.Description,
			SchemaType:  schemaType,
			Style:       string(flag.Style),
			Explode:     flag.Explode,
		}
		if flag.In == surface.InQuery {
			op.QueryParameters = append(op.QueryParameters, parameter)
		} else {
			op.HeaderParameters = append(op.HeaderParameters, parameter)
		}
	}
	return op
}

func surfaceKey(derived surface.Surface) string {
	return OperationKey(derived.Method, derived.Path)
}

func surfaceMap(surfaces []surface.Surface) map[string]surface.Surface {
	mapped := make(map[string]surface.Surface, len(surfaces))
	for _, derived := range surfaces {
		mapped[surfaceKey(derived)] = derived
	}
	return mapped
}

func flagMap(flags []surface.Flag) map[string]surface.Flag {
	mapped := make(map[string]surface.Flag, len(flags))
	for _, flag := range flags {
		mapped[flag.Name] = flag
	}
	return mapped
}

func unsupportedOperationMap(operations []UnsupportedOperation) map[string]UnsupportedOperation {
	mapped := make(map[string]UnsupportedOperation, len(operations))
	for _, operation := range operations {
		mapped[operation.Operation.Key] = operation
	}
	return mapped
}

func removeOperation(operations []Operation, key string) []Operation {
	for i, operation := range operations {
		if operation.Key == key {
			return append(operations[:i], operations[i+1:]...)
		}
	}
	return operations
}

func removeOperationChange(changes []OperationChange, key string) []OperationChange {
	for i, change := range changes {
		if change.Key == key {
			return append(changes[:i], changes[i+1:]...)
		}
	}
	return changes
}

func hydrateDriftOperations(result *DriftResult, baseOperations, headOperations []Operation) {
	baseByKey := rawOperationMap(baseOperations)
	headByKey := rawOperationMap(headOperations)
	for i, operation := range result.SupportedAdditions {
		if parsed, ok := headByKey[operation.Key]; ok {
			result.SupportedAdditions[i] = parsed
		}
	}
	for i, operation := range result.Removals {
		if parsed, ok := baseByKey[operation.Key]; ok {
			result.Removals[i] = parsed
		}
	}
	for i, change := range result.Changes {
		if parsed, ok := baseByKey[change.Key]; ok {
			result.Changes[i].Base = parsed
		}
		if parsed, ok := headByKey[change.Key]; ok {
			result.Changes[i].Head = parsed
		}
	}
}

func rawOperationMap(operations []Operation) map[string]Operation {
	mapped := make(map[string]Operation, len(operations))
	for _, operation := range operations {
		mapped[operation.Key] = operation
	}
	return mapped
}
