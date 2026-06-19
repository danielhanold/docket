---
id: 25
slug: closeout-scripts
title: Extract the shared terminal-transition close-out mechanics into deterministic scripts
status: done
priority: medium
created: 2026-06-19
updated: 2026-06-19
depends_on: []
related: [11, 22, 23]
adrs: [1, 2, 7]
spec: docs/superpowers/specs/2026-06-19-closeout-scripts-design.md
plan: docs/superpowers/plans/2026-06-19-closeout-scripts.md
results: docs/results/2026-06-19-closeout-scripts-results.md
trivial: false
auto_groomable:
branch: feat/closeout-scripts
pr: https://github.com/danielhanold/docket/pull/36
blocked_by:
reconciled: true
---

## Why

A single `docket-finalize-change` run costs real money — the ~$3.50 finalize that
triggered this change. Instrumenting that run isolated the cause: of ~30 model turns,
only ~4 needed judgment (the learnings harvest). The other ~26 were the model
narrating and sequencing **pure git/gh mechanics** step-by-step — and every one of
those turns re-sent the full context (the finalize SKILL ≈ 3.6k words + the
convention ≈ 3.6k words + a growing transcript + per-turn reasoning). We are paying a
reasoning model to drive a shell sequence it already knows.

The largest mechanical block is the **terminal-transition close-out**: archive the
change on `metadata_branch`, copy its terminal records onto the integration branch
(terminal-publish), and remove the feature branch + worktree. This exact primitive
runs in **three** places — `docket-finalize-change`, `docket-status`'s merge sweep,
and the two kill paths (`docket-new-change` proposed-kill, `docket-implement-next`
reconcile-kill). Change **0023 explicitly deferred** scripting the sweep for precisely
this reason (§5b: "its archive add is the *same* terminal-transition primitive
`docket-finalize-change` performs and must not diverge from; scripting it correctly
means routing **both** through one shared archive helper — out of scope [there]").
This change builds that shared helper, and is the same extraction the project has
already done twice — 0011 (`github-mirror.sh`) and 0022 (`render-board.sh`).

## What changes

To be built per [the spec](../../superpowers/specs/2026-06-19-closeout-scripts-design.md):

- Three scripts under `scripts/`, each sourcing the existing
  `scripts/lib/docket-frontmatter.sh` (no new parser), with `done` and `killed`
  **unified through one primitive** so all call sites reuse it:
  `archive-change.sh` (probe-reuse → dated `git mv` → frontmatter → change-file-only
  commit + push), `terminal-publish.sh` (the docket-mode copy → CAS-push onto the
  integration branch → teardown, with the **Accepted**-ADR gate at the copy site),
  and `cleanup-feature-branch.sh` (provenance-guarded worktree + branch removal).
- **Git-write boundary:** the scripts own the deterministic plumbing *and* the
  CAS-retry loops; the **model authors each commit message** and passes it as
  `--message` (each script ships a sensible default). The scripts are **fail-closed**
  — each self-verifies its postconditions and exits non-zero with a diagnostic on any
  deviation, so the skill **trusts the exit code** (proceed on 0, abort-and-report on
  non-zero) instead of re-confirming every mechanical step by hand.
- **Rewire the four call sites** (finalize, the status sweep, proposed-kill,
  reconcile-kill) to *invoke* the scripts rather than restate the bash; their SKILL
  prose shrinks to "author a message → call the script," which is the per-turn
  input-token saving that compounds across every future close-out.
- `tests/test_closeout.sh` with **hermetic local bare-origin** fixtures (no `gh`, no
  network), matching the `test_github_mirror.sh` / `test_render_board.sh` pattern.

## Out of scope

- **The merge-gate spine** (rebase → run suite → force-push) — a separate, higher-risk
  decision, deliberately deselected from this change.
- **The config-resolution / bootstrap-guard helper** (`docket-config.sh`) — its own
  change (broad blast radius across all skills, distinct concern).
- **The harvest** — stays model-driven (judgment).
- **Any behavior change** — this moves work from model to script; the archive filename
  contract, terminal-publish copy-set rules, Accepted-ADR gate, and the racing-sweep
  idempotency guarantees are reproduced exactly, not redesigned.
- **The health checks** and the **`github` surface** — 0023 / already scripted.

## Open questions

Resolved at brainstorm 2026-06-19 — see the spec. None blocking; build-ready. One
non-blocking item for build-time review: whether ADR-0002's "terminal-publish
single-sourced in finalize" wording wants a one-line `## Update` clarifying that
finalize remains the documented owner while delegating the *mechanics* to the script
(exactly as `docket-status` delegates to `render-board.sh`).

## Reconcile log

### 2026-06-19 — reconciled at claim (no drift)

Same-day claim: the change and spec were both authored 2026-06-19 and
`origin/main` is unchanged since the 0022 close-out (`bd6ca91`) that triggered
this change, so the snapshot the design was drafted against is still current.
Confirmed against current reality before planning:

- **Cited code present and matching.** `scripts/lib/docket-frontmatter.sh`
  exposes `field` / `list_field` / `has_section` (the accessors the scripts
  source — no new parser). The canonical *Terminal publish (docket-mode)*
  procedure, the per-change archive (steps 3–4), and the `cleanup` provenance
  guard all live in `docket-finalize-change/SKILL.md` and match spec §4a–4c
  verbatim. The `tests/test_github_mirror.sh` / `tests/test_render_board.sh`
  hermetic-fixture pattern §8 mirrors exists.
- **The four call sites verified.** `docket-finalize-change` is the single
  source of the shared procedure; `docket-status`'s sweep, `docket-new-change`'s
  proposed-kill, and `docket-implement-next`'s reconcile-kill each *reference*
  it rather than restate it — so the rewire ("author a message → call the
  script") is exactly the four sites the spec names.
- **ADRs.** ADR-0001 (publish by copy), ADR-0002 (terminal-publish
  single-sourced in finalize), ADR-0007 (script-extraction boundary) are all
  `Accepted` with titles matching the spec's assumptions. The §2 build-time
  review item stands: decide whether ADR-0002 wants a one-line `## Update`
  noting finalize delegates the *mechanics* to the script while remaining the
  documented owner. No new ADR required (faithful extraction, as 0022 needed
  none).
- **Related.** 0011 (`github-mirror.sh`) and 0022 (`render-board.sh`) are
  `done`; 0023 (script-sweep-and-health-checks) is still `proposed` — this
  change *unblocks* its §5b deferred sweep-scripting and does not conflict (0023
  is not built). Not obsolete; design not invalidated. Scope unchanged: faithful
  extraction, no behavior change.
