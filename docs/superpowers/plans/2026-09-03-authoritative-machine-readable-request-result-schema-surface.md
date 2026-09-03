<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0399 — Authoritative machine-readable request/result schema surface](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-03-0399-authoritative-machine-readable-request-result-schema-surface.md)**
<!-- docket:backlink:end -->
# Authoritative Machine-Readable Request/Result Schema Surface — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: this plan is executed by the **docket-build** skill task-by-task (its profile workers run the docket-build-task contract). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one read-only, versioned `docket schema` operation that derives every operation's request-body, result/envelope, and closed-vocabulary descriptors from the live Go types — plus the message fixes that make findings name the real JSON keys and flags.

**Architecture:** A reflection-based generator in `internal/app` walks the live `*Request`/`*Result` structs registered in a new operation-schema registry, reading co-located `docket:` struct tags for required-ness, success/refusal presence, and enum references; a new `docket schema` Cobra leaf (own capability id, `read` effect, no repo/config/git/network access) emits the protocol-v1 document with its own fail-closed `schema_version`. A typed finding-code registry replaces inline string minting, guarded by a repo-wide shape scan. Correspondence and mutation guards mirror change 0394's catalog guards in both directions.

**Tech Stack:** Go (reflect, go/ast for guards), Cobra, the existing protocol-v1 envelope (`internal/app/result.go`), the existing capability annotation machinery (`internal/cli/capability.go`).

**Spec:** `docs/superpowers/specs/2026-09-03-authoritative-machine-readable-request-result-schema-surface-design.md` (on the `docket` metadata branch; synchronized copy read at plan time). The change file is `docs/changes/active/0399-authoritative-machine-readable-request-result-schema-surface.md` on the same branch.

## Global Constraints

- The suite gate is `go run ./cmd/docket development test` (Go-native runner; run the WHOLE suite at the gate, never only the tests a task names).
- Every mutation probe defeats Go's test cache: `go test -count=1 <pkg>` — a `(cached)` verdict is absence of evidence (learnings: cached-runner-serves-a-mutated-tree).
- Every guard is mutation-tested: strip the guarded thing, watch the guard redden, restore. Key guards on syntactic **shape**, never an enumerated list of spellings; derive site lists from whole-repo greps, never by hand (AGENTS.md).
- Correspondence guards run in **both** directions plus the pairing, and are mutation-tested in both directions (learnings: correspondence-guard-runs-one-way).
- Message-fix tasks (1–2) land FIRST on the branch (spec, Resolved question 5).
- Out of scope: growing `capabilities` or its 12 KB budget; standard JSON Schema output; a validate/dry-run flag; per-command `--json-schema` sugar; MCP; the other 0360 legs.
- JSON field names/types in emitted documents are protocol: additive within v1; the schema document carries its own `schema_version` integer (starts at 1), independent of `protocol_version` and `capability_version`.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).
- Commits end with the trailer `Claude-Session: https://claude.ai/code/session_016rpn4gcoGkh979tHWA74aK`.

---

### Task 1: Decode errors name the real flag and list accepted keys

The shared decoder `decodeRequest` (internal/cli/change.go) hardcodes `--request` into all three of its error messages, but `decodeInputFlag` routes `--input` commands through it — so `change reconcile --input …` fails with "decoding --request JSON". Additionally, an unknown-field error (`json: unknown field "nope"`) does not list the accepted keys.

