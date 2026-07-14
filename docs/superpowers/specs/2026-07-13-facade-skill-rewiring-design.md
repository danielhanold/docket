# Facade skill rewiring — retire the eval preamble (#0072)

**Date:** 2026-07-13
**Change:** #0072 `facade-skill-rewiring`
**Depends on:** #0068 (done — `scripts/docket.sh` facade, PR #78, merged 2026-07-14)
**Related:** #0073 (Cursor sandbox & permissions guide — consumes this change's two-command-shape surface)

## Problem

Change 0068 shipped the finite executable facade (`docket.sh`: 13 operations, a `preflight`
verb performing all Step-0 side effects, raw `KEY=value` config on stdout), but the seven
operating skills and the convention's *Step-0 preamble* still instruct agents to build the old
shapes: `eval "$(docket-config.sh --export)"`, inline worktree ensure + hook disable, inline
`fetch`/`pull --rebase` programs, and direct per-helper invocations (~36 sites across skill
prose; 8 files carry the eval spelling). Until the prose moves, the facade is unused and the
permission surface is unchanged.

## Design

### 1. The new Step-0 preamble (convention SKILL.md)

Three steps:

1. Load the convention (blocking, unchanged).
2. Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` **as its own Bash
   call** — never compounded with other commands, because the model must read the printed
   `KEY=value` block off stdout.
3. Read the block and carry the values forward **in context**, interpolating them as literals
   into later commands.

No `eval`, no `source`, no inline worktree/hook/fetch-pull programs. `preflight` performs all
former Step-0 side effects (bootstrap verdict fail-closed, metadata worktree ensure + hook
disable, fetch + `pull --rebase`; main-mode degradation internal) and prints the env block on
success.

**Failure handling.** `preflight` exits non-zero with a stderr diagnostic on any verdict other
than `PROCEED`:

- `STOP_MIGRATE` → refuse and point at `migrate-to-docket.sh` (human-initiated tier; prose
  reference, never a runtime invocation).
- `CREATE_ORPHAN` → the **one sanctioned direct-helper spelling**, byte-exact:
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --bootstrap`
  (fresh-repo-only, once per repo, human-attended) — then re-run `preflight`. This carve-out
  exists because the facade deliberately does not expose `docket-config.sh` and `preflight`
  fails closed, so no facade spelling can perform the orphan-create write. It lives in exactly
  one place: the convention's Step-0 preamble. A future `bootstrap` facade verb is a candidate
  follow-up stub, deliberately NOT part of this change (no facade behavior change here).

The convention keeps a **prose description** of what `preflight` does (worktree ensure, hook
disable, sync, the main-mode degradation) — describing mechanics is fine; only *instructing
inline programs* is retired. The tokenizer (§4) judges code spans, not prose sentences.

### 2. Value interpolation convention

- Skill prose keeps its `<changes_dir>`-style placeholders, now **defined once in the
  convention** as: "the corresponding KEY from the most recent `preflight`/`env` block,
  interpolated as a literal."
- Metadata paths compose per ADR-0029: `<METADATA_WORKTREE>/<CHANGES_DIR>` (absolute root +
  repo-relative subpath); same for `ADRS_DIR`. `RESULTS_DIR` composes against the feature
  worktree, never the metadata worktree.
- Shell-variable reads in prose (`"$BOARD_SURFACES"`, `$SKILL_BRAINSTORM`, `$SKILL_PLAN`,
  `$SKILL_BUILD`, `$SKILL_REVIEW`, `$SKILL_FINISH`) become literal-interpolation references to
  the block's keys — a shell variable set by an eval that no longer exists must not survive in
  any command.
- **Coverage verified at groom time:** every config value skill prose reads is emitted by
  `env`/`preflight` (19 keys). `GITHUB_PROJECT`/`AGENT_HARNESSES` are never read by skill
  prose (their consumers — `github-mirror.sh`, `sync-agents.sh` — self-resolve config
  internally). Reconcile re-verifies by grep before build.

### 3. Invocation rewiring

**Files in scope (live agent-facing prose only):** the seven operating skills'
`SKILL.md` (docket-new-change, docket-groom-next, docket-implement-next, docket-status,
docket-finalize-change, docket-adr, docket-auto-groom), `docket-convention/SKILL.md`, and
`docket-convention/references/terminal-close-out.md`. `references/agent-layer.md` is rewired
only if the build-time grep finds old shapes in it. The full site inventory is derived by
whole-repo grep at reconcile/build time (LEARNINGS: never hand-list; the counts above are a
floor, not the set).

**Rules:**

