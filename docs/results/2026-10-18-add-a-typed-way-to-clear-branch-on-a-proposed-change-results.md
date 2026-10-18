<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0568 — Stop deferring a change that is already being built](../changes/archive/2026-10-18-0568-add-a-typed-way-to-clear-branch-on-a-proposed-change.md)**
<!-- docket:backlink:end -->

# Stop deferring a change that is already being built — Results

**Human action:** No action is required before merge. One optional walkthrough lets you see the new terminal messages for yourself.

## Outcome

Before this change, `docket change defer` accepted a change that was being built (`in-progress`). The deferred record kept its `branch:`, claim time, plan, and results, and reviving it later left a proposed change that `change.groom` could neither re-stack nor retitle, with no typed way out.

Now:

- `docket change defer` on an in-progress change refuses and writes nothing. The refusal has its own finding code, `not-deferrable`, and a plain message: block the change with a reason to pause it and keep its branch (`change.unblock` resumes it), or kill it to drop it. A kill leaves the git branch and worktree in place for you to delete. Deferring a proposed change works as before. Every other defer refusal keeps `illegal-source-status`.
- The four lifecycle commands (block, defer, unblock, revive) now print each refusal finding's message under the `<operation>: <result>` line. Before, the terminal showed only `change.defer: invalid-state`.
- `docket change kill` on a record that had a `branch:` returns a warning finding, `branch-left-behind`. It names the branch and says the local and remote branch and any worktree are untouched and can be deleted by hand. The warning is also printed in terminal output. A kill of a change with no branch prints exactly what it printed before. Kill still never probes or deletes git state.
- The convention skill, the glossary, the lifecycle concepts page, the upgrade guide, and the gate comments now say `deferred` is entered only from `proposed`.

## Human actions and testing

### Optional — see the refusal and the kill warning in a terminal

You might want to read the new messages as a user would see them. Prerequisites: a scratch git repository initialized with `docket repository init`, and a binary built from this branch (`go run ./cmd/docket` from the feature worktree).

1. Create a change with `docket change create`, groom it as trivial, and claim it with `docket change claim --id <id> --revision <rev>`.
   Expected: the change is `in-progress`, with a `branch:`.
2. Run `docket change defer --id <id> --revision <rev>`.
   Expected: `change.defer: invalid-state`, followed by an indented sentence starting `change NNNN is being built (in progress)` that names block and kill.
3. Run `docket change kill --id <id> --revision <rev>`.
   Expected: the applied kill line, followed by an indented warning that names the branch and says to delete it by hand if it is not needed.

Cleanup: delete the scratch repository.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the final head. The parallel run printed several `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` screening lines for integration shards and no `SERIAL CONFIRMED OVER BUDGET:` line.
- Each new guard was mutation-tested by its worker: the not-deferrable message path, the HumanText message lines, the finding-code registration, the kill warning append, and the receipt's HTML-safe encoding.
- Whole-branch review (standard tier): 3 findings (1 important, 2 minor), all fixed in-branch. The full table is in the PR body.

## Known issues and follow-ups

- **`change.groom` refusals print their message in raw form (suspected; same cause found while fixing review).** When `change.groom` refuses with `not-restackable` or another message-bearing refusal, the readable text sits in the finding's `message` detail. Groom's result mapping does not lift it out, so the text is shown as `not-restackable (message=...)`. Nothing breaks, and the text is still there. Workaround: read past the wrapper. Suggested next action: apply the same small override in groom's result mapping that this change added for lifecycle results. No existing change fits (checked 25), so a human would capture this as a new change, related to #563.
- **`illegal-source-status` lifecycle refusals print raw detail (confirmed).** For example, deferring a blocked change now prints `illegal-source-status (action=defer, from=blocked, reason=illegal-source-status)` under the result line. This is more than the terminal showed before, but it is not a plain sentence, because no plain-text explainer exists for that code. Impact is cosmetic. Suggested next action: add a plain explainer for illegal-source-status refusals. No existing change fits (checked 25), so a human would capture this as a new change.
