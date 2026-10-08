<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0543 — Cursor section of the Bash upgrade guide, proven on the saved cases](../../changes/archive/2026-10-08-0543-cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved.md)**
<!-- docket:backlink:end -->

# Cursor section of the Bash upgrade guide, proven on the saved cases

**Change:** 0543 · **Type:** docs · **Priority:** high · **Groomed:** 2026-10-08 · **Status:** Approved design

## Purpose

`docs/release/upgrading-from-bash.md` (change 0511) moves a Bash docket install to the Go binary
for Claude Code only. This change extends it to Cursor, proven by the same
`internal/bashupgrade` integration test against the saved `v0.9.2` and `v0.9.3` cases, which
already hold the Bash Cursor files. It must merge before the `v1.0.0-alpha.2` candidate is cut
(change 0512), so the test runs inside that candidate's source gate.

No product code changes. If the test exposes something the binary does wrong, rather than
something the guide can explain, the build halts and that bug gets its own fix change, as in 0511.

## What the saved cases show (traced 2026-10-08)

A scratch reproduction ran the current `docket install --harness cursor` against each restored
saved home:

| Path | v0.9.2 | v0.9.3 |
|---|---|---|
| `~/.cursor/agents/docket-*.md` | taken over by the installer (legacy byte match) | taken over, except `docket-plan-writer.md`: conflict |
| `~/.cursor/skills/docket-*` (links into the old checkout) | conflict | conflict |
| `~/.cursor/rules/docket-dispatch.mdc` (Bash's user-level dispatch rule) | removed by the installer (matches the frozen v0.9.2 rule) | conflict (v0.9.3's bytes differ from the frozen floor) |

After `rm ~/.cursor/skills/docket-*` and
`rm -f ~/.cursor/agents/docket-plan-writer.md ~/.cursor/rules/docket-dispatch.mdc`, the second
install reports `applied` on both cases. `install check` is then clean except the `runtime.bash`
`obsolete-setting` warning, which the guide's existing global-config step clears. Nothing under
`~/.cursor` links into the old checkout. The installer also writes `~/.cursor/hooks.json`.

The v0.9.3 rule conflict is by design (ADR-0096: the legacy reproducer is a frozen v0.9.2
floor), so it is a guide remedy, the same way `docket-plan-writer.md` already is for Claude Code.

There is no Cursor counterpart of the repository `CLAUDE.md` dispatch block: Bash wrote its Cursor
rule at user level. The Go binary writes `.cursor/rules/docket-dispatch.mdc` per repository when
`agent_harnesses` lists `cursor`, and the guide's `.gitignore` block already ignores it.

## Design (human, 2026-10-08)

**One combined path.** The guide has a single flow for Claude Code and Cursor. Each harness's
lines are labelled, and the reader drops the harness they don't use. The test runs each saved case
once, through both harnesses. A separate Claude-only run is not kept: it would differ by one
`--harness` flag, and each harness's install targets are separate. Every existing Claude Code step
and assertion stays, because the test is what keeps the guide's Claude Code half true against each
new binary.

### Guide changes

- **§1 Who this is for.** "You use docket with Claude Code, Cursor, or both." Cursor is covered;
  OpenCode follows in a later pre-release; Codex is not supported.
- **§3 Install the docket binary.** `VERSION=v1.0.0-alpha.2`. The installer line becomes
  `sh install.sh --harness claude --harness cursor`, with a sentence to drop the `--harness` you
  don't use. The text about the first run stopping with `conflict` lines is unchanged.
- **§4** becomes "Take over the old Claude Code and Cursor install".
  - The existing Claude Code text and its marked `takeover-remedy` step stay. Its re-run line
    becomes the same two-harness `sh install.sh` line as §3.
  - A Cursor paragraph: the installer takes over Bash's agent files under `~/.cursor/agents/` by
    itself. It reports as conflicts every `docket-*` link under `~/.cursor/skills/`, and on `v0.9.3`
    `~/.cursor/agents/docket-plan-writer.md` and Bash's user-level rule
    `~/.cursor/rules/docket-dispatch.mdc`. On `v0.9.2` the installer removes that rule itself.
  - A new marked step, `cursor-takeover-remedy`, run before the re-run:

    ```sh
    rm ~/.cursor/skills/docket-*
    rm -f ~/.cursor/agents/docket-plan-writer.md ~/.cursor/rules/docket-dispatch.mdc
    ```

  - The global-config cleanup and `confirm-install` steps are unchanged.
- **§5 Upgrade each repository.** The final `configure-harnesses` example becomes
  `--harnesses claude,cursor`. For Cursor, it writes `.cursor/rules/docket-dispatch.mdc` into the
  repository, which the `.gitignore` block keeps out of commits.
- **§7 Leftovers you can delete.**
  - Nothing under `~/.claude` or `~/.cursor` points into the old checkout after the upgrade. The
    links Bash made under `~/.codex` and `~/.agents` still do. Using docket from OpenCode on an
    upgraded repository is not supported until its section arrives; Codex is not supported.
  - The marked `repo-agent-files` step also runs `rm -f .cursor/agents/docket-*.md`. The saved
    cases carry no repository Cursor agent files, so the guide says the test does not prove that
    line.
  - A sentence that docket must run outside Cursor's sandbox, linking `docs/install/cursor.md`.
- **§8** becomes "Restart Claude Code and Cursor": quit each and start it again; both load agents
  and skills at startup.

### Test changes (`internal/bashupgrade`)

The guide-driven run (`TestIntegrationBashUpgradeGuide`) executes the new and changed marked steps
like every other. New assertions:

- The first install's conflicts include every `~/.cursor/skills/docket-*` link on both cases, and on
  `v0.9.3` also `~/.cursor/agents/docket-plan-writer.md` and `~/.cursor/rules/docket-dispatch.mdc`.
- After the guide, `~/.cursor/rules/docket-dispatch.mdc` does not exist on either case.
- The clean end state covers both harnesses: `docket install check --json` reports `cursor` among
  its harnesses with no error or warning finding.
- Nothing under `~/.cursor` links into the old checkout. The existing assertion that `~/.cursor`
  still has such links flips to require zero; `~/.codex` and `~/.agents` keep requiring at least one.
- The clone has `.cursor/rules/docket-dispatch.mdc` after the final `configure-harnesses` step,
  with any guide-shape check the test already applies to marked steps.

Existing assertions stay, including the guide-shape and finding/settings table checks. The test
stays in the integration build tag and inside the whole-suite gate.

## Out of scope

- OpenCode (alpha.3, change 0513) and Codex.
- The release itself (change 0512).
- Retiring the saved cases and the test (change 0514).
- A Claude-only test run, and new saved cases (for example repository `.cursor/agents` files).
- Any change to the binary's takeover rules, including teaching the frozen floor v0.9.3's rule.

## Acceptance

- The guide reads as above, and every marked step runs in the test.
- `TestBashUpgrade` passes on both saved cases with the new Cursor assertions, inside the whole
  suite.
- Mutation check: deleting the `cursor-takeover-remedy` step from the guide makes the test fail.
