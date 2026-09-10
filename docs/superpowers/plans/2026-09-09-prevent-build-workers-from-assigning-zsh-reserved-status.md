<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0420 — Prevent build workers from assigning zsh's read-only status parameter](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-10-0420-prevent-build-workers-from-assigning-zsh-s-read-only-status.md)**
<!-- docket:backlink:end -->
# Prevent Build Workers From Assigning zsh's Read-Only `status` Parameter — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Under docket, the docket-build role executes this plan task-by-task.

**Goal:** Make the maintained gate-call capture instructions shell-safe under zsh (named capture variables `gate_reply`/`gate_rc`, an explicit ban on zsh read-only special parameters), and add a mutation-proven repoguard test that keeps every maintained gate-call capture site free of reserved-parameter assignments.

**Architecture:** Two deliverables. (1) A new whole-repository repoguard test (`internal/repoguard/`) with two prongs modeled on the existing `gatedrive_json_capture_test.go`: a contract prong asserting the shared caller contract carries the shell-safe capture clauses, and a corpus prong scanning the workflow-markdown population (source `skills/`, `agents/`, and their embedded mirrors under `internal/assets/embedded/tree/`) for reserved-parameter assignment *shapes*, plus population floors so deleting the instruction reddens. (2) Prose edits to the four maintained instruction files that name the shell-safe variables at every gate capture site, followed by embedded-asset regeneration through the existing generator so source and distributed surfaces stay byte-aligned.

**Tech Stack:** Go test (RE2 regexps) in `internal/repoguard`; markdown skill bodies; `cmd/genassets` via `go generate ./internal/assets`.

**Spec:** none (change is `trivial: true`). The requirements document is the change file itself: `docs/changes/active/0420-prevent-build-workers-from-assigning-zsh-s-read-only-status.md` on the docket metadata branch — its Why / What changes / Acceptance criteria / Out of scope sections. Acceptance criteria, verbatim:
1. A build worker following the maintained gate-start capture instructions can run under zsh without assigning a read-only special parameter and can parse the original response's drive id and owner generation.
2. The instructions provide explicit shell-safe response and exit-code variable names and preserve the rule that a missing or malformed first response halts rather than rerunning `gate.drive.start`.
3. A whole-repository guard covers every maintained executable gate-call capture instruction; inserting `status=$?` at a covered site makes the guard fail for the intended reason.
4. Source and generated skill surfaces remain byte-aligned, and the configured whole suite passes at the build gate.

## Global Constraints

- Build gate command (controller-run after all tasks): `go run ./cmd/docket development test` — the whole suite, never only the tests named here.
- Never hand-copy between `skills/` and `internal/assets/embedded/tree/skills/`. The only sanctioned channel is the generator: `go generate ./internal/assets` (which runs `go run ../../cmd/genassets -repo ../..`); verify with `go run ./cmd/genassets -repo . -check` (exit 0 = byte-aligned).
- Every mutation probe and manual re-verification defeats the Go test cache: `go test -count=1 …` (cached-runner-serves-a-mutated-tree).
- Out of scope (from the change file): gate-driver ownership/identity/continuation/retry semantics; idempotent gate starts; weakening fail-closed validation; linting immutable archives, accepted ADRs, historical plans, or unrelated shell code (`docs/` and all `testdata` are already excluded by the repoguard population); resuming/implementing change 0323.
- Preserve, byte-compatible with their existing guards, the clauses other repoguard tests pin in the files being edited — especially `internal/repoguard/gatedrive_json_capture_test.go` prong 1 (regexes `reqMustJSON`…`reqParentCap` over `skills/docket-build/references/gate-caller-loop.md`) and its prong-2 rule that every flag-bearing credential-op paragraph carries `--json`. The edits below are additive sentences; do not reword existing sentences.
- Maintained prose that must NAME a reserved parameter writes it backticked without a trailing `=` (e.g. `` `status` ``, never the literal assignment `status` immediately followed by `=`) — otherwise the new guard flags its own instruction text.
- Cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054). Line numbers in this plan locate edits for the implementer only; do not write them into source.

## File Structure

