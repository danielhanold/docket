<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0298 — Stacked changes — build a new change on top of a parent change's branch](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-12-0298-stacked-changes-build-a-new-change-on-top-of-a-parent-change.md)**
<!-- docket:backlink:end -->

# Stacked changes — build a new change on top of a parent change's branch — results

**Change:** 0298 · **Date:** 2026-08-12 · **Type:** feat
**ADR produced:** [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md)
**Follow-up minted:** #0300 · **Minted then killed as a duplicate:** #0299 (see *Follow-ups*)

## What shipped

The `stacked_on:` subsystem, as specified: a new optional manifest field, a shared effective-base
resolver (`scripts/lib/docket-stack.sh`), the `stacked-merged` lifecycle status, an idempotent stack
close-out (`scripts/stack-closeout.sh`) driven from both merge sites, board rendering for the
waiting / awaiting-root / rebase-pending states, two health checks, and `verify-run` treating
`stacked-merged` as a completed run. Eleven plan-task commits, then eight fix commits from review.

Two surfaces exist that the plan did not name, both added by review fixes:

- **`scripts/stack-children.sh`** (+ contract, + `docket.sh stack-children`) — the descendant scan
  exposed as a CLI, because the finalize open-children gate needed a live oracle and had none.
- **`sweep_stack_recovery`** in `scripts/docket-status.sh` — the pass that re-drives a stack
  close-out that failed on an earlier sweep.

## Verification

Full suite green at `d3f2b7c1`: `SUITE files=112 passed=112 failed=0 asserts=9259 wall=198s`.

The one `OVER BUDGET:` line is `test_sync_agents_runners` (198s against a 60s ceiling). That file is
**not touched by this change** and the breach is pre-existing — it is exactly what change **0280**
tracks. It is not a finding against this branch.

Three suite runs were spent in total: one to re-mint a build-evidence record that did not survive a
session resume, then the fix loop's gate (red), then the gate's re-run after a revert (green).

## Review

Reviewer rung **`docket-review-deep`**. The rung was selected by rule, not judgement: no build-evidence
record survived the session resume, so the base rung defaulted to `standard`, and the whole-branch
diff exceeded 1500 changed lines, which bumps one step.

Eight findings: 2 blocker, 3 important, 3 minor. Seven fixed, one reverted. The full disposition
table is in the PR body; what follows is what a reader of the merged history should know.

### The two blockers were both "the spec's second half was never wired"

Neither was a coding error. Spec §7 names **two** invokers of the stack close-out and only the status
sweep was wired, so a stack root merged through `docket-finalize-change` — the primary human
path — stranded every descendant permanently, with no sweep able to recover it (an archived root is
never re-enumerated). Spec §11 requires the finalize open-children gate to derive its child set by
scanning, and the scan existed but was reachable only from inside `stack-closeout.sh`; the only
parent-side artifact was a rendered row that is **absent in exactly the motivating case**, because
`render-change-links.sh` runs on a write to the *parent*, and a child stacked on an already-
`implemented` parent is created after the parent's last such write.

Both are the same failure shape: a contract satisfied at one of its two ends. Worth remembering that
the tests were green throughout — nothing in a hermetic suite asks "is this prose gate reachable?"

### One decision deviates from the accepted spec (ADR-0092)

Spec §3 rule 2 lumps a `done` parent together with a `stacked-merged` one and resolves both by
recursing to the parent's own effective base. That is right for `stacked-merged` and wrong for
`done`: docket has **two** merge destinations, and `done` means the code reached the *integration
branch*, while `stacked-merged` means it reached the *parent's branch*. Recursing on `done` returns a
grandparent branch cut before that merge — a base missing its own parent's work, the exact failure
stacking exists to prevent.

The code now resolves a `done` parent to the integration branch directly. **The spec and the plan
were deliberately left as written** — they are point-in-time records — so ADR-0092 is the reconciling
document, and a future reader who diffs code against spec §3 should read it before "fixing" the
divergence back.

A second-order consequence, pinned by test: a `done` parent whose own parent is `killed` used to exit
3 and drop the child out of the ready queue. It now resolves correctly, because the `done` parent's
merge already made the killed ancestor irrelevant.

