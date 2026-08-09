---
id: 208
slug: runner-dispatch-worktree-gate-3-proves-repo-containment-not
title: Harden runner-dispatch — worktree membership gate, feature-scoped coverage, flag-parse guards
status: proposed
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-09
depends_on: [237]
related: [209, 210, 220, 237, 270, 274]
discovered_from: [206]
adrs: []
spec: docs/superpowers/specs/2026-08-07-runner-dispatch-worktree-gate-3-proves-repo-containment-not-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-runner-dispatch-worktree-gate-3-proves-repo-containment-not-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-runner-dispatch-worktree-gate-3-proves-repo-containment-not-design.md) |
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

Settled design in the linked spec; the shape:

Membership gate (a):

- Gate 3 becomes a real membership test: one `git -C "$ANCHOR" worktree list --porcelain` capture
  (into a variable, never piped into `grep -q` under `pipefail`); same-repo is the **first** line
  equalling `$REPO_ROOT` (an anywhere-match admits stale foreign-list records), membership an
  exact `worktree $ANCHOR` line; `$ANCHOR` is `pwd -P`-normalized first (git prints physical
  paths). Diagnostic wording now matches what is verified.
- Feature-scoped agents additionally reject `ANCHOR == REPO_ROOT` with a diagnostic naming the
  integration-branch hazard.
- Close the paired test gap: a success-path leg — `build-*` *with* a real `--worktree` succeeding
  and anchoring at the named tree (fixture worktrees become real `git worktree add` members).

Feature-scoped coverage (b):

- The gate keys on a **declared agent scope**: a required `worktree-scope: feature|metadata`
  frontmatter key on every built-in agent source, validated loudly by `sync-agents.sh` at
  generation, read tolerantly by the facade at runtime via a new `${AGENTS_SRC:-}` seam. No name
  list anywhere. Feature set: the four `build-*` profiles, `rebase-resolver`,
  `integration-repair`, the three `review-*` rungs.
- `emit_shim`'s required `--worktree` slot keys on the same declaration with one generalized rule
  text; metadata shims stay byte-identical (0206's bidirectional guard, now scope-keyed).
- The dispatcher skills of the newly gated set (`docket-implement-next` §6 review dispatch,
  `docket-finalize-change` gate prose) each gain the docket-build-shaped sentence naming the
  feature worktree in the dispatch payload — without it every delegated dispatch of the widened
  set would abort on the shim's no-worktree rule.
- One new ADR (scope is a declared fact; gates key on the declaration) plus a dated `## Update`
  on ADR-0068, both ids delivered via this change's `adrs:`.

Flag-parse guards (c):

- Guard the value before consuming it —
  `--worktree) [ $# -ge 2 ] || die "--worktree requires a value"; WORKTREE="$2"; shift 2 ;;` —
  same shape at all five flag sites, site list derived from a grep of the parse loop.
- A test leg per flag asserting a trailing valueless flag exits nonzero rather than hanging,
  bounded by a background+poll+kill helper (no `timeout(1)` on stock macOS).

## Out of scope

- Any change to what the flags mean or how their values are resolved; arg-shaped flag values
  (`--model --effort high`) — a wrong-value problem, not a hang.
- The `sync-agents.sh` gate findings from change 0207's review — change 0220's territory, being
  groomed separately (the `worktree-scope` validation added here is a new gate, not one of
  0220's findings).
- The facade's exec→call-and-return tail rewrite — change 0237's territory; build after 0237
  merges (same file, disjoint regions).

## Consolidation note

2026-08-05: absorbed changes 0209 and 0210 (both killed pointing here). The original stubs'
mutual "its own change" out-of-scope lines reflected per-stub scoping at mint time and are
superseded by this merge.

2026-08-09: absorbed change 0274 (killed pointing here) — it re-discovered leg (c) from 0271's
build, independently of 0210. Fresh evidence it contributed: measured, not inferred —
`timeout 3 bash scripts/runner-dispatch.sh --runner` returns 124 (observed 2026-08-09). The
spec's leg (c) already prescribes the identical fix and per-flag hang-regression tests; no spec
change needed.
