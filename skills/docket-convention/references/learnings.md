# Learnings ledger — full mechanics

Deep mechanics behind the convention's *Learnings ledger* section (which owns the ledger's
identity and the read contract; extracted in change 0085). This reference owns the write side:
finding-file shape, the harvest, promotion, capacity, and the off switch.

- [Structure — index + detail](#structure--index--detail)
- [Finding-file frontmatter](#finding-file-frontmatter)
- [Harvest — create / extend, never merge](#harvest--create--extend-never-merge)
- [Promotion — the shrink valve](#promotion--the-shrink-valve)
- [Capacity](#capacity)
- [Off switch](#off-switch)

## Structure — index + detail

A **finding** is one lesson or one consolidated family. The finding *files* are curated prose,
written only by the harvest and by human curation — **never regenerated**. The *index*
(`learnings/README.md`) is a derived view, rendered by `render-learnings-index.sh` (its sole
writer, ADR-0012), which joins the derived-view script family. That split is the whole design:
readers pay for a small hint surface, not for history. `LEARNINGS.md` remains as a pointer stub
to the pre-0067 single-file ledger.

## Finding-file frontmatter

```yaml
---
slug: guards-are-code
hook: "A guard is code — mutation-test it or it is decoration."   # QUOTED (carries a colon-space)
topics: [testing, sentinels]        # first tag is the PRIMARY grouping topic
changes: [14, 15, 21]               # provenance + the harvest's idempotency key
created: 2026-06-17
updated: 2026-07-16
promotion_state: retained           # retained | candidate | promoted  (default retained, ADR-0032)
promoted_to:                        # set only when promoted: the agent-instructions file it graduated into
---

## Apply
<the distilled, actionable rule>

## War story
- 2026-07-14 (#72, PR #79) — <what happened>. …
```

## Harvest — create / extend, never merge

Only the harvest at close-out appends (single source: the *Harvest learnings* step in
`docket-finalize-change`; `docket-status`'s sweep invokes it by reference). The harvest
**creates** a new finding or **extends** an existing one (append a dated `## War story` entry,
add the change id to `changes:`, bump `updated:`) — it **never merges two distinct findings**,
which is human-gated curation. Zero findings is normal; kills are not harvested.

## Promotion — the shrink valve

Tiering criterion: *"will the agent know to search for this?"* A rule that must fire
**unprompted** graduates; a war story stays in retrieval. The harvest sets
`promotion_state: candidate` on `metadata_branch` and **never touches the integration branch**
(ADR-0005). A human lands the graduation in the integration-branch agent-instructions file
(`AGENTS.md`/`CLAUDE.md`, symlink-aware; `AGENTS.md` is the neutral spelling when neither
exists) and flips `promoted` + `promoted_to:`. A promoted finding leaves the topic groups for a
compressed `## Promoted` appendix and **stops counting against the cap** — but its file is
**kept**, never deleted: it is the graduated rule's receipt, the harvest's dedup memory against
re-minting a duplicate, and a one-line-reversible demotion path.

## Capacity

`learnings.cap` (default 300) counts **active findings** (`retained` + `candidate`) — not raw
lines, and not promoted ones. Past the cap the loop **flags** `learnings over-cap — needs
curation` through the digest's needs-you channel; it **never auto-merges its own memory**.
Consolidation and promotion are human acts.

## Off switch

`learnings.enabled: false` makes the whole subsystem a no-op **read/write gate, never a purge**:
readers skip, the harvest no-ops with a one-line note, `docket-status` skips the advisories and
the index self-heal, and `render-learnings-index.sh` is never invoked. Existing `learnings/`
files are left byte-untouched, and re-enabling resumes from them.
