<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0587 — Update tickets automatically after each event](../../changes/active/0587-update-tickets-automatically-after-each-event.md)**
<!-- docket:backlink:end -->

# Update tickets automatically after each event — design

Change #587, groomed interactively on 2026-11-11.

## Intent

Today a change's ticket gets an update only when someone asks for one ("update CLOUD-123 now that the PR is open"). A team that wants its ticket system kept current has to remember to ask at every step.

This change makes four config switches do something. When a switch is on and the change already has a ticket, the docket workflow that just finished that step posts the update as a comment, without asking anyone. When a switch is off, nothing happens at all.

- **Before:** the PR opens; nothing reaches the ticket until a person asks.
- **After, with `tickets.auto.pr_opened: true`:** the PR opens; docket-implement-next posts the PR-opened comment to the change's ticket and its report says so in one line.

This spec stands alone: it restates every rule the build needs. Nothing has to be read from the earlier ticket-integration design.

## Words used here

- **ticket**: the item in the team's ticket system (a Jira issue, a Linear issue, …).
- **connector**: the session's own tools for that system (an MCP server or plugin the person installed and signed in to). Docket installs no connector and stores no credential.
- **event**: a point in a change's life after which docket can post an update to its ticket.
- **update**: a short comment on the ticket, with the links that go with it.
- **automatic update**: an update a docket workflow posts by itself after an event whose switch is on. The other kind is **on request**: a person asks for it. On-request updates already exist and do not change.
- **site**: one place in a workflow skill where an automatic update can be posted.

## Goals

- With a switch on and a ticket linked, the update for that event is posted by the workflow running at the event, attended or not.
- With a switch off, no workflow posts, asks, or reports anything about tickets. The only cost is one extra read-only call at each site (decision 3).
- A failed or impossible post never stops, fails, blocks, or retries anything. It costs one report line.

## Non-goals

- Creating a ticket automatically (decision 6).
- Automatic updates in a private repository (decision 7; a later change adds them).
- Any new docket operation. The existing `context.ticket` read already returns everything a site needs.
- Any change to `docket-link-ticket`, or to how on-request updates work.

## Anti-goals

- A prompt, question, or approval step for an automatic update.
- A queue, ledger, delivery record, pending-update marker, or retry of any kind.
- A new blocking gate: no automatic update may change a run's disposition, write a halt or blocked marker, or stop a step that would otherwise run.
- A master switch or an "enable all" switch.

## Decisions

### 1. Four events, four switches

| Event (`--event` token and switch name) | Fires when | Site |
|---|---|---|
| `spec_written` | a change's **first** spec lands: `change.groom` with `outcome: spec` | docket-groom-next, its Spec exit; docket-auto-groom, its Spec exit |
| `pr_opened` | the PR is open and the change is marked implemented: `change.mark-implemented` applied | docket-implement-next, Step 7 |
| `pr_merged` | a merged change is closed out: `finalize.closeout` | docket-finalize-change, after step 9 |
| `results_completed` | a results-only change completes: `change.complete` | docket-implement-next's results-artifact ending (`references/results-artifact.md`, section 6) |

That is five sites in four skills. The switches are `tickets.auto.spec_written`, `tickets.auto.pr_opened`, `tickets.auto.pr_merged`, and `tickets.auto.results_completed`. Each is a boolean, `false` by default, settable at any config layer with normal precedence. They already exist; this change gives them their effect.

Site details:

- **docket-groom-next** posts only on its Spec exit (exit 1), after the transaction lands. Its kill, defer, revise, and re-enable exits never post. The person is in the session, and the update still posts without asking: the switch is the consent.
- **docket-auto-groom** posts per stub on its Spec exit, after the transaction lands and before it selects the next stub. An abstain never posts. Report lines go in the drain's final report.
- **docket-implement-next, Step 7** posts after `change.mark-implemented` returns its applied outcome and before the run declares its disposition. The disposition is `advanced` whatever the update's outcome.
- **The results-artifact ending** posts after `change.complete` returns `completed` or `already-completed`, before the run ends `advanced`.
- **docket-finalize-change** posts after `finalize.closeout` returns `done-archived`, `stacked-merged`, `root-archived`, or `already-closed-out`, and only for the change id that closeout call was made for. See decision 5 for what it never posts.

**A replayed success posts like a first success.** `already-completed`, `already-closed-out`, and a `change.mark-implemented` retry that returns its earlier applied outcome all mean the event has happened, and the site cannot tell whether an earlier attempt got as far as posting. Docket keeps no record of what it posted, so a retried run can post the same update twice. That is accepted: a duplicate comment is deleted by hand, and a missed one would be silent.

