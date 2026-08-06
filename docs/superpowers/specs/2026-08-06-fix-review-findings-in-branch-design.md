<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0218 — Fix review findings in-branch instead of minting a stub for every one](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-06-0218-fix-review-findings-in-branch-instead-of-minting-a-stub-for.md)**
<!-- docket:backlink:end -->

# Fix review findings in-branch instead of minting a stub for every one — design

Change: #0218 · Groomed: 2026-08-06 · Status at grooming: proposed → build-ready

## Problem

54 of 56 proposed changes carry `discovered_from` — the loop manufactures its own backlog because
no station may fix a review finding. `docket-review` is read-only by contract (ADR-0066), and
`docket-implement-next` Step 6 records every non-blocking finding in the PR body "for merge-time
judgment, never auto-fixed," so the only exit for a finding is a stub that competes against real
feature work. The pattern is live: 0197, 0200, and 0220 are "clear the unfixed review findings"
changes sitting in `active/` at grooming time; 0215–0217 (minted from 0202's findings) were killed
and consolidated into 0200 — one full change lifecycle consumed per batch of mostly one-line fixes.

## Decision summary

A bounded **fix loop inside `docket-implement-next` Step 6** — an extended phase, not a new role
skill — that repairs findings on the open branch, after review returns and before the PR opens.
Routing is by the **character of the fix** using docket-build's task-routing rubric (extracted to a
shared reference); **severity sets only the failure posture**, never the model tier. The human's
merge gate does not move: every auto-authored fix arrives inside the diff they already review.

Two axes, deliberately orthogonal:

- **Character → profile.** A finding is a very small plan task with the diagnosis pre-written.
  Route it with the same rubric docket-build applies to plan tasks: `economy` only when positively
  established, `standard` as the uncertainty sink, `premium` for named/consequential-but-correctable
  risk. Same one-bounded-escalation rule as build, truncated at premium.
- **Severity → posture.** Blocker: must fix or halt. Important / minor: a failed fix falls back to
  a PR-body record — the branch is never worse off for having tried.

## Routing table (final form)

| Finding character (rubric) | blocker | important | minor |
|---|---|---|---|
| economy | fix (→ 1 escalation) | fix (→ 1 escalation) | fix, batched (→ 1 escalation) |
| standard | fix (→ 1 escalation) | fix (→ 1 escalation) | fix (→ 1 escalation) |
| premium | fix (no retry — next rung is max) | fix (no retry) | fix (no retry) |
| max | **halt** (today's ladder endpoint) | PR-body record | PR-body record |

- **`max` is never dispatched from the fix loop, for any severity.** Premium is "consequential but
  correctable" — still walk-backable inside a reviewed diff; max is defined by irreversibility,
  which must never happen to a branch as an unplanned side-quest. This matches today's blocker
  ladder (standard → premium → halt), which also never reaches max.
- The rubric doubles as the size ceiling: no separate knob. A max-character non-blocker — rare by
  construction (unresolved architecture / irreversible data change flagged as minor essentially
  does not occur) — becomes a line in the PR body for the human's merge-time judgment, **not** a
  follow-up change.
- Failure after exhausted escalation: blocker → halt (abort-and-report, unchanged); important /
  minor → PR-body record with the failure as the recorded reason.

## Shared rubric — extraction

Extract docket-build's §Routing rubric to **`docket-build/references/task-routing.md`**, leaving a
stub + pointer in `docket-build/SKILL.md` (the `skill-extraction-and-stub-pointer` pattern). Both
consumers — docket-build's per-task routing and the fix loop — read the same file; the fix phase
never restates the rubric (restatement drift is a documented learnings class). The justification
for extraction is shared consumption, not section weight.

## Tasks, batching, commits

All fix tasks run the **`docket-build-task` contract** (focused test → implement → verify →
self-review → one commit), dispatched by profile name, foreground, sequentially. One task = one
commit:

- **Blockers and importants:** one task per finding → individual commits naming the finding and the
  reasoning. This **replaces** today's single synthetic all-blockers task — per-finding tasks give
  failure isolation and a bisectable narrative; blockers are rare, so the extra dispatches are
  negligible.
- **Minors:** route each finding first, then batch minors sharing a profile into one task per
  profile (in practice one `economy` batch). The batch tier is its members' shared tier —
  homogeneous by construction. One commit enumerating the findings; a failed batch falls back to
  recording its members.
- Track each task's commit SHA(s), and whether the task fixed a blocker, for the suite gate below.

## Suite gate — revert-and-record

One full-suite re-run after all fix tasks land; refresh the build-evidence record. If red:

1. Revert the **non-blocker** fix commits (by tracked SHA), re-run the suite once more.
2. Green → proceed; the reverted findings are recorded unfixed in the PR body (the fallback they
   already had).
3. Still red → the blocker fixes are implicated → **halt**, as today (no second repair chain).

Bounded at two suite runs. The loop can never leave the branch worse than the green build that
entered it.

## No re-review

No second review round after fixes (today's rule, kept). Remediation is carried by the worker's
own self-review, the suite gate, and the human reading every fix in the PR diff.

## Configuration — `review.min_fix_severity`

```yaml
review:
  min_fix_severity: minor   # minor (default) | important | blocker
```

The minimum severity that enters the fix loop; blockers are always fixed regardless (a run cannot
proceed past an unfixed blocker). `important` = minors are recorded, not fixed; `blocker` =
pre-0218 behavior, the compat escape hatch. Findings below the threshold take today's record path.
Resolved by `docket-config.sh --export` as **`REVIEW_MIN_FIX_SEVERITY`**; not a coordination key (it
shapes branch content, not shared metadata), so it is global-able like `finalize.gate`. Per the
`config-knob-ship-end-to-end` learning, the sample config, README, and the now-relaxed Step 6
prose ship in the same change.

## Recording surfaces

- **PR body:** the findings section becomes a disposition table — fixed (with SHA) / deferred
  (with reason) / reverted / recorded.
- **`## Verify (human)`** (results file): shrinks to genuinely manual checks. Fixed findings never
  enter it — the fix plus the green suite is the verification.
- **Auto-capture narrows:** a finding about this branch's own diff is **never mintable** — it is
  fixed or recorded. The materiality bar in `docket-convention/references/auto-capture.md` gains an
  explicit clause: *work fixable by a small in-branch edit fails the bar.* Minting from review
  remains only for distinct beyond-the-branch work that independently clears the bar
  (retroactively, 0217 would not have been mintable).

## Out of scope

- `docket-review`'s read-only contract — the reviewer stays a reviewer (ADR-0066 unchanged).
- Retroactively clearing the existing self-generated backlog (its own triage pass).
- The finalize-side `docket-integration-repair` path, correct as-is for its purpose.

## Testing notes (build-time detail belongs to the plan)

Sentinel/guard updates for: the rewritten Step 6 triage prose; the §Routing extraction stub +
pointer (the `restatement-accumulates-its-own-guards` learning bites here — existing tests may
grep the copy); the new `REVIEW_MIN_FIX_SEVERITY` resolver export (export-list order guard); the
rubric reference being read by two consumers; and the auto-capture materiality-bar clause.
