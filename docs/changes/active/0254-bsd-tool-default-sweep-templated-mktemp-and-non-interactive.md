---
id: 254
slug: bsd-tool-default-sweep-templated-mktemp-and-non-interactive
title: 'BSD tool-default sweep: templated mktemp and non-interactive mv'
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [188, 189]
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

Consolidates #0188 and #0189 (2026-08-07 triage): same origin (0186), same root class — a BSD tool's default behavior silently defeats a guard in `scripts/` — and both end in the same proposed `AGENTS.md` rule. The strongest merge pair in the triage.

Verified 2026-08-07:

- **Bare `mktemp -d` ignores `TMPDIR` on macOS (#0188).** `scripts/backfill-change-types.sh:113` — `stage="$(mktemp -d)"`, so the test fixtures' documented `TMPDIR=` redirect (test_backfill_change_types.sh:195,206,286) is a no-op and `uchg` rollback fixtures leak undeletable dirs permanently. More bare sites: `profile-one-test.sh:73`, `profile-asserts.sh:73`, `run-tests.sh:185`, `terminal-publish.sh:112,270`, `sync-agents.sh:1364`. The correct form exists in-repo: `migrate-to-docket.sh:213` — `mktemp -d "${TMPDIR:-/tmp}/….XXXXXX"`.
- **Bare `mv` prompts and exits 0 (#0189) — count reproduces exactly: 15 sites.** On BSD `mv` with an unwritable destination and a tty on stdin, the prompt self-answers `n` and exits 0, so every `|| die` guard is unreachable and the write is silently discarded. Sites: `archive-change.sh:71,95`, `board-refresh.sh:128`, `docket-status.sh:1042`, `ensure-claude-settings.sh:68`, `ensure-docket-env.sh:92,119`, `ensure-global-config.sh:169`, `mark-publish-deferred.sh:116,192`, `mint-stub.sh:148,201`, `reclaim-claims.sh:49`, `render-artifact-backlink.sh:117`, `render-change-links.sh:181`. The only fixed site is 0186's (`backfill-change-types.sh:169`, `mv -f` with rationale at `:152`); the only guard is site-scoped to that one call (test_backfill_change_types.sh:241-242 — whose comment explicitly rejects a whole-file negative grep as too brittle, so the repo-wide shape-keyed guard is genuinely absent).

## What changes

- Template every bare `mktemp -d` in `scripts/` (`"${TMPDIR:-/tmp}/<name>.XXXXXX"` form).
- Convert the 15 bare atomic-replace `mv` sites to `mv -f`. Carve out `archive-change.sh:95`'s `git mv` explicitly — different tool, different prompting semantics.
- Add a shape-keyed repo-wide guard for both classes (new bare `mktemp -d` or bare `mv "` in `scripts/` reddens), designed to avoid the brittleness the 0186 test comment named.
- Promote both rules to `AGENTS.md` (non-interactive flags; TMPDIR-respecting mktemp).
- Audit `cp`/`rm` interactive-prompt analogues while sweeping (expected outcome per #0188: none — verify).

## Out of scope

- Sweeping existing leaked `uchg` debris from past runs (age-gated cleanup, if ever, is separate).
- `tests/` fixtures (this change is `scripts/`-scoped; the tests/lib fixture change owns test-side hygiene).
