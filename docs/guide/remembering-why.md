# ADRs and learnings: Remembering why

By the end of this page you will know where docket keeps the two kinds of institutional memory a
project accumulates — the decisions it made and the lessons it learned — why they are kept apart,
and how you turn a lesson into a rule the tools always follow.

Two things are worth remembering long after a change (one unit of planned work, roughly one pull
request, tracked as one markdown file) ships: **why** a non-obvious call was made, and **what**
the build taught you that the next build should not have to re-learn. docket keeps each in its own
place — decisions in an immutable ledger, lessons in a curated one — because they age
differently. A decision is fixed the day it is made; a lesson keeps getting refined or retired.

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

The repo gets smarter as changes ship — when someone writes the lesson down. Each lesson in the
ledger is a **finding** that records what a build taught you: *on this change, this bit us, and
here is what we did.* Findings are written deliberately, by you or by an agent you ask:
`docket learning record` adds a new finding and `docket learning update` edits an existing one,
each from a JSON request. Findings are never written for you when a change closes out, so a change
that taught nothing worth keeping simply adds none.

- **One file per finding.** Each finding is one file under the learnings directory on the `docket`
  branch (where the backlog, specs, and decisions are stored, separate from the code). A
  hand-maintained index file there, `learnings/README.md`, lists them; docket never rewrites it,
  so keep it current when you add a finding you want found.
- **Read on demand.** The design, planning, and review steps load only the *index* — a small
  hint surface — and pull the full text of just the findings that bear on the change at hand.
  Nobody pays to re-read the whole history on every run; the index is how a growing memory stays
  cheap to carry.
- **One switch.** `learnings.enabled: false` turns the ledger off: `docket learning record` and
  `docket learning update` refuse, and the workflows stop reading findings. It is a gate, never a
  purge — your existing findings stay untouched, and turning it back on resumes where you left off.

## War story or rule

Most findings stay war stories, pulled in by relevance when a similar change comes along. But some
lessons are not war stories at all — they are **rules** that must fire *unprompted*, on every run,
whether or not anyone thought to look them up.

The test is one question: **will the agent know to search for this?** If it would — the lesson is
discoverable exactly when it is relevant — it stays a finding, pulled by relevance. If it would
not — the agent has to already know it to avoid the mistake — it belongs in the project's
always-in-context instructions (the `AGENTS.md` / `CLAUDE.md` file the agent loads on every run).

Moving a rule there is **your** decision and your edit. docket never edits your always-in-context
file itself — the one place a wrong entry would silently reshape every future run is the one place
a person always decides.
