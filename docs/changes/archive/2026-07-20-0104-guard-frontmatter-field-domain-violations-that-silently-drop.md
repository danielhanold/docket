---
id: 104
slug: guard-frontmatter-field-domain-violations-that-silently-drop
title: Guard frontmatter field-domain violations that silently drop board rows
status: done
priority: high
created: 2026-07-19
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [94]
adrs: [49, 50]
spec: docs/superpowers/specs/2026-07-20-frontmatter-field-domain-guard-design.md
plan: docs/superpowers/plans/2026-07-20-frontmatter-field-domain-guard-plan.md
results: docs/results/2026-07-20-guard-frontmatter-field-domain-violations-that-silently-drop-results.md
trivial: false
auto_groomable: false
branch: feat/guard-frontmatter-field-domain-violations-that-silently-drop
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/113
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-frontmatter-field-domain-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-frontmatter-field-domain-guard-design.md) |
| Plan | [2026-07-20-frontmatter-field-domain-guard-plan.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-20-frontmatter-field-domain-guard-plan.md) |
| Results | [2026-07-20-guard-frontmatter-field-domain-violations-that-silently-drop-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-20-guard-frontmatter-field-domain-violations-that-silently-drop-results.md) |
| PR | [#113](https://github.com/danielhanold/docket/pull/113) |
| ADRs | [ADR-0049](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0049-board-checks-findings-channel-structural-columns-only-validated-values.md), [ADR-0050](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0050-backstop-checks-must-compute-not-reenumerate.md) |
<!-- docket:artifacts:end -->

## Why

**A frontmatter value that is well-formed text but outside its field's domain silently deletes the
change from every board surface.** No diagnostic fires on any channel. Reproduced against the
current renderer:

| | Trigger | Effect |
|---|---|---|
| A | `status: proposed  # awaiting X` | The change vanishes from the digest, the `ready` queue, and `BOARD.md`. `SECTION["$st"]` buckets on the raw read; both renderers iterate a fixed seven-name list, so an unrecognized bucket is never emitted. **No detection exists.** |
| B | `id: 4  # allocated by …` | Same total deletion. Already detected by `board-checks.sh`'s `malformed-id` (`board-checks.sh:74`) — this half needs surfacing/eventing, not a new check. |
| C | `title: T5 \| injected \| row` | Injects extra columns into the `BOARD.md` table row. |

For A and B, `render-board.sh:86` still counts the file in `total` while dropping its row, so the
board's count line and its tables **silently disagree** — a louder symptom than a missing row alone.
A three-change fixture with one poisoned `status:` renders `**3 changes** — 🟡 2 proposed` above two
rows, and the digest's `ready` queue drops the id entirely.

That last part is what makes this more than cosmetic: since change 0094 the `ready` line is the
**machine-parsed selection channel** for `docket-implement-next`. A change can therefore be quietly
removed from the autonomous build queue by a stray inline comment, with the board still reporting a
healthy-looking count.

## What changes

Detect field-domain violations and surface them, without letting one bad file take the board out.
Guard site is `scripts/board-checks.sh` — the existing warn-only frontmatter-validation channel,
whose findings `docket-status` surfaces through a generic passthrough (`docket-status.sh:623-630`),
so a new check-id needs zero extra wiring. Posture is warn-only throughout: `render-board.sh` sits
on the must-land Board pass and must never exit non-zero.

Four parts, designed in the linked spec:

- **`field-domain`** — a new check-id enumerating the domains the renderers actually consume:
  `status`, `slug` (a positive `[a-z0-9-]` grammar), `priority`, and `title` (no pipe). `id` stays
  with the existing `malformed-id`; no second overlapping check.
- **`board-row-dropped`** — the count-vs-rows invariant as a backstop, **suppressed** when a
  `field-domain` or `malformed-id` finding already explains that id, so it means exactly one thing:
  a row vanished and nothing enumerated explains why.
- **Sanitize the findings channel** — `emit` puts an untrusted frontmatter value in the
  TAB-separated change-id column, so a TAB in `id:` shifts the message into the wrong field when
  `docket-status.sh` reads it back. The guard's own reporting channel is injectable by the input
  class this change exists to catch.
- **Single-source the status vocabulary** — the seven-name list is written out at four sites in
  `render-board.sh`. Duplication makes the checker and the renderer drift in two directions and only
  one is caught; sharing one ordered array (authored as its active/terminal groups) eliminates both
  and is what makes the drift test load-bearing rather than a duplicate asserted against a duplicate.

`docket-implement-next` **reports, never gates** — a warn-only channel that halts autonomy would
turn one malformed file into a total backlog stall.

**Registration points a builder must not miss:** the `board-checks.sh` header block,
`scripts/board-checks.md`, and the closed check-id enumeration in `scripts/docket-status.md`.
**Both enumerations are already stale, in opposite directions** — `board-checks.sh:11-12` omits
`malformed-id`; `scripts/docket-status.md:344` omits `stale-finalize-blocked` (change 0098's
check-id was never registered there). Reconciling both is in scope: this change edits those exact
lines anyway, and a guard against silent drift that ships alongside two drifted enumerations of its
own check-ids would be self-refuting.

## Out of scope

- Rejecting or rewriting the offending change files. This change makes the failure **visible**; it
  does not decide what a malformed file's canonical value should be.
- Broader frontmatter schema validation beyond what board rendering actually consumes.
- Restructuring `render-board.sh`'s `emoji_for` / `label_for_title` case statements into data — they
  are a parallel representation of the same vocabulary, pinned by test rather than unified.

## History

**2026-07-20 — re-scoped by hand.** This change was filed as "Guard TAB in frontmatter values
feeding the digest's TAB-joined sort rows". `docket-auto-groom` verified that premise **falsified**:
the `created:` → `ready`-line trigger does not reproduce, because 0094 already closed it twice over
(`created:` is shape-checked at `render-board.sh:180-183` and fails to the `9999-99-99` sentinel;
`id` comes from `int_field`). A 322-case fuzz produced zero violations of `^ready( [0-9]+)*$`, and
`field()` truncates at the first newline so no value can fabricate an extra `ready` line.

The `slug:` half was real but small, and the groomer's probe surfaced the far more severe defect
this change now targets. Re-scoping onto it — rather than killing 0104 and minting fresh — keeps the
verified findings and the settled guard-site design attached to the work they inform.

Rejected guard sites, recorded so they are not re-litigated: changing `field()` (66 call sites
across 13 scripts inherit the contract) and renderer-stderr warnings (outside the closed report-line
vocabulary callers key on — nothing would ever parse them).

**2026-07-20 — groomed.** One premise from the re-scope did not reproduce: there is no
filename-derived slug fallback in the board path (`render-board.sh:135` reads `field "$f" slug`
bare), so the slug check is frontmatter-only. `reclaim-claims.sh:71,101` and `archive-change.sh:88-89`
do derive a slug from the basename, but they are not board consumers and stay out of scope.
Grooming also surfaced the findings-channel injection (part 3) and the case-statement residual
(part 4), neither of which was in the stub. Rejected during grooming: gating
`docket-implement-next` only on selection-affecting findings — it requires knowing whether the
poisoned file *would* have been `proposed`, which is exactly unknowable when the poisoned field
**is** `status`.

## Reconcile log

**2026-07-20 — reconciled at claim.** Every load-bearing claim in the change body and the linked
spec was re-verified against `origin/main` at `2748ed9`. All of it holds, line numbers included:

- `render-board.sh` buckets on the raw read (`SECTION["$st"]`, `:78`) and counts the file in `total`
  (`:86`) regardless — the count-vs-rows disagreement is real.
- The seven-name status list is written out at exactly the four sites the spec names: `:123` and
  `:193` (full seven), `:137` and `:290` (active five). Substitution targets confirmed.
- `emit malformed-id "$raw" …` (`board-checks.sh:74`) does place an untrusted frontmatter value in
  the TAB-separated change-id column that `docket-status.sh:627` reads back with
  `IFS=$'\t' read -r check_id change_id message`. The injection hole is live.
- `emoji_for` (`:63-66`, seven arms) and `label_for_title` (`:205-208`, five arms) carry no
  catch-all; `label_for` (`:67`) does. Matches the spec's residual analysis.
- `docket-status.sh`'s findings passthrough (`health_checks`, `:623-630`) is fully generic — a new
  check-id needs zero wiring there.
- The slug alphabet `^[a-z0-9-]+$` is exactly `slugify`'s output alphabet (`mint-stub.sh:88-91`).

**New finding — the registration drift runs both ways.** The spec flagged `board-checks.sh:11-12`
as stale (omits `malformed-id`). Reconcile found the *other* enumeration stale too:
`scripts/docket-status.md:344` lists eight check-ids and omits `stale-finalize-blocked`, which
change 0098 shipped without registering. Folded into scope rather than minted as a follow-up — it
is drift on the very lines this change already has to edit, and shipping a drift guard with two
drifted copies of its own check-id list would undercut the change's own premise.

**Spec's deferred reconcile question, answered.** The spec asked whether `docket-implement-next`
already echoes step-0 `check` lines generically before planning an edit to it. It does not, and no
edit is warranted: implement-next dispatches `docket-status` as a subagent whose contract is git
state, not an in-context return, so check lines surface through `docket-status`'s own report
(SKILL.md:66 summarizes health-check findings generically) rather than through implement-next.
Adding a bespoke echo path to implement-next for a warn-only channel would buy nothing and edge
toward the gating posture the spec explicitly rejects. **No skill-file edits in this change** — the
"reports, never gates" posture is agent behavior at run time, satisfied by the existing generic
passthrough at both layers.

**No collision with in-flight work.** Change 0106 (`in-progress`) touches
`tests/test_docket_config.sh` only — disjoint from `board-checks.sh`, `render-board.sh`,
`lib/docket-frontmatter.sh`, and their tests. Nothing in the recently-archived set (0107, 0105,
0101) or in ADRs 0044-0048 alters this change's design.

**Scope unchanged otherwise.** All four parts stand as specified; no work has been done elsewhere,
and no new constraint narrows or widens the design.
