<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0244 — One selection rule for the four frontmatter read shapes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0244-one-selection-rule-for-the-four-frontmatter-read-shapes.md)**
<!-- docket:backlink:end -->

# One selection rule for the four frontmatter read shapes — design

**Change:** 0244 · **Date:** 2026-08-07 · **Type:** refactor

## Problem

`scripts/lib/docket-frontmatter.sh` offers four scalar read shapes — whole-file `field()`
(plus its raw twin `field_raw()`), and the anchored trio `fm_field` / `fm_field_raw` /
`fm_field_verbatim` — with silently different behaviors (whole-file vs first-`---` block;
quote strip; inline-comment strip). No recorded rule says which shape a call site should
use, and the difference is invisible until a key is absent, a value carries ` #`, or a
value is quoted. The 0235 `_fm_scan` edit changed `fm_field`'s return for quoted values
across all consumers without per-consumer verification — the motivating incident.

## Deliverables

1. **Fresh census** — every `field` / `field_raw` / `fm_field` / `fm_field_raw` /
   `fm_field_verbatim` / `int_field` / `list_field` call site in `scripts/` and
   `scripts/lib/`, classified by (a) can the key be legitimately absent from frontmatter,
   (b) can the value legitimately contain a whitespace-preceded `#`, (c) does the caller
   do its own quote/escape decoding or judge YAML form. The census in this spec (taken
   2026-08-07 against main) is the starting inventory; the build re-takes it against its
   own base commit before migrating.
2. **The selection rule**, recorded as a decision table in the library header (canonical)
   plus a short cross-reference in `scripts/board-checks.md` beside the existing
   ADR-0057 anchoring note:
   - Key **guaranteed present** in every file the caller reads (change files: `id`,
     `status`, `slug`, `title`, `priority`, `created`, `updated`; ADRs: `id`, `status`,
     `title`, `change`, `date`; findings: `slug`, `hook`, `topics`) → `field()`
     (whole-file is safe: the frontmatter line always wins). Existing sites stay; no churn.
   - Key that **may be absent** → anchored read, never `field()` (ADR-0057). Within the
     anchored tier: ordinary structured values (paths, ids, ISO timestamps, branch names,
     URLs, state tokens) → `fm_field`; free-prose values where ` #` is data
     (`blocked_by`) → `fm_field_verbatim`, because `fm_field`'s comment strip is exactly
     the truncation that loses `PR #69 is stale…`.
   - Caller does its **own quote/escape decoding** → raw tier (`field_raw` /
     `fm_field_raw`), per ADR-0058. Two live examples, both `field_raw` on
     always-present keys: `render-learnings-index.sh` `dequote()` on `hook`, and
     `board-checks.sh:395` `field_raw title` feeding `scalar_form_check` (which must see
     the quotes).
   - Caller **judges the scalar's YAML form as authored** → `fm_field_verbatim`
     (board-checks `scalar_form_check`).
   - A new call site unsure whether its key is optional uses the anchored shape —
     anchoring is always correct; whole-file is only ever an optimization grandfathered
     for guaranteed-present keys.
3. **Migrations** — the optional-key `field()` reads move to the anchored shape:
   - `render-change-links.sh:87-91` — `branch`, `spec`, `plan`, `results`, `pr` → `fm_field`
   - `terminal-publish.sh:248,311` — `spec`, `plan`, `results` → `fm_field`
   - `reclaim-claims.sh:67,70` — `claimed_at`, `branch` → `fm_field`
   - `docket-status.sh:1017` — `blocked_by` → `fm_field_verbatim`; `:1056` —
     `promotion_state` → `fm_field`
   - `board-checks.sh:257` — `spec`, `trivial`; `:409` — `plan`/`results`; `:420-421` —
     `branch`, `claimed_at` → `fm_field`
   - `github-mirror.sh:113,203` — `issue`; `:157` — `spec`; `:159` — `plan`, `results`
     → `fm_field`
   - `render-learnings-index.sh:115,120` — `promotion_state`, `promoted_to` → `fm_field`
   - `render-board.sh:279` — `pr`; `:310` — `spec`, `branch` → `fm_field`; `:316` —
     `blocked_by` → `fm_field_verbatim`
   - `archive-change.sh:120,122` — `claimed_at`, `results` → `fm_field`
   - `docket-frontmatter.sh` `readiness()` (~:312) — `spec`, `trivial` → `fm_field`
   Line numbers are as-of-census; the build resolves against its base.
