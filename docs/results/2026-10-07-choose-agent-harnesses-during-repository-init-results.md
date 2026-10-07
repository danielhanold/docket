<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0538 — Choose agent harnesses during init and with configure-harnesses](../changes/active/0538-choose-agent-harnesses-during-repository-init.md)**
<!-- docket:backlink:end -->

# Choose agent harnesses during init and with configure-harnesses — Results

**Human action:** Optional only. Nothing blocks merge; trying the interactive picker in a real terminal is the one check automated tests cannot make.

## Outcome

`docket repository init` now asks which coding agents get docket's dispatch instructions, and a new `docket repository configure-harnesses` changes that choice later. Both go through one shared step that writes `agent_harnesses` to the repository config (`.docket.yml` as an uncommitted edit in a shared repository, `.git/dckt/config.yml` in a private one) and refreshes the instructions in the same run, using the same surface install and proof-gated removals as `docket install`.

- `--harnesses claude,cursor` or `--harnesses none` (writes `[]`) works for both commands. Unknown, repeated, and mixed `none` tokens are refused before anything is written.
- On a terminal with no flag, an arrow-key checkbox picker (charmbracelet/huh, ADR-0146) appears, pre-checked with the repository value, then the global value, then the agents detected on the machine. Cancelling writes nothing; init decides the selection before it writes anything.
- With no terminal, init succeeds and warns, naming the exact line to add and the command to run; configure-harnesses refuses.
- `configure-harnesses` runs on a dirty or behind checkout and in `needs-review`, but still requires the primary checkout to be on the integration branch.
- `repository check` reports `harnesses-unset` (a warning) while no repository-level layer declares the key. Like any finding on a healthy repository it makes `repository check` exit 1, so healthy test fixtures and the Bash-upgrade guide now declare the key.

Departures from the spec:

- The YAML writer drops a comment on the `agent_harnesses` line itself and comments between block-list items when it rewrites the key. Every other byte is preserved. Multi-line flow lists are replaced in place.
- Init and configure-harnesses now take their pending paths from git status, so a gitignored surface (the Cursor rule) or an unchanged file is never listed for commit.
- The agent-layer reference grew past its size budget; the budget in `internal/repoguard/budgets_test.go` was raised.
- The binary grows by about 1.3–1.8 MB (ADR-0146 records the measurement made at grooming).

## Human actions and testing

### Optional — try the picker in a real terminal

Prerequisites: a docket binary built from this branch, and a scratch repository with an `origin` remote that has never been set up with docket.

1. In the scratch repository run `docket repository init` from an interactive terminal.
   Expected: a checkbox list of claude, codex, cursor, opencode appears, with the agents you have installed (or your global `agent_harnesses`) already checked. Arrow keys move, space toggles, enter confirms.
2. Confirm with only `claude` checked.
   Expected: `.docket.yml` contains `agent_harnesses: [claude]` and the output lists `.docket.yml` and `CLAUDE.md` as pending paths.
3. Run `docket repository configure-harnesses` and press Ctrl-C.
   Expected: a cancellation message; `git status` is unchanged.
4. Run `docket repository configure-harnesses --harnesses none`.
   Expected: `agent_harnesses: []` and the docket block is gone from `CLAUDE.md`.

Cleanup: delete the scratch repository and its remote `docket` branch.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green on the final head after the fix pass, and green on the pre-review head. The runner printed parallel `BUDGET WATCH` / `PARALLEL-SENSITIVE` screening lines for several shards (reposetup, race, release, toolchain, and others); none was serial-confirmed over budget.
- Each build task and fix task mutation-checked its new asserts.
- Whole-branch review (deep tier): 9 findings, 1 important and 8 minor; all 9 were fixed in-branch. The disposition table is in the PR body.
- No out-of-scope follow-ups came out of this run, so no backlog match was needed.

## Known issues and follow-ups

### The config writer drops comments next to the key

When `agent_harnesses` is rewritten, a comment on the key's own line, or a comment between items of a block-style list, is removed. It happens only when the key already existed with such a comment. Impact is cosmetic: the value is correct and every other line is untouched. Confirmed (a test pins it). Workaround: put comments above the key. Suggested next action: none unless someone asks for it.
