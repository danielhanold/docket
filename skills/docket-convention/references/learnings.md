# Learnings ledger — full mechanics

Deep mechanics behind the convention's *Learnings ledger* section (which owns the ledger's
identity and the read contract). This reference owns the record side: finding-file shape,
recording, promotion, and the off switch.

- [Structure — index + detail](#structure--index--detail)
- [Finding-file frontmatter](#finding-file-frontmatter)
- [Recording a finding — create / extend, never merge](#recording-a-finding--create--extend-never-merge)
- [Promotion — the shrink valve](#promotion--the-shrink-valve)
- [Off switch](#off-switch)

## Structure — index + detail

A **finding** is one lesson or one consolidated family. The finding *files* are curated prose,
written by the `learning.record` and `learning.update` operations — **never regenerated**. The
*index* (`learnings/README.md`) is a derived view that no operation refreshes. That split is the
whole design: readers pay for a small hint surface, not for history. `LEARNINGS.md` remains as a
pointer stub to the earlier single-file ledger.

## Finding-file frontmatter

```yaml
---
slug: guards-are-code
hook: "A guard is code — mutation-test it or it is decoration."   # QUOTED (carries a colon-space)
topics: [testing, sentinels]        # first tag is the PRIMARY grouping topic
changes: [14, 15, 21]               # provenance: the change ids it came from
created: 2026-06-17
updated: 2026-07-16
promotion_state: retained           # retained | candidate | promoted  (default retained, ADR-0032)
promoted_to:                        # set only when promoted: the agent-instructions file it graduated into
---

## Apply
<the distilled, actionable rule>

## War story
- 2026-07-14 — <what happened>. …
```

## Recording a finding — create / extend, never merge

The `learning.record` operation **creates** a new finding (`slug`, `hook`, `topics`, `changes`,
`## Apply`, `## War story`) under its `request_id` idempotency key; the `learning.update`
operation **extends** an existing one, pinned by its exact record revision (append a dated
`## War story` entry, add the change id to `changes:`). Both write only the one finding file on the metadata branch and refuse
when `learnings.enabled` is not `true`. Neither **merges two distinct findings** — that is a human
act. Zero findings is normal; kills are not recorded.

## Promotion — the shrink valve

Tiering criterion: *"will the agent know to search for this?"* A rule that must fire
**unprompted** graduates; a war story stays in retrieval. A candidate carries
`promotion_state: candidate` on the metadata branch and **never touches the integration branch**
(ADR-0005). A human lands the graduation in the integration-branch agent-instructions file
(`AGENTS.md`/`CLAUDE.md`, symlink-aware; `AGENTS.md` is the neutral spelling when neither
exists) and flips `promoted` + `promoted_to:`. A promoted finding leaves the topic groups for a
compressed `## Promoted` appendix — but its file is **kept**, never deleted: it is the graduated rule's receipt, the dedup memory against
re-recording a duplicate, and a one-line-reversible demotion path.

## Off switch

`learnings.enabled: false` makes the whole subsystem a no-op **read/write gate, never a purge**:
readers skip. Existing `learnings/` files are left byte-untouched, and re-enabling resumes from
them.
