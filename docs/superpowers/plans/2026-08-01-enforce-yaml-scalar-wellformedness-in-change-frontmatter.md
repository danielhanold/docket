<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0191 — Enforce YAML scalar well-formedness in change-file frontmatter](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0191-enforce-yaml-scalar-wellformedness-in-change-frontmatter.md)**
<!-- docket:backlink:end -->

# Enforce YAML scalar well-formedness in change-file frontmatter — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: push through focused tests for each task, using TDD
> (red → green), and never rely on the whole-suite gate to tell you a task is done — the gate runs
> once at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a new `scalar-form` board-checks check-id that flags an **unquoted** change-file
frontmatter scalar carrying a `: ` (colon-space) or exactly matching a YAML 1.1 bare boolean keyword
(`on`/`off`/`yes`/`no`/`true`/`false`, case-insensitive), covering the only two free-text string
scalars docket reads that are not already gated — `title` (via `field_raw`) and the optional
`blocked_by:` (via a new anchored `fm_field_raw` helper). A scalar that OPENS with `"` or `'` is
well-formed by definition and must NOT flag (the 0190 quoted-title regression shape is the accept
case). Warn-only: no auto-fix, no reader rewrite, ADR-0062 stands.

**Architecture:** Pure Bash + tests + docs — a new probe block in `scripts/board-checks.sh`'s
per-file walk, a new read-side helper `fm_field_raw` in `scripts/lib/docket-frontmatter.sh` (the
`fm_field` twin that keeps surrounding quotes intact, anchored to the first `---…---` block per
ADR-0057, same inline-comment strip), and the well-trodden 13→14 `BOARD_CHECK_IDS` pinning across
its four surfaces. The covered field set is a **derivation** (free-text string scalars that are read
and not already shape/domain-gated), never a hand-listed "bad fields" enumeration; the natively
boolean fields (`trivial`, `auto_groomable`, `reconciled`) are deliberately NOT scanned — a bare
`true`/`false` there is *correct* well-formed YAML. The check inherits board-checks.sh's warn-only
posture, never marks `EXPLAINED`, and never touches `board-row-dropped` (a malformed scalar does not
drop a row). No ADR is produced (the change codifies AGENTS.md's house rule + ADR-0065's
already-"present and future" quote leg into a guard; spec Assumption 5).

