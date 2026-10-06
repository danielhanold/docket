<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0535 — Load a private repository's agent instructions without repository files](../changes/archive/2026-10-06-0535-load-a-private-repository-s-agent-instructions-without-repos.md)**
<!-- docket:backlink:end -->

# Load a private repository's agent instructions without repository files — Results

**Human action:** Needed before relying on it. Run the two Important checks below. Automated acceptance covered only headless print/exec sessions, not a fresh interactive Claude Code session or Cursor's real user-level hooks file.

## Outcome

Before this change, a private-visibility repository had no way to give the agent docket's dispatch and run-tracker rules, because those rules normally live in the repository's own AGENTS.md or CLAUDE.md. Now:

- **The private instructions file.** In a private repository, the dispatch block lives in `.git/dckt/AGENTS.md`, and promoted lessons go there too, outside the managed block. Docket writes nothing into the working tree and never edits `.git/info/exclude`. Both `repository init --private` and install's repository phase write only this file, from any worktree. They also retire dispatch surfaces an earlier install left in the working tree, including surfaces an install wrote from a linked worktree.
- **`docket instructions` (read-only).** It prints that file from any worktree of a private repository and prints nothing anywhere else.
  - `--section dispatch|lessons` prints one part.
  - `--hook claude|cursor` wraps the output as session-start hook JSON. Hook modes never fail a session.
- **User-level triggers.** `docket install` adds one content-free trigger per harness, once per machine. None contains rule text or the word "docket", and each is inert outside a private repository:
  - Claude Code: two `SessionStart` hooks in `~/.claude/settings.json`, one for the dispatch block and one for the lessons, so each fits Claude Code's per-hook size cap.
  - Cursor: a `sessionStart` hook in `~/.cursor/hooks.json`.
  - OpenCode: the plugin `~/.config/opencode/plugins/dckt-instructions.js`.
  - Codex: a `dckt:private-instructions` pointer block in `~/.codex/AGENTS.md`. This one is best effort, and it replaces any leftover `docket:dispatch` block in that file.
- **Hooks files are edited carefully.** Docket adds only its own exact entries and leaves every other byte alone. Uninstall removes an entry only while it is unchanged. A hooks file docket cannot edit in place (a symlink, or a file it cannot parse, such as one with comments) is left untouched: install reports a `hook-file-not-editable` warning and still installs everything else.
- **Shared repositories are unchanged.**

**Departures from the plan:**
- After review, a hooks file docket can't edit became a warning instead of a failed install.
- Changed trigger commands are now retired automatically.
- An extra commit fixed a flaky test in `internal/process` that the build gate tripped on.

ADR-0145 records the delivery design.

## Human actions and testing

### Important — Fresh interactive Claude Code session in a private repository

**Why:** automated acceptance used `claude -p` with a temporary `--settings` file. Claude Code may load user-level `SessionStart` hooks differently in an interactive session started from the real `~/.claude/settings.json`. If you skip this check, it stays unproven that the rules reach a normal session.

**Prerequisites:** this branch's binary installed (`docket development install --source <checkout>`), the `dckt` alias on `PATH`, and a scratch private repository: `git init`, one commit, an `origin` remote, then `docket repository init --private`. Add `agent_harnesses: [claude]` to `.git/dckt/config.yml` and run `docket repository init --private` again.

1. Run `grep -c "dckt instructions" ~/.claude/settings.json`.
   Expected: `2`.
2. In the scratch repository run `dckt instructions --section dispatch | head -3`.
   Expected: the `docket:dispatch` start marker and the block heading.
3. Start a new interactive `claude` session in that repository and ask: "Quote the step of your instructions that starts with 'After the run returns'."
   Expected: it quotes the run-tracker step (`run.verdict` ... "Obey the resulting").
4. Start a session in a directory outside any repository and ask the same question.
   Expected: no such step.

**Cleanup:** delete the scratch repository. `docket uninstall` removes the hooks if you want them gone.

### Important — Cursor through the real user-level hooks file

**Why:** acceptance copied the hooks file into a project-level `.cursor/hooks.json`. User-level loading from `~/.cursor/hooks.json` with a logged-in Cursor was not exercised.

