<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0585 — Post a ticket update on request (shared repositories)](../../changes/active/0585-post-a-ticket-update-on-request-shared-repositories.md)**
<!-- docket:backlink:end -->

# Post a ticket update on request (shared repositories): design

Change #585, groomed interactively on 2026-11-10.

## What this change delivers

A person can ask an agent to post a short update about a change to that change's linked ticket ("update CLOUD-123, the PR for #612 is ready for review"). The agent writes a few lines under a fixed editorial standard and posts them as a comment, with links to the spec or results file pinned to the commit that holds them. Two pieces:

1. **`context.ticket`**, a new read-only operation that returns, in one call, everything an update needs for a change and (optionally) an event.
2. **`docket-update-ticket`**, a new inline skill that refuses when an update is not possible, composes the comment, mentions the people the person names so the ticket system notifies them, posts it through the session's own connector tools, and reports the outcome truthfully.

This change covers shared repositories only. In a private repository the skill posts nothing and says private repositories are not supported yet; change #586 adds private delivery. Automatic posting after each event is change #587.

## Why

People who work in a ticket system should get short, deliberate updates at the moments that matter, with a link to the artifact as it stood then, without learning docket or reading an agent's working history. The most common case is a PR ready for review: the reviewer opens the PR on GitHub, reads the spec, then the code. The comment exists to get them there quickly and tell them what to watch for; it does not replace the PR.

## Vocabulary

- **ticket**: the item in the team's ticket system (a Jira issue, a Linear issue, a GitHub issue, an Azure DevOps work item).
- **provider**: which ticket system (`jira`, `linear`, …), stored on the change as `ticket_provider`.
- **connector**: the session's own tools for that system (an MCP server or plugin the harness owner installed and signed in to).
- **event**: a point in a change's life after which an update makes sense. This change handles four: `spec_written`, `pr_opened`, `pr_merged`, `results_completed` (the same tokens as the `tickets.auto.<event>` config keys).
- **update**: the comment posted to the ticket, with its links.

People say "update the ticket after the PR was opened", not event names. The skill's trigger wording uses the conversational form; event names appear only in the read operation and config.

## Settled decisions (carried from change 581; not reopened)

These were fixed by change 581's design and are restated here so the build needs nothing from that change.

1. **On request only, here.** The person asks at any time after an event has happened; the update is composed from the artifacts as they stand. If the event has not happened (a merge update for an unmerged change), the agent says so and posts nothing.
2. **A late update never misstates the current state.** Asked for the PR-opened update after the merge, it describes the PR opening and says the PR has since merged.
3. **An update needs a linked ticket.** A change with no ticket never gets one; the agent says the change has no ticket. A broken link (a hand-edited partial set) counts as not linked; the agent points the person to `docket-link-ticket` to fix or remove it.
4. **Events and content:**

   | Event | Has happened when | The update says | Links |
   |---|---|---|---|
   | spec written | the change has a spec | the approach and key decisions, plus where the change actually stands, from facts docket holds (designed, not yet built; being built; built, PR open; merged). Never a claim of review, approval, or authorship | the spec |
   | PR opened | the change has a PR | the PR link, what was built, what the reviewer needs to know | the PR, **the spec, and the results file** |
   | PR merged | the change is `done` with a merged PR, or `stacked-merged` | what merged, where (the integration branch; or the parent branch for a stacked child, worded as "not yet in <integration branch>"), key outcomes, anything left to do; from the results file's Outcome and Known issues plus the change's `## Closeout notes`. Never claims deployment or acceptance | the results file |
   | results completed | a results-only change (`deliverable: results-artifact`) is `done` | findings, conclusions, limits, any action needed | the results file |

   The PR-opened row's spec link is added by this grooming (581 linked only the results file); see *Decided here*.
