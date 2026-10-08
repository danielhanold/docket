# Harness: Cursor (Phase 3)

| Field | Value |
|---|---|
| Cursor version | 3.23.23 (`/Applications/Cursor.app`), a separate instance with its own user-data and extensions directories inside the test home |
| Run Mode | Run Everything (unsandboxed); no `~/.cursor/permissions.json` allowlist |
| Chat model | Grok 4.6 medium (operator chat). Subagents ran at grok-4.5 high (see *Model pins* below) |
| Account | Cursor Pro (the free tier's limit was hit after the groom; the operator signed in to a Pro account and continued) |
| Test home | `$TEST_HOME` (throwaway folder on the operator account; `HOME` and every `XDG_*` inside it) |
| Launch | `Cursor.app/Contents/MacOS/Cursor --user-data-dir $TEST_HOME/cursor/user-data --extensions-dir $TEST_HOME/cursor/extensions` under `env -i` with the test home's `HOME`, `XDG_*` and `PATH=$TEST_HOME/bin:…` |
| Candidate commit | `ec4c2b1841954c2d6a3dd018e1133ee762861137` |
| Archive used | `docket_v1.0.0-alpha.2_darwin_arm64.tar.gz`, SHA-256 `846cd193fafd827f481c1310cfd9e0d6c708b123f9e15b6d8d81912c769fd808`; installed binary byte-identical to its `docket` member (`a465dcff…`) |
| Install source | the read-only copy through `DOCKET_RELEASE_BASE_URL=file://$TEST_HOME/mirror`; the public URL is proved in Phase 5 |
| Installed binary | `v1.0.0-alpha.2` @ `ec4c2b1`; `install check`: `no-op`, `release`, `[cursor]`, no findings; `supported_target: true`, go1.26.8 darwin/arm64 |
| Git transport | SSH (operator decision; OpenSSH reads the account's real `~/.ssh`); `gh auth login -p ssh` inside the test home |
| Fixture | private `danielhanold/docket-accept-v1-0-0-alpha-2-cursor`; shared metadata; `agent_harnesses: [cursor]`; `build`/`finalize` `gate: local`, `test_command: sh ./test.sh`; repository `.cursor/rules/docket-dispatch.mdc` written and git-ignored |
| PR | https://github.com/danielhanold/docket-accept-v1-0-0-alpha-2-cursor/pull/1 (MERGED 2026-10-08T19:30:57Z, merge commit `82acd7f`) |

## Isolation

- Agent shell probe: `HOME=$TEST_HOME`, `docket=$TEST_HOME/bin/docket`, `v1.0.0-alpha.2` @ `ec4c2b1`, `install check` `no-op`/`release`/`[cursor]`.
- Registry probe: the test-home-only agent `~/.cursor/agents/zz-isolation-probe.md` was dispatched and replied `ALPHA2-TESTHOME-7f3c`.
- The test Cursor was launched after the install; the operator's own Cursor was not running during Phase 3.

## Lifecycle

1. `change.create` → change 1 "Add a greeting line to README" (18:50:51Z).
2. Groom → **trivial** verdict (18:52:00Z). The spec asked for a short spec; the groomer judged it trivial, which is build-ready and still requires a plan and results.
3. Implement: `run start`, dispatch `docket-implement-next`; claim 19:05:33Z (with the run context: claim `gate_context_hash` set), reconcile 19:07:03Z, plan attached 19:08:42Z, build commit `429b42c` 19:09:37Z.
4. Kill and resume (below).
5. Resumed run reached `implemented` 19:24:41Z with PR #1 open; `docket run verify --id 1`: `run-complete`.
6. `docket-finalize-change` merged PR #1 by itself (19:30:57Z) and closed out (19:31:08Z). Closeout late finding: "Merge method came from the repository settings alone (branch-rules-unavailable …)", as expected on a private repository without branch rules.

## Named children that ran

From the test Cursor's agent records:

- `docket-implement-next` — the original dispatch (19:04:55Z) and the retry dispatch (19:19:01Z), by name.
- `docket-plan-writer` — by name (19:08:09Z); returned `PLAN_PATH`.
- Build tier — a subagent (19:09:27Z) that loaded `~/.cursor/skills/docket-build-task/SKILL.md` and ran the build-task contract (check, edit, test, commit `429b42c`). Its model, grok-4.5 high, is `docket-build-economy`'s pin; the persisted dispatch parameters were cut short by the kill, so the tier name is inferred from the contract and the model rather than read from a logged name.
- `docket-review-lean` — by name (19:22:31Z); results: "Whole-branch review (lean tier): clean".

## Kill and resume

- Trigger: a local poller fired on the first feature-branch commit touching `README.md` (the build's first code commit), and sent SIGTERM to the test Cursor's main process, falling back to SIGKILL after 10 s. The main process exited on SIGTERM at once.
- Finding: an orphaned `cursor-agent` worker process (parent `launchd`, idle, no children) survived the main-process kill; the operator terminated it before the relaunch. Killing Cursor's window process does not stop its agent worker.
- Finding: earlier, with an agent chat open but idle, SIGTERM raised a quit-confirmation dialog instead of exiting; hence the SIGKILL fallback.
- Fixture state at the kill: `docket` at `ce1e868` (claim refreshed), feature worktree clean at `429b42c`, feature branch not yet pushed.
- Relaunched 19:10:18Z; in a new chat the operator ran `run start implement-next --resume 1` and dispatched again.

Poller log:

    2026-10-08T18:43:59Z poller started
    2026-10-08T18:48:07Z poller started
    2026-10-08T19:09:37Z trigger: branch feat/add-a-greeting-line-to-readme commit 429b42c6fa3ac09c38b81325684627083cc32f96 (feat: add greeting line to README)
    2026-10-08T19:09:38Z SIGTERM sent to 62389
    2026-10-08T19:09:38Z test Cursor exited
    2026-10-08T19:10:08Z orphaned cursor-agent worker 75198 (ppid 1, idle, no children) survived the main-process kill; SIGTERM'd
    2026-10-08T19:10:18Z test Cursor relaunched, pid 83475

Verdict lines, copied verbatim by the agent into `$TEST_HOME/run-lines.txt` as they printed:

    run-started implement-next-20261008t190446z-77522-fa5f 86f5588aa08e5014e4c107321288ce5f
    run-untracked resume-active-run
    run-retry-once implement-next-20261008t190446z-77522-fa5f run-incomplete 1 not-implemented results-unlinked remote-head-mismatch pr-unverified evidence-unverified
    run-done implement-next-20261008t190446z-77522-fa5f run-complete 1

The resume start refused a second run over the still-active one (`resume-active-run`); the verdict on the original key authorized one retry, which completed the same change. Exactly one claim exists on the metadata branch.

## Model pins

The installed wrappers pin `docket-implement-next` and `docket-review-lean` to grok-4.5 medium and `docket-plan-writer` to grok-4.5 low, but all ran at grok-4.5 high: the dispatching agent passed a `model` parameter, and Cursor's records show the high-effort model for each child. Recorded as an observation; it does not affect the terminal predicate.

## Terminal predicate

| Check | Result |
|---|---|
| Record under `archive/` with `status: done`, full-URL `pr:`, `plan:` and `results:` set | pass |
| `gh pr view 1`: `MERGED` | pass |
| Default branch carries the greeting line | pass (`README.md` line 5) |
| `docket status --json`: no ready changes, 0 error findings | pass (0 ready, 0 errors, 0 warnings) |
| `docket repository check --json` clean | pass (`no-op`, no findings; no `repository prepare` needed) |
| Remote `docket` branch holds the archived record and re-rendered `BOARD.md` | pass (`29e4c26`) |

## Transcript

Not published. The agent records live in the test Cursor's `state.vscdb` under `$TEST_HOME/cursor/user-data`; they hold home paths and account details and were not sanitized to the bundle's standard, so this record summarizes them instead.
