---
id: 41
slug: learnings-findings-directory-and-promotion-valve
title: Learnings ledger restructure — findings directory + derived index + human-gated promotion valve
status: Accepted
date: 2026-07-16
supersedes: []
reverses: []
relates_to: [5, 12, 19, 28, 30, 31, 32, 39]
change: 67
---

## Context

Change 0006 gave the learnings ledger a single append-only file, harvested only at close-out
(ADR-0005). ADR-0005's decision — harvest only at close-out, single writer, single moment, ledger
unpublished, an idempotency probe — is unchanged and stays `Accepted`. What failed is its founding
*consequence*: "the ledger stays short enough to actually be read." The convention's distill rule
named two levers past ~300 lines — (a) merge near-duplicates, (b) promote durable conventions to
CLAUDE.md — but lever (b) never existed: this repo shipped no `CLAUDE.md`/`AGENTS.md` and nothing
wrote one. With only lever (a) available, distillation ran dry: change 0065's harvest merged five
near-duplicate families for ~31 lines net, and by 2026-07-16 the ledger stood at 485 lines / 34
top-level entries, of which merging further would mean deleting genuinely distinct lessons — not
distillation but destruction. Every one of those lines is also a context tax paid in full at three
hot read moments (`docket-groom-next` before a brainstorm; `docket-implement-next` at plan time and
at review) regardless of relevance to the change at hand.

`AGENTS.md` did not exist on `main` before this change — which is precisely why the promotion lever
could never fire and the ledger could only grow. Its absence is this change's founding motivation,
and this change creates it.

## Decision

The ledger becomes a **findings directory plus a rendered, derived index**
(`<changes_dir>/learnings/<slug>.md` + `learnings/README.md`), migrated from the 485-line/34-entry
`LEARNINGS.md` into 34 finding files (7 flagged `candidate`; provenance verified complete — 55
cited change ids resolve to 54 in the migrated findings, the residual `#0018` being a forward
reference rather than an unmigrated source). Six facets, recorded as one ADR because none has an
independent-reversal seam (see Consequences):

