# Design: extract config resolution + bootstrap guard into a deterministic script (change 0026)

**Status:** design (brainstormed 2026-06-19)
**Change:** 0026
**Depends on:** — (foundational; independent of 0025)
**Precedent:** 0011 (`github-mirror.sh`, ADR-0007) · 0022 (`render-board.sh`) · 0025 (close-out scripts) — same extraction pattern.
**Governs:** ADR-0002 (docket-mode default; refuse-and-migrate bootstrap) defines the semantics this script implements verbatim.

---

## 1. Context

Every docket skill opens with the **same startup boilerplate**, today written as prose
each skill's model re-executes turn-by-turn:

1. **Config resolution** — repair `origin/HEAD` (`git remote set-head origin -a` →
   resolve `git symbolic-ref refs/remotes/origin/HEAD`), then read `.docket.yml`
   authoritatively (`git show origin/HEAD:.docket.yml` after a fetch) and resolve every
   knob (`metadata_branch`, `integration_branch` with its `auto → origin/HEAD → main`
   resolution, the dirs, `finalize.gate`, `board_surfaces`, …).
2. **Bootstrap guard** (docket-mode) — fetch, evaluate the `DOCKET`/`LIVE` 2×2, and
   decide: proceed, **STOP** and point at `migrate-to-docket.sh`, or create the empty
   orphan `docket` branch on a fresh repo.

This is pure, judgment-free, deterministic work that runs at the front of **all 7
skills** on **every** invocation — so its model-token cost is paid more often than any
other block in docket. It is also **subtle**: the convention itself flags the `LIVE`
probe traps (probe *only* the pruned surface; use `git ls-tree`, never bare
`<ref>:<path>`; an unreachable `origin/HEAD` is a hard error, **not** `¬LIVE`; abort
keys on the `set-head`/fetch return code, never on `git show`, which a cached ref lets
succeed with stale bytes). Re-deriving that correctly from prose every session is both
costly and a real failure surface. One tested script removes both problems — the same
extraction already applied to the mirror (0011), the board (0022), and close-out (0025).

## 2. Guiding principle (ADR-0007, applied)

> Mechanical, judgment-free startup work that runs on model tokens moves into a
> **deterministic, idempotent, fixture-tested script**; the skill consumes its output
> and acts only on the verdicts that need judgment.

The **semantics** are unchanged and already specified by **ADR-0002** (docket-mode is
the default; the refuse-and-migrate bootstrap) and the convention's *Configuration* and
*Bootstrap guard* sections — this script is their faithful implementation, not a
redesign. No new ADR required.

## 3. Output contract (read-only by default)

`docket-config.sh` is invoked once at a skill's startup and emits **eval-able
`KEY=value` lines** the skill consumes in a single turn, replacing the multi-step prose
resolution:

```
$ eval "$(docket-config.sh --export)"
# resolved config (examples):
DOCKET_MODE=docket                 # docket | main
DEFAULT_BRANCH=main                # origin/HEAD
METADATA_BRANCH=docket
INTEGRATION_BRANCH=main            # auto → origin/HEAD → main, or explicit
METADATA_WORKTREE=.docket          # the metadata working tree (.docket in docket-mode; '.' in main-mode)
CHANGES_DIR=docs/changes
ADRS_DIR=docs/adrs
RESULTS_DIR=docs/results
FINALIZE_GATE=local
FINALIZE_TEST_COMMAND=
BOARD_SURFACES=inline
AUTO_GROOM=false
BOOTSTRAP=PROCEED                  # PROCEED | STOP_MIGRATE | CREATE_ORPHAN
```

**Read-only by default.** The default `--export` invocation only *reads* (and the
benign local `set-head`/`fetch`); it never mutates branches. The bootstrap guard is
*evaluated* and reported as the `BOOTSTRAP=` verdict — it is not acted on:

- `PROCEED` — migrated repo (or main-mode); the skill continues.
- `STOP_MIGRATE` — existing single-branch or half-migrated repo; the skill **aborts and
  points at `migrate-to-docket.sh`** (the one judgment-adjacent outcome — a human must
  run the migration; never auto-created or auto-moved).
- `CREATE_ORPHAN` — fresh repo (`¬DOCKET ∧ ¬LIVE`); the orphan-create **write** is
  **opt-in**: either the skill performs it, or `docket-config.sh --bootstrap` does it
  explicitly. The default path stays read-only, so the lone write in the whole helper
  is never implicit.

`--export` shell-escapes values (paths/commands may contain spaces) so `eval` is safe.

## 4. Components

A single `scripts/docket-config.sh` with three internal stages, sharing one fetch:

