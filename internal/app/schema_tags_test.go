package app

import (
	"context"
	"fmt"
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
		// scalar keys, so the nested target and successor are supplied valid
		// (their findings would otherwise name keys such as "target-id") and
		// the remaining findings must name exactly the top-level scalar
		// required keys. The required target and successor objects themselves
		// are proven by TestNestedRequiredTagMatchesValidator.
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
			want := flatRequiredKeys(t, tc.prototype)
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

// TestNestedRequiredTagMatchesValidator proves the docket:"required" tag on
// every nested request field, and on every object or list-of-objects field at
// any depth, against the operation's shape validator. Each operation starts
// from a known-valid fixture; one field alone is blanked and the request is
// validated. A field the published descriptor marks required must be refused
// (at least one finding); an unmarked field must be accepted (zero findings).
// Finding codes are not used: nested codes have no single spelling. The fields
// checked are walked from reflectDescriptor, the descriptor `docket schema`
// publishes, so none is hand-listed. Fixtures populate every optional object
// and list, so a required field inside an optional object is checked with that
// object sent, which is what its required marker means. A list field is
// blanked in every element; each element is held to its element type's tags.
func TestNestedRequiredTagMatchesValidator(t *testing.T) {
	adrReplace := nestedFixture{
		valid: func() any { return validADRReplaceFixture() },
		validate: func(v any) []StatusFinding {
			return validateADRReplaceShape(v.(ADRReplaceRequest))
		},
	}
	fixtures := map[string]nestedFixture{
		"adr.record": {
			valid: func() any { return validADRContentFixture("adr-record-fixture") },
			validate: func(v any) []StatusFinding {
				return validateADRRecordShape(v.(ADRRecordRequest))
			},
		},
		"adr.reverse":   adrReplace,
		"adr.supersede": adrReplace,
		"change.groom": {
			valid: func() any {
				return ChangeGroomRequest{ChangeID: 1, Path: "p", Revision: "r", Outcome: GroomSpec,
					SpecMarkdown: "Body.\n",
					Sections:     []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "Body."}}}
			},
			validate: func(v any) []StatusFinding {
				return validateChangeGroomShape(v.(ChangeGroomRequest))
			},
		},
		"change.reconcile": {
			valid: func() any {
				stacked := 2
				return ChangeReconcileRequest{ID: 1, Revision: "r", ReconcileLogEntry: "Reconciled.",
					Relations: &DesiredRelations{DependsOn: []int{1}, StackedOn: &stacked,
						Related: []int{3}, ADRs: []int{4}, DiscoveredFrom: []int{5}}}
			},
			validate: func(v any) []StatusFinding {
				return validateChangeReconcileShape(v.(ChangeReconcileRequest))
			},
		},
		// The bound request is the --input file; the parent id and record
		// revision ride on flags, so the validator gets valid flag values.
		"finalize.retarget-children": {
			valid: func() any {
				return RetargetChildrenInput{Children: []AuthorizedChild{{ID: 2, PRNumber: 20, PRRevision: "cv20"}}}
			},
			validate: func(v any) []StatusFinding {
				return validateRetargetShape(RetargetChildrenRequest{ID: 1, Revision: "r",
					Children: v.(RetargetChildrenInput).Children})
			},
		},
		"learning.update": {
			valid: func() any {
				return LearningUpdateRequest{Path: "p", Revision: "r", Topics: []string{"testing"},
					Changes:  []int{1},
					Sections: []SectionEditRequest{{Heading: "## Apply", Intent: "replace", Markdown: "Body."}}}
			},
			validate: func(v any) []StatusFinding {
				return validateLearningUpdateShape(v.(LearningUpdateRequest))
			},
		},
	}
	// Fields whose tag and validator knowingly disagree, keyed "<op> <path>".
	const ignoredSuccessorRequestID = "the outer request_id governs supersede and reverse; the successor's own " +
		"request_id carries ADRRecordRequest's required tag but is ignored, and splitting ADRRecordRequest " +
		"into a content type plus a wrapper is out of scope"
	exempt := map[string]string{
		"adr.reverse successor.request_id":   ignoredSuccessorRequestID,
		"adr.supersede successor.request_id": ignoredSuccessorRequestID,
	}

	coveredFixture := map[string]bool{}
	seenExempt := map[string]bool{}
	required, optional := 0, 0
	for _, b := range OperationBindings() {
		if b.Request == nil {
			continue
		}
		d, err := reflectDescriptor(b.Request)
		if err != nil {
			t.Fatalf("op %s: reflectDescriptor: %v", b.ID, err)
		}
		cases := nestedZeroCases(d.Fields, nil)
		fx, hasFixture := fixtures[b.ID]
		if hasFixture {
			coveredFixture[b.ID] = true
		}
		switch {
		case len(cases) == 0 && hasFixture:
			t.Errorf("op %s has no nested object field; drop its fixture", b.ID)
			continue
		case len(cases) == 0:
			continue
		case !hasFixture:
			t.Errorf("op %s binds request %T with nested object fields but has no fixture here", b.ID, b.Request)
			continue
		}
		base := fx.valid()
		if reflect.TypeOf(base) != reflect.TypeOf(b.Request) {
			t.Errorf("op %s: fixture is %T, want the bound request %T", b.ID, base, b.Request)
			continue
		}
		if f := fx.validate(base); len(f) != 0 {
			t.Errorf("op %s: fixture is not valid, findings %v", b.ID, findingCodes(f))
			continue
		}
		for _, c := range cases {
			name := b.ID + " " + strings.Join(c.path, ".")
			v := reflect.New(reflect.TypeOf(base)).Elem()
			v.Set(reflect.ValueOf(fx.valid()))
			if err := zeroJSONPath(v, c.path); err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			got := fx.validate(v.Interface())
			if reason, ok := exempt[name]; ok {
				seenExempt[name] = true
				if !c.required || len(got) != 0 {
					t.Errorf("%s: exemption is stale (required=%v, findings %v); drop it — it was: %s",
						name, c.required, findingCodes(got), reason)
				}
				continue
			}
			switch {
			case c.required && len(got) == 0:
				t.Errorf("%s is docket:\"required\" but the validator accepts it blank", name)
			case !c.required && len(got) > 0:
				t.Errorf("%s is not docket:\"required\" but the validator refuses it blank: %v", name, findingCodes(got))
			}
			if c.required {
				required++
			} else {
				optional++
			}
		}
	}
	for id := range fixtures {
		if !coveredFixture[id] {
			t.Errorf("fixture %s names no operation binding a request; drop it", id)
		}
	}
	for name := range exempt {
		if !seenExempt[name] {
			t.Errorf("exemption %s names no checked field; drop it", name)
		}
	}
	if required == 0 || optional == 0 {
		t.Errorf("vacuous: checked %d required and %d optional nested fields; want both > 0", required, optional)
	}
}