**Files:**
- Modify: `internal/cli/change.go` (functions `decodeRequestFlag`, `decodeInputFlag`, `decodeRequest`)
- Create: `internal/cli/requestkeys.go` (accepted-key reflection helper — Task 5's generator is app-side and cannot be imported by cli without a dependency inversion; this helper is deliberately tiny and top-level-only)
- Test: `internal/cli/change_test.go` (extend), `internal/cli/requestkeys_test.go`

**Interfaces:**
- Produces: `decodeRequest(stdin io.Reader, flagName, source string, dst any) error` — `flagName` is the literal registered flag (`"--request"` or `"--input"`), interpolated into every message.
- Produces: `requestJSONKeys(dst any) []string` — sorted top-level JSON keys of dst's struct (dereferencing pointers), used to append `(accepted keys: a, b, c)` to unknown-field errors.

- [ ] **Step 1: Write the failing tests.** In `internal/cli/change_test.go`, extend the existing `--input` tests (see `TestChangeReconcileUnknownFieldRejected` as the pattern — it runs `runCLIStdin(t, `{"id":1,"nope":true}`, "change", "reconcile", "--input", "-", "--json")`):

```go
// TestInputDecodeErrorNamesInputFlag proves a malformed --input body's error
// names the flag the caller actually passed, never --request.
func TestInputDecodeErrorNamesInputFlag(t *testing.T) {
	_, errS, code := runCLIStdin(t, `{not json`, "change", "reconcile", "--input", "-", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr %q)", code, errS)
	}
	if !strings.Contains(errS, "--input") || strings.Contains(errS, "--request") {
		t.Errorf("decode error must name --input and not --request: %q", errS)
	}
}

// TestUnknownFieldErrorListsAcceptedKeys proves an unknown-field refusal
// teaches the caller the real key set instead of only naming the bad key.
func TestUnknownFieldErrorListsAcceptedKeys(t *testing.T) {
	_, errS, code := runCLIStdin(t, `{"change_id":1}`, "change", "reconcile", "--input", "-", "--json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr %q)", code, errS)
	}
	for _, key := range []string{"id", "version", "sections", "spec_sections", "relations", "reconcile_log_entry"} {
		if !strings.Contains(errS, key) {
			t.Errorf("unknown-field error must list accepted key %q: %q", key, errS)
		}
	}
}
```

Also write `internal/cli/requestkeys_test.go` unit tests: `requestJSONKeys(&app.ChangeReconcileRequest{})` returns exactly the six keys above sorted; a field tagged `json:"-"` is excluded; an embedded struct's promoted keys are included (verify against a small local fixture struct, not only app types).

- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/cli/ -run 'TestInputDecodeError|TestUnknownFieldError|TestRequestJSONKeys' -v` — expect FAIL (message currently says `--request`; no key list; helper undefined).

- [ ] **Step 3: Implement.** In `internal/cli/requestkeys.go`:

```go
package cli

import (
	"reflect"
	"sort"
	"strings"
)

// requestJSONKeys returns the sorted top-level JSON keys a closed request
// struct accepts — the exact set DisallowUnknownFields enforces. Embedded
// structs contribute their promoted keys; `json:"-"` fields contribute none.
func requestJSONKeys(dst any) []string {
	t := reflect.TypeOf(dst)
	for t.Kind() == reflect.Pointer {
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
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag == "-" || tag == "" && !f.IsExported() {
				continue
			}
			if tag == "" {
				tag = f.Name
			}
			seen[tag] = true
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

In `change.go`, thread the flag name through: `decodeRequestFlag` calls `decodeRequest(c.InOrStdin(), "--request", source, dst)`; `decodeInputFlag` calls `decodeRequest(c.InOrStdin(), "--input", source, dst)`. Inside `decodeRequest`, replace the three hardcoded messages with `flagName`-interpolated forms (`"reading %s %q: %w"`, `"decoding %s JSON: %w"`, `"%s must contain exactly one JSON document"`), and on a decode error whose text contains `"unknown field"`, wrap it as:

```go
if err := dec.Decode(dst); err != nil {
	if strings.Contains(err.Error(), "unknown field") {
		return fmt.Errorf("decoding %s JSON: %w (accepted keys: %s)", flagName, err, strings.Join(requestJSONKeys(dst), ", "))
	}
	return fmt.Errorf("decoding %s JSON: %w", flagName, err)
}
```

- [ ] **Step 4: Verify green + no regression.** `go test -count=1 ./internal/cli/` — all pass. Grep for other callers of `decodeRequest` (adr.go's `decodeRequestFlag` sites compile unchanged; `git grep -n 'decodeRequest('` must show only the two wrappers as direct callers).

- [ ] **Step 5: Commit.** `git add internal/cli/change.go internal/cli/requestkeys.go internal/cli/requestkeys_test.go internal/cli/change_test.go && git commit -m "fix(cli): decode errors name the actual flag and list accepted keys"`

---

### Task 2: Lifecycle-shape findings name the real JSON key

`validateLifecycleShape` (internal/app/change_lifecycle.go) hardcodes `change_id`/`invalid-change_id`, but most of its 15 call sites validate requests whose JSON key is `id` (reconcile, claim, halt, implemented, reclaim, attach, finalize.*) — only `ChangeBlockRequest`, `ChangeDeferRequest`, `ChangeKillRequest`, and `ChangeGroomRequest` actually spell it `change_id`.

**Files:**
- Modify: `internal/app/change_lifecycle.go` (`validateLifecycleShape` + both lifecycle callers), every other caller found by `git grep -n 'validateLifecycleShape(' internal/app/ | grep -v _test`
- Test: `internal/app/change_lifecycle_test.go` (extend), plus whichever existing tests assert `invalid-change_id` (find them: `git grep -rn 'invalid-change_id' internal/`)

**Interfaces:**
- Produces: `validateLifecycleShape(idKey string, id int, recPath, version string) []StatusFinding` — `idKey` is the request's real JSON key (`"id"` or `"change_id"`); the finding code becomes `"invalid-" + idKey` and the message `idKey + " must be a positive change id"`.

- [ ] **Step 1: Write the failing test.**

```go
// TestLifecycleShapeFindingNamesRealKey proves the id-shape finding carries the
// JSON key the request actually decodes — "id" for reconcile-family requests,
// "change_id" for block/defer/kill/groom — in both code and message.
func TestLifecycleShapeFindingNamesRealKey(t *testing.T) {
	got := validateLifecycleShape("id", 0, "p", "v")
	if len(got) != 1 || got[0].Code != "invalid-id" || !strings.Contains(got[0].Message, "id must be") {
		t.Errorf("id-keyed shape finding = %+v, want code invalid-id naming key id", got)
	}
	got = validateLifecycleShape("change_id", 0, "p", "v")
	if len(got) != 1 || got[0].Code != "invalid-change_id" {
		t.Errorf("change_id-keyed shape finding = %+v, want code invalid-change_id", got)
	}
}
```

- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/app/ -run TestLifecycleShapeFindingNamesRealKey` — FAIL (wrong arity).

- [ ] **Step 3: Implement.** Change the signature and body:

```go
func validateLifecycleShape(idKey string, id int, recPath, version string) []StatusFinding {
	var findings []StatusFinding
	if id <= 0 {
		findings = append(findings, lifecycleFinding("invalid-"+idKey, idKey+" must be a positive change id"))
	}
	...
}
```

Update every caller: pass `"change_id"` from `ChangeBlock`, `ChangeDefer`, `ChangeKill`, `ChangeGroom` (grep groom for its own shape validator — if it mints its own finding, leave it; only rewire actual `validateLifecycleShape` callers); pass `"id"` from every caller whose request struct tags the field `json:"id"` (`change_attach.go`, `change_claim.go` ×2, `change_halt.go` ×2, `change_implemented.go`, `change_reclaim.go`, `change_reconcile.go`, `finalize_block.go` ×2, `finalize_rebase.go`, `finalize_merge.go`, `finalize_retarget.go`). Derive each key by reading that request struct's json tag, not from this list — the list above is the plan-time survey, the struct tag is the authority.

- [ ] **Step 4: Fix the reddened assertions deliberately.** `go test -count=1 ./internal/app/ ./internal/cli/` — tests asserting `invalid-change_id` for `id`-keyed ops now fail; update each to `invalid-id` ONLY where the request's real key is `id` (this is the behavior change shipping, not test laundering — say so in the commit body). Tests for block/defer/kill/groom keep `invalid-change_id`.

- [ ] **Step 5: Commit.** `git add -u internal/ && git commit -m "fix(app): lifecycle shape findings name the request's real JSON id key"` (verify `git status` staged only intended files first; never `add -A`).

---

### Task 3: Typed finding-code registry with a repo-wide minting guard

`StatusFinding.Code` and `domain.Finding.Code` values are minted as inline string literals at ~40 production sites (`git grep -cE 'Code:\s*"|lifecycleFinding\("|refuseLifecycle\("' -- 'internal/**/*.go' ':!*_test.go'`). Declare the vocabulary once, mint from it everywhere, and guard by shape.

**Files:**
- Create: `internal/app/finding_codes.go` (the registry)
- Modify: every production minting site in `internal/app` (derived by grep, Step 3)
- Test: `internal/app/finding_codes_test.go` (the shape guard + registry integrity)

**Interfaces:**
- Produces: `type FindingCode string`; one exported constant per distinct code (naming: `FC` prefix + PascalCase of the code, e.g. `FCInvalidID FindingCode = "invalid-id"`, `FCEmptyPath FindingCode = "empty-path"`); `var AllFindingCodes []FindingCode` (sorted, deduplicated, includes the existing `ReasonStatus*` reason tokens and the `domain` policy-reason tokens that surface as finding codes — referenced via their existing constants where those exist, e.g. `FindingCode(ReasonStatusInternalError)`).
- Produces: `lifecycleFinding(code FindingCode, msg string) StatusFinding` (signature change; Task 2's `"invalid-"+idKey` composition becomes a registry lookup: add `FCInvalidID` (`invalid-id`) and `FCInvalidChangeID` (`invalid-change_id`) and select between them by `idKey`).
- Task 6 consumes `AllFindingCodes` as the finding-code vocabulary.

- [ ] **Step 1: Derive the true code set.** Whole-repo grep, sorted into executable vs prose per AGENTS.md:

```bash
git grep -nE 'Code:\s*"[^"]+"' -- 'internal/**/*.go' ':!*_test.go'
git grep -nE '(lifecycleFinding|refuseLifecycle|attachRefusal|haltRefusal|implementedRefusal|reclaimSkip|repairRefusal|closeoutRefusal|mergeRefusal|maintenanceRefusal|prRefusal|backlinkRefusal)\("' -- 'internal/app/*.go' ':!*_test.go'
git grep -nE '\bReason[A-Z][A-Za-z]*\s*=' -- 'internal/app/*.go' ':!*_test.go'
```

Also grep `internal/domain` for the policy-failure reason tokens (`fail.Reason` values flow into finding codes via `refuseLifecyclePolicy`): `git grep -nE '(Reason|reason).*=.*"' internal/domain/ | grep -v _test`. Record the complete deduplicated list in the test file's header comment as the plan-time census (the guard, not the comment, is the enforcement).

- [ ] **Step 2: Write the failing guard test.** In `internal/app/finding_codes_test.go`:

```go
// TestNoInlineFindingCodeLiterals is the repo-wide minting guard: every
// production finding code is minted from the FindingCode registry in
// finding_codes.go. The scan keys on syntactic shape — a string literal in a
// Code: field position or as the first argument of a finding-constructor call —
// never on an enumerated spelling list. _test.go files are excluded (they
// assert codes; they do not mint them), and the exclusion is bounded to that
// suffix alone.
func TestNoInlineFindingCodeLiterals(t *testing.T) {
	codeLit := regexp.MustCompile(`Code:\s*"[^"]*"`)
	ctorLit := regexp.MustCompile(`(?:lifecycleFinding|refuseLifecycle|attachRefusal|haltRefusal|implementedRefusal|reclaimSkip|repairRefusal|closeoutRefusal|mergeRefusal|maintenanceRefusal|prRefusal|backlinkRefusal)\(\s*"`)
	root := repoRootForTest(t) // reuse or add the helper the package's other tree-walking tests use
	var violations []string
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		if filepath.Base(path) == "finding_codes.go" {
			return nil // the registry is the one sanctioned literal site
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range append(codeLit.FindAllString(string(b), -1), ctorLit.FindAllString(string(b), -1)...) {
			violations = append(violations, path+": "+m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Errorf("finding codes minted outside the registry — mint from a FindingCode constant in finding_codes.go:\n%s", strings.Join(violations, "\n"))
	}
}
```

Note on the ctor pattern: it enumerates the constructor NAMES (a closed set this package owns — verify it is complete against the Step-1 grep and note in the test comment that a NEW constructor must be added here; the `Code:` shape pattern is the backstop that catches a constructor this list misses, because a new constructor's body must still build a `Finding`/`StatusFinding` with a `Code:` field somewhere). Also add `TestFindingCodeRegistryIntegrity`: `AllFindingCodes` is sorted, deduplicated, non-empty, every member matches `^[a-z][a-z0-9_-]*$` (the underscore admits `invalid-change_id`).

- [ ] **Step 3: Run to verify the guard fails loudly.** `go test -count=1 ./internal/app/ -run TestNoInlineFindingCodeLiterals` — FAIL listing ~40 sites. This red run IS the census check: its site list must match Step 1's grep.

- [ ] **Step 4: Create the registry and migrate every site.** Write `finding_codes.go` with one constant per distinct code from the census, `AllFindingCodes`, and change `lifecycleFinding`'s signature to `(code FindingCode, msg string)`. Migrate each production site to reference its constant (`Code: string(FCParseFailed)` where the field is a plain string; pass the typed constant where the helper signature now takes `FindingCode`). Where a site passes a variable (e.g. `refuseLifecyclePolicy` forwarding `fail.Reason`) leave it — the guard's shape patterns match literals only, and the domain reason tokens are registered as constants referencing domain's spellings. `dropFindingCode(..., "empty-path")` callers (grep them) switch to `dropFindingCode(..., FCEmptyPath)` — adjust that helper's signature too.

- [ ] **Step 5: Verify green, then mutation-test.** `go test -count=1 ./internal/app/ ./internal/cli/` green. Mutations (each: edit, run `-count=1`, confirm red, revert): (a) change one migrated site back to an inline literal `Code: "parse-failed"` → `TestNoInlineFindingCodeLiterals` reds; (b) mint a literal through a constructor: `lifecycleFinding("rogue-code", …)` won't compile (typed param) — instead plant `haltRefusal(op, r, "rogue", …)`-shaped literal text in a non-registry file if any constructor still takes a string, else plant a `Code: "x"` composite in an arbitrary app file → reds; (c) delete `FCEmptyPath` from `AllFindingCodes` but keep the constant → `TestFindingCodeRegistryIntegrity` alone does NOT catch it — this is expected; Task 6's AST completeness guard covers the constant↔All-list correspondence. Note this residual explicitly in the commit body so Task 6 must close it.

- [ ] **Step 6: Commit.** `git add internal/app/finding_codes.go internal/app/finding_codes_test.go && git add -u internal/app internal/cli && git commit -m "refactor(app): typed finding-code registry with repo-wide minting guard"`

---

### Task 4: Co-located required-ness tag + tag/validator agreement test

Required-ness lives only in hand-written validators today. Add a `docket:"required"` struct tag on every request field a validator requires, and prove tag and validator cannot silently disagree.

**Files:**
- Modify: request struct declarations in `internal/app` (`change_lifecycle.go`, `change_create.go`, `change_groom.go`, `change_reconcile.go`, `change_kill.go`, and every other `*Request` a shape validator guards — derive the set from Task 2's caller list plus `git grep -ln 'func validate.*Shape' internal/app/`)
- Create: `internal/app/schema_tags.go` (tag reader), `internal/app/schema_tags_test.go` (agreement test)

**Interfaces:**
- Produces: the `docket:` struct-tag vocabulary, comma-separated options: `required`, `success-only`, `refusal-only`, `enum=<vocabulary-name>` (this task ships `required`; Task 6 ships the rest — declare the full vocabulary in schema_tags.go's doc comment now so the spellings are settled once).
- Produces: `requiredJSONKeys(prototype any) []string` — sorted top-level JSON keys whose field carries `docket:"required"` (same walk shape as Task 1's `requestJSONKeys`, filtered to tagged fields; lives app-side because Task 5's generator consumes it).
- Produces: `hasDocketOption(tag reflect.StructTag, opt string) bool` and `docketEnumRef(tag reflect.StructTag) string` (parse helpers Task 5/6 reuse).

- [ ] **Step 1: Derive each op's validator-required set.** For each request struct, read its shape validator and list the fields whose absence/zero mints an error finding. Plan-time survey to verify, not to copy: `ChangeBlockRequest` → change_id, path, version, reason; `ChangeDeferRequest` → change_id, path, version, why_deferred; `ChangeReconcileRequest` → id, version (reconcile drops empty-path); read `validateChangeCreateShape` and `ChangeGroom`'s validator firsthand for their sets.

- [ ] **Step 2: Write the failing agreement test.**

```go
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
		{"change.groom", ChangeGroomRequest{}, nil /* wire to groom's shape validator, same pattern */},
		{"change.reconcile", ChangeReconcileRequest{}, nil /* reconcile's pre-transaction validator */},
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
```

Fill the two `nil` closures with the real validator calls after reading those files (groom and reconcile validate pre-transaction like block/defer; if a validator is unexported and entangled with deps, call the op function with zero-value `PlanningDeps{}` exactly as block/defer — shape refusal returns first). If create's validator requires a field via a code that does not follow the `invalid-`/`empty-` prefix convention, rename that code to the convention in this task (it is a message-fix-family change, in scope) rather than special-casing the test.

- [ ] **Step 3: Run to verify failure** (`requiredJSONKeys` undefined). `go test -count=1 ./internal/app/ -run TestRequiredTagMatchesValidator`.

- [ ] **Step 4: Implement.** Write `schema_tags.go` (tag parsing + `requiredJSONKeys` via the reflection walk), then add `docket:"required"` to the surveyed fields, e.g.:

```go
type ChangeBlockRequest struct {
	ChangeID int    `json:"change_id" docket:"required"`
	Path     string `json:"path" docket:"required"`
	Version  string `json:"version" docket:"required"`
	Reason   string `json:"reason" docket:"required"`
}
```

- [ ] **Step 5: Verify green + mutation-test both directions.** (a) Remove `docket:"required"` from `ChangeBlockRequest.Reason` → test reds (validator names it, tag doesn't); (b) add `docket:"required"` to `ChangeReconcileRequest.Sections` (not validator-required) → test reds. Revert both; `-count=1` throughout.

- [ ] **Step 6: Commit.** `git add -u internal/app && git add internal/app/schema_tags.go internal/app/schema_tags_test.go && git commit -m "feat(app): co-located docket:required tags with tag/validator agreement guard"`

---

### Task 5: Schema descriptor types + reflection generator (request side)

The core generator: reflect a request struct into a typed field descriptor tree.

**Files:**
- Create: `internal/app/schema.go` (descriptor types, `SchemaVersion`, the reflector)
- Test: `internal/app/schema_test.go`

**Interfaces:**
- Produces:

```go
// SchemaVersion identifies the schema-surface contract, versioned separately
// from protocol_version and capability_version; consumers refuse an
// unsupported value fail-closed, exactly as capability_version works.
const SchemaVersion = 1

// FieldDescriptor is one field of a request or result document. Key is the
// REAL JSON key; Type is the docket-native type word: "int", "string", "bool",
// "object" (Fields nested), "map[string]string". Repeated marks arrays (Type
// then describes the element). Enum names a document-level vocabulary. Presence
// is "" (always may appear), "success-only", or "refusal-only" (result fields
// only; from the docket tag). Optional mirrors NOT-required for requests and
// omitempty for results.
type FieldDescriptor struct {
	Key      string            `json:"key"`
	Type     string            `json:"type"`
	Required bool              `json:"required,omitempty"`
	Repeated bool              `json:"repeated,omitempty"`
	Enum     string            `json:"enum,omitempty"`
	Presence string            `json:"presence,omitempty"`
	Fields   []FieldDescriptor `json:"fields,omitempty"`
}

// TypeDescriptor is one document side (request or result body).
type TypeDescriptor struct {
	Fields []FieldDescriptor `json:"fields"`
}

// reflectDescriptor walks a prototype struct into its descriptor. Embedded
// structs promote (the Envelope embed is handled by the registry, Task 7, which
// strips envelope keys from per-op results); pointer fields describe the
// element and are never required-by-shape; a nested struct (e.g.
// DesiredRelations under relations) recurses into Fields.
func reflectDescriptor(prototype any) TypeDescriptor
```

- [ ] **Step 1: Write the failing field-fidelity tests** — the spec's representative ops, asserting real key spellings, nesting, repeatability, required-ness against the live structs:

```go
func TestReflectDescriptorChangeReconcileRequest(t *testing.T) {
	d := reflectDescriptor(ChangeReconcileRequest{})
	keys := descriptorKeys(d) // test helper: top-level keys in order
	want := []string{"id", "version", "sections", "spec_sections", "relations", "reconcile_log_entry"}
	if !reflect.DeepEqual(keys, want) { t.Fatalf("keys = %v, want %v", keys, want) }
	rel := fieldByKey(t, d, "relations")
	if rel.Type != "object" { t.Errorf("relations type = %q, want object", rel.Type) }
	dep := fieldByKey(t, TypeDescriptor{Fields: rel.Fields}, "depends_on")
	if !dep.Repeated || dep.Type != "int" { t.Errorf("relations.depends_on = %+v, want repeated int", dep) }
	id := fieldByKey(t, d, "id")
	if !id.Required { t.Errorf("id must be required (docket tag)") }
}

func TestReflectDescriptorChangeGroomRequest(t *testing.T) {
	d := reflectDescriptor(ChangeGroomRequest{})
	if fieldByKey(t, d, "change_id").Key != "change_id" { t.Fatal("groom's id key is change_id, not id") }
	// spec_markdown optional (omitempty + not required); sections repeated object
	// with heading/intent/markdown; stacked_on a non-repeated optional int (*int).
	so := fieldByKey(t, d, "stacked_on")
	if so.Repeated || so.Type != "int" || so.Required { t.Errorf("stacked_on = %+v, want optional scalar int", so) }
}

func TestReflectDescriptorChangeCreateRequest(t *testing.T) { /* same pattern:
	request_id/title/type/priority/why/what_changes/out_of_scope + the five
	relation collections as repeated int; required set per Task 4's tags */ }
```

- [ ] **Step 2: Run to verify failure** (types undefined). `go test -count=1 ./internal/app/ -run TestReflectDescriptor`.

- [ ] **Step 3: Implement `reflectDescriptor`.** Kind switch: Int→"int", String→"string", Bool→"bool", Slice→Repeated=true + recurse on element, Pointer→recurse on element (never required-by-shape alone — required still comes only from the tag), Struct→"object"+nested Fields (except a struct kind with a registered scalar rendering — none exist today; fail loudly `panic`-free by returning Type "object"), Map with string key and string value→"map[string]string" (any other map shape: return an error — the generator must fail closed on a shape it cannot describe, and the registry test in Task 7 surfaces it). Read `json` tag for Key (skip `-` and untagged unexported), `docket` tag via Task 4's helpers for Required/Presence/Enum. Embedded structs promote inline.

- [ ] **Step 4: Verify green.** `go test -count=1 ./internal/app/`.

- [ ] **Step 5: Commit.** `git add internal/app/schema.go internal/app/schema_test.go && git commit -m "feat(app): schema descriptor types and request reflection generator"`

---

### Task 6: Result presence tags + vocabularies with completeness guards

**Files:**
- Modify: result struct declarations for presence/enum tags (`change_lifecycle.go`, `change_create.go`, `change_groom.go`, `change_reconcile.go`, `status_result.go`, and every result struct the Task 7 registry binds — tag as you register)
- Create: `internal/app/schema_vocab.go`, `internal/app/schema_vocab_test.go`
- Modify: `internal/app/finding_codes.go` (if Step 3's guard finds an All-list gap)

**Interfaces:**
- Produces:

```go
// Vocabulary is one closed set, emitted once and referenced by name from
// FieldDescriptor.Enum. Exactly one of Members or Pattern is set: Members for
// a closed enumeration, Pattern for a shape-closed token set (change types,
// which domain deliberately validates by ValidTypeToken's shape, not a list —
// closing them here would reject stored corpora domain accepts, so the
// vocabulary reports the truth: the pattern).
type Vocabulary struct {
	Members []string `json:"members,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
}

// Vocabularies returns the complete named-vocabulary map: finding_codes,
// results, effects (passed in by the cli caller — app cannot import cli),
// priorities, statuses, change_types, groom_outcomes, section_intents, and one
// per disposition family a bound result field references (claim_dispositions,
// halt_dispositions, reclaim_dispositions, reconcile_dispositions,
// block_dispositions, cleanup_dispositions, … — derive the family list from
// the registry's enum references, Task 7, never free-hand).
func SchemaVocabularies(effects []string) map[string]Vocabulary
```

- [ ] **Step 1: Write the failing vocabulary tests.** (a) `TestSchemaVocabulariesCore`: the map contains `finding_codes` (== AllFindingCodes stringified), `results` (== AllResults), `priorities` (critical/high/medium/low via domain constants), `section_intents` (preserve/replace/remove via render constants), `groom_outcomes` (spec/trivial), `statuses` (the eight domain statuses), `change_types` with Pattern non-empty and Members empty, and `effects` echoing its argument. (b) `TestVocabularyConstCompleteness` — the AST guard closing Task 3 Step 5's residual:

```go
// TestVocabularyConstCompleteness parses internal/app and internal/domain and,
// for each emitted Members vocabulary, asserts set-equality between the
// emitted members and the string values of the const group it is derived from
// (located by the declared type name — Priority, Status, GroomOutcome,
// SectionIntent, Result, FindingCode — or, for untyped disposition families,
// by const-name prefix within one const block, e.g. HaltDisp*). Adding a
// constant without it reaching the surface reddens here; so does a phantom
// member with no constant. Both directions, per the correspondence rule.
func TestVocabularyConstCompleteness(t *testing.T) { … }
```

Implement with `go/parser`+`go/ast` over the package dirs: collect `const` ValueSpecs whose declared type (or name prefix, for the disposition families) matches, take their string literal values, compare as sets AND lengths against the emitted vocabulary. For AllFindingCodes this is the guard that a constant declared in finding_codes.go is also listed.

- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/app/ -run 'TestSchemaVocab|TestVocabularyConst'`.

- [ ] **Step 3: Implement `SchemaVocabularies`** referencing existing constants/slices only (AllResults, AllFindingCodes, domain.ParsePriority's members via new exported `domain.AllPriorities`/`domain.AllStatuses` slices if none exist — add them beside the Parse functions with their own completeness coverage from Step 1's AST guard; render section intents likewise). `change_types` Pattern: the exact regexp `ValidTypeToken` enforces, quoted from that function — read it first, never guess.

- [ ] **Step 4: Add presence tags to result structs as needed for Task 7** — e.g. `ChangeLifecycleResult`: `ID`, `Status`, `Revision` get `docket:"success-only"`; `Findings` stays untagged (present on every path); `StatusResult.Reason`/`Message` get `docket:"refusal-only"` (its own comment says failure-results-only). Tag only what a struct's own doc comments/constructors establish; where genuinely always-possible, leave untagged. Fields whose type is a closed set get `docket:"enum=<name>"` (e.g. reconcile's `Disposition string` → `enum=reconcile_dispositions`, create's `Type` → `enum=change_types`, `Priority` → `enum=priorities`).

- [ ] **Step 5: Verify green + mutation-test.** Mutations: (a) append a phantom `"rogue"` to the emitted priorities vocabulary → completeness guard reds; (b) add a new `HaltDispX = "x"` constant to change_halt.go's const block → guard reds until surfaced; (c) delete `FCEmptyPath` from AllFindingCodes (Task 3's residual) → guard reds. Revert all; `-count=1`.

- [ ] **Step 6: Commit.** `git add internal/app/schema_vocab.go internal/app/schema_vocab_test.go && git add -u internal/app internal/domain internal/render && git commit -m "feat(app): schema vocabularies with AST completeness guards and presence tags"`

---

### Task 7: Operation schema registry + correspondence guards

Bind every catalog operation id to its request (when it takes one) and result prototypes, and guard the binding in both directions.

**Files:**
- Create: `internal/app/schema_registry.go`, `internal/app/schema_registry_test.go`
- Test (cli side): `internal/cli/schema_production_test.go` (catalog↔schema id join, mirrors `capability_production_test.go`)

**Interfaces:**
- Produces:

```go
// OperationBinding joins one capabilities operation id to the live Go types
// its handler decodes and returns. Request is nil for leaves that take no
// JSON body. The id is the SAME stable id the capability catalog uses — the
// join key across the two surfaces.
type OperationBinding struct {
	ID      string
	Request any // prototype struct value, e.g. ChangeBlockRequest{}
	Result  any // prototype struct value embedding Envelope
}

// OperationBindings returns the complete registry sorted by id.
func OperationBindings() []OperationBinding

// OperationSchema / SchemaResult — the emitted document (consumed by Task 8):
type OperationSchema struct {
	ID      string          `json:"id"`
	Request *TypeDescriptor `json:"request,omitempty"`
	Result  TypeDescriptor  `json:"result"`
}
type SchemaResult struct {
	Envelope
	SchemaVersion int                   `json:"schema_version"`
	EnvelopeShape TypeDescriptor        `json:"envelope"` // emitted once; per-op results exclude envelope keys
	Operations    []OperationSchema     `json:"operations"`
	Vocabularies  map[string]Vocabulary `json:"vocabularies"`
}

// Schema assembles the full document; SchemaFor filters to one id (ok=false
// for an unknown id — the cli maps that to ResultInvalidInput with finding
// code FCUnknownOperation, a new registry constant "unknown-operation").
func Schema(effects []string) (SchemaResult, error)
func SchemaFor(id string, effects []string) (SchemaResult, bool, error)
```

- [ ] **Step 1: Derive the binding list.** For every catalog id (run `go run ./cmd/docket capabilities --json | jq -r '.commands[].id'` from the worktree), read the cli command's RunE to find the app function it calls and that function's request/result types. Ops with flag-only inputs (claim, halt via `changeIDVersionSubcommand`, gate.*, finalize.* scalar flags) still bind their app-side `*Request` struct when the cli assembles one from flags — bind what the handler DECODES OR ASSEMBLES; a leaf with neither (capabilities, schema itself, status) binds Request nil. Record per-binding derivation as a one-line comment naming the app function symbol (never a line number).

- [ ] **Step 2: Write the failing registry-accounting test** (app side):

```go
// TestEveryRequestAndResultStructIsBound parses internal/app's AST for every
// exported type named *Request or *Result and asserts each is reachable from
// OperationBindings() — either bound directly or nested inside a bound
// prototype (walked by reflection). A new operation's structs without a
// binding redden here; a phantom binding naming no real type cannot compile.
// Reverse direction: every binding's prototypes must BE app types named
// *Request/*Result (or Envelope-embedding documents like StatusResult) — a
// binding pointing at a helper struct reddens. Both directions, with counts
// logged so a gross population collapse is visible.
func TestEveryRequestAndResultStructIsBound(t *testing.T) { … }
```

Plus `TestOperationBindingsSortedUniqueAndDescribable`: ids sorted+unique; `reflectDescriptor` succeeds on every prototype (this is where an undescribable map shape from Task 5 surfaces); every result prototype embeds `Envelope`; every `docket:"enum=…"` reference across all bound prototypes names a key present in `SchemaVocabularies` (the pairing assert — and its converse: every Members vocabulary is referenced by at least one bound field OR is one of the always-emitted core sets, so an orphan family vocabulary reddens).

- [ ] **Step 3: cli-side production join test** in `internal/cli/schema_production_test.go`, using `productionRootForTest` exactly as `capability_production_test.go` does:

```go
// TestSchemaCatalogCorrespondence joins the two surfaces on id, both ways:
// every catalog entry has a schema binding, every binding resolves to a
// catalog entry, counts equal. Mirrors TestProductionCapabilityCorrespondence.
func TestSchemaCatalogCorrespondence(t *testing.T) {
	entries, err := collectCapabilities(productionRootForTest(t))
	if err != nil { t.Fatal(err) }
	catalog := map[string]bool{}
	for _, e := range entries { catalog[e.ID] = true }
	bindings := app.OperationBindings()
	for _, b := range bindings {
		if !catalog[b.ID] { t.Errorf("schema binding %q has no catalog entry", b.ID) }
	}
	if len(bindings) != len(entries) {
		t.Errorf("bindings %d != catalog entries %d — a new leaf without a schema binding, or a stale binding", len(bindings), len(entries))
	}
	// pairing is the id itself; count+membership both ways closes the mirror
}
```

- [ ] **Step 4: Run to verify failure, then implement.** Write `schema_registry.go`: the bindings table, `Schema` (envelope shape reflected once from `Envelope{}`; per-op result descriptors reflected from the prototype then filtered to exclude the envelope's keys — compute the envelope key set from `reflectDescriptor(Envelope{})`, never a hand list), `SchemaFor`. Add `FCUnknownOperation FindingCode = "unknown-operation"` to the Task 3 registry.

- [ ] **Step 5: Verify green + mutation-test the correspondence in both directions.** (a) Comment out the `change.block` binding → app accounting test AND cli join test redden; (b) add a phantom binding `{ID: "change.phantom", …}` → cli join test reds; (c) with `-count=1`, confirm the cached-runner rule when probing from `internal/cli` after mutating `internal/app`.

- [ ] **Step 6: Commit.** `git add internal/app/schema_registry.go internal/app/schema_registry_test.go internal/cli/schema_production_test.go && git add -u internal/app && git commit -m "feat(app): operation schema registry with two-way catalog correspondence guards"`

---

### Task 8: `docket schema` CLI leaf — read-only, repo-independent

**Files:**
- Create: `internal/cli/schema.go`
- Modify: the root command registration site (find where `newCapabilitiesCommand` is added: `git grep -n newCapabilitiesCommand internal/cli/`) — register the schema command beside it
- Modify: `internal/app/schema.go` (add `HumanText()` on `SchemaResult`)
- Test: `internal/cli/schema_command_test.go`

**Interfaces:**
- Consumes: `app.Schema`, `app.SchemaFor`, `allEffects` (project to sorted strings for the effects vocabulary), the `capability(id, effects…)` annotation helper.
- Produces: capability id `"schema"`, effect `read`, argv `docket schema [--operation <id>]` (+ global `--json`).

- [ ] **Step 1: Write the failing tests.**

```go
// TestSchemaCommandRepositoryIndependent proves the same posture capabilities
// holds: callable in a bare non-git temp dir, applied result, no file created.
func TestSchemaCommandRepositoryIndependent(t *testing.T) {
	dir := testsupport.TempDir(t) // and chdir there per the harness's pattern
	out, _, code := runCLIInDir(t, dir, "schema", "--json")
	if code != 0 { t.Fatalf("exit %d", code) }
	var doc struct {
		ProtocolVersion int `json:"protocol_version"`
		SchemaVersion   int `json:"schema_version"`
		Operations      []struct{ ID string } `json:"operations"`
		Vocabularies    map[string]json.RawMessage `json:"vocabularies"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil { t.Fatal(err) }
	if doc.SchemaVersion != 1 || doc.ProtocolVersion != 1 { t.Errorf("versions = %+v", doc) }
	if len(doc.Operations) == 0 || len(doc.Vocabularies) == 0 { t.Error("empty surface") }
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 { t.Errorf("schema wrote files: %v", entries) }
}

// TestSchemaCommandSingleOperation: --operation change.create returns exactly
// one operation whose request keys include title/type/priority/relations set;
// --operation nonsense returns result invalid-input, exit 2, finding code
// unknown-operation.
// TestSchemaCapabilityAnnotation: walk the production tree (reuse
// productionRootForTest) and assert the schema entry's effects == ["read"].
```

Adapt `runCLIInDir`/`runCLIStdin` to whatever this package's existing harness provides (read `capabilities_command_test.go` first and copy its invocation pattern — it already proves repo-independence for capabilities; mirror it, don't invent a new harness).

- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/cli/ -run TestSchemaCommand`.

- [ ] **Step 3: Implement `newSchemaCommand`** mirroring `newCapabilitiesCommand` (same file-level rules: RunE touches no filesystem, config, git, or network):

```go
func newSchemaCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "schema",
		Short:       "Emit request/result payload schemas and closed vocabularies (read-only, repository-independent)",
		Args:        cobra.NoArgs,
		Annotations: capability("schema", EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			op, _ := c.Flags().GetString("operation")
			effects := sortedEffectStrings() // project allEffects
			if op == "" {
				doc, err := app.Schema(effects)
				if err != nil { return fmt.Errorf("schema construction failed: %w", err) }
				setResult(doc)
				return nil
			}
			doc, ok, err := app.SchemaFor(op, effects)
			if err != nil { return fmt.Errorf("schema construction failed: %w", err) }
			if !ok { setResult(app.SchemaUnknownOperation(op)); return nil }
			setResult(doc)
			return nil
		},
	}
	cmd.Flags().String("operation", "", "emit only this operation `id` (e.g. change.create)")
	return cmd
}
```

Add `app.SchemaUnknownOperation(id string) SchemaResult` (envelope operation `"schema"`, `ResultInvalidInput`, a findings field carrying `FCUnknownOperation` — give `SchemaResult` a `Findings []StatusFinding \`json:"findings"\`` normalized to `[]`). `HumanText()`: header `schema v%d — %d operations` plus one line per op (`id  request:n-fields  result:n-fields`), mirroring `CapabilitiesResult.HumanText`'s compact style. Register the command wherever the root assembles its children, passing `setResult` exactly as capabilities does.

- [ ] **Step 4: Verify green, including the untouched budget.** `go test -count=1 ./internal/cli/ ./internal/app/` — the existing capabilities 12 KB budget guard must still pass untouched (this change adds a catalog ENTRY for `schema`, which grows the capabilities document slightly; if the budget guard reds, the answer is NEVER raising the budget (out of scope) — the entry is one compact line and should fit; if it genuinely does not, halt and surface).

- [ ] **Step 5: Commit.** `git add internal/cli/schema.go internal/cli/schema_command_test.go internal/app/schema.go && git add -u internal/cli internal/app && git commit -m "feat(cli): docket schema — read-only payload-schema surface"`

---

### Task 9: Result + vocabulary fidelity for the representative ops

Spec verification bullets: emitted result descriptors match the `*Result` structs and envelope; vocabulary references resolve.

**Files:**
- Test: `internal/app/schema_fidelity_test.go`

- [ ] **Step 1: Write the tests (they should pass against Tasks 5–8; any red is a generator bug to fix, not a test to bend).**

```go
// TestSchemaResultFidelityRepresentativeOps: for change.create, change.groom,
// change.reconcile — fetch SchemaFor(id) and assert: (a) the envelope shape
// lists protocol_version/operation/result/failure exactly (derived once,
// asserted literally here as the frozen v1 contract); (b) the op's result
// fields match the struct: reconcile carries disposition with
// enum=reconcile_dispositions, create carries id/slug/path/committed_revision
// with presence success-only and findings untagged; (c) every Enum reference
// across the full Schema() document resolves to a vocabulary key (walk all
// descriptors recursively — the whole-surface pairing sweep, stronger than
// Task 7's per-binding check because it runs on the EMITTED document).
// TestSchemaRequestAbsentForReadOnlyLeaves: status/capabilities/schema
// entries carry no request block.
```

- [ ] **Step 2: Run, fix any generator defects, verify green.** `go test -count=1 ./internal/app/`.

- [ ] **Step 3: Mutation-test the whole-surface sweep.** Temporarily tag a bound field `docket:"enum=nonexistent"` → the (c) sweep reds. Revert.

- [ ] **Step 4: Commit.** `git add internal/app/schema_fidelity_test.go && git commit -m "test(app): result, envelope, and vocabulary fidelity for representative ops"`

---

### Task 10: Skill contract — resolve request bodies from `docket schema`

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — two touch points: (a) the "Reaching docket's operations." paragraph (quote-anchor: "resolves the executable spelling from the capability catalog"); (b) the "**Mid-run posture.**" paragraph in the Step-0 section.

- [ ] **Step 1: Author the additions.** In (a), after the sentence ending "never a `DOCKET_*` transport variable resolved into a command.", insert:

> An operation's **request or result payload shape** is resolved the same way: run the `schema` operation from the catalog (`docket schema --operation <id> --json`), validate fail-closed exactly as the capability bootstrap does (refuse `protocol_version` ≠ 1 or an unsupported `schema_version`, a malformed envelope, or an id the surface does not carry), and construct the body from the descriptor's real JSON keys, required markers, and named vocabularies — **never** from `--help` text, `strings` on the binary, the docket source tree, or a probe invocation. Historical records keep the spellings that were true when written.

In (b), append to the Mid-run posture paragraph: "A `--request`/`--input` body an operation needs is constructed from the `schema` operation's descriptor for that id, per *Reaching docket's operations* — never guessed, probed, or read from source."

- [ ] **Step 2: Check for prose guards.** `git grep -rln 'docket-convention' tests/ internal/` and run any matching sentinel tests; a guard over the edited paragraphs that reds means the guard must be extended for the new sentence (collapse-whitespace matching per learnings: phrase-grep-over-wrapped-prose), not the sentence bent to the guard.

- [ ] **Step 3: Run the full suite.** `go run ./cmd/docket development test` — skill-prose sentinels live in the shell-test corpus the Go runner drives.

- [ ] **Step 4: Commit.** `git add skills/docket-convention/SKILL.md && git commit -m "docs(skills): resolve request bodies from docket schema, never help/strings/source"`

---

### Task 11: Full-suite gate + human-verification residuals

- [ ] **Step 1: Run the whole suite from source:** `go run ./cmd/docket development test`. Fix anything red (budget clause lines: `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` are screening findings; `SERIAL CONFIRMED OVER BUDGET:` is authoritative — act per tests/README.md).
- [ ] **Step 2: Race + cache-defeating spot check:** `go test -race -count=1 ./internal/app/ ./internal/cli/` (the suite's race gate covers this; the explicit run is the task-level receipt).
- [ ] **Step 3: Record the two residuals for the results file / human:** (a) the spec's **cross-harness check** (in Cursor, build a `--request` body for a mutating op using only `docket schema` — no `--help`, no `strings`, no source read) is external to this repo's suite: name it as a human verification item, per learnings: external-truth-needs-a-human-checkpoint; (b) change-types ship as a **pattern vocabulary** (domain's `ValidTypeToken` shape), not a closed member list — deliberate: closing the set would reject stored corpora domain accepts; the spec's "brought under the same typed pattern" is satisfied by the existing typed shape validator being the emitted truth.
- [ ] **Step 4: Commit any gate fixes** (each with its own focused message), leaving the tree clean.

---

## Self-Review (performed at plan time)

- **Spec coverage:** dedicated op with own capability id + read effect → Task 8; `--json` all-ops and `--operation` single-op → Task 8; live-type derivation with real keys/types/required/nesting/repeatability/enums → Tasks 4–7; required-ness tag + agreement test → Task 4; typed finding-code registry + grep guard → Task 3; already-typed sets reflected, change types handled (pattern, with recorded rationale) → Task 6 / Task 11; three required sections (Request/Result/Vocabularies) → Tasks 5–7; `schema_version` fail-closed like `capability_version` → Tasks 5, 8, 10; no git/config/network/write + callable when repo-aware ops refuse → Task 8; message fixes first → Tasks 1–2; Step-0/skill contract → Task 10; round-trip correspondence + mutation guards mirroring 0394 → Task 7; field fidelity for create/groom/reconcile → Task 5; result+vocabulary fidelity → Task 9; read-only assertions → Task 8; message-fix tests → Tasks 1–2; mutation-tested tag/registry guards → Tasks 3–4, 6–7; no-probe/cross-harness → Task 11 residual. Out-of-scope items are excluded everywhere (no budget raise — Task 8 Step 4 says halt, never raise).
- **Type consistency:** `FindingCode`/`AllFindingCodes` (T3) consumed by T6; `requiredJSONKeys`/`hasDocketOption`/`docketEnumRef` (T4) consumed by T5; `TypeDescriptor`/`FieldDescriptor`/`reflectDescriptor` (T5) consumed by T6–T9; `OperationBinding`/`OperationBindings`/`Schema`/`SchemaFor`/`SchemaResult` (T7) consumed by T8–T9; `decodeRequest(flagName…)` stays cli-internal.
- **Known judgment calls the builder must not silently reverse:** (1) the `id`-keyed finding code changes from `invalid-change_id` to `invalid-id` (Task 2) — a deliberate protocol-message fix, recorded in the commit; (2) change types stay pattern-closed (Task 6/11); (3) the constructor-name list in Task 3's guard is backed by the `Code:` shape backstop, per learnings: backstop-must-compute-not-reenumerate.
