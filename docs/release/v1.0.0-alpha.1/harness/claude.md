# Harness: Claude Code (Phase 3)

| Field | Value |
|---|---|
| Claude Code version | 2.1.289 (Caskroom `claude-code@latest`), permission mode `auto` |
| Mode | release install (`docket install check`: `mode: release`, harness `claude` only) |
| Test home | `$TEST_HOME` (throwaway folder on the operator's account; `HOME` and every `XDG_*` pointed inside it) |
| Candidate commit | `49e4af94b838ed11beb7ee32027eb122425632d9` |
| Archive used | `docket_v1.0.0-alpha.1_darwin_arm64.tar.gz`, SHA-256 `ceb35a19c28db67f59eabe2e928ec48203c421242d15f06ec191d68f8881721a` |
| Install source | local read-only copy of the bundle through `DOCKET_RELEASE_BASE_URL=file://…` (the release did not yet exist); the public-URL path is proved in Phase 5 |
| Installed binary | `docket version --json`: `v1.0.0-alpha.1`, commit `49e4af94b838…`; `install check` clean; `diagnostic runtime`: `supported_target: true`, go1.26.8 darwin/arm64 |
| Fixture | disposable private repo `danielhanold/docket-accept-v1-0-0-alpha-1-claude`; `build` and `finalize` both `gate: local`, `test_command: sh ./test.sh` |
| PR | https://github.com/danielhanold/docket-accept-v1-0-0-alpha-1-claude/pull/1 (MERGED) |

## Fresh process after install

Claude Code was first started in the test home after the install. The original run was session `931e8bbb` (2026-10-04 23:48:17Z–23:57:35Z); the resume ran in a different, newly started process, session `92eab679` (from 2026-10-05 00:00:14Z).

## Named children that ran

- `docket-plan-writer` (plan)
- `docket-build-economy` (build tier, plan task 1)
- `docket-review-lean` (review tier, original run) and `docket-review-standard` (review tier, resumed run)
- `docket-implement-next` (original dispatch and the resume dispatch, `resume change 1`)
- `docket-finalize-change` (two dispatches; see the finalize note below)

## Kill and resume

- The test-home Claude process (PID verified by cwd with `lsof`) was terminated with SIGTERM while the run was past build and in review/gate stage. Last activity of the killed session: 2026-10-04 23:57:35Z. The record's `plan:` was not yet visible to the poller when the kill was made; the plan commit (`e9e865e`) already existed on the feature branch.
- A fresh Claude Code process started, `docket run start implement-next --resume 1` was run, and `docket-implement-next` was dispatched for change 1 again. It resumed change 1 (the only change) at the step after the interruption, and reached `implemented` with PR #1 open.
- `docket run verify --id 1` after the resumed run: `run-complete`.
- **Not captured:** the literal printed `run-started` / `run-verdict` / continuation lines. The session transcripts do not contain them and the operator did not copy them. This record does not reconstruct them.

## Finalize

`docket-finalize-change` stopped at its merge gate: the branch-protection check calls the GitHub branch-rules API, which a private repository on a plan without it cannot answer. The operator merged PR #1 on GitHub by hand and re-ran finalize, which archived the merged change and cleaned up. Recorded as a known gap, see `decisions.md` STOP 2.

## Terminal predicate

| Check | Result |
|---|---|
| Record under `archive/` with `status: done`, full-URL `pr:`, `plan:` and `results:` set | pass |
| `gh pr view 1`: `MERGED` | pass |
| Default branch carries the greeting line | pass |
| `docket status --json`: no ready changes, 0 error findings | pass |
| `docket repository check --json` clean | pass after `docket repository prepare` (before it, a behind-only `.docket` copy was reported as `conflict`: known bug, change 0523) |
| Remote `docket` branch holds the archived record and re-rendered `BOARD.md` | pass |

## Transcript

Not published. The raw transcripts live under `$TEST_HOME/.claude/projects/`; they contain home paths and account names and were not sanitized to the bundle's standard, so this record summarizes them instead.
