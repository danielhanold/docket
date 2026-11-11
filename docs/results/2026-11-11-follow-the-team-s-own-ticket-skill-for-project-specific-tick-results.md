<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0591 — Follow a companion skill when creating a ticket](../changes/archive/2026-11-11-0591-follow-the-team-s-own-ticket-skill-for-project-specific-tick.md)**
<!-- docket:backlink:end -->

# Results: Follow a companion skill when creating a ticket

**Human action:** Needed. One Important live check from another repository with a companion skill installed; the automated suite cannot exercise a real ticket system.

## Outcome

Creating a ticket from a change now looks for a companion skill (the team's own ticket skill) and follows it for project-specific details. A new optional key, `tickets.companion_skill`, names the skill at any config layer (global, repository, or `.docket.local.yml`, normal precedence); unlike its repository-only `tickets.*` siblings it is never warned and ignored at a machine layer. `skills/docket-link-ticket/references/create.md` gains a step 4 that reads the key, warns in one line and falls back to description matching when the named skill is missing, and asks the person when several skills match. The companion skill decides the ticket type (only when it states one), the title format, field values, labels, after-creation actions, and call pacing; docket keeps the readiness check, the description body, the approved draft (which now shows everything the companion skill will do), and recording the link only after the connector confirms. The rule that a created ticket's description is never rewritten is removed. With the key unset and no matching skill, creation behaves as before.

## Human actions and testing

### Important: live companion-skill check from another repository

Why: no test can call a real ticket system or load a real companion skill; this is the only check that the agent actually follows the new step 4. If skipped, it stays unconfirmed that the title format, sprint, custom fields, labels, type, and after-creation actions come through correctly.

Prerequisites: a repository other than docket with `tickets.provider`, `tickets.site`, and `tickets.project` set in its `.docket.yml`; a working ticket connector (for example the Atlassian connector); a companion skill installed whose description names that ticket project; the `docket` binary built from this change.

Steps:
1. Set `tickets.companion_skill: <skill name>` in `~/.config/docket/config.yml`. In a session in that repository, ask the agent to create a ticket from a groomed change. Expected: the draft names the companion skill and lists the formatted title, type, sprint, custom fields, labels, and any after-creation actions before anything is created. Approve it. Expected: the ticket in the ticket system matches the draft, and the change record shows the link.
2. Remove the key and repeat with another change. Expected: the agent finds the companion skill by its description and the draft again shows its values.
3. Set the key to a name that is not installed (`tickets.companion_skill: no-such-skill`) and start a creation. Expected: one warning line saying the named skill is not available in this session, then the description-matching fallback. Decline the draft.

Cleanup: delete or close the test tickets in the ticket system, unlink the test changes (`docket-link-ticket` unlink), and remove the key from the global config.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at bd2cbdcba.
- Task 1: new `TestTicketsCompanionSkillAnyLayer` covers each layer and precedence, plus a mixed global block where `provider` is warned and dropped while `companion_skill` is kept; mutation-tested by setting the leaf back to repository-only (tests reddened, then restored).
- Task 2: new `change_0591_companion_skill` alignment contract pins the create.md text and forbids "never rewritten", "cannot be rewritten", and "written once"; five mutation probes each reddened it. The old `change_0584_link_ticket_skill` line pinning the never-rewritten sentence is removed.
- Review: docket-review-standard reviewed the whole branch and returned no findings.
- Backlog check: 21 proposed and deferred changes checked.

## Known issues and follow-ups

- **Stale sentence in `.docket.example.yml`.** The Tickets section still says "No docket command reads these settings yet, so setting them has no effect today." The ticket skills now read these keys through `diagnostic.config`, so a reader may wrongly think the keys are inert. Confirmed, cosmetic, pre-existing (not introduced by this change). Workaround: none needed. Suggested next action: fix the sentence when the tickets block is next rewritten. Fits #592 (Layer the tickets.* config keys at every normal level), which already updates the config docs for these keys; edit it through `docket-groom-next 592` (revise) to name this sentence.