// nestedFixture is one operation's known-valid request (built fresh on every
// call, so a blanked copy never aliases another) and the shape validator the
// operation runs first.
type nestedFixture struct {
	valid    func() any
	validate func(any) []StatusFinding
}

// nestedZeroCase is one field the zeroing check blanks: its JSON key path from
// the request root, and whether the published descriptor marks it required.
type nestedZeroCase struct {
	path     []string
	required bool
}

// nestedZeroCases walks a published descriptor and returns every field below
// the top level, plus every object-typed field at any depth (top level
// included). Top-level scalars stay with TestRequiredTagMatchesValidator.
func nestedZeroCases(fields []FieldDescriptor, prefix []string) []nestedZeroCase {
	var out []nestedZeroCase
	for _, f := range fields {
		path := append(append([]string(nil), prefix...), f.Key)
		if len(prefix) > 0 || f.Type == "object" {
			out = append(out, nestedZeroCase{path: path, required: f.Required})
		}
		if f.Type == "object" {
			out = append(out, nestedZeroCases(f.Fields, path)...)
		}
	}
	return out
}

// zeroJSONPath sets the field at path (JSON keys from v) to its zero value. A
// pointer on the way is followed; a list on the way is descended in every
// element. A nil pointer or an empty list on the way is an error: the fixture
// must populate every nested object it asks the check to reach.
func zeroJSONPath(v reflect.Value, path []string) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return fmt.Errorf("fixture leaves the object holding %q nil; populate every nested object", path[0])
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Slice {
		if v.Len() == 0 {
			return fmt.Errorf("fixture leaves the list holding %q empty; give it one element", path[0])
		}
		for i := 0; i < v.Len(); i++ {
			if err := zeroJSONPath(v.Index(i), path); err != nil {
				return err
			}
		}
		return nil
	}
	f, ok := jsonFieldByKey(v, path[0])
	if !ok {
		return fmt.Errorf("no field with JSON key %q in %v", path[0], v.Type())
	}
	if len(path) == 1 {
		f.Set(reflect.Zero(f.Type()))
		return nil
	}
	return zeroJSONPath(f, path[1:])
}

