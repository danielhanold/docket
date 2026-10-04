<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0520 — Make every published request schema match the JSON file the operation reads](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0520-make-the-published-finalize-request-schemas-match-what-input.md)**
<!-- docket:backlink:end -->
# Published request schemas match the JSON file each operation reads: Implementation Plan

> **For agentic workers:** `docket-build` executes this plan. It sends each `### Task N` to one
> tier worker that follows the `docket-build-task` contract. The tasks run in order, and each one
> builds on the commit before it. Steps use checkbox (`- [ ]`) syntax for tracking. Do not tick
> them, because your commit is the progress record.

**Goal:** `docket schema` publishes, as an operation's request, exactly the JSON document that the
operation strictly decodes from a file (`--request`, `--input` or `--body`). An operation that
decodes no such file publishes no request. A mutation-tested guard keeps the published request and
the decoder from drifting apart.

**Architecture:** Each JSON file gets exactly one `internal/app` type, and both the CLI decoder and
the schema registry use that type. In `internal/cli`, every strict JSON decode goes through a
single generic helper, `declareJSONFile[T]`. When a command is built, the helper records `T` (and
its flag) as cobra annotations and returns the only decoder that command's `RunE` may call. The
declaration and the decode share the type parameter, so they cannot name different types. There
are two guards in `internal/cli`. The first walks the production command tree and compares each
command's recorded type with its registry binding and with the keys that `docket schema` publishes.
The second is an AST scan that fails if anything references the decode primitives outside the
helper.

**Tech Stack:** Go 1.26 (`go.mod`), cobra, `go/ast` + `go/parser` for the source scan, and
reflection for type identity.

**Spec:** `docs/superpowers/specs/2026-10-04-make-the-published-finalize-request-schemas-match-what-input-design.md`
on the `docket` branch. Read it through the synchronized metadata worktree:
`/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-make-the-published-finalize-request-schemas-match-what-input-design.md`.
The **Rule** and **Guard** sections are binding.

## Global Constraints

Every task's requirements implicitly include all of these.

- **Repo root.** Run every command from the feature worktree root:
  `/Users/homer/dev/docket/.worktrees/make-the-published-finalize-request-schemas-match-what-input`.
- **The rule (spec, verbatim):** "An operation's published request is exactly the authored JSON
  document it strictly decodes (through `decodeRequest`, whatever the flag is called: `--request`,
  `--input`, `--body`), or absent when it decodes none." Only the capability catalog's `signature`
  describes scalar flags. Non-JSON file inputs are not requests and stay unpublished: the build
  evidence record (`--evidence`/`--record`, read by `readRecordSource`) and `agent.enter`'s
  plain-text `--request`.
- **Do not change:** the schema document format or `SchemaVersion`. Which values travel as flags
  and which travel in the JSON file. Decode refusal behaviour (an unknown key is still refused, and
  the accepted keys are still listed). The `docket-rebase-resolver` agent text
  (`agents/docket-rebase-resolver.md`). `finalize.resolver_max_attempts` and rebase replay
  mechanics. The names of the flag-assembled `*Request` structs (`BlockRequest`, `HaltRequest`,
  `RetargetChildrenRequest`, `PRPublishRequest`, and so on). They stay as internal app inputs and
  are just no longer bound.
- **No ADR file.** The coordinator records the new ADR separately. Do not create anything under
  `docs/adrs/`. Code comments may cite ADR-0109.
- **Comments anchor on symbol names** (for example `TestPublishedRequestIsTheDecodedJSONFile`),
  never on line numbers (AGENTS.md, ADR-0054).
- **Docs describe only current behaviour.** Do not cite changes or PRs, and do not write "this used
  to…" text.
- **Guards key on shape and never hand-list sites.** The set of JSON-reading operations is derived
  from the command tree and the helper's annotations, never from an enumerated list.
- **Every mutation probe and manual re-verification defeats the Go test cache** with `-count=1`.
  To restore a mutated file, copy back a backup you made first
  (`cp -f <file> "${TMPDIR:-/tmp}/<name>.bak"` … `cp -f "${TMPDIR:-/tmp}/<name>.bak" <file>`).
  Never use `git checkout -- <file>`, because that restores HEAD and throws away uncommitted work.
- **Tagged tests must keep compiling.** After any edit to `internal/app` or `internal/cli`, run
  `go vet -tags integration ./internal/app/ ./internal/cli/` and `go vet -tags e2e ./internal/app/`
  in addition to the untagged focused tests.
- **gofmt** every edited Go file (`gofmt -l internal/` must print nothing).
- **Stage explicit paths only**, never `git add -A` or `git commit -a`.

## Review Focus

1. **An agent pastes an envelope key (`schema_version`) into a request file.** It must still be
   refused, and the refusal must name every accepted key for that operation. This is pinned for
   every JSON-reading operation by `TestPublishedRequestKeysAreAccepted` (Task 4).
2. **Nested published keys** (`finalize.retarget-children` → `children[].id`, `pr_number`,
   `pr_revision`) must be accepted when an agent builds the file from the published descriptor.
   `TestPublishedRequestKeysAreAccepted` (Task 4) pins this by filling nested objects from their
   published sub-fields, and `TestSchemaPublishesTheJSONFileOfEachFixedOperation` (Task 4) asserts
   the nested keys.
3. **`finalize closeout` with no `--input`** must stay the unchanged no-notes path. The helper's
   decoder refuses an empty source, so the closeout `RunE` must keep its `src != ""` check before
   calling the decoder. `TestFinalizeCloseoutInputFlag` and `TestFinalizeCloseoutInputDecode`
   (Task 3) pin this, along with the existing closeout tests.
4. **`pr publish` reads two files, and only `--body` is JSON.** The evidence file (`readRecordSource`)
   must never be declared or published. `TestSchemaPublishesTheJSONFileOfEachFixedOperation`
   (Task 4) asserts that `pr.publish` publishes exactly `body`, `title`. The guard asserts that the
   declared flag exists, and `declareJSONFile` refuses a second declaration on the same command
   (Task 2).
5. **A flag-only operation id passed to `docket schema --operation`** must still resolve, with no
   request block, and must not be an unknown-operation refusal (exit 0, `request` absent).
   `TestSchemaPublishesTheJSONFileOfEachFixedOperation` (Task 4) pins this for `finalize.rebase`,
   `change.claim` and `finalize.merge`.

Note: the spec's **Design** section gives "report and remedy" as the example of required keys for
`finalize.block`. Its binding constraint is that `docket:"required"` tags must **agree with the
validators**, and `validateBlockShape` requires only `report` (`remedy` is optional and only
size-bounded). So this plan tags `report` alone, and `TestRequiredTagMatchesValidator` enforces
that. Do not change the validator. Say this in the Task 1 commit body so the coordinator can
record it.

---

### Task 1: One `internal/app` type per JSON file, with required tags that agree with the validators

**Build tier:** standard

**Files:**
- Modify: `internal/app/finalize_block.go` (add `FinalizeBlockInput` beside `BlockRequest`)
- Modify: `internal/app/finalize_retarget.go` (add `RetargetChildrenInput` beside `RetargetChildrenRequest`)
- Modify: `internal/app/change_halt.go` (add `ChangeHaltInput` beside `HaltRequest`)
- Modify: `internal/app/pr_publish.go` (add `PRPublishInput` beside `PRPublishRequest`)
- Modify: `internal/app/finalize_closeout_notes.go` (json tags on `CloseoutNotes`)
- Test: `internal/app/schema_tags_test.go` (`TestRequiredTagMatchesValidator`)

