<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0331 — docket-implement-next's re-mint path never names docket gate launch, so a resumed run cannot produce the run directory evidence record requires** — `docs/changes/active/0331-docket-implement-next-s-re-mint-path-never-names-docket-gate.md`
<!-- docket:backlink:end -->
# Step 6 Re-mint Gate-Launch Chain Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `docket-implement-next` Step 6's evidence re-mint path an executable launch → observe → record → verify chain that names `docket gate launch`, and prove that chain with a mutation-tested Bash contract guard.

**Architecture:** This is a skill-contract (Markdown) fix plus a Bash test extension plus a mechanical embedded-asset regeneration. No Go runtime or CLI change. The authored source of truth is `skills/docket-implement-next/SKILL.md`; `internal/assets/embedded/tree/...` is generated from it by `go generate ./internal/assets/` and is never hand-edited.

**Tech Stack:** Markdown skill contracts, Bash test suite (`tests/`, run via `scripts/run-tests.sh`), Go `go generate` asset pipeline.

**Spec:** `docs/superpowers/specs/2026-08-20-docket-implement-next-evidence-remint-gate-launch-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/...`).

## Global Constraints

- Word/line budget for `skills/docket-implement-next/SKILL.md` is **180 lines / 6150 words** (row in `tests/test_skill_size_budgets.sh`). The file is at 6089 words. **Do NOT raise the allowance** — tighten Step 6 prose to fit.
- The guard reads the **authored** `skills/docket-implement-next/SKILL.md`, never the embedded copy.
- No Go semantic test is added (spec §3: `go test` caching does not track `skills/` edits).
- One bounded gap per ERE pattern (learnings: stacked-gap-regex-hangs-instead-of-failing — two stacked `{0,N}` gaps hang on the mutated/non-matching input instead of reddening).
- Collapse whitespace before matching prose-spanning relationships (learnings: phrase-grep-over-wrapped-prose) — the file's existing `flatten(){ tr -s '[:space:]' ' '; }` idiom.
- `grep` patterns leading with `--` must be declared: `grep -qE -e "<pat>"` or `grep -qF -- "<pat>"` (repo AGENTS.md).
- Mutation probes work on a **temp copy** (`mktemp` with an explicit `"${TMPDIR:-/tmp}/....XXXXXX"` template — bare `mktemp` ignores TMPDIR on macOS), never on the real worktree, and never restore via `git checkout --` (learnings: mutation-restore-needs-a-backup-copy).
- Out of scope: any change to `docket gate` / `docket evidence` interfaces, to their passed-run/exact-head/durable-run-dir requirements, to `docket-build`'s observation policy, or to `docket change mark-implemented`.

---

### Task 1: Contract guard + Step 6 rewrite + embedded regeneration

One task on purpose: the guard is the failing test, the Step 6 rewrite makes it pass, and the regeneration keeps the asset-drift shard green — splitting them would commit a red intermediate state.

**Files:**
- Modify: `tests/test_docket_review.sh` (append a new section at the **end of the file**, after all existing sections — it must be after the `flatten()` definition, which sits mid-file)
- Modify: `skills/docket-implement-next/SKILL.md` (the two paragraphs of "### Step 6 — Review + ADRs" beginning `**Validate the build evidence (change 0170).**` and `**Create the durable evidence.**`)
- Regenerate: `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md` and the embedded manifest (via `go generate ./internal/assets/` — never by hand)

**Interfaces:**
- Consumes: existing helpers in `tests/test_docket_review.sh` — `assert(){ ... }` (line-1 region), `$IMPL` (the authored implement-next SKILL.md path), `flatten(){ tr -s '[:space:]' ' '; }`.
- Produces: `check_remint_chain <file>` (returns 0 iff the Step 6 chain holds in `<file>`; prints a short failure tag on stdout otherwise) and `remint_pos <haystack> <needle>` — both used only within this new section; Task 2/3 rely on the committed guard being green.

- [ ] **Step 1: Write the failing guard**

Append this section verbatim at the end of `tests/test_docket_review.sh` (before any final summary/exit lines if the file has them — check the tail; the file counts `fails` and the exit must remain last):

