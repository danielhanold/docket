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
auto_groomable: true
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
