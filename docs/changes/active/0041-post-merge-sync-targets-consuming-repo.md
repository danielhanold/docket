---
id: 41
slug: post-merge-sync-targets-consuming-repo
title: Post-merge integration sync fast-forwards the docket clone, not the consuming repo where the merge landed
status: proposed
priority: medium
created: 2026-06-23
updated: 2026-06-23
depends_on: []
related: [29, 34]
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

The post-merge integration sync (`sync-integration-branch.sh`, change 0029) exists to
fast-forward "the clone's local `<integration_branch>` checkout to the tip the merges just
pushed," so the primary checkout does not drift after a docket merge. It works when docket
dogfoods itself — there the consuming repo *is* the docket clone, so the script's default
target (`--clone-dir` = `dirname "$0"/..`, the clone the script physically lives in) is the
same repo the merge landed in.

In a **separate-clone install** — the model ADR-0014 established, where docket lives in its
own clone and consuming repos reach its scripts via `DOCKET_SCRIPTS_DIR` and its skills via
symlink — that default resolves to the **docket clone**, never the consuming repo. So when
finalize (or the `docket-status` sweep) runs in a consuming repo:

- it fast-forwards the **docket** clone's integration branch — a no-op, because a
  consuming-repo PR merge never advances docket's own `origin`; and
- the **consuming repo's** integration checkout — the one the merge actually pushed to — is
  never advanced, so it silently drifts behind `origin/<integration_branch>`.

The finalize and sweep sites invoke the helper bare (`sync-integration-branch.sh
--integration-branch <integration_branch>`, no `--clone-dir`), so they always inherit that
default and have no way to retarget it.

**Evidence (markhaus, 2026-06-23).** Finalizing change #53 merged PR #39 to markhaus's
`origin/main`. The end-of-run sync reported `main already current (b5c5902…)` — but
`b5c5902` is the *docket* clone's `main` tip, not a markhaus object; markhaus's local `main`
stayed 6 commits behind `origin/main`. Change 0029's spec foresaw this edge ("Consumer-repo
skill freshness … the skills clone is a separate repo a project merge doesn't advance") but
scoped it out, leaving the consuming repo's own integration checkout unhandled.

## What changes

