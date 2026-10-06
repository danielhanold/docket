<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0535 — Load a private repository's agent instructions without repository files](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0535-load-a-private-repository-s-agent-instructions-without-repos.md)**
<!-- docket:backlink:end -->

# Load a private repository's agent instructions without repository files: design

Change #535, groomed interactively on 2026-10-05, split out of #532 in the private-visibility series. Depends on #531 (private mode, `.git/dckt/`) and #534 (the `dckt` alias). #533 depends on it.

## Summary

In shared repositories, the parent agent learns docket's dispatch and run-tracker rules from the repository's own CLAUDE.md or AGENTS.md. A private repository can't carry those files. Instead, it gets a private instructions file under `.git/dckt/`, plus one static, rule-free trigger per harness, installed once per machine, that loads it. Promoted lessons move into the same private file.

## Evidence gathered at grooming

- **The dispatch rules exist only in the repository's own instructions file.** Since changes 0334 and 0351, the dispatch block (dispatch the named agent; bracket each implement-next run with `run.start` / `run.verdict`) is written only to a repository's own CLAUDE.md or AGENTS.md.
  - `docket install` writes no user-level copy and retires old ones (`GlobalDispatchTarget` in `internal/harness/claude/claude.go`; the codex and opencode adapters say the same).
  - 0334 removed the user-level copy because a second copy of the rules drifted.
  - At grooming, `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`, and `~/.config/opencode/AGENTS.md` contained no docket text.
- **Repository-level surfaces.** The `agent_harnesses` repository phase (`reposeed.Plan`, `installAuthorizedSurfaces` in `internal/app/repository_init.go`, `internal/app/repophase.go`) writes:
  - the AGENTS.md and CLAUDE.md dispatch blocks, with the interior from `harness.DispatchInterior` / `CodexDispatchInterior` in `internal/harness/dispatch.go`;
  - `.cursor/rules/docket-dispatch.mdc`.
- **Learnings promotion** lands a graduated rule in the integration-branch AGENTS.md or CLAUDE.md by hand (`skills/docket-convention/references/learnings.md`, *Promotion*).
- **Claude Code `SessionStart` hooks** configured in user settings inject their output as session context. This session itself received plugin context that way.

## Decisions (settled with the human)

1. **The private instructions file.** A private repository's parent-facing rules live in `<git-common-dir>/dckt/AGENTS.md`: the dispatch block plus promoted lessons.
2. **Delivery is a hybrid, chosen over a pointer for every harness:**
   - Claude Code loads the file mechanically through a user-level `SessionStart` hook.
   - Codex and OpenCode get a static pointer in their user-level AGENTS.md.

   The hook is a guarantee where the harness offers one, and the pointer is the best available elsewhere.