- Create: `internal/repoguard/gatecapture_reserved_param_test.go` — the new guard (same package as the helpers it reuses: `guardRoot`, `maintainedPop`, `readMaintained` from `guards_test.go`; `isWorkflowMD` from `gatedriver_test.go`; `paragraphs` and `sharedContractRel` from `gatedrive_json_capture_test.go`).
- Modify: `skills/docket-build/references/gate-caller-loop.md` — shared contract gains one "Shell-safe capture names" paragraph in the "JSON capture" section.
- Modify: `skills/docket-build-task/SKILL.md` — the per-task gate-start capture paragraph gains the shell-safe naming sentence.
- Modify: `skills/docket-build/SKILL.md` — the build-gate clause-2 capture parenthetical gains the shell-safe naming.
- Modify: `skills/docket-implement-next/SKILL.md` — the evidence re-mint capture paragraph gains the shell-safe naming.
- Regenerate (never hand-edit): `internal/assets/embedded/tree/skills/…` mirrors of the four files plus `internal/assets/embedded/manifest.json`, via the generator.

---

### Task 1: Reserved-parameter guard + shell-safe capture instructions

**Files:**
- Create: `internal/repoguard/gatecapture_reserved_param_test.go`
- Modify: `skills/docket-build/references/gate-caller-loop.md` (section "## JSON capture — the required transport for consumed results")
- Modify: `skills/docket-build-task/SKILL.md` (the paragraph beginning "**Every test execution this task runs**", currently around lines 62–74)
- Modify: `skills/docket-build/SKILL.md` (numbered clause 2 under the build-gate section, currently around lines 235–239)
- Modify: `skills/docket-implement-next/SKILL.md` (the "**Validate the build evidence (change 0170).**" paragraph, currently around line 93)

**Interfaces:**
- Consumes: package-level helpers already defined in `internal/repoguard`: `guardRoot(t)`, `maintainedPop(t, root)`, `readMaintained(t, root, rel)`, `isWorkflowMD(rel)`, `paragraphs(content)`, and the const `sharedContractRel = "skills/docket-build/references/gate-caller-loop.md"`. Do not redefine any of them.
- Produces: `TestGateCaptureReservedShellParams` (guard consumed by the suite); the contract vocabulary `gate_reply` / `gate_rc` that Task 2's regenerated mirrors and all future skill edits must keep.

- [ ] **Step 1: Write the failing guard test**

Create `internal/repoguard/gatecapture_reserved_param_test.go` with exactly this content:

