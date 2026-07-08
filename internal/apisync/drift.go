// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.
package apisync

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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
	Key    string    `json:"key"`
	Base   Operation `json:"base"`
	Head   Operation `json:"head"`
	Reason string    `json:"reason"`
}

func DriftSpecs(basePath, headPath string) (DriftResult, error) {
	base, err := LoadSpec(basePath)
	if err != nil {
		return DriftResult{}, err
	}
	head, err := LoadSpec(headPath)
	if err != nil {
		return DriftResult{}, err
	}
	return ClassifyDrift(base, head), nil
}

func ClassifyDrift(baseOps, headOps []Operation) DriftResult {
	result := DriftResult{
		BaseOperations:        len(baseOps),
		HeadOperations:        len(headOps),
		SupportedAdditions:    []Operation{},
		Changes:               []OperationChange{},
		Removals:              []Operation{},
		UnsupportedOperations: []UnsupportedOperation{},
	}
	baseByKey := operationMap(baseOps)
	headByKey := operationMap(headOps)

	for _, head := range headOps {
		base, ok := baseByKey[head.Key]
		if !ok {
			if reasons := UnsupportedReasons(head); len(reasons) > 0 {
				result.UnsupportedOperations = append(result.UnsupportedOperations, UnsupportedOperation{Operation: head, Reasons: reasons})
			} else {
				result.SupportedAdditions = append(result.SupportedAdditions, head)
			}
			continue
		}
		if base.Fingerprint != head.Fingerprint {
			result.Changes = append(result.Changes, OperationChange{Key: head.Key, Base: base, Head: head, Reason: "operation fingerprint changed"})
		}
	}
	for _, base := range baseOps {
		if _, ok := headByKey[base.Key]; !ok {
			result.Removals = append(result.Removals, base)
		}
	}
	SortOperations(result.SupportedAdditions)
	SortOperations(result.Removals)
	sort.Slice(result.UnsupportedOperations, func(i, j int) bool {
		return result.UnsupportedOperations[i].Operation.Key < result.UnsupportedOperations[j].Operation.Key
	})
	sort.Slice(result.Changes, func(i, j int) bool { return result.Changes[i].Key < result.Changes[j].Key })
	result.NoDrift = len(result.SupportedAdditions) == 0 && len(result.Changes) == 0 && len(result.Removals) == 0 && len(result.UnsupportedOperations) == 0
	return result
}

func ReadDrift(path string) (DriftResult, error) {
	var result DriftResult
	data, err := os.ReadFile(path) // #nosec G304 -- drift paths are explicit local CLI/workflow inputs.
	if err != nil {
		return result, fmt.Errorf("reading drift %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("parsing drift %s: %w", path, err)
	}
	return result, nil
}

func operationMap(ops []Operation) map[string]Operation {
	m := make(map[string]Operation, len(ops))
	for _, op := range ops {
		m[op.Key] = op
	}
	return m
}