**Tech Stack:** Bash 3.2/4.0-compatible shell (the repo's floor), the shared line-based YAML
readers (`field_raw`/`fm_field`/new `fm_field_raw`), the flat `tests/test_*.sh` suite run with the
configured `"$DOCKER_BASH_PATH" tests/test_<name>.sh`.

## Global Constraints

- **Repo instructions bind (`AGENTS.md`).** Never `producer | early-exiting-consumer` under
  `set -o pipefail` (the existing `has_finding` helper consumes from a here-string, not a pipe —
  new code must not reintroduce the pipe shape); `grep` for a leading `--` pattern declares `-e`/`-F
  --`; awk indent classes are `[^[:space:]]`; anchor a frontmatter-field edit to the first
  `---…---` block; quote any hand-authored YAML scalar carrying a colon-space or a boolean keyword;
  a guard is code — mutation-test it (strip what it guards, watch it redden) or it is decoration;
  key a guard on syntactic **shape**, never an enumerated list of spellings; never hand-list the
  sites of a literal you are gating; cross-references anchor on a symbol name or a verbatim-quoted
  clause, never a line number.
- **The raw view is lossless; the unwrapped view is not.** `field()`/`fm_field()` unwrap surrounding
  quotes (change 0138), so a quoted colon-space title is indistinguishable from a bare one *after*
  unwrapping. The check MUST read the raw token — `field_raw` for `title`, the NEW `fm_field_raw`
  for `blocked_by` — never `field()`/`fm_field()`.
- **The `blocked_by` read must be anchored (ADR-0057 + the `frontmatter-anchored-read` learning).**
  `blocked_by:` is optional; `field_raw` scans the whole file, so for a change that omits it while
  the body happens to open a `blocked_by:` line, an unanchored read returns body prose — a false
  finding on a clean file. Hence the new `fm_field_raw` (frontmatter-scoped, quotes kept). The
  non-negotiable fixture: a change that **omits** `blocked_by:` while its body opens a
  `blocked_by:` line must stay green (anchored → empty → skip leg).
- **Boolean-matching is case-insensitive (YAML 1.1).** `Yes`/`TRUE` are booleans even though
  AGENTS.md spells the rule lowercase. Skip leg first: quoted or empty is never inspected.
- **One finding per violated leg per field** (not one per field), with the shape-specific message
  wording (`': '` leg vs bare-boolean leg), consistent with `field-domain`'s one-voice messages.
- **The `BOARD_CHECK_IDS` pinning is four surfaces.** Adding `scalar-form` means: the array in
  `scripts/lib/docket-frontmatter.sh` (13→14), the `--help` header enumeration in
  `scripts/board-checks.sh`, the per-check section in `scripts/board-checks.md`, the `check
  <check-id>` report-line row in `scripts/docket-status.md`, and `tests/test_board_checks.sh`'s
  count pin (13→14) + its emitted-set compare (auto-adapts once the array grows).
- **Mutation-test every leg** (`guards-are-code`, `assert-detects-removal-not-replacement`): dropping
  any leg must redden its own fixture, and the mutation must be confirmed to have landed (`grep -c`
  before/after) before the red run is believed.
- **Portability.** The interactive shell's `grep`/`rg` is ugrep and accepts constructs BSD `grep`
  rejects; any new regex or grep construct must be re-verified under `/usr/bin/grep` (no `\b`, no
  `\<`). Pure-bash `case` is preferred over `grep -Eq` (see `docket_change_type_is_wellformed`).
- **TDD.** Write the failing assertions first (including the mutation tests), watch them redden
  against the pre-change tree, then build to green.

## File Structure

**New files**

| Path | Responsibility |
|---|---|
| *(none — no new files; the new helper and check live in existing files)* | |

**Modified files**

| Path | Change |
|---|---|
| `scripts/lib/docket-frontmatter.sh` | Add `fm_field_raw` (anchored raw twin of `fm_field` — quotes intact, same `---…---` scope, same inline-comment strip); refactor `fm_field` to delegate so the awk body is single-sourced; add `scalar-form` to `BOARD_CHECK_IDS` (13 → 14). |
| `scripts/board-checks.sh` | Add `scalar-form` to the `--help` header enumeration; add the new probe block in the per-file walk (three legs via `field_raw`/`fm_field_raw`; one emit per violated leg). |
| `scripts/board-checks.md` | New `**`scalar-form`**` per-check section (the derived field set, the three legs, the warn-only posture); the pinned-against-`BOARD_CHECK_IDS` paragraph is unchanged. |
| `scripts/docket-status.md` | Add `scalar-form` to the `check <check-id>` report-line row enumeration. |
| `tests/test_board_checks.sh` | `BOARD_CHECK_IDS` count pin 13 → 14; new `scalar-form` fixtures (red: colon-space title, bare-boolean title, colon-space `blocked_by`, bare-boolean `blocked_by`; green: quoted colon-space title (the 0190 accept shape), clean bare title, present+clean `blocked_by`, **absent `blocked_by` + body-prose line**, natively-boolean `trivial: true` untouched); mutation tests for each leg + the anchoring. |
| `tests/test_docket_frontmatter.sh` | New `fm_field_raw` assertions (quotes kept, absent key empty, body-prose fall-through empty, interior/unterminated quotes intact, comment strip). |

---

## Task 1 — Add the `fm_field_raw` helper (and keep `fm_field` byte-identical)

**Build profile:** economy

**Files:** `scripts/lib/docket-frontmatter.sh`, `tests/test_docket_frontmatter.sh`.

Move the existing awk body of `fm_field()` into a new `fm_field_raw()` that returns the **raw**
token with surrounding quotes INTACT (no `_docket_unwrap_quotes` call), keeping the same contract
fm_field already has: frontmatter-scoped to the first `---…---` block, the same inline-comment
strip (whitespace-preceded `#` to EOL), the same trailing-trim, empty when the key is absent.
Then re-implement `fm_field()` as `_docket_unwrap_quotes "$(fm_field_raw "$1" "$2")"` so the awk
body lives in exactly one place (single-source; the existing `fm_field` tests prove byte-identical
behavior). Match the function-name/doc-comment style of the existing readers. No other reader
changes shape.

Add assertions in `tests/test_docket_frontmatter.sh` beside the existing `fm_field` quote block:
- `fm_field_raw` preserves a double-quoted value (`"Comma, title"`) and a single-quoted value.
- `fm_field_raw` returns a bare value unchanged.
- `fm_field_raw` is empty when the key is absent from the first block.
- **`fm_field_raw` is empty when the key is ABSENT from frontmatter but a body line opens with it**
  (the anchored-read proof, ADR-0057 fixture discipline) — construct the fixture file accordingly.
- `fm_field_raw` leaves an interior quote and an unterminated open quote untouched.
- `fm_field_raw` strips the same inline-comment shape `fm_field` strips (a template `type:`-style
  line with a trailing `# comment`), and keeps a `#` not preceded by whitespace as part of the value.

Use TDD: write the new assert block first (red on the absent `fm_field_raw`), then add the helper to
green.

**Focused verification:** `"$DOCKET_BASH_PATH" tests/test_docket_frontmatter.sh` green (the new
asserts redden before the helper exists; all pre-existing `fm_field`/`field_raw` asserts stay green
after the refactor).

---

## Task 2 — Add the `scalar-form` check-ids + probe block (script side)

**Build profile:** standard

**Files:** `scripts/lib/docket-frontmatter.sh`, `scripts/board-checks.sh`,
`tests/test_board_checks.sh` (count-pin literal only).

1. In `scripts/lib/docket-frontmatter.sh`: add `scalar-form` to the `BOARD_CHECK_IDS` array —
   alphabetical position — making the count 13 → 14 (place it after `field-domain`).
2. In `scripts/board-checks.sh`: add `scalar-form` to the `--help` header enumeration (the
   `check-id ∈ {…}` comment block near the top).
3. In `scripts/board-checks.sh`'s per-file walk (a new block whose placement mirrors the
   `field-domain` block): read the raw tokens —
   `sf_title="$(field_raw "$f" title)"` and `sf_blocked_by="$(fm_field_raw "$f" blocked_by)"` — and
   for each covered field apply the three legs over the raw value:
   - **Skip leg:** empty, or the raw value opens with `"` or `'` (ADR-0065 quote leg — quoted is
     well-formed by definition; never inspect the interior). Pure-bash `case`, no grep.
   - **Colon-space leg:** the unquoted raw value contains `: ` → one finding.
   - **Boolean leg:** the unquoted raw value is exactly one of `on off yes no true false`
     case-insensitive (whole-value match; YAML 1.1) → one finding.
   Emit per violated leg per field, message naming the field and the shape, e.g.
   `title: unquoted scalar contains ': ' — quote it or reword (well-formed YAML)` and
   `blocked_by: unquoted bare YAML boolean (true) — quote it or reword (well-formed YAML)`.
   Do NOT mark `EXPLAINED`; do NOT touch `board-row-dropped`; keep warn-only (no exit-code change).
