---
id: 190
slug: close-the-build-evidence-value-gap-a-post-gate-results-commi
title: "Close the build-evidence value gap: a post-gate results commit always defeats finalize's suite skip"
status: in-progress
priority: medium
type: feat
created: 2026-08-01
updated: 2026-08-01
depends_on: []
related: []
discovered_from: [170]
adrs: [66]
spec: docs/superpowers/specs/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md
plan: docs/superpowers/plans/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi.md
results:
trivial: false
auto_groomable: true
branch: feat/close-the-build-evidence-value-gap-a-post-gate-results-commi
claimed_at: 2026-08-01T23:00:23Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md) |
| Plan | [2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi.md](https://github.com/danielhanold/docket/blob/feat/close-the-build-evidence-value-gap-a-post-gate-results-commi/docs/superpowers/plans/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi.md) |
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


