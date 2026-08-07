---
id: 219
slug: aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d
title: aborted-run's sixth signature: PR opened and pr: written, run dies before status: implemented
status: proposed
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-07
depends_on: [211]
related: [200, 222]
discovered_from: [211]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`aborted-run`'s three legs (0113's A and B, 0211's C) leave one abort signature undetected at
run-scale: the run opens the PR and writes `pr:`, then dies before writing `status: implemented`.

- Leg A sees no incoherence — `plan:` and `results:` are both recorded by then.
- Leg C is gated on `pr:` being EMPTY (deliberately: a recorded PR means the branch was delivered),
  so a written `pr:` makes leg C skip with zero git calls.
- Leg B catches it, but only at 12h — the same lag 0211 exists to close for the build-complete case.

This is the signature 0194's second stop actually produced (PR open, manifest unwritten), so it is
observed, not hypothetical.

Both 0211's spec and its change body name it explicitly as a deliberate follow-up rather than a
fold-in, for one reason: its evidence is a **manifest/GitHub comparison** (is the PR that `pr:`
names open, merged, or gone?), and `board-checks.sh` is git-only by contract — it shells no `gh`.
So the oracle for this signature has to live somewhere else, or the contract has to change. That is
a design question, not a fourth leg.

## What changes

Decide where a `pr:`-set, `status: in-progress` change gets its run-scale abort check, and build it:
either a new check outside `board-checks.sh` (the `docket-status` pass already runs `gh` elsewhere),
or an explicit, scoped relaxation of the git-only contract with the offline/rate-limit posture
settled. Then emit a finding naming the missing `status: implemented` write.

## Out of scope

- Retuning leg B's 12h horizon.
- Any status flip or claim release — the advisory posture holds.

## Auto-groom blocked

### 2026-08-07

The draft design was gated by the adversarial critic and abstained on one undecidable decision. No
spec was emitted; the stub stays needs-brainstorm and first in the interactive queue.

**The undecidable decision — which of two signatures this change is actually for.** The stub's title
and body describe the state *`pr:` written, `status:` still `in-progress`*. Read against the running
code, that state needs no GitHub call at all: `pr:` has exactly one writer — `docket-implement-next`
step 7 — which sets `status: implemented` **and** `pr:` in a single field-write, and no script in
`scripts/` writes the field. So the signature *as literally stated* is manifest-internal and is
detectable git-only, by a cheap fourth `aborted-run` leg inside `board-checks.sh` with no relaxation
of its git-only contract and no `gh` anywhere. The design was drafted that way, and the leg itself is
sound.

But 0211 records the **observed** incident (0194's second stop) as "the PR open and the **manifest
unwritten**" — the *other* variant: a PR that exists on GitHub while `pr:` was never recorded. That
one genuinely does need a manifest/GitHub comparison, which is why both 0211's out-of-scope note and
this stub's `## Why` say the evidence is a manifest/GitHub comparison. The two halves of the
human-authored text point at different states, and picking either horn costs something real:

- **Build the literal title state (the git-only leg).** Cheap, contract-preserving — but this stub
  then loses its "observed, not hypothetical" motivation, and the state that was actually seen goes
  both unbuilt and unfiled. The critic also noted that under this reading the human's own sentence
  "leg C's `pr:`-empty gate makes it invisible" becomes a non-sequitur, which is further evidence the
  GitHub variant is what was meant.
- **Build the GitHub-evidence check.** Matches the observed incident and the stated evidence — but
  requires either a new check outside `board-checks.sh` (in `docket-status.sh`, which already shells
  `gh`) or a scoped relaxation of the git-only contract, each with an offline/rate-limit posture to
  settle.

This is a question of **what you meant**, not of fact, so no reconcile pass and no amount of code
reading resolves it — which is exactly the abstain condition.

**What a human should supply.** One line naming which state 0219 is for. If it is the literal
`pr:`-written state, the git-only leg-D design is ready to re-derive and cheap. If it is the
GitHub-evidence residual, say whether the check goes in `docket-status.sh` or the `board-checks.sh`
git-only contract gets relaxed, and what the offline/rate-limit posture is when `gh` is unavailable.

**Recommendation.** Split rather than choose: keep 0219 for the git-only leg (small, additive,
contract-preserving, and it closes a real partial-write hole even if that hole is only reachable via
an uncommitted edit or a non-compliant driver), and mint a separate stub for the GitHub-evidence
residual, which is the one carrying the design question. Not killable or deferrable — a genuine gap
exists on both readings.

**Findings from the critic worth carrying into whichever design is picked**, so they are not
rediscovered:

- If the git-only leg is built, its honest yield is *uncommitted partial edits in the shared
  `.docket` worktree plus non-compliant drivers* — `board-checks.sh` reads the filesystem, not a git
  blob. Say that plainly as a "Known residual" in the leg-C shape; no idle floor is constructible for
  an uncommitted edit, so no floor is right, but not for the reason a first draft reaches for.
- Hoist one `ar_pr="$(fm_field "$f" pr)"` above leg C and have both legs test it — `board-checks.sh`
  documents that path as cost-sensitive (change 0176), and a second read there is a real regression.
- The `## Not covered` paragraph in `scripts/board-checks.md` must be **rewritten** to name whatever
  residual survives, never deleted — it is the only written record of the gap.
- The `aborted-run` preamble comment in `scripts/board-checks.sh` still says "Two INDEPENDENT legs;
  either emits, and both can emit on one change" — stale since 0211 added leg C. Fix both halves in
  passing, to the "any emits / more than one may emit" phrasing `board-checks.md` already uses.
- `related: [200, 222]` was recorded during this groom: both edit `scripts/board-checks.sh` and its
  test suite concurrently. Neither gates this work; expect a rebase compose, and do not hardcode
  fixture ids (`tests/test_board_checks.sh` was at a max of 248 and is moving).