```bash
# --- change 0331: Step 6's re-mint path names its producer ---------------------------------
# The recovery path for missing/malformed/stale evidence must be an executable chain:
# `docket gate launch` produces the run dir that `docket gate observe` reports and
# `docket evidence record` consumes at the exact feature head, with `docket evidence verify`
# re-checking the same head. Guarded on command SHAPE and ORDERING, whitespace-collapsed so a
# Markdown re-flow is not a semantic failure, one bounded gap per ERE (two stacked gaps
# backtrack catastrophically on exactly the mutated input). The checker takes the file as an
# argument so the same code runs against the authored skill and the mutated copy.

# Position of a fixed literal in a haystack (fails when absent) — pure bash, no regex.
remint_pos(){ local pre="${1%%"$2"*}"; [ "$pre" != "$1" ] || return 1; printf '%s\n' "${#pre}"; }

check_remint_chain(){
  local file="$1" sec flat p_launch p_observe p_record p_verify
  sec="$(awk '/^### Step 6 — Review/{f=1;next} /^### Step 6\.5 — Results close-out/{f=0} f' "$file")"
  [ -n "$sec" ] || { echo "step6-slice-empty"; return 1; }
  flat="$(tr -s '[:space:]' ' ' <<<"$sec")"
  # (a) the producer exists on the re-mint path
  grep -qF -- "docket gate launch" <<<"$flat" || { echo "launch-missing"; return 1; }
  # (b) launch shape: --root, --cwd, and the child-command `--` boundary (one gap per pattern)
  grep -qE -e "docket gate launch [^.]{0,120}--root" <<<"$flat" || { echo "root-missing"; return 1; }
  grep -qE -e "--root [^.]{0,120}--cwd" <<<"$flat" || { echo "cwd-missing"; return 1; }
  grep -qE -e "--cwd [^.]{0,160} -- " <<<"$flat" || { echo "separator-missing"; return 1; }
  # (c) ordering: launch precedes observe precedes record; record precedes verify
  p_launch="$(remint_pos "$flat" "docket gate launch")" || { echo "launch-missing"; return 1; }
  p_observe="$(remint_pos "$flat" "docket gate observe")" || { echo "observe-missing"; return 1; }
  p_record="$(remint_pos "$flat" "docket evidence record")" || { echo "record-missing"; return 1; }
  p_verify="$(remint_pos "$flat" "docket evidence verify")" || { echo "verify-missing"; return 1; }
  [ "$p_launch" -lt "$p_observe" ] || { echo "launch-after-observe"; return 1; }
  [ "$p_observe" -lt "$p_record" ] || { echo "observe-after-record"; return 1; }
  [ "$p_record" -lt "$p_verify" ] || { echo "record-after-verify"; return 1; }
  # (d) record consumes the produced run dir and binds the exact feature head
  grep -qE -e "docket gate observe [^.]{0,40}run" <<<"$flat" || { echo "observe-no-rundir"; return 1; }
  grep -qE -e "docket evidence record [^.]{0,80}--run" <<<"$flat" || { echo "record-no-rundir"; return 1; }
  grep -qE -e "--run [^.]{0,80}--head <feature head>" <<<"$flat" || { echo "record-no-head"; return 1; }
  # (e) verify follows record and checks the same head
  grep -qE -e "docket evidence verify [^.]{0,100}--head <feature head>" <<<"$flat" || { echo "verify-no-head"; return 1; }
  return 0
}

# SEPARATE non-vacuity anchors: an empty or renamed section must fail HERE, positively, so the
# negative conditions inside the checker can never pass against text that was simply not searched.
remint_sec="$(awk '/^### Step 6 — Review/{f=1;next} /^### Step 6\.5 — Results close-out/{f=0} f' "$IMPL")"
assert "remint: Step 6 section slice is non-empty (existence anchor)" '[ -n "$remint_sec" ]'
assert "remint: the named terminator heading still exists (slice cannot widen to EOF)" \
  'grep -qF -- "### Step 6.5 — Results close-out" "$IMPL"'

assert "remint: launch -> observe -> record -> verify chain holds in the authored skill" \
  'check_remint_chain "$IMPL"'

# Mutation proof: the guard is load-bearing. Copy, confirm the occurrence, remove it, confirm the
# removal landed, and require the SAME checker to reject the copy. Temp copy only — the real
# worktree is never edited, so no restoration step exists to get wrong.
remint_mut="$(mktemp "${TMPDIR:-/tmp}/remint-mutation.XXXXXX")"
assert "remint mutation: the gate-launch occurrence exists before removal" \
  'grep -qF -- "docket gate launch" "$IMPL"'
grep -vF -- "docket gate launch" "$IMPL" >"$remint_mut"
assert "remint mutation: the removal landed in the copy" \
  '! grep -qF -- "docket gate launch" "$remint_mut"'
assert "remint mutation: the checker rejects the launch-less copy" \
  '! check_remint_chain "$remint_mut" >/dev/null'
rm -f "$remint_mut"
```