**Interfaces:**
- Produces (exact names later tasks use):
  - `app.FinalizeBlockInput{Report string \`json:"report" docket:"required"\`; Remedy string \`json:"remedy"\`}`
  - `app.RetargetChildrenInput{Children []AuthorizedChild \`json:"children"\`}`
  - `app.ChangeHaltInput{Report string \`json:"report" docket:"required"\`}`
  - `app.PRPublishInput{Title string \`json:"title"\`; Body string \`json:"body"\`}`
  - `app.CloseoutNotes{VerificationOutcomes []string \`json:"verification_outcomes"\`; LateFindings []string \`json:"late_findings"\`}`
    (this is the existing type with json tags added. `ResolverReport` is unchanged.)

`CloseoutNotes` is never JSON-marshaled today. The closeout receipt stores `closeoutNotesDigest`,
a string, so adding json tags changes no persisted bytes. Confirm that with
`grep -n "json.Marshal" internal/app/finalize_closeout.go` before you edit, and check that no hit
marshals a `CloseoutNotes` value.

- [ ] **Step 1: Write the failing test.** In `internal/app/schema_tags_test.go`, add four cases to
  the `cases` slice of `TestRequiredTagMatchesValidator`. Each case fills every flag-assembled field
  with a valid value and leaves the file's fields at zero, so it exercises an empty file:

```go
		{"finalize.block --input", FinalizeBlockInput{}, func() []StatusFinding {
			return validateBlockShape(BlockRequest{ID: 1, Revision: "r", PRNumber: 1, Attempt: "a", Reason: "x", Head: "h"})
		}},
		{"change.halt --input", ChangeHaltInput{}, func() []StatusFinding {
			return validateHaltShape(HaltRequest{ID: 1, Revision: "r"})
		}},
		{"finalize.retarget-children --input", RetargetChildrenInput{}, func() []StatusFinding {
			return validateRetargetShape(RetargetChildrenRequest{ID: 1, Revision: "r"})
		}},
		{"finalize.closeout --input", CloseoutNotes{}, func() []StatusFinding {
			_, findings := normalizeCloseoutNotes(CloseoutNotes{})
			return findings
		}},
```

  `requiredJSONKeys` returns a non-nil empty slice, while `got` stays nil when there are no
  findings. In the loop body, just before the `reflect.DeepEqual`, add:

```go
			if got == nil {
				got = []string{}
			}
```

  `PRPublishInput` and `ResolverReport` get no case. Neither has a shape validator that emits
  `empty-<key>`/`invalid-<key>` codes (`PRPublish` refuses on the head first, and the resolver
  report is checked against live rebase state). So neither type carries a `required` tag.

- [ ] **Step 2: Run it and confirm it fails.**
  Run: `go test -count=1 -timeout 300s -run TestRequiredTagMatchesValidator ./internal/app/`
  Expected: a build failure (`undefined: FinalizeBlockInput`, and the same for the other new
  types). That is the intended RED.

- [ ] **Step 3: Add the types.** Put each one directly after its assembled request struct:

```go
// FinalizeBlockInput is the JSON file `finalize block --input` reads: the
// authored report that crosses to the PR comment and the authored remedy
// recorded in the marker. The scalar identities (id, revision, pr number,
// attempt, reason, head) ride on flags; the CLI assembles both into
// BlockRequest. This type is the operation's published request.
type FinalizeBlockInput struct {
	Report string `json:"report" docket:"required"`
	Remedy string `json:"remedy"`
}
```

```go
// RetargetChildrenInput is the JSON file `finalize retarget-children --input`
// reads: the exact human-authorized child set from context finalize. The parent
// id and record revision ride on flags; the CLI assembles both into
// RetargetChildrenRequest. This type is the operation's published request.
type RetargetChildrenInput struct {
	Children []AuthorizedChild `json:"children"`
}
```

```go
// ChangeHaltInput is the JSON file `change halt --input` reads: the authored
// run-halted report, section body only (the operation owns the "## Run halted"
// heading and dated sub-heading). The id and revision ride on flags; the CLI
// assembles both into HaltRequest. This type is the operation's published request.
type ChangeHaltInput struct {
	Report string `json:"report" docket:"required"`
}
```

```go
// PRPublishInput is the JSON file `pr publish --body` reads: the authored PR
// title and body. The change id, head, and evidence record ride on flags (the
// evidence record is a canonical record file, not a JSON request); the CLI
// assembles all of them into PRPublishRequest. This type is the operation's
// published request.
type PRPublishInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}
```

  In `internal/app/finalize_closeout_notes.go`, tag the two fields and add one sentence to the
  doc comment: "It is also the JSON file `finalize closeout --input` reads, and that operation's
  published request."

```go
type CloseoutNotes struct {
	VerificationOutcomes []string `json:"verification_outcomes"`
	LateFindings         []string `json:"late_findings"`
}
```

- [ ] **Step 4: Run it and confirm it passes.**
  Run: `go test -count=1 -timeout 300s -run 'TestRequiredTagMatchesValidator|TestEveryRequestAndResultStructIsBound|TestOperationBindingsSortedUniqueAndDescribable' ./internal/app/`
  Expected: PASS. The new types end in `Input` or are already reachable, so the registry
  accounting does not see them yet.
  Then run: `go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/`
  Expected: no output.

- [ ] **Step 5: Mutation probe (not committed).** Back up `internal/app/finalize_block.go`, remove
  `docket:"required"` from `FinalizeBlockInput.Report`, and re-run the Step 4 `-run
  TestRequiredTagMatchesValidator` command with `-count=1`. Expected: FAIL naming
  `finalize.block --input`. Restore from the backup, re-run, and expect PASS.

- [ ] **Step 6: Commit.**

```bash
git add internal/app/finalize_block.go internal/app/finalize_retarget.go internal/app/change_halt.go internal/app/pr_publish.go internal/app/finalize_closeout_notes.go internal/app/schema_tags_test.go
git commit -m "feat(app): one app type per JSON request file" -m "FinalizeBlockInput tags only report as required: validateBlockShape requires report and only bounds remedy, so the tag follows the validator rather than the spec's example parenthetical."
```

---

### Task 2: The `declareJSONFile` helper, and the change/adr/learning builders routed through it

**Build tier:** standard

**Files:**
- Create: `internal/cli/jsonfile.go`
- Create: `internal/cli/jsonfile_test.go`
- Modify: `internal/cli/change.go` (`changeSubcommand`, `changeInputSubcommand`, every call site in
  `newChangeCommand`, delete `changeHaltInput` and `decodeRequestFlag`)
- Modify: `internal/cli/adr.go`, `internal/cli/learning.go` (call sites, and the plumbing comment
  that names `decodeRequestFlag`)

