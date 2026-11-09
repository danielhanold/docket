<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0581 — Split the ticket integration design into buildable changes](../../changes/active/0581-split-the-ticket-integration-design-into-buildable-changes.md)**
<!-- docket:backlink:end -->

# Ticket integration: final design and change split

Change #581, groomed interactively on 2026-11-09.

## What this change delivers

This change ends in a results artifact, not code: a project plan that splits the ticket integration described below into docket changes, each roughly one PR, in build order.

This spec is the integration's final design. It replaces an earlier draft ("Docket external project management integration", 2026-11-07) that left five groups of questions open and had not been checked against the code. Every open question from that draft is answered here, and the facts it relied on were re-traced at main `89e2c58b0` and docket `57d34bed9` (see *What the code says today*).

### How the work flows

1. **docket-implement-next builds this change.** Its results-artifact ending reads this spec, re-traces the code it names, and writes the project plan as this change's results artifact. It writes no code and creates no changes. Completing the results artifact completes this change.
2. **The human creates the planned changes from the final results artifact.** After this change is done, the human reads the plan and creates each change it lists, using the stub text and field values the plan gives for it.
3. **The human grooms each planned change by hand**, building each one before grooming the changes that depend on it.

## Why

Docket should let an engineer share meaningful progress with people who work in a ticket system (Jira first, others possible), without those people having to learn docket or read an agent's working history. Two problems drive it:

1. **Requests and solutions get mixed.** A change can go straight from stub to spec. Exported as a story, a proposed implementation reads like a settled requirement. The engineer receiving it needs the problem, the outcome, the scope, the constraints, and the acceptance criteria, and must stay free to judge the technical approach.
2. **Agent output is a poor handoff.** Raw notes and long explanations hide what happened and what the next person has to do. A ticket should get short, deliberate updates, with the detail left in docket's artifacts.

Docket stays where the change and its artifacts are developed. The ticket gets a clear request and the updates the human chooses. The team keeps running its own ticket workflow.

## Vocabulary

- **ticket**: the item in the team's ticket system (a Jira issue, a Linear issue, a GitHub issue, an Azure DevOps work item). Chosen because it is the word people use in conversation. Not "issue" (in a repository it reads as a GitHub issue, and it collides with docket's retired `issue:` field), not "work item", not "story" (one Jira type of several).
- **provider**: which ticket system (`jira`, `linear`, …).
- **event**: a point in a change's life after which docket can update its ticket. There are five (decision 2).
- **update**: what gets posted: a short comment on the ticket plus the artifact that goes with it. For the change-created event, the update is the new ticket itself.

People say "update the ticket after the PR was opened", not event names. Event names appear only in config and docs, and the skills' trigger wording matches the conversational form.

## Decisions

### 1. How an update reaches a ticket

Exactly two ways:

1. **On request.** At any time after an event has happened, the human asks ("update the ticket after the PR was opened for #612"). The update is composed from the artifacts as they stand. If the event has not happened (a merge update for an unmerged change), the agent says so and posts nothing. A late update describes the event asked for but never misstates the current state: asked for the PR-opened update after the merge, it says the PR has since merged.
2. **Automatically, per event.** Each event has its own switch, off by default. When it is on, the workflow running at the event posts the update, attended or in a loop.

**Off means nothing happens**: no prompt, no question, no line in any report. There is no "ask before publishing" mode, no parent-session handoff, no queue, no pending-publication record, and no ledger. The artifacts are durable, so an on-request update can always be composed later. Loops follow the same rules, with no loop-specific machinery.

An update needs a linked ticket. A change with no ticket never gets an update, automatic or requested; on request, the agent says the change has no ticket.

### 2. Events