Note on `$IMPL`: the file already defines it for the existing Step 6 controller section (near the `step6=` extraction). Confirm the variable name by reading that block; if it is scoped differently, define `IMPL="$REPO/skills/docket-implement-next/SKILL.md"` at the top of the new section instead of assuming it.

- [ ] **Step 2: Run the test file and verify the NEW asserts fail for the right reason**

Run: `bash tests/test_docket_review.sh`
Expected: the two anchors pass; `remint: launch -> observe -> record -> verify chain holds...` prints `NOT OK` with tag `launch-missing`; the three mutation asserts: the pre-removal existence assert FAILS too (`docket gate launch` is not yet in the file — that is the defect), the removal-landed assert passes vacuously, the rejection assert passes. This is the expected red state; every pre-existing assert in the file must still pass.

- [ ] **Step 3: Rewrite the two Step 6 paragraphs in `skills/docket-implement-next/SKILL.md`**

Replace the paragraph beginning `**Validate the build evidence (change 0170).**` (currently ending "...re-run the full suite once to mint the record yourself rather than reviewing an uncertified branch.") with:

```markdown
**Validate the build evidence (change 0170).** Read the build-evidence record step 5's gate emitted — it must be present, `result: green`, and its `head_sha` equal to the branch HEAD. Missing, malformed, or stale is a build-contract violation — never review an uncertified branch. Re-mint once yourself: `docket gate launch --root <absolute-run-root> --cwd <absolute-feature-worktree> -- <resolved-suite-command>` (`--root` is any writable absolute run root; the `--` keeps the resolved suite command on the child side; a bare suite run yields no recordable run dir), then `docket gate observe <run-dir>` under docket-build's bounded gate-execution posture until terminal — only `passed` at the current head feeds the recording below.
```

