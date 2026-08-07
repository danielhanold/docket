<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0190 — Close the build-evidence value gap: a post-gate results commit always defeats finalize's suite skip](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0190-close-the-build-evidence-value-gap-a-post-gate-results-commi.md)**
<!-- docket:backlink:end -->

# Close the build-evidence value gap: a post-gate results commit always defeats finalize's suite skip — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: push through focused tests for each task, using TDD
> (red → green), and never rely on the whole-suite gate to tell you a task is done — the gate runs
> once at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `docket-finalize-change`'s post-rebase suite-skip predicate so the skip also fires
when the evidence `head_sha` is a **strict ancestor** of the branch HEAD and every path changed in
`head_sha..HEAD` lies under the allowlisted `<results_dir>/` prefix — the tree `docket-implement-next`
Step 6.5 commits post-gate — while keeping the fail-toward-running posture, the loud skip log, and the
`ci`/`both`-CI/`off` scoping untouched. Verified per-repo (no suite component reads `<results_dir>` as
a content source) and guarded by a new live mutation-tested guard test.

**Architecture:** This is a **prose-and-rail change** — two SKILL files, one README section, a budget
re-measure, and one brand-new guard test. No application runtime code. The extension ships
consumer-side (finalize derives the delta from git at skip time): the block's `head_sha` and
`result: green` continue to be the only producer inputs, and the range is computed fresh from
`git diff --name-only -z <head_sha>..HEAD`. The new guard test (`tests/test_skip_allowlist_invisibility.sh`)
is the trust-boundary backstop: it scans the **committed** tree (`git grep` over HEAD) for the
`<results_dir>` literal across `tests/`, `scripts/`, and suite-reachable skill files, classifies every
occurrence hazard-vs-benign, pins a population floor on the benign corpus, and is mutation-tested in
both directions. The ADR-0066 update is a **metadata-branch write run by the controller after the
build** (via the `docket-adr` dispatch), never a worker task — worker agents carry no docket metadata
operations (docket-build-task scope).

**Tech Stack:** Bash 3.2-compatible shell, markdown skill contracts, the repo's flat `tests/test_*.sh`
suite run with the configured `"$DOCKET_BASH_PATH" tests/test_<name>.sh`.

## Reconcile amendments — 2026-08-07 (base `f7fb123f`; authoritative over the text below)

The base advanced ~215 commits past the `e108568a` this plan was written against. The design is
unchanged; four factual anchors moved and **override** any conflicting number later in this file:

1. **Task 2 retargets.** The sentence "finalize's SHA-equality condition simply fails, and the suite
   runs" now lives in **`skills/docket-implement-next/references/edge-paths.md`**, in its
   *Build-evidence block (change 0170)* paragraph — **not** in `skills/docket-implement-next/SKILL.md`
   (Step 7 there delegates PR-body mechanics to that reference). Edit the reference; leave `SKILL.md`
   alone unless a sentence there genuinely conflicts.
2. **Budget rows moved.** Live `tests/test_skill_size_budgets.sh` rows / actuals:
   `docket-finalize-change/SKILL.md` **180 / 3500** (actual 176 / 3445);
   `docket-implement-next/SKILL.md` **165 / 4300** (actual 162 / **4285** — 15 words spare);
   `docket-implement-next/references/edge-paths.md` **35 / 450** (actual 28 / 411).
   Task 4 must now also check the **edge-paths** row. Re-measure with `wc` before raising anything.
3. **Guard corpus grew to 54** occurrences of the `docs/results` literal in the committed tree
   (`tests/` 48, `scripts/` 5, `skills/docket-convention/SKILL.md` 2, `README.md` 2,
   `.docket.example.yml` 1) — not the "~37" quoted below. **Derive the population floor from a live
   count at build time; never hard-code a number copied from this plan or the spec.**
4. **README anchor.** The clause to retract is in the evidence-chain paragraph, quoted verbatim as
   "the skip is the clean-path optimization, not the majority path". Anchor on the quoted clause,
   never a line number.

