# Frontmatter field-domain guard — design

**Change:** [#0104](../../changes/active/0104-guard-frontmatter-field-domain-violations-that-silently-drop.md)
**Date:** 2026-07-20

## Problem

A frontmatter value that is well-formed *text* but outside its field's *domain* silently deletes the
change from every board surface. No diagnostic fires on any channel.

`render-board.sh` buckets on the raw read (`SECTION["$st"]`) and both renderers iterate a fixed
seven-name list, so an unrecognized bucket is never emitted. The file is still counted in `total`
(`render-board.sh:86`), so the board's count line and its tables silently disagree — a three-change
fixture with one poisoned `status:` renders `**3 changes** — 🟡 2 proposed` above two rows.

Since change 0094 the digest's `ready` line is the machine-parsed selection channel for
`docket-implement-next`. A stray inline comment can therefore remove a change from the autonomous
build queue while the board still reports a healthy count.

Settled before this spec, and not re-litigated here: the guard site is `scripts/board-checks.sh`
(the existing warn-only frontmatter-validation channel), the posture is warn-only, and two guard
sites were rejected — changing `field()` (66 call sites across 13 scripts inherit the contract) and
renderer-stderr warnings (outside the closed report-line vocabulary callers key on).

## Design

Four parts. Parts 1 and 2 are the guard; part 3 fixes a hole in the channel the guard reports
through; part 4 single-sources the vocabulary both the guard and the renderer depend on.

### 1 — `field-domain`: a new check-id in `board-checks.sh`

Emitted from the existing `FILES` walk (active + archive), warn-only, git-only. One finding per
violated field. The domains are chosen by what the renderers actually consume — not by a general
schema ambition (see *Out of scope*).

| field | domain | failure today |
|---|---|---|
| `status` | one of `DOCKET_STATUSES`; **empty also fails** | row dropped from every board surface |
| `slug` | `^[a-z0-9-]+$` — `slugify`'s own alphabet (`mint-stub.sh:88-91`); empty fails | leaks raw into the digest's space-joined `change` line |
| `priority` | one of `low\|medium\|high\|critical`; **empty is legal** | silently sorts as `medium` in the `ready` line, renders raw in the Priority cell |
| `title` | contains no `\|` | injects columns into the `BOARD.md` table row |

`priority`'s empty case is legal because the convention documents `medium` as the default and
`render-board.sh`'s sort already implements it. `status` and `slug` have no documented default —
empty is a violation.

`id` is deliberately **not** covered here. The existing `malformed-id` check already detects it; a
second overlapping check would double-report the same file.

The slug check is frontmatter-only. The original stub anticipated a filename-derived fallback, but
`render-board.sh:135` reads `field "$f" slug` bare — no fallback exists in the board path.
(`reclaim-claims.sh:71,101` and `archive-change.sh:88-89` do derive a slug from the basename, but
they are not board consumers and are out of scope.)

### 2 — `board-row-dropped`: a suppressed backstop

An active file counted in `total` but rendered in no section is itself a detectable invariant
violation. It is emitted **only when no `field-domain` or `malformed-id` finding exists for that
same change id** — both accumulate into `FINDINGS` keyed by id, so suppression is a lookup.

The suppression is what makes the backstop mean something. A backstop that fires alongside every
domain finding trains the reader to ignore it; suppressed, it says exactly one thing: *a row
vanished and nothing enumerated explains why.* With part 4 in place its remaining trigger is a
future renderer-added drop path — which is what a backstop is for.

### 3 — Sanitize the findings channel

`emit malformed-id "$raw" …` places an **untrusted frontmatter value in the TAB-separated change-id
column**, and `docket-status.sh:627` reads findings back with
`IFS=$'\t' read -r check_id change_id message`. An `id: 4<TAB>EVIL` shifts the message into the
wrong field. `field()` truncates at the first newline and strips trailing whitespace, but an
interior TAB survives.

Two fixes, both in `board-checks.sh`:

- `emit` escapes TAB and CR to visible `\t` / `\r` in every embedded value.
- The change-id column never carries a raw frontmatter value. It uses the filename-derived padded
  id, falling back to `?` when the filename yields none.

This closes the existing `malformed-id` hole and pre-empts the same hole in `field-domain`, whose
messages quote the offending value by design. The message column is the last field of the
`read -r`, so a TAB there is harmless — only the change-id column is at risk.

### 4 — Single-source the status vocabulary

The seven-name list is currently written out at four sites in `render-board.sh` in two shapes: the
full seven at `:123` and `:193`, the active five at `:137` and `:290`.

Move it into `lib/docket-frontmatter.sh`, authored as its two semantic groups with the seven
derived:

```bash
DOCKET_STATUSES_ACTIVE=(in-progress proposed blocked deferred implemented)
DOCKET_STATUSES_TERMINAL=(done killed)
DOCKET_STATUSES=("${DOCKET_STATUSES_ACTIVE[@]}" "${DOCKET_STATUSES_TERMINAL[@]}")
```

Every name appears exactly once. The split is not arbitrary — it is the convention's
terminal/non-terminal distinction, the same one the directory layout encodes (`active/` holds every
non-terminal status; `archive/` holds the two terminal outcomes). The concatenation reproduces the
existing order byte-for-byte, so `:123`/`:193` take `DOCKET_STATUSES` and `:137`/`:290` take
`DOCKET_STATUSES_ACTIVE`.

**Why this is load-bearing rather than tidying.** Duplicating the list makes `field-domain` and the
renderer drift in two directions, and only one of them is caught:

- New status added to `render-board.sh` but not `board-checks.sh` → `field-domain` fires a **false**
  finding on every file carrying it, and `board-row-dropped` is suppressed because a domain finding
  "explained" it. The guard becomes the noise source. The backstop is blind to this.
- New status added to `board-checks.sh` but not `render-board.sh` → the row is silently dropped and
  `board-row-dropped` fires correctly.

The duplication that gives the backstop its only realistic trigger is the same duplication that
manufactures the false-positive direction. Sharing the array eliminates both.

It also makes the test meaningful. Without the shared array, a test that "the two lists agree" must
hard-code the seven names a third time — asserting a duplicate against a duplicate, which passes
while both copies drift together.

**Residual: the case-statement mappings.** The list iterations are not the only place the vocabulary
is written down. `render-board.sh` also carries `emoji_for` (seven arms, no catch-all) and
`label_for_title` (five arms, no catch-all); an unknown status yields empty output from either.
`label_for` has a `*` catch-all and is total. These are a parallel representation the array cannot
unify, and they fail by printing *nothing* — the same silent-emptiness class this change exists to
kill, one layer down. Do **not** restructure them into the array; a case statement is the right
shape for a mapping. Pin them with a test instead (see *Testing*).

## Autonomy posture

`docket-implement-next` **reports, never gates.** Findings are surfaced in the run report and PR
body; selection and building proceed normally.

The argument for gating is real — a poisoned `status:` on a `critical` change does not make the
implementer fail, it makes it silently build the wrong change, and the dropped change's absence is
indistinguishable from it never being ready. It loses anyway: a warn-only channel that halts
autonomy converts one malformed file into a total backlog stall, which is the posture this change
exists to avoid. A precise "gate only on selection-affecting findings" variant was considered and
rejected — it needs the checker to know whether the poisoned file *would* have been `proposed`,
which is exactly unknowable when the poisoned field **is** `status`.

At reconcile, check whether `docket-implement-next` already echoes step-0 `check` lines generically
before planning an edit to it. The `docket-status.sh:623-630` passthrough auto-discovers new
check-ids with zero wiring; the report path may too.

## Registration points

A new check-id must be registered in three places beyond the emitting code:

- `scripts/board-checks.sh`'s header block at `:11-12` — currently **stale**, it omits the existing
  `malformed-id`. Add both.
- `scripts/board-checks.md` — the script contract.
- `scripts/docket-status.md` — the closed check-id enumeration in the `check <check-id>` report-line
  row.

## Testing

- One fixture per domain in `tests/test_board_checks.sh`: poisoned `status` (inline comment), bad
  `slug` (TAB and space), unrecognized `priority`, `title` with a pipe. Assert the exact
  `<check-id>\t<change-id>\t<message>` line.
- `priority:` empty asserts **no** finding (the documented default), guarding against an
  over-eager domain check.
- `board-row-dropped` suppression: a poisoned-`status` fixture asserts exactly one finding, not two.
- A TAB-in-`id` fixture asserting all three findings columns survive intact through
  `docket-status.sh`'s `IFS=$'\t' read`.
- The existing golden byte-compare and idempotence assertions (`tests/test_render_board.sh:227`,
  `:232`) must still pass unchanged after part 4 — order is the only hazard in the substitution and
  the golden catches an order slip on the first run.
- New: assert `render-board.sh`'s buckets are exactly the lib's array.
- New: every name in `DOCKET_STATUSES` yields a non-empty `emoji_for`, and every name in
  `DOCKET_STATUSES_ACTIVE` yields a non-empty `label_for_title` — the residual guard from part 4.
- Mutation-check: stripping each guard reddens at least one assertion.

## Out of scope

- Rejecting or rewriting the offending change files. This change makes the failure **visible**; it
  does not decide what a malformed file's canonical value should be.
- Broader frontmatter schema validation beyond what board rendering actually consumes.
- Restructuring `emoji_for` / `label_for_title` into data. They are pinned by test, not unified.
- The non-board slug derivations in `reclaim-claims.sh` and `archive-change.sh`.
