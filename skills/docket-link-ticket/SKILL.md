---
name: docket-link-ticket
description: 'Use when the person asks to link a change in docket to a ticket (`link #12 to CLOUD-123`), to create a ticket from a change (`create a Jira ticket from #12`, `put #12 in Jira`), or to make a change from a ticket (`make a change from CLOUD-123`) — always with the person present; not for posting an update or comment to a ticket.'
---

# docket-link-ticket

This skill runs inline, in your own session, and only when the person asks: no autonomous workflow ever creates, links, or imports a ticket. You reach the ticket system with your own tools, and you ask the person directly where a step below says to.

Words used here: a **ticket** is the item in the team's ticket system; the **provider** is which ticket system it is (`jira`, `linear`, …); a **connector** is the session's own tools for that system (an MCP server or plugin the person installed and signed in to). Docket installs no connector and stores no credential.

## Convention (load first — blocking)

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *startup check*. Everything below uses its vocabulary.

Writes are change records on the metadata branch, made only through the `change.link-ticket`, `change.groom`, and `change.create` operations (argv from the capability catalog). Build each request body from that operation's `schema` descriptor. A `contended` refusal means re-sync, re-read `path` and `revision`, and retry, or stop if the change moved out from under the request.

## Shared rules

- **Where tickets go.** Read `tickets.provider`, `tickets.site`, and `tickets.project` from the `diagnostic.config` operation at `effective.tickets.<key>.value`. When one the path needs is empty, ask the person for it for this run; never edit configuration. `project` maps to the provider's own term: a Jira project, a Linear team, a GitHub repository.
- **Find the connector** for the provider. Never conclude an operation is missing because it is not in your tool list: some connectors expose operations only through a discovery tool (the Atlassian connector's `discover`), so look there first. With no connector, creating and importing stop with nothing created or recorded. Linking may still go ahead by URL (link path, step 3).
- **Known links.** Known links come from the `records` array of the `status` operation run with `--json --records` — rows of kind `change`, whose `path` and `revision` feed every write. A change is linked only when its record carries a `ticket` object.
- **Broken links.** A change with a `ticket-link-invalid` finding is not linked, but `change.link-ticket` will not write over it without `replace: true`: before anything else, stop and ask the person to fix the broken link or remove it. Fixing it is a link with `replace: true`; removing it is `unlink: true`.
- **Shared tickets.** One ticket may be on several changes. When another change already carries this ticket, name it (`CLOUD-123 is also on #10`) and go ahead.
- **Confirmation.** Record only after the connector confirms. Never report a ticket as created, found, or attached without the connector's confirmation.
- **Failures.** A missing connector or a failed call is said plainly. An uncertain outcome is reported as uncertain, and the person decides whether to try again. Never run a retry loop of your own.
- **Not this skill's.** Posting an update or comment to a ticket (`update CLOUD-123 now that the PR is open`).

## Routing

When creating a ticket from a change, **read `references/create.md` now (blocking)**. When making a change from a ticket, **read `references/import.md` now (blocking)**. Linking an existing ticket stays here.

## Link path

1. The person names a change and a ticket: a URL, or an id when `tickets.site` is set.
2. **Provider and id.** When the URL's host matches `tickets.site`, the provider is `tickets.provider`; otherwise ask the person. The id is the ticket's key as the provider shows it, read through the connector (preferred) or from the URL. Given only an id, the provider is `tickets.provider` and the URL comes from the connector's read of the ticket; with no connector, ask the person for the URL.
3. **Check the ticket.** Confirm the ticket exists through the connector when one is available. With none, the person may still link by URL — the link is a record, not a ticket call — and the report says the ticket was not verified.
4. **Other changes.** Name any other change that already carries this ticket (shared rules).
5. **Record it.** Run the `change.link-ticket` operation with the change's `path` and `revision`. On `already-linked-elsewhere`, tell the person which ticket the change carries now, and send `replace: true` only if they say so. Unlink only when the person asks. Report the disposition in plain words: `linked` (now linked), `already-linked` (it already carried this ticket), `replaced` (the old link was swapped for the new one), `unlinked` (the link was removed), or `nothing-to-unlink` (it had no link to remove).
