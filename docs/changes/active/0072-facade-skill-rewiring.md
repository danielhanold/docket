---
id: 72
slug: facade-skill-rewiring
title: Rewire the operating skills and Step-0 to the facade — retire the eval preamble
status: implemented
priority: medium
created: 2026-07-13
updated: 2026-07-14
depends_on: [68]
related: [68, 73]
adrs: [30]
spec: docs/superpowers/specs/2026-07-13-facade-skill-rewiring-design.md
plan: docs/superpowers/plans/2026-07-14-facade-skill-rewiring.md
results: docs/results/2026-07-14-facade-skill-rewiring-results.md
trivial: false
auto_groomable:
branch: feat/facade-skill-rewiring
pr: https://github.com/danielhanold/docket/pull/79
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-13-facade-skill-rewiring-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-13-facade-skill-rewiring-design.md) |
| Plan | [2026-07-14-facade-skill-rewiring.md](https://github.com/danielhanold/docket/blob/feat/facade-skill-rewiring/docs/superpowers/plans/2026-07-14-facade-skill-rewiring.md) |
| Results | [2026-07-14-facade-skill-rewiring-results.md](https://github.com/danielhanold/docket/blob/feat/facade-skill-rewiring/docs/results/2026-07-14-facade-skill-rewiring-results.md) |
| PR | [#79](https://github.com/danielhanold/docket/pull/79) |
| ADRs | [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md) |
<!-- docket:artifacts:end -->

## Why

Change 0068 gives docket a finite executable facade with read-not-eval config emission, but the
seven operating skills and the convention's Step-0 preamble still instruct agents to build the
old shapes: `eval "$(docket-config.sh --export)"`, inline worktree ensure + hook disable, inline
`fetch/pull`, and direct per-helper invocations. Until the prose moves, the facade is unused and
the permission surface is unchanged.

## What changes

- Rewrite the convention's *Step-0 preamble* to: run `docket.sh preflight` as its own Bash
  call, read the printed `KEY=value` block, and interpolate the values as literals in later
  commands — no `eval`, no `source`, no inline sync programs. `preflight` fails closed;
  `CREATE_ORPHAN` keeps `docket-config.sh --bootstrap` as the one sanctioned direct-helper
  spelling (byte-exact, convention-only — the facade deliberately doesn't expose it).
- Update every operating skill (and the terminal-close-out reference) to invoke daily helpers
  only through the facade's canonical spelling; prose that reads config values (shell
  variables like `$BOARD_SURFACES`/`$SKILL_*`) switches to literal interpolation from the
  preflight/env output — verified at groom time to cover every value skill prose reads.
- Route ALL metadata-tree sync instructions through "re-run `docket.sh preflight`" — pre-read
  syncs and the push-retry CAS loops alike; plain git plumbing (add/commit/push) stays direct.
- Wiring tests (tokenizer + unique anchors): a strip-then-scan sweep judging code spans per
  invocation — canonical spelling byte-exact then stripped, ops derived from `scripts/docket.md`'s
  inventory by grep, human-initiated tier allowed in prose position only, `eval`/fetch/pull
  shapes forbidden — plus mutation-tested presence anchors for the new Step-0 and re-sync
  instructions. Existing skill-prose test anchors are followed to the new spellings, never
  loosened.

## Out of scope

- Any facade or helper behavior change (0068 owns the facade; a `bootstrap` facade verb is a
  possible future stub, not this change).
- The Cursor guide and published permission fragment (0073).
- Changing what the skills do — only how their shell surface is expressed.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-14 — reconcile (docket-implement-next)

Verified against current `origin/main` + `origin/docket`; no scope change, spec unchanged. Findings:

- **Dependency 0068 is `done`** (archived `2026-07-14-0068-docket-command-facade.md`; facade PR #78 merged). The facade shipped exactly as the spec assumes: `scripts/docket.sh` with 13 operations (11 wrapped + `preflight`/`env` verbs), the inventory table in `scripts/docket.md` IS the permission inventory, `docket.sh env`/`preflight` emit raw plain `KEY=value` (19 keys) with `METADATA_WORKTREE` absolutized and `*_DIR` kept repo-relative.
- **Scope floor confirmed against live prose:** the `eval "$(…docket-config.sh --export)"` spelling is present in exactly 8 files (the 7 operating skills' `SKILL.md` + `docket-convention/SKILL.md`); `docket-config.sh --export` is referenced in 9 files (those 8 + `references/terminal-close-out.md`); 36 direct `"${DOCKET_SCRIPTS_DIR…}"/<helper>.sh` invocation sites across the same 9 files. `docket-brainstorm/SKILL.md` carries no Step-0 preamble (correctly out of scope). Build must re-derive the exact site set by whole-repo grep — these are a floor.
- **ADR-0029 (Accepted, change 68)** pins the two contracts this change consumes: metadata paths compose as `$METADATA_WORKTREE/$CHANGES_DIR` (absolute root + relative subpath), `RESULTS_DIR` composes against the feature worktree; `--format plain` backs `env`/`preflight` while `--format shell` stays byte-unchanged for the sole `docket-config.sh --bootstrap` CREATE_ORPHAN carve-out.
- **Related 0073 (Cursor guide) is still `proposed`** — a downstream consumer of this change's command surface, no coupling into this build.
- **No config value read by skill prose is missing from the `env`/`preflight` block** (19 keys present incl. `BOARD_SURFACES` + all five `SKILL_*`); `GITHUB_PROJECT`/`AGENT_HARNESSES` are not read by skill prose (their consumers self-resolve). Build re-verifies by grep.
