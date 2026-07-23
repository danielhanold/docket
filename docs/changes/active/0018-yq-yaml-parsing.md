---
id: 18
slug: yq-yaml-parsing
title: Evaluate adopting yq for YAML parsing across docket scripts
status: proposed
priority: low
created: 2026-06-16
updated: 2026-06-16
depends_on: [16]
related: [11]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
type: refactor
---

## Why

docket's shell scripts parse YAML and markdown frontmatter with hand-rolled
`sed`/`awk`/`grep` — `sync-agents.sh` (change 0016), `scripts/github-mirror.sh`
(change 0011), and the frontmatter readers sprinkled through the tooling. In
`sync-agents.sh` the config-reading helpers (`entry_line`/`field_of`/`block_names`)
are dense regex over YAML, raised as a readability concern (it is run manually by a
human). `yq` would make those ~40 lines more readable and more *robust* (real YAML:
flow-vs-block mappings, quoting, spacing — the hand parser only handles the
documented block-style subset, and silently ignores top-level flow-style
`agents: {…}`).

The decision today (2026-06-16) was to **keep the scripts as-is** — but the
tradeoff is worth a deliberate future review rather than leaving it implicit. This
stub captures that.

## What changes

To be decided at brainstorm. The likely shape: **decide whether docket adopts `yq`
project-wide for YAML/frontmatter parsing**, and if so, do it consistently — the
honest move is all-or-nothing, not one bilingual script.

- If **yes**: rewrite `sync-agents.sh` and `scripts/github-mirror.sh` config/frontmatter
  parsing onto `yq`; add `yq` to the documented install prerequisites; pin *which*
  `yq` (the Go `mikefarah/yq` vs the Python `kislyuk/yq` are incompatible binaries);
  make the test suite require it. Record the decision as an ADR (it reverses the
  current implicit "pure bash, zero external deps" stance).
- If **no**: document the pure-bash convention explicitly (it currently lives only as
  a LEARNINGS entry + the `github-mirror.sh` contract) so the question doesn't keep
  resurfacing — possibly a short ADR.

## Out of scope

- **Partial adoption** (yq in one script, hand-rolled in another) — a bilingual
  codebase is the worst outcome; whatever is decided applies project-wide.
- `sync-agents.sh`'s `emit()` frontmatter rewrite — the wrappers are markdown with
  YAML frontmatter, not pure YAML; `yq` has no clean "edit a `.md`'s frontmatter"
  mode, so that part stays `awk` either way and is not a reason to adopt `yq`.

## Open questions

- Is the readability/robustness gain worth a new runtime dependency on tools whose
  whole pitch is low-friction "clone + run two bash scripts"?
- If adopted, which `yq` fork, and how is its presence (and version) checked at runtime?
- Does this warrant an ADR — adopting it would reverse the implicit pure-bash stance
  ([[0008]]'s consequences touch the same zero-dependency philosophy)?

## Reconcile log
