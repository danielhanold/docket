package app

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestRequiredTagMatchesValidator proves, for each representative op, that an
// EMPTY request's shape findings name exactly the fields the docket:"required"
// tag marks — so the tag (which the schema surface reports) and the validator
// (which enforces) cannot silently disagree. The finding-code convention
// "invalid-<key>" / "empty-<key>" is the join; extract the key by stripping
// the prefix. Every op validated pre-transaction is callable with zero deps:
// shape refusal returns before any seam is touched.
func TestRequiredTagMatchesValidator(t *testing.T) {
	cases := []struct {
		op        string
		prototype any
		findings  func() []StatusFinding
	}{
		{"adr.record", ADRRecordRequest{}, func() []StatusFinding {
			return validateADRRecordShape(ADRRecordRequest{})
		}},
		// adr.reverse/adr.supersede: the flat key join sees only top-level
		// keys, so the nested target and successor are supplied valid and the
		// empty request's remaining findings must name exactly the top-level
		// required keys.
		{"adr.reverse", ADRReplaceRequest{}, func() []StatusFinding {
			return ADRReverse(context.Background(), PlanningDeps{}, "", validNestedADRReplace()).Findings
		}},
		{"adr.supersede", ADRReplaceRequest{}, func() []StatusFinding {
			return ADRSupersede(context.Background(), PlanningDeps{}, "", validNestedADRReplace()).Findings
		}},
		{"change.kill", ChangeKillRequest{}, func() []StatusFinding {
			return ChangeKill(context.Background(), PlanningDeps{}, "", ChangeKillRequest{}).Findings
		}},
		{"learning.record", LearningRecordRequest{}, func() []StatusFinding {
			return LearningRecordOp(context.Background(), PlanningDeps{}, "", LearningRecordRequest{}).Findings
		}},
		{"learning.update", LearningUpdateRequest{}, func() []StatusFinding {
			return LearningUpdate(context.Background(), PlanningDeps{}, "", LearningUpdateRequest{}).Findings
		}},
		{"change.block", ChangeBlockRequest{}, func() []StatusFinding {
			return ChangeBlock(context.Background(), PlanningDeps{}, "", ChangeBlockRequest{}).Findings
		}},
		{"change.defer", ChangeDeferRequest{}, func() []StatusFinding {
			return ChangeDefer(context.Background(), PlanningDeps{}, "", ChangeDeferRequest{}).Findings
		}},
		{"change.unblock", ChangeUnblockRequest{}, func() []StatusFinding {
			return ChangeUnblock(context.Background(), PlanningDeps{}, "", ChangeUnblockRequest{}).Findings
		}},
		{"change.revive", ChangeReviveRequest{}, func() []StatusFinding {
			return ChangeRevive(context.Background(), PlanningDeps{}, "", ChangeReviveRequest{}).Findings
		}},
		{"change.create", ChangeCreateRequest{}, func() []StatusFinding {
			return validateChangeCreateShape(ChangeCreateRequest{})
		}},
		{"change.groom", ChangeGroomRequest{}, func() []StatusFinding {
			return validateChangeGroomShape(ChangeGroomRequest{})
		}},
		{"change.reconcile", ChangeReconcileRequest{}, func() []StatusFinding {
			return validateChangeReconcileShape(ChangeReconcileRequest{})
		}},
		{"finalize.block", FinalizeBlockInput{}, func() []StatusFinding {
			return validateBlockShape(BlockRequest{ID: 1, Revision: "r", PRNumber: 1, Attempt: "a", Reason: "x", Head: "h"})
		}},
		{"change.halt", ChangeHaltInput{}, func() []StatusFinding {
			return validateHaltShape(HaltRequest{ID: 1, Revision: "r"})
		}},
		{"finalize.retarget-children", RetargetChildrenInput{}, func() []StatusFinding {
			return validateRetargetShape(RetargetChildrenRequest{ID: 1, Revision: "r"})
		}},
		{"finalize.closeout", CloseoutNotes{}, func() []StatusFinding {
			_, findings := normalizeCloseoutNotes(CloseoutNotes{})
			return findings
		}},
	}
	// Completeness: every bound request type is covered by a case or exempted
	// with a stated reason, so a newly bound request cannot silently go
	// unchecked. A case's op is the binding id.
	exempt := map[string]string{
		"pr.publish":               "PRPublishInput has no shape validator: title and body are free-form authored prose, and PRPublish refuses only an oversized body",
		"finalize.rebase-abort":    "ResolverReport has no shape validator: its refusals are verified against the owned receipt and live Git (needs FinalizeDeps), and abort accepts an empty report",
		"finalize.rebase-continue": "ResolverReport has no shape validator: its refusals are verified against the owned receipt and live Git (needs FinalizeDeps) after the attempt is proven",
	}
	covered := map[string]bool{}
	for _, tc := range cases {
		covered[tc.op] = true
	}
	for _, b := range OperationBindings() {
		if b.Request == nil {
			continue
		}
		_, isExempt := exempt[b.ID]
		switch {
		case covered[b.ID] && isExempt:
			t.Errorf("op %s is both a case and exempt; drop the exemption", b.ID)
		case !covered[b.ID] && !isExempt:
			t.Errorf("op %s binds request %T but has no case here and no exemption with a reason", b.ID, b.Request)
		}
	}
	for id := range exempt {
		found := false
		for _, b := range OperationBindings() {
			found = found || (b.ID == id && b.Request != nil)
		}
		if !found {
			t.Errorf("exemption %s names no operation binding a request; drop it", id)
		}
	}

	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			var got []string
			for _, f := range tc.findings() {
				key := strings.TrimPrefix(strings.TrimPrefix(f.Code, "invalid-"), "empty-")
				if key != f.Code { // only shape-convention codes name a key
					got = append(got, key)
				}
			}
			sort.Strings(got)
			if got == nil {
				got = []string{}
			}
			want := requiredJSONKeys(tc.prototype)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("op %s: empty-request findings name %v; docket:\"required\" tags mark %v", tc.op, got, want)
			}
		})
	}
}

// validNestedADRReplace is an ADRReplaceRequest whose nested target and
// successor pass shape validation and whose top-level fields are empty.
func validNestedADRReplace() ADRReplaceRequest {
	return ADRReplaceRequest{
		Target: ADRTarget{ID: 1, Path: "p", Revision: "r"},
		Successor: ADRRecordRequest{Title: "t", Context: "c", Decision: "d",
			Consequences: "q", Alternatives: "a"},
	}
}
