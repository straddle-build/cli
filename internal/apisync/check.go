// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import "sort"

type CheckResult struct {
	OK                    bool                   `json:"ok"`
	SpecOperations        int                    `json:"spec_operations"`
	AnnotatedEndpoints    int                    `json:"annotated_endpoints"`
	Missing               []Operation            `json:"missing,omitempty"`
	Extra                 []Annotation           `json:"extra,omitempty"`
	DuplicateAnnotations  []Duplicate            `json:"duplicate_annotations,omitempty"`
	InvalidAnnotations    []AnnotationIssue      `json:"invalid_annotations,omitempty"`
	OperationIDMismatches []OperationIDMismatch  `json:"operation_id_mismatches,omitempty"`
	UnsupportedOperations []UnsupportedOperation `json:"unsupported_operations,omitempty"`
	StaleGenerated        []string               `json:"stale_generated,omitempty"`
}

type OperationIDMismatch struct {
	Key                 string     `json:"key"`
	ContractOperationID string     `json:"contract_operation_id"`
	Annotation          Annotation `json:"annotation"`
}

type Duplicate struct {
	Key         string       `json:"key"`
	Annotations []Annotation `json:"annotations"`
}

func (result CheckResult) HasBlockingIssues() bool {
	return len(result.Missing) > 0 || len(result.DuplicateAnnotations) > 0 || len(result.InvalidAnnotations) > 0 || len(result.StaleGenerated) > 0
}

func CheckSpecAgainstRepo(specPath, repo string) (CheckResult, error) {
	ops, err := LoadSpec(specPath)
	if err != nil {
		return CheckResult{}, err
	}
	_, unsupported, err := DeriveSurfaces(specPath)
	if err != nil {
		return CheckResult{}, err
	}
	inv, err := InventoryRepo(repo)
	if err != nil {
		return CheckResult{}, err
	}

	unsupportedByKey := make(map[string]bool, len(unsupported))
	for _, operation := range unsupported {
		unsupportedByKey[operation.Operation.Key] = true
	}
	supported := make([]Operation, 0, len(ops)-len(unsupportedByKey))
	for _, op := range ops {
		if !unsupportedByKey[op.Key] {
			supported = append(supported, op)
		}
	}
	result := CheckCoverage(supported, inv)
	result.SpecOperations = len(ops)
	result.UnsupportedOperations = unsupported
	generated, err := GenerateAll(specPath, repo, true)
	if err != nil {
		return CheckResult{}, err
	}
	result.StaleGenerated = append(result.StaleGenerated, generated.Generated...)
	result.StaleGenerated = append(result.StaleGenerated, generated.Deleted...)
	result.StaleGenerated = uniqueSortedStrings(result.StaleGenerated)
	result.OK = result.OK && len(result.StaleGenerated) == 0
	return result, nil
}

func CheckCoverage(ops []Operation, inv Inventory) CheckResult {
	result := CheckResult{
		SpecOperations:     len(ops),
		InvalidAnnotations: append([]AnnotationIssue(nil), inv.Issues...),
	}

	specByKey := make(map[string]Operation, len(ops))
	for _, op := range ops {
		specByKey[op.Key] = op
	}
	annotationsByKey := make(map[string][]Annotation, len(inv.Annotations))
	for _, annotation := range inv.Annotations {
		if annotation.Internal {
			continue
		}
		result.AnnotatedEndpoints++
		key := OperationKey(annotation.Method, annotation.Path)
		annotationsByKey[key] = append(annotationsByKey[key], annotation)
		if op, ok := specByKey[key]; !ok {
			result.Extra = append(result.Extra, annotation)
		} else if annotation.OperationID != op.OperationID {
			result.OperationIDMismatches = append(result.OperationIDMismatches, OperationIDMismatch{
				Key:                 key,
				ContractOperationID: op.OperationID,
				Annotation:          annotation,
			})
		}
	}
	for _, op := range ops {
		if len(annotationsByKey[op.Key]) == 0 {
			result.Missing = append(result.Missing, op)
		}
	}
	for key, annotations := range annotationsByKey {
		if len(annotations) > 1 {
			result.DuplicateAnnotations = append(result.DuplicateAnnotations, Duplicate{Key: key, Annotations: annotations})
		}
	}
	sort.Slice(result.Extra, func(i, j int) bool { return result.Extra[i].File < result.Extra[j].File })
	sort.Slice(result.DuplicateAnnotations, func(i, j int) bool { return result.DuplicateAnnotations[i].Key < result.DuplicateAnnotations[j].Key })
	sort.Slice(result.OperationIDMismatches, func(i, j int) bool { return result.OperationIDMismatches[i].Key < result.OperationIDMismatches[j].Key })
	result.OK = len(result.Missing) == 0 && len(result.Extra) == 0 && len(result.DuplicateAnnotations) == 0 && len(result.InvalidAnnotations) == 0 && len(result.OperationIDMismatches) == 0
	return result
}
