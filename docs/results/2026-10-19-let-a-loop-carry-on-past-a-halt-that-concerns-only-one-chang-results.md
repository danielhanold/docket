<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0570 — Let a loop carry on past a halt that concerns only one change](../changes/active/0570-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang.md)**
<!-- docket:backlink:end -->

# Let a loop carry on past a halt that concerns only one change — Results

**Human action:** Restart Claude Code sessions after this merges and the binary is reinstalled, then optionally watch one loop carry on past a halt. No other action is needed.

## Outcome

Before this change, every `halted` run stopped an implement-next loop. That meant one change that needed a person ended an overnight drain.

Now a tracked implement-next loop keeps going after a run halts on a change it picked from the queue. The halted change keeps its "run halted — needs you" marker, and the loop moves on to the next build-ready change. A per-repository count of halts in a row is the backstop. When the count reaches `run.max_consecutive_halts` (a new key, default 2, minimum 1), the loop stops. The verdict then adds a note asking the person whether to reset the count with the new `docket run reset-halts` command, and the agent never runs it without a yes.

- **New verdict line.** `run-next <key> run-halted <id>` means: never re-dispatch that change, but a loop driver may start a fresh `run.start`. A single dispatch stops on it, just as it stops on `run-stop`.
- **What the count counts.** Only halts on queue-picked changes add to it. The claim now takes `--selection queue|named`. Without the flag the selection is "unknown": it counts, but prints today's line.
- **What resets it.** Any clean build (`run-complete`), whether named or queue-picked.
- **What stays exactly as before.** Named runs, resumes, retries, continuations, observe mode and finalize print today's lines. Faults in the count fall back to today's `run-stop`, and no schema was bumped.
- **Docs.** They now say docket supports one implement loop and one finalize loop at a time.

Departures from the spec:

- **The named path passes `--selection named` explicitly.** The spec said the named path passes nothing, but an absent flag reads as "unknown", which counts. So passing nothing would have broken the rule that a named halt never touches the count.
- **A visibility switch moves the count with the state folder.** The spec said the switch effectively resets it.
- **The limit is read when the verdict runs**, not snapshotted at `run.start`.

## Human actions and testing

### Important — Restart sessions after install

The dispatch block in AGENTS.md/CLAUDE.md and the implement-next skill both changed. A session started before the reinstall still runs the old prose: it passes no `--selection` flag, so its halts count as "unknown" and the loop stops as it does today. That is safe, but the new behavior stays off until you restart.

1. After the merge, the binary rebuild and the reinstall, quit and restart every Claude Code session that drives docket.
   Expected: in a new session, `docket capabilities --json` lists `run.reset-halts`.

### Optional — Watch a loop carry on past a halt

Do this only if you want to see the behavior end to end.

You need at least two build-ready changes in a scratch repository, one of which will halt. A spec contradiction that the reconcile step rejects is enough to make it halt.

1. Run `/loop docket-implement-next` there.
   Expected: the first change halts, its verdict prints `run-next … run-halted <id>`, and the loop starts the next change.
2. Let a second change halt in a row.
   Expected: the loop prints `run-stop … run-halted <id>` plus a note asking whether to reset the halt count.

Cleanup: answer yes to the note, or run `docket run reset-halts`. Kill or resume the halted scratch changes.

## Verification performed

- **Full suite.** The build gate (`go run ./cmd/docket development test`) was red on its first run for one reason: a new test file used a bare `t.TempDir()`. That was repaired. The gate was then green, and green again on the final head after the review fixes.
- **Mutation testing.** Every isolation guarantee was mutation-tested during the build: the named early return, resume handling, replayed decisions, reset only on `run-done`, count faults keeping today's line, the unknown selection, and the at-limit note.
- **Budget report.** It showed only BUDGET WATCH lines: parallel-overrun streaks of up to 4/5 on existing integration shards. No breach lines appeared.
- **Whole-branch review (deep tier).** It returned 2 findings, one important and one minor, and both were fixed in-branch. The full table is in the PR body.
- **Backlog check.** No out-of-scope follow-ups came up, so the backlog match had nothing to check.

## Known issues and follow-ups

### A claim reserved by an older binary reads as "unknown"

This applies to a run that was reserved under the pre-install binary and then claimed after the install. The binding carries no selection, so a halt on that run counts toward the limit but prints today's `run-stop`. Its practical impact is that, for loops spanning the install, the loop stops instead of carrying on. This is the safe direction. It is confirmed by reading the code and is not worth fixing. Workaround: do not let a loop span the install. The next action is none.

### Two implement loops at once break the halt limit

If two implement loops run at once, they share one count, so a healthy loop's clean builds keep resetting a broken loop's halts. This is a documented support boundary, not a gate. The next action is none.
