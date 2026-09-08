<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0376 — `docket gate drive start` human-readable output omits drive_id/generation](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-08-0376-gate-drive-start-human-output-omits-drive-id-generation.md)**
<!-- docket:backlink:end -->
# Gate-Drive JSON Capture Guidance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every maintained instruction surface that consumes a `gate.drive` result explicitly require `--json` capture of the first response, and guard that requirement with a mutation-testable repoguard test — a docs/guidance-only change; the gate driver's behavior and its human-output credential boundary do not change.

**Architecture:** Two tasks. Task 1 edits the shared caller contract (`skills/docket-build/references/gate-caller-loop.md`) to state the JSON-capture requirement once, updates every actionable credential-consuming caller site in the three skill bodies to pass `--json` and capture/validate the first response, and regenerates the byte-identical embedded distribution copies. Task 2 adds a shape-keyed Go guard in `internal/repoguard` that (a) asserts the shared contract carries the capture requirement and the contract-failure rule, and (b) scans the workflow-markdown corpus for flag-bearing invocations of credential-returning `gate.drive` ops and fails any that omit `--json` — proven by mutation.

**Tech Stack:** Markdown skill bodies; Go (`internal/repoguard`, stdlib `regexp`/`strings`/`testing` only); `go run ./cmd/genassets -repo .` for embedded-tree regeneration.

**Spec:** No spec file (`trivial: true`). The plan argues from the change file `docs/changes/active/0376-gate-drive-start-human-output-omits-drive-id-generation.md` on the `docket` metadata branch (its "What changes" + acceptance criteria are the contract; its reconcile log of 2026-09-07 confirms the site inventory below against current main).

## Global Constraints

