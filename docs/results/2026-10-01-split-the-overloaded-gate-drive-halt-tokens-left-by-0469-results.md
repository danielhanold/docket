<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0481 — Split the overloaded gate-drive halt tokens left by 0469](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0481-split-the-overloaded-gate-drive-halt-tokens-left-by-0469.md)**
<!-- docket:backlink:end -->
# Split the overloaded gate-drive halt tokens left by 0469 — Results

**Human action:** None needed beyond the normal PR read. This change only renames halt reasons, and automated tests cover every renamed site.

## Outcome

When the gate driver stops a test run ("halts"), it reports a short reason token that tells the operator what to fix. Change 0469 left two of those tokens covering a second condition they do not describe. That pointed operators at the wrong remedy. Each condition now gets its own token:

- **A takeover finds the drive's recorded scope no longer matches** (its branch, worktree, change, task or phase drifted). This now halts `scope-identity-mismatch`, the token `start` already uses for the same mismatch. Before, it halted `worktree-changed`, which suggests the worktree's files changed.
- **The drive loses the link to its run.** This covers a worktree slot that is absent or unreadable, and one that was reassigned to a different reservation. Both now halt with a new token, `run-link-lost`. Before, they halted `launch-unconfirmed`, which suggests a launch of uncertain status. The remedy is to cancel the run and start fresh.

When the driver halts and how it recovers are unchanged. The correct `worktree-changed` and `launch-unconfirmed` sites stay as they were. Finalize treats both new tokens like the old ones (as "unavailable"). The glossary has an updated *Worktree changed* entry and a new `run-link-lost` entry. ADR-0129 has a dated `## Update` that records the split.

## Verification performed

- Each task was test-driven. For the five takeover scope-drift cases and the two lost-link cases, a test failed first and then passed after the fix.
- Each emitting site was mutation-tested: putting the old token back makes its tests fail. Each of the two lost-link sites has its own failing test.
- The full suite runs at the build gate, and the build-evidence record in the PR body records the result.
- A standard-tier whole-branch review returned one minor finding: the glossary's list of takeover identity fields left out "repo". It was fixed in-branch (187f48544).

## Known issues and follow-ups

### Finalize mapping rows are pinned but not mutation-tested

New `mapDriveHaltCause` test rows check that both new tokens map to finalize's "unavailable" outcome. That mapping is a fall-through, so no mutation was run against it. Impact is low: finalize already treats any unrecognised token as unavailable. No action is needed.
