---
id: 297
slug: relax-0212-s-sites-backtick-ban-now-that-the-hygiene-gate-en
title: 'Relax 0212''s SITES backtick ban now that the hygiene gate enforces it'
status: 'killed'
priority: medium
type: refactor
created: 2026-08-11
updated: '2026-09-03'
depends_on: []
related: []
discovered_from: [221]
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
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while reconciling change 0221 (the test-source hygiene gate). 0221's spec
records it as Assumption 7 and explicitly places it out of scope; auto-groom could not mint it
because auto-groom mints nothing.

**Opportunity** — change 0212 left a per-file comment in `tests/test_inline_role_stop_scoping.sh`
forbidding backticks inside its `SITES` anchor table. That comment is a convention standing in for
an enforcement that did not exist yet. Once 0221's checker lands, the enforcement is real and
repo-wide, and the ban can be relaxed the right way: move `SITES` from a double-quoted assignment
into a quoted-delimiter heredoc (`<<'EOF'`), which the checker treats as inert, so backticked
verbatim anchors become legal data again rather than a forbidden character.

**Independent value** — stands with 0221 reverted only in part, so the honest framing is that it
is worth doing once 0221 has merged: it restores the ability to anchor guards on the repo's own
house style (verbatim clauses containing backticked code spans), which AGENTS.md and ADR-0054
actively mandate. Today a guard author must choose between the mandated anchor style and the 0212
comment.

**Boundary** — one file's `SITES` block converted to a quoted-delimiter heredoc, the 0212 comment
rewritten to point at the checker instead of banning a character, and the anchors restored to
their verbatim form where they were degraded. It does not touch the checker, any other test file,
or what the guard asserts.

**Reason for deferral** — 0221's spec draws this boundary explicitly ("No edit to 0212's in-file
comment"), and the relaxation is only safe *after* 0221's gate is merged and proven; folding it in
would expand 0221's branch into the very file whose incident motivated it, mixing the enforcement
with a relaxation that depends on it.

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): superseded by the Go migration — the SITES block lived in the deleted tests/test_inline_role_stop_scoping.sh and the Bash hygiene checker is gone with it.