**docket-new-change is deliberately not a site.** It creates a change and lands its spec in the same run, and nothing in that skill links a ticket, so the change has no ticket when its spec lands and a site there could never post. A change that already has a ticket when it is designed (one imported from a ticket, or linked while still a stub) is groomed by docket-groom-next or docket-auto-groom, which are sites.

### 2. Off means nothing happens

When the switch for an event is off: no post, no prompt, no question, and no line in any report. The same holds when the switch is on and the change has no ticket: a change with no ticket never gets an update, and says nothing about it.

There is no "ask before posting" mode, no handoff to a parent session, and no record that an update is owed. The artifacts are durable, so a person can always ask for the update later and it is composed from the artifacts as they stand.

### 3. What every site does

Each site does the same three things, in this order:

1. **Read.** Run the `context.ticket` operation (argv from the capability catalog) with `--id <id> --event <event> --json`.
2. **Stay silent** when `context.event.switch` is `false`, or `context.ticket.state` is `not-linked`. Do nothing more and say nothing.
3. **Otherwise invoke `docket-update-ticket`** with a call-site direction, marked with the house marker `DIRECTED to:`, that states the outcome up front: post the update for this event and this change as an automatic update; nobody is present, so anything the skill would ask is answered by posting nothing and reporting one line; then return to the workflow.

The update skill is loaded only in step 3, so a repository with the switches off, or a change with no ticket, never pays for it.

If the read in step 1 does not return `result: applied`, the site reports one line (`Ticket update for #<id> not checked: <reason>`) and carries on. It is a failure, not an "off": the site cannot know the switch's value, so it says so once. This is the one case where a repository with every switch off can see a ticket line, and it only happens when docket's own read is broken.

**Where the rule lives.** The three steps are written once, as a shared definition in `docket-convention` (suggested heading: *Automatic ticket updates*). Each site is one short paragraph that names its event, points at that definition, and carries the `DIRECTED to:` direction itself. The direction must sit at the site, not only in the convention: a direction read at the moment of invocation is what outranks an instruction inside the invoked skill.

### 4. What `docket-update-ticket` does when a workflow invokes it

`docket-update-ticket` gains an automatic path. Its rules:

- **The caller supplies the change id and the event.** Nothing is worked out from a person's words, and no one is mentioned.
- **It never asks and never waits.** No draft is shown. Wherever the on-request path would ask the person something, the automatic path posts nothing and reports one line.
- **It does its own `context.ticket` read**, as on request, so the update is composed from one pinned commit.
- **Cases that post nothing and report one line:**
  - `ticket.state` is `partial`: the ticket link is broken.
  - `event.happened` is `false`: name the missing fact from `event.reason`.
  - `visibility` is anything other than `shared` (decision 7).
  - no connector for `ticket.provider` is available in this session.
  - the post failed, or its outcome is uncertain. An uncertain outcome is reported as uncertain.
- **`ticket.state` is `not-linked`:** post nothing and say nothing (the site normally filters this out first).
- **Otherwise it posts** exactly what an on-request update for that event posts in a shared repository: the same content table, the same editorial standard, the same commit-pinned links from the shared-delivery reference.
- **It reports one line** and returns: the update as posted only with the connector's confirmation, naming the ticket and the event; or what was not posted and why.
- **It never retries, and never touches the calling run.** No halt, no blocked marker, no changed disposition, no metadata write. A duplicate comment is acceptable and is deleted by hand.

Report lines are plain, for example:

> Posted the PR-opened update to CLOUD-123.

> Not posted to CLOUD-123 (PR opened): no Jira connector is available in this session.

The skill's opening, which today says it runs "only when the person asks" and that "Nothing here runs on its own after an event", is rewritten to say it runs when a person asks or when a docket workflow invokes it after an event whose switch is on.

### 5. What never posts automatically