**Interfaces:**
- Consumes: `app.ChangeHaltInput` (Task 1). `decodeRequest(stdin io.Reader, flagName, source string, dst any) error` (unchanged, in `change.go`).
- Produces:
  - `const jsonFileAnnotationFlag = "docket.jsonfile.flag"` and `const jsonFileAnnotationType = "docket.jsonfile.type"`
  - `func jsonFileTypeName(t reflect.Type) string`, which returns `PkgPath() + "." + Name()` after
    dereferencing pointers
  - `func declareJSONFile[T any](cmd *cobra.Command, flag string) func(c *cobra.Command) (T, error)`
  - `func changeSubcommand[T any](group, verb, short string, run func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req T), effects ...Effect) *cobra.Command`
  - `func changeInputSubcommand[T any](verb, short string, run func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req T), effects ...Effect) *cobra.Command`

`decodeInputFlag` stays in this task, because the finalize commands still call it. Task 3 deletes
it.

- [ ] **Step 1: Write the failing tests.** Create `internal/cli/jsonfile_test.go`:

```go
package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// TestDeclareJSONFileRecordsTheDecodedType proves the declaration records the
// flag and the full type identity of T, and the returned decoder strictly
// decodes that same T from the declared flag (stdin via "-").
func TestDeclareJSONFileRecordsTheDecodedType(t *testing.T) {
	cmd := &cobra.Command{Use: "probe"} // nil Annotations: the helper must allocate
	cmd.Flags().String("input", "", "")
	decode := declareJSONFile[app.ChangeHaltInput](cmd, "input")

	if got := cmd.Annotations[jsonFileAnnotationFlag]; got != "input" {
		t.Errorf("flag annotation = %q, want input", got)
	}
	want := jsonFileTypeName(reflect.TypeFor[app.ChangeHaltInput]())
	if !strings.HasSuffix(want, "/internal/app.ChangeHaltInput") {
		t.Fatalf("jsonFileTypeName = %q, want a full package path ending /internal/app.ChangeHaltInput", want)
	}
	if got := cmd.Annotations[jsonFileAnnotationType]; got != want {
		t.Errorf("type annotation = %q, want %q", got, want)
	}

	if err := cmd.Flags().Set("input", "-"); err != nil {
		t.Fatal(err)
	}
	cmd.SetIn(strings.NewReader(`{"report":"r"}`))
	got, err := decode(cmd)
	if err != nil || got.Report != "r" {
		t.Fatalf("decode = %+v, %v; want Report r", got, err)
	}

	cmd.SetIn(strings.NewReader(`{"report":"r","schema_version":1}`))
	if _, err := decode(cmd); err == nil || !strings.Contains(err.Error(), "--input") || !strings.Contains(err.Error(), "accepted keys: report") {
		t.Fatalf("unknown key: err = %v, want a --input refusal naming the accepted keys", err)
	}
}

// TestDeclareJSONFileKeepsCapabilityAnnotations proves the declaration adds to,
// never replaces, the capability annotations a leaf already carries.
func TestDeclareJSONFileKeepsCapabilityAnnotations(t *testing.T) {
	cmd := &cobra.Command{Use: "probe", Annotations: capability("probe.op", EffectRead)}
	declareJSONFile[app.ChangeHaltInput](cmd, "input")
	if cmd.Annotations[capAnnotationID] != "probe.op" || cmd.Annotations[capAnnotationEffects] != string(EffectRead) {
		t.Fatalf("capability annotations lost: %v", cmd.Annotations)
	}
}

// TestDeclareJSONFileRefusesASecondDeclaration proves one command declares at
// most one JSON file, so the recorded type can never be silently overwritten.
func TestDeclareJSONFileRefusesASecondDeclaration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a second declareJSONFile on one command did not panic")
		}
	}()
	cmd := &cobra.Command{Use: "probe"}
	declareJSONFile[app.ChangeHaltInput](cmd, "input")
	declareJSONFile[app.FinalizeBlockInput](cmd, "request")
}
```

- [ ] **Step 2: Run them and confirm they fail.**
  Run: `go test -count=1 -timeout 300s -run 'TestDeclareJSONFile' ./internal/cli/`
  Expected: build failure, `undefined: declareJSONFile`.

- [ ] **Step 3: Implement the helper.** Create `internal/cli/jsonfile.go`:

```go
package cli

import (
	"fmt"
	"reflect"

	"github.com/spf13/cobra"
)

// The JSON-file declaration annotations. declareJSONFile writes them when a
// command is built; TestPublishedRequestIsTheDecodedJSONFile reads them to
// prove each operation's published request (internal/app operationBindings)
// is exactly the type its decoder reads (ADR-0109's request surface).
const (
	jsonFileAnnotationFlag = "docket.jsonfile.flag"
	jsonFileAnnotationType = "docket.jsonfile.type"
)

// jsonFileTypeName is the type identity the declaration records and the guard
// compares: the full package path and type name, pointers dereferenced.
func jsonFileTypeName(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.PkgPath() + "." + t.Name()
}

// declareJSONFile is the single place a command declares "I read one strict
// JSON document of type T from --flag". It records T and the flag as
// annotations on cmd at construction time and returns the only decoder the
// command's RunE may use. The declaration and the decode share the type
// parameter T, so they cannot diverge. Every strict JSON decode in this
// package goes through here; TestJSONFileDecodesGoThroughTheRegisteringHelper
// fails on any other reference to decodeRequest.
//
// A second declaration on the same command panics: one operation reads at
// most one JSON file, and a silent overwrite would hide the first type from
// the guard. The panic fires when the command tree is built, which every
// cli test does.
func declareJSONFile[T any](cmd *cobra.Command, flag string) func(c *cobra.Command) (T, error) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	if prior, dup := cmd.Annotations[jsonFileAnnotationType]; dup {
		panic(fmt.Sprintf("command %q already declares JSON file type %s", cmd.Name(), prior))
	}
	cmd.Annotations[jsonFileAnnotationFlag] = flag
	cmd.Annotations[jsonFileAnnotationType] = jsonFileTypeName(reflect.TypeFor[T]())
	return func(c *cobra.Command) (T, error) {
		var v T
		source, _ := c.Flags().GetString(flag)
		err := decodeRequest(c.InOrStdin(), "--"+flag, source, &v)
		return v, err
	}
}
```

- [ ] **Step 4: Run the helper tests and confirm they pass.**
  Run: `go test -count=1 -timeout 300s -run 'TestDeclareJSONFile' ./internal/cli/`
  Expected: PASS.

- [ ] **Step 5: Make the two shared builders generic.** In `internal/cli/change.go`, rewrite
  `changeSubcommand` so the builder decodes, and the `run` callback receives the decoded request
  and returns nothing. Keep the existing order: repo dir, then deps, then decode.

```go
func changeSubcommand[T any](group, verb, short string, run func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req T), effects ...Effect) *cobra.Command {
	cmd := &cobra.Command{
		Use:         verb,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: capability(group+"."+verb, effects...),
	}
	cmd.Flags().String("request", "", "JSON request `file`, or - to read the request from stdin (required)")
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = cmd.MarkFlagRequired("request")
	decode := declareJSONFile[T](cmd, "request")
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		repoDir, err := resolveRepoDir(c)
		if err != nil {
			return err
		}
		deps, err := newPlanningDeps(repoDir)
		if err != nil {
			return err
		}
		req, err := decode(c)
		if err != nil {
			return err
		}
		run(c, deps, repoDir, req)
		return nil
	}
	return cmd
}
```

  Rewrite `changeInputSubcommand` the same way, with flag `"input"` and its existing help string
  and `MarkFlagRequired("input")`. Update both doc comments. Say that the builder declares and
  decodes the request through `declareJSONFile`, and remove the sentence about `run` decoding the
  request.

