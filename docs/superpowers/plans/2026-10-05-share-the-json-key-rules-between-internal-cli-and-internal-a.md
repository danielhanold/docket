<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0522 — Share the JSON-key rules between internal/cli and internal/app](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0522-share-the-json-key-rules-between-internal-cli-and-internal-a.md)**
<!-- docket:backlink:end -->
# Share the JSON-key rules between internal/cli and internal/app Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` (one tier worker per task under the `docket-build-task` contract, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the CLI's private copy of the request-struct JSON-key walk (`requestJSONKeys` in `internal/cli/requestkeys.go`) with one exported app-side function, `app.RequestJSONKeys`, that shares its walk with `requiredJSONKeys`.

**Architecture:** `internal/app/schema_tags.go` gains one private walk, `walkJSONKeys(prototype, keep)`, which promotes embedded struct fields and takes each other field's key from the existing `jsonFieldKey`. `RequestJSONKeys` is that walk keeping every field; `requiredJSONKeys` is that walk keeping only `docket:"required"` fields. `internal/cli` calls `app.RequestJSONKeys` (it already imports `app`), and `requestkeys.go` plus its test file are deleted, with the two tests moved to `internal/app`.

**Tech Stack:** Go (`reflect`, `sort`), `go test`.

**Spec:** none: change 0522 is `trivial: true`. The requirement is the change body's "What changes" section, `docs/changes/active/0522-share-the-json-key-rules-between-internal-cli-and-internal-a.md` on the `docket` metadata branch. Summary:
- export `RequestJSONKeys` in `internal/app/schema_tags.go`: one walk using `jsonFieldKey`, promoting embedded struct fields; `requiredJSONKeys` is the same walk filtered to `docket:"required"`.
- `internal/cli` uses `app.RequestJSONKeys`; delete `internal/cli/requestkeys.go`; move its two unit tests into `internal/app`.
- update the comments on `requiredJSONKeys` (`schema_tags.go`) and `reflectFields` (`schema.go`) that say they mirror the CLI's `requestJSONKeys`.
- the cross-check tests in `internal/cli/jsonfile_production_test.go` keep their intent, now against the shared function.

## Global Constraints

- Behavior-neutral refactor: the JSON-key rules, the schema tags, and validator behavior do not change (change body, "Out of scope").
- `jsonTagName` in `internal/repoguard/testexec_boundary_test.go` stays as is (test-only, answers a different question).
- Exported name is exactly `RequestJSONKeys(prototype any) []string`, returning a non-nil sorted slice.
- Cross-references in maintained source anchor on symbol names, never line numbers (AGENTS.md, ADR-0054).
- Do not edit historical plan/results files under `docs/superpowers/plans/` or `docs/results/` that mention `requestJSONKeys`; they are point-in-time records.
- Every `go test` run in a mutation or re-verification step uses `-count=1` (defeats the result cache).
- Full suite at the gate: `go run ./cmd/docket development test` (the configured `build.test_command`), run from the worktree root.

## Review Focus

1. **Variance between the two walks.** The CLI copy and `requiredJSONKeys` must be character-for-character the same walk apart from the filter before they merge. They are: the CLI's inline key rule (`tag == "-" || tag == "" && !f.IsExported()`, Go-name fallback) is exactly `jsonFieldKey`, and both promote only `f.Anonymous && f.Type.Kind() == reflect.Struct`. The one difference is pointer dereferencing: the CLI loops `for t.Kind() == reflect.Pointer`, and the app loops `for t != nil && t.Kind() == reflect.Pointer`. Keep the app's nil-safe form. Behavior is unchanged for every real caller, since all of them pass a non-nil struct or a pointer to one.
2. **A required field inside an embedded struct** must still count as required after the refactor (promotion plus the filter). This is pinned by the new `TestRequiredJSONKeysFiltersTheSharedWalk` in Task 1.
3. **A `json:"-"` field that carries `docket:"required"`** must contribute no key to either function. It is pinned in the same test.
4. **The unknown-key refusal text** (`accepted keys: …`) must stay byte-identical for every operation. `TestPublishedRequestIsTheDecodedJSONFile` and `TestPublishedRequestKeysAreAccepted` (its schema_version refusal check) in `internal/cli/jsonfile_production_test.go`, along with `internal/cli/jsonfile_test.go`'s `accepted keys: report` assert, already pin this. Task 2 runs them unchanged.
5. **A stale copy survives somewhere.** After the deletion, no maintained Go source may still define or call `requestJSONKeys`. Task 2's grep step checks this over the whole repo, not over a hand-listed set of files.

---

### Task 1: Export `RequestJSONKeys` from one shared walk in `internal/app`