```go
package repoguard

// Change 0420: a resumed build worker executed `status=$?` after capturing a
// gate.drive.start JSON response; in zsh `status` is a read-only special
// parameter, so the shell aborted before the drive id and owner generation
// were parsed. Two prongs:
//   (1) the shared caller contract (gate-caller-loop.md) names the shell-safe
//       capture variables (`gate_reply` for the response, `gate_rc` for the
//       exit code), requires them to work in both zsh and bash, and forbids
//       assigning zsh read-only special parameters — phrase bound to claim,
//       one bounded gap per assert (stacked gaps backtrack catastrophically);
//   (2) no paragraph of the workflow-markdown corpus (source skills/ and
//       agents/ plus their embedded mirrors) carries a reserved-parameter
//       ASSIGNMENT shape, and the shell-safe names actually appear at the
//       gate-capture sites (population floors, so deleting the instruction
//       or displacing the scan reddens rather than passing vacuously).
// Keyed on syntactic shape — a reserved name immediately followed by `=`,
// bounded on the left — never on an enumerated list of RHS spellings:
// status=$?, status="$?", and status=$(...) all match because the RHS is
// unconstrained.
// LIMITATION (byte-pattern-guard-matches-a-spelling): the reserved-name set
// is the zsh read-only special parameters a capture site plausibly reaches
// for (status, pipestatus, signals, ARGC), derived from zshparam(1), not
// from repo spellings; a TOML-style `status = "x"` with spaces around `=` is
// out of shape and out of scope. Maintained prose that must NAME a reserved
// parameter writes it without a trailing `=` (backticked `status`), which is
// also what keeps this guard's own contract clauses out of its violation set.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var (
	// An assignment to a zsh read-only special parameter: the name abutting
	// `=`, left-bounded so `--status=`, `pipestatus` inside a longer word,
	// and `gate_rc=` never match. `-` in the class exempts flag spellings;
	// `$` exempts dereferences.
	reservedAssignRe = regexp.MustCompile(`(^|[^[:alnum:]_$-])(status|pipestatus|signals|ARGC)=`)

	// A gate-capture site mention: any gate.drive operation reference.
	gateOpMentionRe = regexp.MustCompile(`gate\.drive\.(start|advance|handoff|claim|takeover|prepare-scope)`)

	// The shell-safe capture vocabulary the contract mints.
	shellSafeNameRe = regexp.MustCompile(`gate_reply|gate_rc`)

	// Contract-clause asserts: one bounded gap each.
	reqReplyName   = regexp.MustCompile(`(?i)capture[^.]{0,120}gate_reply`)
	reqRcName      = regexp.MustCompile(`(?i)exit code[^.]{0,80}gate_rc`)
	reqBothShells  = regexp.MustCompile(`(?i)zsh and bash`)
	reqNeverAssign = regexp.MustCompile(`(?i)never assign[^.]{0,60}read-only special parameter`)
	reqAbortReason = regexp.MustCompile(`(?i)read-only special parameter[^.]{0,160}aborts`)
)

func TestGateCaptureReservedShellParams(t *testing.T) {
	root := guardRoot(t)

	// Prong 1 — the shared contract carries the shell-safe capture clauses,
	// phrase bound to claim. The embedded mirror is byte-identical by the
	// assets drift check, so the source copy is the one asserted here.
	contract := strings.Join(strings.Fields(readMaintained(t, root, sharedContractRel)), " ")
	for name, re := range map[string]*regexp.Regexp{
		"reply-capture-name":  reqReplyName,
		"rc-capture-name":     reqRcName,
		"works-in-both":       reqBothShells,
		"never-assign":        reqNeverAssign,
		"abort-consequence":   reqAbortReason,
	} {
		if !re.MatchString(contract) {
			t.Errorf("shared contract %s lost its %s clause (pattern %v)", sharedContractRel, name, re)
		}
	}

	// Prong 2 — corpus scan over the workflow markdown population (source and
	// embedded mirrors both; docs/ and testdata are excluded categorically by
	// MaintainedFiles). Violations and floors are collected in one pass.
	var violations []string
	sites := 0
	perFile := map[string]int{}
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if reservedAssignRe.MatchString(p) {
				violations = append(violations, fmt.Sprintf("%s: assignment to a zsh read-only special parameter: %.160s", rel, p))
			}
			if gateOpMentionRe.MatchString(p) && shellSafeNameRe.MatchString(p) {
				sites++
				perFile[rel]++
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative): each
	// caller skill carries the shell-safe names at a gate.drive paragraph,
	// and the corpus (source + embedded mirrors) stays above a global floor.
	if sites < 6 {
		t.Fatalf("population floor: only %d gate.drive paragraphs carry the shell-safe capture names (expected >= 6: three caller skills plus embedded mirrors)", sites)
	}
	for _, rel := range []string{
		"skills/docket-build-task/SKILL.md",
		"skills/docket-build/SKILL.md",
		"skills/docket-implement-next/SKILL.md",
	} {
		if perFile[rel] == 0 {
			t.Errorf("coverage floor: %s names no shell-safe capture variable at a gate.drive site (instruction deleted, or scan drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("reserved zsh parameter assignments (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		if !reservedAssignRe.MatchString("then run status=$? to record the exit code") {
			t.Errorf("a space-preceded status=$? assignment was not classified as a violation")
		}
		if !reservedAssignRe.MatchString(`reply="$(op --json)"; status=$?`) {
			t.Errorf("a semicolon-preceded status=$? assignment was not classified as a violation")
		}
		if !reservedAssignRe.MatchString("pipestatus=(0 1)") {
			t.Errorf("a line-leading pipestatus= assignment was not classified as a violation")
		}
		if !reservedAssignRe.MatchString(`status="$?"`) {
			t.Errorf("a quoted-RHS status= assignment was not classified as a violation (shape must be RHS-independent)")
		}
		if reservedAssignRe.MatchString("gate_rc=$? captures the exit code") {
			t.Errorf("the shell-safe gate_rc= assignment was wrongly flagged")
		}
		if reservedAssignRe.MatchString("--status=green is a flag, not an assignment") {
			t.Errorf("a --status= flag spelling was wrongly flagged")
		}
		if reservedAssignRe.MatchString("read the exit status of the command") {
			t.Errorf("prose mentioning the words exit status was wrongly flagged")
		}
		if reservedAssignRe.MatchString(`never assign `+"`status`"+` or `+"`pipestatus`") {
			t.Errorf("a backticked parameter NAME without `=` was wrongly flagged — the contract prose itself must stay legal")
		}
		if reqRcName.MatchString("the exit code matters. gate_rc is discussed elsewhere") {
			t.Errorf("bounded gap failed: exit code and gate_rc in separate sentences must not satisfy the clause bind")
		}
	})
}
```

