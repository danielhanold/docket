# Tickets

## The problem it solves

A team tracks its work in a ticket system (Jira, Linear, and the like), and
docket tracks it in changes. Without a connection between the two, a person
copies status across by hand: pastes the spec link into the ticket, notes
that the pull request opened, notes the merge.

Docket links a change to its ticket and keeps the ticket informed, without
owning the ticket system. It installs no connector, stores no credential,
and has no code for any one ticket system. It decides nothing about what the
team is asked to work on: a ticket exists because a person created it or
chose it.

## The moving parts

```
  you, present ──▶ docket-link-ticket ── link, create, or read ─────┐
                           │                                        │
                           │ records the link                       │
                           ▼                                        ▼
               ┌─ change record ────────┐              ┌─ ticket system ──┐
               │ ticket: system, id,    │              │   the ticket     │
               │ and web address        │              └──────────────────┘
               └───────────┬────────────┘                           ▲
                           │ read with context.ticket               │
                           ▼                                        │
  you ask, or ───▶ docket-update-ticket ──── one short comment ─────┘
  a workflow after
  an event whose
  switch is on
```

Every arrow into the ticket system goes through the **connector**: the
ticket-system tools the running agent session already has.

**The link.** A change carries at most one ticket: which ticket system it is
in, the ticket's id, and its web address. The link is stored on the change
record and shown as a Ticket row in the change's artifacts table. One ticket
may be on several changes, and docket names the others when it links one. A
change can be linked in any status, including after it is archived. A link
that was hand-edited into a broken state counts as not linked and is
reported as a warning (`ticket-link-invalid`), never a block; ticket work on
that change waits until a person fixes or removes it.

**Three ways a change gets a ticket.** All three go through
`docket-link-ticket`, which runs only when a person asks, with that person
present:

- **link an existing ticket to a change.** Docket checks through the
  connector that the ticket exists. With no connector, you can still link it
  by its web address, and docket says the ticket was not verified.
- **create a ticket from a change.** Docket first asks whether the change is
  ready to hand over, asks you about whatever is missing, and follows your
  team's companion skill, if you have one, for how the ticket is filed. It
  shows you the whole draft and creates the ticket only on your go. The spec,
  when the change has one, goes with it as the proposed approach, never as
  requirements.
- **make a change from a ticket.** The ticket is read once and becomes a new
  change: a stub that waits to be groomed like any other, with the ticket
  already linked. Nothing is synced from the ticket afterwards.

**Updates.** `docket-update-ticket` posts one short comment, written for
people who never open docket, and runs inside whichever session or workflow
agent invokes it. An update is posted in one of two ways:

- **on request**, at any time: for any of the four events below, and also
  for a revised spec, a blocker or a decision that is needed, or a killed
  change. You can name people to mention, and the comment mentions them
  in the ticket system's own way, so they are notified.
- **automatically**, by the docket workflow that just finished one of the
  four events, when that event's switch is on and the change already has a
  ticket. Nothing is asked and no draft is shown: the switch is the consent.
  An automatic update mentions no one.

**The four events.** Each event has its own switch, and each switch is off
until you turn it on.

| Event and switch | When it happens | Posted automatically by | The update says | Goes with it |
|---|---|---|---|---|
| spec written, `tickets.auto.spec_written` | the change's first spec lands | `docket-groom-next` or `docket-auto-groom`, when grooming ends in a spec | the approach and key decisions, and where the change stands now | the spec |
| PR opened, `tickets.auto.pr_opened` | the pull request is open and the change is marked implemented | `docket-implement-next` | the pull request, what was built, and what the reviewer needs to know | the pull request, the spec, and the results file |
| PR merged, `tickets.auto.pr_merged` | the merged change is closed out | `docket-finalize-change` | what merged and where, key outcomes, and anything left to do; never a claim that it was deployed or accepted | the results file |
| results completed, `tickets.auto.results_completed` | a change whose deliverable is a results file completes | `docket-implement-next`, at the end of a results-only change | findings, conclusions, limits, and any action needed | the results file |