4. **`fm_field_raw` disposition** — keep it; record in the header that it has zero
   in-repo production callers (tests only), mirroring how 0235 resolved its finding 7 for
   `docket_scalar_needs_quoting`, and that this is not an invitation to delete it: it is
   the documented raw twin the next optional-key decoding consumer reaches for.
5. **Guard** — in `tests/test_docket_frontmatter.sh` (or a sibling
   `tests/test_frontmatter_read_shapes.sh` if size warrants):
   - a census guard: grep `scripts/` for `$(field ` / `$(field_raw ` reads and fail when
     the (accessor, key) pair is outside the guaranteed-present allowlist — a new
     unanchored optional-key read turns the suite red with a message pointing at the
     rule. The allowlist must include `(field_raw, title)` (board-checks.sh:395) as well
     as `(field_raw, hook)`. Two non-literal-key shapes need explicit handling:
     `scripts/lib/docket-frontmatter.sh`'s own delegations inside `list_field`/`int_field`
     (`field "$1" "$2"`) are excluded by path, and any variable-key consumer call
     (e.g. `field "$f" "$key"` — gone from board-checks.sh:409 after its migration)
     fails the guard by default so it must be either migrated or consciously allowlisted;
   - an orphan pin: `fm_field_raw` production-caller count == 0 (the lib itself excluded
     by the same path exclusion as the census guard — `fm_field` delegates to it at
     lib line ~241 — plus comments), so silent adoption or deletion is both visible;
   - one behavioral fixture: a change file with `branch:`/`spec:` absent from frontmatter
     but present as body prose, asserted empty through the migrated
     `render-change-links.sh` read path (the highest-blast-radius consumer: it stamps
     values into specs, plans, results and PR bodies).
   Grep patterns stay `case`/fixed-string simple — no bounded repetition (BSD
   `/usr/bin/grep` vs ugrep divergence, change 0130 learning).

## Out of scope (from the stub, confirmed)

- Inverting `field()`'s whole-file default (ADR-0058's two-tier split stands; ADR-0057 is
  satisfied per-site).