- [ ] **Step 6: Convert every call site.** Each `run` closure now takes the typed request as its
  fourth parameter, and the type is inferred from it. Example (`change create`):

```go
	create := changeSubcommand("change", "create",
		"Create a new proposed change from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req app.ChangeCreateRequest) {
			setResult(app.ChangeCreate(c.Context(), deps, repoDir, req))
		}, EffectMetadataWrite)
```

  Apply the same shape to `change groom|block|defer|unblock|revive|kill` (the existing request
  types), `change reconcile` (`app.ChangeReconcileRequest`, via `changeInputSubcommand`),
  `adr record` (`app.ADRRecordRequest`), `adr supersede` and `adr reverse` (`app.ADRReplaceRequest`),
  and `learning record|update` (`app.LearningRecordRequest`, `app.LearningUpdateRequest`). Then do
  `change halt`:

```go
	halt := changeInputSubcommand("halt",
		"Record a bounded run-halted report on an in-progress change from a JSON request",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string, in app.ChangeHaltInput) {
			id, _ := c.Flags().GetInt("id")
			revision, _ := c.Flags().GetString("revision")
			setResult(app.ChangeHalt(c.Context(), deps, repoDir, app.HaltRequest{ID: id, Revision: revision, Report: in.Report}))
		}, EffectMetadataWrite)
```

  Delete the `changeHaltInput` type and `decodeRequestFlag`. In `adr.go` and `learning.go`, edit
  the file-header plumbing comment so it names `declareJSONFile` instead of `decodeRequestFlag`.
  Confirm nothing else references the deleted names:
  `grep -rn -e decodeRequestFlag -e changeHaltInput internal/` must print nothing.

- [ ] **Step 7: Run the focused tests.**
  Run: `go build ./... && go test -count=1 -timeout 900s ./internal/cli/`
  Expected: PASS. The existing change, adr, learning, halt and revision tests prove that behaviour
  did not change.
  Run: `go vet ./internal/cli/ && go vet -tags integration ./internal/cli/ && gofmt -l internal/cli/`
  Expected: no output.

- [ ] **Step 8: Commit.**

```bash
git add internal/cli/jsonfile.go internal/cli/jsonfile_test.go internal/cli/change.go internal/cli/adr.go internal/cli/learning.go
git commit -m "feat(cli): declare each command's JSON request file through one helper"
```

---

### Task 3: Route finalize and pr decodes through the helper, guarded by an AST scan

**Build tier:** standard

**Files:**
- Create: `internal/cli/jsonfile_scan_test.go`
- Modify: `internal/cli/finalize.go` (closeout, block, retarget-children, rebase-continue,
  rebase-abort, delete `closeoutInput`, `finalizeBlockInput`, `retargetChildrenInput`, and
  update the comments that name `decodeInputFlag`)
- Modify: `internal/cli/pr.go` (publish, delete `prBodyRequest`)
- Modify: `internal/cli/change.go` (delete `decodeInputFlag`)
- Modify: `internal/cli/finalize_test.go` (`TestFinalizeCloseoutInputDecode` uses `app.CloseoutNotes`)

**Interfaces:**
- Consumes: `declareJSONFile[T]` (Task 2). `app.FinalizeBlockInput`, `app.RetargetChildrenInput`,
  `app.PRPublishInput`, `app.CloseoutNotes`, `app.ResolverReport` (Task 1 / existing).
- Produces: `func scanJSONDecodeSites(dir string) (violations []string, homed map[string]int, err error)`
  and `var jsonDecodeHome = map[string]string{...}` (test file only).

- [ ] **Step 1: Write the failing guard.** Create `internal/cli/jsonfile_scan_test.go`:

```go
package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// jsonDecodeHome maps each strict-JSON decode primitive to the single function
// allowed to reference it. decodeRequest may be reached only through
// declareJSONFile, which records the decoded type on the command; the raw
// encoding/json primitives live only inside decodeRequest.
var jsonDecodeHome = map[string]string{
	"decodeRequest":         "declareJSONFile",
	"json.NewDecoder":       "decodeRequest",
	"DisallowUnknownFields": "decodeRequest",
}

// scanJSONDecodeSites parses every non-test .go file in dir and reports each
// reference to a decode primitive outside its home function. The scan keys on
// syntactic shape: any identifier decodeRequest (a call, or a value
// reference), any selector .DisallowUnknownFields, and json.NewDecoder. It
// never consults a list of call sites. A reference at package level (outside
// any function) is always a violation. homed counts the references found in
// their home, so a caller can prove the scan still sees the real decode path.
func scanJSONDecodeSites(dir string) (violations []string, homed map[string]int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	homed = map[string]int{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, nil, err
		}
		for _, decl := range file.Decls {
			owner := "" // package level
			var root ast.Node = decl
			if fd, ok := decl.(*ast.FuncDecl); ok {
				if fd.Body == nil {
					continue
				}
				owner, root = fd.Name.Name, fd.Body
			}
			ast.Inspect(root, func(n ast.Node) bool {
				key := ""
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if x.Sel.Name == "DisallowUnknownFields" {
						key = "DisallowUnknownFields"
					} else if pkg, ok := x.X.(*ast.Ident); ok && pkg.Name == "json" && x.Sel.Name == "NewDecoder" {
						key = "json.NewDecoder"
					}
				case *ast.Ident:
					if x.Name == "decodeRequest" {
						key = "decodeRequest"
					}
				}
				if key == "" {
					return true
				}
				if owner != "" && owner == jsonDecodeHome[key] {
					homed[key]++
					return true
				}
				where := owner
				if where == "" {
					where = "package level"
				}
				violations = append(violations, fmt.Sprintf("%s: %s references %s; only %s may (route the decode through declareJSONFile)",
					fset.Position(n.Pos()), where, key, jsonDecodeHome[key]))
				return true
			})
		}
	}
	return violations, homed, nil
}

// TestJSONFileDecodesGoThroughTheRegisteringHelper is the bypass guard: no
// production file in this package may strictly decode JSON except through
// declareJSONFile, so every JSON request file an operation reads is declared
// on its command (where TestPublishedRequestIsTheDecodedJSONFile compares it
// with the published schema). The homed floor proves the scan still reaches
// the real decode path; a rename that blinds it reddens here.
func TestJSONFileDecodesGoThroughTheRegisteringHelper(t *testing.T) {
	violations, homed, err := scanJSONDecodeSites(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations {
		t.Error(v)
	}
	for key, home := range jsonDecodeHome {
		if homed[key] == 0 {
			t.Errorf("no reference to %s found inside %s; the scan no longer sees the decode path, so it guards nothing", key, home)
		}
	}
}

// TestJSONDecodeScanReportsAStrayDecode is the scan's positive control: a
// scratch package with one stray decodeRequest call and one package-level
// value reference reports exactly those two, and nothing for the homed uses.
func TestJSONDecodeScanReportsAStrayDecode(t *testing.T) {
	dir := testsupport.TempDir(t)
	src := `package cli

import "encoding/json"

