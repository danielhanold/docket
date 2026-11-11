<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0587 — Update tickets automatically after each event](../changes/active/0587-update-tickets-automatically-after-each-event.md)**
<!-- docket:backlink:end -->

# Update tickets automatically after each event — results

**Human action:** Needed. One Important live check: confirm from a shared scratch repository that automatic ticket updates really post, and that they stay silent when they should. Docket's own tests cannot reach a ticket system.

## Outcome

The four `tickets.auto.*` switches now take effect: `spec_written`, `pr_opened`, `pr_merged`, and `results_completed`. When a switch is on and the change has a ticket, the workflow that just finished that step posts the update as a comment, without asking. That happens at five places:

- docket-groom-next, on its Spec exit;
- docket-auto-groom, on its Spec exit;
- docket-implement-next, Step 7;
- docket-implement-next's results-artifact ending;
- docket-finalize-change, after closeout.

When the switch is off, or the change has no ticket, nothing happens and nothing is reported. A missing connector, a failed post, or a private repository costs one report line and never changes how the run ends.

- The three steps every site follows are written once in `docket-convention`, under *Automatic ticket updates*.
- `docket-update-ticket` gains an automatic path in `references/automatic.md`. On that path it never asks anything and never runs a startup check of its own. If the switch is off it posts nothing, and if any read is refused it reports one line and returns to the workflow.
- `tickets.auto.change_created` is removed. A config file that still sets it now gets the normal unknown-key error.
- A new concepts page, `docs/concepts/tickets.md`, explains how ticket work fits together and why docket never creates a ticket by itself. Both docs indexes list it, and `config-keys.md` and the glossary point to it.
- ADR-0163 records the decision. ADR-0162 is still Accepted and now has a dated update note: in Claude Code, a dispatched agent could reach the session's connector tools.

## Human actions and testing

### Important: live check from a shared scratch repository

**Why:** No test in this repository calls a ticket system, and none can. It is also unproven whether docket's own dispatched and forked agents (docket-implement-next, docket-auto-groom) can reach the session's ticket connector. A probe at grooming used a general-purpose agent, not docket's wrappers. If you skip this check, a team that turns a switch on may find that only the "no connector" line ever appears for the dispatched workflows.

**When:** before relying on automatic updates in a real project.

**Prerequisites:**
- A separate shared (non-private) scratch repository with docket initialized. Never use the docket repository itself.
- A sandbox ticket project.
- A signed-in ticket connector (for example a Jira MCP server) in the Claude Code session.
- The `docket` binary from this branch, or from `main` after merge, installed with `docket install`.
- A fresh session after install.

**Setup:** in the scratch repository's `.docket.yml`, set `tickets.provider`, `tickets.site`, and `tickets.project` for the sandbox. Create a test change and link it to a sandbox ticket ("link #1 to SANDBOX-1").

**Steps:**
1. Set `tickets.auto.spec_written: true` and groom the change with docket-groom-next. Expected: a spec-written comment appears on SANDBOX-1 with a working spec link, and the run reports one line saying it posted.
2. Set `tickets.auto.pr_opened: true` and run docket-implement-next on the change, dispatched as normal. Expected: either a PR-opened comment with working PR and results links, or exactly the line "Not posted to SANDBOX-1 (PR opened): no … connector is available in this session". The run ends `advanced` either way. Write down which one you got.
3. Set `tickets.auto.pr_merged: true` and run docket-finalize-change after approving the PR. Expected: a merged comment.
4. For `results_completed`, create a `deliverable: results-artifact` change, link it, and run docket-implement-next on it. Expected: a results-completed comment, or the "no connector" line.
5. Mark a stub `auto_groomable: true` with `tickets.auto.spec_written: true` and run docket-auto-groom. Expected: a comment, or the "no connector" line. Note which.
6. Turn every switch off and repeat step 1 on another linked change. Then turn a switch on and run it on a change with no ticket. Expected both times: no comment, and no ticket line in the run report.

**Cleanup:** delete the sandbox comments and tickets, and delete or archive the scratch repository.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed at the build head and again after the review fixes, at 2dae6e503. Both runs were driven through the gate driver. The budget report listed one screening note (`test_go_integration_app_concurrency.sh` at 88s under -j11, overrun streak 1/5) and no breaches.
- Every new guard was mutation-tested by its build worker: the site guard (`TestTicketAutoUpdateSites`) and the prose contracts (`TestAutoTicketUpdateContracts`). For the site guard, that meant removing a site, dropping `DIRECTED to:`, swapping an event token, adding a site to docket-status (including one worded "run" rather than "invoke"), and removing the pointer to the convention. For the prose contracts, it meant deleting each pinned clause. Each mutation turned the test red.
- A whole-repository search finds `change_created` only in deliberate rejection and absence tests and in earlier records.
- The deep review found 7 issues (0 blocker, 2 important, 5 minor), and all 7 were fixed in-branch:
  - an untested switch-off guarantee at the sites, fixed by a pointer check plus an `event.switch` rule in the update skill;
  - a startup-check stop that could fire inside the calling workflow;
  - the ADR missing from the page's *Decided in*;
  - two over-absolute "nothing is reported" statements;
  - a long line;
  - a site guard that recognized only the verb "invoke";
  - the skill description.
- The backlog match below checked 19 proposed and deferred changes.

## Known issues and follow-ups

- **Two glossary entries are now slightly loose.** They are confirmed, low impact, and wording only. The glossary's "Interactive vs autonomous skills" entry says the ticket skills "post with your session's own tools". For an automatic update, the poster is now the running workflow agent. The "Ticket context" entry says only docket-update-ticket reads the bundle, but the five sites read it too. A reader could come away thinking automatic updates come from the person's session. The first sentence is pinned verbatim by an earlier prose contract, so changing it means updating that contract row as well. Suggested next action: a one-commit wording fix. No existing change fits (checked 19); this is a new change for a human to capture, related to #588.
- **Whether docket's dispatched agents can reach a ticket connector is unconfirmed.** This is the subject of the Important live check above. If they cannot, automatic updates from docket-implement-next and docket-auto-groom always report the "no connector" line and never post. Deciding what to do about that would be a new decision. Suggested next action: run the live check. Related to #588 (automatic updates in a private repository), which builds on the same path; folding it in would change what #588 is for.