**Learnings that bind this build** (read the finding file before writing the guard — index at
`docs/changes/learnings/README.md` on the metadata branch):
`marker-scoped-guard-needs-a-population-floor` (existence / attachment / **coverage** are three
separate asserts; "at least one" pins a population, not coverage — close with a positive control on
a throwaway mutated copy), `backstop-must-compute-not-reenumerate` (derive the hazard predicate from
the consuming code, not a hand-written list of causes; mutation-test the **population**, not only the
suppression), `guard-remedy-must-not-teach-the-evasion` (a count guard whose failure message says
"bump the number" *is* the exploit — lead the remedy with the substantive check),
`assert-detects-removal-not-replacement` (write the assert that detects the state you removed, and
prove the mutation landed before believing the red), `plan-supplied-test-code-is-unverified` (any
snippet this plan hands you is unverified — prove it CAN pass, then mutation-test its key),
`phrase-grep-over-wrapped-prose` (collapse whitespace before matching prose, or a reflow reddens),
`escape-ere-metacharacters-in-key`, `agent-shell-noop-reads-as-success` (a sweep over zero items
prints success — assert the count), and `test-helper-interpolates-its-own-description` (no backticks
in assert descriptions).

## Global Constraints

- **Repo instructions bind.** `AGENTS.md` at the repo root is always in context. In particular:
  never `producer | early-exiting-consumer` under `set -o pipefail`; `grep` for a leading `--`
  pattern uses `-e`/`-F --`; awk indent classes are `[^[:space:]]`; anchor frontmatter edits to the
  first `---…---` block; before rewriting a marker-delimited managed block validate marker order and
  balance; never hand-list the sites of a literal you are gating; a guard is code — mutation-test it
  (strip what it guards, watch it redden) or it is decoration; cross-references anchor on a symbol
  name or a verbatim-quoted clause, **never a line number** (`tests/test_comment_anchor_style.sh`
  rejects the filename-plus-line form).
- **`<results_dir>` placeholder discipline is load-bearing.** This change's own prose in the two
  SKILL files, the README, and the new guard test uses the placeholder `<results_dir>` (or the
  configured value via config), **never the literal `docs/results/`** — because the new guard test
  greps the committed tree for that literal, and the spec's guard would otherwise match this change's
  own prose and demand classification of it (`docs/results/` occurs ~37× in the committed corpus
  today: ~34 in `tests/`, plus config-key refs in `scripts/docket-config.sh` and
  `skills/docket-convention/SKILL.md`).
- **The executable fragment stays pure.** `tests/test_docket_review.sh` asserts the
  `configured-bash-finalize` fragment in `skills/docket-finalize-change/SKILL.md` contains none of
  `evidence|skip|head_sha`. The new skip prose lives in the prose item, never in that fragment.
- **Size budgets are enforced.** `tests/test_skill_size_budgets.sh` fails on any file over its cap.
  Current caps (**see reconcile amendment 2 — these are stale**): `docket-finalize-change/SKILL.md`
  193 ln / 4350 w; `docket-implement-next/SKILL.md`
  147 ln / 3950 w — both within 3 lines of their cap, so a raise is expected and ships in this same
  PR per the file's own house rule (re-measure the live actual, then round: lines → next multiple of
  5, words → next multiple of 50, taking the multiple *after* if within 25 words of the actual; add a
  header-comment paragraph explaining each raise).
- **The guardian must gain, not lose.** `tests/test_docket_review.sh`'s existing finalize sentinels
  (no-op rebase / `result: green` / `head_sha` / fails-toward-running / `ci` untouched / fragment
  purity) all survive the extension unchanged; the change adds exactly one shape assert for the new
  ancestor+allowlist limb.
- **Portability.** The interactive shell's `grep`/`rg` is ugrep and accepts constructs BSD `grep`
  rejects; any new regex or grep construct must be re-verified under `/usr/bin/grep` (no `\b`, no
  `\<`) before the task is complete. `git grep -E` is POSIX ERE.
