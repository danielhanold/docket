---
id: 190
slug: close-the-build-evidence-value-gap-a-post-gate-results-commi
title: "Close the build-evidence value gap: a post-gate results commit always defeats finalize's suite skip"
status: done
priority: medium
type: feat
created: 2026-08-01
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [170]
adrs: [66]
spec: docs/superpowers/specs/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md
plan: docs/superpowers/plans/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi.md
results: docs/results/2026-08-07-close-the-build-evidence-value-gap-a-post-gate-results-commi-results.md
trivial: false
auto_groomable: true
branch: feat/close-the-build-evidence-value-gap-a-post-gate-results-commi
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/173
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md) |
| Plan | [2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi.md) |
| Results | [2026-08-07-close-the-build-evidence-value-gap-a-post-gate-results-commi-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-close-the-build-evidence-value-gap-a-post-gate-results-commi-results.md) |
| ADRs | [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md) |
<!-- docket:artifacts:end -->

## Why

Change 0170's build-evidence chain lets `docket-finalize-change` skip its post-rebase suite run
only when the rebase was a no-op AND the PR body's evidence block is green AND its `head_sha`
equals the branch HEAD being merged. That third condition is exact SHA equality, which is what
makes the predicate safe.

But `docket-implement-next` Step 6.5 commits the `results:` file **on the feature branch** after
the build gate has already minted the evidence. Any such post-gate commit moves HEAD, so the
`head_sha` no longer matches and the skip never fires. The whole-branch review measured the
frequency against this repo's own history: roughly 73% of archived changes carry a results file.
So the headline benefit — one full-suite run on the clean path — is inert on the majority path.

This is **not a safety bug**: the predicate fails toward running, which is the correct posture,
and 0170 documents the caveat honestly in both Step 7 and the README rather than hiding it. It is
a value gap, deliberately left open rather than closed in haste.

## What changes

Extend `docket-finalize-change`'s post-rebase suite-skip predicate (from change 0170) with a
narrow **docs-only ancestor** disjunct: the skip fires when the rebase is a no-op AND the PR body's
evidence block is green AND (`head_sha` equals the branch HEAD, as 0170 ships — **or** `head_sha` is
a strict ancestor of HEAD and every path changed in `head_sha..HEAD` lies under the repo's
`<results_dir>/`, the tree Step 6.5 commits post-gate). Anything else — a missing/malformed block,
a non-ancestor SHA, any changed path outside the allowlist — runs the suite exactly as today; the
"fails toward running" posture and the loud one-line skip log survive unchanged.

