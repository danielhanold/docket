<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0410 — Require durable results artifacts with human testing and coordinator findings](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0410-require-durable-results-artifacts-with-human-testing-and-coo.md)**
<!-- docket:backlink:end -->
# Require Durable Results Artifacts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the results artifact a required completion artifact for every implemented change (trivial included), adopt the human-approved five-section template, capture coordinator findings at defined checkpoints, and enforce all of it through the shared Go completion checks (`ChangeAttachResults`, `ChangeMarkImplemented`, `RunVerify`) plus the shared skill prose delivered to all four harnesses.

**Architecture:** One focused shared results-content validator (`internal/app/results_content.go`, reusing `internal/document`'s parse/blocks machinery) with an **explicit phase** (checkpoint vs final) is wired into the three existing Go seams. The workflow prose (results-template.md, docket-implement-next, docket-build, docket-review, docket-convention, user guidance) is updated in source, then the embedded harness bundle is regenerated through the tracked `go:generate` mechanism — never by hand-editing generated copies. Guards live in `internal/repoguard` (prose-contract rows) and Go unit/integration tests beside each seam.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), `internal/document` for markdown byte-span parsing, `internal/repoguard` phrase-table guards, `cmd/genassets` for the embedded asset bundle. Suite gate: `go run ./cmd/docket development test` (run from the feature worktree root; use `go test -count=1` for any mutation probe — the Go cache serves stale verdicts otherwise).

**Spec:** `docs/superpowers/specs/2026-09-07-require-durable-results-artifacts-with-human-testing-and-coo-design.md` (on the metadata branch, readable at `.docket/docs/superpowers/specs/...` from the primary tree; the change file is `.docket/docs/changes/active/0410-...md`). The spec's Acceptance criteria 1–15 and "Implementation surfaces" are the authoritative task source; every task below cites the criteria it discharges.

## Global Constraints

- Results file path convention is unchanged: `<results_dir>/<YYYY-MM-DD>-<slug>-results.md` (`results_dir` = `docs/results` in this repo). Date = first authoring date; the path is chosen once per change and reused on resume.
- The results **file** lives on the feature branch; the `results:` **field** lands only via the `change.attach-results` transaction on the metadata branch. Never cross the streams.
- Results prose is never gate evidence. Preserve ADR-0102's exact-head/observed-gate/resolved-command contracts, existing halted/waiting precedence in RunVerify, and all existing conjuncts in ChangeMarkImplemented.
- No new lifecycle state, no config opt-out, no mandatory follow-up item, no automatic change/issue/ADR/learning creation, no finalize-gate change, no post-merge results edits. Historical/terminal records are untouched.
- Never hand-edit anything under `internal/assets/embedded/` or a harness `testdata/golden` copy: regenerate through the tracked mechanism; obey any drift guard's own remedy message (new versioned fixture tree + re-derive) rather than weakening it.
- New refusal reasons are stable lowercase-hyphen strings, additive. Before finishing any task that adds one, grep for pinned reason vocabularies (`grep -rn "results-identity-broken" --include="*.go" .` plus docs) and update every pin in the same commit.
- Prose guards: bind phrase to claim, keep gaps bounded, prefer short unwrappable clauses; prove each new guard by mutation with `go test -count=1`.
- Commit per task with a conventional message carrying the change id, e.g. `feat(0410): ...`.

---

### Task 1: Shared results-content validator

**Files:**
- Create: `internal/app/results_content.go`
- Test: `internal/app/results_content_test.go`

**Interfaces:**
- Produces: `type ResultsPhase int` with `ResultsPhaseCheckpoint`, `ResultsPhaseFinal`; `type ResultsContentFinding struct { Reason, Message string }`; `func ValidateResultsContent(source []byte, phase ResultsPhase) []ResultsContentFinding` (empty slice = valid). Reason vocabulary (stable strings): `results-doc-malformed`, `results-title-missing`, `results-placeholder`, `results-outcome-missing`, `results-outcome-empty`, `results-empty-section`, `results-filler-section`.
- Consumes: `internal/document.Parse` (managed-block spans), nothing else new.

Semantics (from the spec's "Required completion checks" + Content rules; criteria 2, 3-in-part):

- Both phases: source must `document.Parse` cleanly (else `results-doc-malformed`); there must be an H1 (`# ` line with nonempty text) outside fenced code and outside managed `docket:*` blocks (else `results-title-missing`); no unfilled authoring placeholder (else `results-placeholder`). A placeholder is (a) any whole-word match, outside fences, of the exact token set already defined by `placeholderTokenRE` in `internal/app/change_attach.go` — reuse that same word alternation verbatim (share the existing var, or declare a results-local copy of its pattern; do not invent a different token list), or (b) a line whose text — after stripping leading `#` markers, list markers, and whitespace — begins with `<` and is not an HTML comment (`<!--`) or autolink (`<http`). This catches the template's `<What was delivered...>` scaffolding and `### <Finding>` headings.
- Final phase only: a `## Outcome` section must exist (`results-outcome-missing`) with substantive body text (`results-outcome-empty`); every present H2/H3 section must be substantive (`results-empty-section`): a section is empty when it has no body text of its own **and** no child subsection that is itself substantive (so `## Human testing` whose only content is a filled `### scenario` is fine); a section whose entire body, whitespace-stripped and lowercased, is one of `none`, `n/a`, `not applicable`, `no findings`, `nothing`, `-`, `—` (optional trailing `.`) is whole-section filler (`results-filler-section`). Prose that merely *mentions* "None"/"Not applicable" inside a longer body is legal.
- Heading recognition ignores lines inside fenced code blocks (track ``` and ~~~ fences, per the `section-slice-needs-a-named-terminator` learning) and inside managed-block spans from `document.Parse` (`doc.Blocks()` gives byte spans — skip any line whose start offset falls inside a block span). Do not enforce a closed heading roster: unknown extra H2s get the same substantiveness rules, nothing more.
- Checkpoint phase performs **only** the both-phases checks — a truthful in-progress artifact (title + backlink + real prose) attaches; the full content contract binds only at the implemented boundary. The phase parameter is explicit precisely so shared helper code cannot accidentally claim final completion (spec, "Required completion checks").

- [ ] **Step 1: Write the failing tests.** Table-driven over inline fixture strings (focused document fixtures, criterion 15). Cover at least: valid Outcome-only final artifact (accepted — criterion 2); missing Outcome (final refused, checkpoint accepted); empty `## Outcome` body; empty `### ` subsection under a filled parent (refused final); filled `###` under bodyless `## Human testing` parent (accepted); whole-section `None.` filler (refused final); "None of the flags are read at startup" inside a real paragraph (accepted — no false positive, criterion 2); raw template scaffold with `<angle-bracket>` lines (refused both phases as `results-placeholder`); `## Outcome`-looking heading inside a fenced example (ignored — the real Outcome elsewhere still validates); a document whose only H1 is inside a fence (`results-title-missing`); malformed managed markers (`results-doc-malformed`); backlink managed block present at top (its interior never counted as body text). Assert on exact `Reason` strings.

```go
func TestValidateResultsContentOutcomeOnlyFinal(t *testing.T) {
	src := []byte("<!-- docket:backlink:start (generated — do not hand-edit) -->\n> home\n<!-- docket:backlink:end -->\n\n# Change title — Results\n\n## Outcome\n\nDelivered the thing; behavior X now refuses Y.\n")
	if fs := ValidateResultsContent(src, ResultsPhaseFinal); len(fs) != 0 {
		t.Fatalf("want valid, got %v", fs)
	}
}

func TestValidateResultsContentFillerSectionRefusedFinal(t *testing.T) {
	src := []byte("# T — Results\n\n## Outcome\n\nReal outcome prose.\n\n## Findings and limitations\n\nNone.\n")
	fs := ValidateResultsContent(src, ResultsPhaseFinal)
	if len(fs) != 1 || fs[0].Reason != "results-filler-section" {
		t.Fatalf("want one results-filler-section, got %v", fs)
	}
}
```

(Write the full table in this style — one focused case per behavior above.)

- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/app/ -run TestValidateResultsContent` — expect compile failure (symbols undefined).
- [ ] **Step 3: Implement `internal/app/results_content.go`.** Single pass over source lines with byte offsets; classify each line as in-fence / in-managed-block / plain; collect headings (level, text, span) and per-section body presence; then apply the phase's rules. Keep it ~200 lines; it is a focused validator, not a second markdown framework (spec).
- [ ] **Step 4: Run to verify pass.** `go test -count=1 ./internal/app/ -run TestValidateResultsContent` — PASS.
- [ ] **Step 5: Mutation-prove two guards.** Temporarily blank the filler-word list and the fence-skip branch (`cp` the file aside first — never rely on `git checkout --` to restore uncommitted work); each mutation must redden at least one test under `-count=1`; restore from the copy.
- [ ] **Step 6: Commit.** `git add internal/app/results_content.go internal/app/results_content_test.go && git commit -m "feat(0410): shared results-content validator with explicit checkpoint/final phase"`

---

### Task 2: Checkpoint-phase validation in ChangeAttachResults

**Files:**
- Modify: `internal/app/change_attach.go` (after the `verifyBacklink` call, step (10)/(11) region; and the reason-const block)
- Test: `internal/app/change_attach_test.go` and/or `internal/app/change_attach_git_test.go` (follow whichever file already exercises results-kind attaches — grep `attachKindResults` in tests)

**Interfaces:**
- Consumes: `ValidateResultsContent(..., ResultsPhaseCheckpoint)` from Task 1.
- Produces: new reason const `ReasonAttachResultsContent = "results-content-invalid"` in the attach reason block; behavior later tasks rely on: a results attach refuses raw-template/malformed/titleless artifacts but accepts a truthful in-progress artifact.

- [ ] **Step 1: Write the failing test.** In the existing results-attach test harness (temp-repo integration style, criterion 15), attach a results file that is the unfilled template (angle-bracket scaffold): expect refusal with reason `results-content-invalid`. Add the positive control: an in-progress artifact with title + correct backlink + one real Outcome paragraph but **no** other sections attaches successfully (checkpoint phase must not demand the final contract).
- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/app/ -run <NewTestNames>` — the scaffold case currently attaches, so the refusal assert fails.
- [ ] **Step 3: Implement.** In `changeAttach`, after the backlink verification, add:

```go
	// (11b, results only) checkpoint-phase content sanity: a truthful in-progress
	// artifact passes; raw template scaffolding, a missing title, or a malformed
	// document refuses. The FINAL content contract binds at mark-implemented, not
	// here — the phase is explicit so checkpoint attachment can never claim it.
	if kind == attachKindResults {
		if fs := ValidateResultsContent(blob.Blob.Bytes, ResultsPhaseCheckpoint); len(fs) > 0 {
			return attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachResultsContent,
				fmt.Sprintf("the results artifact fails checkpoint content validation: %s: %s", fs[0].Reason, fs[0].Message))
		}
	}
```

Declare `ReasonAttachResultsContent = "results-content-invalid"` beside the other `ReasonAttach*` consts with a doc comment. Update the `ChangeAttachResults` doc comment: it is no longer "an optional authored results record" — say "an authored results record (required at completion since change 0410); checkpoint attaches validate checkpoint-phase content".
- [ ] **Step 4: Run to verify pass**, then the package: `go test -count=1 ./internal/app/`.
- [ ] **Step 5: Commit.** `git commit -m "feat(0410): checkpoint-phase content validation on change.attach-results"` (stage only the two files).

---

### Task 3: ChangeMarkImplemented refuses missing/invalid/unattached results

**Files:**
- Modify: `internal/app/change_implemented.go` (Conjunct 5 region ~line 287, `verifyImplementedResults` ~line 380, reason-const block ~line 110)
- Modify: `internal/app/change_attach.go` (extract the backlink-interior comparison from `verifyBacklink` into a shared helper)
- Test: `internal/app/change_implemented_test.go` (+ the integration file that drives mark-implemented against a temp repo — grep `ChangeMarkImplemented` in `*_test.go`)

**Interfaces:**
- Consumes: `ValidateResultsContent(..., ResultsPhaseFinal)`; the extracted helper.
- Produces: reason consts `ReasonImplementedResultsMissing = "results-missing"`, `ReasonImplementedResultsInvalid = "results-content-invalid"` (keep `ReasonImplementedResultsIdentity = "results-identity-broken"` unchanged); shared helper `func backlinkTargets(artifactBytes []byte, ch domain.Change, link render.LinkContext) (bool, error)` in `change_attach.go` — returns whether the artifact carries a balanced `docket:backlink` block whose interior equals the rendered backlink for `ch` (refactor `verifyBacklink` to call it; adapt to `attachChange`'s actual field types when extracting).

- [ ] **Step 1: Write the failing tests.** (a) A change with empty `results:` and every other conjunct green is refused with `results-missing` — including a `trivial: true` change fixture (criterion 1; there is no trivial exemption to remove, so this is a pin, not a change of a branch). (b) An attached results file whose content is final-invalid (e.g. filler `## Findings and limitations` body `None.`) is refused with `results-content-invalid`. (c) A results file whose backlink targets a different change id is refused with `results-content-invalid` (or keep `results-identity-broken` for the backlink case — pick ONE and assert it; recommendation: backlink mismatch → `results-identity-broken`, since it is identity, not prose). (d) The existing already-implemented replay test still returns no-op untouched (idempotent replay, criterion 14).
- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/app/ -run <names>`.
- [ ] **Step 3: Implement.** Replace the Conjunct-5 block:

```go
	// (Conjunct 5) a results artifact is REQUIRED at the implemented boundary
	// (change 0410) — for trivial changes too. The attached path must resolve to a
	// tracked regular file at the supplied head, carry this change's backlink, and
	// satisfy the FINAL results content contract.
	resultsPath := strings.TrimSpace(c.Results().Value)
	if resultsPath == "" {
		return implementedRefusal(ResultInvalidState, ReasonImplementedResultsMissing,
			fmt.Sprintf("change %04d has no attached results artifact; author and attach the final results before marking implemented", req.ID), req.ID)
	}
	if r := verifyImplementedResults(ctx, deps, repo, req.Head, resultsPath, c, linkContextOf(pin), req.ID); r != nil {
		return *r
	}
```

Extend `verifyImplementedResults` to keep its existing tracked-regular-file/symlink identity checks (reason `results-identity-broken`), then read the blob bytes and add: backlink verification via `backlinkTargets` (mismatch/absent → `results-identity-broken`), then `ValidateResultsContent(bytes, ResultsPhaseFinal)` (findings → `results-content-invalid`, message naming the first finding). Add the two reason consts with doc comments mirroring the neighbors.
- [ ] **Step 4: Run to verify pass**, then `go test -count=1 ./internal/app/`.
- [ ] **Step 5: Reason-pin sweep.** `grep -rn "results-identity-broken\|results-missing\|results-content-invalid" --include="*.go" . | grep -v _test` and `grep -rn` the same over `docs/reference` and `skills/` — update any vocabulary pin (capability catalog docs, CLI help) that enumerates mark-implemented reasons; spec says update public payload surfaces only if they actually change (a new refusal reason string in the findings list is additive — check `internal/repoguard/capability_surface_test.go` and `internal/app/capabilities_test.go` for enumerations).
- [ ] **Step 6: Commit.** `git commit -m "feat(0410): mark-implemented requires an attached, final-valid results artifact"`

---

### Task 4: RunVerify reports missing/invalid results as unmet conjuncts

**Files:**
- Modify: `internal/app/run_verify.go` (reason consts ~line 71–104; the plan/results blob block ~lines 335–365; `trackedRegularBlob` ~line 503)
- Test: `internal/app/run_verify_test.go`

**Interfaces:**
- Consumes: `ValidateResultsContent(..., ResultsPhaseFinal)`.
- Produces: `ReasonRunResultsUnlinked = "results-unlinked"`, `ReasonRunResultsInvalid = "results-content-invalid"` (keep `ReasonRunResultsIdentity` unchanged). Stable, specific reasons: missing linkage → `results-unlinked`; broken identity → `results-identity-broken`; invalid content → `results-content-invalid` (spec, "Required completion checks").

- [ ] **Step 1: Write the failing tests.** (a) An otherwise-complete run with empty `results:` yields `run-incomplete` with an unmet conjunct `results-unlinked` — it must NOT return `run-complete` on PR + green evidence alone (criterion 1). (b) A linked results path whose blob is final-invalid yields unmet `results-content-invalid` with the path as `Observed`. (c) Halted precedence unchanged: a `## Run halted` change with no results still returns `run-halted` (existing `TestRunVerifyHaltedVerdict` stays green — extend, don't weaken). (d) run-waiting precedence unchanged: the missing-results conjunct does not suppress a valid waiting receipt (it adds to `unmet`, and waiting evaluation only runs when `unmet` is nonempty — pin that a waiting fixture with missing results still reports `run-waiting`).
- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/app/ -run TestRunVerify`.
- [ ] **Step 3: Implement.** In the artifact block: change the results branch so `resultsPath == ""` does `add(ReasonRunResultsUnlinked, "")`; when present, keep the tracked-regular check (`results-identity-broken`) and, when the blob exists, validate content:

```go
		if resultsPath == "" {
			add(ReasonRunResultsUnlinked, "")
		} else {
			blob, okFile, rerr := trackedRegularBlobBytes(ctx, src, resultsPath)
			if rerr != nil {
				return runOperationalRefusal(ResultExternalFailed, ReasonRunArtifactRead, rerr.Error(), req.ID)
			}
			if !okFile {
				add(ReasonRunResultsIdentity, resultsPath)
			} else if fs := ValidateResultsContent(blob, ResultsPhaseFinal); len(fs) > 0 {
				add(ReasonRunResultsInvalid, resultsPath+": "+fs[0].Reason)
			}
		}
```

Note the structural consequence: the object source `src` is currently opened only when `planPath != "" || resultsPath != ""`; the missing-results conjunct needs no blob read, so hoist the `add(ReasonRunResultsUnlinked, "")` outside that guard. Add `trackedRegularBlobBytes` beside `trackedRegularBlob` (same probe, also returning the bytes) or extend the existing helper and update its plan-path caller.
- [ ] **Step 4: Run to verify pass**, then `go test -count=1 ./internal/app/`.
- [ ] **Step 5: Commit.** `git commit -m "feat(0410): run.verify reports missing/invalid results as unmet completion conjuncts"`

---

### Task 5: Replace results-template.md with the canonical template

**Files:**
- Modify: `skills/docket-implement-next/results-template.md` (full replacement)

The spec's "Authoring template" section is canonical. Replace the entire file with:

````markdown
<!-- results-template.md — REQUIRED close-out artifact for every implemented change (trivial
     included; change 0410). Authored and consolidated by the coordinator in the FEATURE worktree,
     committed on <type>/<slug> at each checkpoint and finally before the implemented transition.
     Angle-bracket instructions are authoring guidance only — remove them from actual artifacts.
     Omit an entire optional section, including its subsections, when there is no substantive
     content; Outcome is required at finalization. The generated docket:backlink block above the
     title is owned by the artifact.backlink operation — never hand-author its markers, and do not
     add empty header fields for unavailable links. Content rules and the checkpoint lifecycle are
     normative in docket-implement-next's Step 6.5. -->
# <Change title> — Results

## Outcome

<What was delivered and how the behavior changed.
Explain any material departures from the spec and why.>

## Human testing

### <Functional scenario not covered by automated tests>

<Prerequisites and setup needed for this scenario.>

1. <Human action.>
   Expected: <Observable behavior.>
2. <Human action.>
   Expected: <Observable behavior.>

<Cleanup instructions, only when needed.>

## Verification performed

<Concise account of checks the agents actually performed,
their outcomes, and links to supporting evidence.
Identify skipped, failed, or incomplete verification explicitly.
Do not reproduce test logs or individual automated test cases.>

## Findings and limitations

### <Finding>

<What was observed, supporting evidence, and practical impact.
Distinguish confirmed problems from suspected issues.
Explain any workaround or remaining limitation.>

## Follow-ups

### <Actionable follow-up>

<Problem or opportunity, supporting evidence or reproduction
details, why it falls outside this change, and suggested next action.
Link an existing change when available.>
````

- [ ] **Step 1: Replace the file** with exactly the content above.
- [ ] **Step 2: Verify** the old triggers are gone: `grep -c "OPTIONAL" skills/docket-implement-next/results-template.md` → 0; `grep -c "## Outcome" ...` → 1. Cross-check against the spec's template block (they must match section-for-section: Outcome, Human testing, Verification performed, Findings and limitations, Follow-ups).
- [ ] **Step 3: Sanity-check against Task 1's validator:** the raw template must FAIL checkpoint validation (placeholder scaffold) — add nothing; just confirm the template's `<...>` lines match the Task-1 placeholder rule by eye (Task 9 guards it in code).
- [ ] **Step 4: Commit.** `git commit -m "feat(0410): canonical five-section required results template"`

---

### Task 6: docket-implement-next — mandatory Step 6.5, checkpoint lifecycle, Step 7 postcondition, resume/edge paths

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 6.5 ~line 98–100; Step 7 mark-implemented paragraph ~line 110; postconditions table row 7 + note ~lines 118–131; final-report/dummy-mode prose ~line 154)
- Modify: `skills/docket-implement-next/references/edge-paths.md` (results-only/ancestor prose ~line 54; add resume/recovery rules)
- Modify: `skills/docket-implement-next/references/fix-loop.md` (findings-disposition → results checkpoint linkage)

This is a prose task; the worker must read each file fully first and reconcile surrounding sentences (consolidation flattens caller variance — diff intent, don't template blindly). Normative content to land, per spec sections "Checkpoint lifecycle", "Artifact location and ownership", "Test-evidence sequencing", "Persistence and recovery" (criteria 5–12):

- [ ] **Step 1: Rewrite Step 6.5** — retitle `### Step 6.5 — Results (required)`. Replace the optional trigger sentence entirely. New content must state, in the skill's existing register:
  - The results artifact is **required for every change, trivial included**; author `<results_dir>/<YYYY-MM-DD>-<slug>-results.md` from `results-template.md` in the FEATURE worktree; choose the path once, reuse it on resume even when the date changed; sections beyond Outcome are conditional — omit empty headings and filler (never `None`/`N/A` bodies).
  - Human testing contains only functional scenarios automated tests do not cover; never a suite rerun or a manual translation of an automated test; do not infer coverage from a green suite — inspect the relevant tests and evidence, and report coverage uncertainty under Findings/Verification instead of a speculative checklist.
  - **Checkpoint lifecycle**: capture coordinator-known findings (i) after the build role returns (prefer consolidating before the existing end-of-build gate so the gate tests the checkpoint-containing head), (ii) after review returns and again after fixes with actual dispositions, (iii) before a planned pause/halt/handoff when the ownership and gate-drive contracts permit a safe write, (iv) at final consolidation before the implemented transition. A checkpoint is an evaluated boundary: unchanged substantive content is a no-op — no empty commits, no per-observation commits, never commit or move HEAD beneath a live gate, running worker, or transferred drive.
  - **Per-checkpoint mechanics** (spec "Persistence and recovery", numbered): verify ownership/quiescence → read + preserve prior content → write the update, stamp/validate the backlink via the `artifact.backlink` operation → commit ONLY the results path (stage by explicit path) → publish through the existing feature-branch transport without force → on first creation attach via the `change.attach-results` operation with the fresh entity version; later updates keep the same path and revalidate/reattach before completion. Local commit, remote publication, and metadata attachment are distinct facts — a denied push is not remote durability; keep the local commit + diagnostic and follow the existing halt posture.
  - **Evidence sequencing**: results prose is never gate evidence; batch review/fix checkpoint edits before the final certification run; after final authored results are committed, obtain gate evidence for that exact head through the gate driver (a post-gate results edit invalidates stale evidence and can require another certification run — say so); never rewrite results solely to paste the final gate's SHA/timestamp; a new material finding from the final gate is preserved, fix/halt policy applies, evidence is re-established after any commit.
  - Follow-ups hold out-of-scope work for **human triage** — link an existing change when known, never mint a change/issue/ADR/learning automatically; an in-scope defect stays fix/block/halt work regardless of being written under Follow-ups. Final consolidation verifies no material coordinator handoff remains only in chat; the final report links the durable results (criterion 12).
  - Ownership: the coordinator authors and consolidates; a controller executing the invoked build role may perform the build checkpoint on the coordinator's behalf under the explicit caller contract; task workers and reviewers never independently edit the file; no concurrent writers.
- [ ] **Step 2: Update Step 7 + postconditions.** In the mark-implemented paragraph, replace "and any attached results path still satisfies its artifact identity" with the required-form: "a results artifact is attached and satisfies the final content contract and its artifact identity (missing, invalid, or unattached results — trivial changes included — refuse the transition)". In the postconditions table, change row 7's clause `` `results:` set **iff** a results file and its backlink stamp are committed on `<type>/<slug>` `` to an unconditional conjunct: `` `results:` set and the final results file with its backlink stamp committed on `<type>/<slug>` ``. Update the note at ~line 131: 6.5 is no longer "optional — its artifact rides in Step 7's `iff` conjunct"; reword to "6.5's artifact is certified by Step 7's results conjunct". Also update the field-write paragraph (~line 172) and workspace paragraph (~line 176) only where they still call results optional.
- [ ] **Step 3: edge-paths.md.** (a) Reconcile the ancestor/results-only passage at ~line 54 against the ACTUAL current finalize behavior: read `skills/docket-finalize-change/SKILL.md` and `internal/config`'s `finalize.skip_results_only_delta` handling first; keep whatever is true (the armed-by-config permit exists in config), correct whatever contradicts current finalize prose, and do not activate any deferred capability (spec, "Current behavior"; verify-the-claim learning). The "a step-6.5 results commit ... is EXPECTED" sentence stays, updated for mandatory results. (b) Add a **Resume** rule block: on resume, load the committed results before new work; reuse `results:` when present; recover a commit-before-attach interruption only when the owned branch holds exactly one safe, correctly backlinked candidate at the canonical location — otherwise surface the ambiguity; never overwrite newer remote work; a changed date never mints a second file (criterion 6). (c) A pre-workspace halt or unsafe-write pause keeps its existing disposition and reports the capture limitation through the existing halt/continuation channel — no fabricated path, no stolen lease, no delayed gate handoff (criterion 7).
- [ ] **Step 4: fix-loop.md.** One addition where the disposition table is defined: the coordinator persists returned review findings to the results artifact before a long fix loop when the workspace is safe, and updates their actual dispositions after fixes return; unresolved findings and relevant fix consequences are retained (criterion 5). Keep the PR body as the disposition table's durable home — the results file preserves findings, not the machine evidence block.
- [ ] **Step 5: Sweep for stragglers.** `grep -rn -i "results" skills/docket-implement-next/ | grep -i "optional\|skip\|only if\|otherwise skip"` — fix every active instruction that still says results are conditional. Then repo-wide per the spec: `grep -rn -i "skip.*results\|results.*optional" skills/ docs/reference docs/guide README.md` and fix active maintained callers only (leave point-in-time records: specs, archived changes, results files, ADRs).
- [ ] **Step 6: Commit.** `git commit -m "docs(0410): implement-next requires results — mandatory Step 6.5, checkpoint lifecycle, resume/edge paths"`

---

### Task 7: docket-build and docket-review boundaries

**Files:**
- Modify: `skills/docket-build/SKILL.md` (the results sentence at ~line 389 and the end-of-build gate section)
- Modify: `skills/docket-review/SKILL.md` (return contract)

- [ ] **Step 1: docket-build.** Read the SKILL fully. Update ~line 389 ("material TDD exceptions and residual risks flow into the PR description or the results ...") to the required-results world: material TDD exceptions, residual risks, and worker-surfaced findings flow to the **coordinator's results artifact** (and the PR description where evidence belongs). Add, at the end-of-build gate section: when docket-build runs as the invoked build role for a coordinator that owns a results artifact, the controller MAY perform the build-findings checkpoint on that coordinator's behalf under the explicit caller contract — consolidating available build findings before the final full-suite gate where possible so the gate certifies the checkpoint-containing head; task workers never edit the results file; no concurrent writers; a checkpoint never independently launches tests (spec "Build findings"; criteria 5, 9, 13 — preserve all four profiles and custom-binding neutrality: a custom build skill returns findings through its own contract and the coordinator captures at the next safe boundary it controls).
- [ ] **Step 2: docket-review.** Read fully. Add to the findings-return contract: findings return through the reviewer's existing report — the reviewer never writes the results artifact; returned findings should carry the evidence, impact, and any actionable out-of-scope framing the coordinator needs to preserve them durably (dispositions land via the fix loop and the coordinator's checkpoint). Keep the reviewer read-only.
- [ ] **Step 3: Commit.** `git commit -m "docs(0410): build/review capture ownership — controller checkpoint, single results writer"`

---

### Task 8: docket-convention and user guidance

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (~lines 163–164, 190, 218, 224, 321 — every "optional" results mention)
- Modify: user-facing guidance found by grep (`docs/guide/`, `docs/reference/`, `README.md` — active maintained docs only)

- [ ] **Step 1: Convention.** Line ~163: `optional close-out artifacts` → `required close-out artifacts (one per implemented change, trivial included; change 0410)`. Line ~190 (`results:` field comment): drop `(optional)`, state the field is set by `change.attach-results` at the first checkpoint and required before the implemented transition. Preserve VERBATIM the guarded sentence `Merged plans and results are frozen build records.` (repoguard prose row pins it) and the Closeout-notes paragraph (~224) unchanged in meaning — late findings still go to Closeout notes via the typed operation, merged results stay frozen (criterion 14). Add one compact normative paragraph near the results directory entry: template shape (Outcome required; Human testing / Verification performed / Findings and limitations / Follow-ups conditional — omit empty sections and filler), the file/field branch split, checkpoint capture during implementation, follow-ups held for human triage with no automatic creation.
- [ ] **Step 2: User guidance.** `grep -rln -i "results" docs/guide docs/reference README.md .docket.example.yml` — read each hit; update active guidance that describes results as optional (including `.docket.example.yml`'s results_dir comment if it does). Historical records untouched.
- [ ] **Step 3: Verify** the frozen sentence survived: `grep -Fq "Merged plans and results are frozen build records." skills/docket-convention/SKILL.md`.
- [ ] **Step 4: Commit.** `git commit -m "docs(0410): convention + user guidance — results required, conditional sections, frozen merged artifacts"`

---

### Task 9: Workflow guards (repoguard) for the new obligations

**Files:**
- Modify: `internal/repoguard/prose_contracts_test.go` (new rows)
- Possibly create: `internal/repoguard/results_contract_test.go` (if a template-shape guard doesn't fit the phrase table)

- [ ] **Step 1: Add prose-contract rows** (mutation-proven workflow guards for obligations expressed in prose, criterion 15). One row per (contract, file), phrases anchored on the exact post-Task-5..8 prose — pick short, distinctive, unwrappable clauses (phrase-grep-over-wrapped-prose learning; the table matches raw bytes, so avoid clauses likely to hard-wrap, or collapse whitespace in a dedicated check):
  - `skills/docket-implement-next/results-template.md`: present `## Outcome`, `## Human testing`, `## Verification performed`, `## Findings and limitations`, `## Follow-ups`; absent `OPTIONAL: write one only` and `## Verify (human)` (assert the state that was REMOVED, not just the new words — assert-detects-removal learning).
  - `skills/docket-implement-next/SKILL.md`: present the mandatory-results clause and a checkpoint clause you wrote in Task 6 (e.g. `Results (required)` and the never-commit-under-a-live-gate clause); absent `Results close-out (optional)`.
  - `skills/docket-convention/SKILL.md`: present the required-results phrasing from Task 8; keep the existing frozen-records row untouched.
  - `skills/docket-build/SKILL.md`: present the controller-checkpoint/single-writer clause.
- [ ] **Step 2: Add a template↔validator coupling guard.** New test (in `results_contract_test.go`, package repoguard or app): read `skills/docket-implement-next/results-template.md` from the repo root and assert `app.ValidateResultsContent(template, app.ResultsPhaseCheckpoint)` reports `results-placeholder` — proving the shipped scaffold can never attach as a real artifact and pinning validator/template correspondence (validator-must-match-the-reader learning). If a repoguard→app import is awkward (check existing imports; repoguard is a test-only package), place this test in `internal/app/results_content_test.go` reading the template via a repo-root-relative path helper already used by repoguard (`MaintainedFiles`-style root discovery — copy the root-finding idiom from `repoguard.go`).
- [ ] **Step 3: Run** `go test -count=1 ./internal/repoguard/ ./internal/app/` — PASS.
- [ ] **Step 4: Mutation-prove.** For each new row: temporarily revert the guarded sentence in the live skill file (keep a copy aside), run the guard test with `-count=1`, watch it redden, restore. Record in the commit message that rows were mutation-proven.
- [ ] **Step 5: Commit.** `git commit -m "test(0410): mutation-proven prose guards for required results and checkpoint obligations"`

---

### Task 10: Regenerate embedded assets; harness fixtures and registry-derived coverage

**Files:**
- Regenerate: `internal/assets/embedded/` (via `go generate ./internal/assets` → `cmd/genassets`)
- Possibly modify: harness `testdata` fixtures under `internal/harness/{claude,codex,cursor,opencode}/` and `internal/harness/cross_harness_test.go` / `inventory_test.go` — only as their own drift guards direct

- [ ] **Step 1: Regenerate.** `go generate ./internal/assets` (the tracked mechanism; never hand-edit `internal/assets/embedded/tree` or `manifest.json`). `git status` — the changed embedded copies of every file Tasks 5–8 touched appear.
- [ ] **Step 2: Run the asset + harness packages.** `go test -count=1 ./internal/assets/ ./internal/harness/...`. If any frozen-fixture drift guard reddens (harness golden testdata byte-copying a live skill), obey the guard's OWN remedy — a new versioned fixture tree copied from the prior one with only the changed files re-derived by running the generator and reading actual output, plus provenance — never an in-place fixture edit and never a weakened guard (config-edit-trips-its-own-frozen-drift-guard learning).
- [ ] **Step 3: Registry-derived coverage check.** Read `internal/harness/inventory.go` + `inventory_test.go` and `cross_harness_test.go`: confirm the tests that prove each registered harness ships the implement-next/build/review skills and results-template derive their harness list from the registry (criterion 13: Claude, Codex, Cursor, OpenCode). If the existing inventory tests already assert per-harness delivery of the `skills` root files, the regenerated bundle satisfies them — add an assertion only if no existing test ties `results-template.md`'s embedded copy to the source bytes (frozen-copy-needs-a-drift-assert learning: check whether `internal/assets` tests already do byte-equality between source tree and embedded tree; if they do, note it and add nothing).
- [ ] **Step 4: Commit.** `git add internal/assets/embedded <any fixture trees> && git commit -m "chore(0410): regenerate embedded harness assets for required-results workflow"` — stage by explicit path.

---

### Task 11: Full-suite gate and truthfulness sweep

**Files:** none new — verification only.

- [ ] **Step 1: Self-check spec coverage.** Walk Acceptance criteria 1–15 against the branch diff (`git diff <base>..HEAD --stat`); each must map to a landed task. Criteria that are behavioral-only in live runs (e.g. actual checkpoint publication against a real remote, live-harness delivery) cannot be exercised hermetically — they are recorded truthfully in this change's OWN results artifact at build close-out, not faked as tests (metadata-branch-invisible-to-suite learning; criterion 15's "report unexercised live-harness behavior truthfully").
- [ ] **Step 2: Run the full suite** from the feature worktree root: `go run ./cmd/docket development test`. Green required; treat `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines as screening findings and `SERIAL CONFIRMED OVER BUDGET:` as an authoritative breach to act on.
- [ ] **Step 3: Fix-forward** anything red (never weaken a guard to get green), re-run, and finish with the suite green at the branch head.

---

## Self-Review (performed at authoring)

- **Spec coverage:** Template → Task 5; content rules + final validator → Tasks 1, 3; checkpoint attach phase → Task 2; RunVerify conjuncts → Task 4; checkpoint lifecycle/persistence/recovery/evidence sequencing prose → Task 6; build/review ownership → Task 7; convention + user guidance + compatibility prose → Task 8; workflow guards → Task 9; harness regeneration + registry matrix → Task 10; full suite + truthful reporting → Task 11. Compatibility (criterion 14) is enforced by construction: only `ChangeMarkImplemented` fresh transitions and active `RunVerify` gained conjuncts; finalize, closeout, replay, and archives are untouched by every task.
- **Type consistency:** `ValidateResultsContent(source []byte, phase ResultsPhase) []ResultsContentFinding` is consumed with that exact signature in Tasks 2, 3, 4, 9; reason strings are declared once per seam and asserted by exact value in tests.
- **Known judgment points left to the builder, deliberately:** exact placement/wording inside prose files (workers read the files first; quoted anchors above pin the load-bearing clauses), the backlink-helper extraction's exact receiver types, and whether harness fixture trees need re-versioning (their own guards decide).