- [ ] **Step 2: Run the new guard to verify it fails for the right reasons**

Run: `go test ./internal/repoguard/ -run TestGateCaptureReservedShellParams -count=1 -v`

Expected: FAIL. Exactly these failures — five `lost its … clause` errors (prong 1: the contract does not yet carry the clauses) and the `population floor: only 0 gate.drive paragraphs carry the shell-safe capture names` fatal. Expected NOT to appear: any `reserved zsh parameter assignments` violation (the corpus is clean today — a pre-existing violation is a finding to stop and report, not to silently fix) and any `non_vacuity` failure (a non_vacuity failure means the regexes are wrong; fix the regex, not the sample).

- [ ] **Step 3: Add the shell-safe capture clause to the shared contract**

In `skills/docket-build/references/gate-caller-loop.md`, section `## JSON capture — the required transport for consumed results`, insert a new paragraph between the paragraph ending "…the parent capability from `prepare-scope` stays with the parent as before." and the paragraph beginning "A missing, malformed, or incomplete required response…":

```markdown
**Shell-safe capture names.** Capture the emitted document into an explicitly named,
non-reserved variable — `gate_reply` — and capture the invocation's exit code, when the
caller needs it, into `gate_rc` on the very next line. Both names behave identically in
zsh and bash. Never assign to a zsh read-only special parameter — `status` and
`pipestatus` are the two a capture site reaches for — because under zsh that assignment
aborts the caller before the captured response is parsed, stranding a live drive.
```

Do not touch any existing sentence in the section: `gatedrive_json_capture_test.go` prong 1 pins them.

- [ ] **Step 4: Name the shell-safe variables at the three caller capture sites**

Edit A — `skills/docket-build-task/SKILL.md`. In the paragraph beginning `**Every test execution this task runs`, after the sentence ending "…(the shared JSON-capture requirement in `docket-build`'s `references/gate-caller-loop.md`; human text omits the generation)." and before "A response missing them is a caller-contract failure", insert:

```markdown
Capture the response into `gate_reply` and its exit code into `gate_rc` — the shared
contract's shell-safe names, valid in both zsh and bash; never assign a zsh read-only
special parameter such as `status` or `pipestatus`.
```

