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
		{"change.block", ChangeBlockRequest{}, func() []StatusFinding {
			return ChangeBlock(context.Background(), PlanningDeps{}, "", ChangeBlockRequest{}).Findings
		}},
		{"change.defer", ChangeDeferRequest{}, func() []StatusFinding {
			return ChangeDefer(context.Background(), PlanningDeps{}, "", ChangeDeferRequest{}).Findings
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
			want := requiredJSONKeys(tc.prototype)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("op %s: empty-request findings name %v; docket:\"required\" tags mark %v", tc.op, got, want)
			}
		})
	}
}
