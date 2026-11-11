<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0587 — Update tickets automatically after each event](../../changes/archive/2026-11-11-0587-update-tickets-automatically-after-each-event.md)**
<!-- docket:backlink:end -->

# Update tickets automatically after each event — design

Change #587, groomed interactively on 2026-11-11. Revised the same day at the person's request to add decision 9, which documents why ticket creation is not automated. Revised again to add the update note on ADR-0162, and again to widen the concepts page to the whole of ticket work.

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

This departs from the earlier ticket-integration design, which planned a fifth switch, `tickets.auto.change_created`, to create a ticket when a change is created. It was dropped at grooming, by the person's decision, for two reasons:

1. **Creating a ticket is an action for a person, not for an agent.** A ticket tells a team that a piece of work is real and what done looks like. That is a person's statement to make, so automating it is usually the wrong approach. Automatic updates are different: they report something that already happened, on a ticket a person already chose to have.
2. **There is no clear place for the trigger.** At creation the change is often a quickly captured stub, which is not ready to be a ticket: it normally has no acceptance criteria, and capturing one must stay fast and ask nothing. After grooming the change has a full spec and is much closer to ready, but that moment has its own problems (decision 9 lists them). Supporting both moments was judged too complex for what it saves, which is one sentence from the person.

Decision 9 writes both reasons out in full, and requires them in docket's documentation.

**The `tickets.auto.change_created` key is removed.** It has no effect today and now never will. It has not been in a tagged release, so it is removed outright with no alias and no deprecation path: a config file that still sets it gets the normal unknown-key error (a warning from `install`).

### 7. Private repositories and unknown visibility

In a private repository nothing can be linked, so an update there needs cleaned attachments and a check that nothing leaks. That is a later change. Until then, an automatic update whose `context.ticket` read reports a `visibility` other than `shared` posts nothing and reports one line:

> Not posted to CLOUD-123 (PR opened): automatic updates are not available in a private repository.

On-request updates in a private repository are unaffected.

### 8. Who posts

The agent running the workflow at the event posts, with its own connector tools. That may be a dispatched or forked agent: docket-implement-next and docket-auto-groom both run that way, and they hold three of the five sites.

Whether a connector is reachable inside such an agent depends on the harness. ADR-0162's context says a forked skill cannot reach the session's connector tools. A probe at grooming (2026-11-11, Claude Code) found otherwise for that harness: a dispatched general-purpose agent listed the session's ticket connector tools and could load their schemas. Nothing was called, and the probe did not use docket's own agent wrappers, so the live check in *Acceptance criteria* is what settles it for them.

Where a connector is not reachable, the result is the "no connector" line in decision 4. It is never a handoff to the parent session, and no site is moved to make up for it.

### 9. A concepts page explains how ticket work fits together, and why ticket creation is not automated

Docket's user docs have no page that explains ticket work. It appears only in the reference tier: the config keys, the glossary, and the skills list. A person who wants to know what docket does with their ticket system has to piece it together from those entries, and a person who turns on the four switches will ask why there is no switch that creates the ticket.

This change adds that page. The person added it to the design in two steps after the first spec landed: first the detailed reasoning for why ticket creation is not automated, then the higher-level description of how ticket work fits together.

**Where it goes.** A new page in the concepts tier, `docs/concepts/tickets.md`. That tier is where docket explains "what each piece is and why it is shaped the way it is". The concepts tier was chosen at grooming; the person asked for "the normal docket documentation sections" and did not name a tier.

**What kind of page it is.** It is the one place that explains the whole of ticket work at the level of what the pieces are, who acts, and what docket guarantees. It describes docket as it behaves once this change is built, including the parts earlier changes built (linking, creating, importing, on-request updates). It is not a walkthrough with steps, and it does not repeat the reference tier: it names each setting in a line and links to `config-keys.md` for the rest.