- **Docs/guidance fix only.** No change to driver behavior, gate outcomes, human-output redaction (human text keeps naming identity and disposition only — never generations, tokens, or capabilities), handoff/claim/takeover authorization, or any caller's WAITING/continuation policy. Only the transport guidance is clarified.
- **Out of scope:** printing credentials in human output; `start` idempotency / duplicate-drive prevention (#375); prepare-scope/start handshake or concurrent-scope identity (#405); new credential-recovery commands; rewriting frozen plans, results, historical specs, or Accepted ADR prose (`docs/superpowers/plans/`, `docs/changes/`, archived material are point-in-time records — the guard must not scan them and the edits must not touch them).
- **Catalog-resolved argv only.** Skill prose keeps referring to operations as "the `gate.drive.start` operation with `--flags…`" — do not introduce hard-coded `docket gate drive …` argv beyond what already exists.
- **Embedded copies are generated, never hand-edited.** `internal/assets/embedded/tree/skills/...` must be regenerated with `go run ./cmd/genassets -repo .` after any `skills/` edit; the assets DiffTree check enforces byte-identity.
- **Guard house rules (AGENTS.md):** key on syntactic shape, never an enumerated list of spellings; derive the site population from a corpus walk, never a hand-list; mutation-test every guard (remove the guarded thing, watch it redden); collapse whitespace before matching wrapped prose; at most one bounded gap per regex (two stacked bounded gaps backtrack catastrophically on non-matching input); defeat Go's test cache with `-count=1` on every mutation probe; restore mutations only from a committed state (`git checkout -- <file>` restores to HEAD).
- **Fact verified 2026-09-07:** all five credential-returning ops (`start`, `handoff`, `claim`, `takeover`, `prepare-scope`) already accept `--json` ("emit protocol-v1 JSON on stdout"); `advance` consumes credentials but returns no new ones, so it is deliberately outside the credential-op set.
- The build gate runs the whole configured suite (`go run ./cmd/docket development test`) — the controller's final gate, not a per-task step.

## Site inventory (derived, to re-derive at build time)

Derived by whole-repo search over maintained (non-generated, non-frozen) surfaces:

```bash
grep -rnE 'gate\.drive\.(start|handoff|claim|takeover|prepare-scope)' \
  --include='*.md' --include='*.toml' skills/ agents/ docs/ internal/ | \
  grep -v 'internal/assets/embedded/' | grep -v 'docs/superpowers/plans/' | grep -v 'docs/changes/'
```

Today that yields exactly four maintained files (agents/ has zero hits; embedded mirrors follow by regeneration):

1. `skills/docket-build/references/gate-caller-loop.md` — the shared contract; mentions `--json` only in passing ("`--json` emits the shared document; human text names identity and disposition only").
2. `skills/docket-build-task/SKILL.md` — worker `gate.drive.start --owner task …` invocation (~line 62) and the `WAITING` → `gate.drive.handoff --drive-id <id> --owner-gen <gen>` step (~lines 73–75). The outcome-list mention at ~line 129 is prose, not an invocation.
3. `skills/docket-build/SKILL.md` — `prepare-scope` before each dispatch (~lines 75–77), `claim`+`advance` continuation (~line 128), `takeover` exceptional branch (~lines 135–142), final full-suite `start --owner build` (~lines 225–228).
4. `skills/docket-implement-next/SKILL.md` — Step 6 evidence re-mint `start --owner build` / `advance --drive-id --owner-gen` (~line 88), and the Step-2/verify prose naming `prepare-scope` / `start --gate-context` (~line 112).

Re-run the grep at build time and fix any site it finds that this inventory missed — the grep, not this list, is authoritative.

---

### Task 1: State the JSON-capture requirement in the shared contract, apply it at every caller site, regenerate embedded copies

**Build profile:** standard (multi-file contract prose where a wrong sentence weakens a live ownership protocol; no code behavior change)

**Files:**
- Modify: `skills/docket-build/references/gate-caller-loop.md`
- Modify: `skills/docket-build-task/SKILL.md`
- Modify: `skills/docket-build/SKILL.md`
- Modify: `skills/docket-implement-next/SKILL.md`
- Regenerate: `internal/assets/embedded/tree/skills/...` (via generator only)

**Interfaces:**
- Produces: the shared-contract section heading `## JSON capture — the required transport for consumed results` and, in every flag-bearing credential-op invocation paragraph, the literal flag `` `--json` ``. Task 2's guard regexes key on exactly these (collapsed-whitespace phrases quoted in Task 2), so keep the bolded phrases below verbatim.

- [ ] **Step 1: Add the shared requirement section to `skills/docket-build/references/gate-caller-loop.md`.**

Insert a new section between "## The driver's operations" (after the paragraph ending "…not an excuse to omit the document.") and "## The disposition vocabulary and what each earns":

```markdown
## JSON capture — the required transport for consumed results

Any invocation whose result a workflow consumes MUST pass `--json` and **capture** the emitted
protocol-v1 document from that same first response, then **validate** that the fields the caller
needs are present before acting on any of them:

| Operation | Required from the captured first response |
|---|---|
| `start` | the drive identifier **and** the ownership generation |
| `handoff` | the **single-use** handoff token |
| `claim` / `takeover` | the **fresh** owner generation |
| `prepare-scope` | the scope identifier and the **separated** parent and child capabilities |

Human-readable output can never substitute for the captured document: it names identity and
disposition only, deliberately omitting generations, tokens, and capabilities. Each token keeps
its existing meaning — nothing here widens handoff, claim, or takeover authorization, and the
parent capability from `prepare-scope` stays with the parent as before.

A missing, malformed, or incomplete required response is a **caller-contract failure**, not
permission to rerun `start` (or any sibling op) to recover credentials. The caller maps it to its
**existing** blocked/halt posture — a build-task worker returns `BLOCKED` with the
missing-response reason — and never invents credentials, infers them from a drive id, or mints a
new recovery path.
```

- [ ] **Step 2: Update `skills/docket-build-task/SKILL.md` (worker start + handoff).**

In the paragraph "**Every test execution this task runs … starts through the native gate driver.**": change the invocation to read ``the `gate.drive.start` operation with `--owner task --scope-id <id> --child-cap <token> --run-root <task-scratch-dir> --json -- <the test command>` `` and, directly after the sentence ending "…the run root is a scratch dir you pick and read from.", insert:

```markdown
Capture the drive id and owner generation from that first `--json` response and validate both are
present before any advance or handoff — human text omits the generation (the JSON-capture
requirement in `docket-build`'s `references/gate-caller-loop.md`). A response missing them is a
caller-contract failure: return `BLOCKED` with the missing-response reason; never rerun `start` to
recover credentials.
```

In the disposition sentence, change the `WAITING` arm to: ``…`WAITING` → **immediately** perform the `gate.drive.handoff` operation with `--drive-id <id> --owner-gen <gen> --json`, capture the single-use handoff token from its response, and return `WAITING` naming the drive id and that token.``

- [ ] **Step 3: Update `skills/docket-build/SKILL.md` (prepare-scope, claim, takeover, final gate start).**

Four edits, preserving each surrounding paragraph's meaning:

- *Dispatching a task*: extend the prepare-scope flags to ``…--worktree <worktree> --gate-context <dispatch-context> --json`` and, after "(the dispatch context arrived in *your* prompt from the gated parent — pass its value through).", add: "Capture the scope id and **both** capabilities from the `--json` response before dispatching (per the JSON-capture requirement in [`references/gate-caller-loop.md`](references/gate-caller-loop.md)); the parent capability stays in your notes."
- *Task-level WAITING and the continuation*: change to "run the `gate.drive.claim` operation with `--drive-id <id> --handoff-token <token> --json` on the named handoff, capture the **fresh** owner generation from its response, and drive the same drive through short `gate.drive.advance` operation calls…" (check `docket gate drive claim --help` at build time and use its real flag names verbatim; if claim takes positional/other flags, keep the existing wording and only append `--json` plus the capture sentence).
- *Exceptional branch*: change to "run the `gate.drive.takeover` operation with `--scope-id <id> --parent-cap <token> --json` — authorized by the return event you just observed, **never** a timer, heartbeat, or quiet log — capture the fresh owner generation from its response, then advance that same drive to a terminal disposition via `gate.drive.advance`."
- Final gate item 2: change to ``drive it through the native gate **driver**: the `gate.drive.start` operation with `--owner build --json` — capture the drive id and owner generation from that first response — then `gate.drive.advance` operation slices, exactly as *Gate execution posture* describes.``

- [ ] **Step 4: Update `skills/docket-implement-next/SKILL.md` (Step-6 re-mint, resume-path prose).**

- In the Step-6 build-evidence paragraph, change the invocation to ``the `gate.drive.start` operation with `--repo-dir <absolute-feature-worktree> --run-root <absolute-run-root> --owner build --json` (`--owner build` resolves the build-owned command; no suite argv) — capture the drive id and owner generation from that first JSON response (human text omits the generation; the JSON-capture requirement in `docket-build`'s `references/gate-caller-loop.md`) — then the `gate.drive.advance` operation with `--drive-id <id> --owner-gen <gen>`…``
- In the run-verification paragraph, change the closing sentence to: "…pass it into every `gate.drive.prepare-scope` / `gate.drive.start --gate-context` this run performs — each invoked with `--json` per the shared capture requirement — and into the Step-2 claim's --gate-context."

- [ ] **Step 5: Re-derive the site population and sweep for missed sites.**

Run the inventory grep from "Site inventory" above. Every hit in a maintained file that is an actionable flag-bearing invocation of `start`/`handoff`/`claim`/`takeover`/`prepare-scope` must now carry `` `--json` `` in its paragraph; fix any stragglers the same way. Prose mentions without flags (e.g. the outcome-list line in docket-build-task) need nothing.

- [ ] **Step 6: Regenerate the embedded copies and verify byte-identity.**

```bash
go run ./cmd/genassets -repo .
git status --short   # expect ONLY skills/* and internal/assets/embedded/tree/skills/* modified
go test ./internal/assets/... -count=1
```

Expected: assets tests PASS (the DiffTree byte-identity check is green).

- [ ] **Step 7: Verification (evidence-bound discretion — docs-only, no RED/GREEN available yet).**

This task is documentation-only; the failing-guard cycle lands in Task 2 (whose mutation probe re-proves this task's edits are what the guard detects). Substitute precise inspection: re-read each edited paragraph against Global Constraints — no new hard-coded `docket …` argv, no credential values promised in human output, WAITING/continuation policy sentences unchanged around the edits, parent-capability-stays-with-parent preserved.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-build/references/gate-caller-loop.md skills/docket-build-task/SKILL.md \
  skills/docket-build/SKILL.md skills/docket-implement-next/SKILL.md internal/assets/embedded/tree/skills
git commit -m "docs(0376): require --json capture at every credential-consuming gate.drive caller site"
```

---

### Task 2: Shape-keyed repoguard guard for the JSON-capture requirement, with mutation evidence

**Build profile:** standard (a guard whose vacuity would silently unprotect a live ownership protocol; mutation evidence is the acceptance criterion)

**Files:**
- Create: `internal/repoguard/gatedrive_json_capture_test.go` (package `repoguard`, alongside `gatedriver_test.go`)
- Test: itself — runs under `go test ./internal/repoguard/`, reaching the suite through the existing `go test ./...` wrapper (`tests/test_go_toolchain.sh`); **no new `tests/` shell file and no `runtime-budgets.tsv` row** (the shell-suite categories are closed per `tests/README.md`; maintained-tree prose guards are build-gate Go guards there by convention).

**Interfaces:**
- Consumes: Task 1's verbatim phrases and `--json` flags; existing same-package helpers `guardRoot(t)`, `maintainedPop(t, root)`, `readMaintained(t, root, rel)` (from `guards_test.go`/`gatedriver_test.go`) and the corpus classifier `isWorkflowMD(rel)` (from `gatedriver_test.go` — skills/, agents/, plus both embedded mirrors; frozen plans and changes are outside it by construction).
- Produces: `TestGateDriveJSONCapture` with a `non_vacuity` subtest.

- [ ] **Step 1: Write the guard.**

Create `internal/repoguard/gatedrive_json_capture_test.go`:

```go
package repoguard

// Change 0376: any maintained workflow instruction that invokes a
// credential-returning gate.drive operation with concrete flags must pass
// --json and capture the first response — a drive id gleaned from human text
// never authorizes advancement or handoff. Two prongs:
//   (1) the shared caller contract states the capture requirement and the
//       caller-contract-failure rule (phrase bound to claim, bounded gaps);
//   (2) every flag-bearing invocation paragraph of a credential-returning op
//       (start|handoff|claim|takeover|prepare-scope — advance returns no new
//       credentials) in the workflow-markdown corpus carries --json.
// Keyed on syntactic shape (op reference + flag token in one paragraph),
// never a per-file allowlist. Residual risk, recorded not hidden: an author
// who names an op without any --flag in the same paragraph is classified as
// prose mention, not invocation; the shared contract carries that case.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const sharedContractRel = "skills/docket-build/references/gate-caller-loop.md"

var (
	credOpRe   = regexp.MustCompile(`gate\.drive\.(start|handoff|claim|takeover|prepare-scope)`)
	flagRe     = regexp.MustCompile(`--[a-z][a-z-]+`)
	jsonFlagRe = regexp.MustCompile(`--json`)

	// Contract-clause asserts: one bounded gap each (two stacked gaps
	// backtrack catastrophically on non-matching input).
	reqMustJSON  = regexp.MustCompile(`(?i)MUST pass[^.]{0,40}--json`)
	reqCapture   = regexp.MustCompile(`(?i)--json[^.]{0,200}capture`)
	reqValidate  = regexp.MustCompile(`(?i)validate[^.]{0,120}before acting`)
	reqFailure   = regexp.MustCompile(`(?i)missing, malformed, or incomplete[^.]{0,160}caller-contract failure`)
	reqNoRerun   = regexp.MustCompile(`(?i)not[^.]{0,20}permission to rerun`)
	reqBlocked   = regexp.MustCompile("(?i)returns `BLOCKED` with the[^.]{0,60}reason")
	reqParentCap = regexp.MustCompile(`(?i)parent capability[^.]{0,80}stays with the parent`)
)

// paragraphs splits markdown into blank-line-delimited blocks with all runs
// of whitespace collapsed to single spaces, so wrapped prose matches
// single-line patterns (phrase-grep-over-wrapped-prose).
func paragraphs(content string) []string {
	var out []string
	for _, block := range regexp.MustCompile(`\n[ \t]*\n`).Split(content, -1) {
		out = append(out, strings.Join(strings.Fields(block), " "))
	}
	return out
}

// isJSONCaptureSite: a paragraph is a SITE when it references a
// credential-returning op AND carries at least one concrete --flag token; a
// site VIOLATES when no --json appears in the same paragraph.
func isJSONCaptureSite(p string) bool {
	return credOpRe.MatchString(p) && flagRe.MatchString(p)
}

func TestGateDriveJSONCapture(t *testing.T) {
	root := guardRoot(t)

	// Prong 1 — the shared contract carries the requirement, phrase bound to
	// claim. The embedded mirror is byte-identical by the assets DiffTree
	// check, so the source copy is the one asserted here.
	contract := strings.Join(strings.Fields(readMaintained(t, root, sharedContractRel)), " ")
	for name, re := range map[string]*regexp.Regexp{
		"must-pass-json":        reqMustJSON,
		"json-bound-to-capture": reqCapture,
		"validate-before-use":   reqValidate,
		"contract-failure":      reqFailure,
		"no-rerun-recovery":     reqNoRerun,
		"maps-to-blocked":       reqBlocked,
		"parent-cap-stays":      reqParentCap,
	} {
		if !re.MatchString(contract) {
			t.Errorf("shared contract %s lost its %s clause (pattern %v)", sharedContractRel, name, re)
		}
	}

	// Prong 2 — corpus scan. The shared contract file (and its embedded
	// mirror) is the requirement's definition, not a caller: excluded.
	var sites, violations []string
	perFile := map[string]int{}
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) || strings.HasSuffix(rel, "docket-build/references/gate-caller-loop.md") {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if !isJSONCaptureSite(p) {
				continue
			}
			sites = append(sites, rel)
			perFile[rel]++
			if !jsonFlagRe.MatchString(p) {
				violations = append(violations, fmt.Sprintf("%s: credential gate.drive invocation without --json: %.160s", rel, p))
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative): the
	// three caller skills each contribute, and the corpus (source + embedded
	// mirrors) stays above a global floor.
	if len(sites) < 8 {
		t.Fatalf("population floor: only %d credential gate.drive invocation sites discovered (expected >= 8)", len(sites))
	}
	for _, rel := range []string{
		"skills/docket-build-task/SKILL.md",
		"skills/docket-build/SKILL.md",
		"skills/docket-implement-next/SKILL.md",
	} {
		if perFile[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no invocation site (scan or corpus drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("gate.drive JSON-capture violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		bad := "run the `gate.drive.start` operation with `--owner task --scope-id <id>` and read the drive id"
		good := bad + " from the `--json` response"
		mention := "performed an explicit `gate.drive.claim` operation on the named handoff"
		advOnly := "the `gate.drive.advance` operation with `--drive-id <id> --owner-gen <gen>`"
		if !isJSONCaptureSite(bad) || jsonFlagRe.MatchString(bad) {
			t.Errorf("a flag-bearing credential invocation without --json was not classified as a violating site")
		}
		if !isJSONCaptureSite(good) || !jsonFlagRe.MatchString(good) {
			t.Errorf("a --json-carrying invocation was not classified as a compliant site")
		}
		if isJSONCaptureSite(mention) {
			t.Errorf("a flagless prose mention was wrongly classified as an invocation site")
		}
		if isJSONCaptureSite(advOnly) {
			t.Errorf("an advance-only paragraph was wrongly classified as a credential site")
		}
		wrapped := "run the `gate.drive.handoff` operation\nwith `--drive-id <id>\n--owner-gen <gen>`"
		if got := paragraphs(wrapped); len(got) != 1 || !isJSONCaptureSite(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped invocation was not one matchable paragraph")
		}
		if reqCapture.MatchString("the --json flag exists. capture is discussed elsewhere") {
			t.Errorf("bounded gap failed: --json and capture in separate sentences must not satisfy the clause bind")
		}
	})
}
```

Adjust the prong-1 patterns only if Task 1's landed prose genuinely differs — by tightening the prose to match the pattern, or the pattern to the prose, keeping every clause bound (never a bare "the phrase appears somewhere in the file").

- [ ] **Step 2: Run the guard — expect PASS against the Task-1 tree.**

```bash
go test ./internal/repoguard/ -run TestGateDriveJSONCapture -count=1 -v
```

Expected: PASS, and the `-v` output confirms both floors held (no `population floor`/`coverage floor` failure). If it fails, the failure names either a real straggler site (fix it in the skill source, regenerate embedded, re-run) or a pattern/prose mismatch (Step 1's adjustment rule).

- [ ] **Step 3: Mutation evidence (acceptance criterion 4) — the guard reddens when the requirement is removed.**

Task 1 is already committed, so `git checkout --` restores safely (never mutation-test uncommitted work). Three probes, each followed by restore + a re-run to green:

```bash
# (a) a caller site loses --json
perl -0pi -e 's/--run-root <task-scratch-dir> --json/--run-root <task-scratch-dir>/' skills/docket-build-task/SKILL.md
go run ./cmd/genassets -repo .
go test ./internal/repoguard/ -run TestGateDriveJSONCapture -count=1   # expect FAIL: "...without --json"
git checkout -- skills/docket-build-task/SKILL.md internal/assets/embedded/tree/skills

# (b) the shared contract loses its requirement clause
perl -0pi -e 's/MUST pass `--json`/should use JSON/' skills/docket-build/references/gate-caller-loop.md
go test ./internal/repoguard/ -run TestGateDriveJSONCapture -count=1   # expect FAIL: "must-pass-json clause"
git checkout -- skills/docket-build/references/gate-caller-loop.md

# (c) the population goes vacuous (scan pointed at nothing): with the Edit
# tool, change credOpRe's pattern text `gate\.drive\.` to `gate\.drivex\.` in
# internal/repoguard/gatedrive_json_capture_test.go, then:
go test ./internal/repoguard/ -run TestGateDriveJSONCapture -count=1   # expect FAIL: "population floor"
# revert with a second Edit changing `gate\.drivex\.` back to `gate\.drive\.`

go test ./internal/repoguard/ -run TestGateDriveJSONCapture -count=1   # expect PASS again
```

(Probe (c)'s file is new and uncommitted at this point — restore it by reversing the edit, never by `git checkout`, which would delete-to-HEAD your uncommitted guard. The non_vacuity subtest also fails under probe (c) — its fixture asserts are the same population check in miniature; either failure line is acceptable evidence. If any probe stays green, that is a defect in the guard: fix the guard, do not proceed.) Record all three FAIL observations and the final PASS in the task return.

- [ ] **Step 4: Run the package and the assets check clean.**

```bash
gofmt -l internal/repoguard/            # expect no output
go vet ./internal/repoguard/
go test ./internal/repoguard/ -count=1
go test ./internal/assets/... -count=1
```

Expected: all PASS (this proves the new guard coexists with `TestGateDriverBoundary` and friends, and the tree is byte-identical again after the mutation probes).

- [ ] **Step 5: Commit**

```bash
git add internal/repoguard/gatedrive_json_capture_test.go
git commit -m "test(0376): repoguard guard — credential gate.drive invocations must capture --json"
```

---

## Final verification (controller-owned)

The build controller's final full-suite gate (`go run ./cmd/docket development test`, driven through the native gate driver per docket-build's own policy) certifies the branch; inspect budget clause lines per house rules. Acceptance criteria mapping: AC1/AC2 → Task 1 edits + Task 2 prong 2 with coverage floors; AC3 → Task 1 Step 1's contract-failure paragraph + the build-task `BLOCKED` sentence, asserted by `reqFailure`/`reqNoRerun`/`reqBlocked`; AC4 → mutation probes in Task 2 Step 3 plus the whole-suite gate; redaction/continuation preservation → Task 1 Step 7 inspection (no behavior code touched anywhere in this plan).
