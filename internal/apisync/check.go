// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import "sort"

type CheckResult struct {
	OK                   bool              `json:"ok"`
	SpecOperations       int               `json:"spec_operations"`
	AnnotatedEndpoints   int               `json:"annotated_endpoints"`
	Missing              []Operation       `json:"missing,omitempty"`
	Extra                []Annotation      `json:"extra,omitempty"`
	DuplicateAnnotations []Duplicate       `json:"duplicate_annotations,omitempty"`
	InvalidAnnotations   []AnnotationIssue `json:"invalid_annotations,omitempty"`
}

type Duplicate struct {
	Key         string       `json:"key"`
	Annotations []Annotation `json:"annotations"`
}

func CheckSpecAgainstRepo(specPath, repo string) (CheckResult, error) {
	ops, err := LoadSpec(specPath)
	if err != nil {
		return CheckResult{}, err
	}
	inv, err := InventoryRepo(repo)
	if err != nil {
		return CheckResult{}, err
	}
	return CheckCoverage(ops, inv), nil
}

func CheckCoverage(ops []Operation, inv Inventory) CheckResult {
	result := CheckResult{
		SpecOperations:     len(ops),
		AnnotatedEndpoints: len(inv.Annotations),
		InvalidAnnotations: append([]AnnotationIssue(nil), inv.Issues...),
	}

	specByKey := make(map[string]Operation, len(ops))
	for _, op := range ops {
		specByKey[op.Key] = op
	}
	annotationsByKey := make(map[string][]Annotation, len(inv.Annotations))
	for _, annotation := range inv.Annotations {
		key := OperationKey(annotation.Method, annotation.Path)
		annotationsByKey[key] = append(annotationsByKey[key], annotation)
		if _, ok := specByKey[key]; !ok {
			result.Extra = append(result.Extra, annotation)
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
	result.OK = len(result.Missing) == 0 && len(result.Extra) == 0 && len(result.DuplicateAnnotations) == 0 && len(result.InvalidAnnotations) == 0
	return result
}
