# Remembering why

By the end of this page you will know where docket keeps the two kinds of institutional memory a
project accumulates — the decisions it made and the lessons it learned — why they are kept apart,
and how a lesson earns its way from a one-off note into a rule the tools always follow.

Two things are worth remembering long after a change (one unit of planned work, roughly one pull
request, tracked as one markdown file) ships: **why** a non-obvious call was made, and **what**
the build taught you that the next build should not have to re-learn. docket keeps each in its own
place — decisions in an immutable ledger, lessons in a curated one — because they age
differently. A decision is fixed the day it is made; a lesson keeps getting refined, promoted, or
retired.

## Architecture decisions (ADRs)

An **ADR** (an architecture decision record: one file per decision, immutable once accepted)
captures the reasoning behind a non-obvious technical choice — the *why* you would otherwise
re-litigate months later, once the context has faded. Decisions are recorded separately from the
code and from the backlog, as their own ledger, so the code stays the single source of truth
about *current* state and never has to double as a record of how it got there.

Three properties make the ledger trustworthy:

- **One file per decision.** Each ADR is self-contained and cited by number, so a change or a
  documentation page can point at exactly the decision it rests on.
- **Immutable once accepted.** You do not edit an accepted ADR to change its mind — rewriting it
  would falsify the record of what was decided, and when.
- **Superseded, not overwritten.** When a later decision changes or reverses an earlier one, you
  record a *new* ADR that supersedes the old, and the old one stays in place marked as superseded.
  The trail of how the thinking moved is preserved, not erased.

Recording decisions, handling supersessions and reversals, and keeping the ADR index current is
the job of the `docket-adr` skill — a **skill** being a named, reusable instruction set an agent
loads for one job.

## The learnings ledger

The repo gets smarter as changes ship. Every change that reaches `done` distills its close-out
lessons into a curated **learnings** entry (the loop's memory of lessons from past builds, curated
by a human) — a **finding**. Zero findings from a change is normal, and abandoned (`killed`)
changes are never harvested for lessons at all.

- **Findings plus a rendered index.** Each lesson — or a consolidated family of related lessons —
  is one file under the learnings directory on the **metadata branch** (the `docket` git branch
  where the backlog, specs, and decisions are stored, separate from the code), alongside a
  generated index that lists them all.
- **Pay per relevance.** The design, planning, and review steps load only the *index* — a small
  hint surface — and pull the full text of just the findings that bear on the change at hand.
  Nobody pays to re-read the whole history on every run; the index is how a growing memory stays
  cheap to carry.
- **Controls.** One switch turns the whole subsystem off (a gate on reading and writing findings,
  never a purge — your existing findings stay on disk), and another sets the active-finding count
  past which docket flags that the ledger "needs curation", so it does not grow without bound.

The finding format and the harvest procedure themselves are owned elsewhere — the shared
convention documents the schema, and close-out is where a change's lessons are harvested — so this
page is about what the ledger is *for*, not its field layout.

## Promotion: war story or rule

A finding starts as a war story: *on this change, this bit us, and here is what we did.* Most stay
that way, pulled in by relevance when a similar change comes along. But some findings are not war
stories at all — they are **rules** that must fire *unprompted*, on every run, whether or not
anyone thought to look them up.

Those graduate. A rule that must always be in context is promoted out of the learnings ledger and
into the project's always-in-context instructions (the `AGENTS.md` / `CLAUDE.md` file the agent
loads on every run). Once promoted, the finding has done its job and stops taxing the retrieval
surface — it is now a standing rule, not a lesson to be looked up.

The test for whether a finding graduates is one question: **will the agent know to search for
this?** If it would — the lesson is discoverable exactly when it is relevant — it stays a finding,
pulled by relevance. If it would not — the agent has to already know it to avoid the mistake — it
belongs in the always-in-context file.

Promotion is **human-gated by construction**. docket *proposes* a candidate for promotion; a
human disposes. docket never edits your always-in-context file itself, and never auto-merges its
own memory into the rules it runs under — the one place a wrong entry would silently reshape every
future run is the one place a person always stands in the loop.