**Files:**
- Modify: `internal/app/schema_tags.go` (the `jsonFieldKey` doc comment and the `requiredJSONKeys` function with its comment; add `RequestJSONKeys` and `walkJSONKeys`)
- Modify: `internal/app/schema.go` (the `reflectFields` doc comment only)
- Test: `internal/app/schema_tags_test.go` (append three tests)

**Interfaces:**
- Consumes: existing `jsonFieldKey(f reflect.StructField) (key string, ok bool)` and `hasDocketOption(tag reflect.StructTag, opt string) bool` in `internal/app/schema_tags.go`.
- Produces: `func RequestJSONKeys(prototype any) []string`, which returns the sorted, non-nil top-level JSON keys of the struct (pointers dereferenced). Task 2 calls it as `app.RequestJSONKeys`. The signature of `requiredJSONKeys(prototype any) []string` does not change.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/schema_tags_test.go` (it is `package app` and already imports `reflect` and `testing`):

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestRequestJSONKeys|TestRequiredJSONKeysFiltersTheSharedWalk'`
Expected: build failure, `undefined: RequestJSONKeys`.

- [ ] **Step 3: Implement the shared walk**

In `internal/app/schema_tags.go`, replace the whole `requiredJSONKeys` function and its doc comment with:

```go
// RequestJSONKeys returns the sorted top-level JSON keys a closed request
// struct accepts, which is the exact set DisallowUnknownFields enforces. The
// CLI lists them in an unknown-key refusal. It is walkJSONKeys keeping every
// field.
func RequestJSONKeys(prototype any) []string {
	return walkJSONKeys(prototype, func(reflect.StructField) bool { return true })
}

// requiredJSONKeys returns the sorted top-level JSON keys of prototype whose
// field carries docket:"required": the RequestJSONKeys walk filtered to
// required-tagged fields. The request-shape validator tests and the schema
// surface consume it.
func requiredJSONKeys(prototype any) []string {
	return walkJSONKeys(prototype, func(f reflect.StructField) bool {
		return hasDocketOption(f.Tag, "required")
	})
}

// walkJSONKeys is the one walk of a struct's top-level JSON keys behind
// RequestJSONKeys and requiredJSONKeys. It dereferences pointers, lets an
// embedded struct promote its fields, takes every other field's key from
// jsonFieldKey, and keeps a key only when keep reports true for its field.
// The result is sorted and non-nil.
func walkJSONKeys(prototype any, keep func(reflect.StructField) bool) []string {
	t := reflect.TypeOf(prototype)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	seen := map[string]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				walk(f.Type)
				continue
			}
			key, ok := jsonFieldKey(f)
			if !ok {
				continue
			}
			if keep(f) {
				seen[key] = true
			}
		}
	}
	walk(t)
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```

In the same file, update the first two lines of the `jsonFieldKey` doc comment so the list of sharers names the shared walk:

```go
// jsonFieldKey derives the JSON key of one non-embedded struct field, the
// single copy of the key rules reflectFields, walkJSONKeys (behind
// RequestJSONKeys and requiredJSONKeys), and the tests' field lookup share:
// a `json:"-"` field and an untagged unexported field contribute nothing (ok
// is false), and an untagged exported field falls back to its Go field name.
// Promoting an embedded struct's fields is the caller's walk.
```

In `internal/app/schema.go`, replace the `reflectFields` doc comment's sentence "It mirrors requiredJSONKeys / the CLI's requestJSONKeys walk:" so the comment reads:

```go
// reflectFields walks a struct type's fields into descriptors, in declaration
// order. It walks the same shape as walkJSONKeys (behind RequestJSONKeys and
// requiredJSONKeys): an embedded struct promotes its fields inline; a
// `json:"-"` field and an untagged unexported field contribute nothing; an
// untagged exported field falls back to its Go field name. The docket tag (via
// Task 4's helpers) supplies Required, Presence, and Enum.
```