5. **Updates with no event, on request only:** a material revision (including a re-groomed spec), a blocker or a decision that is needed, and a killed change. The content comes from the person's words and the artifacts (for a kill, the change's `## Why killed`).
6. **Change created is not this skill's.** Creating a ticket from a change belongs to `docket-link-ticket`; this skill points the person there.
7. **Plans never go out.** They are the agent's working document; no link or content from a plan.
8. **Who posts.** The agent running the skill, with the session's own connector tools. Docket installs no connector, stores no credential, and contains no provider API code. Some connectors expose operations only through a discovery tool (the Atlassian connector's `discover`); the skill looks there before concluding an operation is missing.
9. **Failures.** Connector missing, post failed, or outcome uncertain: one plain line naming what was not posted and why. Nothing stops, retries, or is recorded. An uncertain outcome is reported as uncertain and the person decides whether to ask again. An update is never reported as posted without the connector's confirmation. Duplicate comments are acceptable and are deleted by hand; there is no exactly-once guarantee, no ledger, no queue, and no retry.
10. **Shared delivery.** The comment links the artifact on GitHub, pinned to the commit holding the version being discussed (the version at posting time). Readers are assumed to be able to open the repository; docket checks no permissions.
11. **Editorial contract.** Updates are for humans: a few short sentences or bullets, scaled to how much the update matters. Include, when relevant: what happened; what matters (key outcomes, consequential decisions, limitations); what is needed next (an action, test, review, or decision for the reader); supporting material (the PR, the artifact links). Omit agent narration and how the agent worked, brainstorming notes and work logs, file lists, test inventories, pasted command output, repeated background, generic filler, empty headings, and pasted artifact contents. Concision must keep material blockers, risks, and required actions. Say only what the change and its evidence support: a merge is not a deployment or an acceptance.

    **Layout (decided here; a principle, not a template).** Write for a person reading the comment in the ticket system. Lay it out so it scans: short paragraphs, a list, or a single line, whichever reads best for this update. Use the format the connector accepts, so the layout survives posting. Label links (`Spec:`, `Results:`). The examples below are laid out the way a comment should read; a skill's examples teach layout more strongly than its rules, so they must be well laid out themselves.

    Examples, for tone, length, and layout:

    > **@Jane Doe** — PR ready for review: https://github.com/acme/api/pull/48
    >
    > Invalid requests now fail with a clear error before any work starts, instead of failing partway through. The API contract is unchanged, so existing clients need no update.
    >
    > Worth a close look: the validation runs before authentication, so a malformed request from an unauthenticated caller now gets a 400 instead of a 401.
    >
    > Before merging: one manual check against staging, described in the results.
    >
    > Spec: https://github.com/acme/api/blob/4f2c9e1/docs/superpowers/specs/2026-11-08-validate-requests-design.md
    > Results: https://github.com/acme/api/blob/4f2c9e1/docs/results/2026-11-08-validate-requests-results.md

    > Merged into main. Invalid requests now return a clear error before processing starts. Deployment has not been confirmed.
    >
    > Automated checks passed; one manual check remains, described in the results.
    >
    > Results: https://github.com/acme/api/blob/4f2c9e1/docs/results/2026-11-08-validate-requests-results.md

    > Merged into main. Left to do before this can ship:
    >
    > - run the staging data migration (steps in the results)
    > - confirm the new rate limit with the platform team
    > - remove the old endpoint once clients have moved (tracked separately)
    >
    > Results: https://github.com/acme/api/blob/4f2c9e1/docs/results/2026-11-08-validate-requests-results.md

    (Real links carry the full commit id.)

12. **Harness responsibility.** The harness owner installs and configures connectors, credentials, and permissions. Docket defines what to post and when. The design is generic: Jira is the first provider exercised; there is no provider framework.
13. **Testing.** No ticket-system call is ever made from the docket repository; docket's own `.docket.yml` carries no `tickets.*` key. Verification is repository tests and fixtures only. The live check is a human step outside this repository (see *Testing*).

## Decided here