Edit B — `skills/docket-build/SKILL.md`. In numbered clause 2 of the build-gate section (`**build_gate: local`, non-empty `build_test_command`**`), replace the fragment:

```
— capture the drive id and
   owner generation from that first response —
```

with:

```
— capture that first response into `gate_reply` (its exit
   code, if needed, into `gate_rc`; never a zsh read-only special parameter such as
   `status`) and read the drive id and owner generation from it —
```

Edit C — `skills/docket-implement-next/SKILL.md`. In the `**Validate the build evidence (change 0170).**` paragraph, replace the fragment:

```
— capture the drive id and owner generation from that first JSON response (the shared JSON-capture requirement; human text omits the generation) —
```

with:

```
— capture that first JSON response into `gate_reply` and the drive id and owner generation from it, its exit code (if needed) into `gate_rc`, never a zsh read-only special parameter such as `status` (the shared JSON-capture requirement; human text omits the generation) —
```

All three edits keep `--json` present in the same paragraph (the existing JSON-capture guard requires it) and name reserved parameters only backticked, never followed by `=`.

- [ ] **Step 5: Run the new guard to verify it passes**

Run: `go test ./internal/repoguard/ -run TestGateCaptureReservedShellParams -count=1 -v`

Expected: PASS, including `non_vacuity`. If the `sites < 6` floor still fails, the embedded mirrors have not been regenerated yet — the three source-skill paragraphs plus the source contract satisfy per-file floors, and the global floor of 6 is reached only after Task 2's regeneration IF fewer than 6 source paragraphs match. Count: source contributes 3 caller-skill paragraphs + the agents/ wrappers contribute none — so if this run reports sites=3–5 and only the global floor fails, lower expectations are wrong: proceed to run the full check again after Task 2 regeneration and treat THIS step as passed when the only remaining failure is the global floor. If any prong-1 clause still fails, fix the prose to match the regex (bounded gap: keep each bound phrase inside one sentence).

- [ ] **Step 6: Run the whole repoguard package to verify no sibling guard regressed**

Run: `go test ./internal/repoguard/ -count=1`

Expected: PASS for every test except (possibly) an assets/embedded drift or byte-alignment failure and the new test's global sites floor, both resolved by Task 2's regeneration. Specifically `TestGateDriveJSONCapture`, the prose-contract tests, budget tests, and skill-handoff tests must all be green — a failure there means an edit reworded a pinned clause; restore the pinned wording and re-apply the insertion additively.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/prevent-build-workers-from-assigning-zsh-s-read-only-status
git add internal/repoguard/gatecapture_reserved_param_test.go \
  skills/docket-build/references/gate-caller-loop.md \
  skills/docket-build-task/SKILL.md \
  skills/docket-build/SKILL.md \
  skills/docket-implement-next/SKILL.md
git commit -m "fix(0420): shell-safe gate capture names + reserved-parameter repoguard"
```

- [ ] **Step 8: Mutation-test the guard (after the commit, so restore is safe)**

`git checkout --` restores to HEAD; running probes before committing would destroy the work (mutation-restore-needs-a-backup-copy). Three probes, each: mutate → run `go test ./internal/repoguard/ -run TestGateCaptureReservedShellParams -count=1` → confirm RED for the named reason → restore → confirm GREEN.

Probe 1 (AC 3's literal mutation): append the line `status=$?` inside the gate-start paragraph of `skills/docket-build-task/SKILL.md` (same blank-line-delimited block as the `gate.drive.start` invocation). Expected RED: `reserved zsh parameter assignments` naming `skills/docket-build-task/SKILL.md`. Restore: `git checkout -- skills/docket-build-task/SKILL.md`.

Probe 2 (clause removal): delete the entire `**Shell-safe capture names.**` paragraph from `skills/docket-build/references/gate-caller-loop.md`. Expected RED: all five `lost its … clause` errors. Restore: `git checkout -- skills/docket-build/references/gate-caller-loop.md`.

Probe 3 (instruction removal at a caller): delete the inserted `Capture the response into `gate_reply`…` sentence from `skills/docket-build-task/SKILL.md`. Expected RED: `coverage floor: skills/docket-build-task/SKILL.md names no shell-safe capture variable`. Restore: `git checkout -- skills/docket-build-task/SKILL.md`.

After all probes: `git status --porcelain` must be empty and a final `go test ./internal/repoguard/ -run TestGateCaptureReservedShellParams -count=1` must PASS (modulo the global sites floor case noted in Step 5). A probe that stays green is a defect in the guard — stop and fix the guard, never proceed.

---

### Task 2: Regenerate the embedded mirror and prove byte alignment

**Files:**
- Regenerate: `internal/assets/embedded/tree/skills/docket-build/references/gate-caller-loop.md`, `internal/assets/embedded/tree/skills/docket-build-task/SKILL.md`, `internal/assets/embedded/tree/skills/docket-build/SKILL.md`, `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md`, and `internal/assets/embedded/manifest.json` — via the generator only, never by hand.

**Interfaces:**
- Consumes: Task 1's committed source-skill edits; the generator `cmd/genassets` wired as `//go:generate go run ../../cmd/genassets -repo ../..` in `internal/assets/generate.go`.
- Produces: a byte-aligned embedded bundle; `TestGateCaptureReservedShellParams`'s global sites floor (>= 6) satisfied by the mirrored paragraphs.

- [ ] **Step 1: Verify the drift check currently reports the four edited files**

Run: `go run ./cmd/genassets -repo . -check`

Expected: non-zero exit listing (at least) the four edited skill paths — proof the mirror is stale for exactly the files Task 1 touched. If it lists nothing, Task 1's edits did not land; stop.

- [ ] **Step 2: Regenerate**

Run: `go generate ./internal/assets`

Expected: exit 0. Then `git status --porcelain` shows changes only under `internal/assets/embedded/` — any change elsewhere is a generator misfire; stop and investigate.

- [ ] **Step 3: Verify byte alignment and the guard's full population**

Run: `go run ./cmd/genassets -repo . -check && go test ./internal/assets/ ./internal/repoguard/ -count=1`

Expected: `-check` exits 0 (byte-aligned, AC 4); both packages PASS, and `TestGateCaptureReservedShellParams` now sees >= 6 shell-safe gate.drive paragraphs (3 source + 3 embedded mirrors).

- [ ] **Step 4: Mutation-test that the embedded mirror is inside the guard population**

The scan must redden for a violation planted in the DISTRIBUTED copy, not only the source (correspondence-guard-runs-one-way). Append `status=$?` inside the gate-start paragraph of `internal/assets/embedded/tree/skills/docket-build-task/SKILL.md`. Run `go test ./internal/repoguard/ -run TestGateCaptureReservedShellParams -count=1`. Expected RED naming `internal/assets/embedded/tree/skills/docket-build-task/SKILL.md`. Restore with `git checkout -- internal/assets/embedded/tree/skills/docket-build-task/SKILL.md` (safe only if Step 5's commit has not happened yet — this probe runs on the uncommitted regenerated file, so restore instead with the generator: re-run `go generate ./internal/assets` and confirm `go run ./cmd/genassets -repo . -check` exits 0). Then re-run the guard: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/prevent-build-workers-from-assigning-zsh-s-read-only-status
git add internal/assets/embedded
git commit -m "chore(0420): regenerate embedded skill mirror (shell-safe gate capture)"
```

- [ ] **Step 6: Final verification against the acceptance criteria**

- AC 1–2: read the regenerated `internal/assets/embedded/tree/skills/docket-build/references/gate-caller-loop.md` JSON-capture section — it names `gate_reply`/`gate_rc`, says both work in zsh and bash, forbids read-only special parameters, and the untouched next paragraph still maps a missing/malformed first response to a caller-contract failure, never a `gate.drive.start` rerun.
- AC 3: Task 1 Step 8 Probe 1 and Task 2 Step 4 receipts.
- AC 4: Step 3's `-check` exit 0. The whole suite (`go run ./cmd/docket development test`) is the controller's single post-task gate — do not skip it, and read its budget report lines even on green.

---

## Self-Review (performed at plan time)

- Requirement coverage: change-file "What changes" sentence 1 → Task 1 Steps 3–4; sentence 2 (guard + mutation) → Task 1 Steps 1–2, 8 and Task 2 Step 4; sentence 3 (regeneration/byte-alignment) → Task 2. All four acceptance criteria mapped in Task 2 Step 6.
- No placeholders: the guard test is given in full; every prose insertion is given verbatim; every command is concrete.
- Consistency: variable names `gate_reply`/`gate_rc`, test name `TestGateCaptureReservedShellParams`, and file path `internal/repoguard/gatecapture_reserved_param_test.go` are identical across tasks.
- Known judgment points left to the implementer: exact wrap points of inserted markdown (wrap freely — the guard collapses whitespace per phrase-grep-over-wrapped-prose), and adjusting a prong-1 regex only if a bound phrase cannot be kept inside one sentence (keep one gap per regex).
