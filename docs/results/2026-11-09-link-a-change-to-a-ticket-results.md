<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0583 — Link a change to a ticket](../changes/active/0583-link-a-change-to-a-ticket.md)**
<!-- docket:backlink:end -->

# Link a change to a ticket — results

**Human action:** None required to merge. One optional walkthrough is listed below for anyone who wants to see the new linking command work end to end.

## Outcome

A change can now carry a link to a ticket in the team's ticket system. Three new record fields, `ticket_provider`, `ticket_id`, and `ticket_url`, are written together by the new `docket change link-ticket` command (link, replace on explicit request, unlink, in any status, archived changes included) and accepted by `change.create`. A linked change shows a Ticket row in its `## Artifacts` block, and `context.implementation` exposes a typed `ticket` object that implement-next uses to add a `Ticket: [<id>](<url>)` line to the PR body. A hand-edited partial or invalid set is treated as not linked and raises a `ticket-link-invalid` warning, never a block. The per-repository `tickets.*` settings (`provider`, `site`, `project`, five `auto.*` switches, all off) are accepted and validated but have no effect yet. The unused `issue:` field is gone from every read path, `docket open --help` names the export type `ticket`, and a repository test keeps `tickets.*` out of docket's own `.docket.yml`. The decision is recorded as ADR-0161.

## Human actions and testing

- **Optional — link, replace, and unlink a ticket on a scratch change.** Prerequisites: a docket build containing this branch on `PATH`, and a throwaway docket repository (for example one made with `docket repository init` in a temporary git repo with an origin), never a real backlog. Steps: create a change; run `docket status --json` to read its `path` and `revision`; write a request file `{"change_id": <id>, "path": "<path>", "revision": "<revision>", "ticket_provider": "jira", "ticket_id": "DEMO-1", "ticket_url": "https://example.atlassian.net/browse/DEMO-1"}` and run `docket change link-ticket --request <file> --json`. Expected: disposition `linked`, and the record shows the three fields plus a `| Ticket | [DEMO-1](…) |` row. Re-run with the same file: `already-linked`, no new commit. Change the id to `DEMO-2` at the fresh revision: refused `already-linked-elsewhere`; add `"replace": true`: `replaced`. Send `{"change_id", "path", "revision", "unlink": true}`: `unlinked`, all three fields empty, no Ticket row. Cleanup: delete the scratch repository.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the build head and again after the review fixes. The budget report showed only `BUDGET WATCH` screening notes for the repovisibility and state integration scripts, which this change does not touch.
- Every build task ran focused tests with RED/GREEN, and most added a mutation probe, covering the URL host check, the non-scalar warning path, the replace guard, the context omission, the repo-only scope, the single-line check, the `.docket.yml` guard, and the prose contract. The documentation fix (finding 2) and the minor batch (findings 3–4) recorded TDD exceptions, because neither changes runtime behaviour.
- Deep-tier review (bumped from standard because the diff exceeds 1500 lines) returned 5 findings: 0 blocker, 2 important, 3 minor. All four branch findings were fixed in-branch. The fifth, the ADR, was satisfied by recording ADR-0161.

## Known issues and follow-ups

- **Skill size budgets were raised.** The convention skill went from 356/7352 to 370/7527 (lines/words) and implement-next from 8806 to 8813 words, mostly for the new ticket-link paragraph. This is confirmed and harmless. A reviewer may want the convention paragraph shortened.
- **`tickets.*` docs will need updating when code starts reading them.** The example config and reference table now say the settings have no effect yet, and no test checks that statement. The later ticket changes that add consumers should revise this wording.
