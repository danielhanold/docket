# docket command facade — results

Change: #68 · Branch: feat/docket-command-facade · Plan: docs/superpowers/plans/2026-07-13-docket-command-facade.md · ADRs: 29

## Verify (human)

<!-- Automated suite is green (full tests/ suite, hermetic; no CI — the suite is the gate). These are
     the judgment checks only a human should make before merging. -->
- [ ] The facade is a **permission trust boundary**: confirm `scripts/docket.md`'s subcommand
      inventory table lists exactly the operations you intend to be runnable through
      `docket.sh` before you (or Cursor) allowlist it. The grep-derived, mutation-tested sentinel
      keeps `docket.sh` ↔ `docket.md` in sync, but the *contents* of that allowlist are a human
      trust decision.
- [x] Sanity-run against this clone at the merge gate: `scripts/docket.sh env` prints raw
      `KEY=value` with an absolute `METADATA_WORKTREE`; `scripts/docket.sh definitely-not-an-op`
      exits 2 and lists the supported operations; `scripts/docket.sh preflight` re-syncs `.docket`
      and prints the env block.
      **Run post-merge on `main` (f5a4e54), all three as specified:** `env` printed 20 raw
      `KEY=value` lines with `METADATA_WORKTREE=/Users/homer/dev/docket/.docket` and
      `BOOTSTRAP=PROCEED`; the unknown op exited 2 listing the 13 supported operations;
      `preflight` re-synced `.docket` and printed the env block (exit 0).

## Findings

- **Three spec ambiguities resolved during the build → ADR-0029** (`docs/adrs/0029-docket-facade-routing-and-config-presentation.md`):
  1. **Pure pass-through dispatch.** Wrapped helper ops forward args verbatim and `exec` the
     helper; they do NOT self-preflight. The shared preflight is realized only in the `preflight`
     verb and in `docket-status.sh`. Resolving the spec's self-sufficiency bullet toward the
     binding "routing boundary, not a second implementation" constraint (per-op preflight would
     double-sync after Step-0 and misfire for primary-tree ops). Consequence: wrapped metadata
     ops assume the caller ran `preflight` at Step-0 — the pre-0068 contract, preserved.
  2. **`env`/`preflight` absolutize only `METADATA_WORKTREE`**; the `*_DIR` keys stay
     repo-relative subpaths (their correct absolute root differs by consumer — metadata worktree
     vs feature worktree). A disclosed narrowing of the spec's "path-valued keys are absolute".
  3. **Raw presentation lives in `docket-config.sh --format plain`** (one key list, DRY;
     `--format shell`/`%q` stays the default, byte-unchanged for existing `eval` callers). The
     whole-branch reviewer judged all three sound (deviation 3 "arguably better than the spec").

- **Sentinel false-pass holes found and fixed in review (LEARNINGS #64 territory).** The
  inventory sentinel shipped two structural holes that only mutation-testing exposes:
  - The escape-hatch check looked for the literal token `exec-op` (a typo for `exec`), anchored
    `^\s*NAME)` so it missed pipe-combined case arms (`run|shell|eval)`), and its eval-regex
    missed input laundered through a variable. Fixed (commit `0d69041`): `run exec shell eval`
    tokens, a `(^|\|)…(\)|\|)` arm pattern, and a comment-stripped "never calls the eval builtin
    anywhere" scan; mutation-verified.
  - The sentinel derived the op set from the `WRAPPED_OPS` array, proving "WRAPPED_OPS matches
    the doc" but not "the dispatch matches the doc" — a `case` arm hand-added outside the
    WRAPPED_OPS loop would route while set-equality still held. Fixed (commit `f4a3f88`):
    assert the dispatch `case` block contains only the known arms; mutation-verified a rogue
    `deploy-secret)` arm reddens. **Apply-worthy lesson:** a structural sentinel over a
    single-source list (here `WRAPPED_OPS`) does not by itself guard the *surface that consumes*
    that list (the `case` dispatch) — assert the consuming surface directly, and mutation-test
    every invariant, not just the ones the plan enumerated.

- **Behavior-preserving refactor caught by a structural test.** Extracting `docket-status.sh`'s
  private worktree-sync into `scripts/lib/docket-preflight.sh` moved the `disable-worktree-hooks.sh`
  call site, reddening `test_worktree_hooks_wiring.sh` (change 0063's audit). Fixed (commit
  `5ceb257`): the audit now follows the call into the shared lib and additionally covers the new
  facade worktree-creation site. The delegated stderr prefix shifted `docket-status:` →
  `docket-preflight:` (no test/contract depended on the old literal).

## Follow-ups

- **#0072 (facade-skill-rewiring, proposed, depends on this)** — rewire the seven operating
  skills and the convention's Step-0 preamble to `preflight` + literal interpolation; add the
  wiring tests (no inline `eval`/`if`/worktree/fetch-pull in skill prose; facade-only runtime
  invocations). This change deliberately did NOT touch skill prose — the facade exists but the
  skills still call helpers directly until 0072.
- **#0073 (cursor-sandbox-permissions-guide, proposed, depends on this)** — the Cursor
  permissions/sandbox guide + the copyable permission fragment matching the canonical
  `docket.sh <op>` spelling.
- **Minor items accepted as-is** (whole-branch review triage, non-blocking): redundant
  `GIT="${GIT:-git}"` in `docket.sh`; unused `runf()` helper and a comment naming the removed
  `ensure_and_sync_worktree` in the tests; `md_ops` regex is whitespace-brittle but fail-safe
  (drift reddens set-equality, never false-passes).