(Leave everything after the comment untouched. "Task 4's helpers" is pre-existing wording and outside this change's scope.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestRequestJSONKeys|TestRequiredJSONKeysFiltersTheSharedWalk|TestRequiredTagMatchesValidator|TestNestedRequiredTagMatchesValidator'`
Expected: PASS. The last two are existing consumers of `requiredJSONKeys`, and they must stay green.

- [ ] **Step 5: Mutation-check the promotion and filter**

Back up the file with `cp internal/app/schema_tags.go "${TMPDIR:-/tmp}/schema_tags.go.bak"`. Do not use `git checkout --`, because it would restore HEAD and discard this task's uncommitted edit. Then apply each mutation alone, run the command, and restore with `cp -f "${TMPDIR:-/tmp}/schema_tags.go.bak" internal/app/schema_tags.go`:

1. In `walkJSONKeys`, delete the line `walk(f.Type)` from the embedded-struct branch. Expected: `TestRequestJSONKeysSkipsAndPromotes` and `TestRequiredJSONKeysFiltersTheSharedWalk` FAIL.
2. Change `requiredJSONKeys`'s keep func to `return true`. Expected: `TestRequiredJSONKeysFiltersTheSharedWalk` FAILS on the `requiredJSONKeys` assert.

Run for each: `go test -count=1 ./internal/app/ -run 'TestRequestJSONKeys|TestRequiredJSONKeysFiltersTheSharedWalk'`

After restoring, confirm `git diff --stat` shows only this task's three files and that Step 4's command passes again.

- [ ] **Step 6: Commit**

```bash
git add internal/app/schema_tags.go internal/app/schema.go internal/app/schema_tags_test.go
git commit -m "refactor(app): export RequestJSONKeys from one walk shared with requiredJSONKeys"
```

---

### Task 2: Point `internal/cli` at `app.RequestJSONKeys` and delete the CLI copy

**Files:**
- Modify: `internal/cli/change.go` (`decodeRequest`, its unknown-field refusal)
- Modify: `internal/cli/jsonfile_production_test.go` (two call sites: in `TestPublishedRequestIsTheDecodedJSONFile`, and in `TestPublishedRequestKeysAreAccepted`)
- Delete: `internal/cli/requestkeys.go`
- Delete: `internal/cli/requestkeys_test.go` (its two tests now live in `internal/app/schema_tags_test.go` from Task 1)

**Interfaces:**
- Consumes: `app.RequestJSONKeys(prototype any) []string` from Task 1.
- Produces: nothing new.

- [ ] **Step 1: Delete the CLI copy and its tests**

```bash
git rm internal/cli/requestkeys.go internal/cli/requestkeys_test.go
```

- [ ] **Step 2: Verify the CLI no longer builds**

Run: `go vet ./internal/cli/`
Expected: FAIL with `undefined: requestJSONKeys`, reported at three sites: `change.go` in `decodeRequest`, and two in `jsonfile_production_test.go`.

- [ ] **Step 3: Rewire the call sites**

In `internal/cli/change.go`, inside `decodeRequest`, change

```go
			return fmt.Errorf("decoding %s JSON: %w (accepted keys: %s)", flagName, err, strings.Join(requestJSONKeys(dst), ", "))
```

to

```go
			return fmt.Errorf("decoding %s JSON: %w (accepted keys: %s)", flagName, err, strings.Join(app.RequestJSONKeys(dst), ", "))
```

(`change.go` already imports `github.com/danielhanold/docket/internal/app`.)

In `internal/cli/jsonfile_production_test.go` (already imports `app`), change

```go
			if got := requestJSONKeys(b.Request); !reflect.DeepEqual(got, published) {
```

to

```go
			if got := app.RequestJSONKeys(b.Request); !reflect.DeepEqual(got, published) {
```

and change

```go
		accepted := "accepted keys: " + strings.Join(requestJSONKeys(reflect.New(typ).Interface()), ", ")
```

to

```go
		accepted := "accepted keys: " + strings.Join(app.RequestJSONKeys(reflect.New(typ).Interface()), ", ")
```

- [ ] **Step 4: Prove no copy or caller of the old name survives**

Run from the worktree root (this derives the sites from the whole repo instead of a hand-made list):

```bash
out=$(grep -rn -e 'requestJSONKeys' --include='*.go' . || true); printf '%s\n' "$out"
```

Expected: empty output. Hits in `docs/superpowers/plans/*.md` and `docs/results/*.md` are historical records and are excluded by `--include='*.go'`. Leave them alone.

- [ ] **Step 5: Run the affected packages**

Run: `go build ./... && go vet ./internal/cli/ ./internal/app/ && go test -count=1 -v ./internal/cli/ -run 'TestPublishedRequestIsTheDecodedJSONFile|TestPublishedRequestKeysAreAccepted|TestDeclareJSONFile|TestRetiredRevisionRequestKeysRefused|UnknownField|InputDecode' && go test -count=1 ./internal/app/ -run 'TestRequestJSONKeys|TestRequiredJSONKeysFiltersTheSharedWalk|TestRequiredTagMatchesValidator'`
Expected: PASS. In the `-v` output, confirm that `TestPublishedRequestIsTheDecodedJSONFile`, `TestPublishedRequestKeysAreAccepted`, and `TestUnknownFieldErrorListsAcceptedKeys` each report `--- PASS`. A `-run` pattern that matches nothing still exits 0, so the exit code alone proves nothing.

- [ ] **Step 6: Run the full suite (build gate)**

Run: `go run ./cmd/docket development test`
Expected: green. Read the budget report even on a green run, and record any `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, or `SERIAL CONFIRMED OVER BUDGET:` line in the build evidence.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/change.go internal/cli/jsonfile_production_test.go
git commit -m "refactor(cli): use app.RequestJSONKeys and drop the CLI's copy of the key walk"
```

(The `git rm` in Step 1 already staged both deletions.)
