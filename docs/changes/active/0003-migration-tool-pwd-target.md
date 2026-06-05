---
id: 3
slug: migration-tool-pwd-target
title: migrate-to-docket.sh targets the invoking repo ($PWD) — usable for consuming repos
status: implemented
priority: medium
created: 2026-06-04
updated: 2026-06-04
depends_on: []
related: [2]
adrs: []
spec: docs/superpowers/specs/2026-06-04-migration-tool-pwd-target-design.md
plan: docs/superpowers/plans/2026-06-04-migration-tool-pwd-target.md
results: docs/results/2026-06-04-migration-tool-pwd-target-results.md
trivial: false
branch: feat/migration-tool-pwd-target
pr: https://github.com/danielhanold/docket/pull/4
blocked_by:
reconciled: true
---

## Why

`migrate-to-docket.sh` (from change 0002) can only migrate the repo it physically lives in: it opens with `cd "$SCRIPT_DIR"`, and it is not distributed to consuming repos (the *skills* are symlinked globally; the script is not). So migrating any **other** repo to docket-mode can't use it — `~/dev/obsidian-wiki` had to be migrated by manual staged git steps, and one step (the `.gitignore` `.docket/` entry) was nearly missed. The migration tool should actually be usable on the consuming repos it exists to serve. (See memory `docket-migration-tool-not-distributed`.)

## What changes

Retarget the script to operate on **the git repo containing the invocation directory (`$PWD`)** — resolve it with `git rev-parse --show-toplevel` instead of `cd "$SCRIPT_DIR"` — so `cd <target-repo> && bash ~/dev/docket/migrate-to-docket.sh` migrates *that* repo. Because it now acts on whatever repo you're standing in (branch surgery + prune + push), add a **confirmation prompt** (printing the resolved target repo, read from `/dev/tty`) with a `--yes`/`-y` bypass for automation. Reachability stays minimal — invoke docket's single copy by absolute path; no global install. Touch-points: the script, a `tests/` assertion, and the README migration usage. Full design + the exact resolution/prompt mechanics in the linked spec.

## Out of scope

- **Distributing** the script globally (symlink / PATH install) or turning migration into a **skill** — reachability stays "invoke by absolute path"; revisit as a separate change if that proves painful.
- **Auto-running** migration — the refuse-and-migrate bootstrap guard (ADR-0002) stands; the guard still STOPs and points to the script.
- The migration's **behavior** (seed/prune sets, idempotency, `ls-tree` probes) — unchanged.

## Open questions

None — the design is settled (target via `--show-toplevel`; confirm-from-`/dev/tty` with `--yes` bypass).

## Reconcile log

**2026-06-04:** Reconciled at claim time — a currency check, not a rewrite (spec + change authored today; `origin/main` carries `migrate-to-docket.sh` as merged by 0002; `origin/docket` advanced only by this change's own commits). Verified against current code + reality:
- **Current `migrate-to-docket.sh` matches the spec's assumptions** — it opens with `cd "$SCRIPT_DIR"` and `SCRIPT_DIR` is used nowhere else, so the §3 retarget (swap to `git rev-parse --show-toplevel`) is a clean one-line change; no `--yes` flag or confirm prompt exists yet.
- **A third repo hit the gap:** `~/dev/markhaus` was migrated 2026-06-04 via a `/tmp`-patched-`cd` workaround (copy the script, patch its one `cd` line to the target path) — exactly the manual pain 0003 removes. Native `$PWD` targeting supersedes patching the `cd` line. Motivation strengthened; design unchanged.
- **Adjacent gaps stay OUT of scope:** the markhaus run also showed the script does not flip `.docket.yml` (`metadata_branch: main → docket`) nor fast-forward the local integration branch after its transient-worktree push. Real, but **separate** from 0003's targeting+confirm scope — leave to a future change; do not expand 0003.

Scope otherwise unchanged. Build approach: TDD-for-docs/script — extend `tests/test_docket_metadata_branch.sh` with the §4 assertions (`git rev-parse --show-toplevel` present; no `cd "$SCRIPT_DIR"`; `--yes`/`-y` bypass), then the one-line target swap + flag-parse + `/dev/tty` confirm prompt, and the README usage update. `bash -n` clean; exercise from a subdir of a throwaway repo at build.