- **TDD.** For the new guard test: write the failing assertions first (including the mutation tests),
  watch them redden against the pre-change tree, then build the classifier to green.

## File Structure

**New files**

| Path | Responsibility |
|---|---|
| `tests/test_skip_allowlist_invisibility.sh` | The live trust-boundary guard: `git grep` over HEAD for `<results_dir>` in `tests/`, `scripts/`, and suite-reachable skill files; hazard-vs-benign classification with a population floor; positive claim keyed on the exclusion magic tokens; mutation-tested both ways. |

**Modified files**

| Path | Change |
|---|---|
| `skills/docket-finalize-change/SKILL.md` | Step 4 conditional-skip: add the ancestor+allowlist disjunct, the loud log extension, the degrade-off rule; keep the fragment pure. |
| `tests/test_docket_review.sh` | One new shape assert binding the new disjunct (ancestor + allowlist), plus a non-vacuity guard for it; existing sentinels untouched. |
| `skills/docket-implement-next/references/edge-paths.md` | **(retargeted — amendment 1)** *Build-evidence block* paragraph: replace "finalize's SHA-equality condition simply fails, and the suite runs" with the extended-predicate outcome. |
| `README.md` | Evidence-chain section: document the docs-only skip + per-repo verification rule; retract/qualify the "clean-path optimization, not the majority path" caveat. |
| `tests/test_skill_size_budgets.sh` | Re-measure and raise the caps for `docket-finalize-change/SKILL.md` and `docket-implement-next/references/edge-paths.md` (and `SKILL.md` only if it grew); header-comment paragraphs. |

---

## Task 1 — Extend finalize's skip predicate (finalize SKILL + its guardian)

**Build profile:** standard

**Files:** `skills/docket-finalize-change/SKILL.md`, `tests/test_docket_review.sh`.

In `skills/docket-finalize-change/SKILL.md` step 4 (the "Conditional skip of the local suite run"
item), extend the predicate: the skip fires when ALL hold:

1. the rebase was a no-op (unchanged), **and**
2. the PR body carries a parseable `docket:build-evidence` block whose `result: green` (unchanged),
   **and**
3. the block's `head_sha` **equals** the branch HEAD being merged (unchanged) **OR both of**: the
   `head_sha` is a **strict ancestor** of the branch HEAD **and** every path changed in
   `head_sha..HEAD` lies under the allowlisted prefix — the repo's `<results_dir>/` (trailing slash,
   prefix-matched; config-derived, never hard-coded path names in the prose).

