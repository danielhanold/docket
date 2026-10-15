<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0552 — Install Claude Code allow rules so finalize can publish and merge under auto mode](../changes/archive/2026-10-15-0552-install-claude-code-allow-rules-so-finalize-can-publish-and.md)**
<!-- docket:backlink:end -->

# Install Claude Code allow rules so finalize can publish and merge under auto mode — Results

**Human action:** No action is required before merge. One optional check on a real machine in auto mode confirms that the classifier no longer stops finalize.

## Outcome

Claude Code's auto mode used to soft-deny `docket finalize publish` and `docket finalize merge` when finalize reused the green suite run from implement-next. It read the reuse as a "CI bypass", so a clean finalize stopped on a prompt. Now `docket install` writes two allow rules into `permissions.allow` in the user-level `~/.claude/settings.json`: `Bash(docket finalize publish:*)` and `Bash(docket finalize merge:*)`. They sit beside the two SessionStart hooks install already manages there.

- Every install converges the file. A rule you deleted by hand comes back. A rule a newer docket no longer ships is removed. A file that already has both rules is left byte-for-byte unchanged. A rule you typed yourself is not duplicated.
- Uninstall removes the rules along with the hooks. A hand-typed copy of the same string is removed too, the same way the hooks behave.
- If docket cannot safely edit the file (a symlink, unparseable JSON, or a `permissions` / `allow` value of the wrong type), install skips it with a warning. Every byte you wrote is preserved.
- A test ties each rule to the capability catalog's spelling of `finalize.publish` / `finalize.merge`, so renaming either command fails the test.
- The install docs list the rules. ADR-0150 records why this narrow grant is user-level.

One deliberate behavior change: when `permissions` in `settings.json` has the wrong type, install now skips the whole file, so the SessionStart hooks are not installed either, and uninstall refuses to edit that file. See Known issues.

## Human actions and testing

### Optional — confirm auto mode lets finalize through

The rules are proven against the classifier only by the original report from another machine. No automated test can drive Claude Code's auto-mode classifier.

Prerequisites: this branch merged and installed (`docket development install --source <repo>`), Claude Code running in auto mode, and one change that implement-next has just built with a green suite.

1. Open `~/.claude/settings.json`.
   Expected: `permissions.allow` contains `Bash(docket finalize publish:*)` and `Bash(docket finalize merge:*)`.
2. Ask the session to finalize that change right after the implement-next run.
   Expected: finalize skips the suite re-run, then publishes and merges without a permission prompt or a "CI bypass" denial.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the final head through the build gate, both after the build and again after the review fixes. The budget report showed screening-only `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines and no `SERIAL CONFIRMED OVER BUDGET:` line.
- Workers mutation-tested the new guards: the round-trip insert/remove tests, the digest test that keeps a hooks-only record valid, the inspection add/retire logic, the uninstall removal, and the catalog drift guard. Each went red under its mutation and green after the restore.
- Whole-branch review (standard tier) returned 3 findings. 2 were fixed in-branch: the trigger.go header now limits the content-free property to the hook commands, and the invalid-`permissions` remedy no longer offers a hand-grant alternative. 1 was deferred as an accepted design trade (below). The full table is in the PR body.
- Plan deviation: `skills/docket-convention/references/agent-layer.md` describes the rules by operation name instead of literal `docket finalize …` text, because a repository guard forbids hard-coded command spellings on skill surfaces. The literal rules appear in `docs/install/`.

## Known issues and follow-ups

- **A malformed `permissions` value now also blocks the hooks.** When `~/.claude/settings.json` has `permissions` that is not an object, or `permissions.allow` that is not an array, install skips the whole file and warns, so the SessionStart hooks are not installed either. Uninstall also refuses to edit that file. Before this change, the hooks were still managed. The spec accepts this trade so that docket edits only a file it fully understands. Confirmed by design. Workaround: fix the `permissions` value and re-run `docket install`. Suggested next action: none unless users hit it.
- **An empty `allow` or `permissions` you wrote yourself disappears after install then uninstall.** If removing the rules leaves `allow` empty, it is removed, and an emptied `permissions` goes with it. That happens even when you created the empty value before installing. This matches how hooks behave and is pinned by a test. Practical impact: cosmetic only. Suggested next action: none.
