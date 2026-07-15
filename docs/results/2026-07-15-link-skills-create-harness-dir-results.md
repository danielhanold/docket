# link-skills.sh creates a missing skills subdir when the harness is present — results

Change: #80 · Branch: feat/link-skills-create-harness-dir · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-07-15-link-skills-create-harness-dir.md · ADRs: none

## Verify (human)

Fully covered by automated tests — no manual checks are required at the merge gate. `tests/test_link_skills.sh` (13 assertions) exercises all three harness states (present+populated, present+missing-subdir, fully-absent) plus the new non-directory-skills-path resilience case; `tests/test_install.sh` still green; the whole 41-file suite passes. The new guard was mutation-tested in both directions (revert the fix → the load-bearing assertions redden). One optional courtesy check:

- [ ] Skim the README **install primitives** section on GitHub — the reworded `link-skills.sh` bullet ("creating the `skills/` subdirectory when the harness itself is present but that subdirectory is missing") reads correctly and renders.

## Findings

- **No ADR** — a mechanical guard fix; the install/linking design is untouched and no non-obvious decision was made.
- **Whole-branch review caught + fixed an `set -e` abort regression (Important).** The first-cut fix (`[ -d "$dir" ] || mkdir -p "$dir"`) would, under `set -euo pipefail`, abort the ENTIRE script the moment a harness's `skills` path pre-existed as a non-directory (a stray file, or a dangling symlink so `[ -d ]` is false) — leaving a partial install across *all* harnesses, where the pre-0080 guard had skipped just that one harness. The reviewer reproduced both triggers empirically. Fixed to `[ -d "$dir" ] || mkdir -p "$dir" || continue` (commit `8144537`) and covered by a mutation-confirmed regression test (`.kiro` given a regular file at its skills path; assert a *later* harness `.windsurf` still links and the run exits 0). Re-review: ready to merge.
- **Prose accuracy shipped with the behavior.** The change made two existing statements false; both corrected in the same fix commit: the README `link-skills.sh` bullet ("only writes into harness directories that already exist") and a `tests/test_install.sh` comment ("leaf-checks `<root>/.claude/skills`" → parent-checks and creates `skills/`).

## Follow-ups

- **Hoist the harness-validity check out of the inner skill loop (deferred Minor).** The parent-dir check + `mkdir -p` currently run once per `(skill, harness)` pair rather than once per harness. It is idempotent and correct (later skill iterations short-circuit on `[ -d "$dir" ]`), so this is pure cleanup — but on the non-directory-skills-path failure path it re-emits the `mkdir` stderr once per skill; hoisting the check to a per-harness pre-pass would both de-duplicate the work and silence that repeated noise. Left as a follow-up to keep this change minimal.
