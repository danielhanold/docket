# Creating a ticket from a change (push path)

Loaded from `SKILL.md` when the person asks to create a ticket from a change. The shared rules in `SKILL.md` apply to every step; they are not repeated here. Read the change's record from the `status` operation and the change file itself before step 1.

1. **Already linked.** If the change's record carries a `ticket`, refuse and name it (`#12 is already linked to CLOUD-123 <url>`); nothing else happens. A `ticket-link-invalid` finding on the change also stops here: otherwise the ticket would be created and could never be recorded. Ask the person to fix or remove the broken link first (shared rules).

2. **Readiness check.** Ask one question of the change: Could another engineer take ownership of this as a user story? It can when the change states:
   - the problem and why it matters;
   - the desired outcome;
   - bounded scope and non-goals;
   - genuine constraints, kept distinct from implementation preferences;
   - clear, verifiable acceptance criteria that belong to this change;
   - no open question that blocks understanding.

   Technical questions may stay open: the request must be ready for ownership, not necessarily for coding. A spec is not required, and an existing spec never turns its technical choices into requirements.

3. **Refinement, only if needed.** When the check finds gaps, ask the person targeted questions about what is missing — nothing more. Write the answers through `change.groom` with `outcome: revise` and its owned sections, which works only while the change is `proposed`. `## Acceptance criteria` is one of those sections. Past `proposed`, build the ticket from the change as it stands: name what is missing and let the person decide whether to go ahead. Never invent scope or acceptance criteria, and never promote an unreviewed solution into a requirement.

4. **Ticket type.** Read the ticket types the project actually has through the connector (in Jira, `listJiraProjectIssueTypesMetadata`). `fix` maps to the bug type, `feat` to the story or feature type, and any other type to the task type. With no obvious match, ask the person. A tool without ticket types (Linear, Asana, Trello) gets none.

5. **Export the body.** Invoke the `docket-export` skill as a **file request** — its rules are its own; do not restate or re-run them — **DIRECTED to:** export `change <id>` with `--output-path` set to a fresh directory from `mktemp -d "${TMPDIR:-/tmp}/docket-link-ticket.XXXXXX"`, return the path and, in a private repository, the before → after list, and post nothing. The export's H1 is the title; its body keeps Why, What changes, Acceptance criteria, Out of scope, and Open questions.

6. **Spec, when the change has a `spec:`.** The spec is always included, labeled as the proposed approach, never as requirements. There is no separate question: the person's go on the draft is the only approval.
   - **Shared repository:** run the `open` argv as `<open argv> spec <id> --print --json`. When `target_kind` is `url`, the body ends with one line, `Proposed approach: <target>`. That is a branch link, not one pinned to a commit: the description is written once, so a pinned link would go stale after a later spec revision.
   - **No URL, or a private repository:** A local path never goes into a ticket. When `target_kind` is `file`, or the repository is private, export the spec through `docket-export` as a file request (as in step 5) and attach it after the ticket exists (step 9). The body ends with `Proposed approach: see the attached <filename>`.

7. **Draft and approval.** Show the person, in one message, the title, the project, the mapped type, the body as it will be posted, the spec line or attachment name, and in a private repository the before → after list for the body and the spec. Create only on their go. A `docket-export` word-check hit stops the draft the way `docket-export` does: show the lines and post nothing until the person decides.

8. **Create** the ticket through the connector in `tickets.project`: the summary is the export's H1, and the description is the rest. When the provider needs its own markup, convert formatting only, never words. The description is written once and never rewritten.

9. **Attach** the exported spec, when step 6 chose an attachment. Never assume a one-call upload, and never substitute a local path for an attachment. Some connectors attach in two steps: the Atlassian connector's `uploadAttachmentToJiraIssue` returns an upload command to run from the shell, and the file id it returns is then attached. If the upload fails, the ticket stays, since its description cannot be rewritten: report the failure and the exported file's path so the person can attach it by hand.

10. **Record** the link with the `change.link-ticket` operation — the provider, the created ticket's id, and its URL — once the connector has confirmed the ticket was created. If recording fails, report the created ticket's URL and retry the link with a fresh `path` and `revision`; never create a second ticket.

11. **Report** the ticket link, its type, whether the spec was linked or attached, and any before → after list.
