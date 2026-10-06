# Load a private repository's agent instructions without repository files: design

Change 0535 — Load a private repository's agent instructions without repository files

Change #535, groomed interactively on 2026-10-05, split out of #532 in the private-visibility series. Depends on #531 (private mode, `.git/dckt/`) and #534 (the `dckt` alias). #533 depends on it.

## Summary

In shared repositories, the parent agent learns docket's dispatch and run-tracker rules from the repository's own CLAUDE.md or AGENTS.md. A private repository can't carry those files. Instead:

- The rules live in a private instructions file under `.git/dckt/`.
- Each harness gets a trigger that loads that file. The triggers carry no rules and are installed once per machine.

Each trigger loads the file mechanically wherever the harness offers a way to do so, and nothing is written into the private repository's working tree. Promoted lessons move into the same private file.

## Evidence gathered at grooming

- **The dispatch rules exist only in the repository's own instructions file.** Since changes 0334 and 0351, the dispatch block (dispatch the named agent; bracket each implement-next run with `run.start` / `run.verdict`) is written only to a repository's own CLAUDE.md or AGENTS.md.
  - `docket install` writes no user-level copy and retires old ones (`GlobalDispatchTarget` in `internal/harness/claude/claude.go`; the codex and opencode adapters say the same).
  - 0334 removed the user-level copy because a second copy of the rules drifted.
  - At grooming, `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`, and `~/.config/opencode/AGENTS.md` contained no docket text.
- **Repository-level surfaces.** The `agent_harnesses` repository phase (`reposeed.Plan`, `installAuthorizedSurfaces` in `internal/app/repository_init.go`, `internal/app/repophase.go`) writes:
  - the AGENTS.md and CLAUDE.md dispatch blocks, with the interior from `harness.DispatchInterior` / `CodexDispatchInterior` in `internal/harness/dispatch.go`;
  - `.cursor/rules/docket-dispatch.mdc`.
- **Learnings promotion** lands a graduated rule in the integration-branch AGENTS.md or CLAUDE.md by hand (`skills/docket-convention/references/learnings.md`, *Promotion*).

### Delivery spike (2026-10-06)

The first build attempt halted at its per-harness spike. The human then had each delivery mechanism re-tested before re-grooming.

**How it was tested:**
- headless, fresh processes, with a stub `dckt`;
- temporary config only: Claude `--settings` / `--plugin-dir`, and `OPENCODE_CONFIG_DIR` for OpenCode. Cursor's user-level hooks file was created only for the test and removed afterwards;
- the payload was this repository's real 12.8KB dispatch block plus lessons, with a test rule as its last line, so truncation would show;
- each passing case was confirmed by having the model quote the run-tracker's step 2 and the first Shell lesson verbatim.

**Results:**

- **Claude Code 2.1.291.**
  - Each hook's `additionalContext` (or plain stdout) is capped at 10,000 characters, and the cap is measured per hook. Over the cap, Claude Code saves the output to a file and shows the model only the path and a 2,000-character preview.
  - With one hook carrying the whole file, the model's view ended in the middle of run-tracker step 1, so it lost step 2 (obey `run.verdict`) and every lesson.
  - Two hooks, one for the dispatch block (6,152 characters) and one for the lessons (6,601), delivered everything: 4/4 runs, in the primary checkout and a linked worktree. They added nothing outside git.
  - A plugin does not lift the cap:
    - a plugin hook has the same limit;
    - a plugin's `CLAUDE.md` is never loaded;
    - MCP server `instructions` were cut at about 2,000 characters.
- **OpenCode 1.18.31.**
  - The AGENTS.md pointer was ignored on a trivial prompt, 2/2.
  - A plugin using `experimental.chat.system.transform` to add `dckt instructions` output to the system prompt delivered everything: 5/5 on `deepseek-v4-flash`, primary checkout and linked worktree. It added nothing outside git.
- **Cursor CLI 2026.10.01.**
  - A user-level `~/.cursor/hooks.json` `sessionStart` hook returning `additional_context` delivered all 12.7KB: 5/5, primary checkout and linked worktree. It added nothing outside git.
  - User-level hooks run with their working directory set to `~/.cursor`. The project arrives as `CURSOR_PROJECT_DIR` and as `workspace_roots` in the stdin payload.
  - The excluded `.cursor/rules` file also worked in the primary checkout, but it does not exist in linked worktrees.
- **Codex 0.154.0.** The pointer was followed on a real task and skipped on a trivial prompt. This is accepted as best effort.

## Decisions (settled with the human)

1. **The private instructions file.** A private repository's parent-facing rules live in `<git-common-dir>/dckt/AGENTS.md`: the dispatch block plus promoted lessons.
2. **Mechanical delivery wherever the harness offers it**, re-decided after the delivery spike:
   - **Claude Code:** two user-level `SessionStart` hook entries in `~/.claude/settings.json`, one for the dispatch block and one for the lessons. `docket install` adds them and `docket uninstall` removes them. The human chose settings entries over a docket-owned Claude plugin: a plugin would change only the packaging, not what the model sees.
   - **Cursor:** one user-level `sessionStart` hook entry in `~/.cursor/hooks.json`.
   - **OpenCode:** a docket-owned plugin file in `~/.config/opencode/plugins/`.
   - **Codex:** a static pointer block in `~/.codex/AGENTS.md`. Whether the model follows it is up to the model; the human accepted that as best effort.