3. **No drift.** Neither user-level surface carries rule text. The rules have one source, so the drift that retired the user-level copy cannot recur.
4. **Neutral spelling.** Both surfaces invoke `dckt` (#534), and the pointer's managed markers use the `dckt:` prefix, so nothing in the user's home directory reads "docket".
5. **Cursor.** It reads rules only from the project, so a private repository gets `.cursor/rules/dckt-dispatch.mdc`, excluded through `.git/info/exclude`. The human accepted this as the one visible trace in a private working tree.
6. **Scope.** The user-level triggers are written once per machine by `docket install`. The private file is written per repository. `agent_harnesses` stays a per-repository choice, exactly as in shared mode.

## Design

### The private instructions file

In private repositories:

- `init --private`, and `install`'s repository phase, write the managed dispatch block into `<git-common-dir>/dckt/AGENTS.md`. It is the same interior the repository-level block carries today, selected by `agent_harnesses`, Codex clause included.
- They write no AGENTS.md or CLAUDE.md dispatch block and no `.cursor/rules/docket-dispatch.mdc` in the worktree.
- The learnings promotion destination is this file, and the learnings reference says so.
- Blocks committed before the repository went private are #533's `--remove-shared-files` concern.

### `docket instructions`

A new read-only catalog operation.

- **In a private repository:** prints the private instructions file from any worktree of the clone. It resolves through the git common directory, because a feature worktree's `.git` is a file.
- **Anywhere else:** prints **nothing** and exits 0. That covers shared repositories, non-docket repositories, and running outside git. It never fails noisily, because it runs in every session.
- **`--hook`:** wraps the same content in Claude Code's `SessionStart` hook output shape (additional context), and prints nothing outside private repositories.

### User-level triggers

`docket install` writes these when it installs the corresponding harness. They are installed even on machines with no private repository yet, and are no-ops everywhere else.

| Harness | Surface | What it does |
|---|---|---|
| Claude Code | `SessionStart` hook entry in `~/.claude/settings.json` running `dckt instructions --hook` | loads the rules automatically at session start; adds nothing outside private repositories |
| Codex | managed pointer block in `~/.codex/AGENTS.md` | tells the model to run `dckt instructions` once per session and follow its output |
| OpenCode | managed pointer block in `~/.config/opencode/AGENTS.md` | same as Codex |
| Cursor | none at user level | per-repository excluded rule `.cursor/rules/dckt-dispatch.mdc` |

The pointer wording is final in the plan, but it must stay rule-free and free of the word "docket":

```markdown
<!-- dckt:private-instructions:start (managed — do not hand-edit) -->
## Private repository instructions

At the start of a session inside a git repository, run `dckt instructions` once.
If it prints anything, treat that output as this repository's own AGENTS.md and
follow it for the rest of the session. If it prints nothing, ignore this section.
<!-- dckt:private-instructions:end -->
```

**Safety requirements:**

- **The hook entry** is identified by its exact command string. Install adds it only when absent and merges without reformatting or dropping any other setting. Uninstall removes only an exact match, and reports, rather than touches, a modified entry.
- **Pointer blocks** use the managed-block machinery, extended to accept the `dckt` marker prefix with the same guarantees: closed-block guard, idempotence, outside bytes preserved, and ownership-proved retirement.
- **This is a deliberate, narrow return of user-level parent-facing writes**, which 0351 retired. It is acceptable because the surfaces carry only a trigger, never rules.
- **Without the alias** (#534 reports a foreign `dckt`), the triggers fail harmlessly, and `docket instructions` still works by hand.

### First plan task (spike)

In a fresh session per harness, confirm that:

- Claude Code's user-level `SessionStart` hook delivers the output as context;
- Codex and OpenCode follow the pointer and run the command;
- Cursor reads an excluded rule file.

Record the results in the results file before building on them. If a harness fails, stop and report rather than inventing another mechanism.

### Documentation

`docket instructions` command help, and the skills (convention: private promotion destination; agent-layer reference: private delivery). No `docs/` pages.

## Acceptance criteria

1. **The private file.** In a private repository, `<git-common-dir>/dckt/AGENTS.md` carries the dispatch block (Codex clause when selected). No AGENTS.md, CLAUDE.md, or docket-named Cursor rule is written in the worktree, and `.cursor/rules/dckt-dispatch.mdc` is excluded and ignored by git.
2. **`docket instructions` output:**
   - it prints the file from the primary worktree, a feature worktree, and the metadata worktree;
   - it prints nothing and exits 0 in a shared repository, a non-docket repository, and outside git;
   - `--hook` emits valid `SessionStart` output in private repositories only.
3. **Installing the triggers:**
   - install adds the hook entry and the two pointer blocks;
   - a second install is a no-op;
   - an unrelated `settings.json` key and hook survive byte-for-byte;
   - uninstall removes only docket's exact entries.
4. **No rule text, no "docket".** The triggers contain no dispatch-block sentence and no word "docket", pinned by a test that fails if either appears.
5. **Learnings promotion** in a private repository names the private file as its destination.
6. **Shared repositories are unchanged:** repository-level blocks, and no private file.
7. **Fresh-session acceptance per harness** (the spike) is recorded in the results file. A private repository's Claude session receives the dispatch rules without any repository file.

## ADRs expected

A private repository's parent-facing rules live in `<git-common-dir>/dckt/AGENTS.md`. They are reached through content-free user-level triggers: a Claude `SessionStart` hook, and pointer blocks for Codex and OpenCode. This narrowly revisits 0351's retirement of user-level parent-facing writes. Relates to ADR-0036 and ADR-0078.

## Out of scope

- Shared repositories.
- Writing rules and the leak check (#532).
- The `dckt` alias (#534).
- Moving instructions on a mode switch (#533).
