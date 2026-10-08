# v1.0.0-alpha.2 — decisions and STOPs

Change 0512. Every Phase 0 decision, every STOP with its reason and resumption probe.

## Phase 0 — Cursor isolation dry run (2026-10-08)

- Operator: Daniel, with the attended agent session.
- Binary: published `v1.0.0-alpha.1` (`49e4af94b838ed11beb7ee32027eb122425632d9`), installed into a scratch test home
  (`$TEST_HOME` = a throwaway folder on the operator account) with `sh install.sh --version v1.0.0-alpha.1 --harness cursor --bin-dir $TEST_HOME/bin`
  after `install.sh: OK` against the release `checksums.txt`. `install check`: `no-op`, `mode: release`, harnesses `[cursor]`; `supported_target: true`.
- Cursor: 3.23.23 (`/Applications/Cursor.app`). The operator's own Cursor was not running.
- Launch: `/Applications/Cursor.app/Contents/MacOS/Cursor --user-data-dir $TEST_HOME/cursor/user-data --extensions-dir $TEST_HOME/cursor/extensions`
  under `env -i` with `HOME=$TEST_HOME`, `XDG_*` inside it, and `PATH=$TEST_HOME/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`;
  the test home's `.zshenv`/`.zprofile`/`.zshrc` prepend `$HOME/bin`. Process environment confirmed with `ps eww`.
- Probe (Cursor terminal and Cursor agent, identical): `HOME=$TEST_HOME`, `docket=$TEST_HOME/bin/docket`, `v1.0.0-alpha.1` at `49e4af9`, `install check` `no-op` / `release` / `[cursor]`.
- Registry probe: a test-home-only agent `~/.cursor/agents/zz-isolation-probe.md` was dispatched by the Cursor agent and replied `ALPHA2-TESTHOME-7f3c`, so Cursor reads `~/.cursor` from the test home.
  (A first attempt with a `~/.cursor/rules/*.mdc` marker was inconclusive: Cursor does not appear to load user rules from that folder.)
- Kill rehearsal: SIGTERM to the test Cursor's main process (found by its `--user-data-dir`) at 2026-10-08T18:06:02Z; it exited within 10 s; relaunched with the same command; sign-in persisted.
- Chat model used: Grok 4.6 medium.
- Run Mode: Run Everything (unsandboxed), selectable and selected; the test home has no `~/.cursor/permissions.json` allowlist.
- Verdict: PASS. Scratch test home deleted afterwards.

## Phase 0 — other items (2026-10-08)

- 0543 merged: PR #408, merged 2026-10-08T17:39:26Z; record archived `done`. `origin/main` = `ec4c2b1841954c2d6a3dd018e1133ee762861137`.
- Backlog: no change `in-progress`, `implemented`, `blocked` or `stacked-merged`. Open PRs: none. `docket status`: 0 errors, 0 warnings, 4 notices.
- No-loops attestation: PENDING operator.
- No-loops attestation: Daniel, 2026-10-08 18:10Z: "no loop or autonomous docket session is running against the repo, nothing is implementing anything".

## Phase 1 — Cut the candidate (recorded 2026-10-08T18:10:59Z)

```
ec4c2b1841954c2d6a3dd018e1133ee762861137 1791481164 docs(upgrade): address two review findings
git ls-remote origin refs/heads/main: ec4c2b1841954c2d6a3dd018e1133ee762861137	refs/heads/main
```

- `docket status --json`: no change in-progress before the claim; 0 errors, 0 warnings, 4 notices.
- `gh pr list --state open`: empty.
- Claimed 0512 at 2026-10-08T18:11:13Z (untracked claim; attended protocol), workspace prepared, record reconciled, pointer plan attached.
- **Freeze rule in force:** every later phase re-probes `git ls-remote origin refs/heads/main` and requires `ec4c2b1841954c2d6a3dd018e1133ee762861137`.

## Phase 2 — Package once (2026-10-08)

