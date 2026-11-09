<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0581 — Split the ticket integration design into buildable changes](../changes/active/0581-split-the-ticket-integration-design-into-buildable-changes.md)**
<!-- docket:backlink:end -->

# Split the ticket integration design into buildable changes — Results

**Human action:** Yes. Create the six changes below as plain proposed stubs, using the field values and stub text given for each, then groom and build them one at a time in the order listed. Nothing else is needed to close this change.

## Outcome

The ticket integration in the linked spec is too big for one PR. This plan cuts it into six changes, each roughly one PR, in build order. Each change leaves docket working, and from change 2 on each one adds something a person can use: link a ticket, create or import one, post an update on request (shared, then private repositories), and finally automatic updates at each event.

The plan was made after re-tracing the code the spec names at main `89e2c58b0` (unchanged since the spec was written). Every fact in the spec held. Three small refinements are in *Trace notes*. The three questions the spec left to this run are answered:

- **(a) What produces `stacked-merged`.** The finalize close-out does: `finalize.closeout` marks the child `stacked-merged` (`closeoutStacked` calls `domain.MarkStackedMerged`), and the maintenance sweep reaches the same code because it calls `FinalizeCloseout`. So the sentence in `stacked-changes.md` calling the sweep "the only producer" is out of date. For tickets this means: a stacked child that docket-finalize-change merges into its parent gets its automatic PR-merged update from docket-finalize-change, like any other merge; a stacked child the sweep closes out gets nothing, under the "maintenance never posts" rule, and the human can ask for the update.
- **(b) Skipping docket-export's private before → after stop for automatic updates.** Recommend a direction from the calling skill (ADR-0044's `DIRECTED to:` pattern: "nobody is present; never stop for review"), not a new docket-export mode. docket-export is an inline skill; the stop is a human interaction, and an autonomous caller already overrides human interactions this way everywhere else. A new mode would add a second code path in a skill whose rule set must stay identical in both. The safety net that replaces the human review is the leak-pattern check, which lives in `docket-update-ticket` (change 6), not in docket-export.
- **(c) The read operation's shape.** A new read-only operation (suggested name `context.ticket`, `--id <id> --event <event> --json`), modelled on `context.implementation`: one fresh snapshot, one envelope. It returns: the ticket fields and a link state (`linked`, `not-linked`, `partial`); `visibility`; the event's switch value; whether the event has happened, plus the facts the update needs (current status, PR URL, merge destination and whether it is the integration branch or a parent branch); the relevant artifact paths; and, in a shared repository, their GitHub URLs pinned to the metadata commit read. Existing reads cover only fragments: `repository.prepare` gives `visibility`; `diagnostic.config` gives `effective.tickets.auto.<event>.value`; `status --json` gives each change's `status`, `path`, and `revision` but no spec, plan, results, PR, or ticket fields. Composing three calls in the skill would push judgment-free work onto the agent, against ADR-0012, so one operation is justified.

### The changes, in build order

#### 1. Add acceptance criteria to changes and rename export's issue kind to ticket

- **Type:** feat
- **Covers:** decisions 6 and 7
- **depends_on:** none
- **Touches:** `change.create` and `change.groom` owned sections; the convention's body-section list (and its embedded twin); `internal/export` (`KindIssue` → `KindTicket`, `issueSections` → `ticketSections`, `stepIssueSections`, `keepIssueSections`, tests `issueH2`, `issue-x.md`); `internal/app/open_export.go` `exportKinds`
- **Size:** S–M
- **Stub text:**
  - **Why:** A ticket needs acceptance criteria that belong to the change, and the export that will become a ticket body is named after GitHub issues. Both are small foundations every later ticket change uses.
  - **What changes:** An optional `## Acceptance criteria` body section, after What changes and before Out of scope, settable by `change.create` and editable through `change.groom`'s owned sections (outcomes `spec` and `revise`), documented in the convention. Export's `issue` kind becomes `ticket`, internal identifiers renamed with it, its section allowlist gains Acceptance criteria, and a change exports as `<date>-ticket-<slug>.md`. Fixed by the spec: placement, optional and never a gate, no compatibility alias. Left to grooming: exact placement wording in the convention, test shape.
  - **Acceptance criteria:** A change created or groomed with acceptance criteria shows the section in place; `docket open change <id> --export` writes `<date>-ticket-<slug>.md` and keeps the section; a change without the section builds and exports exactly as before.
  - **Out of scope:** Reviewers checking a build against acceptance criteria; the legacy `issue:` frontmatter field (change 2).

