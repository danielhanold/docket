<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0585 — Post a ticket update on request (shared repositories)](../changes/archive/2026-11-10-0585-post-a-ticket-update-on-request-shared-repositories.md)**
<!-- docket:backlink:end -->

# Post a ticket update on request (shared repositories): results

**Human action:** Needed. Before merging, run one live check from another repository linked to a sandbox ticket project (Important item below). Nothing in this repository calls a ticket system, so automated tests cannot cover the actual posting.

## Outcome

An agent can now post a short update about a change to its linked ticket when a person asks for one. Two pieces make this work:

- **`docket context ticket --id <id> [--event …] --json`** (`context.ticket`) is a new read-only operation. In one call it returns the ticket link state (`linked`, `not-linked`, `partial`), the repository visibility, and, when an event is named, whether that event has happened and why not. It also returns the PR and merge facts, and spec and results links pinned to the metadata commit the read took. Plans are never included.
- **`docket-update-ticket`** is a new inline skill. It refuses with one plain line when there is no ticket, the link is broken, the request names a different ticket, the event has not happened, or the repository is private. Otherwise it writes a short comment for a person reading the ticket, mentions named people only on an exact single match, posts through the session's own connector, and reports "posted" only when the connector confirms it.

Also in this change:

- `render.LinkContext.BlobURLAtCommit` builds the commit-pinned GitHub links.
- The skill is listed everywhere the other inline skills are.
- `docket-export` no longer suggests it can post ticket comments.
- The `tickets.provider`, `tickets.site`, and `tickets.project` rows in `docs/reference/config-keys.md` now say what uses them.

Beyond the spec: a killed change now reports the reason `killed` for the PR-merged and results-completed updates, instead of the misleading `merge-not-recorded`.

## Human actions and testing

- **Important: live check from another repository.** Docket never calls a ticket system from its own repository, so the real posting path is untested. Do this after the PR opens and before merging. If you skip it, nobody knows whether the comment layout, the pinned links, and the mention survive a real connector.
  - Prerequisites: a separate shared GitHub repository using docket built from this branch, a sandbox Jira project, and a Claude session with the Atlassian connector signed in.
  - Setup: link a test change to a sandbox ticket (`link #N to SANDBOX-1`). The change needs a spec, and ideally an open PR.
  - Steps and expected results:
    1. Ask "update SANDBOX-1, the spec for #N is written". A short comment appears with a `Spec:` link pinned to a full commit id, and the link opens on GitHub.
    2. Ask for the PR-opened update with "mention <a full name in the Jira site>". The comment starts with a real mention, that person gets a notification, and `Spec:` and `Results:` links appear when those files exist.
    3. Ask for a merge update on an unmerged change. Nothing is posted, and the agent says docket has not recorded a merge.
    4. Ask about a change with no ticket. Nothing is posted, and the agent says the change has no ticket.
    5. Ask for one no-event update (for example, a blocker). A short comment appears.
  - Cleanup: delete the test comments on the sandbox ticket by hand, and unlink or kill the test change.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the final head after the review fixes. Each task worker also ran focused tests, and the new repository guards were mutation-tested (each one fails with its guarded text removed).
- Deep review (picked because the diff was over 1500 lines) returned 0 blockers, 2 important, and 3 minor findings. All five were fixed in the branch.
- No ticket-system call was made in any build, test, or review step.

## Known issues and follow-ups

- **The guards check wording only.** The skill's rules on refusals, real mentions, and confirmation before "posted" are checked by repository guards that look for the text, not by running an agent. Whether an agent follows them in practice is confirmed only by the Important live check above. No workaround. Next action: run that check.