| Event | Fires when | Workflow running there | The update says | Goes with it |
|---|---|---|---|---|
| change created | `change.create` lands a change, a stub or a fleshed-out one | docket-new-change (always attended) | the ticket itself: the title plus Why, What changes, Acceptance criteria, Out of scope, Open questions | nothing |
| spec written | a change's first spec lands (`change.groom` outcome `spec`) | docket-new-change, docket-groom-next (attended); docket-auto-groom (unattended) | the approach and the key decisions, with the spec's actual status taken from facts docket holds (designed, not yet built; being built; built, PR open). Never a claim of review, approval, or who wrote it: specs carry no structured status, and docket cannot know whether the team reviewed one | the spec |
| PR opened | docket-implement-next's PR handoff (`pr.publish`, `change.mark-implemented`) | docket-implement-next (unattended) | the PR link, what was built, what the reviewer needs to know | the results file, which is final before the PR opens |
| PR merged | docket-finalize-change merges and closes out (`finalize.merge`, `finalize.closeout`) | docket-finalize-change (attended when typed, unattended in a loop) | what merged, where it merged (the integration branch, or the parent branch for a stacked child, worded as "not yet in main"), key outcomes, anything left to do; sourced from the results file's Outcome and Known issues plus the change's `## Closeout notes`; never claims deployment or acceptance | the results file |
| results completed | a results-artifact change completes (`change.complete`) | the results-artifact ending of docket-implement-next (unattended) | findings, conclusions, limits, any action needed | the results file |

A human running `change.complete` by hand has no agent running, so no automatic update fires; they can ask for one.

**Updates only on request, never automatic:** a material revision (including a re-groomed spec, `change.groom` outcome `revise`), a blocker or a decision that is needed, and a killed change. Most of docket's own stops (run halts, finalize blocks) are machinery a ticket reader should not see; whether a revision is "material" is a judgment only the human makes; and a kill needs a human explanation, while implement-next's reconcile can kill a change on its own.

**No update, ever, on its own:** the maintenance sweep closing out a change merged by other means (`maintenance.sweep`, `maintenance.preflight`) posts nothing; the human can still ask for the merge update later. When a stack root lands, only the root's ticket is updated; the descendants' tickets get nothing (no fan-out). A stacked child's PR-merged update belongs to its own merge into its parent branch (see the stacked-merged trace item under *How the run works*).

**Plans never go out.** They are the agent's working document.

### 3. Who posts, and what happens on failure

The agent running the workflow at the event posts, using the harness's own connector tools (the MCP servers or plugins the harness owner installed and authenticated). Docket installs no connector, stores no credential, and contains no provider API code.

- Connector missing, upload or post failed, or outcome uncertain: the run's report gets one line naming what was not posted and why. The engineering outcome stands; nothing stops, retries, or is recorded. An uncertain outcome is reported as uncertain, and the human decides whether to ask again.
- An update is never reported as posted without the connector's confirmation.
- Duplicate comments are acceptable and are deleted by hand. There is no exactly-once guarantee.
- Attachments go up first and the comment goes last. If a required attachment cannot be uploaded, no comment is posted and the report says why. A local path is never substituted for an attachment.

### 4. Establishing the link: link, import, create

All three are attended: the human asks, or, for an automatic ticket at change creation, docket-new-change is running with the human in the session.

- **Link** an existing change to an existing ticket by recording the ticket fields (decision 5). Works in any status. Replacing an existing link and unlinking are explicit requests.
- **Import** ("make a change from CLOUD-123"): read the ticket through the connector and distill its contents into a new proposed change (Why, What changes, Acceptance criteria, Out of scope, Open questions), with the ticket fields set at creation. One time only: there is no ongoing inbound sync. An imported change never fires change-created, since it is already linked.
- **Create** a ticket from a change:
  1. **Readiness check.** Could another engineer take ownership of this as a user story? The check covers the problem and why it matters; the desired outcome; bounded scope and non-goals; genuine constraints, kept distinct from implementation preferences; clear, verifiable acceptance criteria that belong to this change; and no open question that blocks understanding. Technical questions may stay open: the request must be ready for ownership, not necessarily for coding. A spec is not required, and an existing spec never turns its technical choices into requirements.
  2. **Refinement, only if needed.** Ask targeted questions about what is missing and write the answers into the change through `change.groom`'s owned sections, which works only while the change is `proposed`. For a change past `proposed`, the ticket is built from the change as it stands; if that is not story-ready, the agent names what is missing and the human decides. The agent never invents scope or acceptance criteria and never promotes an unreviewed solution into a requirement.
  3. **Draft.** The ticket body is the change exported with export's `ticket` kind (decision 7), with private cleanup in a private repository.
  4. **Approval.** On request, the agent shows the drafted ticket and creates it on the human's go. Automatic (the change-created switch is on): there is no draft approval; if the change is not ready, the targeted questions are asked in the same docket-new-change session; if the human declines to refine, no ticket is created.
  5. **Record.** After the connector confirms the ticket was created, record the ticket fields on the change.

  Scan-mode stubs never create tickets automatically. Where a new ticket goes comes from config (`tickets.provider`, `site`, `project`, `type`; decision 8). On request with those values missing, the agent asks; automatically with them missing, no ticket is created and the session gets one line saying why.

  Creating a ticket writes its description once. Docket never rewrites a ticket's description afterwards; every later update is a comment.

