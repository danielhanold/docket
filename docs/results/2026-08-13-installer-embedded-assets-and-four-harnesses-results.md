<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0311 — Installer, embedded assets, and four first-class harnesses](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-14-0311-installer-embedded-assets-and-four-harnesses.md)**
<!-- docket:backlink:end -->

# Installer, embedded assets, and four first-class harnesses — results
Change: #311 · Branch: feat/installer-embedded-assets-and-four-harnesses · PR: (opened at step 7) · Plan: docs/superpowers/plans/2026-08-13-installer-embedded-assets-and-four-harnesses.md · ADRs: none new

## Verify (human)

- [ ] Outside-truth: live vendor confirmation that Claude, Codex, Cursor, and OpenCode each load
  the installed user-level agents, skills, and dispatch material in a fresh session. No fixture can
  prove a process-start artifact loaded; the goldens mirror `sync-agents.sh`'s production emitters
  (live-verified at changes 0135/0168/0192). Per the spec this is recorded here and certified as
  part of change 0317's live acceptance, not at this merge gate.
- [ ] Optional smoke on this machine (state to reproduce, not a conclusion to confirm): from the
  worktree, `go run ./cmd/docket development install --source . --bin-dir "$(mktemp -d "${TMPDIR:-/tmp}/dkt-bin.XXXXXX")" --harness claude` against a scratch
  `HOME` — expect `ownership-conflict` with per-target remedies and the legacy-limitation note if
  run against your real home (the Bash-installed files are deliberately not auto-adopted; see
  follow-up 322), or a clean apply against a fresh fake home.

## Findings

- Gate repair 1 (`14d026cb`): the suite runner's per-job `TMPDIR` carries a doubled interior slash
  on macOS; two Go tests compared Join-cleaned paths against the raw `t.TempDir()` string.
  Test-side normalization; production `ResolveRoots` already `filepath.Clean`s and gained a
  regression test.
- Gate repair 2 (`113aba63`): the transaction engine trusted `os.WriteFile`'s creation mode, so
  under a umask-077 environment installed targets landed 0700/0600 instead of the promised
  0755/0644. Explicit `Chmod` enforcement on apply and rollback, with a umask-077 regression test.
  Real defect, only observable under the detached gate runner's umask.
- Deep review returned 0 blockers / 6 important / 5 minor; all six importants and four of five
  minors fixed in-branch (see the PR body's disposition table). No new ADRs: the load-bearing
  decisions (journaled transaction, flock install lock, planner seam, claude `inherit`
  passthrough) are recorded in the approved spec and at their code sites.
- Standing advisory, pre-existing on main: `OVER BUDGET: test_sync_agents_runners` (~200s vs 60s
  ceiling). Untouched by this branch; existing stub 280 covers the shard/re-budget work.

## Follow-ups

- Change 322 (minted from review important #3): adopt legacy Bash-installed user-level artifacts
  via a frozen legacy renderer — until it lands, machines set up by `sync-agents.sh` refuse
  `docket install` with `ownership-conflict` remedies rather than adopting.
- Change 323 (minted from review important #4): `docket uninstall` and version-tree collection —
  superseded immutable trees accumulate under `<data-root>/versions/` with no GC.
- Review minor #10 (deferred, not minted): `cursor-rules/dispatch.head.md` mentions repo-local
  `.cursor/agents/` while this installer writes user-level `~/.cursor/agents/`; the authored asset
  text is opaque to 0311 by spec, so the rewording belongs to whichever change re-authors the
  dispatch payloads (0317/0318 territory).
- Plan deviations worth knowing: `Options.Adapters` became a closure-based `install.Planner` seam
  (import cycle); `Target.Mode`, `TargetRecord.Harness`, and `BeginTxnWithRemovals` were added
  beyond the plan's interface blocks; the embed directive needs the `all:` prefix to carry
  dotfiles; `cmd/genassets` exits via `log.Fatalf` under a tested two-file exit-site allowlist.