- Every daily-helper invocation becomes the canonical spelling
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <op> [args...]` with op = helper
  basename and args unchanged (mechanical edit).
- **All** metadata-tree sync instructions — pre-read syncs AND the push-retry CAS loops —
  become "re-run `docket.sh preflight`" (for CAS: "re-run `preflight`, then retry the push").
  One sanctioned sync verb everywhere; no per-site exception list for the wiring test to
  maintain.
- Plain git plumbing stays direct and unrestricted: `git add`/`commit`/`push` (including
  `git -C` forms), feature-branch git, `gh` calls. The facade covers docket helpers only.
- `disable-worktree-hooks.sh` and `render-board.sh` drop out of runtime prose entirely
  (internal to `preflight` / `board-refresh` respectively).

**Out of the rewiring:** script contracts (`scripts/*.md` — they document script internals,
including the resolver's own `--export`/`--format shell` interface), README-level docs, and
all immutable artifacts (archive, specs, plans, results, ADRs — none carry live instructions).

### 4. Wiring tests — tokenizer + unique anchors

New `tests/test_skill_facade_wiring.sh`, two layers, every assert mutation-tested
(strip the guarded clause → the test must go red; per the LEARNINGS guards-are-code family).

**Layer 1 — absence sweep (strip-then-scan, judged per command, never per line):**
over every live skill markdown file (`skills/**/SKILL.md` + `skills/docket-convention/references/*.md`):

1. Extract the judgeable units: fenced code blocks + inline code spans.
2. Assert every facade invocation matches the canonical spelling **byte-exactly**, and every
   `docket.sh <op>` uses an op ∈ the inventory **grep-derived from `scripts/docket.md`'s
   Subcommand inventory table** (never hand-listed — same derivation the facade's own sentinel
   uses).
3. **Strip** the canonical matches and the byte-exact `CREATE_ORPHAN` bootstrap carve-out from
   the working copy. (This is what makes the scan sound: the canonical spelling itself contains
   `install.sh` in its `:?` guard, so the legitimate needle must be removed before hunting the
   haystack.)
4. In the remainder: any `*.sh` token inside a code span is a violation — the human-initiated
   tier (`install.sh`, `migrate-to-docket.sh`, `sync-agents.sh`) is permitted in prose position
   only, never as a code-span invocation; and the shapes `eval "$(`, `fetch origin`,
   `pull --rebase` are forbidden in code spans.
5. Assert the bootstrap carve-out occurs exactly once repo-wide in skill prose (in the
   convention's Step-0 preamble) — a second copy is a violation.

**Layer 2 — presence anchors** (unique-phrase, `grep -c == 1`, byte-offset ordering where two
anchors can share a paragraph):

- The convention's Step-0 preamble instructs running `preflight` as its own call and reading
  the printed block.
- Mid-run re-sync is documented as "re-run `preflight`".
- The CREATE_ORPHAN carve-out is present (its uniqueness is Layer 1's job).

A pure absence sweep cannot detect a deleted-rather-than-rewritten section; the anchors close
that hole.

**Existing sentinel migration.** ~23 existing test files grep skill prose; many anchor old
spellings (`docket-config.sh --export`, direct helper paths, inline fetch/pull phrases). The
build derives every affected anchor by whole-repo grep and **follows the call** to the new
spellings — narrowing an assert to its load-bearing property where the new prose legitimately
violates an old absolutist form, never deleting or loosening it (LEARNINGS 2026-07-13 #64 and
the 0063→0068 hooks-audit precedent).

## Decisions (with rejected alternatives)

1. **Wiring test = tokenizer + unique anchors.** Rejected: per-file sentinel greps (the exact
   shape the guards-are-code LEARNINGS family shows shipping green while guarding nothing —
   wrong anchor, double-guarded, wrong unit); rejected: sweep-only (proves absence, not
   presence).
2. **All metadata syncs route through `preflight`,** including push-retry CAS loops. Rejected:
   pre-read-only replacement — it forces the wiring test to recognize a legitimate inline
   retry-loop shape, a hand-enumerated per-site exception list.
3. **CREATE_ORPHAN keeps `docket-config.sh --bootstrap` as a byte-exact carve-out.** Rejected:
   adding a `bootstrap` facade op in this change (contradicts the stub's no-facade-change
   boundary; mixes script engineering into a prose migration); rejected: filing the follow-up
   stub now (grooming never mints ids — noted as a candidate for a later capture instead).
   The carve-out never needs to be in Cursor's auto-run allowlist: it is once-per-repo and
   human-attended, like the install tier.
4. **Canonical spelling is 0068's, unchanged:**
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <op> [args...]` — already
   test-enforced on the emit side; this change makes the consume side match.

## Testing summary

- New `tests/test_skill_facade_wiring.sh` (Layers 1+2 above), mutation-tested.
- Updated anchors in the existing skill-prose test files (follow-the-call, derived by grep).
- The whole suite runs at the merge gate, never only the enumerated tests (LEARNINGS
  2026-07-10/11 goal-scoped-rewrite family).

## Out of scope

- Any facade or helper behavior change (0068 owns the facade; a `bootstrap` verb is a possible
  future stub).
- The Cursor guide and published permission fragment (#0073).
- Changing what the skills do — only how their shell surface is expressed.
- Script contracts, README, and immutable historical artifacts.
