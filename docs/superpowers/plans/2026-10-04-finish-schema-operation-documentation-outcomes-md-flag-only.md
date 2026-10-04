<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0521 — Mark every nested required request field in the schema, and fix the stale schema docs](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0521-finish-schema-operation-documentation-outcomes-md-flag-only.md)**
<!-- docket:backlink:end -->
# Mark every nested required request field, and fix the stale schema docs: Implementation Plan

> **For agentic workers:** `docket-build` executes this plan. It sends each `### Task N` to one
> tier worker that follows the `docket-build-task` contract. The tasks run in order, and each one
> builds on the commit before it. Steps use checkbox (`- [ ]`) syntax for tracking. Do not tick
> them, because your commit is the progress record.

**Goal:** A request field at any depth carries `docket:"required"` exactly when the shape
validator always refuses the request if that field alone is blank, a test proves this for every
nested field by blanking it, and the two docs that say every operation shows a request shape stop
saying so.

**Architecture:** Tags only in `internal/app` (no validator changes). `reflectFields` already
reports the tag at every depth, so the published schema picks the new tags up for free. A new test
in `internal/app/schema_tags_test.go` walks the descriptor `docket schema` publishes, starts each
operation from a known-valid fixture, blanks one nested field (or one object/list field at any
depth) at a time, and runs the operation's shape validator: required means at least one finding,
not required means zero findings. The existing flat test stops comparing object-typed keys, which
the new test now proves. The docs edit touches `docs/reference/outcomes.md` and
`skills/docket-convention/references/close-out.md`; the embedded copy is regenerated with
`go generate ./internal/assets/`.

**Tech Stack:** Go 1.26 (`go.mod`), `reflect`, the standard `testing` package.

**Spec:** `docs/superpowers/specs/2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only-design.md`
on the `docket` branch. Read it through the synchronized metadata worktree:
`/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only-design.md`.

## Global Constraints

- No validator behavior changes. No finding-code changes. Only `docket:"required"` struct tags
  (and their doc comments) change in production code.
- No new `docket:` tag vocabulary (no conditional-required, no "ignored").
- Do not bump `SchemaVersion` (`internal/app/schema.go`).
- Do not fix `successor.request_id` (published required, ignored by supersede/reverse). It is a
  known, accepted mismatch: exempt it in the test with a reason and report it for the results file.
- A field required only under a condition stays untagged (example: `sections[].markdown`, required
  only for `intent: replace`). `ADRRecordRequest.change` (pointer) and
  `RetargetChildrenInput.children` stay optional.
- Docs describe current behavior only: no change numbers, no PR citations.
- The embedded copy under `internal/assets/embedded/tree/` is never hand-edited. Edit the source
  file, then run `go generate ./internal/assets/` from the worktree root.
- `skills/docket-convention/references/close-out.md` is at its exact size ceiling
  (`internal/repoguard/budgets_test.go`: 141 lines, 1349 words). The edit must not add lines or
  words. Never raise the ceiling.
- Skill files must not gain a `docket <argv>` literal (`TestCapabilitySurface`). Name the `schema`
  operation and "the capability catalog" in prose instead.
- Every mutation probe and every re-verification runs with `go test -count=1` (the Go test cache can
  serve a stale pass). Restore a mutated file from a backup copy (`cp f f.bak` … `mv -f f.bak f`),
  never `git checkout --`.
- The whole-suite gate is `go run ./cmd/docket development test`; `docket-build` runs it at the end.
  Tasks run focused `go test` commands only.

## Review Focus

- **A request type gains a nested object later** (a new binding, or a new nested field on an
  existing one): the new test must redden until a fixture covers it. Pinned in Task 1 by the
  completeness loop and by the "drop one fixture" mutation probe.
- **A fixture leaves an optional nested object nil or a list empty:** the nested fields under it
  would never be checked. Pinned in Task 1: `zeroJSONPath` returns an error for a nil pointer or an
  empty list on the path, and the test reports it.
- **The `successor.request_id` mismatch is later fixed** (tag dropped or validator changed): the
  exemption must not linger. Pinned in Task 1 by the stale-exemption check and the "delete the
  exemption" mutation probe.
- **An agent reads the schema for `adr.record` without `change` and believes `change.id` is
  mandatory:** the doc must say a required field inside an optional object is required only when
  that object is sent. Pinned in Task 2 (the `outcomes.md` sentence) and in the `required` vocabulary
  comment in Task 1.
- **The embedded `close-out.md` drifts from the source** or the edit breaks the size budget: pinned
  in Task 2 by `go test -count=1 ./internal/assets/ ./internal/repoguard/`.

---

### Task 1: Tag every nested required request field, proven by a field-zeroing test

