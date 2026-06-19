# Extract inline board rendering into a deterministic script — results

Change: #22 · Branch: `feat/render-board-script` · PR: _(opened at close-out — see `pr:` on change 0022)_ · Plan: `docs/superpowers/plans/2026-06-18-render-board-script.md` · ADRs: none

## Verify (human)

The automated tests plus a live smoke test (the new renderer run against the real backlog) cover the change; no manual gate is required. Two optional spot-checks at merge:

- [ ] The renderer was run against the **live** backlog during the build — output was byte-stable, idempotent, clean stderr, exit 0. It is **not** wired to overwrite `BOARD.md` until merged; the first `docket-status` after merge will regenerate `BOARD.md` via the script.
- [ ] That first post-merge render will **re-sort the live board into canonical order** (proposed before implemented). The current `BOARD.md` renders implemented-before-proposed — a pre-existing model-render drift the deterministic script corrects. Expect that one-time reordering; it is the change working as intended.

## Findings

- **The shared resolver populates four arrays, not the spec's three.** `resolve_deps` adds `DEP_ON[id]` (the bare id of the worst unmet dependency) alongside `STATUS_OF`/`DEP_STATE`/`DEP_REASON`, so the board can render `⏳ waiting on #N` with the id and reason chosen in the **same** walk (they can never disagree). This is the interface **0023 and 0024 build against**. The whole-branch reviewer judged it self-evident enough to need **no ADR** (an obvious display sibling of `DEP_REASON`); it is documented inline in `scripts/lib/docket-frontmatter.sh` and in the plan.

- **The live smoke test caught two real-data bugs the golden fixture missed** — both fixed and now locked by tests:
  1. **`pr:` is a full URL, not a bare number.** Every real change stores `pr: https://github.com/<owner>/<repo>/pull/<n>`; the first `pr_cell` assumed a bare number and double-wrapped the URL. Fixed to render `[#<n>](<url>)` from the URL (bare-number fallback kept). The fixture had used `pr: 142`, so the golden never exercised the real convention.
  2. **`field()` lost its trailing newline in the SIGPIPE fix.** The SIGPIPE-141 fix (removing `sed | head -n1`) replaced the final `sed` with `printf '%s'`, dropping the newline the original emitted. Every `$(field …)` caller masks this (command substitution strips trailing newlines), but the mermaid done-node loop pipes `field "$f" id` **directly** into `sort` — so with the newline gone, all done ids concatenated into one number passed to `pad` (`printf: Result too large`). Only reproducible with **≥2 done changes**; the fixture had one. Fixed by restoring `printf '%s\n'` (the original behavior); the fixture gained a second done change so the golden locks multi-done output.

- **`render-board.sh` has no wrong-tree guard** (unlike `github-mirror.sh`). The whole-branch reviewer judged the absence acceptable: it emits to stdout (the caller redirects), `docket-status` always points `--changes-dir` at the metadata tree, and a mis-pointed run yields a thin-but-valid board that self-heals next run — not corrupted external state.

## Follow-ups

- **0023** (scripting decision for the merge sweep + health checks) and **0024** (retire the inline board/source-drift check) are unblocked: both consume `scripts/lib/docket-frontmatter.sh` (`field`/`list_field`/`has_section`/`resolve_deps`/`readiness`) and the deterministic render this change ships.
- **LEARNINGS harvest candidates** (for close-out):
  - A "byte-identical" claim for a shell helper refactor must be validated against a **direct-pipe** caller, not only `$(…)` callers — `$()` silently strips a dropped trailing newline, hiding the regression.
  - A golden/list fixture must include **plurality** (≥2 of any kind it renders as a list) or concatenation/separator bugs stay invisible — here both a single-PR-form and a single-done-change hid real bugs until the renderer met real data.
  - Run a new deterministic renderer against **real data** before merge — the fixture is necessary but not sufficient; production field conventions (full-URL `pr:`, 14 done changes) exposed two bugs a hand-authored fixture did not.
