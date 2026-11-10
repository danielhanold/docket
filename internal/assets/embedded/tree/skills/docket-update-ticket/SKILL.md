---
name: docket-update-ticket
description: 'Use when the person asks to post an update or comment about a change to its ticket (`update CLOUD-123 now that the PR is open`, `tell the ticket the spec for #12 is written`, `let PROJ-9 know #40 was killed`); not for creating or linking a ticket.'
---

# docket-update-ticket

This skill runs inline, in your own session, and only when the person asks: it posts one short comment to a change's linked ticket with your own tools. Nothing here runs on its own after an event.

Words used here: a **ticket** is the item in the team's ticket system; the **provider** is which ticket system it is (`jira`, `linear`, …); a **connector** is the session's own tools for that system (an MCP server or plugin the person installed and signed in to); an **update** is the comment you post, with its links. Docket installs no connector and stores no credential.

## Convention (load first — blocking)

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *startup check*. Everything below uses its vocabulary.

This skill makes no docket write. Its one read is the `context.ticket` operation (argv from the capability catalog), run with `--id <id>`, `--event <event>` when the update follows an event, and `--json`. Its result is under `context`: `visibility`, `change` (`status`, `deliverable`, `path`), `ticket` (`state`: `linked`, `not-linked`, or `partial`; and `provider`, `id`, `url` when `linked`), `event` (`happened`, and `reason` when not), `facts` (`pr_url`, `merge`), and `artifacts` (`role`, `path`, `url`).

## 1. Work out the request

- **The change.** Usually named (`#612`). When the request names only a ticket, find the changes whose record carries that ticket in the `records` array of the `status` operation run with `--json --records`. Exactly one: use it. Several (one ticket may be on several changes): ask the person which. None: say no change carries that ticket.
- **The event.** Map the person's words to one event: spec written (`spec_written`), PR opened (`pr_opened`), PR merged (`pr_merged`), or results completed (`results_completed`). A material revision, a blocker or a decision that is needed, or a killed change has no event: run the read without `--event`.
- **People to mention**, when the person names any (section 5).
- **Creating a ticket** from a change is `docket-link-ticket`'s job; point the person there.

## 2. Refuse when an update is not possible

Post nothing and say why in one plain line when:

- `ticket.state` is `not-linked`: say the change has no ticket (`#612 has no ticket`).
- `ticket.state` is `partial`: the link is broken; point the person to `docket-link-ticket` to fix or remove it.
- The request names a different ticket than `ticket.id`: say which ticket the change carries.
- `event.happened` is false: name the missing fact from `event.reason`. `no-spec`: the change has no spec. `no-pr`: it has no PR. `not-results-artifact`: it does not end in a results file. `not-completed`: its results are not complete. `merge-not-recorded`: say docket has not recorded a merge for the change and that, if the PR has merged, refreshing with `docket-status` records it; never say the PR has not merged.
- `visibility` is `private`: say private repositories are not supported yet.

## 3. Write the update

Read what the update needs: each file in `artifacts` at its `path`, and the change record at `change.path` for `## Closeout notes` or `## Why killed`. The record is read, never linked. Plans never go out: no link to a plan and nothing from one.

| Update | It says | Links |
|---|---|---|
| spec written | the approach and key decisions, and where the change stands now from its `status` (designed, not yet built; being built; built, PR open; merged); never a claim of review, approval, or authorship | the spec |
| PR opened | the PR link, what was built, what the reviewer needs to know | the PR, the spec, and the results file |
| PR merged | what merged and where (`facts.merge.destination_branch`; when `in_integration` is false, say it is not yet in the integration branch), key outcomes, and anything left to do, from the results file's Outcome and Known issues and the record's `## Closeout notes`; never a claim of deployment or acceptance | the results file |
| results completed | findings, conclusions, limits, and any action needed | the results file |
| revision, blocker, or kill (no event) | what the person said, backed by the artifacts; for a kill, the record's `## Why killed` | the artifacts it discusses |

A late update never misstates the current state: compare the event with the current `change.status` and say what has happened since (a PR-opened update asked for after the merge says the PR has since merged).

**Editorial standard.** Updates are for people: a few short sentences or bullets, scaled to how much the update matters. Include, when relevant, what happened; what matters (key outcomes, consequential decisions, limitations); what is needed next from the reader (an action, test, review, or decision); and the links. Leave out how you worked, brainstorming notes and work logs, file lists, test inventories, command output, repeated background, filler, empty headings, and pasted artifact contents. Keep every material blocker, risk, and required action, however short the update. Say only what the change and its evidence support: a merge is not a deployment or an acceptance.

**Layout.** Write for a person reading the comment in the ticket system. Lay it out so it scans: short paragraphs, a list, or a single line, whichever reads best for this update. Use the format the connector accepts, so the layout survives posting. Label links (`Spec:`, `Results:`). The examples show tone, length, and layout:

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

## 4. Links

When `visibility` is `shared`, **read `references/shared.md` now (blocking)**.

## 5. Mentions

A mention makes the ticket system notify a person. Mention only people the person names, and only as they ask. For each name, search the connector's users (in Jira, its user lookup, which may be reachable only through its discovery tool).

- **Exactly one match:** mention that person and post; there is no approval step.
- **Several matches:** ask the person which one, listing each candidate's name and email.
- **No match:** ask the person whether to post without that mention or try a different spelling.
- Use the ticket system's own mention form, the one that notifies (in Jira, a mention bound to the person's account id). Never substitute plain text such as "@Jane", which notifies nobody. If the connector cannot make a real mention, tell the person before posting.
- Mentions lead the comment. Nothing about mentions is saved.

## 6. Post and report

- **Find the connector** for `ticket.provider`. Never conclude an operation is missing because it is not in your tool list: some connectors expose operations only through a discovery tool (the Atlassian connector's `discover`), so look there first.
- **Post** the comment to `ticket.id`, making connector calls one at a time. Do not show a draft and wait: the person asked for the update. When they ask to see it first, show it and post on their go.
- **Report.** Report the update as posted only with the connector's confirmation, naming the ticket and anyone mentioned. For a missing connector, a failed post, or an uncertain outcome, say in one plain line what was not posted and why; an uncertain outcome is reported as uncertain, and the person decides whether to ask again. Never run a retry loop. A duplicate comment is deleted by hand.
