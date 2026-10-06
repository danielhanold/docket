<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0532 — Implement private visibility for PRs, commits, and shipped files](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0532-private-visibility-keep-docket-out-of-prs-commits-and-shippe.md)**
<!-- docket:backlink:end -->

# Implement private visibility for PRs, commits, and shipped files: design

Change #532, groomed interactively on 2026-10-05. Third in the private-visibility series (#529–#533). Depends on #530, which moves build artifacts off the feature branch, and #531, which adds private mode itself.

## Summary

In a private repository, other people see what docket ships: feature branches, PRs, commits, the spec copy, and code. None of them use docket. After #530 and #531, the remaining traces are PR-description blocks and lines, finalize's PR comments, model habits (change ids in commit subjects and PR titles, docket vocabulary in shipped text), and repository-level instruction files.

This change removes those traces and adds a **blocking** leak check as the backstop, in private repositories only. The goal is that nothing reaching `origin` through a private repository's feature branch or PR contains a docket or `dckt` fingerprint.

It also gives private repositories their own **instructions file**, `<git-common-dir>/dckt/AGENTS.md`. That file holds the dispatch and run-tracker rules and any promoted lessons: the role the repository's own AGENTS.md plays in shared repositories. It is delivered to each harness without any file in the repository.

## Evidence gathered at grooming

- **PR body.**
  - `pr.publish` (`assemblePRBody`, `internal/app/pr_publish.go`) always inserts the backlink block rendered by `render.BacklinkContent`. #530 removes the evidence block.
  - `skills/docket-implement-next/references/edge-paths.md` adds a `↩ Change <padded-id> — <title>` line and a best-effort `#<issue>` reference.
  - The coordinator writes PR titles freely; an observed habit is a `(0507)` suffix.
- **PR comments.** `finalize.block` posts a comment carrying the docket attempt marker (`finalizeBlockedCommentMarker` in `internal/app/finalize_block.go`, through `githubcli.Client.EnsureComment`).
- **Commit text.**
  - `skills/docket-build-task/SKILL.md` ("The commit") sets no message format.
  - The fix-pass reference (`skills/docket-implement-next/references/fix-pass.md`) produces fix commits.
  - Observed `docs(plan): change 0507 …` subjects are inherited habit, not a template, and change ids leak into commit subjects through plan text.
- **Spec copy.** #530 ships the spec as the feature branch's first commit. Specs habitually carry `Change #N, groomed …`, `.docket/` paths, ADR numbers, and backlog references.
- **Repository-level instruction files.** The `agent_harnesses` repository phase (`reposeed.Plan`, `installAuthorizedSurfaces` in `internal/app/repository_init.go`, `internal/app/repophase.go`) writes the AGENTS.md and CLAUDE.md dispatch blocks and `.cursor/rules/docket-dispatch.mdc`.
- **The dispatch rules exist only in the repository's own instructions file.** Since changes 0334 and 0351, the dispatch block (dispatch the named agent, bracket each implement-next run with `run.start` / `run.verdict`) is written only to a repository's own CLAUDE.md or AGENTS.md. `docket install` writes no user-level copy and retires old ones (`GlobalDispatchTarget` in `internal/harness/claude/claude.go`; the same pattern holds for codex and opencode). 0334 removed the user-level copy because a **second copy of the rules drifted**.
  - At grooming, `~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md`, and `~/.config/opencode/AGENTS.md` contained no docket text.
  - Simply suppressing the repository-level block in a private repository would therefore leave the parent agent with **no** dispatch or run-tracker rules.
- **Learnings promotion** lands a graduated rule in the integration-branch AGENTS.md or CLAUDE.md by hand (`skills/docket-convention/references/learnings.md`, *Promotion*). A restricted repository does not allow that edit.
- **Feature-branch pushes** go through `workspace.Service.PublishHead` (`internal/workspace/publish.go`). PR create and edit go through `pr.publish`. Finalize's force-push after a rebase goes through `finalize.publish`.

## Decisions (settled with the human)

