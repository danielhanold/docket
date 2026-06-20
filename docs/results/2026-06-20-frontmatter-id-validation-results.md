# Validate numeric `id` across the frontmatter script family — results
Change: #32 · Branch: feat/frontmatter-id-validation · Plan: docs/superpowers/plans/2026-06-20-frontmatter-id-validation.md · ADRs: none (governed by ADR-0012)

## Findings

- **`int_field` omits the trailing newline that `field` emits — and that bit one direct-pipe site.**
  `field` ends `printf '%s\n'`; the new `int_field` ends `printf '%s'`. Every adoption is a
  command substitution `id="$(int_field …)"` (which strips trailing newlines anyway) **except**
  `render-board.sh`'s done-node list, a *direct pipe* (`… && field "$f" id; done | sort -n`) that
  relied on the newline as the record separator. A naive swap there concatenated ids (`10`,`12` →
  `1012`). Fixed by capturing and re-emitting: `{ v="$(int_field "$f" id)"; [ -n "$v" ] && printf '%s\n' "$v"; }`.
  Apply: when replacing `field` with `int_field` (or any accessor that drops the trailing newline),
  audit for *un-captured* uses in a pipe/concatenation — command-substitution sites are safe, raw-pipe
  sites are not.

- **A validator that silently skips malformed input hides the very inconsistency it exists to report.**
  The by-role split is the load-bearing design call: renderers + the shared `resolve_deps`/`readiness`
  scan **skip** a malformed id (so one bad file never blanks the board/index), but `board-checks.sh` /
  `adr-checks.sh` **emit a first-class warn-only `malformed-id` finding** — because skipping there would
  make the health check complicit. `adr-checks.sh` also had to relocate its `emit`/`FINDINGS` definition
  *above* the scan loop (the finding is now emitted from within it) and skip the malformed file *before*
  the `[ "$id" -gt "$MAXID" ]` arithmetic (which previously would have emitted `integer expression
  expected` on a non-integer id under `set -u`).

- **Reconcile widened the surface map the spec enumerated.** The spec listed the id-read sites
  illustratively and told the builder to `grep` them all; reconcile pinned two the spec under-named —
  `readiness()` (lib L71, a *third* lib read beyond `resolve_deps`' two passes) and `board-checks.sh`'s
  dep-cycle `cid` (L92). Both were hardened. Apply: treat a spec's site list as a floor, not a ceiling —
  `grep` the whole family before editing.

## Follow-ups

- **Other numeric frontmatter fields (`depends_on`, `adrs:`, `change:`) are unguarded** — the same
  `int_field` technique applies, but this change is scoped to `id:` per the spec. A natural next change
  if a malformed cross-reference ever surfaces; not pursued speculatively (YAGNI).
- **`terminal-publish.sh` had no dedicated test before this change** — this change adds
  `tests/test_terminal_publish.sh` covering only the new arg guard (no git). Its broader git behavior is
  still only exercised indirectly (via `test_closeout.sh` / `test_docket_metadata_branch.sh`); a fuller
  functional test is out of scope here.