**Build tier:** standard

**Files:**
- Modify: `internal/app/schema_tags_test.go` (flat test adjustment, new test, helpers)
- Modify: `internal/app/adr_ops.go` (`ADRProducingChange`, `ADRTarget`, `ADRReplaceRequest`)
- Modify: `internal/app/change_groom.go` (`SectionEditRequest`)
- Modify: `internal/app/finalize_retarget.go` (`AuthorizedChild`)
- Modify: `internal/app/schema_tags.go` (the `required` vocabulary comment only)

**Interfaces:**
- Consumes (existing, unchanged): `OperationBindings() []OperationBinding` (`ID string`,
  `Request any`); `reflectDescriptor(prototype any) (TypeDescriptor, error)`;
  `FieldDescriptor{Key, Type, Required, Fields, ...}` (`Type == "object"` marks a struct, pointer to
  struct, or list of structs); `requiredJSONKeys(prototype any) []string`; the shape validators
  `validateADRRecordShape(ADRRecordRequest)`, `validateADRReplaceShape(ADRReplaceRequest)`,
  `validateChangeGroomShape(ChangeGroomRequest)`, `validateChangeReconcileShape(ChangeReconcileRequest)`,
  `validateRetargetShape(RetargetChildrenRequest)`, `validateLearningUpdateShape(LearningUpdateRequest)`,
  each returning `[]StatusFinding`. Each operation calls its validator first and refuses on any
  finding, so a valid fixture never reaches a dependency seam (do NOT call `ADRReverse`,
  `LearningUpdate`, … with a valid fixture: they would dereference nil deps).
- Produces (test-only, in package `app`): `TestNestedRequiredTagMatchesValidator`;
  `type nestedFixture struct{ valid func() any; validate func(any) []StatusFinding }`;
  `type nestedZeroCase struct{ path []string; required bool }`;
  `nestedZeroCases([]FieldDescriptor, []string) []nestedZeroCase`;
  `zeroJSONPath(reflect.Value, []string) error`; `jsonFieldByKey(reflect.Value, string) (reflect.Value, bool)`;
  `flatRequiredKeys(*testing.T, any) []string`; `findingCodes([]StatusFinding) []string`.

- [ ] **Step 1: Write the failing nested test**

Append to `internal/app/schema_tags_test.go`, and add `"fmt"` to its import block:

```go
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

// jsonFieldByKey finds a struct field by its JSON key, with the key rules
// reflectFields uses: an embedded struct promotes its fields, `json:"-"` and an
// untagged unexported field contribute nothing, and an untagged exported field
// falls back to its Go name.
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
		k := strings.Split(f.Tag.Get("json"), ",")[0]
		if k == "-" || (k == "" && !f.IsExported()) {
			continue
		}
		if k == "" {
			k = f.Name
		}
		if k == key {
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
```

The fixture values are a draft: if Step 2 reports `fixture is not valid`, fix the fixture value
(read the named validator to see which input it rejects), never the validator.

- [ ] **Step 2: Run it and confirm it fails for the missing tags only**

Run: `go test -count=1 -run 'TestNestedRequiredTagMatchesValidator' ./internal/app/`

Expected: FAIL. Every failure line reads `<op> <path> is not docket:"required" but the validator
refuses it blank`, for exactly these paths: `adr.record change.id|change.path|change.revision`;
`adr.reverse` and `adr.supersede` `target`, `target.id|path|revision`, `successor`,
`successor.change.id|path|revision`; `change.groom sections.heading|sections.intent`;
`learning.update sections.heading|sections.intent`;
`finalize.retarget-children children.id|children.pr_number|children.pr_revision`.
There must be NO `fixture is not valid`, no `exemption is stale`, and no `accepts it blank` line.

- [ ] **Step 3: Add the tags**

`internal/app/adr_ops.go`:

```go
type ADRProducingChange struct {
	ID       int    `json:"id" docket:"required"`
	Path     string `json:"path" docket:"required"`
	Revision string `json:"revision" docket:"required"`
}
```

```go
type ADRTarget struct {
	ID       int    `json:"id" docket:"required"`
	Path     string `json:"path" docket:"required"`
	Revision string `json:"revision" docket:"required"`
}
```

```go
// ADRReplaceRequest is the closed, caller-supplied request for one supersede or
// reverse. RequestID governs idempotency; Target pins the flipped ADR; Successor
// carries the brand-new ADR's authored content and references (its own RequestID
// is ignored — the outer key governs — although the published schema marks it
// required, since the successor reuses ADRRecordRequest).
type ADRReplaceRequest struct {
	RequestID string           `json:"request_id" docket:"required"`
	Target    ADRTarget        `json:"target" docket:"required"`
	Successor ADRRecordRequest `json:"successor" docket:"required"`
}
```