#### 2. Link a change to a ticket

- **Type:** feat
- **Covers:** decision 5 (all parts, including the retirement of `issue:` and the PR-body ticket line), decision 8 (config keys), decision 13's repository guard
- **depends_on:** none
- **Touches:** frontmatter decode, domain model, `newChangeBuilder`, `ChangeRecord`, repair's field lists, `ArtifactBlockContent` (Ticket row); a new revision-pinned operation that sets or clears the three fields in any status (modelled on `change.relink`); `change.create` ticket keys; `internal/config` (`tickets.provider`, `site`, `project`, `auto.*`, all `scopeRepoOnly`) with defaults, `assemble`, `diagnostic.config`, `.docket.example.yml` (with the tool table as a comment); implement-next Step 7 (ticket line replaces the "Best-effort PR→issue reference"); convention field sketch and embedded twins
- **Size:** M–L
- **Stub text:**
  - **Why:** Every ticket capability needs a durable, typed link from a change to its ticket and a place for the team's ticket settings. The old `issue:` field is an unused leftover of the removed GitHub board mirror and would confuse the new vocabulary.
  - **What changes:** Three flat fields, `ticket_provider`, `ticket_id`, `ticket_url`, written together by one operation (link, replace, unlink; refuses a partial set; any status; `## Artifacts` re-rendered with a Ticket row) and accepted by `change.create`. A partial set is reported as a finding only. The `tickets.*` keys, per-repo only, all automatic switches off. The PR body gets one plain line linking the ticket. `issue:` is removed from decode, domain, builder, repair, and implement-next. Fixed by the spec: field names, flatness, stored provider and id, no ticket-type key, no master switch. Left to grooming: the operation's name and request shape; whether the four archived empty `issue:` values stay as unknown data or are dropped.
  - **Acceptance criteria:** Linking writes all three fields and a Ticket row in one commit; unlinking clears them; a partial request is refused; `tickets.*` set in global or local config is warned and ignored; implement-next's PR body carries the ticket line; no record or operation reads `issue:`; docket's own `.docket.yml` carries no `tickets.*` key (a test guards it).
  - **Out of scope:** Any connector call; skills that create or update tickets (changes 3–6); board changes.

#### 3. Create a ticket from a change, or a change from a ticket

- **Type:** feat
- **Covers:** decision 4 (link, import, create, ticket type), decision 10's `docket-link-ticket` half
- **depends_on:** 1, 2
- **Touches:** new inline skill `docket-link-ticket` with a create reference (readiness check, refinement, draft) and an import reference; uses docket-export's ticket kind and change 2's operation; install/skill registration and generated surfaces
- **Size:** M
- **Stub text:**
  - **Why:** An engineer should be able to hand work to a ticket system as a clear request, or pull a ticket into docket, without retyping it, while keeping requests separate from proposed solutions.
  - **What changes:** `docket-link-ticket`, always attended: link an existing ticket by URL; create a ticket from a change after a readiness check and targeted refinement written back through `change.groom` (only while `proposed`), with a draft the human approves and the ticket type mapped from the change's type through the connector; import a ticket into a new proposed change with mapped type and ticket fields set. Records the fields only after the connector confirms. Fixed by the spec: the readiness criteria, never inventing scope, the type mapping rule, description written once. Left to grooming: reference file layout and trigger wording.
  - **Acceptance criteria:** The skill's text covers each path and every refusal the spec names; repository tests check its structure; results list an Important live check against a sandbox project from another repository.
  - **Out of scope:** Updates (changes 4–6); automatic ticket creation at change creation (change 6); any ticket call from this repository.

#### 4. Post a ticket update on request (shared repositories)

