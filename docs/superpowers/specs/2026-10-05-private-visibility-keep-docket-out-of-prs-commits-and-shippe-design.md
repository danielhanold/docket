<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0532 — Implement private visibility for PRs, commits, and shipped files](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0532-private-visibility-keep-docket-out-of-prs-commits-and-shippe.md)**
<!-- docket:backlink:end -->

# Implement private visibility for PRs, commits, and shipped files: design

Change #532, groomed interactively on 2026-10-05. Third in the private-visibility series (#529–#533). Depends on #530, which moves build artifacts off the feature branch, and #531, which adds private mode itself.

## Summary

In a private repository, other people see what docket ships: feature branches, PRs, commits, the spec copy, and code. None of them use docket. After #530 and #531, the remaining traces are PR-description blocks and lines, finalize's PR comments, model habits (change ids in commit subjects and PR titles, docket vocabulary in shipped text), and repository-level instruction files.

This change removes those traces and adds a **blocking** leak check as the backstop, in private repositories only. The goal is that nothing reaching `origin` through a private repository's feature branch or PR contains a docket or `dckt` fingerprint.

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
- **Feature-branch pushes** go through `workspace.Service.PublishHead` (`internal/workspace/publish.go`). PR create and edit go through `pr.publish`. Finalize's force-push after a rebase goes through `finalize.publish`.

## Decisions (settled with the human)

1. In private repositories PR descriptions are plain authored prose: no backlink block, no evidence block, no change line, no `#issue` reference.
2. A finalize block in a private repository is recorded on the change record (`## Finalize blocked`) only. No PR comment is posted.
3. Writing rules apply to everything that ships: no docket or `dckt` vocabulary and no change ids.
4. The leak check **blocks**, in private repositories only. This is a deliberate exception to docket's report-only default for new checks: a pushed leak is irreversible and outward-facing, and the check cannot affect shared repositories.
5. The check also matches `dckt`. Matching the bare word "docket" can be switched off for codebases where it is an ordinary domain word (court or shipping dockets).
6. A `docket` or `dckt` branch appearing on `origin` in a private repository is reported, never blocked.
7. Sparse documentation: `.docket.example.yml`, command help, and skills only.

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

### 5. Repository-level instruction files

In private repositories, `init` and `install`'s repository phase write no AGENTS.md or CLAUDE.md dispatch block and no `.cursor/rules/docket-dispatch.mdc`. They print one note saying that dispatch rules come from the user-level surfaces `docket install` already maintains. Blocks committed before the repository went private are #533's `--remove-shared-files` concern.

### 6. Documentation

`leak_check.match_word` is documented in `.docket.example.yml` and its twin. Skills carry the writing rule and the leak-check halt handling. No `docs/` pages.

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

## ADRs expected

The private-repository leak check blocks: a deliberate, scoped exception to report-only checks, justified because a pushed leak is irreversible and outward-facing. Relates to ADR-0036 and ADR-0078, the repository-level dispatch surfaces it suppresses in private repositories.

## Out of scope

- The metadata branch's content, which stays private and may use docket vocabulary freely.
- Rewriting already-pushed history.
- Shared-mode PR bodies and comments.
- Scanning for references to the host repository's own ADRs or tickets.
