<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0334 — Make Docket dispatch minimal, non-recursive, and mechanically gated](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-26-0334-claude-md-global-and-project-copies-out-of-sync-on-finalize-a.md)**
<!-- docket:backlink:end -->
# Make Docket dispatch minimal, non-recursive, and mechanically gated — results
Change: #0334 · Branch: feat/claude-md-global-and-project-copies-out-of-sync-on-finalize-a · Plan: docs/superpowers/plans/2026-08-26-minimal-non-recursive-gated-dispatch.md · ADRs: none

## Verify (human)

These are merge-gate preconditions no automated test can certify — process-start artifacts and a personal global file. Each stays PENDING until a human completes it.

- [ ] Remove the hand-authored Docket dispatch block from your personal global `~/.claude/CLAUDE.md`. Docket never touches that file; its stale verbose copy must be deleted by hand before external acceptance, otherwise a stale process-start block can still supply the old roster/run-gate prose.
- [ ] Four-harness fresh-process behavioral acceptance (Claude, Codex, Cursor, OpenCode) per the spec's "External harness acceptance" checklist. Sync assets, delete the global block, terminate stale harness processes, then start wholly fresh sessions and record for each: harness + exact version, invocation mode, the generated artifact paths, and the observed agent tree. Each harness must show: native agent discovery exposes the same-name wrapper without the prose roster; a registered-workflow request dispatches exactly one entry wrapper; that wrapper does not dispatch another instance of itself for the current assignment; its charter runs once; a representative controller can still dispatch a *different* required child; and no behavior depends on the deleted global block. A failed native-discovery check on any harness is a release blocker for that harness's roster removal.

## Findings

- **Compact dispatch block ships in both lockstep generators.** The AGENTS.md/CLAUDE.md managed block dropped from ~1156 words to 352 (guarded by `tests/test_dispatch_block_budget.sh`, whose ceiling of 400 hard-asserts below the recorded pre-change actual). The 17-agent roster and the hand-executed run-gate attribution/retry prose are both gone; the native registry is now authoritative, and the run gate is a compact `gate-before`/`gate-verdict` trigger.
- **Run-gate facade is real Go, delegating the predicate.** `docket run gate-before` / `gate-verdict` (attributed + `--unattributed` observe) over durable records under `<git-common-dir>/docket/rungate/`, reusing the dispatch-record machinery pattern. The one-retry grant is an O_EXCL filesystem CAS (`ConsumeGateRetry`) consumed before the report is chosen; observe-mode is structurally unable to emit `gate-retry-once` (separate renderer). `internal/app/run_verify.go` and `scripts/verify-run.sh` predicates and the ADR-0075/0084/0088 policy are byte-unchanged — the facade relocates mechanics, it does not re-derive the run verdict.
- **Exact-name recursion guard** is injected into every generated wrapper by one shared emitter (`internal/harness/guard.go` + the byte-identical `sync-agents.sh` path), prohibiting only `docket-X → docket-X` for the current assignment while preserving required cross-agent dispatch; the guard shape-checks the wrapper's own literal name and never mentions "your preloaded skill". Its mutation matrix is asserted in `tests/test_sync_agents_recursion_guard.sh` and the per-adapter Go tests.
- **Deep review (docket-review-deep): clean** — 0 blockers, 0 important, 1 minor.
  - *Minor (intentional, not a code defect):* the "minimal dispatch" compaction reaches Claude/Codex/OpenCode but **not** Cursor's always-applied `~/.cursor/rules/docket-dispatch.mdc`, which by spec Part 4 intentionally retains its own per-agent fragments (its distinct routing surface) and is out of the AGENTS.md budget guard's scope. Cursor still consumes the *same* compact gate payload. Recorded here so the reviewer/merger reads the asymmetry as deliberate; uniform compaction of the Cursor surface, if ever wanted, is a separate change.

## Follow-ups

- None required. Change 0294's roster/run-gate slimming scope was absorbed here (0294 already killed/archived). If uniform compaction of the Cursor `docket-dispatch.mdc` surface is later desired, that is net-new work, not a regression of this change.