1. **Resolve `origin/HEAD`** — `git remote set-head origin -a`; resolve the symbolic
   ref. Unresolvable / `origin` unreachable ⇒ **hard config error** (non-zero, clear
   message) — *not* a silent default. This keys on the `set-head`/fetch return code,
   never on `git show`.
2. **Read + resolve `.docket.yml`** — authoritatively via `git show origin/HEAD:…`;
   apply all defaults (`metadata_branch: docket`, `integration_branch: auto`, the dirs,
   `finalize.gate: local`, `board_surfaces: [inline]`, …); resolve `integration_branch:
   auto → DEFAULT_BRANCH → main`; derive `DOCKET_MODE` and `METADATA_WORKTREE`. A
   genuinely absent file with a resolvable `origin/HEAD` ⇒ all defaults (not an error).
3. **Bootstrap guard** (only when `DOCKET_MODE=docket`; a no-op verdict `PROCEED` in
   main-mode) — evaluate `DOCKET` (branch exists, origin or local) and `LIVE` (the
   pruned live-surface probe via `git ls-tree origin/<integration> -- …`), map the 2×2
   to a verdict. `--bootstrap` additionally performs the `CREATE_ORPHAN` write (create
   the empty orphan `docket`, push) — the only mutation, opt-in and guarded to the
   `¬DOCKET ∧ ¬LIVE` cell.

The script is **fail-closed** (per 0025's verification model): it exits non-zero with a
diagnostic on any **hard error** — unreachable `origin`, unresolvable `origin/HEAD`, an
unparseable `.docket.yml`. `STOP_MIGRATE` and `CREATE_ORPHAN` are **valid verdicts
emitted with exit 0** (the config resolved fine; the repo state is the skill's to act
on), so a skill can `eval` the output, trust that a zero exit means the config is
sound, then branch on `BOOTSTRAP=`.

## 5. What the skills keep owning

- **Acting on `STOP_MIGRATE`** — surface the migration instruction and abort (the
  refuse-and-migrate contract; a human runs `migrate-to-docket.sh`).
- **The migration itself** — `migrate-to-docket.sh`, unchanged and separate.
- **Whether to opt into the `CREATE_ORPHAN` write** — the skill calls `--bootstrap` (or
  performs the create) when it intends to initialize a fresh repo.
- Everything downstream — the skills consume the resolved `KEY=value`s exactly as they
  use the hand-resolved values today; only the *resolution step* moves into the script.

## 6. Out of scope

- **The close-out scripts** (0025), the **board / sweep / health checks** (0022 / 0023),
  the **`github` surface** (already scripted) — separate extractions.
- **`migrate-to-docket.sh`** — the migration tool is unchanged; this script only
  *detects* the states that point at it.
- **Any config or bootstrap *semantics*** — defaults, the `auto` resolution, the 2×2
  cells, and the error distinctions are reproduced exactly from ADR-0002 + the
  convention, not changed.
- **The `agents:` block** — consumed by `sync-agents.sh` at install time, not at skill
  startup; out of the runtime resolver's scope.

## 7. Testing

`tests/test_docket_config.sh`, matching the existing script tests, with **hermetic
local fixtures** (temp repos + bare origins, no network):

- **Resolution permutations:** absent `.docket.yml` (all defaults), explicit
  `metadata_branch: main` (main-mode → `METADATA_WORKTREE=.`), `integration_branch:
  develop`, `integration_branch: auto` (→ `origin/HEAD`), `board_surfaces` variants,
  a `finalize.test_command` override, values containing spaces (escaping).
- **Bootstrap 2×2:** all four cells assert the right verdict — `PROCEED` (migrated),
  `STOP_MIGRATE` (existing single-branch **and** half-migrated), `CREATE_ORPHAN` (fresh)
  — and that `--bootstrap` creates the orphan only in the fresh cell.
- **Error paths (fail-closed):** unreachable `origin` / unresolvable `origin/HEAD` exits
  non-zero; a cached-but-stale `origin/HEAD` does **not** mask an unreachable origin
  (the return-code-not-`git show` rule).

## 8. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Broad blast radius — consumed by all 7 skills, so any bug affects everything | The argument *for* one tested implementation over 7 prose copies that each drift; heaviest fixture coverage in the suite |
| The lone orphan-create write fires wrongly | Opt-in (`--bootstrap`), guarded to the `¬DOCKET ∧ ¬LIVE` cell, pinned by a per-cell test |
| Subtle probe/error semantics lost in translation | Ported verbatim from the convention + ADR-0002 and locked by the error-path tests (stale-ref, unreachable-origin, half-migrated) |
| Skills drift in *how* they consume the output | One documented `KEY=value` contract; skills `eval --export` rather than re-parsing |