func declareJSONFile() { _ = decodeRequest(nil, "", "", nil) }

func decodeRequest(a, b, c, d any) error {
	dec := json.NewDecoder(nil)
	dec.DisallowUnknownFields()
	return nil
}

func stray() { _ = decodeRequest(nil, "--x", "-", nil) }

var leaked = decodeRequest
`
	if err := os.WriteFile(filepath.Join(dir, "stray.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, homed, err := scanJSONDecodeSites(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 2 || !strings.Contains(violations[0], "stray references decodeRequest") || !strings.Contains(violations[1], "package level references decodeRequest") {
		t.Fatalf("violations = %q, want exactly the stray call and the package-level reference", violations)
	}
	for key := range jsonDecodeHome {
		if homed[key] != 1 {
			t.Errorf("homed[%s] = %d, want 1", key, homed[key])
		}
	}
}
```

- [ ] **Step 2: Run the guard and confirm it fails for the intended reason.**
  Run: `go test -count=1 -timeout 300s -run 'TestJSONFileDecodesGoThroughTheRegisteringHelper|TestJSONDecodeScanReportsAStrayDecode' ./internal/cli/`
  Expected: `TestJSONDecodeScanReportsAStrayDecode` PASSES. `TestJSONFileDecodesGoThroughTheRegisteringHelper`
  FAILS with exactly two violations: `decodeInputFlag references decodeRequest` (`change.go`)
  and `newPRCommand references decodeRequest` (`pr.go`). If the list is different, stop and
  investigate before going on.

- [ ] **Step 3: Convert the five finalize commands.** Each builder creates `cmd` without `RunE`,
  declares the file, and then assigns `cmd.RunE`. Example (`finalize block`):

```go
	cmd := &cobra.Command{
		Use:         "block",
		Short:       "Record a blocked finalize attempt: an owned PR comment then a durable marker",
		Args:        cobra.NoArgs,
		Annotations: capability("finalize.block", EffectExternalWrite, EffectMetadataWrite),
	}
	decode := declareJSONFile[app.FinalizeBlockInput](cmd, "input")
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		repoDir, err := resolveRepoDir(c)
		if err != nil {
			return err
		}
		id, _ := c.Flags().GetInt("id")
		revision, _ := c.Flags().GetString("revision")
		prNumber, _ := c.Flags().GetInt("pr-number")
		attempt, _ := c.Flags().GetString("attempt")
		reason, _ := c.Flags().GetString("reason")
		head, _ := c.Flags().GetString("head")
		in, err := decode(c)
		if err != nil {
			return err
		}
		deps, err := newFinalizeDeps(repoDir)
		if err != nil {
			return err
		}
		setResult(app.FinalizeBlock(c.Context(), deps, repoDir, app.BlockRequest{
			ID: id, Revision: revision, PRNumber: prNumber, Attempt: attempt,
			Reason: reason, Head: head, Report: in.Report, Remedy: in.Remedy,
		}))
		return nil
	}
```

  Keep every existing comment on the annotations (the effect rationale), and keep every flag
  definition and `MarkFlagRequired` exactly as it is. Apply the same pattern to:
  - `finalize retarget-children`: `declareJSONFile[app.RetargetChildrenInput](cmd, "input")`, and
    pass `in.Children` into `app.RetargetChildrenRequest`.
  - `finalize rebase-continue` and `finalize rebase-abort`: `declareJSONFile[app.ResolverReport](cmd, "input")`,
    and pass the decoded report directly. The decode still happens before `newFinalizeDeps`.
  - `finalize closeout`: `declareJSONFile[app.CloseoutNotes](cmd, "input")`. The `--input` flag
    stays optional, so keep the guard:

```go
		var notes app.CloseoutNotes
		if src, _ := c.Flags().GetString("input"); src != "" {
			if notes, err = decode(c); err != nil {
				return err
			}
		}
```

    Then pass `notes` straight to `app.FinalizeCloseout(..., id, notes)`.

  Delete `closeoutInput`, `finalizeBlockInput` and `retargetChildrenInput` along with their doc
  comments. Move any still-true sentence (for example "the scalar identities ride on flags") into
  the builder's doc comment. Edit the remaining comments that say "DisallowUnknownFields (via
  decodeInputFlag)" so they say the file is declared and strictly decoded through
  `declareJSONFile`.

- [ ] **Step 4: Convert `pr publish`.** In `internal/cli/pr.go`, declare the body file on
  `publish` and assign `publish.RunE` after the struct literal:

```go
	decodeBody := declareJSONFile[app.PRPublishInput](publish, "body")
```

  In the `RunE`, replace the `prBodyRequest` decode with `body, err := decodeBody(c)` and keep
  `readRecordSource` for `--evidence` unchanged (it is not a JSON request). Delete `prBodyRequest`
  and its comment, and say in the file header comment that the `--body` file type is
  `app.PRPublishInput`.

- [ ] **Step 5: Delete `decodeInputFlag`** from `internal/cli/change.go`.
  `grep -rn -e decodeInputFlag -e prBodyRequest -e closeoutInput -e finalizeBlockInput -e retargetChildrenInput internal/`
  should then only print `internal/cli/finalize_test.go`. Fix that file in the next step.

- [ ] **Step 6: Update `TestFinalizeCloseoutInputDecode`** in `internal/cli/finalize_test.go`.
  Replace both `closeoutInput` declarations with `app.CloseoutNotes`. Its `decodeRequest` calls
  stay, because the scan excludes test files.

- [ ] **Step 7: Run the focused tests.**
  Run: `go build ./... && go test -count=1 -timeout 900s ./internal/cli/`
  Expected: PASS, including `TestJSONFileDecodesGoThroughTheRegisteringHelper` with no violations,
  the closeout, block, retarget, rebase and pr tests, and `TestRevisionFlagReachesRequest`.
  Run: `go vet ./internal/cli/ && go vet -tags integration ./internal/cli/ && gofmt -l internal/cli/`
  Expected: no output.

- [ ] **Step 8: Commit.**

```bash
git add internal/cli/jsonfile_scan_test.go internal/cli/finalize.go internal/cli/pr.go internal/cli/change.go internal/cli/finalize_test.go
git commit -m "feat(cli): route finalize and pr request files through declareJSONFile"
```

---

### Task 4: Bind each operation's request to its JSON file, and add the published-request guard

**Build tier:** premium

The named risk: this task replaces registry accounting with a new guard. Following the learnings
on compensating asserts and test premises, write the new guard and watch it go red **before**
relaxing or retiring any existing assert.

**Files:**
- Create: `internal/cli/jsonfile_production_test.go`
- Modify: `internal/app/schema_registry.go` (bindings, plus the `OperationBinding` and `operationBindings` doc comments)
- Modify: `internal/app/schema_registry_test.go` (`TestEveryRequestAndResultStructIsBound`)
- Modify: `internal/app/schema_test.go` (delete `TestFlagAssembledRequestsEmitSnakeCaseKeys`)
- Modify: `internal/app/schema_revision_test.go` (`TestSchemaRevisionKeys`)
- Modify: `docs/reference/glossary.md` (the "Schema / request file" entry)