4. In `tests/test_board_checks.sh`: bump the `BOARD_CHECK_IDS` count-pin literal `13` → `14`
   (the "holds the 13 check-ids" assert). The emitted-set compare is already set-based and adapts
   automatically. Do NOT yet add the fixture block — that is Task 3.

**Focused verification:** `"$DOCKET_BASH_PATH" tests/test_board_checks.sh` green (the count pin
moves with the array in the same step so no intermediate red); `"$DOCKET_BASH_PATH"
tests/test_docket_frontmatter.sh` still green.

---

## Task 3 — `scalar-form` fixtures + mutation tests

**Build profile:** standard

**Files:** `tests/test_board_checks.sh`.

Add a new `=== scalar-form (change 0191) ===` fixture section mirroring the `field-domain` section's
conventions (each fixture its own `new_repo`, one change file per id, `read -r W O < <(new_repo)`,
run `board-checks.sh` against it, `has_finding` for assertions). Fixtures:

**Red (must fire):**
- unquoted `title` containing `: ` (the 0190 regression shape, unquoted — e.g. id 90) → exactly one
  `scalar-form` finding naming `title` and `': '`.
- unquoted `title` that is exactly a bare boolean (`title: yes`, id 91) → one `scalar-form` finding
  naming the boolean shape (case-insensitive: also cover `title: TRUE` under the same leg if
  practical).
- unquoted `blocked_by:` containing `: ` (id 92) → fires.
- unquoted `blocked_by:` equal to a bare boolean (id 93) → fires.