1. **Findings directory + derived index, not a single prose file.** Finding *files* stay curated
   prose — single-writer (the close-out harvest), human-curatable, **never regenerated**. This is
   the explicit departure from the convention's former "the ledger is prose, never regenerated"
   sentence and from change 0006's single-file structure. The *index*
   (`learnings/README.md`) joins the derived-view family (ADR-0012 script-vs-model boundary; ADR-
   0030/0031's guard-bound discipline) with its own sole-writer renderer,
   `scripts/render-learnings-index.sh` — offline, deterministic, STDOUT-only, no git writes,
   exactly the `render-adr-index.sh` shape.

2. **Finding files are named by bare `<slug>.md` — no index ordinal, deliberately unlike ADRs.**
   An ADR number encodes an immutable, ordered record: a new decision mints a new number and
   numbers never move. A finding is the opposite — a living file, extended every time a later
   change re-hits the same failure class. The slug is the harvest's dedup key ("does a finding
   about this class already exist?" is a direct lookup, not a scan for a free number) and it
   avoids a number-minting race between concurrent autonomous sweeps. Chronology lives in
   `created:`/`updated:` (reusing the change manifest's existing field spellings) and the dated
   `## War story` entries — a filename ordinal would be redundant and would fight the living-file
   model.

3. **The promotion valve.** The harvest sets `promotion_state: candidate` on `metadata_branch`
   when a lesson meets the tiering criterion — "will the agent know to search for this?" — but
   never touches the integration branch, preserving ADR-0005's never-publish rule intact. A human
   graduates a must-fire-unprompted rule by editing the integration-branch always-in-context
   agent-instructions file (`AGENTS.md` or `CLAUDE.md`, symlink-aware; `AGENTS.md` is the neutral
   spelling recommended when neither exists — the state this repo was in) and then flips the
   finding's `promotion_state: promoted` on `metadata_branch`.

4. **A promoted finding keeps its file.** It leaves the paid surface — the topic-grouped hint
   list and the cap count — but is never deleted. The file is: (a) the graduated rule's
   receipt/provenance (the war story and change ids that earned it); (b) the harvest's dedup
   memory, so a later re-hit of the same class extends the existing finding instead of re-minting
   a duplicate for an already-graduated lesson; (c) a one-line-reversible demotion path
   (`promoted → retained`, clear `promoted_to:`) with no git archaeology required. Decay is via
   status metadata, never deletion.

5. **Consolidation is human-gated, never autonomous.** Past `learnings.cap` (default 300, counted
   over active — `retained` + `candidate` — findings only) the harvest surfaces
   `learnings over-cap — needs curation` through the existing ungated report channel (ADR-0028); it
   never auto-merges its own memory (the Stanford ACE 2025 "context collapse" rationale — LLM
   self-consolidation of its own memory is exactly the operation refused here). The harvest may
   create a new finding or extend an existing one; it never merges two distinct findings — that
   stays human curation.

6. **Config controls and their fence classifications.** `promotion_state`'s default `retained` is
   a positive off-state per ADR-0032. The `learnings:` block (`scripts/docket-config.sh`) mirrors
   `finalize:`'s block *shape* per ADR-0019's nested-block precedent, exported as
   `LEARNINGS_ENABLED`/`LEARNINGS_CAP`. Both leaves are **global-able** per ADR-0019's fence
   classification: `learnings.cap` (default 300) gates only a human-facing, self-healing advisory;
   `learnings.enabled` (default `true`) is defined as a **read/write gate, not a purge** —
   disabling it on one machine only *omits* that machine's enrichment writes, never writes
   conflicting or corrupt state (no divergence, no "which ledger is authoritative" question), and
   the index self-heals on any enabled machine's next render. The accepted edge: a change that
   happens to close on a disabled machine is permanently un-harvested (no later enabled machine
   catches it up) — a strict subset of the omissions ADR-0005 already accepts (zero-entry
   harvests, results-file-only mid-build lessons, unharvested kills). `config.yml.example`
   documents both keys per ADR-0039 (the example is a wrapper-defaults mirror).

**A seventh, build-time decision**, judged non-obvious enough to record alongside the six above:
`learnings:`'s leaves are read differently from `finalize:`'s despite the block-shape mirroring.
`finalize.gate`/`finalize.test_command` are read by bare leaf-key (`yaml_get "$CFG" gate`) — safe
only because `gate`/`test_command` are unusual words unlikely to collide with another top-level or
future block. The learnings leaves, `enabled` and `cap`, are generic words a bare read would let
**any** other block (or a future top-level key) shadow. So each leaf is read *within* the block via
`yaml_block_body "$CFG" learnings` — the same mechanism the `skills:` block reader already uses for
exactly this reason — at every config layer (repo-local, repo-committed, global), before falling
back to the built-in default. The shadow risk is guarded by a dedicated, mutation-verified test
(`tests/test_docket_config.sh`: "a foreign block's `enabled:`/`cap:` does not shadow
`learnings.enabled`/`learnings.cap`" — swapping the block reads for bare `yaml_get` calls reddens
it).

## Consequences

- The ledger gets a real shrink valve — retrieval by relevance (index + pull-relevant-findings)
  instead of pay-by-history (read the whole file), plus a genuine graduation path out of the
  retrieval tier entirely. The three hot readers (`docket-groom-next`, `docket-implement-next` at
  plan and review) now load a small grouped index and pull only findings that look relevant,
  rather than paying the full ledger's token cost on every run.
- `learnings.enabled: false` zeroes the token cost end to end (readers skip both index and finding
  reads; the harvest no-ops with a one-line note, never silently) without destroying accumulated
  findings — re-enabling resumes from whatever finding files already exist, and the index
  self-heals on the next enabled render.
- The finding-files-stay-prose / index-is-derived split doubles the "derived view must never
  trail" surface `docket-status` must keep fresh (alongside `BOARD.md` and the ADR index) — guarded
  by the same commit-only-if-changed discipline and a determinism test on
  `render-learnings-index.sh`.
- All six facets are bound to one ADR because none has an independent-reversal seam: the bare-slug
  naming (#2) cannot be reversed without unwinding the living-file model the index (#1), the
  harvest dedup (#4), and the promotion valve (#3) all rest on; the keep-promoted-file rule (#4) is
  only coherent given the promotion valve (#3) and the derived-index/cap split (#1, #6); the enable
  gate (#6) is an on/off switch for the whole subsystem, meaningless to reverse apart from it. A
  later change that reverses only one facet (e.g. promotion) gets its own superseding ADR at that
  time, rather than this one being split preemptively into ADRs that could only ever be superseded
  together.
- The promotion valve is only as live as human attention to the `promotion-pending` advisory —
  candidates can pile up `candidate`-tagged indefinitely without graduating, which is the designed
  escalation (needs-you), not a failure mode.
- Because `learnings.enabled` is global-able rather than repo-fenced, a team running mixed
  enabled/disabled machines should expect disabled machines' close-outs to permanently skip
  enrichment for whatever they close — an accepted, bounded cost, not a defect to chase.
