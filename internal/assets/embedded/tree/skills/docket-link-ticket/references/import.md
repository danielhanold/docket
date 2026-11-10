# Making a change from a ticket (import path)

Loaded from `SKILL.md` when the person asks to make a change from a ticket. The shared rules in `SKILL.md` apply to every step; they are not repeated here.

1. **Read the ticket** through the connector: its summary, description, type, and URL.

2. **Already on a change?** Check the known links (shared rules). If any change record, active or archived, carries a `ticket` with the same provider and id, name it in the draft (`CLOUD-123 is also on #10`) and go ahead. One ticket may be split across several changes, so this is never a refusal; the person's go on the draft decides.

3. **Distill** the ticket into a draft change. The title comes from the ticket's summary; Why, What changes, Acceptance criteria, Out of scope, and Open questions come from its content. Never invent: when the ticket gives no basis for Why, What changes, or Out of scope, ask the person. Read the configured change types from the `diagnostic.config` operation at `effective.change_types.value`. Map the ticket's type to one of the configured `change_types` (a Bug to `fix`, a Story to `feat`); with no obvious match, ask.

4. **Draft and approval.** Show the drafted change (title, type, sections, and ticket) and create it only on the person's go.

5. **Create** the change with the `change.create` operation, building the request from its `schema` descriptor: set `ticket_provider`, `ticket_id`, and `ticket_url`, leave the priority at its default, and give it a stable `request_id` so a retry cannot create a second change. `change.create` does not write `## Open questions`. When the draft has open questions, follow with one `change.groom` `outcome: revise` carrying `## Open questions`, using a fresh `path` and `revision` from the `status` operation.

6. **Report** the new change's id and its link. The change is a needs-grooming stub with no spec; the person grooms it as usual. The import happens once: nothing is synced from the ticket afterwards.