3. **No drift.** No user-level surface carries rule text. The rules have one source, so the drift that retired the user-level copy cannot recur.
4. **Neutral spelling.** Every trigger invokes `dckt` (#534). The pointer's managed markers use the `dckt:` prefix, and the plugin file is `dckt-instructions.js`, so nothing in the user's home directory reads "docket".
5. **Nothing in the private working tree.** This replaces the earlier excluded Cursor rule file. The user-level Cursor hook makes that file unnecessary, and an excluded file would be missing from every linked worktree.
6. **Split by meaning, not by size (Claude).** Each Claude hook carries a complete document: the dispatch block, or the lessons. That way the order in which they arrive doesn't matter.
   - Docket generates the dispatch block, so it is kept under the 10,000-character cap and pinned by a test.
   - Lessons that grow past the cap fall back to Claude Code's own path-plus-preview. That is visible to the model and never stops anything.
7. **Scope.** `docket install` writes the user-level triggers once per machine. The private file is written per repository. `agent_harnesses` stays a per-repository choice, exactly as in shared mode.

## Design

### The private instructions file

In private repositories:

- `init --private`, and `install`'s repository phase, write the managed dispatch block into `<git-common-dir>/dckt/AGENTS.md`. It is the same interior the repository-level block carries today, selected by `agent_harnesses`, Codex clause included.
- They write nothing in the worktree: no AGENTS.md or CLAUDE.md dispatch block and no Cursor rule file. They add nothing to `.git/info/exclude` either.
- The learnings promotion destination is this file, and the learnings reference says so.
- Blocks committed before the repository went private are #533's `--remove-shared-files` concern.

### `docket instructions`

A new read-only catalog operation.

- **In a private repository:** prints the private instructions file from any worktree of the clone. It resolves through the git common directory, because a feature worktree's `.git` is a file.
- **Anywhere else:** prints **nothing** and exits 0. That covers shared repositories, non-docket repositories, and running outside git. It never fails noisily, because it runs in every session.
- **`--section dispatch|lessons`:** `dispatch` prints the managed dispatch block, markers included. `lessons` prints everything outside it. A section with no content prints nothing.
- **`--hook claude`:** wraps the selected content in Claude Code's `SessionStart` hook output (`hookSpecificOutput.additionalContext`). It prints nothing outside private repositories or when the content is empty.
- **`--hook cursor`:** wraps the content in Cursor's `sessionStart` hook output (`{"additional_context": …}`).
  - Cursor runs user-level hooks from `~/.cursor`, so this mode finds the repository from `CURSOR_PROJECT_DIR`, or failing that from the first `workspace_roots` entry of the hook's stdin payload.
  - It prints nothing when neither names a private repository.
- **Hook modes never fail a session:** any error means no output and exit 0.

### User-level triggers

`docket install` writes these when it installs the corresponding harness. They are installed even on machines with no private repository yet, and are no-ops everywhere else.

| Harness | Surface | What it does |
|---|---|---|
| Claude Code | two `SessionStart` command entries in `~/.claude/settings.json`: `dckt instructions --hook claude --section dispatch` and `dckt instructions --hook claude --section lessons` | loads the dispatch block and the lessons at session start, each under Claude Code's per-hook cap; adds nothing outside private repositories |
| Cursor | one `sessionStart` command entry in `~/.cursor/hooks.json` (`"version": 1`): `dckt instructions --hook cursor` | loads the whole file at session start; adds nothing outside private repositories |
| OpenCode | plugin file `~/.config/opencode/plugins/dckt-instructions.js` | adds `dckt instructions` output to the system prompt on every model call |
| Codex | managed pointer block in `~/.codex/AGENTS.md` | asks the model to run `dckt instructions` once per session and follow its output (best effort) |

The OpenCode plugin and the Codex pointer wording are final in the plan. Both must stay rule-free and free of the word "docket". The plugin, as proven in the spike:

```js
// Adds a private repository's instructions to the system prompt.
// `dckt instructions` prints nothing outside a private repository.
export const DcktInstructions = async ({ $, directory }) => ({
  "experimental.chat.system.transform": async (_input, output) => {
    const text = await $`dckt instructions`.cwd(directory).quiet().nothrow().text()
    if (text.trim()) output.system.push(text)
  },
})
```

The Codex pointer:

```markdown
<!-- dckt:private-instructions:start (managed — do not hand-edit) -->
## Private repository instructions

At the start of a session inside a git repository, run `dckt instructions` once.
If it prints anything, treat that output as this repository's own AGENTS.md and
follow it for the rest of the session. If it prints nothing, ignore this section.
<!-- dckt:private-instructions:end -->
```

**Safety requirements:**

- **JSON hook entries** (Claude's `settings.json`, Cursor's `hooks.json`):
  - Each entry is identified by its exact command string.
  - Install adds an entry only when it is absent, and merges it without reformatting or dropping any other setting or hook.
  - Install creates `~/.cursor/hooks.json` with `"version": 1` when the file is absent.
  - Uninstall removes only an exact match, and reports a modified entry rather than touching it.
- **The OpenCode plugin file** is wholly docket-owned and identified by its exact bytes.
  - Install writes it when absent and does nothing when it is identical.
  - It reports a modified file and never overwrites it.
  - Uninstall removes only an exact match.
- **The Codex pointer block** uses the managed-block machinery, extended to accept the `dckt` marker prefix with the same guarantees: closed-block guard, idempotence, outside bytes preserved, and ownership-proved retirement.
- **This is a deliberate, narrow return of user-level parent-facing writes**, which 0351 retired. It is acceptable because the surfaces carry only a trigger, never rules.
- **Without the alias** (#534 reports a foreign `dckt`), the triggers fail harmlessly: the hooks print nothing, the plugin gets empty output, and the pointer's command fails. `docket instructions` still works by hand.

**Known limits, accepted:**
- OpenCode's hook is `experimental.`, so a future OpenCode release may rename it.
- Cursor documents `sessionStart` as fire-and-forget (it was delivered in every spike run).
- Only the Cursor CLI was exercised, not the desktop app.
- Lessons past 10,000 characters reach Claude as Claude Code's path-plus-preview.

### Fresh-session acceptance (replaces the spike task)

The grooming spike proved each mechanism with a stub. The build repeats the check against the real built `dckt`:

- in fresh headless sessions, with temporary configuration only;
- Cursor's user-level hook is exercised through a project-level `hooks.json` in a temporary fixture, plus a run from an unrelated working directory with `CURSOR_PROJECT_DIR` set;
- results are recorded in the results file.

If Claude Code, Cursor, or OpenCode fails, the build stops, because their mechanisms are proven. A Codex result is recorded and never stops the build. A harness that can't be run (missing CLI or login) is recorded as an Important human verification item, as is a real user-level Cursor `~/.cursor/hooks.json` check.

### Documentation

- `docket instructions` command help.
- The skills:
  - convention: the private promotion destination;
  - agent-layer reference: private delivery, listing the four triggers.
- No `docs/` pages.

## Acceptance criteria

1. **The private file.** In a private repository, `<git-common-dir>/dckt/AGENTS.md` carries the dispatch block (Codex clause when selected). Nothing is written in the worktree (no AGENTS.md, CLAUDE.md, or Cursor rule file), and nothing is added to `.git/info/exclude`.
2. **`docket instructions` output:**
   - it prints the file from the primary worktree, a feature worktree, and the metadata worktree;
   - it prints nothing and exits 0 in a shared repository, a non-docket repository, and outside git;
   - `--section dispatch` prints exactly the managed block, and `--section lessons` prints everything else;
   - `--hook claude` emits valid `SessionStart` output only in private repositories, and only for non-empty content;
   - run from an unrelated working directory, `--hook cursor` finds the repository from `CURSOR_PROJECT_DIR`, or failing that from the stdin `workspace_roots`, and emits `additional_context` only for a private repository.
3. **Installing the triggers:**
   - install adds the two Claude hook entries, the Cursor hook entry, the OpenCode plugin file, and the Codex pointer block;
   - a second install is a no-op;
   - an unrelated `settings.json` or `hooks.json` key and hook survive byte-for-byte;
   - uninstall removes only docket's exact entries, file, and block, and reports a modified one without touching it.
4. **No rule text, no "docket".** The triggers contain no dispatch-block sentence and no word "docket", pinned by a test that fails if either appears.
5. **The dispatch block fits one Claude hook.** The dispatch block docket generates stays under 10,000 characters for every `agent_harnesses` selection, pinned by a test.
6. **Learnings promotion** in a private repository names the private file as its destination.
7. **Shared repositories are unchanged:** repository-level blocks, and no private file.
8. **Fresh-session acceptance per harness** is recorded in the results file, as *Fresh-session acceptance* describes. In a private repository with no repository instruction files, a Claude Code, Cursor, and OpenCode session each receives the complete dispatch rules.

## ADRs expected

A private repository's parent-facing rules live in `<git-common-dir>/dckt/AGENTS.md`. They are reached through content-free user-level triggers:

- two Claude Code `SessionStart` hooks, split by meaning to fit the per-hook cap;
- a Cursor `sessionStart` hook;
- an OpenCode system-prompt plugin;
- a Codex pointer block (best effort).

This narrowly revisits 0351's retirement of user-level parent-facing writes. Relates to ADR-0036 and ADR-0078.

## Out of scope

- Shared repositories.
- Writing rules and the leak check (#532).
- The `dckt` alias (#534).
- Moving instructions on a mode switch (#533).