- Run 37822474785 green on all seven jobs at the candidate; no STOP.
- `evidence.json` checks exact, including the `checksums_txt` byte match (no waiver needed; 0524).
- `TestBashUpgrade` (with the 0543 Cursor assertions) ran and passed: `rc=0 ok=4`.
- Eight `BUDGET WATCH` screening lines, no `SERIAL CONFIRMED OVER BUDGET`. Details in `candidate/run.txt`.
- Read-only copy made; every later phase reads it.

## Phase 3 — setup (2026-10-08)

- Test home `$TEST_HOME` (throwaway folder on the operator account); candidate installed from the read-only copy with `DOCKET_RELEASE_BASE_URL=file://$TEST_HOME/mirror` (`mirror/v1.0.0-alpha.2` links to the copy) and `--harness cursor`. `docket version`: `v1.0.0-alpha.2` @ `ec4c2b1`; `install check`: `no-op`, `release`, `[cursor]`, no findings; `supported_target: true`. Installed `docket` is byte-identical to the darwin/arm64 archive member (`a465dcff…`).
- **Deviation — Git transport (operator decision):** SSH instead of the spec's HTTPS + `gh auth setup-git`, to avoid credential prompts. OpenSSH reads keys from the account's real home regardless of `$HOME`, so the operator's existing key authenticates (verified with and without the agent socket). `gh auth login -p ssh` inside the test home for the API token; `Git operations protocol: ssh`.
- Test-home `.gitconfig`: operator identity and `init.defaultBranch main`, so the empty `gh repo create --clone` started on `main` (no rename needed).
- Fixture: private `danielhanold/docket-accept-v1-0-0-alpha-2-cursor`; `README.md` + `test.sh` pushed; `docket repository init --shared --harnesses cursor` (wrote `agent_harnesses: [cursor]`, gates `off`, as 0526 documents); committed; `configure-tests --command "sh ./test.sh"` set both gates `local`; committed; `docket install --harness cursor --repo-dir <fixture>` wrote the ignored `.cursor/rules/docket-dispatch.mdc`. `repository check`: `no-op`, no findings.
- Isolation probe in the test Cursor (Cursor 3.23.23, Run Everything, chat model Grok 4.6 medium): agent shell shows `$TEST_HOME`, `$TEST_HOME/bin/docket`, `v1.0.0-alpha.2` @ `ec4c2b1`, `install check` `no-op`/`release`/`[cursor]`; the test-home-only `zz-isolation-probe` agent dispatched and replied `ALPHA2-TESTHOME-7f3c`.
- **Finding — SIGTERM does not kill Cursor while an agent chat is open:** a SIGTERM at 18:44Z raised a quit-confirmation dialog instead of exiting (the dry run, with no active chat, exited within 10 s). The kill poller therefore sends SIGTERM, waits 10 s, then SIGKILL. The kill trigger is the first feature-branch commit touching `README.md` (the build's first code commit).

## Phase 3 — gate (2026-10-08)

- Terminal predicate: all six checks pass. Finalize merged PR #1 by itself (no hand merge); `repository check` clean without `repository prepare`.
- Recorded deviations and findings (see `harness/cursor.md`): groom exited trivial rather than with a short spec; the build tier name is inferred (dispatch parameters cut short by the kill); an orphaned `cursor-agent` worker survived the main-process kill and was terminated before the relaunch; SIGTERM with an open chat raises a quit dialog; subagents ran at grok-4.5 high regardless of their pinned efforts.
- Verdict: **pass**.

## Phase 4 — publish decision (2026-10-08)

- Release notes reviewed by Daniel. He had change 0545 created to confirm Cursor's subagent model pins in a sandbox, and kept the other known gaps as drafted.
- Daniel, 2026-10-08T19:36:32Z: "go ahead and publish".

## Phases 4–5 (2026-10-08)

- Phase 4: tag, draft, six verified assets, published 19:37:30Z as a pre-release, `v0.9.3` still Latest. See `publication.md`.
- Phase 5: public install from the release URL passed. See `public-install.md`.