**Prerequisites:** a logged-in Cursor, `docket install` run with `cursor` opted in, and the scratch private repository above with `cursor` added to `agent_harnesses`.

1. Run `grep -c "dckt instructions --hook cursor" ~/.cursor/hooks.json`.
   Expected: `1`.
2. Open the scratch repository in Cursor, start a new agent chat, and ask the same quote question.
   Expected: it quotes the run-tracker step.

**Cleanup:** delete the scratch repository.

## Verification performed

- **Full suite (`go run ./cmd/docket development test`):** green at the final head (82 of 82 files), including the new `test_go_integration_app_privateinstructions.sh` shard.
  - The first build-gate run failed only `internal/process` `TestRecoverLeavesUnprovableGroupForInspection`. That is an existing race in the test's own setup: killing a process group does not stop every process at once. It was fixed by killing the supervisor before its command.
  - The final run printed `BUDGET WATCH` for `test_go_integration_app_reposetup.sh` (90s under -j11, streak 1/5). That is a screening line, not a confirmed breach.
- **Fresh-session acceptance (plan Task 9):** run with temporary homes and a private repository fixture. The dispatch section is 7773 characters, under Claude Code's 10,000-character cap.

  | Harness | Version | Mode | Verdict |
  |---|---|---|---|
  | Claude Code | 2.1.291 | `claude -p --setting-sources project --strict-mcp-config --settings <tmp>/settings.json` | PASS in the primary worktree and a feature worktree; no token in the control run outside git |
  | Cursor | cursor-agent 2026.10.01 | `-p --trust`, hooks file copied to the project | PASS; the hook command also passes with `CURSOR_PROJECT_DIR` and with stdin `workspace_roots` |
  | OpenCode | 1.18.31 | `opencode run` with `OPENCODE_CONFIG_DIR=<tmp>` | PASS in both worktrees; control clean |
  | Codex | codex-cli 0.154.0 | `codex exec --sandbox workspace-write`, pointer as project AGENTS.md | PASS (recorded, not blocking); it ran `dckt instructions` itself; user-level loading not exercised |

  The plan's literal prompt asked only for the sentence that starts "After the run returns", and that sentence stops before "Obey the resulting". The verdicts therefore rest on a prompt asking for the whole step 2, which every harness passed. With the literal prompt, every harness quoted the sentence and the lesson token.
- **Guards:** each new guard was mutation-tested (stripped, seen to turn red, restored). This covers the no-rule-text trigger guard, the hooks-file round trip, the private-mode retirement, the section selection, the cursor directory lookup, and the skill-prose guard.
- **Whole-branch review (deep tier):** 8 findings (0 blocker, 4 important, 4 minor). 7 were fixed in-branch and 1 was recorded unfixed. The full table is in the PR body.

## Known issues and follow-ups

- **An empty AGENTS.md is left behind after retirement (recorded, not fixed).** When docket retires a dispatch block an earlier install wrote into a private working tree, it removes the block but leaves a zero-byte, untracked `AGENTS.md`. Nothing leaks, but `git add -A` would commit the empty file. Docket's ownership record cannot tell a file docket created from an empty file the user created, so deleting the file safely needs a new record field. Workaround: delete the empty file by hand. Confirmed. Suggested next action: Related to #533 (repository visibility switching also retires working-tree surfaces), but not within its stated scope. No existing change fits (checked 24). A human may capture it as a new change linking #533 under `related:`.
- **Other limits of the design:**
  - Triggers take effect only in sessions started after install.
  - A private repository whose git directory is not `<primary>/.git` (`--separate-git-dir`) refuses the repository phase.
  - Once an install has run after a recorded hooks file became unparseable, a later uninstall leaves docket's entries in that file alone.
  - Uninstall leaves a hooks file docket created as `{}` or `{"version": 1}`.
  - An older binary reads the new hook-entries install state as invalid after a downgrade.
  - OpenCode's hook is experimental, and Codex following the pointer is best effort.
  - Lessons longer than 10,000 characters reach Claude only as a path plus a 2KB preview.
  - A scoped `docket install --harness <one>` records only that harness's ownership of the private file. It keeps the Codex section, though, whenever Codex is opted in.