1. In private repositories PR descriptions are plain authored prose: no backlink block, no evidence block, no change line, no `#issue` reference.
2. A finalize block in a private repository is recorded on the change record (`## Finalize blocked`) only. No PR comment is posted.
3. Writing rules apply to everything that ships: no docket or `dckt` vocabulary and no change ids.
4. The leak check **blocks**, in private repositories only. This is a deliberate exception to docket's report-only default for new checks: a pushed leak is irreversible and outward-facing, and the check cannot affect shared repositories.
5. The check also matches `dckt`. Matching the bare word "docket" can be switched off for codebases where it is an ordinary domain word (court or shipping dockets).
6. A `docket` or `dckt` branch appearing on `origin` in a private repository is reported, never blocked.
7. Sparse documentation: `.docket.example.yml`, command help, and skills only.
8. **A private repository's parent-facing rules live in `<git-common-dir>/dckt/AGENTS.md`**: the dispatch block plus promoted lessons. They are loaded per harness as follows:
   - **Claude Code** runs `dckt instructions --hook` automatically at session start, through a user-level `SessionStart` hook in `~/.claude/settings.json`. The rules are always loaded.
   - **Codex and OpenCode** get a short, static pointer block in their user-level AGENTS.md telling the model to run `dckt instructions` once per session and follow its output. The human chose this hybrid over a pointer for every harness: the hook is mechanical where a harness offers one, and the pointer is the best available elsewhere.
   - Neither user-level surface carries any rule text, so the drift that made 0334 retire the user-level copy cannot recur. The rules have one source.
   - **Both surfaces are spelled neutrally.** The pointer block's managed markers use the `dckt:` prefix, and both surfaces invoke `dckt`, a new alias for the `docket` binary. So nothing in them reads "docket", even in the user's home directory or a published dotfiles repository.
9. **In private repositories, promoted lessons land in that private instructions file,** not the integration-branch AGENTS.md.

## Design

### 1. PR text and comments

- `pr.publish` in a private repository submits the authored body verbatim, inserting no docket-owned block. `finalize.publish` and `evidence.recertify` make no PR-body writes in private repositories; after #530 the evidence lives in the record anyway.
- The implement-next PR-authoring instructions omit the `↩ Change` line and the `#issue` reference in private repositories. The title carries no change id.
- `finalize.block` in a private repository records `## Finalize blocked` on the change and skips `EnsureComment`. Comment idempotency keyed on the marker is unaffected in shared repositories.
- Retargeting stacked children's PR bases carries no text, so it is unchanged.

### 2. Writing rules

One rule, stated once in docket-convention and referenced from each caller:

> In a private repository, nothing that ships through the feature branch or the PR may contain the words "docket" or "dckt", docket marker comments, `Docket-` trailers, `.docket`/`dckt` paths, change ids or change references ("change 0612", "#0612", "(0612)"), docket ADR numbers, or references to the backlog or board. The metadata branch's own files (plan, results, records) are exempt and must not be copied from.

The rule applies to:

- **Grooming in private repositories:** `docket-new-change`, `docket-groom-next`, `docket-auto-groom`. The spec ships, so the spec body follows the rule, and the `Change #N, groomed …` line is omitted. #530's spec-copy operation omits its `Change NNNN — title` line in private repositories.
- **Build-task and fix workers:** commit messages, code, comments, and test names.
- **The implement-next coordinator:** PR title and description.

The repository's mode reaches each worker through the payload or context it already receives. Shared repositories are unaffected.

### 3. The leak check