**Interfaces:**
- Consumes: `jsonFileAnnotationFlag`, `jsonFileAnnotationType`, `jsonFileTypeName` (Task 2).
  `requestJSONKeys(dst any) []string` (`internal/cli/requestkeys.go`). `productionRootForTest(t)`
  (`internal/cli/capability_production_test.go`). `capAnnotationID`. `runCLI(t, args...)`
  (`internal/cli/root_test.go`). `app.OperationBindings()`, `app.SchemaFor(id, effects)`,
  `app.Schema(effects)`, `app.FieldDescriptor{Key, Type, Repeated, Enum, Fields}`,
  `app.SchemaResult.Vocabularies[name].Members`.
- Produces: the guards `TestPublishedRequestIsTheDecodedJSONFile`,
  `TestPublishedRequestKeysAreAccepted` and `TestSchemaPublishesTheJSONFileOfEachFixedOperation`.

- [ ] **Step 1: Write the guard and the two behaviour tests.** Create
  `internal/cli/jsonfile_production_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// TestPublishedRequestIsTheDecodedJSONFile is the guard for the published
// request rule: an operation's `docket schema` request is exactly the JSON
// file type its command decodes (recorded by declareJSONFile), or absent when
// the command decodes none. It walks the real production tree, so the set of
// JSON-reading operations is derived, never listed. For each JSON-reading
// operation it also compares the decoder's accepted keys with the keys the
// schema publishes, so it checks the published surface, not only Go types.
func TestPublishedRequestIsTheDecodedJSONFile(t *testing.T) {
	bindings := map[string]app.OperationBinding{}
	for _, b := range app.OperationBindings() {
		bindings[b.ID] = b
	}
	leaves, readers := 0, 0
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, child := range c.Commands() {
			walk(child)
			typ, declares := child.Annotations[jsonFileAnnotationType]
			id, isOp := child.Annotations[capAnnotationID]
			if !isOp {
				if declares {
					t.Errorf("%q declares JSON file type %s but carries no capability id", child.CommandPath(), typ)
				}
				continue
			}
			leaves++
			b, bound := bindings[id]
			if !bound {
				if declares {
					t.Errorf("operation %q reads JSON file type %s but has no schema binding", id, typ)
				}
				continue
			}
			want := ""
			if b.Request != nil {
				want = jsonFileTypeName(reflect.TypeOf(b.Request))
			}
			switch {
			case typ == want:
			case !declares:
				t.Errorf("operation %q reads no JSON file, but docket schema publishes request %s; bind Request: nil", id, want)
				continue
			case want == "":
				t.Errorf("operation %q reads JSON file type %s, but docket schema publishes no request; bind that type", id, typ)
				continue
			default:
				t.Errorf("operation %q reads JSON file type %s, but docket schema publishes %s", id, typ, want)
				continue
			}
			if !declares {
				continue
			}
			readers++
			if flag := child.Annotations[jsonFileAnnotationFlag]; child.Flags().Lookup(flag) == nil {
				t.Errorf("operation %q declares JSON file flag --%s, which the command does not define", id, flag)
			}
			doc, ok, err := app.SchemaFor(id, []string{"read"})
			if err != nil || !ok || len(doc.Operations) != 1 || doc.Operations[0].Request == nil {
				t.Errorf("operation %q: docket schema publishes no request block (ok=%v err=%v)", id, ok, err)
				continue
			}
			var published []string
			for _, f := range doc.Operations[0].Request.Fields {
				published = append(published, f.Key)
			}
			sort.Strings(published)
			if got := requestJSONKeys(b.Request); !reflect.DeepEqual(got, published) {
				t.Errorf("operation %q: the decoder accepts keys %v but docket schema publishes %v", id, got, published)
			}
		}
	}
	walk(productionRootForTest(t))
	t.Logf("%d operation leaves, %d read a JSON request file", leaves, readers)
	if leaves == 0 || readers == 0 {
		t.Fatalf("vacuous walk: %d operation leaves, %d JSON-file readers", leaves, readers)
	}
}

// sampleRequest builds a JSON object from exactly the published fields: a
// zero value per type word, the first member of an enum vocabulary when it has
// members, nested objects filled from their own published fields, and a
// one-element array for a repeated field.
func sampleRequest(fields []app.FieldDescriptor, vocab map[string]app.Vocabulary) map[string]any {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		var v any
		switch f.Type {
		case "int":
			v = 0
		case "bool":
			v = false
		case "object":
			v = sampleRequest(f.Fields, vocab)
		case "map[string]string":
			v = map[string]string{}
		default:
			v = ""
			if m := vocab[f.Enum].Members; f.Enum != "" && len(m) > 0 {
				v = m[0]
			}
		}
		if f.Repeated {
			v = []any{v}
		}
		out[f.Key] = v
	}
	return out
}

// TestPublishedRequestKeysAreAccepted proves, for every operation that
// publishes a request, that a file built only from the published keys is
// accepted by the strict decoder, and that adding the envelope key
// schema_version is refused with every accepted key named.
func TestPublishedRequestKeysAreAccepted(t *testing.T) {
	doc, err := app.Schema([]string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]app.OperationBinding{}
	for _, b := range app.OperationBindings() {
		bindings[b.ID] = b
	}
	checked := 0
	for _, op := range doc.Operations {
		if op.Request == nil {
			continue
		}
		checked++
		typ := reflect.TypeOf(bindings[op.ID].Request)
		sample := sampleRequest(op.Request.Fields, doc.Vocabularies)
		body, _ := json.Marshal(sample)
		if err := decodeRequest(bytes.NewReader(body), "--input", "-", reflect.New(typ).Interface()); err != nil {
			t.Errorf("operation %q: a file built from its published keys is refused: %v\n%s", op.ID, err, body)
		}
		if _, clash := sample["schema_version"]; clash {
			continue
		}
		sample["schema_version"] = 1
		body, _ = json.Marshal(sample)
		err := decodeRequest(bytes.NewReader(body), "--input", "-", reflect.New(typ).Interface())
		accepted := "accepted keys: " + strings.Join(requestJSONKeys(reflect.New(typ).Interface()), ", ")
		if err == nil || !strings.Contains(err.Error(), `unknown field "schema_version"`) || !strings.Contains(err.Error(), accepted) {
			t.Errorf("operation %q: schema_version refusal = %v, want an unknown-field refusal naming %q", op.ID, err, accepted)
		}
	}
	if checked == 0 {
		t.Fatal("no operation publishes a request; the check is vacuous")
	}
}

// TestSchemaPublishesTheJSONFileOfEachFixedOperation pins, end to end through
// `docket schema --operation <id> --json`, the request each operation fixed
// under this rule publishes, and that a flag-only operation resolves with no
// request block (not an unknown-operation refusal).
func TestSchemaPublishesTheJSONFileOfEachFixedOperation(t *testing.T) {
	resolver := []string{"attempt", "change_id", "conflicted_paths", "disposition", "observed_base",
		"observed_head", "recommended_action", "resolver_reservation", "summary", "touched_paths"}
	cases := map[string][]string{
		"finalize.block":             {"remedy", "report"},
		"finalize.rebase-continue":   resolver,
		"finalize.rebase-abort":      resolver,
		"finalize.closeout":          {"late_findings", "verification_outcomes"},
		"finalize.retarget-children": {"children"},
		"change.halt":                {"report"},
		"pr.publish":                 {"body", "title"},
		"finalize.rebase":            nil,
		"change.claim":               nil,
		"finalize.merge":             nil,
	}
	type field struct {
		Key    string  `json:"key"`
		Fields []field `json:"fields"`
	}
	for id, want := range cases {
		out, errS, code := runCLI(t, "schema", "--operation", id, "--json")
		if code != 0 || errS != "" {
			t.Errorf("schema --operation %s: code=%d err=%q", id, code, errS)
			continue
		}
		var doc struct {
			Operations []struct {
				Request *struct {
					Fields []field `json:"fields"`
				} `json:"request"`
			} `json:"operations"`
		}
		if err := json.Unmarshal([]byte(out), &doc); err != nil || len(doc.Operations) != 1 {
			t.Errorf("schema --operation %s: %v\n%s", id, err, out)
			continue
		}
		req := doc.Operations[0].Request
		if want == nil {
			if req != nil {
				t.Errorf("flag-only operation %s publishes a request %v, want none", id, req.Fields)
			}
			continue
		}
		if req == nil {
			t.Errorf("operation %s publishes no request, want keys %v", id, want)
			continue
		}
		var got []string
		for _, f := range req.Fields {
			got = append(got, f.Key)
			if id == "finalize.retarget-children" && f.Key == "children" {
				var nested []string
				for _, n := range f.Fields {
					nested = append(nested, n.Key)
				}
				sort.Strings(nested)
				if !reflect.DeepEqual(nested, []string{"id", "pr_number", "pr_revision"}) {
					t.Errorf("children nested keys = %v, want [id pr_number pr_revision]", nested)
				}
			}
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("operation %s publishes request keys %v, want %v", id, got, want)
		}
	}
}
```

  If `app.Vocabulary` has a different exported name, read it from `internal/app/schema_vocab.go`
  and adjust `sampleRequest`'s parameter type. `SchemaResult.Vocabularies` is
  `map[string]Vocabulary` with a `Members []string` field.