Keep the surrounding posture verbatim in intent: "anything else — a missing, malformed, or
unparseable block, a non-ancestor SHA, a non-green result, **any changed path outside the
allowlist** — runs the suite exactly as before"; "the posture fails toward running: any doubt costs
one suite run, never a broken integration branch"; the skip is scoped to the local leg alone
(`ci`, `both`'s CI leg, `off` untouched). Extend the loud one-line skip log so it names the matched
permit: either the exact-SHA match, or the docs-only ancestor match with a byte-identical delta
summary (`head_sha → HEAD, N files, all under <results_dir>/`). Add the **degrade-off rule**: if the
suite-invisibility verification (Task 5's guard) cannot be established at build reconcile, the
extension ships off — finalize behaves as the pre-0190 equality-only predicate. Use the
`<results_dir>` placeholder, never a literal path.

Do **not** touch the `configured-bash-finalize` executable fragment.

In `tests/test_docket_review.sh`, in the finalize section: add **one shape assert** binding the new
limb — keyed on syntactic shape, not an enumerated spelling: e.g. assert the skip-item text states an
*ancestor* condition AND a *paths-under-<results_dir>*-allowlist condition (both present together),
with a non-vacuity anchor so a renamed/deleted item reddens. Do not weaken the fragment-purity assert;
confirm the existing three sentinels still pass. Mutation-test the new assert: strip the allowlist
clause from a throwaway copy and confirm it reddens.

**Focused verification:** `"$DOCKET_BASH_PATH" tests/test_docket_review.sh` green; the new assert
reddens under its mutation.

---

## Task 2 — Update implement-next Step 7's build-evidence prose

**Build profile:** economy

**Files:** `skills/docket-implement-next/references/edge-paths.md` (**retargeted — amendment 1**).

In its "**Build-evidence block (change 0170)**" paragraph, replace the sentence that reads
"finalize's SHA-equality condition simply fails, and the suite runs" with the extended-predicate
outcome: a step-6.5 results commit still moves HEAD past the minted `head_sha`, but finalize now skips
when the post-gate delta is docs-only — the `head_sha` is a strict ancestor and every path changed in
`head_sha..HEAD` lies under the repo's `<results_dir>/` — and runs the suite for any other post-gate
commit. Use the `<results_dir>` placeholder; keep the sentence a rule the reader executes, not a
summary. No other step-6/7 prose changes.

**Focused verification:** no test asserts this exact sentence; confirm the suite still passes the
budget-relevant asserts after editing by running the affected reviewer guardian
`"$DOCKET_BASH_PATH" tests/test_docket_review.sh` (it asserts the step-7 markers are present, not the
sentence).

---

## Task 3 — README evidence-chain section

**Build profile:** economy

**Files:** `README.md`.

In the evidence-chain section (the paragraph describing the build-evidence chain and its caveats):
document the extended skip — when the pre-merge rebase is a no-op and the evidence is green, finalize
skips the post-rebase run not only on an exact-SHA match but when the `head_sha` is a strict ancestor
whose every changed path lies under the repo's `<results_dir>/` (the tree step 6.5 commits
post-gate), verified per-repo by a live guard that no suite component reads that tree as content.
**Retract/qualify** the existing "the skip is the clean-path optimization, not the majority path"
caveat: 0190 inverts it — the docs-only ancestor permit makes the skip the majority path, while any
real code delta still fails the skip and runs the suite. Keep the PR-body human-editable caveat
untouched (it remains true and load-bearing). Keep the "one full-suite run when the review is clean
and the base has not moved" arithmetic statement consistent with the extended predicate.

**Focused verification:** no test greps the retracted phrase (verified: the phrase appears in no
`tests/` file); run `"$DOCKET_BASH_PATH" tests/test_readme_finalize_docs.sh` and
`"$DOCKET_BASH_PATH" tests/test_docket_review.sh` to confirm no sentinel regressed.

---

## Task 4 — Re-measure and raise the skill size budgets

**Build profile:** economy

**Files:** `tests/test_skill_size_budgets.sh`.

After Tasks 1 and 2 have landed, re-measure the live actuals of the two edited SKILL files:
`skills/docket-finalize-change/SKILL.md`, `skills/docket-implement-next/references/edge-paths.md`,
and `skills/docket-implement-next/SKILL.md`. For each file over its cap, raise the budget per the file's own rounding rule (next multiple of 5 lines, next
multiple of 50 words; take the multiple *after* when within 25 words of the actual) and append a
header-comment paragraph in the established house style explaining the raise (the extended skip
predicate for finalize; the step-7 outcome sentence for implement-next). Do not raise a cap that does
not need raising.

**Focused verification:** `"$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh` green; the measured
actuals recorded in the header paragraphs match the live `wc` counts.

---

## Task 5 — New live guard: `tests/test_skip_allowlist_invisibility.sh`

**Build profile:** premium

**Files:** `tests/test_skip_allowlist_invisibility.sh` (new).

This file is the trust-boundary backstop and the largest task. Following the design in the spec's
"Safety does not rot silently" section (the linked spec is authoritative — re-read its lines on the
guard before starting):