- **Type:** feat
- **Covers:** decision 1 (on request), decision 2 (event preconditions and the on-request-only cases), decision 3, decision 9 (shared delivery), decision 10's read operation and the `docket-update-ticket` skill, decision 11, decision 12
- **depends_on:** 2
- **Touches:** new read operation (`context.ticket`, see answer (c)); a commit-pinned URL builder beside `BlobURL` in `internal/render/link.go`; new inline skill `docket-update-ticket` with its per-event table and a shared-delivery reference
- **Size:** M–L
- **Stub text:**
  - **Why:** People working in the ticket system should get short, deliberate updates at the moments that matter, with a link to the artifact as it stood then.
  - **What changes:** One read returns everything an update needs for a change and an event. `docket-update-ticket` composes the update under the editorial contract and posts it as a comment with a commit-pinned link, only for a linked ticket and only if the event has happened; a late update never misstates the current state. Failures are one report line; nothing is reported posted without the connector's confirmation. Fixed by the spec: the five events and their content, no ledger or retry, plans never go out. Left to grooming: the read operation's exact fields, whether shared mode also adds a native web link.
  - **Acceptance criteria:** The read returns ticket state, visibility, switch, event facts, artifact paths, and pinned URLs, with tests; the skill refuses a change without a ticket and an event that has not happened; results list an Important live check.
  - **Out of scope:** Private repositories (change 5); automatic posting (change 6).

#### 5. Post a ticket update on request in a private repository

- **Type:** feat
- **Covers:** decision 9 (private delivery, attended; the two-step attachment)
- **depends_on:** 4
- **Touches:** `docket-update-ticket`'s private-delivery reference; uses docket-export unchanged (attended)
- **Size:** S–M
- **Stub text:**
  - **Why:** In a private repository the artifact cannot be linked; it must be cleaned and attached without leaking docket.
  - **What changes:** The update exports and cleans the artifact through docket-export with its normal before → after review, uploads the attachment first (in two steps where the connector needs it) and posts the comment last, naming the attachment. A failed required upload posts nothing.
  - **Acceptance criteria:** The skill text covers the attachment order and failure path; results list an Important live check from a private scratch repository.
  - **Out of scope:** Unattended private posting and the leak-pattern check (change 6).

#### 6. Update tickets automatically after each event

- **Type:** feat
- **Covers:** decision 1 (automatic, off means nothing), decision 2 (automatic per event, maintenance and stack fan-out exclusions), decision 9 (unattended private posting, the final check, unknown visibility), decision 10 (workflow wiring, the leak matcher exposed, docket-export directed skip)
- **depends_on:** 3, 5
- **Touches:** a read-only operation exposing `internal/leakscan` for arbitrary text; call sites in docket-new-change, docket-groom-next, docket-auto-groom, docket-implement-next (PR handoff and results-artifact ending), docket-finalize-change (after close-out); `docket-update-ticket` unattended rules
- **Size:** L (grooming may split it, for example change-created separately)
- **Stub text:**
  - **Why:** Teams that want it should get updates without the engineer remembering to ask, and nothing at all when they do not.
  - **What changes:** At each event the running workflow reads its switch; when on, it invokes the ticket skill with a direction that nobody is there to ask. Off means nothing happens: no prompt, no report line. Unattended private posts skip docket-export's review and must pass a final check with the leak check's existing patterns; any hit posts nothing. The sweep never posts; stacked descendants get nothing. Fixed by the spec: per-event switches only, failures never halt, unknown visibility treated as private. Left to grooming: whether change-created's readiness check runs before or after `change.create`, and the matcher operation's name.
  - **Acceptance criteria:** Each wiring site is guarded by a test; with every switch off no workflow output changes; the matcher returns the same hits as the publish leak check for the same text.
  - **Out of scope:** Widening the leak patterns; confirmation prompts; updates for revisions, blockers, or kills.

### Field values for every stub

Each stub: `status: proposed`, no `spec:`, `auto_groomable` unset, `related: [581]`, `discovered_from: [581]`, `priority: medium`, and `depends_on` set to the ids actually assigned to the changes named above (positions: 1 → none, 2 → none, 3 → 1 and 2, 4 → 2, 5 → 4, 6 → 3 and 5).

### Coverage check