**Source of truth.** The lists below say what the page covers, not what is true. The builder writes every statement from the skills and the binary as they stand at build time (`skills/docket-link-ticket/`, `skills/docket-update-ticket/`, the convention's *Ticket link* definition, and the `context.ticket` read), and where one of them disagrees with a line below, the skill or the binary wins.

**The page's sections.** The four every concepts page has, plus one extra:

*The problem it solves.* A team tracks work in a ticket system and docket tracks it in changes. Without a connection, a person copies status across by hand: pastes the spec link, notes that the PR opened, notes the merge. Docket links the two and keeps the ticket informed without owning the ticket system: it installs no connector, stores no credential, has no code for any one ticket system, and decides nothing about what the team is asked to work on.

*The moving parts.* Each of these gets a short paragraph or a bullet, and the page opens the section with a diagram, as the other concepts pages do:

- **The link.** A change carries at most one ticket: the ticket system, the ticket's id, and its web address, stored on the change record and shown in the change's artifacts table. One ticket may be on several changes. A change can be linked in any status, including after it is archived. A broken link is reported as a finding and never blocks anything.
- **Three ways a change gets a ticket**, all through `docket-link-ticket` and all with the person present:
  - link an existing ticket to a change;
  - create a ticket from a change: docket checks that the change is ready to hand over, asks about what is missing, shows the whole draft, and creates on the person's go;
  - make a change from a ticket: the ticket is read once and becomes a stub to groom; nothing is synced from the ticket afterwards.
- **Updates.** `docket-update-ticket` posts one short comment written for people. It is posted in one of two ways:
  - on request, at any time, for any of the four events below and also for a revised spec, a blocker or a needed decision, or a killed change; the person can name people to mention;
  - automatically, after one of the four events, when that event's switch is on and the change already has a ticket.
- **The four events.** One table: the event, when it happens, which workflow posts the automatic update, what the update says, and what goes with it (the spec, the PR, the results file). The page also says what never posts automatically, in plain words: a change the maintenance sweep closed out, the stacked children of a root that lands, and anything that is not one of the four events.
- **What goes with an update.** In a shared repository, links pinned to the commit that holds each file, so a reader sees the file as it stood when the update was written. In a private repository nothing can be linked: an on-request update attaches cleaned copies instead, and an automatic update posts nothing and reports one line.
- **The connector.** Docket reaches the ticket system only through the tools the running agent session already has. With no connector, a ticket can still be linked by its address, unverified; creating, importing, and updating cannot happen and say so.
- **The settings.** One line each for where tickets go (`tickets.provider`, `tickets.site`, `tickets.project`), the team's own ticket skill (`tickets.companion_skill`) and what it may and may not decide, and the four `tickets.auto.*` switches, with a link to `config-keys.md`.
- **When something goes wrong.** An update that fails or cannot be posted costs one report line. Nothing is retried, queued, or recorded as owed, and no run stops for it. The person can ask for the update later, and it is written from the files as they stand then.

*The invariants.*

- A ticket is created only when a person asks and approves the draft. No setting changes that.
- A link is recorded only after the connector confirms the ticket.
- An automatic update posts only when its switch is on and the change already has a ticket. With the switch off, nothing happens and nothing is reported.
- An update is reported as posted only with the connector's confirmation.
- A failed update never stops, fails, or retries a run. A duplicate comment is deleted by hand.
- After a ticket exists, docket only adds comments and attachments to it. It never changes the ticket's status, assignee, fields, or description, and nothing flows back from the ticket into the change.
- Docket installs no connector and stores no credential.

*Why docket never creates a ticket by itself.* The extra section, drafted below.

*Decided in.* The ADR this change records (see *ADR*), ADR-0162, ADR-0161, and ADR-0044, cited the way the docs already cite an ADR whose file is not on the integration branch.

**The extra section, in draft.** The builder may reword it and fit it to the page, but every point below must survive, including the worked example. It is deliberately long: the person asked for very detailed documentation. It is the only part of the page drafted here; the other sections are written at build from the sources above.

> ## Why docket never creates a ticket by itself
>
> Docket posts updates to a ticket automatically. It does not create a ticket automatically, and no setting makes it. A ticket is created only when you ask ("create a Jira ticket from #12"), after you have seen the draft and said go. There are two reasons.
>
> ### Creating a ticket is a person's action
>
> A ticket is a statement to other people. It puts a piece of work into a team's backlog, its planning, and often its notifications, and it says: this is real work, and this is what done looks like. Docket treats that statement as yours to make. Creating a ticket takes judgments that an agent running by itself has no basis for:
>
> - **Whether the work should have a ticket at all.** Many changes never should: a small chore, a fix found and made in the same hour, an idea that is killed when it is designed.
> - **Whether a ticket already exists.** The work may already be in the ticket system in someone else's words. The right action is then to link the change to that ticket, and only someone who knows the backlog can tell.
> - **Whether the change is ready to hand over.** Before creating a ticket, docket asks one question of the change: could another engineer take ownership of this as a user story? When the answer is no, the gaps (most often acceptance criteria) are filled by asking you. Docket never invents scope or acceptance criteria.
> - **Where it goes and how it is filed.** The project, the ticket type, the title format, the sprint, the labels, and the required fields follow your team's conventions.
> - **What it says.** The description is read by people who never open docket.
>
> When you ask for a ticket, `docket-link-ticket` works through each of these with you and shows you the whole draft before anything is created. An agent running unattended has nobody to show it to. Automating creation would therefore mean one of two things: tickets that nobody read before the team saw them, or an unattended run that stops to ask, which defeats the point of running unattended.
>
> A ticket is also hard to take back. Once created it has a number, it may have notified people, and deleting it may need a permission you do not have. A wrong comment is deleted in seconds.
>
> Automatic updates are a different kind of action, which is why docket has them. An update reports something that has already happened (the spec was written, the pull request opened) on a ticket you already chose to have. You made both decisions that matter: the change has a ticket because you created or linked it, and updates are posted because you turned the switch on. The agent decides nothing. It reports.
>
> That is the line docket draws: agents report on tickets, and people create them. Automating a decision that belongs to a person is usually the wrong tool, so docket offers no switch for it.
>
> ### There is no one right moment to create it
>
> An automation needs a trigger: one point in a change's life at which the ticket is created. There are two candidates, and neither works for every change.
>
> Take an idea you capture while working on something else: "retry uploads after a timeout".
>
> - **When the change is created.** You capture it as a stub in a few seconds and go back to your work. The stub says why and what changes. It normally has no acceptance criteria and no design, and capturing has to stay that fast, so nothing can stop to ask you for them. A ticket created now would be a title and two paragraphs with no definition of done. The stub may later be killed, deferred, folded into another change, or reshaped when it is designed, and the ticket would stay in the team's backlog describing work nobody plans to do.
> - **When the design is written.** Once the change is groomed it has acceptance criteria and a spec, and it is much closer to ready. But this moment has problems of its own. A design is written in three different places: when you design a change as you create it, when you groom a stub, and when a stub is groomed unattended. In the unattended case nobody is there to approve a draft. Some changes already have a ticket by then, because they were made from one or linked earlier. And a team that plans its design work in the ticket system needs the ticket before this point, not after it.
>
> A trigger at creation produces thin tickets for ideas that may never be built. A trigger at design time produces unreviewed tickets from unattended grooming, and comes too late for some teams. Supporting both would need a rule for which changes are ready, more than one setting, and a set of special cases, all to save you one sentence.
>
> So the trigger is you. Say "create a Jira ticket from #12" when the change is ready, whenever that is. From then on the switches keep the ticket current without you.

**Pointers to the page.** A reader meets the question in two places, and each gets one sentence that links to the extra section:

- `docs/reference/config-keys.md`, in the paragraph under the table that describes the `tickets` settings: creating a ticket has no switch and is always on request.
- `docs/reference/glossary.md`, in the `docket-link-ticket` entry.

The page is listed in both indexes, `docs/README.md` and `docs/concepts/README.md`.

**Rules for the page.**

- It states docket's standing position and the reasons for it. It does not name the removed `tickets.auto.change_created` key, and it does not say a switch was once planned or dropped.
- It cites no change and no pull request. ADR citations are allowed.
- It describes private repositories only as docket behaves when this change is built: an automatic update there posts nothing and reports one line.
- It makes no ticket-system claim that is not true of ticket systems in general; "may need a permission" is the level of claim wanted, not a statement about one product.

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
- `docs/concepts/tickets.md` (new): the concepts page for ticket work, its two index entries, and the two pointers to it (decision 9).
- Docs describe only how docket behaves now: no mention of the removed key or of what was planned. The new page explains why ticket creation is always on request as a standing position, not as history.

Size budgets in `internal/repoguard/budgets_test.go` are raised for each grown file, with this change's note, per that table's convention.

## Tests

- **Site guard (new).** Derive the sites from a scan of every Markdown file under `skills/`: a site is a line that invokes `docket-update-ticket` and names an event token. The convention's shared definition also invokes the skill but names no token; it is the only such line allowed, and the guard asserts that. Assert:
  - the set of files holding a site is exactly the five in *Where it lands*, so a site added anywhere else (docket-status, docket-new-change, docket-link-ticket) turns it red;
  - every site carries `DIRECTED to:` and names exactly one event token;
  - every token in `ticketEvents` (`internal/app/ticket_context.go`) has at least one site, and no site names a token outside it;
  - each site's token matches its file per the table in decision 1.
  Key the scan on the invocation's shape, not on a list of spellings. `ticketEvents` is unexported, so the token checks belong in an `internal/app` test, beside the existing skill test for the ticket read; no production code is exported for this.
- **Prose contracts.** Pin the convention's shared definition (the read, both silent cases, the direction) and the update skill's automatic rules (never asks, the one-line cases, private posts nothing, never retries, never touches the run). Update the existing contract rows that pin "only when the person asks" on `docket-update-ticket` and the "when you ask" doc entries in the same commit. Add an absent-clause assertion that the skill no longer says nothing runs on its own.
- **The concepts page.** Add prose-contract rows for `docs/concepts/tickets.md`: one pinned clause for each of the two reasons (a person's action; no one right moment), one for the invariant that no setting creates a ticket, one for each of the three ways a change gets a ticket, one for each of the four event tokens, and the page's line in each index. These rows keep the page from losing a part; they cannot prove the page is accurate, so the builder checks each statement against the skills by reading, and the review does the same. Pin the two pointer sentences in `config-keys.md` and the glossary. The living-docs alignment guard already scans `docs/concepts`, so a change or PR citation on the page turns it red with no new test.
- **Config.** `tickets.auto.change_created` is rejected as an unknown key; the four remaining switches resolve as before.
- **Mutation-test every new guard**: remove a site, drop a `DIRECTED to:`, swap an event token, delete each pinned clause, and watch each turn red. Run with the test cache defeated.
- The whole suite runs at the build gate.
- **No ticket-system call is made from this repository**, in tests or otherwise, and this repository's `.docket.yml` still declares no `tickets` key.

## ADR

Record one ADR at build: automatic ticket updates are per-event switches where off means nothing happens, with no queue, ledger, retry, or prompt; a failure is one report line and never a gate; and ticket creation is never automatic. It gives both reasons for that last point as decision 6 states them (creating a ticket is a person's action; there is no clear place for the trigger, stub or groomed change), and names the stub-or-groomed-change alternatives it rejected. It relates to ADR-0044 (call-site directions) and ADR-0162 (the two inline ticket skills). It also states that the update skill now runs inside dispatched and forked agents.

**ADR-0162 is not superseded or reversed.** Its decision stands and this change relies on it: two skills with no wrapper of their own, posting through the running agent's connector tools, with autonomous workflows loading only the update skill. What does not hold is half of one sentence in its Context, "A forked skill cannot reach the session's connector tools or ask the person", which its rejected alternative "A forked or agent-wrapped skill" repeats. The "cannot ask the person" half is still true, and by itself still rules out forking `docket-link-ticket`.

So the build also appends a dated `## Update` note to ADR-0162, using docket-adr's *Update note* action (the one for a decision that stands while its surrounding facts changed). The note says only what was observed, so the later live check cannot make it false:

- on 2026-11-11, in Claude Code, a dispatched general-purpose agent listed the session's ticket connector tools and loaded their schemas; nothing was called, and docket's own agent wrappers were not tried;
- so "cannot reach the session's connector tools" does not hold in every harness, while "cannot ask the person" does;
- the decision is unchanged, and the update skill now runs inside dispatched and forked workflow agents (name the new ADR).

No separate change is needed to revisit ADR-0162. If the live check in *Acceptance criteria* finds that docket's own wrappers cannot reach the connector, the results say so under Known issues, and what to do about it is a new decision made then.

## Acceptance criteria

- With a switch on and a ticket linked, each of the five sites directs `docket-update-ticket` to post that event's update; the site guard pins all five and no others.
- With every switch off, no workflow posts, asks, or reports anything about tickets.
- With a switch on and no ticket, nothing is posted and nothing is reported.
- In a private repository an automatic update posts nothing and reports one line.
- A missing connector or failed post is one report line; the run's disposition and every later step are unchanged.
- The maintenance sweep and a stack root's descendants never get an automatic update.
- `tickets.auto.change_created` no longer exists in the schema, the sample config, or the docs.
- ADR-0162 keeps its `Accepted` status and carries a dated update note with the connector finding; the new ADR relates to it.
- `docs/concepts/tickets.md` exists and is listed in both docs indexes. It explains how ticket work fits together (the link, the three ways a change gets a ticket, on-request and automatic updates, the four events, shared and private delivery, the connector, the settings, and what happens on failure), and every statement on it matches the skills and the binary as built. It also explains in detail why docket never creates a ticket by itself: creating a ticket is a person's action, and there is no one right moment to create it (a stub is too early; a groomed change has its own problems). It keeps every point of the draft in decision 9, and `config-keys.md` and the glossary each point to it.
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
- A how-to guide page for ticket work. The new concepts page explains what the ticket pieces are, how they fit together, and why; it is not a walkthrough.
- Changing how linking, creating, importing, or on-request updates behave. The page describes them as they are.

## Left to the builder

- The exact wording of the convention's shared definition and of each site paragraph, within decisions 1 to 3.
- Whether the automatic path sits in `docket-update-ticket/SKILL.md` or in a new reference file loaded only when a workflow invokes the skill. A reference file matches how the skill already splits shared and private delivery.
- Where the docket-finalize-change site sits among its cleanup, integration-sync, and rebuild steps, as long as it follows closeout and its line reaches the final report.
- The exact wording of report lines.
- The concepts page's title, its wording, the shape of its diagram, and the order of its moving parts, as long as every part in decision 9 is covered and every point of the drafted section survives. Which clauses the prose contracts pin.
- The shape of the site-guard scan.

## What the code says today

Traced at main `4874f44ea`; anchors are symbols and headings, not line numbers.

- `ContextTicketRead` (`internal/app/ticket_context.go`) already returns `event.switch` (the effective `tickets.auto.<event>` value), `ticket.state`, `visibility`, `event.happened`, and commit-pinned artifact URLs. `ticketEvents` holds exactly the four tokens in decision 1; `change_created` was never one of them.
- The five `tickets.auto.*` leaves in `internal/config/schema.go` are `scopeAny`. `docs/reference/config-keys.md` describes all five as having "no effect yet".
- No workflow skill invokes `docket-update-ticket` today. `skills/docket-update-ticket/SKILL.md` opens with "only when the person asks" and "Nothing here runs on its own after an event", and a prose contract pins the first phrase.
- `finalize.closeout` returns `done-archived`, `stacked-merged`, `root-archived`, `already-closed-out`, `contended`, `blocked`, or `unknown`; `root-archived` carries `carried_ids`. Its callers are docket-finalize-change (step 9), the maintenance sweep and preflight, and docket-implement-next's bounded closeout in `references/edge-paths.md`.
- The user docs have no page about tickets outside the reference tier: `docs/concepts/` holds nine pages, none on tickets, and `docs/guide/` has no ticket page. Every concepts page has the four sections *The problem it solves*, *The moving parts*, *The invariants*, and *Decided in*; one page adds a fifth section of its own. `TestLivingDocsAlignment` (`internal/repoguard/docs_alignment_test.go`) scans `docs/concepts` for change and PR citations and allows ADR citations. No guard checks that a concepts page is indexed. The ADR files on the integration branch stop well short of ADR-0162, so the docs cite later ADRs by number without a relative link.
- `internal/repoguard/skill_handoff_sites_test.go` guards `DIRECTED to:` on role-skill invocations only. `docket-update-ticket` is not a role skill, so it needs its own guard.
