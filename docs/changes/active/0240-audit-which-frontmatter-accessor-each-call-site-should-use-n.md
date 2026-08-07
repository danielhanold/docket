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
quote.

**Call-site census (measured 2026-08-07 against `feat/writers-emit-unquoted-yaml-title-scalars-so-six-change-files`, PR #172).** The distribution is
known, so the audit does not have to start by discovering it:

| Accessor | Production call sites | Consumers |
|---|---|---|
| `fm_field` | the large majority | `backfill-change-types.sh`, `board-checks.sh`, `github-mirror.sh`, `render-artifact-backlink.sh`, `render-board.sh` |
| `fm_field_verbatim` | 1 | `board-checks.sh` (`blocked_by`, added by 0235 fix 1) |
| `fm_field_raw` | **0** | none — only a prose mention survives, in a `board-checks.sh` comment |

Two consequences worth carrying into the design:

- **`fm_field_raw` is now an orphan, and 0235 orphaned it.** Fix 1 moved its only caller to
  `fm_field_verbatim`. It is *not* dead application code — it is a documented public accessor in a
  shared library, exercised by `tests/test_docket_frontmatter.sh` — so removing it is a real
  decision, not a cleanup. The default disposition should be to keep it and record that it has no
  in-repo caller, mirroring how 0235 resolved review finding 7 for `docket_scalar_needs_quoting`.
- **The `fm_field` sites are the actual subject.** 0235's fix 4 edited the shared `_fm_scan` body,
  changing what `fm_field` returns for quoted values. The suite is green across all 87 files, but
  green proves only that no test encodes the old behavior — not that each consumer *wants* the new
  one. That verification is the work, and it spans five consumers whose tests live in five
  different files.

This census was taken after PR #172 was reviewed and is recorded here rather than acted on there:
0235 was already twice-expanded, and its fix loop has no re-review round.

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
