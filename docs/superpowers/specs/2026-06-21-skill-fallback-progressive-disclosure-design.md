# Per-script co-located contracts — design

- **Change:** #37 — Slim skills — move the per-skill manual-fallback / script-contract prose into on-demand sibling files
- **Date:** 2026-06-21
- **Depends on:** #34 — **done** (PR #45 merged 2026-06-21). `DOCKET_SCRIPTS_DIR`, fail-loud
  script resolution, and the bare-path static audit are landed, so this dependency is
  satisfied and #37 is build-ready now.
- **Status:** design (build-ready)

## Problem

Every docket `SKILL.md` carries detailed prose describing what its helper scripts *do*
internally — `docket-finalize-change` spells out a ~60-line "The mechanics" section for
`terminal-publish.sh`; `docket-convention` describes the config-resolution + bootstrap
contract; `docket-new-change` walks the `archive-change.sh` / `terminal-publish.sh` kill
steps. That prose is valuable — it is the human-readable spec of each script, and the
convention already asserts "the prose is the contract the script implements verbatim." But
it lives in **always-loaded** skill bodies, so it costs context on *every* invocation, and
it is the *only* readable form of behavior that otherwise lives in hard-to-read bash.

Two needs, currently served by one ball of in-body prose:

1. **Slim the hot path** — the script-internals prose should not load on every skill run.
2. **A durable, readable spec per script** — bash is the hard-to-read part; every script
   deserves an authoritative prose contract that future maintainers edit alongside it.

#34 makes the scripts reliably reachable (`DOCKET_SCRIPTS_DIR`) and switches the skills to
**fail loud** rather than silently hand-work — removing the *urgency* of the historical
"re-implement by hand if the script is absent" fallback, but not the prose itself. #34
explicitly leaves the prose in place and names this change (#37) as the follow-up that
relocates it.

## Decision

**Give every helper script a co-located, authoritative prose contract, and shed the
script-*internals* prose out of the skill bodies onto that contract.**

### 1. Per-script contracts, co-located with the scripts

- Each `scripts/<name>.sh` gets a co-located `scripts/<name>.md` — its authoritative
  contract — **scaled to the script's complexity** (full mechanics for `terminal-publish.sh`;
  a tight few lines for a thin wrapper). Coverage is **exhaustive**: every script gets one.
- Co-location is the primary drift defense: the script and its contract are edited together,
  in the same PR, in the same directory.
- Contracts are reachable from consuming repos via #34's `DOCKET_SCRIPTS_DIR`
  (`$DOCKET_SCRIPTS_DIR/<name>.md`) — the exact mechanism #34 builds for the scripts
  themselves. #37 is a tight complement to #34, not a separate progressive-disclosure scheme.

**Rejected alternatives.** *Per-script references in the `docket-convention/` skill dir*
(the `github-board-mirror.md` precedent) — rejected because a script's contract belongs next
to the script it documents (lower drift, edited together), and #34 already makes `scripts/`
reachable. *Per-skill `operations.md` appendix* — rejected because shared scripts
(`terminal-publish.sh`, `archive-change.sh`) would be restated across skills or cross-point to
each other, recreating today's tangle. *Keep inline, reframe only* — rejected because the
prose is genuinely heavy and loads on every invocation; the context win is real.

### 2. The naming convention is the pointer

State once, in `docket-convention`, the rule: **every `scripts/<name>.sh` has a co-located
`scripts/<name>.md` contract — read it for the script's internals.** This avoids scattering a
hand-written pointer across the ~65 call sites. A reader who wants the internals knows where
to look from the rule alone.

### 3. The body↔contract boundary

The crux of doing this without losing operational correctness:

- **Stays in the skill body** — the *operational* facts the skill needs to act:
  - when to call the script,
  - the command + the args it passes,
  - what it does with the exit code ("trust the exit code" / abort-and-report),
  - **ordering constraints between steps** (e.g. "archive on `docket` *before*
    terminal-publish", because terminal-publish copies the change file from `origin/docket`).
- **Moves to the contract** — the script's *internals*: how it does the work — worktree
  provisioning, copy-set assembly, idempotency / re-run safety, self-verify, the origin/HEAD
  repair sequence, mode guards, the `-B`/prune adoption of leaked branches, etc.

Test for each sentence: *does the skill need this to decide what to do next* (stays) *or does
it explain how the script accomplishes its job* (moves)?

### 4. The convention is special-cased

`docket-convention` is not a procedure body — it is the shared contract reference, and parts
of its script-related prose *are* the authoritative definition rather than a description of a
script's internals. Split it on that line:

- **Stays in the convention** — the *conceptual definitions* it owns: what the `.docket.yml`
  knobs mean, what the `BOOTSTRAP` verdicts mean, the bootstrap 2×2 *semantics* and probe
  definitions ("the contract the script implements"), the branch model, lifecycle. These are
  the spec, not the implementation.
- **Moves to `scripts/docket-config.md`** — the *script's interface and mechanics*: how
  `docket-config.sh --export` repairs `origin/HEAD`, reads `.docket.yml` authoritatively, the
  eval-able `KEY=value` output contract, fail-closed exit codes, and how it realizes the 2×2
  it is handed.

The convention keeps a one-line pointer per its existing style ("resolved deterministically by
`docket-config.sh`; see its contract") plus the global naming-convention rule from §2.

### 5. Contract file template

A light, uniform shape so contracts are scannable and authorable; collapse sections for
trivial scripts.

```
# <name>.sh — <one-line purpose>

## Purpose      — what it does and why it exists
## Usage        — invocation + flags/args
## Behavior     — the mechanics (scaled to complexity)
## Exit codes   — what 0 / non-zero mean (the "trust the exit code" contract)
## Invariants   — guarantees (idempotency, re-run safety, mode guards)
```

A trivial wrapper collapses to **Purpose + Usage + Exit codes**. A complex script
(`terminal-publish.sh`, `docket-config.sh`) uses the full set.

### 6. Drift discipline — existence check

A test-suite **static audit** (`tests/test_script_contracts_coverage.sh`, mirroring
`tests/test_change_links_coverage.sh` and the bare-path audit #34 adds) asserting
`scripts/*.sh` ↔ `scripts/*.md` match **1:1**:

- every `scripts/<name>.sh` has a `scripts/<name>.md` (catches a new script with no contract),
- every `scripts/<name>.md` has a live `scripts/<name>.sh` (catches an orphaned contract).

The repo has no `.github/workflows/` CI; the test suite is the de-facto gate. Content fidelity
(prose actually matching the bash) is left to co-location + review + the convention's "prose is
the contract" rule. Mechanical prose-vs-bash verification is **explicitly out of scope** — it
would be flaky and gameable.

Watch for non-`.sh`/non-script files already in `scripts/` (e.g. a `lib/` dir,
`scripts/lib/docket-frontmatter.sh`). The audit must scope precisely: top-level
`scripts/*.sh` ↔ `scripts/*.md`, deciding deliberately whether `scripts/lib/*.sh` helpers are
in or out of the 1:1 set (recommendation: include any `*.sh` that is an entry point; a sourced
`lib/` helper may be documented within its caller's contract — the builder pins this against
the actual tree).

## Scope / inventory

All ~14 scripts get a contract; the eight skill bodies lose their script-internals prose.
The reconcile pass should treat this list as a **floor**, not a ceiling — `ls scripts/*.sh`
at build time is the authoritative inventory.

Heaviest prose to relocate (highest context win):

- `terminal-publish.sh` — the ~60-line "The mechanics" + copy-set rules in
  `docket-finalize-change`; referenced from finalize, new-change, adr, implement-next.
- `docket-config.sh` — the config-resolution + bootstrap 2×2 in `docket-convention`
  (loads on *every* docket operation; the §4 split applies).
- `archive-change.sh` — the archive-move contract restated in finalize, new-change, status.

Lighter (tight contracts; bodies mostly keep just the invocation): `render-board.sh`,
`render-change-links.sh`, `render-adr-index.sh`, `github-mirror.sh`,
`cleanup-feature-branch.sh`, `sync-integration-branch.sh`, `board-checks.sh`,
`adr-checks.sh`, `sync-agents.sh`, `link-skills.sh`, `migrate-to-docket.sh`.

## Out of scope

- Rewriting any script's logic (behavior is unchanged).
- The `DOCKET_SCRIPTS_DIR` mechanism + fail-loud resolution + bare-path audit — that is **#34**.
- Mechanical content-sync verification of prose against bash.
- `docket-convention/github-board-mirror.md` — it is skill-reference (multi-script, board
  surface), not a single-script contract; it stays in the convention dir.

## Folded-in: harden `tests/test_consuming_repo_scripts.sh` (found at #34's merge gate)

A self-contained test fix, parked here because #37 already revisits the suite's sentinels.
#34's fail-loud assertions (the `${DOCKET_SCRIPTS_DIR:?…}` checks) run `bash -c '…'`
sub-shells that **inherit an exported `DOCKET_SCRIPTS_DIR`** — and #34's `install.sh` exports
exactly that into the dev shell's profile. So in any shell where docket is installed, those
sub-shells see the var set, `:?` never fires, and the three fail-loud assertions go NOT OK
even though the code is correct (observed live finalizing #34; a clean-env
`env -u DOCKET_SCRIPTS_DIR` re-run was all-green). **Fix:** have the test's fail-loud
sub-shells run `env -u DOCKET_SCRIPTS_DIR bash -c '…'` so they exercise the unset path
regardless of the ambient environment.

## Dependency & ordering

**#34 is done** (PR #45 merged) — the bodies already invoke `$DOCKET_SCRIPTS_DIR/<name>.sh`
and #34's bare-path audit is in place, so the dependency is satisfied and #37 is build-ready
now. #37 then:

1. authors `scripts/<name>.md` for every script (template §5, scaled to complexity),
2. strips the script-internals prose from the eight bodies, leaving operational invocation,
3. applies the §4 convention split + states the naming-convention rule (§2),
4. adds the existence audit (§6),
5. lands the folded-in `test_consuming_repo_scripts.sh` hardening (above).

The implementer's reconcile pass re-validates against #34's *as-landed* form (the exact
pointer syntax, the audit file's location/name) before building.

## Testing

- The new `tests/test_script_contracts_coverage.sh` existence audit (§6).
- The folded-in `tests/test_consuming_repo_scripts.sh` hardening — verify the three fail-loud
  assertions are GREEN both in a docket-installed shell (where `DOCKET_SCRIPTS_DIR` is
  exported) and in a clean env, after switching the sub-shells to `env -u DOCKET_SCRIPTS_DIR`.
- The existing wiring-sentinel / test suite must stay green after the body edits — the same
  suite #34 touches (`test_change_links_coverage`, `test_docket_config`, `test_render_board`,
  `test_closeout`, `test_adr_checks`, `test_render_adr_index`, `test_board_checks`). Stripping
  internals prose must not remove a literal substring those sentinels grep for; audit in
  lockstep, as #34 did.

## Risks

- **Operational prose accidentally moved to a contract** → a skill loses a fact it needs to
  act correctly. Mitigation: the §3 boundary test, and the implementer reviews each body diff
  for retained ordering/invocation facts.
- **The convention over-slimmed** (§4) → a conceptual definition that *is* the contract gets
  pushed into a script's mechanics file. Mitigation: keep verdict/probe/2×2 *semantics* in the
  convention; only the script's *realization* moves.