`internal/app/change_groom.go`:

```go
type SectionEditRequest struct {
	Heading  string `json:"heading" docket:"required"`
	Intent   string `json:"intent" docket:"required"` // preserve|replace|remove
	Markdown string `json:"markdown,omitempty"`
}
```

`internal/app/finalize_retarget.go`:

```go
type AuthorizedChild struct {
	ID         int    `json:"id" docket:"required"`
	PRNumber   int    `json:"pr_number" docket:"required"`
	PRRevision string `json:"pr_revision" docket:"required"`
}
```

`internal/app/schema_tags.go`, the `required` line of the vocabulary comment becomes:

```go
//	required            the field must be present/non-zero; a shape validator
//	                    mints an error finding on its absence. On a field inside
//	                    an optional object or a list element, it applies whenever
//	                    that object or element is sent.
```

- [ ] **Step 4: Adjust the flat test so object-typed keys are proven by zeroing**

Tagging `target` and `successor` makes `requiredJSONKeys(ADRReplaceRequest{})` return
`[request_id successor target]`, which the flat code join cannot see (their findings are
`invalid-target-id`, `empty-title`, …). In `TestRequiredTagMatchesValidator`, replace

```go
			want := requiredJSONKeys(tc.prototype)
```

with

```go
			want := flatRequiredKeys(t, tc.prototype)
```

and append:

```go
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
```

Replace the comment above the flat `adr.reverse` case with:

```go
		// adr.reverse/adr.supersede: the flat key join sees only top-level
		// scalar keys, so the nested target and successor are supplied valid
		// (their findings would otherwise name keys such as "target-id") and
		// the remaining findings must name exactly the top-level scalar
		// required keys. The required target and successor objects themselves
		// are proven by TestNestedRequiredTagMatchesValidator.
```

- [ ] **Step 5: Run the focused tests and confirm they pass**

Run: `go test -count=1 -run 'TestRequiredTagMatchesValidator|TestNestedRequiredTagMatchesValidator|TestReflectDescriptor|TestSchema' ./internal/app/`
Expected: PASS.

Run: `go test -count=1 -run 'Schema|Retarget|Finalize' ./internal/cli/` and `go vet ./internal/app/`
Expected: PASS, no vet output.

- [ ] **Step 6: Mutation-probe the guard**

Each probe: back up, mutate, confirm the mutation landed, run, restore. Run from the worktree root.
Every probe must turn the test RED with the named failure; a green reading is a defect in the
guard, not a pass.

Probe A — remove the tag from `ADRTarget.path`:

```bash
f=internal/app/adr_ops.go; cp "$f" "$f.bak"
perl -0pi -e 's/(type ADRTarget struct \{.*?`json:"path") docket:"required"/$1/s' "$f"
grep -n -A4 'type ADRTarget struct' "$f"
go test -count=1 -run 'TestNestedRequiredTagMatchesValidator' ./internal/app/
mv -f "$f.bak" "$f"
```

Expected: FAIL with `adr.reverse target.path is not docket:"required" but the validator refuses it blank`
(and the same for `adr.supersede`).

Probe B — remove the tag from `AuthorizedChild.pr_revision`:

```bash
f=internal/app/finalize_retarget.go; cp "$f" "$f.bak"
perl -0pi -e 's/(type AuthorizedChild struct \{.*?`json:"pr_revision") docket:"required"/$1/s' "$f"
grep -n -A4 'type AuthorizedChild struct' "$f"
go test -count=1 -run 'TestNestedRequiredTagMatchesValidator' ./internal/app/
mv -f "$f.bak" "$f"
```

Expected: FAIL with `finalize.retarget-children children.pr_revision is not docket:"required" but the validator refuses it blank`.

Probe C — add a tag to `SectionEditRequest.markdown`:

```bash
f=internal/app/change_groom.go; cp "$f" "$f.bak"
perl -0pi -e 's/(type SectionEditRequest struct \{.*?`json:"markdown,omitempty")/$1 docket:"required"/s' "$f"
grep -n -A4 'type SectionEditRequest struct' "$f"
go test -count=1 -run 'TestNestedRequiredTagMatchesValidator' ./internal/app/
mv -f "$f.bak" "$f"
```

Expected: FAIL with `change.groom sections.markdown is docket:"required" but the validator accepts it blank`
(and the same for `learning.update`).

Probe D — drop one fixture: `cp internal/app/schema_tags_test.go internal/app/schema_tags_test.go.bak`,
delete the whole `"change.reconcile": {...},` entry from the `fixtures` map with the Edit tool, run
`go test -count=1 -run 'TestNestedRequiredTagMatchesValidator' ./internal/app/`, then
`mv -f internal/app/schema_tags_test.go.bak internal/app/schema_tags_test.go`.
Expected: FAIL with `op change.reconcile binds request app.ChangeReconcileRequest with nested object fields but has no fixture here`.

Probe E — delete one exemption: same backup procedure, delete the
`"adr.reverse successor.request_id": ignoredSuccessorRequestID,` line, run, restore.
Expected: FAIL with `adr.reverse successor.request_id is docket:"required" but the validator accepts it blank`.

After all probes: `git status --porcelain` shows only the five files of this task, no `.bak`
files, and `go test -count=1 -run 'TestRequiredTagMatchesValidator|TestNestedRequiredTagMatchesValidator' ./internal/app/` passes.

Put the five probe readings (mutation, observed failure line) and the accepted
`successor.request_id` mismatch (published required, ignored by supersede/reverse, exempted in the
test, not fixed) in your task report; the results file records them.

- [ ] **Step 7: Commit**

```bash
git add internal/app/schema_tags_test.go internal/app/adr_ops.go internal/app/change_groom.go internal/app/finalize_retarget.go internal/app/schema_tags.go
git commit -m "fix(schema): mark every nested required request field and prove it by zeroing"
```

---

### Task 2: Say that flag-only operations publish no request shape

**Build tier:** economy

**Files:**
- Modify: `docs/reference/outcomes.md` (the opening paragraph)
- Modify: `skills/docket-convention/references/close-out.md` (the header blockquote's last clause)
- Regenerate: `internal/assets/embedded/tree/skills/docket-convention/references/close-out.md` and
  `internal/assets/embedded/manifest.json` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: nothing from Task 1 (the doc sentence describes the tag rule Task 1 applied).
- Produces: nothing code-facing.

- [ ] **Step 1: Confirm no test greps the wording being removed**

Run: `grep -rn "request and result shape" --include='*.go' --include='*.sh' internal cmd tests`
Expected: no output. (If a test matches, repoint it at the new wording's claim; do not keep the old
phrase.)

- [ ] **Step 2: Edit `docs/reference/outcomes.md`**

Replace the opening paragraph (the one ending "shows one operation's request and result shape.")
with:

```markdown
docket reports what happened in a small set of fixed words. This page names each vocabulary and
the surface that owns its current members. The machine-readable source for all of them is
`docket schema` (read-only, repository-independent): `docket schema --json` lists every closed
vocabulary, and `docket schema --operation <id>` shows one operation's result shape. An operation
that reads a JSON file (`--request`, `--input`, or `--body`) also shows that file's request shape.
A flag-only operation shows no request; `docket capabilities --json` lists its flags in its
`signature`. A required field inside an optional object is required only when that object is sent.
```

- [ ] **Step 3: Edit `skills/docket-convention/references/close-out.md`**

Replace these two lines of the header blockquote:

```markdown
> failure posture differs per caller (table below). This file owns ordering and posture; each
> operation's request and result shape comes from the `schema` operation.
```

with:

```markdown
> failure posture differs per caller (table below). This file owns ordering and posture.
> `schema` gives request-file and result shapes, the capability catalog gives flags.
```

Then run `wc -lw skills/docket-convention/references/close-out.md`.
Expected: at most `141` lines and at most `1349` words. If over, shorten this sentence; never
raise the ceiling in `internal/repoguard/budgets_test.go`. Do not write a `docket <argv>` literal.

- [ ] **Step 4: Regenerate the embedded copy**

Run from the worktree root: `go generate ./internal/assets/`
Then: `git status --porcelain`
Expected: modified `docs/reference/outcomes.md`, `skills/docket-convention/references/close-out.md`,
`internal/assets/embedded/tree/skills/docket-convention/references/close-out.md`, and
`internal/assets/embedded/manifest.json`, and nothing else. Run
`diff skills/docket-convention/references/close-out.md internal/assets/embedded/tree/skills/docket-convention/references/close-out.md`
Expected: no output.

- [ ] **Step 5: Run the doc and asset guards**

Run: `go test -count=1 ./internal/assets/ ./internal/repoguard/`
Expected: PASS (covers the embedded-manifest drift check, `TestSkillSizeBudgets`,
`TestCapabilitySurface`, the living-docs alignment guard, and the prose contracts over
`close-out.md`).

- [ ] **Step 6: Commit**

```bash
git add docs/reference/outcomes.md skills/docket-convention/references/close-out.md internal/assets/embedded/tree/skills/docket-convention/references/close-out.md internal/assets/embedded/manifest.json
git commit -m "docs: say flag-only operations publish no request shape"
```