Replace the paragraph beginning `**Create the durable evidence.**` with (tightened; `<absolute-run-dir>` becomes `<run-dir>`, binding record's `--run` to the handle launch produced and observe reported):

```markdown
**Create the durable evidence.** Only from a **`passed`** terminal observation whose head equals the current feature head: `docket evidence record --id <id> --run <run-dir> --head <feature head>` reads the observed gate command and outcome from the run directory — **no agent-supplied `passed` boolean** — and returns the immutable typed record; a failed/running/stopped/vanished/malformed/head-mismatched run produces none. Then `docket evidence verify --record <request-file> --head <feature head>` re-checks its bytes against the head. Any review fix below changes HEAD and **invalidates this evidence** — repeat the launch → observe → record → verify chain before publishing.
```

Semantics deliberately preserved: docket-build's observation posture is **named**, not restated (spec §1); the fail-closed list and no-boolean rule are unchanged; the closing sentence now names the full chain instead of "re-gate, re-record, and re-verify".

- [ ] **Step 4: Verify the guard passes and the budget holds**

Run: `bash tests/test_docket_review.sh`
Expected: PASS end to end, including all six new asserts (the mutation trio now: exists → removed → rejected).

Run: `wc -w -l skills/docket-implement-next/SKILL.md` then `bash tests/test_skill_size_budgets.sh`
Expected: ≤ 180 lines and ≤ 6150 words, budget test PASS. The replacement adds ≈ +55 net words against 61 words of headroom, so it should fit. If over, trim in this order (never the command lines or the chain ordering): (1) in the Validate paragraph, drop "the resolved suite command" → "the suite command"; (2) drop "at the current head" from its last clause (record/verify still bind the head); (3) in Step 6's reviewer-rung paragraph, "matching the uncertainty sink `standard` is in docket-build's own routing" → "matching docket-build's uncertainty sink". Re-run both tests after any trim. Do NOT edit the budget row.

- [ ] **Step 5: Regenerate the embedded asset**

Run: `go generate ./internal/assets/`
Then: `go test ./internal/assets/ -count=1` (`-count=1` defeats the result cache, which does not track `skills/` edits) and `bash tests/test_asset_bundle_drift.sh`
Expected: both PASS; `git status` shows only `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md` and the embedded manifest changed, both mechanically.

- [ ] **Step 6: Commit**

```bash
git add tests/test_docket_review.sh skills/docket-implement-next/SKILL.md internal/assets/embedded
git commit -m "fix(0331): Step 6 re-mint names docket gate launch; mutation-proven chain guard"
```

(If the manifest lives outside `internal/assets/embedded`, add its exact path as shown by `git status` — stage only the generated files the generator wrote.)

---

### Task 2: Missing-producer audit of the rest of the skill

**Files:**
- Read: `skills/docket-implement-next/SKILL.md` (whole file, including `references/edge-paths.md` and `references/fix-loop.md`)
- Modify: only if a same-shape occurrence is found (then also re-run `go generate ./internal/assets/` and restage the embedded files)

**Interfaces:**
- Consumes: the Task-1 guard (must stay green through any edit here).
- Produces: an audit verdict recorded in the commit message (or, if no edit, in the Task 3 results note for the build record).

- [ ] **Step 1: Audit**

Read every instruction in the three files and ask: does it **consume an artifact or handle** (a run dir, a request file, a record, a path) **without naming the operation that produces it**? The shape being hunted is exactly what Step 6 had: consumer named, producer implicit. Known non-findings to skip: `docket change mark-implemented`'s chain (spec says already documented), and anything whose fix would need a new command or policy (out of scope — record it instead).

- [ ] **Step 2: Fix or record**

If a same-shape, bounded, prose-only omission is found: fix it minimally, re-run `bash tests/test_docket_review.sh` and `bash tests/test_skill_size_budgets.sh`, regenerate with `go generate ./internal/assets/`, verify `bash tests/test_asset_bundle_drift.sh`, and commit:

```bash
git add skills/docket-implement-next internal/assets/embedded
git commit -m "fix(0331): name the producer for <artifact> on <path> (same-shape audit hit)"
```

If none is found (or only out-of-scope gaps): make **no** edit and no commit; carry the verdict ("audit clean" or the named out-of-scope gap) into the build record / PR body so the spec's verification bullet ("the audit records whether any additional local omission was found") is discharged.

---

### Task 3: Whole-suite verification

**Files:** none modified.

- [ ] **Step 1: Run the whole suite through the runner**

Run: `bash scripts/run-tests.sh` (the resolved `finalize.test_command` — confirm by reading it from config, never a second copy)
Expected: PASS. This exercises, at minimum: `tests/test_docket_review.sh` (chain guard + mutation proof), `tests/test_skill_size_budgets.sh` (no allowance raised), `tests/test_asset_bundle_drift.sh` (regeneration landed).

- [ ] **Step 2: Investigate any `OVER BUDGET:` trailer**

An `OVER BUDGET:` line does not fail the run and nothing else will catch it. `tests/test_docket_review.sh` gained six asserts and two extra full-file greps — cheap, but check its wall-clock row (`tests/runtime-budgets.tsv`) anyway; a row AT its ceiling is already spent. If it breaches, report it as a finding (do not silently bump the budget).

- [ ] **Step 3: No commit**

Nothing to commit; this task produces the green gate the build-evidence record is minted from.

---

## Self-review notes (spec coverage)

- Spec §1 (executable chain, argument shape, posture named-not-restated, budget not raised) → Task 1 Steps 3–4.
- Spec §3 guard properties (a)–(e), separate non-vacuity anchor, whitespace tolerance, mutation proof on a temp copy, no Go test → Task 1 Steps 1–2 and 4.
- Spec §4 (audit; mechanical regeneration; budget headroom) → Task 1 Step 5, Task 2, Task 1 Step 4.
- Spec Verification list → Task 1 Steps 2/4/5, Task 3.
- Out-of-scope list honored: no gate/evidence CLI change, no observation-policy duplication, no mark-implemented change, no linter, no 0316 harvest restoration.
