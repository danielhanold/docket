# Follow a companion skill for project-specific ticket details — design

Change 0591 — Follow a companion skill when creating a ticket

## Intent

Many people who use a ticket system already have an agent skill for their ticket project. It might come from their company, their team, or themselves. This design calls it a **companion skill**. It knows things docket doesn't: the title format, which sprint new tickets go into, which custom fields must be set and their ids, the default labels, the ticket type the project uses, and limits such as making one ticket-system call at a time.

Docket's create path (`docket-link-ticket`, `references/create.md`) knows none of this. Today a ticket docket creates lands with the wrong title format, no sprint, no custom fields, and none of the default labels, and someone fixes it by hand. This change makes the create path find the companion skill and follow it for those details. Docket keeps everything it owns today.

## Goals

- A ticket docket creates gets the project's title format, sprint, required and default field values, default labels, and ticket type, without fixing them by hand afterwards.
- The person names the companion skill once, in config, at whatever layer fits: global for a skill installed for the user, or the repository for a per-project choice.
- With no config and no matching skill, creating a ticket works exactly as it does today.

## Non-goals

- Linking an existing ticket, importing a change from a ticket, and `docket-update-ticket` (comments) do not use the companion skill in this change. The key's name is generic, so a later change can extend it to updates without renaming anything.
- Naming, shipping, or describing any particular companion skill or its values. Docket ships no provider- or team-specific metadata.
- Syncing sprint, assignee, status, or any other field after creation.
- Provider API code or credentials in docket. Any ticket-system call from this repository.
- Automatic creation with nobody present (`tickets.auto.change_created`, change 587). That change decides how the companion skill behaves when no one can be asked.

## Anti-goals

- A companion skill that overrides docket's own contract: the readiness check, the description body, the person's approval of the draft, or recording only after the connector confirms.
- A ticket-system action the person never saw in the draft, whether it sets a field or runs after creation.
- A missing or misconfigured companion skill that stops ticket creation. It is a warning and a fallback, never a new blocking gate.

## Decisions

### 1. The config key `tickets.companion_skill`

A new optional string key, `tickets.companion_skill`, default `""` (unset). It names a skill by the name the session's skill list shows.

Unlike its `tickets.*` siblings, which are repository-only, this key is **any layer** (`scopeAny`): global user config, repo `.docket.yml`, and `.docket.local.yml`, with the more local one winning per the normal precedence. The difference is deliberate. A companion skill is usually installed for the user, so it shows up in every repository, and the global config is where a user-wide default belongs. The siblings describe the repository itself (which system, which site, which project). This key describes the person's tooling, so it gets normal layering.

- Schema leaf: `kind: kindString`, `merge: mergeScalar`, `scope: scopeAny`, `disp: dispSupported`, `validate: singleLineStringLeaf()`, beside the other `tickets.*` leaves in `internal/config/schema.go`.
- `Tickets` struct gains `CompanionSkill Value[string] \`json:"companion_skill"\``, resolved in `internal/config/resolve.go` like its siblings, so `diagnostic.config` reports it at `effective.tickets.companion_skill.value`.
- The schema comment above the ticket leaves, and the `Tickets` struct comment ("Every leaf is repository-only"), are corrected to name the one any-layer leaf and why.
- `docs/reference/config-keys.md` gains a row: `tickets.companion_skill` | `""` (unset) | any layer | the skill to follow for project-specific details when creating a ticket (title format, sprint, fields, labels, type).
- Docket's own `.docket.yml` still declares no `tickets` block (`TestDocketYMLDeclaresNoTicketsKey` is unchanged).

### 2. Finding the companion skill (create path only)

Before its first ticket-system call, the create path reads `tickets.companion_skill` from `diagnostic.config` at `effective.tickets.companion_skill.value`, the same way it reads `tickets.project`.

- **Key set and the skill is available in the session:** load it and follow it (decisions 3 and 4). Docket does not check whether the skill covers this project. The key is the person's explicit choice.
- **Key set but the skill is not available:** say so in one line (`tickets.companion_skill names <name>, which isn't available in this session`), then fall through to matching by description.
- **Key unset, or fell through:** look through the skills available in the session for one whose description says it handles this ticket project (`tickets.project`, or the project the person gave for this run). The companion skill's own description decides which projects it covers, one or several. Docket never assumes a one-to-one pairing.
  - One match: load it and follow it.
  - Several: ask the person which one (the create path always has a person present).
  - None: create the ticket exactly as today.

Docket never names a particular skill, and never records which one was used anywhere except the draft and the report.

### 3. What the companion skill decides

When a companion skill is in use, it wins on:

- **Ticket type, only when the companion skill states one.** If the companion skill itself prescribes a ticket type (one fixed type, or its own mapping from the change), docket uses it. Otherwise docket chooses the type with its current logic (`fix` → bug, `feat` → story or feature, anything else → task, asking the person when nothing matches). Docket never infers a type from the companion skill's examples or defaults for other things.
- **Title format.** When it prescribes a format, docket fills the format's summary part with the change's title (the export's H1). Docket still supplies the words. The companion skill supplies the shape.
- **Required and default field values**, such as the current sprint, a team field, or other custom fields, with the field ids and any lookups the skill says to make through the connector.
- **Default labels.**
- **Any other field on the new ticket**: assignee, priority, components, fix version, an epic or parent link, story points, acceptance criteria copied into a custom field, and the like.
- **What it does after the ticket exists**: moving it to a status, adding watchers, posting a comment, or any other follow-up the companion skill prescribes for a new ticket. Docket lets it.
- **How to make the calls**: pacing (for example one ticket-system call at a time), and any call sequence it prescribes, including setting a field with an edit after creation when the provider can't take it in the create call.

### 4. What docket always decides

Docket's rules stand, with or without a companion skill:

- the already-linked and broken-link stops;
- the readiness check and refinement;
- **the description body**: docket's export of the change, plus the spec line or attachment, written once and never rewritten. A companion skill's own description template does not replace it.
- the draft the person approves before anything is created;
- recording the link only after the connector confirms the ticket, and never creating a second ticket.

When a companion skill's instruction conflicts with one of these, docket's rule stands and the agent says so in one line in the draft (`<skill> asks for <X>; docket keeps <Y>`). For example, an after-creation step that would replace the description is not run.

### 5. The draft shows everything the companion skill will do

The draft (create.md step 7) adds what comes from the companion skill: its name, the formatted title, each field it sets (type, sprint, custom fields, labels, and any other field), and each action it will take after the ticket exists. Nothing from the companion skill reaches the ticket without the person seeing it in the draft first. The report (step 11) names the companion skill used, if any.

### 6. Order of calls once the person says go

1. Create the ticket (step 8), with every companion field the create call accepts.
2. Apply the companion's remaining fields with edits, then its after-creation actions, in the order and pace it prescribes.
3. Attach the exported spec, when step 6 chose an attachment (step 9).
4. Record the link (step 10).

Once the connector confirms the ticket exists, a later failure in steps 2 or 3 never stops the record. The ticket stays, and the link is still recorded. The report names each field or action that didn't land, with what the person needs to finish it by hand. This is the same treatment a failed attachment gets today. There is still no retry loop and never a second ticket.

### 7. Ticket skills may edit the ticket they work on

The rule that a created ticket's description is written once and never rewritten is removed. Editing the ticket is allowed when the work requires it: setting fields after creation, a companion skill's follow-up actions, or another edit the person asks for. Three places in `create.md` change:

- **Step 6** keeps the branch link (not one pinned to a commit), with a new reason: a branch link stays current after a later spec revision. The "the description is written once" clause goes.
- **Step 8** drops "The description is written once and never rewritten." The `change_0584_link_ticket_skill` contract line pinning it (`internal/repoguard/prose_contracts_test.go`) is removed in the same commit, with no replacement that would forbid edits.
- **Step 9** keeps "the ticket stays" on a failed upload and drops "since its description cannot be rewritten".

Nothing here adds syncing. Docket still never updates a ticket on its own after creation (out of scope above). `docket-update-ticket` has no such rule and is untouched. Its private delivery's "never delete from a ticket" (leaving already-landed attachments in place after a failed upload) concerns deleting, not editing, and stays.

## Where it lands

- `skills/docket-link-ticket/references/create.md`: a step before the ticket-type step that finds the companion skill (decision 2). The ticket-type step defers to it (decision 3). The draft and create steps carry the companion values and actions (decisions 3, 5, and 6). One sentence lists what docket always keeps (decision 4). The never-rewritten wording is removed (decision 7). The existing step text pinned by the `change_0584_link_ticket_skill` contracts stays; where wording must change, update the contract in the same commit.
- `skills/docket-link-ticket/SKILL.md`: only if the shared *Where tickets go* rule needs a pointer. The companion rules live in `create.md`, since only creation uses them.
- The config schema, struct, resolver, and `docs/reference/config-keys.md` (decision 1).
- The embedded skill tree is regenerated (`go generate ./internal/assets/`), and the `docket-link-ticket/references/create.md` line/word budget in `internal/repoguard/budgets_test.go` is raised with a change-tagged note, per that table's convention.

## Tests

- A new prose contract (sentinel for this change) in `internal/repoguard/prose_contracts_test.go` on `create.md` pins: when to look (before the first ticket-system call); the key read path; the missing-skill warning and fallback; description matching with the one, several, and none outcomes; what the companion skill decides; that docket keeps the description body, the readiness check, the approval, and record-after-confirm; that the draft shows companion-set values and after-creation actions; and that a failed field edit or action after creation still records the link and reports what didn't land.
- The `change_0584_link_ticket_skill` contract no longer pins "The description is written once and never rewritten."; an absent-clause assertion pins that `create.md` carries no "never rewritten" / "cannot be rewritten" wording. It is mutation-tested: deleting each pinned clause turns it red.
- Config tests: `tickets.companion_skill` resolves from the global layer, the repo layer, and the local layer with normal precedence, and is not warned-and-ignored at a machine layer the way its siblings are. `diagnostic.config` reports it at the documented path.
- The whole suite runs at the build gate. With the key unset, no existing test's expectations change.

## Acceptance criteria

- `create.md` says when to look for a companion skill, how it is found, what to take from it, and which docket rules always win; the prose contract pins that text.
- `tickets.companion_skill` is accepted at every config layer with normal precedence and documented in the config-keys reference.
- With the key unset and no matching skill, creating a ticket behaves as before, except that the never-rewritten wording is gone.
- The results list an **Important** live check from another repository with a companion skill installed: create a ticket from a change, once with the key set and once relying on description matching. Confirm the title format, sprint, custom fields, labels, type, and any after-creation actions match the companion skill, and that the draft showed them before creation. Also confirm the missing-skill warning appears when the key names a skill that isn't installed.

## Left to the builder

- Exact step numbering and wording in `create.md`, within the decisions above.
- Whether the description-matching rule names the provider and site as well as the project when judging a match.
