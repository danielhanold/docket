---
id: 208
slug: runner-dispatch-worktree-gate-3-proves-repo-containment-not
title: Harden runner-dispatch — worktree membership gate, feature-scoped coverage, flag-parse guards
status: proposed
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: [209, 210]
discovered_from: [206]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Consolidates three fix stubs from change 0206's whole-branch review of the same script —
this one plus killed changes 0209 (feature-scoped families ungated) and 0210 (valueless
trailing flag hangs the parse loop). All three harden `runner-dispatch.sh`'s input gates and
share one test suite; one change avoids three conflicting PRs against the same file.

**(a) Gate 3 proves containment, not membership (original 0208).** Change 0206's whole-branch
review found that `runner-dispatch.sh`'s third `--worktree` gate proves **containment in the
repo**, not **worktree membership**, so it does not reject the one value it most needs to.

`docket_main_worktree` returns the main worktree for *any* directory inside *any* worktree of the
repo. The gate is:

    [ "$(docket_main_worktree "$ANCHOR")" = "$REPO_ROOT" ] || die "--worktree $ANCHOR is not a worktree of this repository"

so it succeeds for the main worktree itself, and for every ordinary subdirectory of it
(`<repo>/docs`, `<repo>/scripts`, ...). A `build-*` delegation whose caller supplies the repo root
therefore clears all three gates and anchors the build worker in the primary checkout on the
integration branch — precisely the failure 0206 exists to eliminate — while the diagnostic asserts
a worktree membership that was never actually checked.

This is 0206's own defect class reappearing inside the code that fixes it (learnings:
`fix-reintroduces-its-own-defect-class`): the gate makes an **omission** loud but leaves a **wrong
value** silent, and the design only accepted that trade because the value is prose-supplied.

**(b) The `--worktree` requirement covers `build-*` only (absorbed from 0209).** 0206's spec
states the decision generally — *"Feature-scoped agents must name their tree, and the facade
refuses to run one that did not"* — but the implementation covers one family. Three more
delegatable families are equally feature-scoped, two of which **commit**:

- `docket-rebase-resolver` — runs `git add` + `git rebase --continue` on an in-progress rebase,
  which by git's own rules is in the feature worktree (the main tree cannot have `feat/<slug>`
  checked out).
- `docket-integration-repair` — "writes a minimal fix", commits.
- The `docket-review-*` trio — read-only, but a wrong tree means findings about the wrong diff,
  silently.

None matches `build-*`, so a `runner:` on any of them still yields the silent
main-tree-on-the-integration-branch anchor 0206 exists to eliminate, under whatever auto-approve
grant the runner carries.

**(c) A valueless trailing flag hangs the parse loop forever (absorbed from 0210).** Every flag
parses as `--flag) VAR="${2:-}"; shift 2 ;;`. When the flag is the **final** argument with no
value, `shift 2` with `$# = 1` returns non-zero and shifts **nothing**; under `set -uo pipefail`
with no `-e`, the `while [ $# -gt 0 ]` loop never terminates. The facade's failure posture
everywhere else is a loud `die`; here it is a hang — under a foreground generated shim invoked
with `timeout 600000`, ten minutes of the caller's budget with no diagnostic. Pre-existing for
the four older flags (`--runner`, `--agent`, `--model`, `--effort`); 0206 added the fifth
(`--worktree`).

## What changes

Membership gate (a):

- Test real membership rather than containment: capture `git -C "$ANCHOR" worktree list --porcelain`
  into a variable (never pipe into `grep -q` under `pipefail`) and require an exact
  `worktree <ANCHOR>` line.
- For `build-*` specifically, reject `ANCHOR == REPO_ROOT` with a diagnostic naming the
  integration-branch hazard.
- Align the gate's wording with what it actually verifies.
- Close the paired test gap the same review raised: no assert covers the change's central success
  path — a `build-*` agent *with* `--worktree` succeeding and anchoring at the named tree. Legs (b)
  and (c) use `--agent status`; leg (d) uses `build-economy` only in its rejected state. A mutation
  making `build-*` abort unconditionally leaves the suite green.

Feature-scoped coverage (b):

- Widen the facade's gate from the `build-*` shape to the feature-scoped set, or better, key it on
  a **declared agent scope** rather than a name list — a name list is an enumerated floor that ages
  into the gap it was written to close.
- Add the corresponding `sync-agents.sh` `emit_shim` required slot for the same set.
- Keep the generation guard bidirectional, as 0206 established.

Flag-parse guards (c):

- Guard the value before consuming it, e.g.
  `--worktree) [ $# -ge 2 ] || die "--worktree requires a value"; WORKTREE="$2"; shift 2 ;;`
- Apply the same shape to all five flags — derive the site list from a grep of the parse loop, not
  by hand.
- Add a test leg per flag asserting a trailing valueless flag exits nonzero rather than hanging
  (bound it with a timeout so a regression fails fast instead of wedging the suite).

## Out of scope

- Any change to what the flags mean or how their values are resolved.
- The `sync-agents.sh` gate findings from change 0207's review — change 0220's territory, being
  groomed separately.

## Consolidation note

2026-08-05: absorbed changes 0209 and 0210 (both killed pointing here). The original stubs'
mutual "its own change" out-of-scope lines reflected per-stub scoping at mint time and are
superseded by this merge.
