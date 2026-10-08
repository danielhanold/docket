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