The consumer-side extension is chosen over the two alternatives because re-testing to earn a skip
never reduces the run count: a producer-side **re-mint** of the evidence at Step 7 is net-neutral
on the common path and costs an extra run when the base moved (gate + re-mint + finalize), and a
producer-issued **attestation** field is strictly weaker than finalize deriving the delta from git
state at skip time. Safety of the allowlist is **per-repo, verified, and guarded**: this change
adds a live guard test asserting no suite component reads `<results_dir>/` as content (this repo's
suite is hermetic to it by construction), with a build-time degrade-off rule — if the verification
cannot be established at build reconcile, the extension ships off (0170's equality-only predicate).

Design detail, the smuggling-vector enumeration, the guard contract, and the ripple list live in
the linked spec. The skip's trust boundary change is recorded as a dated Update note on ADR-0066
(this change's `adrs:`).

One ripple surface has **moved** since the spec was written: the "stale `head_sha` is EXPECTED"
prose the extension must update now lives in
`skills/docket-implement-next/references/edge-paths.md` (its *Build-evidence block* paragraph),
not in that skill's `SKILL.md`. See the 2026-08-07 reconcile entry.

## Out of scope

- Weakening any other condition of the skip predicate.
- Changing where the evidence block lives (the PR body; settled by ADR-0066).

## Reconcile log

### 2026-08-01

Claimed and reconciled against merged `origin/main` — current tip `e108568a`, 0170's terminal
publish; the base has **not** moved since the spec recorded that tip.

**Design holds — no scope change.**

- 0170 is `done`/archived; the skip stanza it ships is live at
  `skills/docket-finalize-change/SKILL.md` step 4 (conjunction of a no-op rebase, a parseable green
  `docket:build-evidence` block, and `head_sha` equal to the branch HEAD, with the fail-toward-running
  posture and the loud skip log). Confirmed the guardian `tests/test_docket_review.sh`'s existing
  sentinels (no-op / `result: green` / `head_sha` / fails-toward-running / `ci` untouched /
  fragment purity) all survive a new ancestor+allowlist disjunct.
- Suite-invisibility re-verified against the merged tree: 106 of 145 archived changes (73%) carry a
  `results:` field, matching the measured frequency. Both exemption sites the spec relies on are
  live — `tests/test_docket_build.sh`'s `:!docs/results` path-exclusion and
  `tests/test_readme_finalize_docs.sh`'s `--glob "!docs/results/**"` — and the remaining
  `docs/results` occurrences in tests/scripts/skills are benign (comments, fixture paths, config-key
  references), not content reads. The `docs/results/` allowlist is suite-invisible in this repo; the
  live guard test ships in-build.
- Budget caps confirmed: `tests/test_skill_size_budgets.sh` holds finalize at 193 ln / 4350 w and
  implement-next at 147 ln / 3950 w — the spec expects in-diff raises for the extended stanza.
- ADR-0066 remains Accepted with its Consequences deferral sentence ("docs-only ancestor
  exemption… deliberately deferred") still open; the build records a dated `## Update` note that
  dates-and-closes it (via `docket-adr`). 0190's `adrs: [66]` is already set and the `## Artifacts`
  block already lists ADR-0066.

### 2026-08-01 (halt — resume marker)

Build halted at Step 5 under the convention's **Tier C (authorized-or-halt)** rule: the resolved
build skill `docket-build` is invocable but **cannot dispatch** its profile agents on this machine —
the harness rejected a dispatch naming `docket-build-standard` ("Unknown agent type") during the
mandatory dispatch-capability probe. `skills.build` is `docket-build` (not `auto`), so there is no
inline authorization. The change stays `in-progress` with the claim lease refreshed; worktree
`.worktrees/close-the-build-evidence-value-gap-a-post-gate-results-commi` and the committed plan
(`9419211d`) are intact for resume.

Remedy: re-run `install.sh` (regenerates the profile wrappers and links the build skills), start a
fresh session so the harness registers the `docket-build-*` profile agents, then resume this change —
the reconcile pass has already run (`reconciled: true`) and must be re-run only if
`origin/<integration_branch>` advances past `e108568a`.


### 2026-08-07 (resume reconcile — base advanced)

Re-reconciled against `origin/main` tip `f7fb123f` (0228's terminal publish). The base advanced
~215 commits past the `e108568a` the previous pass and the spec both recorded, so the full pass
re-ran per the resume-safety guard.

**Design holds — no scope change, no kill.** All four premises re-verified on the live tree:

- Finalize's skip predicate is still **equality-only**: `skills/docket-finalize-change/SKILL.md`
  step 4, item *"Conditional skip of the local suite run"*, conjoins a no-op rebase, a parseable
  green `docket:build-evidence` block, and `head_sha` **equal to** the branch HEAD, with the
  fail-toward-running posture and the loud one-line skip log.
- `docket-implement-next` Step 6.5 still commits the results file on the feature branch **after**
  the gate mints the evidence.
- ADR-0066 is still `Accepted` and its Consequences deferral sentence ("A docs-only ancestor
  exemption was considered and deliberately deferred as separate design work") is still open. Its
  one existing `## Update` (2026-08-02, change 0193) is about the `skills.review` default and does
  not touch the skip. The new note appends as a **second** Update.
- Suite-invisibility of `<results_dir>` still holds. Both exemption tokens are live —
  `tests/test_docket_build.sh`'s `:!docs/results` path-exclusion and
  `tests/test_readme_finalize_docs.sh`'s `--glob "!docs/results/**"` escape — and every other
  occurrence is a fixture path, config-key reference, or comment. `tests/test_skip_allowlist_invisibility.sh`
  does not yet exist; it ships in-build as designed.

**Four measured deltas fold into the plan (revalidated, not discarded):**

1. **Ripple surface moved.** The sentence the spec assigns to `skills/docket-implement-next/SKILL.md`
   Step 7 — "finalize's SHA-equality condition simply fails, and the suite runs" — now lives in
   `skills/docket-implement-next/references/edge-paths.md`, in its *Build-evidence block (change
   0170)* paragraph. `SKILL.md`'s Step 7 delegates PR-body mechanics to that reference. Task 2
   retargets; Task 4 must also budget-check `references/edge-paths.md`.
2. **Budget caps moved in both directions.** The spec/plan cite finalize 193 ln / 4350 w and
   implement-next 147 ln / 3950 w. The live rows in `tests/test_skill_size_budgets.sh` are
   **finalize 180 / 3500** (actual 176 / 3445 — 4 ln, 55 w headroom), **implement-next 165 / 4300**
   (actual 162 / 4285 — 3 ln, **15 w** headroom), and **edge-paths 35 / 450** (actual 28 / 411).
   Implement-next's word headroom is now nearly nil, so the in-diff raise is not merely "expected"
   but near-certain for any file the change touches. Re-measure at build time; do not trust these
   numbers as caps.
3. **Guard corpus grew.** The `docs/results` literal now occurs **54×** in the committed tree across
   `tests/` (48), `scripts/` (5 incl. contracts), `skills/docket-convention/SKILL.md` (2),
   `README.md` (2) and `.docket.example.yml` (1) — not the spec's "~38". The guard's population
   floor must be **derived from a live count at build time**, never pinned to a stale constant.
4. **Change 0218's fix loop is now live** and its post-revert re-run **refreshes** the evidence
   record (asserted by `tests/test_docket_review.sh`). So in-branch fix commits are re-pinned and
   the Step 6.5 results file remains the dominant post-gate delta — the allowlist stays minimal
   (`<results_dir>/` alone) exactly as Assumption 3 argues.

Frequency re-measured on the merged tree: **125 of 172** archived changes (73%) carry a `results:`
field — the value premise is unchanged.

The feature branch (plan-only, one commit) was rebased onto `f7fb123f` so the build runs against
current code. No auto-capture mint: this pass surfaced no follow-up work distinct from the change
itself.