| Spec part | Change |
|---|---|
| 1 on request | 4 |
| 1 automatic per event; off means nothing; no ledger | 6 |
| 1 update needs a linked ticket | 4 |
| 2 event preconditions and content table | 4 |
| 2 on-request-only cases (revision, blocker, kill) | 4 |
| 2 no update on sweep; no stack fan-out | 6 |
| 2 plans never go out | 4 |
| 3 who posts, failure lines, attachment order | 4 |
| 4 link | 3 (operation in 2) |
| 4 import | 3 |
| 4 create (readiness, refinement, draft, approval, record, ticket type) | 3 |
| 4 automatic ticket at change created | 6 |
| 5 fields, operation, partial-set finding, Artifacts row, `change.create` keys | 2 |
| 5 PR-body ticket line; retire `issue:` | 2 |
| 6 acceptance criteria | 1 |
| 7 export kind rename | 1 |
| 8 config keys | 2 |
| 9 shared delivery, commit-pinned URLs | 4 |
| 9 private delivery, attended | 5 |
| 9 private unattended, final check, unknown visibility | 6 |
| 10 `docket-link-ticket` | 3 |
| 10 `docket-update-ticket` and the read operation | 4 |
| 10 docket-export directed skip, workflow wiring, leak matcher | 6 |
| 11 editorial contract | 4 |
| 12 harness responsibility | 4 |
| 13 no ticket calls; `.docket.yml` guard | 2 (guard); every change follows the rule |
| 13 live checks as results steps | each of 3, 4, 5, 6 |
| Out of scope: status sync, description rewrite, inbound sync, deployment, follow-up tickets | 4 (guarded in skill text) |
| Out of scope: ledger, queue, retry, exactly-once; confirmation prompts; master and enable-all switches | 6 (switches: 2) |
| Out of scope: automatic updates for revisions, blockers, kills, sweep; stack fan-out | 6 |
| Out of scope: credentials and connector setup | 4 |
| Out of scope: reviewing against acceptance criteria | 1 |
| Out of scope: board change | 2 |
| Out of scope: widening leak patterns | 6 |

### Build order

1 and 2 are independent and can be built in either order. 3 needs 1 and 2. 4 needs 2 and is independent of 1 and 3. 5 needs 4. 6 needs 3 and 5 and comes last. A workable sequence is 1, 2, 4, 3, 5, 6, or 2, 4, 5 first if updates matter more than ticket creation.

### ADRs

| Decision | Recorded by |
|---|---|
| Ticket fields are three flat stored scalars, and `issue:` is retired | change 2 |
| Two inline skills split by attended linking versus updating | change 3 |
| Off means nothing happens: per-event switches, no ledger, queue, or retry | change 6 |
| Unattended private posting skips human review behind the leak-pattern check | change 6 |

The per-repo fence for `tickets.*` applies ADR-0019 and needs no new ADR.

### Trace notes

- **`stacked-merged` producer** (answer (a)): code produces it in `FinalizeCloseout` for both `finalize.closeout` and `maintenance.sweep`; `skills/docket-convention/references/stacked-changes.md` ("The merge sweep ... is the only producer of the state") is stale. Refines the spec's stacked-child rule: the finalize workflow's merge, not the sweep's, can fire the automatic update.
- **`ChangeRecord` has no `issue` field** (`internal/render/record.go`): retiring `issue:` does not touch it, but the three new ticket fields must be added to it.
- **Embedded twin**: the "Best-effort PR→issue reference" also lives in `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md`, regenerated with `go generate ./internal/assets/`.
- **Leak-check callers**: confirmed as `runLeakCheck` in `workspace_ops.go`, `pr_publish.go`, `finalize_publish.go`; `internal/leakscan` exports `Scan`, `Text`, `Line`, none reachable for arbitrary text.
- **`-issue-` filename**: named only in the archived spec of change 561, a point-in-time record that stays as written.
- All other facts in *What the code says today* hold at main `89e2c58b0`.

## Verification performed

- Re-traced every code fact the spec names by grep and file reads in the feature worktree at main `89e2c58b0`; no code was changed and nothing was committed.
- Backlog check: 17 proposed or deferred changes read by title and scope; none covers ticket integration, so the six changes are all new.

## Known issues and follow-ups

### `stacked-changes.md` says the sweep is the only producer of `stacked-merged`

Anyone reading the convention reference is told the wrong path; the finalize close-out produces the state too. Confirmed (see *Trace notes*). Harmless at run time; only documentation is wrong. Suggested next action: correct the sentence. No existing change fits (checked 17); capture a new change.

### The convention still names a human-present close-out exception

The convention's autonomy paragraph says "docket-finalize-change's human-present close-out is the one exception inside an autonomous file", while `internal/repoguard/skill_handoff_sites_test.go` asserts none remain. Confirmed as text; harmless at run time. Suggested next action: delete the sentence. No existing change fits (checked 17); capture a new change.
