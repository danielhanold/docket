<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0402 — Restructure the technical docs into goal-organised guide, concepts, and reference tiers](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-04-0402-restructure-the-technical-docs-into-goal-organised-guide-con.md)**
<!-- docket:backlink:end -->

# Restructure the technical docs into guide, concepts, and reference tiers — design

## Goal

After change 400 lands, the whole technical body of the README sits verbatim in
`docs/guide/README.md`, organised by mechanism and written in docket's internal vocabulary. This
change replaces that one page, and the three harness directories beside it, with three tiers of
pages sorted by the question each answers, all written for one reader: the shipped default
`dummy_mode` persona — a mid-level engineer who knows architecture, has working fluency in some
language, and is told every docket-internal term with a gloss on first use.

Nothing the relocated body says is lost. Every decision, caveat, config key, and option lands on
exactly one page, tracked by the coverage table below.

## Decisions (settled with the human, 2026-09-03)

| Question | Decision |
|---|---|
| Sort pages by audience (user vs maintainer) or by question | **By question**: guide (how do I), concepts (what is it and why), reference (exact fields). Audience-sorted pages with one reader rot. |
| Reference tier production | **Pointer pages only.** No generator, no Go work; each page names the owner of the fact. |
| Harness directories | **Move content, delete dirs.** Setup prose folds into the guide; runbooks, example JSON, and fixtures move to `docs/reference/harness/`. |
| Concepts page set | **Nine pages**: the core six plus build profiles and the test gate, finalize as a sequencer, learnings and ADRs as memory. |
| Docs entry point | **`docs/README.md`**; `docs/guide/README.md` is removed after the split. |
| Voice enforcement | **Editorial checklist in the spec**; review verifies by reading. No new guard. |
| Change 385 | **Absorbed** by the harness fold; 385 is killed by the human. |
| One change or two | **One change**; a split would need the same coverage table kept in sync across two PRs. |

## Layout after the change

```
README.md                      landing page (change 400; only its docs-map links change here)
docs/README.md                 docs index: three tiers, one hook per page, a start-here path
docs/guide/                    twelve how-to pages + index
docs/concepts/                 nine living explanation pages + index
docs/reference/                pointer pages + index; harness/ holds runbooks, example JSON, fixtures
docs/comparison/               change 400's brief, untouched
docs/adrs, docs/changes, docs/results, docs/superpowers, docs/release   untouched
```

Removed: `docs/guide/README.md` (after the split), `docs/cursor/`, `docs/codex/`, `docs/opencode/`.

## The reader and the voice

Every page in the three tiers is written for the shipped default `dummy_mode` persona. In
practice that means:

- **Gloss on first use.** The first time a page uses a docket term from the glossary below, it
  carries the approved one-clause gloss, in the same sentence or the one after. Later uses are
  bare. A page is a unit; a gloss on one page does not cover another.
- **Concrete consequence over abstraction.** "The second save loses its work and retries" beats
  "a compare-and-swap conflict".
- **No harness-specific jargon in normative prose.** Product names appear only in the harness
  guide page and the harness reference, and only where the instruction differs by harness.
- **Never drop a decision, a caveat, or an option to make the prose simpler.** Simplification is
  about vocabulary and framing, not content.
- **No line-number cross-references** (ADR-0054). Anchor on a heading, a symbol, or a quoted
  clause.
- **Headings are tasks or nouns**, never mechanism names: "Reclaim a stale claim", not
  "`reclaim-claims.sh`".

### Glossary — approved one-clause glosses

The builder uses these glosses verbatim on first use and may extend the list in the same commit
if a page needs a term not listed. Terms are ordered by how early a new reader meets them.

