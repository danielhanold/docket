<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0584 — Create a ticket from a change, or a change from a ticket](../../changes/archive/2026-11-10-0584-create-a-ticket-from-a-change-or-a-change-from-a-ticket.md)**
<!-- docket:backlink:end -->

# Create a ticket from a change, or a change from a ticket — design

Change #584, groomed interactively on 2026-11-09. Position 3 of the ticket-integration split planned by change 581. This spec restates every settled decision it carries, so the build needs nothing from 581's spec or results.

## Intent

An engineer should be able to hand docket work to a ticket system as a clear request, or pull a ticket into docket, without retyping it, while keeping the request (the problem, the outcome, the scope) separate from any proposed solution. The main case is **push**: a docket change goes out to Jira for someone else to pick up. Link and import are in scope but secondary.

## Vocabulary

- **ticket**: the item in the team's ticket system (a Jira issue, a Linear issue, a GitHub issue, an Azure DevOps work item). Never "issue" (reads as a GitHub issue, and collides with the retired `issue:` field), "work item", or "story".
- **provider**: which ticket system (`jira`, `linear`, …), a free lowercase token.
- **connector**: the harness's own tools for a ticket system (an MCP server or plugin the harness owner installed and authenticated). Docket installs no connector, stores no credential, and contains no provider API code.

## Goals

1. A new inline skill, `docket-link-ticket`, always attended, with three paths: **link** an existing ticket, **create** a ticket from a change (push), and **import** a ticket into a new proposed change.
2. `status --json` reports every linked change's ticket, active or archived, so the skill can tell whether a change already has a ticket and which changes a ticket is already on.
3. When the change has a linked spec, the created ticket carries it as the proposed approach.
4. `docket-export` stops claiming ticket work in its trigger description.

## Non-goals

- Posting updates (comments) to a ticket, on request or automatically (changes 585–588). An update request ("update CLOUD-123 now that the PR is open") is not this skill's.
- Automatic ticket creation when a change is created (change 587). This skill runs only when the person asks.
- Rewriting a ticket's description after creation, ever.
- Syncing ticket status, assignee, sprint, or workflow; inbound sync of comments; importing a ticket's comments or attachments.
- Any ticket-system call from the docket repository (see *Testing*).
- Any board change.

## Settled decisions (fixed by 581 and 583 — not reopened)

