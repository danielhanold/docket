<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0584 — Create a ticket from a change, or a change from a ticket](../changes/active/0584-create-a-ticket-from-a-change-or-a-change-from-a-ticket.md)**
<!-- docket:backlink:end -->

# Create a ticket from a change, or a change from a ticket — Results

**Human action:** Needed before merge. Run one live check from another repository against a sandbox ticket project. The new skill's connector behavior exists only as skill text, and no test in this repository can exercise it.

## Outcome

An engineer can now ask an agent to link a docket change to an existing ticket, create a ticket from a change, or create a new proposed change from a ticket. All three run through a new inline skill, `docket-link-ticket`, always with the person present:

- **Link** records an existing ticket on a change. It replaces or removes a link only when the person asks.
- **Create (push)** first checks whether another engineer could own the change as a user story. Anything missing is asked about, never invented, and the answers are written back while the change is still proposed. The agent then shows one draft: title, project, mapped ticket type, the exported body, and the spec as the proposed approach (a GitHub link in a shared repository, a cleaned attachment in a private one or when no link exists). The ticket is created only on the person's go, and the link is recorded only after the ticket system confirms the ticket exists.
- **Import** turns a ticket into a new proposed change. Its type is mapped and the ticket fields are set at creation.

`docket status --json --records` now reports each linked change's ticket (`provider`, `id`, `url`) on its records row, for active and archived changes alike, so the skill can see existing links. The `changes` rows are unchanged. `docket-export` no longer advertises ticket work and points it to the new skill. The convention, the agent-layer reference, and the reference docs list `docket-link-ticket` as an inline skill. ADR-0162 records the decision to split ticket work into two inline skills.

Wording that departs from the spec: the skill's description says "link a change in docket to a ticket" (the literal form tripped an existing CLI-shape guard), and example change references such as `#12` are written in code spans.

## Human actions and testing

### Important — live check against a sandbox ticket project

The skill's connector steps (finding the connector, the type lookup, creation, the two-step attachment upload, and recording the link after confirmation) are skill text only. Docket's tests never call a ticket system, so nothing automated shows they work. Run this after the PR opens and before merge. If you skip it, a wrong connector step only shows up the first time someone uses the skill for real.

Prerequisites: a sandbox Jira project (or another provider) with a connector installed and authenticated in your agent session; the `docket` binary built from this branch (`docket development install --source <this worktree>`); two scratch repositories, one shared (with `tickets.provider`, `tickets.site`, and `tickets.project` in its `.docket.yml` on the default branch) and one private (with the same keys in `.git/dckt/config.yml`).

1. In the shared repository, create a groomed change with a spec, then ask "create a Jira ticket from #<id>".
   Expected: one draft showing the title, project, mapped type, body, and a `Proposed approach:` link to the spec on GitHub. After your go, a ticket exists, and `docket status --json --records` shows its `ticket` on the change.
2. In the private repository, do the same with a change that has a spec.
   Expected: the draft lists every before → after rewrite. The created ticket carries the cleaned spec as an attachment, and the body names it.
3. Ask to push a stub with thin acceptance criteria.
   Expected: the agent asks targeted questions and writes your answers into the change before drafting. It invents nothing.
4. Ask to push the change from step 1 again.
   Expected: refusal naming the existing ticket. Nothing is created.
5. Ask "link #<other id> to <existing ticket URL>".
   Expected: the link is recorded. Asking again with a different ticket asks before replacing.
6. Ask "make a change from <ticket key>".
   Expected: a draft change, then a new proposed change whose ticket fields are set.

Cleanup: delete the sandbox tickets and the scratch repositories.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed at the final head, through the build gate, after the review fixes.
- Go tests cover the `ticket` on records rows: linked, unlinked, partial, invalid, archived, and an active change the type filter drops from `changes` but keeps in `records`. A schema test covers the descriptor.
- Repository guards cover the skill's inline shape, its key rules (prose sentinels, each mutation-tested by its worker), the `docket-export` description change, and budget rows for the new files.
- Review ran at the standard tier and found three minor issues, all fixed in-branch: a missing type-filter test case, the id-only link case, and resolving the `open` operation from the capability catalog.
- The suite's budget report printed only `BUDGET WATCH` screening notes, for integration tests this change did not touch, and no breach.

## Known issues and follow-ups

- **`docket-export` still shows a ticket-comment example.** Its file-request guidance uses "add the spec for 9 as a comment on PROJ-123" as an example. Its description no longer claims ticket work, but this example could still lead an agent to post a ticket comment through export instead of the coming update skill. The impact is small: export only writes a file and never posts. Confirmed by reading the file. Suggested next action: replace the example when the update skill lands. Fits #585 (posting ticket updates on request, which owns ticket comments). It is a needs-grooming proposed change, so edit it through `docket-groom-next 585` (`change.groom` revise); it stays needs-grooming.