| Term | Gloss |
|---|---|
| change | one unit of planned work, roughly one pull request, tracked as one markdown file |
| board | the generated overview of every change and its state, never edited by hand |
| metadata branch | the `docket` git branch where the backlog, specs, and decisions are stored, separate from the code |
| integration branch | the branch code lands on, usually `main` |
| metadata worktree | a second checkout of the repo at `.docket/`, parked on the metadata branch, so backlog edits never touch your code checkout |
| claim | the moment a change is picked up for building; it records which branch will carry the work and when it was taken |
| claim lease | a timestamp on a claim; when it expires with no branch behind it, the change goes back to the queue |
| reconcile | a check at build time that the change is still worth doing and its assumptions still hold, before any code is written |
| build-ready | a proposed change that has a spec or is marked trivial and whose dependencies are all merged |
| needs-brainstorm | a proposed change with neither a spec nor a trivial mark; it needs a design conversation first |
| spec | the design document a change links to, written before building |
| plan | the task-by-task breakdown a build follows, written on the feature branch |
| results | the optional close-out record of what a build actually did |
| ADR | an architecture decision record: one file per decision, immutable once accepted |
| learnings | the loop's memory of lessons from past builds, curated by a human |
| skill | a named, reusable instruction set an agent loads for one job |
| agent | a separately launched worker with its own context, pinned to a model and effort |
| harness | the tool that runs the agent: Claude Code, Cursor, Codex, or opencode |
| dispatch | launching a named agent to do a step and waiting for it to return |
| build profile | one of four worker tiers (economy, standard, premium, max) a plan task is routed to by risk |
| build gate | the full test-suite run at the end of a build that must be green before review |
| build evidence | the committed record of that gate run, read by the reviewer |
| run gate | the bookkeeping around a launched build run: who launched it, whether it finished, whether it may be retried |
| finalize | the close-out sequence: rebase onto the integration branch, retest, merge, archive |
| stacked change | a change built on another change's unmerged branch rather than on the integration branch |
| coordination key | a config key whose value must be identical for every clone, so it may only be set in the committed repo config |
| capability catalog | the machine-readable list of every operation the `docket` binary offers, which skills read instead of hard-coding commands |
| disposition | the one-word outcome an operation reports: applied, no-op, refused, or error |
| health check | a status-time scan for things a human should look at: stale claims, broken links, stalled dependencies |

## Tier 1 — the guide (`docs/guide/`)

