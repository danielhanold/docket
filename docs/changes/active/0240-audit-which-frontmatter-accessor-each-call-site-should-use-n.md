---
id: 240
slug: audit-which-frontmatter-accessor-each-call-site-should-use-n
title: Audit which frontmatter accessor each call site should use, now that three anchored read shapes exist
status: proposed
priority: medium
type: refactor
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [235]
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

**Trigger** — surfaced by the whole-branch review of change 0235, which found that
`fm_field_raw`'s inline-comment strip (`sub(/[[:space:]]+#.*$/, "")`) is not quote-aware, and that
`scalar_form_check`'s `blocked_by` read was silently truncated by it before the value ever reached
the new predicate. 0235 fixed both narrow symptoms — the strip now skips a quoted value, and
`blocked_by` reads through a new comment-strip-free `fm_field_verbatim` — but it deliberately did
not audit the other call sites.

**Opportunity** — `scripts/lib/docket-frontmatter.sh` now offers three anchored read shapes with
different silent behaviors: `fm_field` (quote-stripped, comment-stripped), `fm_field_raw` (quotes
kept, comment-stripped), and `fm_field_verbatim` (neither). Nothing states which shape each call
site *should* use, and the difference is invisible until a value happens to contain ` #` or a
quote. `fm_field_verbatim` currently has exactly one caller, which is what a fresh distinction
looks like before anyone has checked whether its siblings were right all along.

**Independent value** — with 0235 fully reverted, the three accessors still exist minus
`fm_field_verbatim`, and the question of which reads may legitimately lose data to a comment strip
is still unanswered and still silent. The audit stands on its own: it is about the reader contract,
not about YAML quoting.

**Boundary** — enumerate every `fm_field` / `fm_field_raw` / `fm_field_verbatim` call site, decide
and record the correct accessor for each against what that consumer actually needs, and state the
selection rule in the library header and in `scripts/board-checks.md` so the next reader does not
have to re-derive it. Explicitly leaves alone: the unquoted-scalar predicate and its legs, the
`mint-stub` write path, and the change-template's `type: feat   # chosen at creation` contract,
which is the reason the comment strip exists at all and must keep working.

**Reason for deferral** — 0235's branch is scoped to the writer, its reader inverse, and the
checker. Touching `render-artifact-backlink.sh`, `board-checks.sh`'s `aborted-run` reads, and the
template-comment contract would expand a bounded YAML-quoting fix into a reader-contract refactor
across consumers whose tests live in different files.