- [ ] **Step 2: Run the new tests and confirm the guard fails for the intended reason.**
  Run: `go test -count=1 -timeout 600s -run 'TestPublishedRequestIsTheDecodedJSONFile|TestPublishedRequestKeysAreAccepted|TestSchemaPublishesTheJSONFileOfEachFixedOperation' ./internal/cli/`
  Expected: `TestPublishedRequestIsTheDecodedJSONFile` FAILS with one error for each of the 7
  mismatched operations and each of the flag-only operations listed in Step 3. Read the list and
  confirm it matches Step 3 exactly. A difference is a finding about the code, so investigate it
  before going on. `TestSchemaPublishesTheJSONFileOfEachFixedOperation` FAILS on the same
  operations. `TestPublishedRequestKeysAreAccepted` may already pass, because the current bindings
  are internally consistent.

- [ ] **Step 3: Rebind the registry.** In `internal/app/schema_registry.go`, keep every entry's
  `Result` and trailing function-name comment, and change only these `Request` values:
  - To the JSON file type: `change.halt` → `ChangeHaltInput{}`, `finalize.block` →
    `FinalizeBlockInput{}`, `finalize.closeout` → `CloseoutNotes{}`, `finalize.rebase-abort` →
    `ResolverReport{}`, `finalize.rebase-continue` → `ResolverReport{}`,
    `finalize.retarget-children` → `RetargetChildrenInput{}`, `pr.publish` → `PRPublishInput{}`.
  - To `nil` (flag-only, no JSON file): `artifact.backlink`, `change.attach-plan`,
    `change.attach-results`, `change.claim`, `change.mark-implemented`, `change.reclaim`,
    `change.refresh-claim`, `change.relink`, `change.resume-halted`, `context.finalize`,
    `context.implementation`, `evidence.recertify`, `evidence.record`, `evidence.verify`,
    `finalize.clear-block`, `finalize.merge`, `finalize.publish`, `finalize.rebase`,
    `gate.drive.start`, `run.verify`, `workspace.inspect`, `workspace.prepare`,
    `workspace.publish`.
  - Unchanged (already the decoded type): `adr.*`, `change.block|create|defer|groom|kill|reconcile|revive|unblock`,
    `learning.*`.

  The Step 2 failure output is the oracle. If it names a different set, follow the guard, not this
  list, and say so in the commit body.

  Rewrite the doc comments so they state the rule:
  - `OperationBinding`: "Request is exactly the JSON document the operation strictly decodes from a
    file (`--request`, `--input`, or `--body`), or nil when it decodes none. Scalar flags are
    described only by the capability catalog's `signature`, and non-JSON file inputs (the
    canonical build-evidence record, `agent.enter`'s plain-text request) are not requests."
  - `operationBindings`: replace "every entry's Request is the *Request struct that handler
    decodes or assembles (nil when it assembles none)" with the same rule. Add that a
    flag-assembled `*Request` struct is an internal app input and is never bound, and that
    `TestPublishedRequestIsTheDecodedJSONFile` (internal/cli) proves each binding's Request is the
    type the command declares through `declareJSONFile`. Keep the ADR-0109 framing and the
    `development.test` paragraph.

- [ ] **Step 4: Run the new guard and confirm it passes.**
  Run: the Step 2 command.
  Expected: all three PASS. The guard's log line reports the leaf and reader counts. Readers
  should be 20: the 14 commands Task 2 converted (7 `change` `--request`, `change reconcile`,
  `change halt`, 3 `adr`, 2 `learning`) and the 6 from Task 3 (5 `finalize`, `pr publish`).
  Record the actual number in the commit body. Do not assert it in code.
  If `TestPublishedRequestKeysAreAccepted` fails for an operation because a custom
  `UnmarshalJSON` rejects a sample value, rather than because a key is unknown, change
  `sampleRequest` to draw a valid value from the published vocabulary. Never loosen the decoder.

- [ ] **Step 5: Adjust the registry-accounting test.** In `internal/app/schema_registry_test.go`:
  - Rename `TestEveryRequestAndResultStructIsBound` to `TestEveryResultStructIsBound` (the old
    name would misdescribe the test). Run `grep -rn --exclude-dir=.git --exclude-dir=docs -e TestEveryRequestAndResultStructIsBound .`
    and update any maintained-source comment that names it.
  - Narrow `astRequestResultTypeNames` to `astResultTypeNames`, which collects only exported
    struct names ending in `Result`, and update its doc comment and fatal message.
  - Delete `excludedRequestTypes` and its honesty loop. Keep `excludedResultTypes` and its loop.
  - Keep the forward loop over result names unchanged.
  - Reverse check: a non-nil `Request` must be a struct declared in package app, with **no** name
    suffix requirement (`ResolverReport` and `CloseoutNotes` are bound). Keep every Result
    reverse check as it is.
  - Rewrite the doc comment. The forward half accounts for every exported `*Result`. Request
    accounting lives in `TestPublishedRequestIsTheDecodedJSONFile` (internal/cli), which proves
    each bound request is the decoded type. A flag-assembled `*Request` struct is deliberately
    unbound.