- New read shapes. In particular no "anchored, quote-strip, no comment-strip" fifth shape:
  the one field that wants it (`blocked_by`) is served by `fm_field_verbatim`, accepting
  that a quoted `blocked_by` arrives quotes-intact (see Assumptions #3).
- The `list_field`/`int_field` wrappers keep delegating to `field()`. Their keys with
  live production callers (`id`, `depends_on`, `adrs`, `topics`,
  `supersedes`/`reverses`/`relates_to` on ADRs, plus `int_field pr` at
  docket-status.sh:510 — optional-*valued* but key-present) are all empirically
  0-missing across the tree. `related`/`discovered_from` have test-only wrapper callers,
  and `discovered_from:` is genuinely ABSENT from ~96 pre-template change files — the
  rule table must record them as test-only/not-key-guaranteed, never as
  template-guaranteed. Migration of the wrappers themselves stays out of scope.
- mint-stub's write path and the change-template `type: … # chosen at creation` comment
  contract (the reason the comment strip exists) — untouched, per 0240's boundary.

## Assumptions

1. **Rule location: library header canonical + board-checks.md pointer; no new ADR.**
   Rejected: a standalone doc (nobody would find it; the header is where every consumer
   already reads the contract) and a new ADR (ADR-0057 and ADR-0058 already record the
   *decisions*; this change consolidates them into an operational selection table —
   publishing a third overlapping ADR would fragment the record it is trying to unify).
   This resolves the stub's open question: no separate per-tier decision-table document.
2. **Migration set = optional-key `field()` reads only; guaranteed-present keys stay on
   `field()`.** Rejected: migrating everything to anchored reads (safer-looking, but it
   is exactly the "invert the default" move the stub rules out of scope, would churn
   ~40 known-safe sites, and would silently add comment-strip semantics to reads like
   `title` where a `#` is data). Rejected: migrating nothing and only documenting (the
   unanchored optional reads are live ADR-0057 hazards the ADR itself calls un-closed).
3. **`blocked_by` display reads route through `fm_field_verbatim`, not `fm_field`.**
   Chosen because its value is free prose where ` #…` is data (`PR #69 is stale`), and
   `fm_field`'s comment strip would truncate it — the exact defect class 0235 fixed for
   the checker. Cost accepted: a hand-quoted `blocked_by` renders with quotes intact on
   the board/status surface. Rejected: `fm_field` (silent truncation is worse than a rare
   visible quote); a new strip-quotes-only shape (out of scope: no new shapes, and one
   field does not justify a fifth tier). Rejected: unwrapping quotes at the call site
   (re-scatters the decoding the lib centralizes).
4. **`fm_field_raw` is kept as a documented orphan.** Per the stub's own default and
   0240's analysis: it is a public accessor in a shared lib, pinned by
   `tests/test_docket_frontmatter.sh`, and the raw/anchored quadrant of the rule table
   would be empty without it. Rejected: deletion (a real API decision, not cleanup, and
   the next decoding consumer of an optional key needs it).
5. **Guard = static census allowlist + orphan pin + one behavioral fixture.** Rejected:
   behavioral fixtures for every migrated site (ten consumers, five test files — bulk
   without proportionate signal; the census guard catches the *class*) and a
   documentation-only guard (not mutation-detectable). The allowlist is keyed on
   (accessor, key) so a new always-present key is a conscious one-line addition with the
   rule in the failure message. It seeds with both live `field_raw` sites (`hook`,
   `title`); the lib's own `list_field`/`int_field` delegations are path-excluded; a
   variable-key call fails by default (see Deliverable 5) so the guard cannot be silently
   routed around with `field "$f" "$key"`.
6. **Migrating `readiness()`'s internal `spec`/`trivial` reads changes shared-lib
   behavior for all its callers** (docket-status, render-board, github-mirror readiness
   tokens). Treated as safe: the new behavior differs only when `spec:`/`trivial:` is
   absent from frontmatter while body prose opens such a line — today that misread makes
   a needs-brainstorm change silently build-ready (spec appears non-empty), which is the
   *worse* outcome (the autonomous builder claims an undesigned change). Verified by the
   existing readiness tests plus the new absent-key fixture.
7. **The 0235 quoted-value concern is covered by the rule, not re-verified per consumer
   here.** The census records which consumers read possibly-quoted values; the guard pins
   accessor choice per site. Re-auditing `_fm_scan`'s quoted-value return itself is
   0235's landed behavior and out of scope.
8. **Dependency state:** none. #0134 and #0240 are killed (consolidated here); 0235 is
   merged; ADR-0057 names #0134 as its follow-up and this change inherits that pointer —
   the build's close-out should note in its results that ADR-0057's tracked follow-up
   landed as 0244 (no ADR edit; the ledger is immutable and the pointer resolves via
   0134's kill note).

## Build shape

Single branch, ~4 commits: (1) re-census + rule table in the lib header +
board-checks.md note; (2) migrations, consumer by consumer, suite green after each;
(3) guard tests; (4) docs touch-ups. No new scripts, no schema changes, no board
semantics changes — board output must be byte-identical for every change file whose
optional keys live in frontmatter (the only diffs possible are files currently
misreading body prose, and any such diff is a bug fix to be called out in the PR).