Twelve pages, one per non-landing row of change 400's goal-first docs map, plus a short
`README.md` index listing them in reading order. Each page opens with one paragraph saying what
the reader will be able to do, then task-shaped sections. The source column names the sections of
the relocated body (today's README) whose content the page absorbs; the builder's coverage table
(below) is the authoritative account.

| Page | File | Content | Source sections in the relocated body |
|---|---|---|---|
| Capturing work that outlives the session | `capturing-work.md` | change file and manifest fields as a user sees them; lifecycle and the board; priorities, types, dependencies, stacking; scan mode; trivial; capturing discovered work | *The change lifecycle*; *Capturing discovered work and typing it* (incl. taxonomy and migration) |
| Designing before building | `designing-before-building.md` | interactive grooming; autonomous grooming and the critic; consultant-authored specs; dummy mode for the conversation | *Quickstart* step 2; *Consultant-authored brainstorm*; *Speaking your language* (incl. persona gallery) |
| Building without supervision | `building-without-supervision.md` | implement-next end to end; reconcile; plan authoring; build profiles and escalation; worktrees and claims; draining with `/loop`; dispositions and halts | *Why docket*; *The reconcile superpower*; *docket-build*; *Draining hands-free with `/loop`* |
| Proving the build | `proving-the-build.md` | the build gate and its evidence; the gate driver and budgets; integration repair; configuring the suite command | *docket-build* gate paragraphs; `build:` and `finalize:` config keys |
| Reviewing before the human does | `reviewing-before-the-human.md` | the reviewer contract and its rungs; the fix loop and disposition table; why the suite runs before review | *docket-review* |
| Landing changes safely | `landing-changes.md` | finalize end to end; blocked and identity repair; branch protection; closing out with `/loop` | *Closing out hands-free with `/loop`*; *Hands-off finalize*; *Finalize → selective publish* |
| Keeping the backlog honest | `keeping-the-backlog-honest.md` | status versus sweep; health codes and what to do about each; reclaiming stale claims; recovering a halted run | *Reclaiming stale claims*; *Status* |
| Remembering why | `remembering-why.md` | ADRs; learnings, promotion, and what is human-curated | *Learnings — the loop's memory*; *Skills* (docket-adr) |
| Governing through configuration | `governing-through-configuration.md` | the four layers and the fence; every config block by purpose; the `skills:` map; misplaced or malformed files; migrating from `agents.yaml` | *Configuration* (all subsections not claimed above); *Workflow roles* |
| Running on your harness | `running-on-your-harness.md` | install and update; model and effort pins; the two invocation paths; one section each for Claude Code, Cursor, Codex, and opencode setup; runner delegation; Cursor auto-run | *Install*; *Updating docket*; *Tuning agent models & effort*; *Runner delegation*; *Running under Cursor Auto-run*; `docs/cursor/permissions.md`; `docs/codex/setup.md`; `docs/opencode/setup.md` |
| Where the metadata lives | `where-the-metadata-lives.md` | the two-branch model as a user meets it; where each artifact lives; GitFlow; the metadata worktree; `main`-mode; git-hook frameworks; migration | *docket-mode: where metadata lives* (all subsections not claimed by *Landing changes*); *Migration* |
| Daily loop | `daily-loop.md` | the quickstart, one screen long, linking into the pages above | *Quickstart: the daily loop* |

The harness page carries change 385's correction: the Cursor allowlist names the native `docket`
binary invocation, never `scripts/docket.sh`.

## Tier 2 — concepts (`docs/concepts/`)

Nine living pages plus a `README.md` index. Every page has the same four sections, in order:

1. **The problem it solves** — two or three paragraphs, no docket vocabulary until glossed.
2. **The moving parts** — the components and how they connect; one diagram (ASCII or mermaid)
   where a picture is clearer than prose.
3. **The invariants** — the rules that must hold, stated as bullets a reader could check.
4. **Decided in** — a bullet list of ADR links, each with one clause saying what that ADR fixed.
   The page never restates an ADR's context or consequences.

| Page | File | Decided in (seed list; the builder completes it from the ADR index) |
|---|---|---|
| Two branches and the metadata worktree | `two-branches.md` | ADR-0001, ADR-0002, ADR-0025, ADR-0046 |
| The change lifecycle as a state machine | `change-lifecycle.md` | ADR-0004, ADR-0005, ADR-0092 |
| Skills, agents, and harness dispatch | `skills-agents-dispatch.md` | ADR-0008, ADR-0015, ADR-0016, ADR-0018, ADR-0024, ADR-0026, ADR-0044, ADR-0064 |
| The run gate and attribution | `run-gate.md` | ADR-0074 and the run-gate ADRs the builder finds under changes 0271, 0342, 0345 |
| Config layers and the coordination fence | `config-layers.md` | ADR-0019, ADR-0048, ADR-0052 |
| Reconcile | `reconcile.md` | the reconcile ADRs the builder finds; the page also links the comparison brief's reconcile row |
| Build profiles and the test gate | `build-profiles-and-gate.md` | ADR-0023, ADR-0074 |
| Finalize as a sequencer | `finalize-sequencer.md` | ADR-0010, ADR-0011, ADR-0042 |
| Learnings and ADRs as memory | `memory.md` | ADR-0041 |

Concepts pages describe **current state only**. When a decision is later reversed, the concepts
page changes and the ADR does not; the "Decided in" list gains the reversing ADR.

## Tier 3 — reference (`docs/reference/`)

Pointer pages plus a `README.md` index. A reference page says **where the fact is owned and how to
read it**, and quotes nothing that can drift. Each page has one line per item: the item, its
owner, and the command or file that shows the current value.

| Page | File | Owner named |
|---|---|---|
| The CLI by noun and verb | `cli.md` | `docket --help` and `docket <noun> --help`; the capability catalog via `docket capabilities --json` |
| Change manifest and ADR fields | `fields.md` | the `docket-convention` skill's manifest and ADR sections, quoted by heading |
| Config keys | `config-keys.md` | `.docket.example.yml` (ADR-0048) and the convention's config section |
| Dispositions, reason tokens, and health codes | `outcomes.md` | the convention's disposition vocabulary; `docket status --json` for health codes; the finalize skill's failure reference for reason tokens |
| Skills and agents inventory | `skills-and-agents.md` | the `skills/` and `agents/` directories; the harness's own agent registry for availability |
| Harness runbooks and examples | `harness/README.md` | index of the moved files below |

`docs/reference/harness/` receives, by `git mv` with links rewritten: `docs/cursor/validation.md`,
`docs/cursor/permissions.example.json`, `docs/cursor/sandbox.example.json`,
`docs/codex/validation-runbook.md`, `docs/codex/fixtures/`. The runbooks keep their content and
are re-voiced only in their opening paragraph and headings; their step lists are procedures and
stay as written apart from the 385 correction.

## The docs index (`docs/README.md`)

One page: a two-sentence statement of the three tiers; a **Start here** path of four links for a
new user (daily loop → capturing work → building without supervision → landing changes); then
the three tiers, each page with its one-line hook. The top-level README's documentation map
(change 400's section 7) is edited to link `docs/README.md` first, then the tier indexes.

## Guards, links, and records

- **Repoguard rows.** Every `internal/repoguard/prose_contracts_test.go` row whose `file` is
  `docs/guide/README.md` (after 400), `docs/cursor/*`, or `docs/codex/*` is repointed at the page
  that now carries its `present` phrases, in the same commit as the move. Where a phrase is
  re-voiced, the row's phrase is updated to the new wording in the same commit; the row is never
  dropped. The `test_cursor_permissions_docs` README row is repointed at the harness guide page's
  link into `docs/reference/harness/`.
- **Link retargeting.** Every maintained-source link (skills, agents, `docs/`, `.docket.example.yml`,
  `tests/README.md`, `AGENTS.md`) into a removed path is retargeted. Point-in-time records
  (archived changes, results, specs, plans, Accepted ADRs, `docs/release/`) are left alone, per
  change 400's rule and the AGENTS.md cross-reference section.
- **Coverage table.** The builder commits `docs/superpowers/plans/<date>-docs-coverage.md` (or a
  section of the plan) mapping every `##`/`###`/`####` heading of the relocated body, and every
  file under the three harness directories, to the page and section that now carries it. A row
  may say *dropped* only with a reason that names why the content was already stale.

## Acceptance

1. `docs/README.md`, `docs/guide/`, `docs/concepts/`, and `docs/reference/` exist with the pages
   named above; `docs/guide/README.md`, `docs/cursor/`, `docs/codex/`, and `docs/opencode/` do not.
2. The coverage table has a row for every heading of the relocated body and every harness file,
   and no row is *dropped* without a reason.
3. Every relative link under `docs/` and in `README.md` resolves to an existing path (a one-off
   shell check in the build; not a permanent guard).
4. Every concepts page has the four sections in order and a non-empty "Decided in" list.
5. Every reference page names an owner for every item and contains no copied table of values.
6. The glossary's first-use rule holds on every page, verified by the reviewer reading each page
   against the checklist above.
7. The repoguard suite is green with every repointed row present; no row was deleted.
8. The full suite (`go run ./cmd/docket development test`) is green.

## Open questions carried into build

- Whether `daily-loop.md` should merge into the guide index rather than stand alone. The builder
  keeps it standalone unless the index would otherwise be under thirty lines.
- Which ADRs cover the run gate and reconcile; the seed lists above are partial and the builder
  completes them from `docs/adrs/README.md`.
