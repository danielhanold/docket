<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0586 — Post a ticket update on request in a private repository](../../changes/archive/2026-11-10-0586-post-a-ticket-update-on-request-in-a-private-repository.md)**
<!-- docket:backlink:end -->

# Post a ticket update on request in a private repository: design

Change #586, groomed interactively on 2026-11-10.

## What this change delivers

`docket-update-ticket` (built by change #585) posts a short comment to a change's linked ticket when a person asks. In a private repository it currently refuses ("private repositories are not supported yet"), because the commit-pinned links it posts in a shared repository point at files nobody outside can open. This change replaces that refusal with private delivery: each artifact the update names is exported and cleaned through `docket-export`, attached to the ticket, and named in the comment. Attachments go up first; the comment goes last. A failed upload posts no comment.

This change is attended only: a person asked for the update and is in the session. Automatic posting in a private repository, with no person to review the cleanup, is change #588.

## Why

In a private repository the artifact cannot be linked: the metadata branch is local, and `context.ticket` returns an empty `url` for every artifact. The people reading the ticket still need the spec or results file, so it must travel as an attachment, and nothing that leaves (the comment or an attachment) may reveal docket.

## Settled decisions (carried from change 581; not reopened)

Restated so the build needs nothing from change 581.

1. **Delivery follows visibility.** The repository's existing `visibility` (from `context.ticket`, the same value `repository.prepare` reports) decides shared or private delivery. No new flag, no override, and no probe of who can read what.
2. **Private delivery attaches, it does not link.** The relevant artifact is exported (`open … --export --print --json`) and cleaned, then attached to the ticket, and the comment names the attachment.
3. **Nothing posted may contain docket machinery** (the private-repository writing rule): the words "docket" or "dckt", docket marker comments, `Docket-` trailers, `.docket` or `dckt` paths, change ids or change references, docket ADR numbers, or references to the backlog or board. This covers the comment and every attachment.
4. **Attended review is docket-export's.** The person sees the before → after of any rewrite the cleanup made, and the post goes out on their go.
5. **Attachments first, comment last.** If a required attachment cannot be uploaded, no comment is posted and the report says why. A local path is never substituted for an attachment.
6. **Never assume a one-call upload.** In the Claude Code harness the Atlassian connector attaches a file in two steps: `uploadAttachmentToJiraIssue` first returns an upload command that has to run from the shell, then attaches the file id that command returns. An agent that cannot run that step is on the upload-failed path. Connector operations may be reachable only through the connector's discovery tool (the Atlassian connector's `discover`), never assumed to be in the tool list.
7. **Failure is one report line.** Connector missing, upload or post failed, or outcome uncertain: the report names what was not posted and why; nothing stops, retries, or is recorded. An uncertain outcome is reported as uncertain. Nothing is reported as posted or attached without the connector's confirmation. Duplicates (comments or attachments) are acceptable and deleted by hand.
8. **Plans never go out**, as in shared delivery.

Everything else in `docket-update-ticket` is unchanged: the read, the refusals (other than the private one), the per-update content table, the late-update rule, the editorial standard, the layout guidance, mentions, and posting one connector call at a time.

## Decisions made in this grooming

### 1. Which artifacts are attached: the existing Links column, unchanged

`docket-update-ticket`'s update table already names, per update, what goes with it. Private delivery reuses that column as-is; there is no second table.

| Update | Shared | Private |
|---|---|---|
| spec written | spec link | spec attached |
| PR opened | PR, spec, results links | PR link stays a link; spec and results attached |
| PR merged | results link | results attached |
| results completed | results link | results attached |
| revision, blocker, or kill | links to the artifacts it discusses | those artifacts attached |