// jsonFieldByKey finds a struct field by its JSON key: an embedded struct
// promotes its fields, and every other field's key comes from jsonFieldKey.
func jsonFieldByKey(v reflect.Value, key string) (reflect.Value, bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			if fv, ok := jsonFieldByKey(v.Field(i), key); ok {
				return fv, true
			}
			continue
		}
		if k, ok := jsonFieldKey(f); ok && k == key {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// findingCodes lists the codes of findings, for failure messages.
func findingCodes(findings []StatusFinding) []string {
	codes := make([]string, 0, len(findings))
	for _, f := range findings {
		codes = append(codes, f.Code)
	}
	return codes
}

// validADRContentFixture is an ADRRecordRequest that passes validateADRRecordShape
// with every optional field and nested object populated.
func validADRContentFixture(requestID string) ADRRecordRequest {
	return ADRRecordRequest{RequestID: requestID, Title: "t", Context: "c", Decision: "d",
		Consequences: "q", Alternatives: "a", RelatesTo: []int{1},
		Change: &ADRProducingChange{ID: 1, Path: "p", Revision: "r"}}
}

// validADRReplaceFixture is an ADRReplaceRequest that passes
// validateADRReplaceShape with every nested object populated.
func validADRReplaceFixture() ADRReplaceRequest {
	return ADRReplaceRequest{RequestID: "adr-replace-fixture",
		Target:    ADRTarget{ID: 1, Path: "p", Revision: "r"},
		Successor: validADRContentFixture("adr-successor-fixture")}
}

// flatRequiredKeys is requiredJSONKeys without the object-typed keys: a
// required object (adr.reverse/adr.supersede target and successor) is proven
// by TestNestedRequiredTagMatchesValidator's zeroing, not by the flat code
// join, which sees only top-level scalar keys.
func flatRequiredKeys(t *testing.T, prototype any) []string {
	t.Helper()
	d, err := reflectDescriptor(prototype)
	if err != nil {
		t.Fatalf("reflectDescriptor(%T): %v", prototype, err)
	}
	objects := map[string]bool{}
	for _, f := range d.Fields {
		if f.Type == "object" {
			objects[f.Key] = true
		}
	}
	out := []string{}
	for _, k := range requiredJSONKeys(prototype) {
		if !objects[k] {
			out = append(out, k)
		}
	}
	return out
}

// TestRequestJSONKeysReconcile proves RequestJSONKeys returns exactly the
// sorted top-level JSON keys DisallowUnknownFields enforces for a real request.
func TestRequestJSONKeysReconcile(t *testing.T) {
	got := RequestJSONKeys(&ChangeReconcileRequest{})
	want := []string{"id", "reconcile_log_entry", "relations", "revision", "sections", "spec_sections"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RequestJSONKeys = %v, want %v", got, want)
	}
}

// TestRequestJSONKeysSkipsAndPromotes proves a `json:"-"` field contributes no
// key and an embedded struct's fields are promoted into the key set.
func TestRequestJSONKeysSkipsAndPromotes(t *testing.T) {
	type embedded struct {
		Promoted string `json:"promoted"`
	}
	type fixture struct {
		embedded
		Kept    string `json:"kept"`
		Skipped string `json:"-"`
		Named   string `json:"renamed"`
	}
	got := RequestJSONKeys(&fixture{})
	want := []string{"kept", "promoted", "renamed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RequestJSONKeys = %v, want %v", got, want)
	}
}

// TestRequiredJSONKeysFiltersTheSharedWalk proves requiredJSONKeys is the
// RequestJSONKeys walk filtered to docket:"required": a required field promoted
// from an embedded struct counts, an optional field does not, and a `json:"-"`
// field contributes no key to either set even when it is tagged required.
func TestRequiredJSONKeysFiltersTheSharedWalk(t *testing.T) {
	type embedded struct {
		Promoted string `json:"promoted" docket:"required"`
	}
	type fixture struct {
		embedded
		Kept     string `json:"kept" docket:"required"`
		Optional string `json:"optional"`
		Skipped  string `json:"-" docket:"required"`
	}
	if got, want := RequestJSONKeys(&fixture{}), []string{"kept", "optional", "promoted"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RequestJSONKeys = %v, want %v", got, want)
	}
	if got, want := requiredJSONKeys(&fixture{}), []string{"kept", "promoted"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("requiredJSONKeys = %v, want %v", got, want)
	}
}
