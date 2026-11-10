<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0586 — Post a ticket update on request in a private repository](../changes/archive/2026-11-10-0586-post-a-ticket-update-on-request-in-a-private-repository.md)**
<!-- docket:backlink:end -->

# Results: Post a ticket update on request in a private repository

**Human action:** Needed. Run one live check against a sandbox ticket project before relying on private delivery. No automated test can post to a real ticket system.

## Outcome

In a private repository, `docket-update-ticket` no longer refuses. It reads the new `skills/docket-update-ticket/references/private.md` and delivers the update as attachments plus a comment:

- Each spec or results file the update names is exported and cleaned through `docket-export` as a file request.
- The files are uploaded to the ticket at the post step. That comes after mentions are settled and after any go the person gives.
- The comment goes last and names the attachments. The PR stays a link.
- The agent writes the comment under the private-repository writing rule. After a whole-word check over the comment and the attachment file names, it quietly fixes its own text.
- The person is asked once, and only when the export rewrote something.
- A failed or uncertain upload means no comment is posted. The report then names what already landed and where the exported files are.

The docs (`skills-and-agents.md` and the glossary) now describe private delivery. Repository guards pin each load-bearing sentence, and each guard was mutation-tested. The budgets and the embedded skill tree are updated. No binary code changed.

## Human actions and testing

**Important: live private-delivery check against a sandbox Jira project.**

Why it matters: the upload order, the two-step connector upload, and the no-comment-on-failure rule are agent behavior. The guards only prove the instructions say so. Without this check, nobody has seen an agent follow them against a real ticket.

When: before relying on private ticket updates for real work.

Prerequisites:
- A private scratch repository initialized with docket (it has `.git/dckt/`), with its `tickets.*` keys set in `.git/dckt/config.yml`.
- A sandbox Jira project and an Atlassian connector in a Claude Code session.
- A change in that repository linked to a sandbox ticket (`docket-link-ticket`), with a spec written.

Do not run this from the docket repository, and do not write the sandbox site or project name anywhere in it.

Steps:
1. In the scratch repository, ask: "tell the ticket the spec for #<id> is written". Expected: the spec file appears as a ticket attachment before the comment, and the comment says `Spec: attached <filename>`. Neither the comment nor the attached file contains "docket", "dckt", change ids, or board references.
2. Implement the change to an open PR, then ask for a PR-opened update. Expected: the spec and results files are attached first, the comment carries the PR link, and the comment names both attachments.
3. Force an upload failure, for example by giving the connector a file it rejects or removing its upload permission, and ask again. Expected: no comment is posted, and the report names the failed upload, any attachment that already landed, and the exported file paths.

Cleanup: delete the test attachments and comments from the sandbox ticket.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed at the build head and again after the review fixes. The budget report showed no breaches.
- Each new guard was mutation-tested: stripping or weakening each pinned sentence turned the guard red, and restoring the file turned it green.
- `docket-review-standard` reviewed the branch and returned 1 important and 2 minor findings, all fixed in-branch.
