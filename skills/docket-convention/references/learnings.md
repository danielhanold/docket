# Learnings ledger — full mechanics

Deep mechanics behind the convention's *Learnings ledger* section (which owns the ledger's
identity and the read contract; extracted in change 0085). This reference owns the record side:
finding-file shape, promotion, capacity, and the off switch. Automated recording and curation of
findings is deferred from Go v1 — the diagnostics below name what is deferred and the supported
manual alternative.

- [Structure — index + detail](#structure--index--detail)
- [Finding-file frontmatter](#finding-file-frontmatter)
- [Recording a finding — create / extend, never merge](#recording-a-finding--create--extend-never-merge)
- [Promotion — the shrink valve](#promotion--the-shrink-valve)
- [Capacity](#capacity)
- [Off switch](#off-switch)

## Structure — index + detail

A **finding** is one lesson or one consolidated family. The finding *files* are curated prose,
written by human curation — **never regenerated**. The *index* (`learnings/README.md`) is a
derived view: automated learnings-index rendering is deferred from Go v1 — existing
`learnings/README.md` bytes are preserved, not refreshed. That split is the whole design: readers
pay for a small hint surface, not for history. `LEARNINGS.md` remains as a pointer stub to the
pre-0067 single-file ledger.

## Finding-file frontmatter

```yaml
---
slug: guards-are-code
hook: "A guard is code — mutation-test it or it is decoration."   # QUOTED (carries a colon-space)
topics: [testing, sentinels]        # first tag is the PRIMARY grouping topic
changes: [14, 15, 21]               # provenance + the record's idempotency key
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

## Recording a finding — create / extend, never merge

automated learnings harvest is deferred from Go v1 — record or update findings by editing
`learnings/` files directly. A record either **creates** a new finding or **extends** an existing
one (append a dated `## War story` entry, add the change id to `changes:`, bump `updated:`) — it
**never merges two distinct findings**, which is human-gated curation. Zero findings is normal;
kills are not recorded. Explicit record creation and updates are ordinary file edits on
`metadata_branch`. No workflow refreshes `README.md` automatically, and the frozen Bash renderer
is not a supported fallback; existing records, the index, `learnings.cap`, and promotion-state
data remain parseable evidence.

## Promotion — the shrink valve

Tiering criterion: *"will the agent know to search for this?"* A rule that must fire
**unprompted** graduates; a war story stays in retrieval. A candidate carries
`promotion_state: candidate` on `metadata_branch` and **never touches the integration branch**
(ADR-0005). A human lands the graduation in the integration-branch agent-instructions file
(`AGENTS.md`/`CLAUDE.md`, symlink-aware; `AGENTS.md` is the neutral spelling when neither
exists) and flips `promoted` + `promoted_to:`. A promoted finding leaves the topic groups for a
compressed `## Promoted` appendix and **stops counting against the cap** — but its file is
**kept**, never deleted: it is the graduated rule's receipt, the dedup memory against
re-recording a duplicate, and a one-line-reversible demotion path.

## Capacity

`learnings.cap` (default 300) counts **active findings** (`retained` + `candidate`) — not raw
lines, and not promoted ones. Past the cap, ledger curation is human-directed:
automated learnings capacity and promotion are deferred from Go v1 — consolidation and promotion
are human acts, never an automated merge.

## Off switch

`learnings.enabled: false` makes the whole subsystem a no-op **read/write gate, never a purge**:
readers skip. Existing `learnings/` files are left byte-untouched, and re-enabling resumes from
them.