- **The maintenance sweep.** `maintenance.sweep`, `maintenance.preflight`, and docket-status close out changes merged by other means and post nothing. A person can ask for the merge update later.
- **Stacked children when a stack root lands.** A `root-archived` closeout posts for the root only. The carried descendants (`carried_ids`) get nothing. A stacked child gets its own `pr_merged` update when docket-finalize-change merges it into its parent's branch (`stacked-merged`); that update already says the work is not yet in the integration branch.
- **docket-implement-next's bounded closeout of a named change's merged dependencies** (`references/edge-paths.md`, *Named invocation's own merged dependencies*). It calls `finalize.closeout` as housekeeping for other changes, like the sweep, and posts nothing.
- **A closeout that did not succeed.** `contended`, `blocked`, and `unknown` post nothing and report nothing: the event has not happened in this run.
- **A person running `change.complete` by hand.** No agent is running, so nothing fires.
- **A revised spec, a blocker or a needed decision, and a killed change.** These stay on request only. Whether a revision matters is a person's judgment, most of docket's own stops are machinery a ticket reader should not see, and a kill needs a person's explanation.

### 6. Ticket creation is never automatic

A ticket is only ever created when a person asks, through `docket-link-ticket`, where they see a draft and approve it. This change adds no automatic ticket creation at any point.

This departs from the earlier ticket-integration design, which planned a fifth switch, `tickets.auto.change_created`, to create a ticket when a change is created. It was dropped at grooming, by the person's decision, for these reasons:

- A quickly captured stub is not ready to be a ticket: it normally has no acceptance criteria, and capturing one must stay fast and ask nothing.
- The moment a change becomes ticket-ready is when its design lands, not when it is created, so the switch pointed at the wrong moment.
- Creating at the right moment instead was judged too complex for what it saves, which is one sentence from the person.

**The `tickets.auto.change_created` key is removed.** It has no effect today and now never will. It has not been in a tagged release, so it is removed outright with no alias and no deprecation path: a config file that still sets it gets the normal unknown-key error (a warning from `install`).

### 7. Private repositories and unknown visibility

In a private repository nothing can be linked, so an update there needs cleaned attachments and a check that nothing leaks. That is a later change. Until then, an automatic update whose `context.ticket` read reports a `visibility` other than `shared` posts nothing and reports one line:

> Not posted to CLOUD-123 (PR opened): automatic updates are not available in a private repository.

On-request updates in a private repository are unaffected.

### 8. Who posts

The agent running the workflow at the event posts, with its own connector tools. That may be a dispatched or forked agent: docket-implement-next and docket-auto-groom both run that way, and they hold three of the five sites.

Whether a connector is reachable inside such an agent depends on the harness. ADR-0162's context says a forked skill cannot reach the session's connector tools. A probe at grooming (2026-11-11, Claude Code) found otherwise for that harness: a dispatched general-purpose agent listed the session's ticket connector tools and could load their schemas. Nothing was called, and the probe did not use docket's own agent wrappers, so the live check in *Acceptance criteria* is what settles it for them.

Where a connector is not reachable, the result is the "no connector" line in decision 4. It is never a handoff to the parent session, and no site is moved to make up for it.

## Where it lands

Skills (each edit is mirrored in the embedded tree by `go generate ./internal/assets/`):

- `skills/docket-convention/SKILL.md`: the shared definition (decision 3).
- `skills/docket-groom-next/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-implement-next/references/results-artifact.md`, `skills/docket-finalize-change/SKILL.md`: one site paragraph each (decision 1).
- `skills/docket-update-ticket/`: the rewritten opening and the automatic path (decision 4).

Go:

- Remove `tickets.auto.change_created` everywhere it appears. Derive the list from a whole-repository search for `change_created` and `ChangeCreated` rather than from this spec; at grooming it was the schema leaf, the config struct field, the resolver, the defaults, the `diagnostic.config` line, and the tests that enumerate the keys. Records already written (earlier specs and results) stay as they are.
- No other production Go changes. `context.ticket` is unchanged.

Docs and samples:

- `docs/reference/config-keys.md`: drop the `change_created` row; replace "no effect yet" on the other four with what each does; rewrite the paragraph below the table, which says nothing acts on the switches.
- `.docket.example.yml` and its embedded twin: drop `change_created` and rewrite the comment that says the switches name five events and are not acted on. If a frozen fixture copies this file, follow that fixture's own drift-guard remedy.
- `docs/reference/skills-and-agents.md` and `docs/reference/glossary.md`: the `docket-update-ticket` entries say it posts "when you ask"; add the automatic case.
- Docs describe only how docket behaves now: no mention of the removed key or of what was planned.

Size budgets in `internal/repoguard/budgets_test.go` are raised for each grown file, with this change's note, per that table's convention.

## Tests

- **Site guard (new).** Derive the sites from a scan of every Markdown file under `skills/`: a site is a line that invokes `docket-update-ticket` and names an event token. The convention's shared definition also invokes the skill but names no token; it is the only such line allowed, and the guard asserts that. Assert:
  - the set of files holding a site is exactly the five in *Where it lands*, so a site added anywhere else (docket-status, docket-new-change, docket-link-ticket) turns it red;
  - every site carries `DIRECTED to:` and names exactly one event token;
  - every token in `ticketEvents` (`internal/app/ticket_context.go`) has at least one site, and no site names a token outside it;
  - each site's token matches its file per the table in decision 1.
  Key the scan on the invocation's shape, not on a list of spellings. `ticketEvents` is unexported, so the token checks belong in an `internal/app` test, beside the existing skill test for the ticket read; no production code is exported for this.
- **Prose contracts.** Pin the convention's shared definition (the read, both silent cases, the direction) and the update skill's automatic rules (never asks, the one-line cases, private posts nothing, never retries, never touches the run). Update the existing contract rows that pin "only when the person asks" on `docket-update-ticket` and the "when you ask" doc entries in the same commit. Add an absent-clause assertion that the skill no longer says nothing runs on its own.
- **Config.** `tickets.auto.change_created` is rejected as an unknown key; the four remaining switches resolve as before.
- **Mutation-test every new guard**: remove a site, drop a `DIRECTED to:`, swap an event token, delete each pinned clause, and watch each turn red. Run with the test cache defeated.
- The whole suite runs at the build gate.
- **No ticket-system call is made from this repository**, in tests or otherwise, and this repository's `.docket.yml` still declares no `tickets` key.

## ADR

Record one ADR at build: automatic ticket updates are per-event switches where off means nothing happens, with no queue, ledger, retry, or prompt; a failure is one report line and never a gate; and ticket creation is never automatic. It relates to ADR-0044 (call-site directions) and ADR-0162 (the two inline ticket skills). It also states that the update skill now runs inside dispatched and forked agents, and that ADR-0162's remark about forked skills and connector tools does not hold in every harness (decision 8), without rewriting that ADR.

## Acceptance criteria

- With a switch on and a ticket linked, each of the five sites directs `docket-update-ticket` to post that event's update; the site guard pins all five and no others.
- With every switch off, no workflow posts, asks, or reports anything about tickets.
- With a switch on and no ticket, nothing is posted and nothing is reported.
- In a private repository an automatic update posts nothing and reports one line.
- A missing connector or failed post is one report line; the run's disposition and every later step are unchanged.
- The maintenance sweep and a stack root's descendants never get an automatic update.
- `tickets.auto.change_created` no longer exists in the schema, the sample config, or the docs.
- The results list an **Important** live check, done by a person from a shared scratch repository against a sandbox ticket project, never from this repository:
  - with each switch on in turn, run the workflow and confirm the comment arrives with working links;
  - with the switch off, and with the switch on but no ticket, confirm nothing is posted and nothing is reported;
  - confirm what a dispatched docket-implement-next and a forked docket-auto-groom do when they reach the post step: the comment, or the "no connector" line.

## Out of scope

- Automatic ticket creation, at change creation, at grooming, or anywhere else.
- Automatic updates in a private repository, and the leak check they need.
- Automatic updates for a revised spec, a blocker, a kill, or a change the sweep closed out.
- Updates to stacked descendants' tickets when a root lands.
- Mentions in automatic updates.
- Syncing ticket status, assignee, or fields; rewriting a ticket's description.
- Reordering docket-new-change's steps.

## Left to the builder

- The exact wording of the convention's shared definition and of each site paragraph, within decisions 1 to 3.
- Whether the automatic path sits in `docket-update-ticket/SKILL.md` or in a new reference file loaded only when a workflow invokes the skill. A reference file matches how the skill already splits shared and private delivery.
- Where the docket-finalize-change site sits among its cleanup, integration-sync, and rebuild steps, as long as it follows closeout and its line reaches the final report.
- The exact wording of report lines.
- The shape of the site-guard scan.

## What the code says today

Traced at main `4874f44ea`; anchors are symbols and headings, not line numbers.

- `ContextTicketRead` (`internal/app/ticket_context.go`) already returns `event.switch` (the effective `tickets.auto.<event>` value), `ticket.state`, `visibility`, `event.happened`, and commit-pinned artifact URLs. `ticketEvents` holds exactly the four tokens in decision 1; `change_created` was never one of them.
- The five `tickets.auto.*` leaves in `internal/config/schema.go` are `scopeAny`. `docs/reference/config-keys.md` describes all five as having "no effect yet".
- No workflow skill invokes `docket-update-ticket` today. `skills/docket-update-ticket/SKILL.md` opens with "only when the person asks" and "Nothing here runs on its own after an event", and a prose contract pins the first phrase.
- `finalize.closeout` returns `done-archived`, `stacked-merged`, `root-archived`, `already-closed-out`, `contended`, `blocked`, or `unknown`; `root-archived` carries `carried_ids`. Its callers are docket-finalize-change (step 9), the maintenance sweep and preflight, and docket-implement-next's bounded closeout in `references/edge-paths.md`.
- `internal/repoguard/skill_handoff_sites_test.go` guards `DIRECTED to:` on role-skill invocations only. `docket-update-ticket` is not a role skill, so it needs its own guard.