1. **The read operation is `context.ticket`**, shaped as below.
2. **Pinned links point at the metadata commit the read took.** For every change built since plan and results moved to the metadata branch, the spec and results live there, so one commit pins every link in a comment. The read's metadata pin is fetched from the metadata remote, so the commit is already pushed and browsable. The spec copy committed onto the feature branch is not linked. An older change whose results file never moved (`legacyArtifactBranch` in `internal/render/artifacts.go`) is rare: an artifact not present at the metadata commit is left out of `artifacts`, and the update goes without that link.
3. **The PR-opened update links the spec as well as the results file.** The spec is one file among many in the PR; the reviewer should reach it from the comment in one click.
4. **No native ticket "web link".** Jira can add a separate entry to a ticket's links panel (`createJiraIssueRemoteIssueLink`). It is not used: every update would add another entry docket cannot tidy (it keeps no record of what it posted), and it is a Jira-only second call. Links live in the comment only.
5. **A no-event read** serves the on-request-only updates (decision 5): `context.ticket` without `--event` returns everything except the event block.
6. **The change record is read, never linked.** The skill reads the change record (for `## Closeout notes`, `## Why killed`, title, status) but a comment links only the PR, the spec, and the results file.
7. **Leftovers from change 584 fixed here:** `docket-export`'s outside-tool example ("add the spec for 9 as a comment on PROJ-123") is rewritten so export no longer implies it posts ticket comments, and that kind of request is pointed to `docket-update-ticket`; `docket-link-ticket`'s *Not this skill's* line names `docket-update-ticket`.
8. **Mentions.** The person can ask for people to be mentioned so the ticket system notifies them (see *Mentions*).
9. **A request naming only a ticket** ("update CLOUD-123 now that the PR is open") is resolved by the skill, not the read: it finds the changes carrying that ticket in the `records` array of `status --json --records`. Exactly one: use it. Several (one ticket may be on several changes): ask the person which. None: say no change carries that ticket. A request naming both a change and a ticket where the change is linked to a different ticket stops and says which ticket the change carries; nothing is posted.
10. **A merge docket has not recorded yet.** A PR merged on GitHub stays `implemented` until docket-finalize-change or docket-status records it. The read stays offline (no GitHub call), so it cannot tell the difference; its reason is `merge-not-recorded`, never a claim that the PR has not merged. The skill says docket has not recorded a merge for the change and that, if the PR has merged, refreshing with `docket-status` records it.
11. **A stale config doc from change 584 fixed here.** In `docs/reference/config-keys.md`, the `tickets.provider`, `tickets.site`, and `tickets.project` rows end "no effect yet", but `docket-link-ticket` reads them. Their endings become "used when linking or creating a ticket", "used when linking a ticket by URL or id", and "used when creating a ticket". The five `tickets.auto.*` rows keep "no effect yet" (automatic posting is change #587).

## The read operation: `context.ticket`

`docket context ticket --id <id> [--event spec_written|pr_opened|pr_merged|results_completed] [--repo-dir <dir>] --json` (argv resolved from the capability catalog like every operation; listed in the catalog with effect `read`, and its request/result described by the `schema` operation).

Modelled on `context.implementation` (`internal/app/implementation_context.go`): one fresh snapshot of the metadata branch, one protocol-v1 envelope, no writes. It works for any change, active or archived.

Result fields:

| Field | Meaning |
|---|---|
| `metadata_commit` | the full metadata commit the read took; every pinned URL uses it |
| `visibility` | `shared` or `private`, as the `repository.prepare` context reports it |
| `change` | `id`, `slug`, `title`, `status`, `deliverable`, `path` (the record's repo-relative path on the metadata branch, for reading) |
| `ticket` | `state`: `linked`, `not-linked`, or `partial` (a partial or invalid set, the same condition the `ticket-link-invalid` finding reports); plus `provider`, `id`, `url` when `linked` |
| `event` | present only with `--event`: `name`; `switch` (the effective `tickets.auto.<event>` value; reported for automatic posting later, ignored on request); `happened` (bool); `reason` when not happened, one of `no-spec`, `no-pr`, `merge-not-recorded`, `not-results-artifact`, `not-completed` |
| `facts` | `pr_url` (the record's `pr:`, empty when none); `merge`, present when the change has merged: `destination_branch` (the integration branch, or for `stacked-merged` the parent change's recorded `branch:`) and `in_integration` (true for `done`, false for `stacked-merged`) |
| `artifacts` | one entry per artifact among `spec` and `results` (never `plan`) that the record names and that exists at `metadata_commit`: `role`, `path`, and `url` (GitHub blob URL pinned to `metadata_commit`; empty in a private repository or when the repository has no web URL) |

"Has happened" rules: `spec_written` needs `spec:`; `pr_opened` needs `pr:`; `pr_merged` needs status `done` with a `pr:` (deliverable `pr`) or status `stacked-merged`; `results_completed` needs `deliverable: results-artifact` and status `done`. A killed change has happened for none of `pr_merged` and `results_completed`.

Refusals (typed, nothing returned beyond the reason): an unknown `--event` value is `invalid-input`; an unknown change id is `not-found`. "Not linked", "partial", "not happened", and "private" are **not** refusals: the read reports them and the skill decides.

**Pinned URL builder.** A new method beside `BlobURL` in `internal/render/link.go`, e.g. `BlobURLAtCommit(repoRelPath, commit string) string`: `RepoWebURL + "/blob/" + commit + "/" + repoRelPath`, empty when `RepoWebURL` is empty, the commit is empty, or `PrivateMetadata` is set (a private metadata commit has no web page). The operation obtains its `render.LinkContext` through the existing sole constructor `linkContextOf` (guarded by `TestLinkContextSoleConstructor`).

Existing reads cover only fragments (`repository.prepare` gives visibility, `diagnostic.config` the switch, `status --json --records` the ticket object), which is why one operation is justified: composing three calls would push judgment-free work onto the agent (ADR-0012).

## The skill: `docket-update-ticket`

An inline skill (not an agent, no wrapper), harness-neutral, at `skills/docket-update-ticket/` with:

- `SKILL.md`, short: when it applies, the startup check, the refusals, the per-event table (decision 4, plus the no-event updates of decision 5), the editorial contract, how to post and report, and routing to the delivery reference.
- `references/shared.md`: shared delivery (pinned links, which links go with which update, no native web link). Read only when `visibility` is `shared`. Change #586 adds `references/private.md` beside it.

Trigger description, conversational: asked to post an update or comment about a change to its ticket ("update CLOUD-123 now that the PR is open", "tell the ticket the spec for #12 is written", "let PROJ-9 know #40 was killed"); not for creating or linking a ticket.

Flow:

1. Run the convention's startup check.
2. Work out the change, the event (none for the decision-5 cases), and any people to mention from the request. A request naming only a ticket is resolved as *Decided here* 9 says. Run `context.ticket`.
3. Refuse, posting nothing, with one plain line, when: the ticket is `not-linked` ("#612 has no ticket"); it is `partial` (point to `docket-link-ticket` to fix or remove the link); the request names a different ticket than the one linked; the event has not happened (say which fact is missing, e.g. "#612 has no PR"; for `merge-not-recorded`, *Decided here* 10's wording); `visibility` is `private` ("private repositories are not supported yet"). A request to create a ticket goes to `docket-link-ticket`.
4. Read what the update needs: the artifacts named by the read, and the change record for `## Closeout notes` or `## Why killed`. Compose the comment under the editorial contract, comparing the event to the current `status` so a late update says what has happened since.
5. Find the connector for the ticket's provider (looking in any discovery tool first). Resolve any mentions (*Mentions*). Post the comment to `ticket.id`. Make connector calls one at a time.
6. Report: posted (only with the connector's confirmation, naming the ticket and anyone mentioned), or one line for not posted or uncertain. No retry loop.

The skill does not show a draft and wait for approval before posting: the person asked for the update, and the content follows the editorial contract. A person who wants to see it first can say so in the request.

## Mentions

A mention makes the ticket system notify a specific person; a plain comment waits until someone looks. The person asks for it in the request, in plain words: "update CLOUD-123, the PR for #612 is ready, and mention Jane Doe". More than one person may be named.

- **Finding the person.** For each name, the skill runs a best-effort user search through the connector (in Jira, the connector's user lookup, which may be reachable only through its discovery tool).
  - **Exactly one match:** mention that person and post; no approval step. The person asking is responsible for naming people specifically enough (a full name, not a first name alone) that a single match is the right person.
  - **Several matches:** ask the person which one, listing the candidates' names and emails.
  - **No match:** ask the person whether to post without that mention or try a different spelling.
- **A real mention only.** The comment uses the ticket system's own mention form, the one that notifies (in Jira, a mention bound to the person's account id). Plain text such as "@Jane" looks like a mention but notifies nobody, so the skill never substitutes it silently. If the connector cannot produce a real mention, the skill tells the person before posting and does not fake one.
- **Placement.** Mentions lead the comment ("Jane Doe — PR ready for review: …").
- **Report.** The report names who was mentioned, and says so only with the connector's confirmation that the comment posted.
- No saved list of people; nothing about mentions is configured or recorded.

## Registration and documentation

The new skill joins the inline skills wherever `docket-link-ticket` is listed: the convention's agent-layer paragraph (`skills/docket-convention/SKILL.md`) and `skills/docket-convention/references/agent-layer.md`, the known-skill list in `internal/harness/inventory_test.go`, line and size budgets in `internal/repoguard/budgets_test.go`, and the reference docs that list skills (`docs/reference/skills-and-agents.md`) and operations. Embedded twins under `internal/assets/embedded/tree/` are regenerated with `go generate ./internal/assets/`.

## Testing

- **`context.ticket`**, with fixtures: each event happened and not happened with the right `reason`; `pr_merged` for `done` (destination the integration branch, `in_integration` true) and `stacked-merged` (destination the parent's branch, false); a killed change; ticket `linked`, `not-linked`, `partial`; shared repository URLs pinned to `metadata_commit`; private repository with empty URLs; no plan ever in `artifacts`; the no-event read omits `event`; unknown event and unknown id refused. Catalog and schema entries covered by the existing catalog and schema tests.
- **`BlobURLAtCommit`**: shared, no web URL, empty commit, private.
- **Skill structure** (repository guards in the style of `internal/repoguard/prose_contracts_test.go` and `export_skill_test.go`): the skill names each refusal, the per-event table, the never-a-plan rule, confirmation before "posted", the mention rules (one match posts, several or none asks, never a plain-text stand-in), the layout principle (written for a person reading it in the ticket system, in the format the connector accepts, links labeled), and routing to `references/shared.md`; `docket-export` no longer offers a ticket-comment example; `docket-link-ticket` points update requests to `docket-update-ticket`.
- **No ticket-system call** in any test, build, review, or results run.
- **Results:** an Important item under `## Human actions and testing`: from another repository linked to a sandbox ticket project, ask for each of the four event updates and one no-event update, and confirm the comment, its pinned links, the refusals, and that a mention of a fully named person notifies them. Done after the PR opens and before merging; no `hold: true`.

## ADRs

None new. ADR-0162 already records the two-skill split this change completes. The pinned-URL builder and the read operation follow existing patterns.

## Out of scope

- Private repositories (change #586) and automatic posting at each event (change #587).
- Using a team's own ticket skill for project-specific details (change #591).
- A native ticket web link; attachments.
- Mentions in automatic updates (whether those ever name anyone is change #587's decision).
- Syncing ticket status, assignee, sprint, or workflow; rewriting a ticket's description; inbound sync; deployment tracking; treating a merge as acceptance.
- A ledger, delivery-state tracking, a queue, retry, or exactly-once delivery.
- Any ticket-system call from this repository.

## What the code says today

Verified at main `6dacf9706`; anchors are symbols.

- `internal/render/link.go` has `BlobURL` and `BlobURLOnBranch` (branch URLs only, empty for a private metadata branch); no commit-pinned builder. `render.LinkContext` is built only by `linkContextOf` (`internal/app/link_context.go`).
- `context.implementation` (`ImplementationContext` in `internal/app/implementation_context.go`) already carries `MetadataCommit` (from `metadataRevision`, the revision fetched from the metadata remote) and a `ContextTicket` (`provider`, `id`, `url`) built by `contextTicket`.
- The `tickets.provider`, `tickets.site`, `tickets.project`, and `tickets.auto.<event>` keys exist in `buildRegistry` (`internal/config/schema.go`), all repo-only; the switches default to `false`.
- `change.link-ticket` writes the three ticket fields; `status --json --records` reports a `ticket` object for a linked change.
- `skills/docket-link-ticket/` (SKILL.md, `references/create.md`, `references/import.md`) is the pattern for an inline ticket skill with progressive disclosure.
- `skills/docket-export/SKILL.md` still carries the outside-tool example "add the spec for 9 as a comment on PROJ-123".