**Green (must NOT fire):**
- quoted `title` containing `: ` — `title: "a: b"` and single-quoted `'a: b'` (id 94) — the 0190
  **accept** case; the quote leg must keep it green.
- quoted `blocked_by` with a colon-space (id 95).
- clean bare `title` with no colon-space / boolean (id 96).
- present, well-formed `blocked_by` (id 97).
- **change that OMITS `blocked_by:` while its body opens a `blocked_by:` line** (id 98) — proves the
  `fm_field_raw` read is anchored and returns empty (the frontmatter-anchored-read fixture). An
  unanchored `field_raw` would return the prose and misfire; the natural "has the field" fixture
  passes under both implementations, so THIS is the load-bearing green.
- natively-boolean fields untouched: `trivial: true` (id 99) fires nothing (boolean-leg correctness).
- a quoted bare-boolean title `title: "yes"` (id 89) stays silent.

**Mutation tests (confirmed landed via `grep -c` before/after, run against throwaway copies):**
- strip the colon-space leg from `board-checks.sh` → the colon-space fixtures (90/92) go green.
- strip the quote skip (or the `-n "$sf_title"`/quote guard) → the QUOTED colon-space fixture (94)
  reddens — the wrong direction, proving the signature.
- replace `fm_field_raw` with `field_raw` for `blocked_by` → the absent-`blocked_by` + body-prose
  fixture (98) misfires (red), proving the anchoring.
- drop the whole probe block → every red fixture goes green.

**Focused verification:** `"$DOCKET_BASH_PATH" tests/test_board_checks.sh` green; every mutation
demonstrated red against a throwaway copy with the mutation confirmed landed.

---

## Task 4 — Docs: `board-checks.md` per-check section + `docket-status.md` row

**Build profile:** economy

**Files:** `scripts/board-checks.md`, `scripts/docket-status.md`.

In `scripts/board-checks.md`, add a `**`scalar-form`**` section in the established per-check idiom
(alongside the `field-domain` entry): purpose (well-formedness leg of the house yaml-scalar rule),
the **derived field set** (`title`, `blocked_by`; why not the natively-boolean or the
shape/domain-gated fields), the three legs over the raw token, the anchored `blocked_by` read via
the new `fm_field_raw`, the warn-only posture (never `EXPLAINED`, never `board-row-dropped`), and
one running example message. Keep the "pinned against `BOARD_CHECK_IDS`" paragraph as-is. Update the
header enumeration comment if it lists the check-ids.

In `scripts/docket-status.md`, add `scalar-form` to the `check <check-id>` report-line row
enumeration (the `Output contract` table row that lists the `board-checks.sh` finding ids, keeping
alphabetical order alongside `field-domain`).

**Focused verification:** `"$DOCKET_BASH_PATH" tests/test_board_checks.sh` and the suite's
doc-reader tests remain green; confirm no test pins the doc enumerations to a stale count.

---

## Task 5 — Build gate (not a worker task; run by docket-build)

After all plan tasks commit, docket-build runs the whole suite once and mints the build-evidence
record on green. The `scalar-form` check ships warn-only: after the merge, the next board pass
surfaces change 0121's unquoted colon-space `title:` as the check's first finding on real history —
the spec's named expected outcome (Assumption 6), not a regression. Record the corpus verification
(one expected finding: 0121; quoted colon-space titles such as 0190's stay green; no bare-boolean
titles in the corpus) in the results file at step 6.5.

---

## Post-build controller steps (run by docket-implement-next, NEVER worker tasks)

These are **metadata-branch writes** and are outside docket-build's scope; a worker must not attempt
them.

- **No ADR.** The spec's Assumption 5 stands (the change codifies an existing rule into a guard).
  If the reviewer reads the colon-space leg as a new decision, that is auto-captured follow-up, not
  a blocking ambiguity — it does not produce an ADR in this change.
- **Results file (step 6.5).** Author
  `docs/results/2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter-results.md`
  from `results-template.md` IN THE FEATURE WORKTREE, recording the corpus verification (exactly one
  expected finding — 0121; the quoted-title accept cases verified live on 0190), the mutation-test
  results for each leg, and the `BOARD_CHECK_IDS` 13→14 pinning.
- **`plan:` field, `status: implemented`, `pr:` — controller metadata writes on `metadata_branch`.**
