<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0537 — Delete the empty AGENTS.md left behind when a dispatch block is retired](../changes/archive/2026-10-16-0537-delete-the-empty-agents-md-left-behind-when-a-dispatch-block.md)**
<!-- docket:backlink:end -->

# Delete the empty AGENTS.md left behind when a dispatch block is retired — Results

**Human action:** None required. Merge when ready; an optional walkthrough below shows the fix on a scratch repository.

## Outcome

When docket removed one of its managed blocks from a file and nothing but whitespace was left, it wrote the empty file back. A repository that went private kept a zero-byte, untracked `AGENTS.md` that `git add -A` would commit; the same leftover followed a shared-repository harness drop and `docket uninstall` of the Codex pointer.

Now install and uninstall delete such a file instead. A file with any other content keeps it, and only the block goes. An edited block still refuses the run. Rollback and crash recovery restore a deleted file byte-for-byte with its mode, with no journal change.

`docket repository set-visibility private` no longer needs the private instructions file when the declared harness list is empty, so an already-private repository whose file was deleted still reports nothing to do. With any harness declared the file is still required.

## Human actions and testing

### Optional — see the leftover file go away

Automated tests cover this; the walkthrough only shows it by hand.

Prerequisites: a docket binary built from this branch, a scratch repository already set up with docket and `agent_harnesses: [codex]`, whose `AGENTS.md` holds only docket's dispatch block.

1. Set `agent_harnesses: []` in the scratch repository's configuration and run `docket install`.
   Expected: `AGENTS.md` no longer exists, and `git status` shows it deleted.
2. Repeat with a line of your own text added below the block before step 1.
   Expected: `AGENTS.md` remains and holds only your text.

Cleanup: delete the scratch repository.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the final head, after the review fix. The budget report showed only screening lines (`BUDGET WATCH` / `PARALLEL-SENSITIVE`), no serially confirmed breach.
- Each new test was mutation-checked: switching the whitespace check off reddened the deletion, rollback, recovery, uninstall, private-retirement and harness-drop tests; making deletion unconditional reddened the keep-user-text test; reverting the set-visibility check reddened the no-harness cases, and forcing it done reddened the declared-harness case.
- Standard-tier review ran and returned one minor finding (a misplaced test doc comment), fixed in-branch.

## Known issues and follow-ups

- **Empty files from earlier runs stay.** A repository that already holds a zero-byte `AGENTS.md` from a previous install keeps it; docket can no longer prove it is its own. Confirmed, by design. Workaround: delete it by hand. No next action.
- **Dangling CLAUDE.md link after a harness drop.** In a shared repository where `CLAUDE.md` is a link to `AGENTS.md`, dropping codex and opencode while keeping claude now deletes an `AGENTS.md` that held only the block, leaving the link pointing at nothing. Claude already lacked the dispatch rules in that state, so nothing gets worse. Confirmed. Suggested next action: Fits #555 (groomed proposed) — no edit needed; its scope already covers this.