PM-altitude target (detailed design deferred to grooming): make the post-merge sync
fast-forward the integration branch of the **repo where the merge landed** — the consuming
repo (the skill's CWD git toplevel) — instead of the docket clone the script happens to live
in. Likely shape, to be settled at brainstorm:

- Have the finalize and `docket-status`-sweep sites target the consuming repo explicitly
  (e.g. pass `--clone-dir "$(git rev-parse --show-toplevel)"`), and/or change
  `sync-integration-branch.sh`'s default from `dirname "$0"/..` to the CWD's git toplevel.
- Preserve the existing **best-effort, FF-only, triple-gated** posture (on-branch + clean +
  true-FF; every skip a normal exit-0 that never aborts the close-out).
- Keep dogfooding (docket-on-docket) working unchanged — there the CWD toplevel and the
  script's own clone are the same repo.

## Out of scope

- **Keeping the docket *skills* clone fresh in consuming repos** — that is the separate
  "update docket" workflow change 0029 named; skills load from the docket clone regardless of
  the consuming repo's checkout. This change concerns the consuming repo's *own* integration
  checkout, not skill bytes.
- Auto-restart or fixing the in-flight session (0029's out-of-scope stands).
- Changing *which* sites run the sync, or the gate conditions themselves.

## Open questions

- **Where does the retarget belong — the skill (pass `--clone-dir`) or the script (default to
  CWD toplevel)?** The script-owns-*how* / skill-owns-*when* boundary (ADR-0007) and the
  consuming-repo resolution model (ADR-0014) both bear on the choice.
- **Should a consuming-repo finalize also refresh the docket skills clone**, or stay strictly
  the consuming repo's integration FF (keeping the "update docket" workflow separate)?
- **Does the clean-tree gate need any carve-out for consuming repos?** markhaus had an
  untracked `design/` directory that would skip even a correctly-targeted FF — confirm
  skip-and-note is still the right call there rather than a surprise.

## Reconcile log

## Auto-groom blocked

### 2026-06-23 — autonomous groom abstained

A default-biased self-brainstorm designed this change and an adversarial critic gated the
draft. Four of five design decisions cleared as **sound** (independently re-verified); one is
**human intent the groom cannot default**, so no spec was emitted and `auto_groomable` is
flipped to `false`. The settled work is recorded below so an interactive groom (or the author)
starts from it rather than re-deriving it.

**The undecidable decision — does fixing this change require also addressing gate 2 (the
dirty-tree skip) for consuming repos?**

The targeting fix (below) makes the post-merge sync resolve the **consuming repo's main
worktree** instead of the docket clone. But `sync-integration-branch.sh`'s **gate 2** skips on
*any* `git status --porcelain` output — including untracked, non-ignored files. The markhaus
evidence that motivated this change records an **untracked `design/` directory** in markhaus's
primary checkout. So even with correct targeting, gate 2 still trips on that untracked dir and
the FF still skips — meaning **the targeting fix alone leaves markhaus's reported drift (local
`main` 6 commits behind `origin/main`) unresolved.**

The stub contradicts itself here and the contradiction is material:
- **Out of scope** says "Changing … the gate conditions themselves" — forecloses touching gate 2.
- **Open questions** reopens exactly that: "Does the clean-tree gate need any carve-out for
  consuming repos? markhaus had an untracked `design/` directory that would skip even a
  correctly-targeted FF — confirm skip-and-note is still the right call there rather than a
  surprise."

Two defensible intents diverge on whether the change is even *complete*:
1. **Ship targeting-only** — the dirty-tree skip is correct conservatism; markhaus should
   gitignore / clean `design/`; the gate stays untouched.
2. **Stop the drift fully** — the change's whole point is that the consuming repo stops
   drifting; a fix that leaves markhaus still drifting is incomplete, so gate 2 must also be
   addressed for consuming repos (and *how* — stash-and-FF, ignore-untracked in the FF
   decision, or a louder warn — is itself an unsettled design choice the author scoped out).

Resolving #2's "how" is a second design decision, not a mechanical default; and choosing
between #1 and #2 is a product-completeness call the author reserved by attaching the markhaus
evidence to the open question. An autonomous groom cannot supply that intent.

**What a human should supply.** Answer: *is a fix that demonstrably leaves the reporting repo
(markhaus) still drifting an acceptable resolution of this change?* If **yes** → re-arm as the
targeting-only design below (it is build-ready). If **no** → the change must also settle the
gate-2 behavior for consuming repos before it is build-ready (decide the approach), and the
"don't change the gate" line in Out-of-scope should be revised accordingly.

**Recommendation.** Not a kill or defer — the change is valid and worth doing. Re-arm by
answering the gate-2 question (flip `auto_groomable` back to `true` and **delete this section**
once answered), or groom it interactively (`docket-groom-next`), where this is now first in the
needs-you band.

**Settled design (sound per the critic; reuse, do not rebuild):**
- **Retarget belongs in the script's default, not a skill-passed `--clone-dir`.** Change
  `sync-integration-branch.sh`'s default `--clone-dir` from `dirname "$0"/..` (the repo the
  script lives in) to the **main worktree of the repo it was invoked from (CWD)**. Call sites
  stay bare; explicit `--clone-dir` still overrides (the hermetic tests all pass it, so they are
  unaffected). Keeps deterministic git plumbing in the script per ADR-0012 / ADR-0007.
- **Resolution is the main worktree, NOT `git rev-parse --show-toplevel`.** At the sync site
  (finalize step 6 / after the status sweep) the shell CWD is typically a *linked* worktree
  (`.docket/`, on `docket`), so `--show-toplevel` returns the wrong path on the wrong branch and
  gate 1 would skip — the change body's first-named shape does **not** fix the bug (verified
  2026-06-23). Use `git worktree list --porcelain` first entry (≡ `--git-common-dir` parent),
  which resolves the integration-branch primary checkout from any worktree in the set,
  CWD-independent.
- **Dogfooding preserved with no special-casing** — when docket runs on itself, CWD's main
  worktree *is* the docket primary checkout, equal to the old `dirname "$0"/..`.
- **Skills-clone refresh stays out of scope** — the separate "update docket" workflow (per 0029).
- **Test note:** ADR-0014 makes the test suite the de-facto CI gate; a new bare-invocation case
  (from inside a linked worktree, asserting the **main** worktree is FF'd) is needed to lock the
  default — but a clean-tree fixture exercises only the happy path and would NOT surface the
  gate-2 gap above, so the test passing is not evidence the drift is fixed.
