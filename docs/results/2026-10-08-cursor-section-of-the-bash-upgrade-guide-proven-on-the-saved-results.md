<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0543 — Cursor section of the Bash upgrade guide, proven on the saved cases](../changes/archive/2026-10-08-0543-cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved.md)**
<!-- docket:backlink:end -->

# Cursor section of the Bash upgrade guide, proven on the saved cases — Results

**Human action:** No action is required before merging. The Cursor path is proven by the integration test against both saved Bash installs. Running it by hand in a real Cursor install is optional.

## Outcome

The Bash upgrade guide (`docs/release/upgrading-from-bash.md`) now covers Claude Code and Cursor in one flow. Before this change it covered Claude Code only and told Cursor users their setup was not supported. Each harness's lines are labelled, so a reader who uses only one harness drops the other's.

- The installer line is pinned to `v1.0.0-alpha.2` and names both harnesses.
- A new runnable step, `cursor-takeover-remedy`, deletes the Bash Cursor links and files that the installer reports as conflicts. These are `~/.cursor/skills/docket-*` on both saved versions, plus `docket-plan-writer.md` and the user-level `docket-dispatch.mdc` rule on v0.9.3.
- The final per-repository `configure-harnesses --harnesses claude,cursor` command used to be inline prose. It is now a runnable step, `choose-harnesses`, so the test executes it. It writes `.cursor/rules/docket-dispatch.mdc` into the repository.
- The leftovers section now covers `~/.cursor`, the repository `.cursor/agents` files, and the Cursor sandbox note. The last section is now "Restart Claude Code and Cursor".

The saved-case integration test (`TestIntegrationBashUpgrade`) runs the whole guide through both harnesses on v0.9.2 and v0.9.3. It asserts four things:
- the expected Cursor conflicts appear;
- no user-level rule remains;
- `install check` is clean for both harnesses;
- nothing under `~/.cursor` links into the old checkout, and the repository rule is written.

The binary showed no behaviour the guide could not explain, so no product code changed.

Two departures from the spec:
1. The post-guide check that `CLAUDE.md` has no `docket:dispatch:` marker would fail on a correct upgrade, because docket now writes its own block under that marker. The check now keys on the Bash block's own signature, and a new positive check confirms docket's block is present.
2. The guide no longer says the v0.9.2 installer removes the user-level rule itself. On the guide's path the reader's `rm -f` deletes it first. The sentence now says it is not reported as a conflict and that the block deletes it either way.

## Human actions and testing

### Optional — Walk the guide in a real Cursor install

The test runs the guide in a sandboxed home directory. It does not open Cursor itself. This walkthrough confirms that Cursor loads the upgraded agents and rule.

Prerequisites: a machine with a Bash docket v0.9.x install that is linked into Cursor, and a repository that uses it.

1. Follow `docs/release/upgrading-from-bash.md` from section 3 to the end, keeping both harnesses.
   Expected: the second install reports `applied`, and `docket install check` reports no errors for `claude` or `cursor`.
2. Restart Cursor and open the repository.
   Expected: the docket agents are listed, and `.cursor/rules/docket-dispatch.mdc` exists in the repository.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the final head through the gate driver, and the evidence was recorded and verified. It also passed at the pre-fix head.
- The budget report had no `SERIAL CONFIRMED OVER BUDGET` line. It had screening-only `BUDGET WATCH` / `PARALLEL-SENSITIVE` lines for unrelated finalize, app-integration and race test files. None touch `internal/bashupgrade`, which ran in 49s.
- Workers mutation-checked the new guards. Each of these made the test fail:
  - deleting `cursor-takeover-remedy`;
  - dropping the expected v0.9.3 rule conflict;
  - flipping the `~/.cursor` link expectation;
  - narrowing the `git add` line;
  - deleting `choose-harnesses`;
  - reverting the `CLAUDE.md` marker check;
  - deleting the reworded v0.9.2 sentence.
- Whole-branch review (standard tier): 3 minor findings, all fixed in-branch. The full table is in the PR body.

## Known issues and follow-ups

### Two guide lines are not proven by the saved cases

The saved installs do not exercise two lines in the guide. The repository `rm -f .cursor/agents/docket-*.md` line is untested because the saved cases carry no repository Cursor agent files. The guide says so. The v0.9.2 installer's own removal of the user-level rule is never reached on the guide's path, because the reader deletes the rule first. That removal is covered by the existing `internal/install` retirement tests. Both are confirmed limitations, and neither has a practical impact on readers. Adding such saved cases is out of scope by the spec, so no action is suggested.
