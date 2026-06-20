---
id: 30
slug: script-adr-passes
title: Script docket-adr's deterministic passes — index render, ledger checks, ADR-only publish
status: in-progress
priority: high
created: 2026-06-20
updated: 2026-06-20
depends_on: []
related: [22, 23, 25, 26]
adrs: [2, 7, 12]
spec: docs/superpowers/specs/2026-06-20-script-adr-passes-design.md
plan: docs/superpowers/plans/2026-06-20-script-adr-passes.md
results:
trivial: false
auto_groomable:
branch: feat/script-adr-passes
pr:
blocked_by:
reconciled: true
---

## Why

The scripting sweep (changes 0022/0023/0025/0026) lifted every deterministic pass
out of the `docket-status` / `docket-finalize-change` family into tested shell,
governed by **ADR-0012**'s rule: script a pass when it is **mechanical** (no
reading intent from free text) and **free of terminal-transition side effects
shared across callers** (or, when shared, owned by *one* jointly-owned script).
`docket-adr` is the one operating skill that sweep never reached — three of its
passes are still model-prose even though each passes ADR-0012's test cleanly. This
brings `docket-adr` to parity.

Each of the three has a scripted sibling that already exists, so this is faithful
re-implementation, not new design:

- **ADR index render** — `<adrs_dir>/README.md` literally says *"This index is
  generated — do not hand-edit"* yet has no generator; the model re-derives it from
  prose every run. The cost shows as **drift**: the published copy on `main`
  currently lists only ADR-0001/0002 while the authoritative copy on `docket` lists
  all twelve. The exact analog of `render-board.sh` (0022).
- **ADR ledger validation** (numbering gaps, dangling links, status
  inconsistencies) — deterministic file probes, the exact shape of `board-checks.sh`
  (0023): warn-only, one finding per line, `--strict` for CI.
- **ADR-only terminal-publish** (`T = adr-<NN>`) — the **last hand-run git sequence
  embedded in a skill body**. `terminal-publish.sh` (0025) is keyed on `--id`, so
  the single-ADR copy path is run by the literal bash in `docket-finalize-change`'s
  *The mechanics* section, even though its provision→copy→CAS-push→teardown
  mechanics are byte-for-byte the scripted change-publish path. ADR-0012 says own
  shared terminal-transition mechanics in one script, never duplicate per-caller.

## What changes

Per [the spec](../../superpowers/specs/2026-06-20-script-adr-passes-design.md):

- **`scripts/render-adr-index.sh`** — new; emits `<adrs_dir>/README.md` to stdout,
  offline + deterministic (same ADR files ⇒ byte-identical), reproducing the three
  index groups (Active / Superseded·Reversed / Deprecated) and the
  `← change #N` / `→ supersedes ADR-NN` / `· relates to ADR-NN` annotations exactly.
  Reuses `lib/docket-frontmatter.sh`.
- **`scripts/adr-checks.sh`** — new; the ADR-ledger analog of `board-checks.sh`.
  TAB-separated `<check-id>\t<adr-id>\t<message>` findings, warn-only, `--strict`
  exit code. Checks: `adr-numbering-gap`, `adr-dangling-link`,
  `adr-status-inconsistent`.
- **`scripts/terminal-publish.sh`** — add an **`--adr <NN>`** mode (alternative to
  `--id`): copy-set = the single ADR file, step-1 archive skipped, throwaway branch
  `pub-adr-<NN>`, no `Accepted` gate (caller decides — including a status-line
  flip). Reuses the existing CAS-push / self-verify / teardown / main-mode-no-op
  machinery so it is the single executor of both publish shapes.
- **Wire the skills** — `docket-adr`'s *Index / validate* invokes the two new
  scripts (keeping the separate-commit + regenerate-don't-3-way-merge discipline and
  the human-readable check descriptions); `docket-adr`'s and
  `docket-finalize-change`'s ADR-only-publish prose names
  `terminal-publish.sh --adr` instead of a by-hand git block.
- **A new ADR** recording that ADR-0012's script-vs-model boundary now extends to
  the `docket-adr` surface — authored at build time and appended to this change's
  `adrs:`.
- **Tests** — golden + idempotence for the index render; one fixture per check
  (plus a clean-ledger / `--strict` assertion) for the validator; a hermetic
  `--adr` publish case in the terminal-publish harness, with the existing `--id`
  tests as the regression gate.

## Out of scope

- **ADR or board *semantics*** — index grouping, row format, the three checks, and
  the publish mechanics are reproduced exactly, never redesigned.
- **The metadata-worktree ensure-and-sync ritual** (the cross-cutting de-dup
  identified alongside this) — a separate extraction.
- **Next-id allocation** — stays in-model (welded to the CAS-push-rename retry); at
  most a future `lib/` helper, not this change.
- **`yq`** (0018) — these passes read flat frontmatter only; `field`/`list_field`
  already cover them (same finding as 0022). `related`, not `depends_on`.
- **Rewriting the stale `main` ADR index history** — the next normal index-render
  pass heals it; no manual back-fill.

## Open questions

Resolved at brainstorm 2026-06-20 — see the spec. Settled there: mirror the board
pair (two new scripts + extend `terminal-publish.sh`, not a fold into
`board-checks.sh` or a combined `adr.sh`); full scope includes the skill wiring and
a new ADR; priority high. None blocking — build-ready.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-06-20 — reconcile (build start)

Reconciled against current `origin/main` (tip `71074ef`, last change #0029, no
scope overlap) and `origin/docket`. Verdict: **no rescope, no obsolescence** —
the design is faithful and current. Verified against reality:

- **Drift still present** (the motivation holds): the published ADR index on
  `main` (`docs/adrs/README.md`) lists only ADR-0001/0002, while the authoritative
  copy on `docket` lists all twelve. The new `render-adr-index.sh` heals this on
  the next index pass, exactly as the spec states.
- **All sibling scripts exist at the assumed shapes** — `scripts/render-board.sh`
  (`--changes-dir`), `scripts/board-checks.sh`, `scripts/terminal-publish.sh`
  (keyed on `--id`; `--id` is currently a hard requirement at the arg-parse, which
  the `--adr` mode relaxes to "exactly one of `--id`/`--adr`"), and
  `scripts/lib/docket-frontmatter.sh`. The two new scripts and the extension fit
  these unchanged.
- **The prose to retire is intact** — `docket-adr`'s *Index / validate* section
  (hand-render + ledger-validation prose) and its two ADR-only-publish references;
  `docket-finalize-change`'s *Terminal publish (docket-mode)* → *The mechanics*
  by-hand ADR-only block.
- **Related changes 0022/0023/0025/0026 are all `done`** (archived) and none
  pre-empted this work. `tests/` uses the `test_*.sh` convention, so the new
  `tests/test_render_adr_index.sh` / `tests/test_adr_checks.sh` and the
  `terminal-publish.sh --adr` harness addition match existing naming.

No body or spec edits required; proceeding to plan + build.