- **Always attended.** All three paths run only when a person asks, in their own session. No autonomous workflow ever creates a ticket.
- **Ticket fields** are three flat scalars on the change — `ticket_provider`, `ticket_id`, `ticket_url` — written only by the `change.link-ticket` operation (link, replace with `replace: true`, unlink with `unlink: true`; any status; refuses a partial set) and accepted by `change.create` at creation (all three or none). A partial or invalid set counts as not linked and is reported as the `ticket-link-invalid` warning. Replacing or unlinking happens only on the person's explicit plain-words request ("switch it to CLOUD-456"); the agent never replaces a link on its own initiative.
- **Record only after confirmation.** The ticket fields are written only after the connector confirms the ticket was created (or, for link, that the ticket exists). Nothing is ever reported as created or attached without the connector's confirmation.
- **Description written once.** Creating a ticket writes its description once; docket never rewrites it afterwards.
- **Config** (`tickets.*`, per-repo only, read through the `diagnostic.config` operation at `effective.tickets.<key>.value`): `provider` (which tool), `site` (where it lives, e.g. `acme.atlassian.net`), `project` (where new tickets go; generic — the skill maps it to the provider's own term: a Jira project, a Linear team, a GitHub repository). No ticket-type key. The `tickets.auto.*` switches are not this change's (587).
- **Readiness check (create).** Could another engineer take ownership of this as a user story? The check covers: the problem and why it matters; the desired outcome; bounded scope and non-goals; genuine constraints, kept distinct from implementation preferences; clear, verifiable acceptance criteria that belong to this change; and no open question that blocks understanding. Technical questions may stay open — the request must be ready for ownership, not necessarily for coding. A spec is not required, and an existing spec never turns its technical choices into requirements.
- **Refinement (create), only if needed.** Ask targeted questions about what is missing and write the answers into the change through `change.groom`'s owned sections (`outcome: revise`), which works only while the change is `proposed`. For a change past `proposed`, the ticket is built from the change as it stands: if it is not story-ready, the agent names what is missing and the person decides whether to go ahead. **The agent never invents scope or acceptance criteria and never promotes an unreviewed solution into a requirement.**
- **Ticket body.** The change exported with `docket-export`'s machinery and the `ticket` export kind (`docket open change <id> --export --print --json`): its H1 is the title, its body keeps exactly Why, What changes, Acceptance criteria, Out of scope, and Open questions. In a private repository it gets docket-export's private cleanup. When the target needs its own markup (Jira's), only formatting is converted, never words.
- **Ticket type.** No ticket type is configured. The agent reads the ticket types the project actually has through the connector (in Jira, `listJiraProjectIssueTypesMetadata`) and maps the change's `type`: `fix` → the bug type; `feat` → the story or feature type; anything else → the task type. With no obvious match, it asks. A tool without ticket types (Linear, Asana, Trello) gets none. The mapped type appears in the draft.
- **Import.** Read the ticket through the connector and distill it into a new proposed change (Why, What changes, Acceptance criteria, Out of scope, Open questions) with the ticket fields set at creation. The change's `type` is mapped from the ticket's type to one of the configured `change_types` (a Bug → `fix`, a Story → `feat`); with no obvious match, the agent asks. One time only: no ongoing inbound sync.
- **Connector operations may be hidden.** Some connectors expose operations only through a discovery tool (the Atlassian connector's `discover`), not as directly listed tools. The skill must never conclude an operation is missing because it is not in the tool list.
- **Failure posture.** Connector missing, a call failed, or an uncertain outcome: say so plainly; an uncertain outcome is reported as uncertain and the person decides whether to try again. No retry machinery, no ledger.

## Decisions made in grooming

### 1. The binary: `status --json` reports the ticket

`StatusRecord` (`internal/app/status_result.go`), the per-record inventory in `status --json`'s `records` array, gains an optional `ticket` object — `provider`, `id`, `url` — on rows of kind `change`, present only when the change is linked (all three fields present and valid, the shared domain rule 583 built). Same shape and presence rule as `context.implementation`'s `ContextTicket`: reuse that type or its builder; do not invent a second shape. The `status` operation's `schema` descriptor follows from the Go type.

Why `records`, not the `changes` rows (a builder-check finding, verified): `changes` lists only active changes that pass the type/priority filter (`activeChanges` in `internal/app/status.go`), while `records` lists every change, active and archived, independent of the filter, with the `path` and `revision` a write needs. The skill needs both cases: linking a `done` change, and seeing which changes — archived ones included — already carry a ticket. Putting archived changes into `changes` would change what every status reader sees; rejected.

Rejected in grooming: a `has_ticket` boolean. Push needs to name *which* ticket the change already has, and import needs to compare ticket ids across changes; a boolean serves neither, and a skill would fall back to reading raw frontmatter for an optional key (learning `frontmatter-anchored-read`).

### 2. Skill layout and loading

```
skills/docket-link-ticket/
  SKILL.md               # shared rules + routing + the link path
  references/create.md   # push: readiness, refinement, draft, spec, type, create, attach, record
  references/import.md   # import: read, distill, draft, create, open questions
```

- **Inline skill**: frontmatter is exactly `name` and `description` — no `context: fork`, no agent wrapper (a forked skill cannot reach the session's connector tools or ask the person). Same posture as `docket-export`.
- **Loads `docket-convention`** and runs its startup check (capability bootstrap, `repository.prepare`), like the other skills that write change records. Request bodies for `change.groom`, `change.create`, and `change.link-ticket` are built from each operation's `schema` descriptor. Record `path` + `revision` come from `status --json`; a `contended` refusal means re-sync, re-read, and retry, or stop if the change moved out from under the request.
- `SKILL.md` loads `references/create.md` only when pushing and `references/import.md` only when importing; the link path is short enough to live in `SKILL.md`.

### 3. Trigger wording

The description names all three asks in the person's words — link a change to a ticket ("link #12 to CLOUD-123"), create a ticket from a change ("create a Jira ticket from #12", "put #12 in Jira"), make a change from a ticket ("make a change from CLOUD-123") — and says posting an update is not this skill's job. Exact wording is the builder's.

### 4. Shared rules in `SKILL.md`

- **Resolve where tickets go.** Read `tickets.provider`, `tickets.site`, `tickets.project` from `diagnostic.config`. When one the path needs is empty, ask the person for this run. The skill never edits config.
- **Find the connector** for the provider, including through any discovery tool. None available: say so, create and record nothing, and stop.
- **Known links come from `status --json`'s `records`** (rows of kind `change`, their `ticket`). A change is linked only when that object is present. A change whose record shows a `ticket-link-invalid` finding (a hand-edited partial or invalid set) is not linked, but `change.link-ticket` will not write over it without `replace: true` — so every path that would write a link to it stops first and asks the person to fix or remove the broken link (link with `replace: true`, or unlink) before anything else happens.
- **One ticket may be on several changes** (decided in grooming: one ticket can be split into several docket changes). Linking or importing a ticket that another change already carries is allowed; the agent names the other change(s) ("CLOUD-123 is also on #10") and goes ahead.

### 5. Link path

1. The person gives a change and a ticket (URL, or id when the site is configured).
2. **Provider:** when the URL's host matches `tickets.site`, the provider is `tickets.provider`; otherwise ask. **Id:** the ticket's key as the provider shows it, read through the connector (preferred) or from the URL.
3. Confirm the ticket exists through the connector when a connector is available. With none, the person may still link by URL — the link is a record, not a ticket call — and the report says the ticket was not verified. *(Builder's call how to word it.)*
4. If other changes already carry this ticket, name them (allowed; see shared rules).
5. Run `change.link-ticket`. `already-linked-elsewhere` → tell the person which ticket the change carries now and replace only if they say so (`replace: true`). Unlink only on request.

### 6. Push path (`references/create.md`)

1. **Already linked?** If the change's `status --json` record shows a `ticket`, refuse and name it ("#12 is already linked to CLOUD-123 <url>"). Nothing else happens. A `ticket-link-invalid` finding on the change also stops here (shared rules): otherwise the ticket would be created and could never be recorded.
2. **Readiness check** (settled criteria above).
3. **Refinement, only if needed** (settled above): targeted questions, answers written through `change.groom` `outcome: revise` owned sections (`## Acceptance criteria` included) while `proposed`; past `proposed`, name what is missing and let the person decide; never invent.
4. **Ticket type** (settled mapping above).
5. **Export the body** by invoking `docket-export` as a **file request** — never restating its rules — with a call-site direction: export `change <id>` to a fresh temporary directory (`--output-path` from `mktemp -d "${TMPDIR:-/tmp}/docket-link-ticket.XXXXXX"`), and return the path and, in a private repository, the before → after list; post nothing. docket-export's file-request path already runs the private rewrite pass and word check and returns the list with no stop, because nothing is posted; the list goes into this skill's draft instead, so the person approves once. (581's answer (b) recommended this same reuse for unattended updates.)
6. **Spec, when the change has a linked `spec:`** — always included, with no separate question (decided in grooming: the person's go on the draft is the only approval). It is labeled as the proposed approach, never as requirements:
   - **Shared repository:** the body ends with one line, e.g. `Proposed approach: <link>`, where the link is the URL `docket open spec <id> --print --json` returns (the spec on the metadata branch on GitHub). A branch link, not a commit-pinned one: the description is written once and never rewritten, so a pinned link would go stale after a later spec revision; the branch link always shows the current design.
   - **Shared repository with no URL:** `open` returns a URL only when the remote is GitHub and `open.artifacts` is not `local`; otherwise `target_kind` is `file`. A local path is never put in a ticket: the spec is exported (docket-export file request, as for the body) and attached exactly as in a private repository.
   - **Private repository:** the spec is exported through `docket-export` as a file request (as for the body), with the same private cleanup, and attached to the ticket after it exists. The body ends with one line naming the attachment, e.g. `Proposed approach: see the attached <filename>`.
7. **Draft and approval.** Show the person, in one message: the title, the project, the mapped type, the body as it will be posted, the spec line or attachment name, and — in a private repository — the before → after list of every rewrite in the body and the spec. Create only on their go. A docket-export word-check hit stops the draft the way docket-export does: show the lines, post nothing until the person decides.
8. **Create** the ticket through the connector (summary = the export's H1; description = the rest, formatting converted only).
9. **Attach (private, spec present).** Upload the cleaned spec. In the Claude Code harness the Jira connector attaches in two steps (`uploadAttachmentToJiraIssue` returns an upload command to run from the shell, then the returned file id is attached); never assume a one-call upload, and never substitute a local path for an attachment. If the upload fails, the ticket stays (its description cannot be rewritten): report the failure and the exported file's path so the person can attach it by hand.
10. **Record** with `change.link-ticket` (provider, the created ticket's id, its URL) after the connector confirms creation. If recording fails, report the created ticket's URL and retry the link (fresh `path` + `revision`); **never create a second ticket**.
11. **Report:** the ticket link, the type, whether the spec was linked or attached, any rewrite list.

### 7. Import path (`references/import.md`)

1. **Read the ticket** through the connector.
2. **Already on a change?** If any change record in `status --json` (active or archived) carries a `ticket` with the same provider and id, name it in the draft ("CLOUD-123 is also on #10"). Allowed, not refused (shared rules); the person's go on the draft decides.
3. **Distill** into a draft change: title from the ticket summary; Why, What changes, Acceptance criteria, Out of scope, Open questions from its content; `type` mapped as settled. Never invent: `change.create` requires Why, What changes, and Out of scope — when the ticket gives no basis for one, ask the person.
4. **Draft and approval.** Show the drafted change (title, type, sections, ticket) and create on the person's go.
5. **Create** with `change.create` (`ticket_provider`, `ticket_id`, `ticket_url` set; default priority). `change.create` cannot write `## Open questions`; when the draft has open questions, follow with one `change.groom` `outcome: revise` carrying that section (fresh `path` + `revision` from `status --json`).
6. **Report** the new change id and its link.

The imported change is a plain needs-grooming stub (no spec); the person grooms it as usual.

### 8. `docket-export` stops claiming ticket work

`skills/docket-export/SKILL.md`'s description drops "a Jira ticket body or comment" (keeping documents, messages, emails, and "anything outside docket"); its body gains one sentence: creating a ticket from a change, or linking or importing one, is `docket-link-ticket`'s, which calls this skill for a file export. The sentence must not send a file request from `docket-link-ticket` back there — the two skills point at each other only in that one direction each. Posting an update comment is not added back here — that will be `docket-update-ticket`'s (585). `TestExportSkillIsInline`'s description clauses and the `change_0578_export_skill` prose sentinels are updated to match.

### 9. Convention and docs

- The convention (`skills/docket-convention/SKILL.md`, agent-layer paragraph) and `references/agent-layer.md` (fork-exclusion principle) name `docket-link-ticket` beside `docket-export` as an inline skill.
- `docs/reference/skills-and-agents.md` gains a `docket-link-ticket` entry; the `docket-export` entry drops "or put one into another tool" only if it reads as ticket creation (builder's call).
- Generated surfaces: regenerate the embedded tree and manifest with `go generate ./internal/assets/`. Install picks up the new skill by glob; no registration code is needed (as with `docket-export` in 578).
- Skill prose ships into other repositories: no sentence true only in the docket repository (learning `distributed-body-has-no-local-repo`), and no sandbox site, project, or real ticket key.

### 10. ADR

Record via docket-adr at build: **Ticket work is two inline skills, split by attended linking versus updating** — `docket-link-ticket` (link, create, import; always the person) and `docket-update-ticket` (updates; the person or nobody), both inline, harness-neutral, posting with the agent's own connector tools; autonomous workflows only ever load the update skill, so refinement and import instructions never take space in an implement or finalize run. 581's plan assigned this ADR to "the first skill built"; that is now this change, not 585.

## Acceptance criteria

- `status --json`'s `records` rows of kind `change` show `ticket: {provider, id, url}` on a linked change (active and archived) and no `ticket` on an unlinked change or one with a partial or invalid set; the `changes` rows are unchanged; the `status` schema descriptor lists it.
- `skills/docket-link-ticket/SKILL.md` is inline (frontmatter exactly `name`, `description`; no agent wrapper), and its text with its two references covers: link (provider from site match or asked; replace and unlink only on request; a ticket already on another change is named and allowed; a broken link stops first); push (already-linked and broken-link refusals, readiness criteria, refinement only while proposed, never invent, type mapping, draft with one approval, spec always included — branch link shared, attachment private or when no URL — create, attach in two steps, record only after confirmation, no second ticket on a failed record); import (a ticket already on a change is named and allowed, never invent, draft approval, ticket fields at creation, open questions follow-up).
- `docket-export`'s description no longer mentions a Jira ticket body or comment, and its body points ticket work to `docket-link-ticket`.
- Repository tests guard the skill's inline shape and its key rules (prose sentinels bound to their claims), and budget rows exist for the new files.
- Results list an Important live check, run from another repository against a sandbox ticket project, covering push from a shared and from a private repository (with a spec), link, and import.

## Testing

- **No ticket-system call is ever made from the docket repository.** No build, test, review, or results run posts to, creates, or reads a real ticket, and docket's own `.docket.yml` never carries `tickets.*` keys (583's guard). Connector behavior lives in skill text only.
- `status --json`'s `ticket` gets Go tests: linked, unlinked, partial, invalid, archived, and filtered out of `changes` by a type filter but still in `records`.
- Prose guards follow the repo's rules: mutation-test each, bind phrases to claims, collapse whitespace before matching.
- **Live check (human, results `## Human actions and testing`, Important):** after the PR opens and before merge, from a scratch repository (shared, and private with `tickets.*` in `.git/dckt/config.yml`) against a sandbox project: push a groomed change with a spec, push a stub that needs refinement, link an existing ticket, import a ticket. Not a build stop; no `hold`.

## Left to the builder

Exact skill wording and description; how `StatusRecord` reuses `ContextTicket`; test names and sentinel phrasing; how the link path words an unverified link; the exact "Proposed approach" line format.