- [ ] **Step 6: Retire and adjust the request-shape tests.**
  - Delete `TestFlagAssembledRequestsEmitSnakeCaseKeys` from `internal/app/schema_test.go`. None
    of its four types is published any more, so the property it guarded is gone. If that leaves
    the `regexp` import unused, remove it.
  - In `internal/app/schema_revision_test.go` `TestSchemaRevisionKeys`, delete the `REQ` paths
    of operations that no longer publish a request, or whose file no longer carries the key:
    `change.attach-plan`, `change.attach-results`, `change.claim`, `change.halt`,
    `change.mark-implemented`, `change.reclaim`, `change.refresh-claim`, `change.relink`
    (`ExpectRevision`), `change.resume-halted`, `finalize.block`, `finalize.clear-block`,
    `finalize.merge REQ revision`, `finalize.rebase`, `finalize.retarget-children REQ revision`,
    `workspace.inspect`, `workspace.prepare`. Keep every other path, including
    `finalize.retarget-children REQ children.pr_revision`, `finalize.merge RES merge.pr_revision`
    and every `RES` path. Extend its doc comment: request paths cover only operations that read a
    JSON file, and `TestRevisionFlagReachesRequest` (internal/cli) pins the `--revision` flag
    spelling.
  - Leave `TestSchemaRequestAbsentForReadOnlyLeaves`, `TestSchemaExcludesEnvelopeKeysPerOp`
    and `TestSchemaCatalogCorrespondence` unchanged. Their assertions still hold.

- [ ] **Step 7: Update the glossary entry.** In `docs/reference/glossary.md`, replace the two
  prose lines under `### Schema / request file` (keep the code fence) with:

```markdown
`docket schema` emits every operation's result fields plus the allowed values. An operation that
reads a JSON file (`--request`, `--input`, or `--body`) also gets a request block listing exactly
the keys that file accepts. Values passed as flags (`--id`, `--revision`, …) are not part of any
request; `docket capabilities --json` lists each operation's flags in its `signature`. A
**request file** is a JSON body built from that request block.
```

- [ ] **Step 8: Run the focused packages.**
  Run: `go test -count=1 -timeout 900s ./internal/app/ ./internal/cli/ ./internal/repoguard/`
  Expected: PASS. Pay attention to `TestOperationBindingsSortedUniqueAndDescribable`. If its
  converse pairing reports an orphaned `_dispositions` vocabulary, an unbound request was that
  family's only reference. Stop and report it as `NEEDS_ESCALATION`. Do not rebind a request to
  satisfy it.
  Run: `go vet ./internal/... && go vet -tags integration ./internal/app/ ./internal/cli/ && go vet -tags e2e ./internal/app/ && gofmt -l internal/`
  Expected: no output.

- [ ] **Step 9: Mutation probe of the guard (not committed).** Back up
  `internal/app/schema_registry.go`, rebind `finalize.block`'s Request to `BlockRequest{}`, and run
  `go test -count=1 -timeout 600s -run TestPublishedRequestIsTheDecodedJSONFile ./internal/cli/`.
  Expected: FAIL naming `finalize.block`, with `.../internal/app.FinalizeBlockInput` read and
  `.../internal/app.BlockRequest` published. Restore from the backup and re-run (PASS). Task 5
  repeats this for the record.

- [ ] **Step 10: Commit.**

```bash
git add internal/cli/jsonfile_production_test.go internal/app/schema_registry.go internal/app/schema_registry_test.go internal/app/schema_test.go internal/app/schema_revision_test.go docs/reference/glossary.md
git commit -m "fix(schema): publish each operation's request as exactly the JSON file it reads"
```

  Add any other file the Step 5 grep made you touch.

---

### Task 5: Record both guard mutations and run the full suite

**Build tier:** standard

This task changes no source. Its single commit is an **empty commit** whose message body is the
durable mutation record, which the coordinator copies into the results file. The task text
prescribes that commit, and it overrides the one-code-commit default.

**Files:**
- No source changes. Mutated files are restored byte-for-byte from backups before the commit.

- [ ] **Step 1: Check the starting state.** `git status --porcelain` must be empty.

- [ ] **Step 2: Mutation 1 (the published-request guard).**

```bash
cp -f internal/app/schema_registry.go "${TMPDIR:-/tmp}/schema_registry.go.bak"
```

  Edit `internal/app/schema_registry.go`: change `finalize.block`'s `Request: FinalizeBlockInput{}`
  to `Request: BlockRequest{}`. Then run:
  `go test -count=1 -timeout 600s -run 'TestPublishedRequestIsTheDecodedJSONFile|TestSchemaPublishesTheJSONFileOfEachFixedOperation' ./internal/cli/`
  Expected: both FAIL. The guard reports
  `operation "finalize.block" reads JSON file type …/internal/app.FinalizeBlockInput, but docket schema publishes …/internal/app.BlockRequest`.
  Save the failing lines. Then do a second variant: set `finalize.rebase-continue`'s Request to
  `nil` (finalize.block still mutated is fine) and re-run. Expected: the guard reports
  `operation "finalize.rebase-continue" reads JSON file type …ResolverReport, but docket schema publishes no request`.
  Restore and verify:

```bash
cp -f "${TMPDIR:-/tmp}/schema_registry.go.bak" internal/app/schema_registry.go
git diff --exit-code -- internal/app/schema_registry.go
```

  Re-run the same `go test` command and expect PASS.

- [ ] **Step 3: Mutation 2 (the bypass scan).**

```bash
cp -f internal/cli/pr.go "${TMPDIR:-/tmp}/pr.go.bak"
```

  In `internal/cli/pr.go`, append a stray direct decode to the end of the file:

```go
func strayPRDecode(c *cobra.Command) error {
	var v app.PRPublishInput
	return decodeRequest(c.InOrStdin(), "--body", "-", &v)
}
```

  Run: `go test -count=1 -timeout 300s -run TestJSONFileDecodesGoThroughTheRegisteringHelper ./internal/cli/`
  Expected: FAIL with `pr.go:…: strayPRDecode references decodeRequest; only declareJSONFile may`.
  Save the line. Restore and verify:

```bash
cp -f "${TMPDIR:-/tmp}/pr.go.bak" internal/cli/pr.go
git diff --exit-code -- internal/cli/pr.go
```

  Re-run and expect PASS.

- [ ] **Step 4: Full suite.** `git status --porcelain` must be empty. Then run the configured
  `build.test_command`, entered from source:
  `go run ./cmd/docket development test`
  Expected: green. Read the budget report even when the run is green. Copy any `BUDGET WATCH:`,
  `PARALLEL-SENSITIVE:` or `SERIAL CONFIRMED OVER BUDGET:` line into the commit body. On red,
  do not "fix" by weakening a guard. Return `BLOCKED` with the failing tests named.

- [ ] **Step 5: Commit the mutation record (empty commit).**

```bash
git commit --allow-empty -m "test(schema): record guard mutation probes" -m "Mutation 1 (TestPublishedRequestIsTheDecodedJSONFile): rebinding finalize.block to BlockRequest{} -> FAIL: <paste line>; rebinding finalize.rebase-continue to nil -> FAIL: <paste line>; restored, PASS.
Mutation 2 (TestJSONFileDecodesGoThroughTheRegisteringHelper): stray decodeRequest in internal/cli/pr.go -> FAIL: <paste line>; restored, PASS.
Full suite: go run ./cmd/docket development test -> green at <HEAD sha>; budget lines: <none | pasted>."
```

  Replace each `<…>` with the exact output you observed before committing. Never commit the
  angle-bracket placeholders.