### 5. Ticket fields on the change

Three flat scalar fields:

```yaml
ticket_provider: jira
ticket_id: CLOUD-123
ticket_url: https://acme.atlassian.net/browse/CLOUD-123
```

- **Flat**, because the frontmatter value model has no mapping kind; a nested object would need a new one.
- **Provider and id are stored, not derived.** They are worked out once, while linking with the human present; they stay correct if the repository's config changes later; and the id gives readable link text without any provider knowledge. When the URL's host matches `tickets.site`, the provider comes from `tickets.provider`; otherwise the agent asks while linking.
- `ticket_provider` is a free lowercase token. Docket keeps no list of providers.
- **One dedicated operation** writes the three fields together, is revision-pinned, works in any status, refuses a partial set, and clears all three on unlink. It follows `change.relink`'s pattern (one purpose, any status, `## Artifacts` re-rendered) rather than `change.groom`, which works only while a change is `proposed`. `change.create` accepts the three fields at creation, for import.
- A partial or inconsistent set, which only a hand edit can produce, is treated as "not linked" by the skills and reported as a finding by the read side: visibility only, never a block.
- `## Artifacts` gains a row: `| Ticket | [CLOUD-123](<ticket_url>) |`. The board does not change.
- The PR body gets one plain line linking the ticket, replacing implement-next's "Best-effort PR→issue reference". A ticket URL is not docket machinery, so the line is allowed in a private repository; like all PR text, it is still subject to the leak check.
- **The legacy `issue:` field is retired.** Nothing writes it, no operation accepts it, and the only four records that carry it (archived 0065, 0066, 0227, 0298) hold it empty. It is left over from the removed GitHub board mirror (ADR-0007; the removal was decided in the Go migration program map, `docs/superpowers/specs/2026-08-12-go-migration-program-map.md`: "GitHub Issues/Projects mirroring is removed"). It is removed from decode, the domain model, `newChangeBuilder`, repair's field list, and implement-next's Step 7. The change that retires it decides whether the four empty values stay as unknown compatibility data or are dropped.

### 6. Acceptance criteria

