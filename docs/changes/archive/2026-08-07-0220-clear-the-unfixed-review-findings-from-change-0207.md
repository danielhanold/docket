---
id: 220
slug: clear-the-unfixed-review-findings-from-change-0207
title: clear the unfixed review findings from change 0207
status: done
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-07
depends_on: []
related: [82, 140, 141, 207]
discovered_from: [207]
adrs: []
spec: docs/superpowers/specs/2026-08-05-clear-0207-review-findings-design.md
plan: docs/superpowers/plans/2026-08-06-clear-0207-review-findings.md
results: docs/results/2026-08-07-clear-the-unfixed-review-findings-from-change-0207-results.md
trivial: false
auto_groomable: true
branch: feat/clear-the-unfixed-review-findings-from-change-0207
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/164
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-05-clear-0207-review-findings-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-05-clear-0207-review-findings-design.md) |
| Plan | [2026-08-06-clear-0207-review-findings.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-06-clear-0207-review-findings.md) |
| Results | [2026-08-07-clear-the-unfixed-review-findings-from-change-0207-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-clear-the-unfixed-review-findings-from-change-0207-results.md) |
<!-- docket:artifacts:end -->

## Why

Change 0207's whole-branch review (`docket-review-deep`) returned 7 findings — 0 blocker,
3 important, 4 minor. Only blockers are auto-fixed, so all 7 rode into PR #159's body for
merge-time judgment and none were remediated on the branch. The three important ones are real
defects in shipped code, not style notes.

## What changes

Work the findings recorded in PR #159's review section. In the reviewer's own priority order:

**Important**

1. **The gate can under-enumerate on `--check`.** `validate_runner_config`'s project-level leg is
   guarded by `per_repo_opted_in`, but `check_project_level`'s leg (c) — the third `emit_wrapper`
   call site — is guarded by the strictly weaker `gitignore_block_wanted`. When the global config
   sets `agent_harnesses:` to a list without `claude`, a bad `runner:` in the global
   `agents.claude.*` block escapes the gate and dies inside leg (c) with the raw assertion instead
   of the intended `check:` summary, skipping the remaining `--check` legs and leaking leg (c)'s
   `mktemp -d`. The write path is genuinely safe, so this is diagnostic quality — but the in-source
   comment claiming no gap exists is false. Fix: share one predicate between the gate's
   project-level leg and leg (c), then correct the comment.

2. **The gate's user-level leg has no test.** Every `runner:` fixture in every suite lives in
   `.docket.yml` (the project layer), so the whole `for harness in $USER_TARGETS` block — including
   the `set -u` resolution added specifically for `--check` — is mutation-survivable: strip it and
   nothing reddens. This is the leg protecting `~/.claude/agents`, the widest blast radius of the
   original bug. Fix: a fixture writing `runner:` into `$SBX/.config/docket/config.yml` with no
   `.docket.yml`, asserting both the real run and `--check`.

3. **`user_flag_model`'s "spelled once" claim is false.** `emit_wrapper` does not call it; it keeps
   its own copy of the provenance filter, computed from positional `$2` rather than `$RES_MODEL`.
   They agree today only because all three call sites happen to pass `$RES_MODEL` — a convention
   nothing documents or enforces. A future call site passing a post-processed model silently
   reintroduces the mid-loop abort. Fix: either make the single spelling real, or document the
   `$2 == $RES_MODEL` contract on `emit_wrapper`'s header.

**Minor**

4. The `unregistered offender reports the registration rule` assert cannot distinguish the two
   rules — the runner name appears in both diagnostics, so swapping the `if` blocks leaves it
   green. (The ordering is genuinely pinned elsewhere by the ORDERING FENCE fixture.)
5. `validate_runner_config`'s "changes nothing on disk" wording is overbroad — `migrate_legacy_global`
   runs above the gate and renames the user's legacy `agents.yaml`. Scope the comment to wrappers.
6. A bad `runner:` in the global layer visible to both legs is reported twice, verbatim identically,
   against a README promising every offender "in one pass".
7. The `--check wrote no wrappers` assert was already true pre-0207 (leg (c) redirects into a
   `mktemp -d`), so it pins nothing about this change.

## Out of scope

- Re-litigating 0207's design. The atomicity invariant and the all-or-nothing strictness trade are
  settled (spec `2026-08-05-atomic-wrapper-generation-design.md`); these are defects in its
  execution.
- Gate 2's pre-migration blind spot — a documented boundary in 0207's own *Out of scope*.

## Reconcile log

### 2026-08-06

Re-verified all seven findings against `sync-agents.sh` on `origin/main` (tip `3565b749`). Every one
still reproduces, and the spec's decisions (D1–D6) still describe the code as it stands:

- **D1** — the gate's project-level leg is still `per_repo_opted_in`-guarded (line ~648) while
  `check_project_level` returns early on `gitignore_block_wanted` (line ~1258); the in-source comment
  at line ~1433 still asserts "so no gap". `gitignore_block_wanted` still `per_repo_opted_in || …`
  (line ~1104), so the strictly-weaker relationship holds. `prune_orphans`' legs are still
  `per_repo_opted_in`-gated (lines ~1353, ~1377), so D1's consistency argument stands.
- **D2** — no user-level (`$SBX/.config/docket/config.yml`) `runner:` fixture exists; every
  `runner:` fixture is still project-layer.
- **D3** — `user_flag_model` (line ~881) is still not called by `emit_wrapper`, which keeps its own
  `RES_MODEL_FROM_USER` spelling over positional `$2` (line ~919 onward).
- **D4/D5/D6** — the ordering assert, the "changes nothing on disk" comment, and the un-deduplicated
  double report are all unchanged.

Scope unchanged; no work has been done elsewhere. `depends_on` is still empty and 0207 is `done`.
The four `related` changes touching the same file (0082, 0140, 0141) are all still `proposed` and
unbuilt, so no rebase-over is needed now — whichever lands second rebases, as the spec assumed.

No follow-up work surfaced by this pass beyond what the spec already scopes.
