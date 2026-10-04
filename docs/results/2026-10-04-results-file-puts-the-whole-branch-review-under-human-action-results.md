<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0498 — Results file puts the whole-branch review under Human actions and testing](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0498-results-file-puts-the-whole-branch-review-under-human-action.md)**
<!-- docket:backlink:end -->
# Results file puts the whole-branch review under Human actions and testing — Results

**Human action:** None required. This change only rewords guidance for future results files; nothing about existing behavior or stored data changes.

## Outcome

Results files had no named place for whole-branch review outcomes, so runs filed them in different sections. Change 0494 put them under Human actions and testing, and other runs put "fixed after review" notes under Known issues. The guidance now names one home for each kind of review outcome:

- The PR body is the only place for the full per-finding review table.
- Verification performed gets one summary line: which review ran and how its findings ended.
- Known issues and follow-ups gets an entry for every finding left unfixed or reported as follow-up work, plus any fixed finding that still leaves a real risk.
- During the build, the full findings may still be written to the results file so they survive a halt before a PR exists. The final write condenses them to the summary line. This is stated as the one exception to the rule that a checkpoint never truncates earlier content.
- Human actions and testing holds only what a human should do or check, never a record of checks the run already performed.

The wording lives in the results template, implement-next's Step 6.5, and the fix-loop reference, with the embedded copies regenerated. A new prose-contract test pins it. No program behavior changed.

## Verification performed

- Full suite (the build gate command) passed after the build. A final certifying run on the head that contains this file is recorded in the PR body's build-evidence block.
- Every phrase the new guard pins was mutation-tested: removing each one, moving the Step 6.5 sentence out of its section, or putting back the retired fix-loop sentence makes the guard fail. Restoring the text makes it pass again.
- The results-template tests still treat the new Verification performed guidance as an unfilled prompt, so a results file that pastes it unchanged is still refused.
- Whole-branch review (standard tier): 2 minor findings, both fixed in-branch; full table in the PR body.
