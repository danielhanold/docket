---
id: 254
slug: bsd-tool-default-sweep-templated-mktemp-and-non-interactive
title: 'BSD tool-default sweep: templated mktemp and non-interactive mv'
status: implemented
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-08
depends_on: []
related: [118]
discovered_from: [188, 189]
adrs: []
spec: docs/superpowers/specs/2026-08-07-bsd-tool-default-sweep-templated-mktemp-and-non-interactive-design.md
plan: docs/superpowers/plans/2026-08-08-bsd-tool-default-sweep-templated-mktemp-and-non-interactive.md
results: docs/results/2026-08-08-bsd-tool-default-sweep-templated-mktemp-and-non-interactive-results.md
trivial: false
auto_groomable: true
branch: feat/bsd-tool-default-sweep-templated-mktemp-and-non-interactive
claimed_at: 2026-08-08T03:21:13Z
pr: https://github.com/danielhanold/docket/pull/180
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-bsd-tool-default-sweep-templated-mktemp-and-non-interactive-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-bsd-tool-default-sweep-templated-mktemp-and-non-interactive-design.md) |
| Plan | [2026-08-08-bsd-tool-default-sweep-templated-mktemp-and-non-interactive.md](https://github.com/danielhanold/docket/blob/feat/bsd-tool-default-sweep-templated-mktemp-and-non-interactive/docs/superpowers/plans/2026-08-08-bsd-tool-default-sweep-templated-mktemp-and-non-interactive.md) |
| Results | [2026-08-08-bsd-tool-default-sweep-templated-mktemp-and-non-interactive-results.md](https://github.com/danielhanold/docket/blob/feat/bsd-tool-default-sweep-templated-mktemp-and-non-interactive/docs/results/2026-08-08-bsd-tool-default-sweep-templated-mktemp-and-non-interactive-results.md) |
| PR | [#180](https://github.com/danielhanold/docket/pull/180) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0188 and #0189 (2026-08-07 triage): same origin (0186), same root class — a BSD tool's default behavior silently defeats a guard in `scripts/` — and both end in the same proposed `AGENTS.md` rule. The strongest merge pair in the triage.

Verified 2026-08-07:

- **Bare `mktemp -d` ignores `TMPDIR` on macOS (#0188).** `scripts/backfill-change-types.sh:113` — `stage="$(mktemp -d)"`, so the test fixtures' documented `TMPDIR=` redirect (test_backfill_change_types.sh:195,206,286) is a no-op and `uchg` rollback fixtures leak undeletable dirs permanently. More bare sites: `profile-one-test.sh:73`, `profile-asserts.sh:73`, `run-tests.sh:185`, `terminal-publish.sh:112,270`, `sync-agents.sh:1364`. The correct form exists in-repo: `migrate-to-docket.sh:213` — `mktemp -d "${TMPDIR:-/tmp}/….XXXXXX"`.
- **Bare `mv` prompts and exits 0 (#0189) — count reproduces exactly: 15 sites.** On BSD `mv` with an unwritable destination and a tty on stdin, the prompt self-answers `n` and exits 0, so every `|| die` guard is unreachable and the write is silently discarded. Sites: `archive-change.sh:71,95`, `board-refresh.sh:128`, `docket-status.sh:1042`, `ensure-claude-settings.sh:68`, `ensure-docket-env.sh:92,119`, `ensure-global-config.sh:169`, `mark-publish-deferred.sh:116,192`, `mint-stub.sh:148,201`, `reclaim-claims.sh:49`, `render-artifact-backlink.sh:117`, `render-change-links.sh:181`. The only fixed site is 0186's (`backfill-change-types.sh:169`, `mv -f` with rationale at `:152`); the only guard is site-scoped to that one call (test_backfill_change_types.sh:241-242 — whose comment explicitly rejects a whole-file negative grep as too brittle, so the repo-wide shape-keyed guard is genuinely absent).

## What changes

Settled — see the spec (2026-08-07). Groom-time audit widened the sweep along the stub's own
title ("templated mktemp"): bare *file* `mktemp` is probe-verified equally TMPDIR-ignoring on
macOS, so both forms are swept (23 sites), not only `-d`. Scope includes the repo-root entry
scripts the stub itself names (`sync-agents.sh`) plus `install.sh`/`migrate-to-docket.sh` and
`scripts/lib`+`scripts/runners`; `tests/` stays excluded.

- Template every untemplated `mktemp` (both forms) to `"${TMPDIR:-/tmp}/<script-name>.XXXXXX"`
  (the `migrate-to-docket.sh` precedent). Six existing beside-destination templated sites are
  correct (atomic same-filesystem rename) and stay untouched.
- Convert the 16 bare atomic-replace/rename `mv` sites (14 in `scripts/` + 2 repo-root) to
  `mv -f`. Carve out `archive-change.sh:95`'s `git mv` — different tool, different prompting
  semantics; the guard's allowance keys on its actual `$GIT … mv` spelling.
- New shape-keyed guard `tests/test_bsd_tool_defaults.sh`: no bare `mv "` (two split ERE
  patterns under a pinned `/usr/bin/grep` — the combined spelling is vacuous under PATH ugrep,
  probe-verified), and every `$(mktemp` line must carry an `XXXXXX` template (deliberately
  template-required, not TMPDIR-required). Population floors + mutation tests; avoids the
  brittleness the 0186 test comment named.
- Behavioral pin where a fixture already depends on TMPDIR: `test_backfill_change_types.sh`
  asserts the stage remnant lands under the redirected fixture tmpdir.
- Promote both rules to `AGENTS.md` `## Shell` directly via this PR (not a learnings
  promotion): non-interactive `mv -f` on install paths; always-template `mktemp`
  (TMPDIR-rooted unless beside-destination for atomic rename).
- `cp`/`rm` audit closed with corrected shapes (`cp -i`; any `rm` lacking `-f` — BSD `rm`
  prompts *without* `-i` on a write-protected target with a tty): zero sites; build re-verifies.

## Out of scope

- Sweeping existing leaked `uchg` debris from past runs (age-gated cleanup, if ever, is separate).
- `tests/` fixtures (test-side hygiene is owned by the tests/lib fixture change).

## Reconcile log

### 2026-08-08 — build claim

Re-derived every inventory the spec names against the current working tree (`/usr/bin/grep`, not
PATH ugrep). The design survives intact; four notes:

- **`mktemp` inventory holds.** 7 bare `mktemp -d` and 16 bare file-`mktemp` sites, exactly the
  spec's set. Only line numbers drifted (`docket-status.sh`'s templated learnings-index site is
  now `:1037`, its `mv` `:1047`) — the spec already directs the build to re-derive by grep rather
  than trust the listed lines, so this is expected, not drift in the design.
- **`mv` count corrected: 16 conversions, not 17.** The stub's "15 sites in `scripts/`" counted
  `archive-change.sh:95` — the `git mv` the spec then carves out — so the arithmetic double-counts
  it. Live set: 14 in `scripts/*.sh` + 2 repo-root (`sync-agents.sh:128`,
  `migrate-to-docket.sh:345`) = **16**. The carve-out and every other design decision are
  unaffected; only the number in §2 was wrong.
- **cp/rm audit re-verified (spec §6, A8): still zero.** No `cp -i` site in scope; every `rm`
  carries `-f`/`-rf`. Exactly the predicted chaff appeared and nothing more — two `git rm`
  invocations (`sync-agents.sh:1197` inside a constructed remedy string,
  `migrate-to-docket.sh:322`) plus comment mentions. No code change; recorded in the results file.
- **`tests/test_bsd_tool_defaults.sh` does not exist**, so §3 is a clean create.

Coupling with 0118 (A9) unchanged: still `proposed`, still `related:`, no rebase interaction
beyond the same-file/different-function edit the assumption already anticipated. No new ADRs are
implied — every decision the build makes was settled at groom time in A1–A10.
