<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0365 — Make nested Docket dispatch reliable for every Codex agent invocation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0365-codex-nested-dispatch-capability-boundary.md)**
<!-- docket:backlink:end -->
# Codex nested dispatch capability boundary — results
Change: #0365 · Branch: fix/codex-nested-dispatch-capability-boundary · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-29-codex-nested-dispatch-capability-boundary.md · ADRs: 36, 59, 60, 94 (cited, not produced)

## Verify (human)

<!-- GENUINELY MANUAL checks for the merge gate — no automated test reaches these. Codex wrappers
     are process-start-loaded artifacts: this branch validates the GENERATOR (hermetic Go tests +
     goldens + convention guard), NOT the runtime LOADER. The two-path live certification is a human
     act (learnings: generated-artifact-loaded-at-process-start). -->
- [ ] Run runbook **Phase 7** (`docs/codex/validation-runbook.md` — "Nested dispatch from both entry paths") against a **freshly started Codex process** in the disposable fixture repo, after installing this build. It must exercise BOTH entry paths and produce observable child-return sentinels for every composition family (implement-next composition, profile-routed build, rung-routed review, auto-groom critic, finalize resolver + repair):
  - [ ] **Entry path A — prose:** a plain request routed by the managed `AGENTS.md` dispatch block reaches the registered `docket-status` agent, and that agent's own nested dispatches run as child agents.
  - [ ] **Entry path B — direct `@agent`:** `@docket-implement-next` against a fixture build-ready change reaches Step 4 and dispatches the registered `docket-plan-writer` (the exact dispatch the live 0361 run falsely declared unavailable), then continues into build/review composition.
- [ ] **Record the Codex version** (`codex --version`) used for certification in the results doc — the certification is scoped to that exact version.
- [ ] Confirm **negative-evidence discipline**: any "dispatch unavailable" verdict during Phase 7 is valid ONLY if the transcript contains a direct rejection or explicit policy denial of an actually-attempted dispatch; a verdict derived from inspecting a nested tool inventory is the regression this change fixed, not an environment finding.

## Findings

- **Environmental (not a defect in this change):** during the build's full-suite gate, `internal/process`'s `TestRecoverLeavesUnprovableGroupForInspection` failed **only under `go test -race`** (`recover_test.go:246`, `Reason:durable terminal record present / Marked:0`). Root cause was extreme CPU contention from several concurrently-running gate suites (a byproduct of gate-driver owner-generation recovery during this run), which let the fake process write its terminal record before `recover` scanned it. Serial-confirmed PASS ×3 with no contention; the branch does not touch `internal/process`. No code change made. Not tracked as a new change — it is a pre-existing timing-sensitivity in untouched code, surfaced only under artificial load.
- **Real defect, fixed in-branch:** the plan's Task-1 test file (`internal/harness/codex/codex_test.go`) was committed un-`gofmt`'d, reddening `test_go_toolchain`'s gofmt check. Repaired by the integration-repair worker (`gofmt -w`, comment-alignment only, no logic change) — commit `7933c880`.
- **Review (deep rung):** 0 blocker, 0 major, 2 minor (setup.md duplicate restart paragraph; README multi-line bullet). Both fixed in-branch (batched economy) — commit `c19800be`. See the PR body disposition table.

## Follow-ups

- None beyond the human Phase-7 certification above. No new ADR (the design instantiates the retained ADR-0036/0059/0060/0094 rather than changing any of them).

## Plan deviations

- The integration-repair worker committed the gofmt fix **before** its final full-suite run (the plan's shape is fix→suite→commit). Justified: an uncommitted working-tree edit does not survive the suite's isolated-sandbox jobs, so the file had to be committed to be provable-green; focused verification (gofmt clean + focused codex test) was already green, which is the sanctioned commit trigger. End state is identical to the intended order.
- Runbook Phase 7 Step-2 prose was reordered slightly so the `AGENTS.md` reference sits after "dispatch block" — the guard pattern `prose request[^.]{0,120}dispatch block` cannot span the period in `AGENTS.md`. Meaning unchanged; no fabricated path introduced.
