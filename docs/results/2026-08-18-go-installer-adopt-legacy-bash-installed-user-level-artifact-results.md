<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0322 — Bootstrap Go development installation and adopt legacy user-level artifacts](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-18-0322-go-installer-adopt-legacy-bash-installed-user-level-artifact.md)**
<!-- docket:backlink:end -->

# Results — 0322 Bootstrap Go development installation and adopt legacy user-level artifacts

**Change:** 0322 · **Date:** 2026-08-18 · **Branch:** `feat/go-installer-adopt-legacy-bash-installed-user-level-artifact`
**Spec:** `docs/superpowers/specs/2026-08-18-development-install-bootstrap-and-legacy-adoption-design.md`
**Plan:** `docs/superpowers/plans/2026-08-18-development-install-bootstrap-and-legacy-adoption.md`
**ADR:** ADR-0096 — legacy reproduction resolves pins from a frozen embedded v0.9.2 floor.

## Implementation route (non-standard — read this first)

This change was implemented through the **immutable `v0.9.2` Bash `docket-implement-next` workflow**,
not the current Go-driven skill — because 0322 exists precisely to unblock the Go installer, whose
transaction CLI fences off every mutation on this repo's config until 0322/0326 land (spec
"Migration-host transition rule"). A clean `v0.9.2` checkout at the tag supplied the bridge skills and
`docket.sh` helpers; no Go transaction verb (`change`/`workspace`/`evidence`/`pr`/`run`) was used for
any docket mutation. The feature branch was cut from current `origin/main`.

## What shipped

Two seams on top of change 0311's already-built install engine (0311 landed more than this change's
"Why" framing implied — the `development install` command and the whole `internal/install` engine
already existed; scope was narrowed accordingly at reconcile):

1. **`install.sh` → POSIX bootstrapper.** Resolves its own checkout dir CWD-independently, then a
   tri-state `docket` probe: compatible installed binary → `docket development install --source
   <checkout>`; absent → `go run -C <checkout> ./cmd/docket development install --source <checkout>`;
   present-but-broken → refuse (never fall through to go-run). Runs none of the legacy four
   primitives. The obsolete `tests/test_install.sh` (asserted the removed legacy behavior) was
   deleted; the four standalone scripts keep their own direct tests.
2. **Frozen legacy byte-reproducer** filling 0311's third ownership proof (`LegacyReproducer`, was
   `nil`). Reproduces the v0.9.2 user-level closed inventory — native agent defs, Cursor's `.mdc`
   rule, and the managed dispatch-block interior — byte-exact from a frozen embedded v0.9.2 floor +
   embedded agent sources, wired non-nil at both `service.go` inspect sites and threaded through
   `inspectManagedBlock`. An exact legacy install is now adopted instead of reported as a conflict;
   unknown/drifted/foreign targets are still preserved and reported (no `--force`, no hand-delete).

## Decisions & notable findings

- **Frozen embedded floor (ADR-0096).** HEAD's `agents/harness-defaults.yml` has already drifted from
  v0.9.2 (adds a `plan-writer` row). The reproducer resolves pins from a frozen embedded copy of
  v0.9.2's floor overlaid only by the user's global-layer config, so "is this a legacy install?" never
  drifts with shipped defaults.
- **Managed-block adoption (B4) is built but cannot fire on a real machine today.** 0311 places the Go
  dispatch block at user level (`~/.claude/CLAUDE.md`); v0.9.2 wrote its block only at repo level. So
  there is no colliding user-level legacy block yet. Built per the maintainer's explicit decision for
  imminent repo-level work; tested against a synthetic user-level file.
- **Plan deviation (A3 widened):** beyond the plan's text, A3 also fixed a real bug — the no-binary
  `go run` branch was CWD-relative (`./cmd/docket`); anchored with `go run -C <checkout>` — and removed
  the now-obsolete `tests/test_install.sh`.

## Verification

- **Full suite green at branch HEAD** (`scripts/run-tests.sh`): `files=122 passed=122 failed=0`.
- Per-task focused tests green; review returned 1 minor finding (no executable guard on the 14
  uncaptured frozen agent bodies), fixed in-branch with SHA-256 digest pins + a count/membership guard.
- **Known unrelated flake:** `test_gate_run_stop` fails intermittently under full parallel-suite load
  (passes in isolation; passed in the recorded green runs). It is unrelated to this diff and is tracked
  by open change **0325** ("de-flake-the-gate-run-stop-barrier-test"). Not a regression from 0322.
- Pre-existing `OVER BUDGET` files (test_sync_agents*, test_board_checks, test_docket_config, …) are
  unrelated to this change; no 0322 test file breached its budget.

## Human verification items (cannot be proven by a repo test)

1. **Post-install restart/reload.** After running `install.sh` on a real machine, restart the host
   application and confirm it reloads the generated agents and the dispatch instructions (spec records
   this as a human verification item — a repo test cannot prove a process reloaded its start-time
   config).
2. **Real legacy-machine adoption smoke test (optional).** On a machine that actually ran the v0.9.2
   Bash installer, run `install.sh` and confirm the user-level agent defs / Cursor `.mdc` are adopted
   (not reported as ownership conflicts) and that unrelated files are untouched.

## Out of scope / follow-ups

- Repository-local dispatch-block adoption/cleanup (the v0.9.2 per-repo blocks) — deferred; the eventual
  Bash-artifact cleanup is change 0318's, and repo-level installation is future work.
- 0316 (finalize/recovery), 0326 (config contraction) remain the sibling migration slices.