- **Where:** before any feature-branch push (`workspace.publish` / `PublishHead`, and `finalize.publish`'s post-rebase push) and before any PR create or edit (`pr.publish`). Private repositories only.
- **What it scans:**
  - every commit in `<effective base>..<head>`: subject, body, and trailers;
  - every added line in that range's diff, and added file paths;
  - the PR title and description.
  - The spec copy is an added file, so it is covered.
- **Patterns** (case-insensitive; the plan finalizes exact expressions with tests for each):
  - the word `dckt`;
  - the word `docket`, unless `leak_check.match_word: false`;
  - `docket:` and `dckt:` marker prefixes;
  - `Docket-` trailer keys;
  - `.docket` and `.git/dckt` path fragments;
  - change references to ids that exist in the backlog ("change 0612", zero-padded `#0612` and `(0612)`). The zero-padded form avoids matching years.
- **Outcome.** A typed refusal (new finding code `leak-detected`) lists each hit as {commit, file, line, matched text, rule}. Nothing is pushed and no PR is created or edited. The coordinator ends the run `halted` through `change.halt`, carrying the hit list. Nothing retries automatically. A human, or a later fix run, rewords the commit or line and re-runs.
- **Config.** `leak_check.match_word` (bool, default `true`) is an ordinary key in all layers, consulted only in private repositories and documented in `.docket.example.yml`.
- **Shared repositories never invoke the scanner.** A test pins that a shared-mode publish performs no scan.

### 4. Metadata on the shared remote

In a private repository, `repository check` and `repository prepare` probe `origin` for a `docket` or `dckt` branch and report a warning finding (`metadata-on-shared-remote`). The remedy says to delete the branch, or, once #533 lands, to use `set-visibility private --delete-shared-branch`. The finding is report-only.

### 5. Instructions for agents in private repositories

**No repository-level instruction file.** In private repositories, `init` and `install`'s repository phase write no AGENTS.md or CLAUDE.md dispatch block and no `.cursor/rules/docket-dispatch.mdc`. Blocks committed before the repository went private are #533's `--remove-shared-files` concern.

**The private instructions file.** In private repositories, `<git-common-dir>/dckt/AGENTS.md` takes the role the repository's own AGENTS.md plays in shared repositories.
- `init --private`, and `install`'s repository phase in a private repository, write the managed dispatch block into it. It is the same interior the repository-level block carries today, selected by `agent_harnesses` exactly as today, Codex clause included.
- The file also holds promoted lessons. In private repositories the learnings promotion destination is this file, and the learnings reference says so.
- It lives under `.git/`, so it is never committed and never visible in the worktree.

**`docket instructions`**, a new read-only operation in the catalog:
- In a private repository, it prints the private instructions file from any worktree of the clone, resolving `.git` through the git common directory, since a feature worktree's `.git` is a file.
- Anywhere else (a shared repository, a non-docket repository, outside git), it prints **nothing** and exits 0.
- It never fails noisily, because it runs in every session.
- `--hook` wraps the same content in Claude Code's `SessionStart` hook output shape (additional context), and prints nothing outside private repositories.

**Delivery per harness.** These are user-level surfaces written by `docket install`. They are static, carry no rule text, and are identical on every machine:

| Harness | User-level surface | What it does |
|---|---|---|
| Claude Code | `SessionStart` hook entry in `~/.claude/settings.json` running `dckt instructions --hook` | Loads the private rules automatically at session start; adds nothing outside private repositories. |
| Codex | managed pointer block (markers `dckt:`) in `~/.codex/AGENTS.md` | Tells the model to run `dckt instructions` once per session and treat its output as the repository's AGENTS.md. |
| OpenCode | managed pointer block in `~/.config/opencode/AGENTS.md` | Same as Codex. |
| Cursor | out of reach at user level | Cursor reads rules only from the repository: in private repositories the rule file is written as `.cursor/rules/dckt-dispatch.mdc` and excluded through `.git/info/exclude`. |

Pointer block wording, including its `dckt:` markers (final wording is set in the plan, but it must stay rule-free and must not contain the word "docket"):

```markdown
<!-- dckt:private-instructions:start (managed — do not hand-edit) -->
## Private repository instructions

At the start of a session inside a git repository, run `dckt instructions` once.
If it prints anything, treat that output as this repository's own AGENTS.md and
follow it for the rest of the session. If it prints nothing, ignore this section.
<!-- dckt:private-instructions:end -->
```

The managed-block machinery must accept the `dckt` marker prefix for these user-level blocks: install, idempotence, outside-bytes preservation, and ownership-proved removal, with the same guarantees as today's `docket:` blocks.

**The `dckt` alias.**
- `docket development install`, the Go installer that `install.sh` delegates to, creates `dckt` as a symlink to the installed `docket` binary in the same bin directory (`--bin-dir`, default `XDG_BIN_HOME` or `~/.local/bin`). This happens inside the same journaled install transaction. So `install.sh` installs the alias with no change of its own beyond the header comment listing what an install produces. The bootstrapper does not place binaries itself.
- A re-install is a no-op. `docket uninstall` removes `dckt` only when it is a symlink resolving to the installed `docket` binary.
- An existing `dckt` that docket does not own (another tool's binary or link) is **never** overwritten. Install reports it as a finding, and skips the user-level hook and pointer blocks, which depend on the alias. `docket instructions` still works by hand.
- Invoked as `dckt`, the binary behaves exactly as `docket`. The capability catalog keeps spelling `docket`, and skills keep resolving argv from the catalog, so only the user-level hook and pointer use the alias.

**Safety requirements for the user-level writes:**
- The `settings.json` hook entry is identified by its exact command string. Install adds it only when absent and merges without reformatting or dropping any other setting. Uninstall removes only an exact match, and refuses (reporting) on a modified entry.
- The pointer blocks use the existing managed-block machinery (closed-block guard, outside bytes preserved, ownership-proved retirement).
- These surfaces are installed whenever the harness is installed, because a private repository can appear on the machine at any time. They are no-ops everywhere else.
- This is a deliberate, narrow return of user-level parent-facing writes, which 0351 retired. It is acceptable because the surfaces carry only a trigger, never rules.

**First plan task (spike).** In a fresh session for each harness, confirm that:
- Claude Code's user-level `SessionStart` hook delivers the output as context;
- Codex and OpenCode follow the pointer and run the command;
- Cursor reads an excluded rule file.

Record the results in the results file before building on them.

### 6. Documentation

- `leak_check.match_word` is documented in `.docket.example.yml` and its twin.
- `docket instructions` is documented in its command help.
- The `dckt` alias appears in the install summary output and in `install.sh`'s header comment, not in `docs/`.
- Skills carry the writing rule, the leak-check halt handling, and the private promotion destination.
- No `docs/` pages.

## Acceptance criteria

1. **Plain PR.** A private-repository run produces a PR whose description is exactly the authored prose, whose title has no change id, and whose commits contain no fingerprint.
2. **Each seeded leak is refused before anything reaches `origin`:**
   - a commit subject with "(0612)";
   - an added line containing `.docket/`;
   - a spec copy containing "docket";
   - a PR title with "change 0612";
   - a commit containing `dckt`.

   The run halts with the hit list.
3. **`leak_check.match_word: false`** lets the bare word "docket" through, and still blocks markers, trailers, paths, `dckt`, and change ids.
4. **Year and ADR safety.** A year such as "(2026)" and a host repository's own ADR references are not matched.
5. **Shared mode unchanged.** Shared-mode PR bodies, finalize comments, and instruction files are unchanged, and no scan runs (pinned).
6. **Finalize block.** In a private repository it posts no PR comment and records `## Finalize blocked`.
7. **Shared-remote finding.** A `docket` or `dckt` branch on `origin` of a private repository produces the warning finding, with no refusal.
8. **Mutation.** Removing the scanner call before a push turns a test red.
9. **Private instructions file.** In a private repository, `<git-common-dir>/dckt/AGENTS.md` carries the dispatch block, and no AGENTS.md, CLAUDE.md, or docket-named Cursor rule is written in the worktree.
   - `docket instructions` prints the file from the primary worktree, a feature worktree, and the metadata worktree.
   - It prints nothing (and exits 0) in a shared repository, a non-docket repository, and outside git.
   - `--hook` emits valid `SessionStart` output in private repositories only.
10. **User-level surfaces.**
    - `install` adds the `settings.json` hook entry and the two pointer blocks. All three invoke `dckt`, and none contains the word "docket": the pointer blocks' markers are `dckt:` (pinned by a test).
    - `install.sh` (through the Go installer) creates the `dckt` symlink beside `docket`.
    - `dckt instructions` and `docket instructions` produce identical output.
    - `uninstall` removes the alias only with ownership proof.
    - A foreign pre-existing `dckt` is left untouched, reported, and the hook and pointers are skipped.
    - A second install is a no-op.
    - An unrelated `settings.json` key and hook survive byte-for-byte.
    - `uninstall` removes only docket's exact entries.
    - The surfaces contain no rule text, pinned by a test that fails if any dispatch-block sentence appears in them.
11. **Fresh-session acceptance per harness** (the spike) is recorded in the results file. A private repository's Claude session receives the dispatch rules without any repository file.

## ADRs expected

- **The leak check blocks.** It is a deliberate, scoped exception to report-only checks, justified because a pushed leak is irreversible and outward-facing.
- **Where a private repository's parent-facing rules live.** They are in `<git-common-dir>/dckt/AGENTS.md`, reached through content-free user-level triggers: a Claude `SessionStart` hook, and pointer blocks for Codex and OpenCode. This narrowly revisits 0351's retirement of user-level parent-facing writes: the triggers carry no rules, so the drift problem does not return. Relates to ADR-0036 and ADR-0078.

## Out of scope

- The metadata branch's content, which stays private and may use docket vocabulary freely.
- Rewriting already-pushed history.
- Shared-mode PR bodies and comments.
- Scanning for references to the host repository's own ADRs or tickets.