- Scan the **committed** tree via `git grep` over HEAD (restricts the scan to committed tracked
  blobs, the tree the suite runs against) for the `<results_dir>` literal across `tests/`, `scripts/`,
  and suite-reachable skill files. Note: this intentionally does not hide the literal — `docs/results/`
  occurs ~37× in the committed corpus today, so classify, don't exclude.
- Classify every occurrence **hazard-vs-benign** by what the consuming code does: a read/grep/cat of
  the tree as a content source is a hazard; fixture paths, config-key references, comments, and the
  suite's own exemption constructs are benign. The hazard predicate derives from the actual consuming
  code; the benign classification is curated (state this honestly in the file's header).
- **Population floor:** the full committed benign corpus must classify clean, or the guard reddens on
  arrival (~34 in `tests/` + config-key refs in `scripts/docket-config.sh` and
  `skills/docket-convention/SKILL.md`). Seed the curated benign set in this same task. Interior tabs
  or reflow must not shift classification.
- **Positive claim by machine-recognized shape, never the bare path:** key on the exclusion magic
  tokens — `test_docket_build.sh`'s `:!docs/results` path-exclusion construct and
  `test_readme_finalize_docs.sh`'s `--glob "!docs/results/**"` escape — not a bare-path assert (the
  bare literal also sits in comments and armed probes, so a bare-path assert stays green when the real
  exclusion is deleted).
- **Mutation tests, both directions:** (a) deleting the `:!docs/results` exclusion token in
  `test_docket_build.sh` reddens; (b) adding a new content-read of `<results_dir>/` reddens. Each
  mutation must be confirmed to have landed (count before/after) before the red run is believed.
- **Honest limits, stated in the file:** a co-located-verb detector cannot catch an indirect read
  (`r="$ROOT/docs/results"; grep x "$r"`); mutation only proves the direct form reddens. The guard's
  own prose uses the `<results_dir>` placeholder so it never matches its own body.
- POSIX-ERE discipline throughout: no `\b`, no `\<`.

**Focused verification:** `"$DOCKET_BASH_PATH" tests/test_skip_allowlist_invisibility.sh` green
(exit 0, floor satisfied), and both mutations demonstrated red against throwaway copies.

---

## Task 6 — Build gate (not a worker task; run by docket-build)

After all plan tasks commit, docket-build runs the whole suite once and mints the build-evidence
record on green. The suite-invisibility verification is the build gate's own first-class citizen: if
Task 5's guard reddens, the gate is red, and the degrade-off rule (Task 1's prose) is what ships.
Record the verification result in the results file at step 6.5 (see below).

---

## Post-build controller steps (run by docket-implement-next, NEVER worker tasks)

These are **metadata-branch writes** and are outside docket-build's scope; a worker must not attempt
them.

- **ADR-0066 update (via `docket-adr`).** After the build gate is green, the controller dispatches
  the `docket-adr` subagent to append a dated `## Update` note to
  `docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md` extending the skip
  decision with the ancestor+allowlist rule — which **also dates-and-closes** the deferral sentence in
  that ADR's Consequences ("A docs-only ancestor exemption was considered and deliberately deferred…
  separate design work" — now done). Never an edit to the Decision; the appended-note form only. The
  update is delivered to `main` atomically with this change at terminal-publish via this change's
  `adrs: [66]` (adr-update-delivery rule).
- **Results file (step 6.5).** Author `docs/results/<YYYY-MM-DD>-close-the-build-evidence-value-gap-a-post-gate-results-commi-results.md`
  from `results-template.md` IN THE FEATURE WORKTREE, recording the suite-invisibility verification
  outcome (which classification corpus the guard used, both mutation results), the budget raises, and
  the review findings/ADR-0066 note. This is a **post-gate commit** — by design it moves HEAD past the
  minted evidence, and the step-7 evidence block is written anyway (finalize's extended predicate now
  handles exactly this path).
- **`plan:` field, `status: implemented`, `pr:` — controller metadata writes on `metadata_branch`.**