A new, optional change-body section, `## Acceptance criteria`, placed after What changes and before Out of scope (placement is the designer's call).

- Settable at creation (`change.create`) and editable through `change.groom`'s owned sections (outcomes `spec` and `revise`), like the other proposal sections.
- Export carries it: the ticket-kind section allowlist gains it, so a ticket body includes it.
- Optional and never a build gate. Changes that never become tickets need none.
- The convention's body-section list documents it.

Having reviewers check a build against its acceptance criteria is out of scope; it is a possible follow-up.

### 7. Export's `issue` kind becomes `ticket`

`docket open change <id> --export` writes `<date>-issue-<slug>.md` today, because the export kind's value is the filename token: `KindIssue Kind = "issue"` in `internal/export/export.go`, used by `FileName`. Rename the kind and its internal identifiers together, so that one term runs through docket and no name contradicts its value:

| Today | Becomes |
|---|---|
| `KindIssue Kind = "issue"` | `KindTicket Kind = "ticket"` |
| `issueSections`, the section allowlist (which also gains `Acceptance criteria`, decision 6) | `ticketSections` |
| `stepIssueSections`, `keepIssueSections` | `stepTicketSections`, `keepTicketSections` |
| tests: `issueH2` in `corpus_test.go`, the expected `issue-x.md` in `filename_test.go` | their `ticket` equivalents |

Plus the `"change"` → kind mapping in `internal/app/open_export.go`. The exported file becomes `<date>-ticket-<slug>.md`.

- The rename is internal apart from that filename. No JSON field carries the kind (`open`'s `target_kind` is `file`, a different thing), and no skill or guide names the `-issue-` filename.
- No compatibility alias is kept.
- The `"issue"` key in `internal/reposetup/repair.go` is the legacy frontmatter field, which is retired under decision 5. It is not part of this rename.

### 8. Config

```yaml
tickets:
  provider: jira              # which tool
  site: acme.atlassian.net    # where it lives
  project: CLOUD              # where new tickets go
  type: Story                 # kind of ticket, optional
  auto:                       # act on its own after each event; all false by default
    change_created: false     # create the ticket (readiness check first)
    spec_written: false
    pr_opened: false
    pr_merged: false
    results_completed: false  # a results-only change finished
```

- **Every `tickets.*` key is per-repo only**, applying ADR-0019's existing classification: a key whose effect writes external objects is fenced. Set in the global config or `.docket.local.yml`, it is warned and ignored. In a shared repository it lives in the committed `.docket.yml`, so it is the team's setting; in a private repository it lives in `.git/dckt/config.yml`. This applies an existing rule, so it needs no new ADR.
- **No master on/off switch.** With every event switch off by default and every update needing a linked ticket, a fresh repository posts nothing; a master switch would only block the human's explicit requests.
- **No "enable all" switch.** Each event is set on its own, as docket's config works everywhere else.
- **`project` and `type` are generic**, and `docket-link-ticket` maps them to the provider's own terms. `type` is optional.

| Tool | `project` means | `type` means |
|---|---|---|
| Jira | project | issue type (Story, Task, Bug) |
| Azure DevOps | project | work item type |
| GitLab | project | issue type |
| GitHub Issues | repository | issue type, where the organization has them |
| Shortcut | team | story type |
| Linear | team | none (labels) |
| Asana / Trello | project / board | none |

- Each key is a fixed path in the config registry, since the registry has no user-named map keys. The example config carries the table above as a comment.

### 9. Delivery by visibility

Delivery follows the repository's existing visibility (`visibility` in the `repository.prepare` context). There is no new flag, no override, and no probe of who can read what.

- **Shared:** the comment links the relevant artifact on GitHub, pinned to the commit that holds the version being discussed (the version at posting time). Readers are assumed to be able to open the repository; docket checks no permissions. Every link docket builds today is branch-based, so commit-pinned URLs are new, small code. Plan and results live only on the metadata branch (ADR-0141); the spec lives there too and is also copied onto the integration branch by `workspace.commit-spec`.
- **Private:** the relevant artifact is exported (`open … --export --print --json`) and cleaned, then attached to the ticket, and the comment names the attachment. Nothing posted, neither the comment nor an attachment, may contain docket machinery, under the private-repository writing rule: the words "docket" or "dckt", docket marker comments, `Docket-` trailers, `.docket` or `dckt` paths, change ids or change references, docket ADR numbers, or references to the backlog or board.
  - **On request (attended):** today's docket-export behavior holds. The human sees the before → after of any rewrite, and the post goes out on their go.
  - **Automatic (unattended):** the cleanup runs with no before → after review. Then a final check scans everything about to be posted (comment and attachments) with exactly the private-visibility leak check's existing patterns (ADR-0144, as built by change 532): the words `dckt` and `docket` (subject to `leak_check.match_word`), `docket:`/`dckt:` markers, `Docket-` trailers, `.docket`/`.git/dckt` paths, and zero-padded references to known change ids. On any hit nothing is posted, the agent says so in its report to whoever dispatched it, and nothing else stops. The leak matcher exists as pure functions but is not exposed for arbitrary text today; exposing it is part of this work.
  - **What the final check deliberately does not catch:** docket ADR numbers, unpadded change references (`#612`), and mentions of the backlog or board. Change 532 excluded these from the leak check on purpose: a host repository's own ADR numbers overlap docket's, unpadded references collide with years and PR numbers, and "backlog" and "board" are everyday words in a ticket system. In unattended posts they rely on the cleanup pass alone. An ADR number that slips through is acceptable: ADRs are common in agent-driven work and reveal nothing about docket.
- **Attachments in practice:** in the Claude Code harness the Jira connector attaches a file in two steps. `uploadAttachmentToJiraIssue` first returns an upload command that has to run from the shell, then attaches the file id that command returns. The skills must not assume a one-call upload, and an agent that cannot run that step is on the attachment-failed path (decision 3). The same connector can also add a native web link to a ticket (`createJiraIssueRemoteIssueLink`); whether shared mode uses that alongside the comment link is the designer's call. Both operations are reachable only through the connector's `discover` tool, not as directly listed tools, so the skills must not assume a connector's operations appear in the tool list.
- **Visibility unknown, unattended:** when an automatic update cannot establish the repository's visibility or docket's ids (no `repository.prepare` context), it treats the repository as private and, since it cannot ask, posts nothing and reports why. This matches docket-export's "treat the repository as private" posture.

### 10. Layering

**Two skills**, split along the two kinds of action:

| Skill | Does | Who is there |
|---|---|---|
| `docket-link-ticket` | link a ticket; create a ticket from a change (readiness check, refinement, draft); make a change from a ticket | always the human |
| `docket-update-ticket` | post an update, on request or after an event whose switch is on | the human, or nobody |

- Both are inline skills, not agents. They are harness-neutral and post with the agent's own connector tools. Autonomous workflows only ever load `docket-update-ticket`, so the refinement and import instructions never take up space in an implement or finalize run.
- **Progressive disclosure, split by who is asking and by visibility, not by event.** Each SKILL.md stays short. `docket-update-ticket` keeps the per-event specifics (precondition, sources, content) in one table and loads exactly one delivery reference: shared (pinned links) or private (attachments, the final check, attended versus unattended review). `docket-link-ticket` loads its create reference (readiness, refinement, draft) only when creating and its import reference only when importing. There are no per-event files: the events differ by a few lines each, and five copies of the shared rules would drift.
- **`docket-export` keeps its job**: copying one artifact outward. The ticket skills use it for the ticket body and for private attachments instead of reimplementing cleanup. Its private before → after stop must be skipped for automatic updates (decision 9); whether that is a direction from the calling skill or a new mode in docket-export is the plan's to recommend.
- **Workflow wiring.** At each event, the running workflow reads that event's switch and, when it is on, invokes `docket-update-ticket` (or `docket-link-ticket` for change created) with a call-site direction stating that nobody is there to ask (ADR-0044's `DIRECTED to:` pattern, phrased by shape). The sites: docket-new-change (change created, spec written), docket-groom-next (spec written), docket-auto-groom (spec written), docket-implement-next's PR handoff (PR opened) and its results-artifact ending (results completed), and docket-finalize-change after closeout (PR merged). For change created, docket-new-change is attended, so refinement questions are allowed even though the draft approval is skipped; whether the readiness check runs before or after `change.create` is that change's own design call. A failure is one report line, never a halt and never a gate: no new blocking gates.

**The binary does the parts that need no judgment** (ADR-0012's script/model boundary):

- the ticket-field operation (link, replace, unlink) and `change.create`'s ticket keys;
- the `tickets.*` config keys;
- one read operation that, for a change and an event, returns in a single call: the ticket fields (or "not linked" / "partial"), the visibility, the event's switch, whether the event has happened plus the facts the update needs (such as the actual merge destination), the relevant artifact paths, and in shared mode their commit-pinned GitHub URLs;
- the leak matcher exposed for outbound text;
- the `## Acceptance criteria` section in create, groom, and export, and the export kind rename;
- the `## Artifacts` ticket row and the retirement of `issue:`.

**The agent does the parts that need judgment:** the readiness check, the refinement questions, composing the update under the editorial contract, mapping config to the connector's terms, and posting.

### 11. Editorial contract

Updates are written for humans: a few short sentences or bullets, scaled to how much the update matters. Include, when relevant:

- **What happened:** the event or the meaningful change.
- **What matters:** key outcomes, consequential decisions, limitations.
- **What is needed next:** an action, test, review, or decision for the reader.
- **Supporting material:** the PR, and the artifact as a link or attachment.

Omit agent narration and descriptions of how the agent worked, brainstorming notes and work logs, file lists, test inventories, pasted command output, repeated background, generic filler, empty headings, and pasted artifact contents. Concision must keep material blockers, risks, and required actions. The same standard applies on request and automatically. Say only what the change and its evidence support: a merge is not a deployment or an acceptance.

Examples, for tone and length only:

> Proposed approach: validate requests before processing, so invalid ones fail before any work starts. The existing API contract is unchanged. Drafted, not yet reviewed; details in the linked spec.

> Merged into main. Invalid requests now return a clear error before processing starts. Automated checks passed; one manual check remains, described in the results. Deployment has not been confirmed.
> Results: https://github.com/acme/api/blob/4f2c9e1/docs/results/2026-11-08-validate-requests-results.md

### 12. Harness responsibility

The harness owner installs and configures connectors, credentials, and permissions. Docket defines what to post and when; the harness provides access to the ticket system. The design is generic: Jira is the first provider exercised, and there is no provider framework and no build-out for several providers now. The supported harnesses (Claude Code, Cursor, Codex, OpenCode) differ in connector availability, especially inside dispatched agents; a missing connector is the failure path in decision 3.

### 13. Testing

- **No ticket-system call is ever made from the docket repository.** No build, test, review, or results run of these changes posts to, creates, or reads a real ticket, and docket's own `.docket.yml` never carries `tickets.*` keys. The changes are verified by repository tests and fixtures only, with connector calls stubbed or described in skill text.
- **Live checks are human steps outside this repository.** Each change that adds a ticket capability a person can use lists an Important item in its results' `## Human actions and testing`: try the capability against a sandbox ticket project from another repository, typically a private scratch repository, where the `tickets.*` keys live in `.git/dckt/config.yml`. The human does it after the PR opens and before merging. It is a results step, not a stop during the build, so no change needs `hold: true`.
- No sandbox site or project name appears in this repository: it is public.

## Out of scope

- Syncing ticket status, assignee, sprint, or workflow; rewriting a ticket's description after creation; inbound sync of comments or acceptance feedback; deployment tracking; creating follow-up tickets automatically; treating a merge as acceptance or completion.
- A publication ledger, delivery-state tracking, a pending-publication queue, automatic retry, or exactly-once delivery.
- Confirmation prompts for automatic updates, and a parent-session publication handoff.
- A master switch, an "enable all" switch, and ticket keys settable globally or machine-locally.
- Automatic updates for revisions, blockers, kills, or maintenance-closed changes.
- Updates fanned out to stacked descendants' tickets.
- Provider credentials or connector setup inside docket.
- Reviewing builds against acceptance criteria (a possible follow-up).
- Any board change.
- Widening the leak check's patterns, for ticket posts or for pushes and PRs.

## The deliverable: the project plan

The results artifact follows the results template. It is what the human creates the planned changes from, so it must be complete enough to create each one without re-reading this spec. Its body contains:

1. **An ordered list of proposed changes**, each roughly one PR. For each: the title; the type; the decisions above that it covers, by number; `depends_on`, by position in the list; what it touches (binary operations, skills, config, docs); a rough size; and ready-to-use stub text (Why, What changes, Acceptance criteria, Out of scope) written at the change body's PM altitude, saying which design decisions this spec has already fixed and which are left to that change's own grooming.
2. **A coverage check**: a table mapping every decision (1–13), broken into its parts (for example decision 4's link, import, and create, or decision 9's shared and private delivery), and every out-of-scope item to the change that implements or guards it. Every part lands in exactly one change, and no change adds anything this spec does not ask for.
3. **Build order**: the sequence, and which changes are independent of each other.
4. **ADRs**: which decisions warrant an ADR and which change records each one. Expected candidates: "off means nothing happens" with no ledger; the two-skill split; the ticket fields and the retirement of `issue:`; unattended private posting with no review, behind the leak-pattern check.
5. **Trace notes**: anything the run found in the code that contradicts or refines this spec, with its evidence.

**Splitting rules:**

- Each change is one PR that leaves docket working and, where possible, adds one capability a person can use once it merges (for example, "link a ticket", then "ask for an update in a shared repository"), rather than layers that do nothing until the last one lands.
- Order the changes for just-in-time grooming. The human grooms each stub by hand, after everything it depends on has been built and merged. Foundations come first, and no stub may need a later change's design in order to be groomed.
- The human creates each planned change from the final results artifact as a plain proposed stub: no spec, `auto_groomable` unset, `depends_on` set, and both `related` to and `discovered_from` this change. The plan gives each stub's title, type, and field values so it can be created in one step.
- Keep the count small enough to groom one at a time; merge slices that cannot be used or tested on their own.

**How the run works:**

- **Re-trace first.** The facts in *What the code says today* were true at main `89e2c58b0` and docket `57d34bed9`. Confirm each against the current tree and record any drift in the trace notes.
- **Resolve by tracing:**
  - (a) Which path produces `stacked-merged`. `skills/docket-convention/references/stacked-changes.md` says the merge sweep is "the only producer of the state", while docket-finalize-change lists `stacked-merged` as a `finalize.closeout` disposition. If only the sweep produces it, a stacked child's PR-merged update can only ever be on request under the "maintenance never posts" rule, and the plan says so.
  - (b) Whether docket-export's private before → after stop is skipped through a direction from the caller or a new docket-export mode (decision 10); recommend one.
  - (c) The read operation's shape, and whether `diagnostic.config` and `status --json` already cover any part of it.
- **No code, no commits, no change creation, and no ticket-system calls** (decision 13). The run never creates the planned changes; the human does, from the final results artifact. If the split turns out to need a code change, or a file the results artifact cannot carry, halt as the results-artifact ending describes.

## What the code says today

Verified at main `89e2c58b0` and docket `57d34bed9`; anchors are symbols, not line numbers.

- **Frontmatter values** are scalars and scalar sequences only (`internal/document/value.go`: `kindNull`, `kindString`, `kindInt`, `kindBool`, `kindSeq`). The patcher's `classifyShape` treats a mapping as `ShapeUnsupported`. The writer quotes every string and reparses its own output (ADR-0071).
- **Adding a change field** touches `internal/repository/decode.go` (`changeWire`, `decodeChange`), `internal/domain/entities.go` (a `ChangeSpec` field and accessor), `internal/domain/actions.go` `newChangeBuilder` (without it, lifecycle actions drop the field), `internal/render/record.go` `ChangeRecord` (field names and order), `internal/reposetup/repair.go`'s field lists, the convention's field sketch and its embedded twin, and `internal/render/artifacts.go` `ArtifactBlockContent` for a new row.
- **`issue:`** is read by decode, the domain model, `newChangeBuilder`, and repair's `changeStringFields`; written by nothing; accepted by no request schema; and used only by docket-implement-next's "Best-effort PR→issue reference". Four archived records carry it, all empty.
- **Frontmatter edits on an existing change:** `change.groom` works only while the change is `proposed`, and its request carries title, priority, type, deliverable, branch_prefix, hold, hold_reason, the relation lists, `spec_markdown`, and `sections`. `change.relink` writes exactly one field (`branch` or `pr`) plus `updated`, revision-pinned, in any status.
- **Config:** one registry (`internal/config/schema.go` `buildRegistry`), with no user-named map keys. `scopeRepoOnly` fences a key (ADR-0019). Private repositories read `.git/dckt/config.yml` as the repository layer and never read `.docket.yml`. `docket diagnostic config --json` returns `effective.<path>.value`. A new key also needs its typed field, its default, its assignment in `assemble`, a `diagnostic.config` inspection line, and a `.docket.example.yml` entry (checked by `example_correspondence_test.go`).
- **Export:** `issueSections` keeps exactly Why, What changes, Out of scope, and Open questions; every other H2, `## Closeout notes` included, is dropped. `FileName` writes `<date>-<kind>-<name>.md`. The `docket-export` skill posts with the agent's own tools, and in a private repository it stops for a before → after whenever it rewrote anything and on any word-check hit.
- **Links:** `BlobURLOnBranch` and `BlobURL` in `internal/render/link.go` build branch URLs only, and return empty for a private metadata branch. No builder for a commit-pinned URL exists.
- **Leak check:** `internal/leakscan` exports `Line`, `Text`, and `Scan`, pure and stdlib-only. Its callers are `workspace.publish`, `pr.publish`, and `finalize.publish`. No command exposes it for arbitrary text.
- **Workflows:** docket-implement-next, docket-auto-groom, docket-finalize-change, and docket-status are autonomous wrappers carrying an abort-and-report rule. docket-finalize-change launches as `root-coordinator` (unforked) and runs inline when typed. The run tracker brackets docket-implement-next only.
- **Merge-summary sources:** the results file is final before the PR opens (the template says "Name no PR number"); `## Closeout notes` (`verification_outcomes`, `late_findings`) is written by `finalize.closeout` after the merge.

## Related findings outside this change

Found while tracing; not part of this work.

- The convention's agent-layer paragraph still says "docket-finalize-change's human-present close-out is the one exception inside an autonomous file", but `internal/repoguard/skill_handoff_sites_test.go` asserts that zero human-present exceptions remain.
- `stacked-changes.md` and docket-finalize-change disagree on what produces `stacked-merged` (trace item (a) above).