- **The PR link stays a link.** It is the code PR on the hosting site, not docket machinery (change 581 allows a ticket URL in a private repository's PR body by the same reasoning). `facts.pr_url` is used as in shared delivery.
- **The PR-opened update attaches the spec even when an earlier spec-written update already did.** In most teams the PR is the first time the spec reaches the ticket (spec reviews are rarer), and the reviewer gets everything in the one comment without hunting for an earlier attachment. A duplicate file on the ticket is acceptable.
- **A missing artifact is skipped**, as shared delivery skips a missing link: an artifact the update names but `artifacts` does not return (an older change whose file is not on the metadata branch) is left out, and the comment does not mention it.

### 2. Calling docket-export: as a file request

For each artifact to attach, `docket-update-ticket` invokes the `docket-export` skill as a **file request**, the same way `docket-link-ticket`'s create path does — `docket-export`'s rules are its own and are not restated: **DIRECTED to** export `<role> <id>` (`spec` or `results`) with `--output-path` set to a fresh directory from `mktemp -d "${TMPDIR:-/tmp}/docket-update-ticket.XXXXXX"`, return the exported path and the before → after list, and post nothing. `docket-update-ticket` then does the uploading and posting itself.

- **A failed export posts nothing** (`docket-export`'s own rule): no attachment, no comment; the report names the failed export.
- **A word-check hit inside the export stops** the way `docket-export` stops: show the person the path and the lines, and post nothing until they decide.
- Change #588 reuses this same file-request path for unattended posting, so attended and unattended share one export call.
- The export reads the metadata checkout as it stands, while the comment is composed from `metadata_commit`; both come from the same fresh sync moments apart, so they agree in practice. No pinning is added.

### 3. The comment's own text: written clean, checked, fixed quietly

`docket-export` cleans files only; the comment is written by the agent. So:

- The agent writes the comment under the private-repository writing rule: no change ids or change references (`#612`), no docket ADR numbers, no backlog or board references, no "docket" or "dckt".
- Attachments are named, labeled like links: `Spec: attached <filename>`, `Results: attached <filename>`.
- Before posting, the agent searches the comment and every attachment's file name for "docket" and "dckt" as whole words, case-insensitive (the bare word "docket" is exempt when `leak_check.match_word` is `false`, read as `docket-export` reads it). On a hit it fixes its own text — rewording the comment, or renaming the exported file in its temp directory — **quietly**: there is no before → after for the agent's own words.

### 4. When the person is asked

- **The export rewrote something:** stop once, show the before → after list for each attachment and the names of the files to be attached, and post on the person's go.
- **Nothing was rewritten:** post straight away, as shared delivery does and as `docket-export` does. When the person asks to see the update first, show it and post on their go (unchanged).

### 5. Upload order and the partial-failure rule

1. Upload each attachment to `ticket.id`, one connector call at a time, handling the two-step upload where the connector needs it.
2. **Every attachment the update names, and whose file exists, is required.** Any failed or uncertain upload means **no comment**. The report is one plain line naming the ticket, which upload failed and why, which attachments already landed on the ticket (the agent never deletes from a ticket), and the exported files' paths so the person can attach them by hand. For example: `Not posted: the results upload to CLOUD-123 failed (connector error). The spec is already attached there; delete it, or ask again. Exported files: <paths>.` Asking again uploads the already-landed file a second time; that duplicate is acceptable.
3. After every upload is confirmed, post the comment last. Report it as posted only with the connector's confirmation, naming the ticket, the attachments, and anyone mentioned.

Never print a credential: when the two-step upload's shell command carries a token, it is run, not echoed into the report.

## What gets built

- **`skills/docket-update-ticket/SKILL.md`:**
  - Section 2: remove the refusal "`visibility` is `private`: say private repositories are not supported yet".
  - Section 4 (Links): add the private route beside the shared one — when `visibility` is `private`, **read `references/private.md` now (blocking)**.
  - The convention paragraph's "This skill makes no docket write. Its reads are the `context.ticket` operation and, for a request naming only a ticket, the `status` operation" stops being complete: private delivery also invokes `docket-export`, which writes a temp file and may read `leak_check.match_word` through `diagnostic.config`. Reword it to cover that (still no docket metadata write), and update its pinned copy in the `change_0585_update_ticket_skill` sentinel with it. The exact wording is the builder's call.
  - Reword the table's "Links" column header (or a one-line note) so the column reads as "what goes with the update", linked in a shared repository and attached in a private one.
- **New `skills/docket-update-ticket/references/private.md`**, the sibling of `references/shared.md`, carrying decisions 1–5 above: which files attach and the PR-link exception, the docket-export file-request call (with its DIRECTED direction and temp directory), the comment's writing rule and quiet self-fix, the single review stop on a rewrite, the upload order, the two-step upload and discovery-tool note, the partial-failure rule and report line, and the no-credential rule. Like `shared.md`, it says the rules in `SKILL.md` apply and are not repeated.
- **Docs:** `docs/reference/skills-and-agents.md` drops "shared repositories only for now"; `docs/reference/glossary.md`'s `docket-update-ticket` entry replaces "In a private repository it posts nothing yet" with one sentence on attachments.
- **Guards** (`internal/repoguard/prose_contracts_test.go`): change #585's sentinel pins the "not supported yet" string as present; move it to the change's `absent` list and add a `change_0586_update_ticket_private` sentinel pinning `private.md`'s load-bearing sentences (attachments first, comment last; any failed upload posts no comment; never substitute a local path; never assume a one-call upload; the file-request call; the quiet self-fix). Extend the existing check that `references/shared.md` can be read to `references/private.md` (`internal/repoguard/update_ticket_skill_test.go`). Each new guard is mutation-tested.
- **Budgets** (`internal/repoguard/budgets_test.go`): update the `docket-update-ticket/SKILL.md` line and add one for `references/private.md`, each with a change note.
- **Embedded tree:** regenerate with `go generate ./internal/assets/`.
- **No binary change.** `context.ticket` already returns `visibility` and an empty `url` per artifact in a private repository; `docket-export`'s file-request path already returns the cleaned file and its before → after list.

## Acceptance criteria

- In a private repository the skill no longer refuses; it loads `references/private.md`.
- The skill text covers: which artifacts attach (the shared Links column; the PR stays a link); the docket-export file-request call; the comment written under the writing rule with a quiet self-fix after the whole-word check over the comment and file names; one review stop only when the export rewrote something; attachments first, comment last; the two-step upload; any failed or uncertain upload posts no comment, with a report line naming what already landed and the exported paths; posted only on the connector's confirmation.
- Guards pin those sentences and fail when each is removed; the full suite passes.
- The results list an **Important** live check: from a private scratch repository (its `tickets.*` keys in `.git/dckt/config.yml`) linked to a sandbox Jira project, ask for a spec-written update and a PR-opened update; confirm the attachments appear before the comment, the comment names them, neither the comment nor the attachments contain docket machinery, and a forced upload failure (for example, a file the connector rejects) posts no comment and reports what already landed. No ticket-system call is made from the docket repository, and no sandbox site or project name appears in it.

## Out of scope

- Unattended private posting, skipping the before → after review, the final leak-pattern check over everything posted, and treating unknown visibility as private when nobody can be asked (change #588).
- Automatic posting after each event (change #587).
- Any change to `context.ticket`, `docket-export`, or the leak check's patterns.
- Deleting attachments from a ticket, retrying, or recording what was posted.