Some things never post automatically. A change that the maintenance sweep or
a status refresh closes out gets no update, and neither does a change that
`docket-implement-next` closes out as a merged dependency of the change it
was asked to build. When a stack root lands, only the root's ticket is
updated, not its stacked children's; a stacked child gets its own merge
update when `docket-finalize-change` merges it into its parent's branch, and
that update says the work is not yet in the integration branch. A revised
spec, a blocker, a needed decision, and a killed change are updated on
request only. A change designed in the same run that creates it gets no
spec-written update, because it has no ticket yet. A run that is retried
after an event already happened may post the same update again; the
duplicate is deleted by hand.

**What goes with an update.** In a shared repository, the comment carries
links to the spec and the results file, each pinned to the commit that holds
it, so a reader sees the file as it stood when the update was written. In a
private repository those files cannot be linked. There, an on-request update
attaches cleaned copies of them instead and posts the comment only after
every attachment lands, and an automatic update posts nothing and reports one
line. The pull request link stays a link in both.

**The connector.** Docket reaches the ticket system only through the tools
the running agent session already has, such as an MCP server or plugin you
installed and signed in to. With no connector, a ticket can still be linked
by its web address, unverified; creating a ticket, making a change from one,
and posting an update cannot happen, and docket says so. An automatic update
is posted by whichever agent is running the workflow at the event, with that
agent's tools; when it has no connector, the update is not posted and the run
reports one line saying so.

**The settings.** All of them are in
[the config keys reference](../reference/config-keys.md), and each can come
from any config layer.

- `tickets.provider`, `tickets.site`, and `tickets.project` say which ticket
  system the changes link to, where it lives, and where new tickets go. When
  one that is needed is empty, `docket-link-ticket` asks you for it for that
  run.
- `tickets.companion_skill` names your team's own skill for filing tickets.
  It may decide the ticket type, the title format, the field values (sprint,
  labels, assignee, priority, and the like), what happens after the ticket
  exists, and how the calls are paced. It may not skip the readiness check,
  replace the description docket writes, create a ticket you have not seen in
  the draft, or record a link before the ticket is confirmed.
- `tickets.auto.spec_written`, `tickets.auto.pr_opened`,
  `tickets.auto.pr_merged`, and `tickets.auto.results_completed` are the four
  switches, one for each event.

**When something goes wrong.** An update that fails or cannot be posted
costs one report line, naming what was not posted and why; an outcome the
connector did not confirm is reported as uncertain. Nothing is retried,
queued, or recorded as owed, and no run stops, fails, or changes its outcome
for it. If docket's own read of the change fails where an automatic update
would be posted, the workflow reports one line saying the ticket update was
not checked, and the run carries on. You can ask for the update later, and
it is written from the files as they stand then.

## The invariants

- A ticket is created only when a person asks and approves the draft. No
  setting changes that.
- A link is recorded only for a ticket the connector confirmed, or, with no
  connector, for a web address the person gave, reported as not verified. A
  created ticket is recorded only after the connector confirms it exists.
- An automatic update posts only when its switch is on and the change
  already has a ticket. With the switch off, nothing is posted and nothing is
  reported, unless docket's own read of the change fails.
- An update is reported as posted only with the connector's confirmation.
- A failed update never stops, fails, or retries a run. A duplicate comment
  is deleted by hand.
- Beyond creating a ticket, docket only adds comments and attachments to it.
  It never changes the ticket's status, assignee, fields, or description on
  its own, and nothing flows back from the ticket into the change.
- Docket installs no connector and stores no credential.

## Why docket never creates a ticket by itself

Docket posts updates to a ticket automatically. It does not create a ticket
automatically, and no setting makes it. A ticket is created only when you
ask (`create a Jira ticket from #12`), after you have seen the draft and
said go. There are two reasons.

### Creating a ticket is a person's action

A ticket is a statement to other people. It puts a piece of work into a
team's backlog, its planning, and often its notifications, and it says: this
is real work, and this is what done looks like. Docket treats that statement
as yours to make. Creating a ticket takes judgments that an agent running by
itself has no basis for:

- **Whether the work should have a ticket at all.** Many changes never
  should: a small chore, a fix found and made in the same hour, an idea that
  is killed when it is designed.
- **Whether a ticket already exists.** The work may already be in the ticket
  system in someone else's words. The right action is then to link the
  change to that ticket, and only someone who knows the backlog can tell.
- **Whether the change is ready to hand over.** Before creating a ticket,
  docket asks one question of the change: could another engineer take
  ownership of this as a user story? When the answer is no, docket asks you
  about the gaps (most often the acceptance criteria). Docket never invents
  scope or acceptance criteria.
- **Where it goes and how it is filed.** The project, the ticket type, the
  title format, the sprint, the labels, and the required fields follow your
  team's conventions.
- **What it says.** The description is read by people who never open docket.

When you ask for a ticket, you have made the first two judgments by asking,
and `docket-link-ticket` works through the rest with you: it checks that the
change is ready and asks about what is missing, follows your team's
companion skill for where and how the ticket is filed, and shows you the
whole draft before anything is created. An agent running unattended has
nobody to show it to. Automating creation would therefore mean one of two
things: tickets that nobody read before the team saw them, or an unattended
run that stops to ask, which defeats the point of running unattended.

A ticket is also hard to take back. Once created it has a number, it may
have notified people, and deleting it may need a permission you do not have.
A wrong comment is deleted in seconds.

Automatic updates are a different kind of action, which is why docket has
them. An update reports something that has already happened (the spec was
written, the pull request opened) on a ticket you already chose to have. You
made both decisions that matter: the change has a ticket because you created
it, linked it, or made the change from it, and updates are posted because
you turned the switch on. The agent decides nothing. It reports.

That is the line docket draws: agents report on tickets, and people create
them. Automating a decision that belongs to a person is usually the wrong
tool, so docket offers no switch for it.

### There is no one right moment to create it

An automation needs a trigger: one point in a change's life at which the
ticket is created. There are two candidates, and neither works for every
change.

Take an idea you capture while working on something else: "retry uploads
after a timeout".

- **When the change is created.** You capture it as a stub in a few seconds
  and go back to your work. The stub says why and what changes. It normally
  has no acceptance criteria and no design, and capturing has to stay that
  fast, so nothing can stop to ask you for them. A ticket created now would
  be a title and two paragraphs with no definition of done. The stub may
  later be killed, deferred, folded into another change, or reshaped when it
  is designed, and the ticket would stay in the team's backlog describing
  work nobody plans to do.
- **When the design is written.** Once the change is groomed it has
  acceptance criteria and a spec, and it is much closer to ready. But this
  moment has problems of its own. A design is written in three different
  places: when you design a change as you create it, when you groom a stub,
  and when a stub is groomed unattended. In the unattended case nobody is
  there to approve a draft. Some changes already have a ticket by then,
  because they were made from one or linked earlier. And a team that plans
  its design work in the ticket system needs the ticket before this point,
  not after it.

A trigger at creation produces thin tickets for ideas that may never be
built. A trigger at design time produces unreviewed tickets from unattended
grooming, and comes too late for some teams. Supporting both would need a
rule for which changes are ready, more than one setting, and a set of special
cases, all to save you one sentence.

So the trigger is you. Say `create a Jira ticket from #12` when the change
is ready, whenever that is. From then on, in a shared repository, the
switches keep the ticket current without you.

## Decided in

- ADR-0162 — made ticket work two skills that run inline and post through
  the running agent's own connector tools: `docket-link-ticket` for linking,
  creating, and importing, always with a person present, and
  `docket-update-ticket` for updates, the only one an autonomous workflow
  loads.
- ADR-0161 — stored a change's ticket link as three flat fields (the ticket
  system, the id, and the web address), written together or not at all, with
  a broken set reported, never blocking.
- [ADR-0044](../adrs/0044-autonomy-precedence-call-site-pre-specification.md)
  — made the instruction written where a workflow invokes another skill
  decide that skill's outcome, which is how a workflow tells
  `docket-update-ticket` to post without asking.
