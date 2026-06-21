# Per-script co-located contracts — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every top-level `scripts/<name>.sh` a co-located, authoritative prose contract `scripts/<name>.md`, shed the script-*internals* prose out of the eight always-loaded skill bodies onto those contracts, guard the 1:1 mapping with a test-suite audit, and land a folded-in test-hardening fix.

**Architecture:** This is a documentation-relocation change — **no script behavior changes**. Each `scripts/<name>.sh` gets a sibling `scripts/<name>.md` written to a uniform template, scaled to the script's complexity. The skill bodies keep only the *operational* facts they need to act (when to call, args, exit-code handling, step ordering) and drop the script *internals* (how the script does the work), which now live in the contract. `docket-convention` is special-cased: it keeps the conceptual definitions it owns and points to `scripts/docket-config.md` for `docket-config.sh`'s mechanics. A new existence audit (`tests/test_script_contracts_coverage.sh`) keeps `scripts/*.sh ↔ scripts/*.md` matched 1:1.

**Tech Stack:** Bash (POSIX-ish, `set -uo pipefail`), Markdown. No CI — the `tests/test_*.sh` suite (glob-discovered, each file exit-0 = pass) is the de-facto gate.

## Global Constraints

These apply to **every** task. Copy them into each subagent's working context.

- **This is the docket repo itself (self-hosting).** All edits below — `scripts/*.md`, `skills/*/SKILL.md`, `tests/*.sh` — are **code-line** artifacts on the feature branch `feat/skill-fallback-progressive-disclosure`, cut from `origin/main`. Commit them in the feature worktree `.worktrees/skill-fallback-progressive-disclosure`. **Never** edit docket change-file metadata (`docs/changes/…`, `BOARD.md`, ADRs) here — those live on the `docket` branch and are handled by the implementer outside this plan.
- **Pinned inventory — exactly 13 top-level scripts** (`ls scripts/*.sh`, authoritative): `adr-checks.sh`, `archive-change.sh`, `board-checks.sh`, `cleanup-feature-branch.sh`, `docket-config.sh`, `ensure-claude-settings.sh`, `ensure-docket-env.sh`, `github-mirror.sh`, `render-adr-index.sh`, `render-board.sh`, `render-change-links.sh`, `sync-integration-branch.sh`, `terminal-publish.sh`. Each gets one `scripts/<name>.md`.
- **Out of the 1:1 set** (do NOT author contracts for these, and the audit must NOT require them): repo-**root** scripts (`install.sh`, `link-skills.sh`, `sync-agents.sh`, `migrate-to-docket.sh` — not in `scripts/`, unreachable via `$DOCKET_SCRIPTS_DIR/<name>.md`); and `scripts/lib/docket-frontmatter.sh` (sourced helper, not an entry point — documented within its callers' contracts as warranted). The audit globs **top-level** `scripts/*.sh` only (`*` never matches `/`, so `lib/` is excluded by construction).
- **Contract template (§5 of the spec)** — uniform shape, collapse sections for trivial scripts:
  ```
  # <name>.sh — <one-line purpose>

  ## Purpose      — what it does and why it exists
  ## Usage        — invocation + flags/args
  ## Behavior     — the mechanics (scaled to complexity)
  ## Exit codes   — what 0 / non-zero mean (the "trust the exit code" contract)
  ## Invariants   — guarantees (idempotency, re-run safety, mode guards)
  ```
  A trivial wrapper collapses to **Purpose + Usage + Exit codes**. A complex script (`terminal-publish.sh`, `docket-config.sh`) uses the full set. The **script itself is the authoritative behavior source** — read `scripts/<name>.sh` and describe what it actually does; the existing skill-body prose is the human-readable framing to preserve/refine, not gospel.
- **Body↔contract boundary (§3 — the crux).** For each sentence of script-related prose in a skill body, apply: *does the skill need this to decide what to do next* (**STAYS** in the body) *or does it explain how the script accomplishes its job* (**MOVES** to the contract)?
  - **STAYS:** when to call the script; the command + the args it passes; what it does with the exit code ("trust the exit code" / abort-and-report); **ordering constraints between steps** (e.g. "archive on `docket` *before* terminal-publish, because terminal-publish copies the change file from `origin/docket`").
  - **MOVES:** worktree provisioning, copy-set assembly, idempotency/re-run safety, self-verify, the `origin/HEAD` repair sequence, mode guards, `-B`/prune adoption of leaked branches, the `KEY=value` output contract internals, etc.
- **The naming convention is the only pointer (§2).** Do NOT add a per-call-site "see scripts/<name>.md" pointer at the ~43 call sites. State the rule **once**, in `docket-convention` (Task 8): *every `scripts/<name>.sh` has a co-located `scripts/<name>.md` contract — read it for the script's internals*, reachable as `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.md`.
- **Re-point name-based cross-refs (LEARNINGS #20, 2026-06-17).** A section moved out of a skill body is NOT auto-loaded by consumers. Some bodies cross-reference finalize's terminal-publish section by name (e.g. docket-implement-next / docket-new-change kill steps say "the single source — *Terminal publish (docket-mode)* in `docket-finalize-change`"; docket-status says its sweep "is identical to" finalize's). When a referenced section's *internals* move to a contract, **re-point those cross-refs to `scripts/terminal-publish.md`** (or the relevant contract) so the reference still resolves. Operational cross-refs (which scripts to call in what order for a kill) may stay pointing at the body that owns the operational sequence.
- **Sentinel lockstep (spec Testing; LEARNINGS #2/#5/#21).** Stripping prose must NOT delete a literal substring a wiring-sentinel test greps for. After every body/convention edit, run — and keep GREEN — at minimum: `test_convention_extraction.sh`, `test_composition_wiring.sh`, `test_change_links_coverage.sh`, `test_render_board.sh`, and the per-script tests (`test_terminal_publish.sh`, `test_docket_config.sh`, `test_archive*`/`test_closeout.sh`, etc.). A new assertion must be **mutation-tested** (delete the clause it guards → it flips to NOT OK; restore → OK); one assert anchors exactly ONE clause it owns.
- **Shell-safety guards (LEARNINGS):** never `producer | grep -q` under `pipefail` — capture to a var first, then grep the var (#11/#16). Fail-loud `${VAR:?}` sub-shells in tests must `env -u VAR bash -c …` so a dev shell with the var exported never false-REDs (#34). Guard literal globs with `[ -e "$x" ] || continue`.
- **Baseline (measured 2026-06-21 on `origin/main` @ 0e68c54):** 27 test files; **26 green**, **1 false-RED** = `test_consuming_repo_scripts.sh` (green only under `env -u DOCKET_SCRIPTS_DIR`; the ambient export from `install.sh` masks its fail-loud assertions). Task 1 fixes that. **Definition of done:** the full suite is green **both** in an ambient docket-installed shell (`DOCKET_SCRIPTS_DIR` exported) **and** under `env -u DOCKET_SCRIPTS_DIR`, including the new audit.
- **Commit discipline:** one focused commit per task (or per logical sub-step), on `feat/skill-fallback-progressive-disclosure`. Conventional-style messages, e.g. `docs(scripts): add terminal-publish.sh contract`.

---

## Task 1: Harden `tests/test_consuming_repo_scripts.sh` (folded-in fix)

Independent of everything else; done first so the suite is true-green in both environments before the relocation work begins.

**Files:**
- Modify: `tests/test_consuming_repo_scripts.sh` (the two fail-loud sub-shells)

**Why:** Assertions (3), (3b) verify the `${DOCKET_SCRIPTS_DIR:?…}` *unset* path. They run `bash -c '…'` without clearing the var, so in any shell where `install.sh` exported `DOCKET_SCRIPTS_DIR` the sub-shell inherits it, `:?` never fires, and the assertions go NOT OK even though the code is correct (LEARNINGS #34).

- [ ] **Step 1: Reproduce the ambient false-RED**

Run (in your normal shell, where docket is installed):
```bash
echo "DOCKET_SCRIPTS_DIR=${DOCKET_SCRIPTS_DIR:-<unset>}"   # expect: set to .../scripts
bash tests/test_consuming_repo_scripts.sh; echo "exit=$?"
```
Expected: at least one `NOT OK` line for the fail-loud assertions, `exit=1`. Then confirm it is GREEN under clean env:
```bash
env -u DOCKET_SCRIPTS_DIR bash tests/test_consuming_repo_scripts.sh; echo "exit=$?"
```
Expected: all `ok`, `exit=0`. This proves the test (not the code) is at fault.

- [ ] **Step 2: Clear the var in the two fail-loud sub-shells**

In `tests/test_consuming_repo_scripts.sh`, change the **(3) FAIL-LOUD** sub-shell:
```bash
err="$(cd "$tmp" && bash -c 'echo "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh' 2>&1)"; rc=$?
```
to:
```bash
err="$(cd "$tmp" && env -u DOCKET_SCRIPTS_DIR bash -c 'echo "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh' 2>&1)"; rc=$?
```
and the **(3b)** eval-site sub-shell:
```bash
evalerr="$(cd "$tmp" && bash -c 'eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"' 2>&1)"
```
to:
```bash
evalerr="$(cd "$tmp" && env -u DOCKET_SCRIPTS_DIR bash -c 'eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"' 2>&1)"
```
Leave **(2) RESOLUTION** untouched — it sets `DOCKET_SCRIPTS_DIR="$REPO/scripts"` inline on purpose. Add a short inline comment on each fixed line, e.g. `# env -u: exercise the unset path even when the dev shell exports the var (LEARNINGS #34)`.

- [ ] **Step 3: Verify GREEN in BOTH environments**

```bash
bash tests/test_consuming_repo_scripts.sh; echo "ambient exit=$?"
env -u DOCKET_SCRIPTS_DIR bash tests/test_consuming_repo_scripts.sh; echo "clean exit=$?"
```
Expected: both `exit=0`, all `ok` lines.

- [ ] **Step 4: Commit**

```bash
git add tests/test_consuming_repo_scripts.sh
git commit -m "test(0037): env -u DOCKET_SCRIPTS_DIR in fail-loud sub-shells (no ambient false-RED)"
```

---

## Tasks 2–6: Author the 13 co-located contracts (additive — suite stays green)

For **every** contract task below, the per-contract procedure is identical:

1. **Read the script** `scripts/<name>.sh` end-to-end — it is the authoritative behavior source.
2. **Read the existing body prose** for that script (greppable: `grep -rn '<name>\.sh' skills/`) — the human-readable framing to carry over.
3. **Author `scripts/<name>.md`** to the §5 template, scaled to complexity. Describe what the script *actually does*; do not invent behavior. Use the same vocabulary as the convention (metadata working tree, integration branch, copy-set, etc.).
4. **Do NOT edit any skill body in these tasks** — stripping happens in Tasks 8–11. Authoring is purely additive, so the full suite stays green throughout.
5. **Verify**: `bash scripts/<name>.sh --help 2>/dev/null` or read to confirm the Usage you wrote matches; run the script's own test if present (`tests/test_<name>.sh`) to confirm you changed no behavior (you only added a `.md`). Then commit.

### Task 2: Contract — `terminal-publish.sh` (heaviest)

**Files:**
- Create: `scripts/terminal-publish.md`

**Sources:** `scripts/terminal-publish.sh`; `skills/docket-finalize-change/SKILL.md` lines ~212–277 (`## Terminal publish (docket-mode)`, `### The change-publish path`, `**The copy-set**`, `### The mechanics`).

- [ ] **Step 1: Read sources.** `scripts/terminal-publish.sh` (full), and finalize's terminal-publish section.
- [ ] **Step 2: Author `scripts/terminal-publish.md`** (full template). Must cover, as **Behavior/Invariants** (these are the *internals* that move here): the two publish shapes (`--id` change-publish, `--adr` ADR-only); the copy-set assembly (archived change file + `spec:` if set + the **`Accepted`**-gated ADRs in `adrs:`; ADR-only = the single ADR; `BOARD.md` never published); the generic mechanics — provision a transient `pub-<T>` worktree on the integration branch, `git checkout origin/docket -- <paths>` (never `git merge docket`), CAS/fast-forward-or-retry push, **self-verify** the full copy-set landed on `origin/<integration_branch>`, tear down `pub-<T>`; the `-B` + prune adoption of a leaked `pub-<T>` branch/registration; **re-run safety** (guarded copy+commit is a no-op when bytes match; push loop completes an interrupted push; a sweep racing finalize is a safe no-op); the **mode guard** (no-op when `metadata_branch == integration_branch`, i.e. `main`-mode). As **Usage**: the exact `--id …` and `--adr <NN>` invocations with all flags. As **Exit codes**: `0 ⇒ copy-set landed`; non-zero ⇒ abort-and-report. As **Purpose**: the only sanctioned flow of metadata onto the code line.
- [ ] **Step 3: Verify** `tests/test_terminal_publish.sh` still passes (behavior unchanged; you only added a doc).
```bash
bash tests/test_terminal_publish.sh; echo "exit=$?"   # expect 0
```
- [ ] **Step 4: Commit** — `git add scripts/terminal-publish.md && git commit -m "docs(scripts): add terminal-publish.sh contract"`

### Task 3: Contract — `docket-config.sh` (drives the §4 convention split)

**Files:**
- Create: `scripts/docket-config.md`

**Sources:** `scripts/docket-config.sh`; `skills/docket-convention/SKILL.md` lines ~34, ~36, ~229 (the `origin/HEAD` repair sequence; the `--export` resolution + eval-able `KEY=value` output list + fail-closed exit codes; the `--bootstrap` 2×2 realization).

- [ ] **Step 1: Read sources** (script full; the three convention paragraphs).
- [ ] **Step 2: Author `scripts/docket-config.md`** (full template). **Behavior** (the script *mechanics* that move here): repair `origin/HEAD` (`git remote set-head origin -a` → `git symbolic-ref refs/remotes/origin/HEAD`); read `.docket.yml` authoritatively via `git show origin/HEAD:.docket.yml` after a fetch (working-tree copy trusted only on the default branch's primary checkout); the ref-unresolvable-vs-file-absent distinction (resolve+absent ⇒ defaults; unresolvable/unreachable ⇒ abort, keying on `set-head`/fetch return code, never on `git show`); resolve `integration_branch` (`auto`→`origin/HEAD`, fallback `main`); evaluate the bootstrap 2×2 and emit `BOOTSTRAP=`; the `--bootstrap` orphan-create write path (guarded to `¬DOCKET ∧ ¬LIVE`). **Usage:** `--export` (read-only; the `eval "$(… --export)"` consumption shape) and `--bootstrap`. **Behavior/output contract:** the full emitted key list (`DOCKET_MODE, DEFAULT_BRANCH, METADATA_BRANCH, INTEGRATION_BRANCH, METADATA_WORKTREE, CHANGES_DIR, ADRS_DIR, RESULTS_DIR, FINALIZE_GATE, FINALIZE_TEST_COMMAND, BOARD_SURFACES, AUTO_GROOM, BOOTSTRAP`). **Exit codes:** fail-closed — non-zero + diagnostic on a hard error (unreachable `origin`, unresolvable `origin/HEAD`, ref-absent `integration_branch`, `metadata_branch ∉ {docket,main}`). **Do NOT** restate the *meaning* of the knobs or the bootstrap verdicts here — those are conceptual definitions that stay in the convention (Task 8); this file documents how the script *realizes* them.
- [ ] **Step 3: Verify** `tests/test_docket_config.sh` still passes.
```bash
bash tests/test_docket_config.sh; echo "exit=$?"   # expect 0
```
- [ ] **Step 4: Commit** — `git commit -m "docs(scripts): add docket-config.sh contract"`

### Task 4: Contracts — git-operation scripts

**Files:**
- Create: `scripts/archive-change.md`, `scripts/cleanup-feature-branch.md`, `scripts/sync-integration-branch.md`

For each: read the script + its body prose (`grep -rn '<name>.sh' skills/`), author to the template.

- [ ] **Step 1: `scripts/archive-change.md`.** Behavior: the dated `active/ → archive/<UTC-date>-<id>-<slug>.md` move; the `## Why killed` insertion for `--outcome killed`; sets `status: done|killed` / `updated:` / the `results:` link; **change-file-only** commit; push-with-rebase-retry on `metadata_branch`; idempotent (reuses the existing dated filename across a day boundary). Usage: `--changes-dir … --id … --outcome done|killed [--date …] [--reason …] [--message …]`. Exit codes: `0 ⇒ archived` (idempotent no-op if already archived); non-zero ⇒ abort. Invariants: never stages the already-moved `active/` path (LEARNINGS #26).
- [ ] **Step 2: `scripts/cleanup-feature-branch.md`.** Behavior: removes the feature worktree + branch for a slug; provenance guard (resolve symlinks with `pwd -P` before stripping a worktree prefix — LEARNINGS #25). Usage: `--slug <slug>`. Exit codes + idempotency (no-op if already gone).
- [ ] **Step 3: `scripts/sync-integration-branch.md`.** Behavior: best-effort, FF-only fast-forward of the clone's local `<integration_branch>` after a merge lands; no-op on non-FF/dirty/feature-branch/`main`-mode. Usage + exit codes (collapse to Purpose+Usage+Exit codes+Invariants — it is light).
- [ ] **Step 4: Verify** related tests still pass (`test_closeout.sh`, `test_sync_integration_branch.sh`) and **commit** the three files.
```bash
for t in test_closeout test_sync_integration_branch; do bash tests/$t.sh >/dev/null 2>&1 && echo "$t ok" || echo "$t FAIL"; done
git add scripts/archive-change.md scripts/cleanup-feature-branch.md scripts/sync-integration-branch.md
git commit -m "docs(scripts): add git-operation script contracts (archive-change, cleanup-feature-branch, sync-integration-branch)"
```

### Task 5: Contracts — renderers + mirror

**Files:**
- Create: `scripts/render-board.md`, `scripts/render-change-links.md`, `scripts/render-adr-index.md`, `scripts/github-mirror.md`

- [ ] **Step 1–4: Author each** to the template (these are mostly light → Purpose+Usage+Behavior+Exit codes+Invariants, tight). Key facts to capture:
  - `render-board.md`: reads change files (active+archive), emits `BOARD.md` to **STDOUT** (caller redirects+commits); offline; deterministic/idempotent (same inputs → identical bytes); `--changes-dir DIR [--repo OWNER/REPO]`; sole writer of `BOARD.md`.
  - `render-change-links.md`: sole writer of the marker-bounded `## Artifacts` block (ADR-0012 boundary); `--change-file … --adrs-dir …`; offline, falls back to bare code-formatted paths with no GitHub remote.
  - `render-adr-index.md`: regenerates `docs/adrs/README.md` from ADR frontmatter verbatim (no embellishment — LEARNINGS #30); sole writer.
  - `github-mirror.md`: the one-way Issues + Projects v2 mirror executor; never read back; best-effort; invoked only by the Board pass; full external-write mechanics live here (point to `docket-convention/github-board-mirror.md` for the *surface* semantics, which stays put).
- [ ] **Step 5: Verify + commit.**
```bash
for t in test_render_board test_render_change_links test_render_adr_index test_github_mirror; do bash tests/$t.sh >/dev/null 2>&1 && echo "$t ok" || echo "$t FAIL"; done
git add scripts/render-board.md scripts/render-change-links.md scripts/render-adr-index.md scripts/github-mirror.md
git commit -m "docs(scripts): add renderer + mirror contracts"
```

### Task 6: Contracts — checks + ensure-* helpers

**Files:**
- Create: `scripts/board-checks.md`, `scripts/adr-checks.md`, `scripts/ensure-claude-settings.md`, `scripts/ensure-docket-env.md`

- [ ] **Step 1–4: Author each** (light → collapse as warranted). Key facts:
  - `board-checks.md`: the health checks docket-status runs over the board/change set (stale claims, broken links, dep stalls) — enumerate the checks it emits.
  - `adr-checks.md`: the ADR-index/ledger health checks.
  - `ensure-claude-settings.md`: idempotent injection of docket env into Claude Code's user-level `settings.json` (`env` block); re-run safe.
  - `ensure-docket-env.md`: idempotent injection of `DOCKET_SCRIPTS_DIR` (and siblings) into the shell profile so it reaches dispatched subagents; re-run safe; back-fills already-migrated clones.
- [ ] **Step 5: Verify + commit.**
```bash
for t in test_board_checks test_adr_checks test_ensure_claude_settings test_ensure_docket_env; do bash tests/$t.sh >/dev/null 2>&1 && echo "$t ok" || echo "$t FAIL"; done
git add scripts/board-checks.md scripts/adr-checks.md scripts/ensure-claude-settings.md scripts/ensure-docket-env.md
git commit -m "docs(scripts): add checks + ensure-* helper contracts"
```

---

## Task 7: §6 coverage audit — `tests/test_script_contracts_coverage.sh`

After Tasks 2–6, all 13 contracts exist, so this audit goes GREEN immediately; the task's value is locking the 1:1 invariant and proving the assertions non-vacuous.

**Files:**
- Create: `tests/test_script_contracts_coverage.sh`

- [ ] **Step 1: Write the audit** (model: `tests/test_change_links_coverage.sh`):

```bash
#!/usr/bin/env bash
# tests/test_script_contracts_coverage.sh — every top-level scripts/<name>.sh has a co-located
# scripts/<name>.md contract, and every scripts/<name>.md has a live scripts/<name>.sh (change
# 0037). Existence audit ONLY — content fidelity rests on co-location + review + the convention's
# "prose is the contract" rule (mechanical prose-vs-bash checking is out of scope: flaky/gameable).
# Mirrors test_change_links_coverage.sh; the suite is the de-facto gate (no GitHub Actions CI).
# Scope: TOP-LEVEL scripts/*.sh only (the glob's * never matches /, so scripts/lib/*.sh sourced
# helpers are out — they are documented within their callers' contracts).
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# (1) every top-level scripts/<name>.sh has a co-located scripts/<name>.md
for sh in "$ROOT"/scripts/*.sh; do
  [ -e "$sh" ] || continue
  base="$(basename "$sh" .sh)"
  if [ -f "$ROOT/scripts/$base.md" ]; then ok "contract present for $base.sh"; else no "missing scripts/$base.md for $base.sh"; fi
done

# (2) every scripts/<name>.md has a live scripts/<name>.sh (no orphaned contract)
for md in "$ROOT"/scripts/*.md; do
  [ -e "$md" ] || continue
  base="$(basename "$md" .md)"
  if [ -f "$ROOT/scripts/$base.sh" ]; then ok "script present for $base.md"; else no "orphaned scripts/$base.md (no $base.sh)"; fi
done

exit $fail
```

- [ ] **Step 2: Run it — expect GREEN** (all 13 paired):
```bash
chmod +x tests/test_script_contracts_coverage.sh
bash tests/test_script_contracts_coverage.sh; echo "exit=$?"   # expect 0, 26 ok lines (13+13)
```

- [ ] **Step 3: Mutation-test both directions (prove non-vacuous):**
```bash
# (a) missing contract -> RED
mv scripts/adr-checks.md /tmp/_c.md
bash tests/test_script_contracts_coverage.sh >/tmp/m.out 2>&1; echo "missing-contract exit=$? (expect 1)"; grep 'NOT OK' /tmp/m.out
mv /tmp/_c.md scripts/adr-checks.md
# (b) orphaned contract -> RED
touch scripts/__orphan__.md
bash tests/test_script_contracts_coverage.sh >/tmp/m.out 2>&1; echo "orphan exit=$? (expect 1)"; grep 'NOT OK' /tmp/m.out
rm scripts/__orphan__.md
# (c) restored -> GREEN
bash tests/test_script_contracts_coverage.sh; echo "restored exit=$? (expect 0)"
```
Expected: (a) and (b) exit 1 with a matching `NOT OK`; (c) exit 0.

- [ ] **Step 4: Commit** — `git add tests/test_script_contracts_coverage.sh && git commit -m "test(0037): add scripts/*.sh <-> scripts/*.md 1:1 coverage audit"`

---

## Task 8: `docket-convention` — §4 split + §2 naming-convention rule

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (the `### Configuration` and `### Bootstrap guard` prose; add the §2 rule)

**Sources/anchors:** lines ~34, ~36 (config-resolution mechanics), ~229 (bootstrap realization). `scripts/docket-config.md` (Task 3) is the move target.

- [ ] **Step 1: Snapshot sentinels before editing.**
```bash
bash tests/test_convention_extraction.sh >/tmp/conv.before 2>&1; echo "before exit=$?"
bash tests/test_composition_wiring.sh >/dev/null 2>&1 && echo "compwiring ok"
```
- [ ] **Step 2: Apply the §4 split.** In `### Configuration`: KEEP the conceptual definitions — the `.docket.yml` knob meanings (the YAML block + per-knob descriptions), `metadata_branch`/`integration_branch` semantics, the backward-compatible opt-out. REPLACE the *script mechanics* prose (the `git remote set-head`/`symbolic-ref`/`git show origin/HEAD` repair sequence; the `--export` resolution steps + the eval-able `KEY=value` output enumeration + fail-closed exit-code internals) with a **one-line pointer** in the convention's existing style, e.g.:
  > Resolved deterministically by `docket-config.sh --export` (consumed as `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"`); its interface and mechanics — `origin/HEAD` repair, authoritative `.docket.yml` read, the emitted `KEY=value` contract, and fail-closed exit codes — are in its contract `scripts/docket-config.md`.
  Keep the sentence "The prose in this section is the contract the script implements verbatim" intent, but the *mechanics* now live in the contract. In `### Bootstrap guard`: KEEP the 2×2 table + probe definitions (`DOCKET`/`LIVE`) + verdict meanings (`PROCEED`/`STOP_MIGRATE`/`CREATE_ORPHAN`) — these are the spec the script implements; trim only the restated *realization* prose, pointing to `scripts/docket-config.md` for how the script evaluates it.
- [ ] **Step 3: State the §2 naming-convention rule ONCE.** Add a short subsection (e.g. under `### Directory layout` or a new `### Script contracts` note) stating verbatim the rule: *Every `scripts/<name>.sh` has a co-located `scripts/<name>.md` contract — its authoritative, human-readable spec (Purpose / Usage / Behavior / Exit codes / Invariants). Read it for a script's internals; reach it from a consuming repo as `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.md`, the same mechanism that reaches the scripts. `docket-convention/github-board-mirror.md` is skill-reference, not a single-script contract.*
- [ ] **Step 4: Sentinel lockstep.** Re-run the snapshots; diff for regressions:
```bash
bash tests/test_convention_extraction.sh; echo "exit=$? (expect 0)"
bash tests/test_composition_wiring.sh; echo "exit=$? (expect 0)"
```
If a sentinel goes NOT OK, you deleted a substring it owns — restore that exact phrase (the sentinel is sampling; the convention must still carry the concept). Do **not** weaken the test.
- [ ] **Step 5: Full suite + commit.**
```bash
for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAIL: $t"; done; echo "(empty above = all green)"
git add skills/docket-convention/SKILL.md
git commit -m "docs(convention): move docket-config.sh mechanics to its contract; state script-contract naming rule"
```

---

## Tasks 9–11: Strip script-internals prose from the 8 skill bodies

Per-body procedure (apply the §3 boundary, re-point cross-refs, keep sentinels green):

1. `grep -n '<script>.sh' skills/<skill>/SKILL.md` to find the script-related prose.
2. For each block, apply the §3 test. KEEP operational facts (when/args/exit-code/ordering); REMOVE internals (now in the contract).
3. Re-point any name-based cross-ref to a removed section → point to the contract (`scripts/<name>.md`).
4. Run the body's relevant sentinel/per-script tests; keep GREEN. Mutation-test any new assertion.
5. Commit.

### Task 9: `docket-finalize-change` (heaviest body)

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (lines ~212–277, the `## Terminal publish (docket-mode)` + `### The mechanics`; plus the `archive-change.sh` step ~80–94)

- [ ] **Step 1: Strip terminal-publish internals.** Remove `### The mechanics` (provision→copy→CAS-push→teardown, `-B`/prune, self-verify, re-run safety) and the copy-set *assembly* internals. KEEP: the operational call (`terminal-publish.sh --id … --outcome … --integration-branch … --metadata-branch … --changes-dir … --adrs-dir … --message …` and the `--adr <NN>` shape), "trust the exit code" (`0 ⇒ landed`; non-zero ⇒ abort-and-report), and the **archive-on-`docket`-before-publish ordering** (with the *why*: terminal-publish copies from `origin/docket`). Replace the removed internals with a pointer: *(mechanics: `scripts/terminal-publish.md`)*.
- [ ] **Step 2: Strip archive-change internals**, keep the operational call + "trust the exit code" + the change-file-only/idempotency *facts the skill relies on* (point to `scripts/archive-change.md` for internals). Preserve the "identical to docket-status's sweep" cross-note (re-point to the contract as the shared source if appropriate).
- [ ] **Step 3: Sentinel lockstep + per-script tests.**
```bash
for t in test_terminal_publish test_closeout test_finalize_gate test_composition_wiring test_convention_extraction; do bash tests/$t.sh >/dev/null 2>&1 && echo "$t ok" || echo "$t FAIL"; done
```
All must be `ok`. If any FAIL, restore the exact substring it greps for.
- [ ] **Step 4: Commit** — `git commit -am "docs(finalize): move terminal-publish/archive-change internals to contracts, keep operational facts"`

### Task 10: `docket-status` + `docket-new-change`

**Files:**
- Modify: `skills/docket-status/SKILL.md`, `skills/docket-new-change/SKILL.md`

- [ ] **Step 1: `docket-status`** — strip the merge-sweep archive internals + board-render internals; keep: which script it calls, args, exit-code handling, and the sweep ordering (archive-on-`docket` before terminal-publish). Re-point "identical to finalize's archive" / terminal-publish references to the contracts.
- [ ] **Step 2: `docket-new-change`** — strip the `archive-change.sh`/`terminal-publish.sh` *kill-step internals*; keep the operational kill sequence (archive → publish → prune; which script, what order, what args) and re-point the "single source — *Terminal publish* in finalize" cross-ref to `scripts/terminal-publish.md`.
- [ ] **Step 3: Sentinel lockstep.**
```bash
for t in test_change_links_coverage test_composition_wiring test_render_board test_closeout test_learnings_ledger; do bash tests/$t.sh >/dev/null 2>&1 && echo "$t ok" || echo "$t FAIL"; done
```
- [ ] **Step 4: Commit** — `git commit -am "docs(status,new-change): move script internals to contracts, keep operational sequences"`

### Task 11: `docket-adr` + `docket-implement-next` + `docket-auto-groom` + `docket-groom-next`

**Files:**
- Modify: `skills/docket-adr/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-groom-next/SKILL.md`

- [ ] **Step 1:** For each, `grep -n '\.sh' skills/<skill>/SKILL.md`; strip the lighter script-internals references, keep operational calls/args/exit-code/ordering. In `docket-implement-next` and `docket-adr`, re-point the "single source — *Terminal publish (docket-mode)* in `docket-finalize-change`" cross-ref to `scripts/terminal-publish.md`.
- [ ] **Step 2: Sentinel lockstep.**
```bash
for t in test_change_links_coverage test_composition_wiring test_auto_groom test_groom_recap; do bash tests/$t.sh >/dev/null 2>&1 && echo "$t ok" || echo "$t FAIL"; done
```
- [ ] **Step 3: Commit** — `git commit -am "docs(adr,implement-next,auto-groom,groom-next): move script internals to contracts"`

---

## Task 12: Whole-branch verification + context-win check

**Files:** none (verification only; may amend the last commit if a fix is needed)

- [ ] **Step 1: Full suite GREEN in both environments.**
```bash
fail=0
for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || { echo "AMBIENT FAIL: $t"; fail=1; }; done
for t in tests/test_*.sh; do env -u DOCKET_SCRIPTS_DIR bash "$t" >/dev/null 2>&1 || { echo "CLEAN FAIL: $t"; fail=1; }; done
[ $fail = 0 ] && echo "ALL GREEN (both environments)"
```
Expected: `ALL GREEN (both environments)`.
- [ ] **Step 2: Coverage audit final check** — `bash tests/test_script_contracts_coverage.sh` exit 0; `ls scripts/*.md | wc -l` = 13.
- [ ] **Step 3: No operational fact lost (spec risk mitigation).** For each edited body, `git diff origin/main -- skills/<skill>/SKILL.md` and read for retained ordering/invocation facts: every removed block is either (a) purely internals (now in a contract) or (b) re-pointed. Confirm no skill lost a step it needs to act. Confirm no dangling cross-ref to a removed section (grep bodies for "*The mechanics*", "Terminal publish (docket-mode)" and verify each still resolves).
- [ ] **Step 4: Context-win sanity** — `git diff --stat origin/main -- skills/` should show net **removed** lines from the bodies (the relocation goal). Record the net delta for the PR description.
- [ ] **Step 5:** If everything green, no further commit. Otherwise fix + commit, then re-run Step 1.

---

## Self-Review (author's check against the spec)

- **Spec coverage:** §1 co-located contracts → Tasks 2–6. §2 naming-convention pointer → Task 8 Step 3. §3 body↔contract boundary → Global Constraints + Tasks 9–11. §4 convention special-case → Task 8. §5 template → Global Constraints (used by 2–6). §6 existence audit → Task 7. Folded-in `test_consuming_repo_scripts.sh` fix → Task 1. Sentinel lockstep / suite green → Tasks 8–12. All covered.
- **Scope decisions pinned (reconcile):** 13 scripts (incl. the two un-spec'd `ensure-*`); root scripts + `scripts/lib/` excluded from the 1:1 audit. Reflected in Global Constraints + Task 7.
- **Type/name consistency:** contract filenames are exactly `scripts/<sh-basename>.md`; the audit derives both directions by `basename … .sh`/`.md`, so any name mismatch fails the audit (Task 7). Pointer syntax `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.md` is used identically in Task 8.
- **No placeholders:** test tasks (1, 7) carry full shell; doc tasks carry the template + exact source line ranges + a per-contract coverage checklist + the §3 rule (documentation authored by judgment from the named sources, guarded by the audit + whole-branch review — content fidelity is explicitly out of scope for mechanical checking per the spec).
