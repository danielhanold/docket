---
id: 3
slug: migration-tool-pwd-target
title: migrate-to-docket.sh targets the invoking repo ($PWD) — usable for consuming repos
status: proposed
priority: medium
created: 2026-06-04
updated: 2026-06-04
depends_on: []
related: [2]
adrs: []
spec: docs/superpowers/specs/2026-06-04-migration-tool-pwd-target-design.md
plan:
results:
trivial: false
branch:
pr:
blocked_by:
reconciled: false
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