### The cross-cutting defect the build flagged

`stack_effective_base` issued its one `git show-ref --verify` with no `-C`, so it answered "is the
parent's branch pushed" from the **caller's cwd** rather than the repo under `--changes-dir`. Only
`board-checks.sh` had been hardened, via a local wrapper. Because that ref lookup is the *positive*
conjunct of the resolver's rule 1, a failed lookup is indistinguishable from "not pushed" — so the
symptom is not an error but a silent one: every stacked change renders `stack base not built` and
drops out of the ready queue.

Fixed in the library (`"$git" -C "$dir" …`), with `board-checks.sh`'s wrapper removed **in the same
commit** — `git -C a -C b` composes *relatively*, so leaving both would have broken on a relative
`--changes-dir`.

Two git stubs had to change with it, and this is the part worth carrying forward: both matched on
`$1 = show-ref` and would have fallen through their catch-all `exit 0` once `-C` arrived — reporting
**every** branch as pushed. Adding a flag to a mocked call can turn a whole fixture vacuously green
rather than red.

### One fix was reverted by the suite gate

Finding 7 (the convention doc re-arming hardcoded lifecycle cardinalities — "eight states" — in the
same change that stripped them from every script) was fixed at `89937389` and **reverted at
`d3f2b7c1`**. Its guard used a `\\b` word-boundary spelling, which `tests/test_grep_portability.sh`
rejects: BSD grep and git-grep ERE return zero for it *silently*, so the guard was vacuous and the
gate was right to redden.

The finding therefore **stands, unfixed**: `skills/docket-convention/SKILL.md` still says
"eight states" and `github-board-mirror.md` still says "(all eight)". A ninth lifecycle status will
make both lie. This is cheap to redo — the prose edit is three words — but it needs a guard written
with an explicit `[^[:alnum:]_]` class, not `\b`.

## Manual checks for the merge gate

The suite is hermetic: it sees fixtures and the integration-branch checkout, never the metadata
branch or a real stack. Nothing below is covered by the 9259 green asserts.

1. **Cut a real stacked child.** Add `stacked_on: 298` to a scratch stub, run
   `docket.sh stack-base --id <n>`, and confirm it names this branch rather than `main`.
2. **`docket.sh stack-children --id 298`** — confirm it returns empty at exit 0 (no descendants) and
   exit 4 for an id that names no change.
3. **The board cells.** Confirm a stacked stub renders `waiting on #298` rather than an unresolved
   base, and that the killed-ancestor diagnostic names the ancestor it means.
4. **Do not merge this while a real child is open** — the open-children gate is the one path that
   deletes a parent branch, and deleting it closes the child's PR and loses its review history.

## Follow-ups

- **#0300** — close the single-backslash `\b` word-boundary gap. `test_grep_portability.sh` gates the
  two-backslash spelling but only *counts* the single-backslash one: 56 sites, "computed, not
  gating". Every one of them is a potentially vacuous guard on BSD, which is the defect class that
  reverted finding 7 above. Minted from this run.
- **#0280** — already owns the `OVER BUDGET` resharding work above; nothing new was filed for it.
  This run **did** mint a duplicate stub for it (#0299) because `mint-stub`'s dedup did not match
  0280's differently-worded title; #0299 was killed the same day and its terminal record published.
  Worth knowing that title-similarity dedup will miss a general-form parent of a specific discovery.
- **Finding 7 is unfixed**, per above — a candidate for whoever next touches the convention doc.

## Notable deviations from the plan

- Two scripts and one sweep pass exist that the plan did not name; all three came from review fixes
  closing spec-mandated behavior the plan had not broken out into tasks.
- The `## Stacked children` rendered row's "drift-free by construction" claim was **honestly
  weakened** rather than fixed: it is now documented as a human view that can lag a later child.
  Nothing that decides reads it — the gate reads the live scan instead.
- `tests/test_frontmatter_read_shapes.sh`'s census went red mid-fix-loop because two fix commits
  introduced new `field status` call sites; the three sites were classified (not repaired — all read
  a template-guaranteed key with the correct unanchored helper) and the census is green.
