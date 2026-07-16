# Codex CLI Live-Validation Runbook Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `docs/codex/validation-runbook.md` — a six-phase guided checklist Daniel executes interactively in Codex CLI to prove docket works end-to-end under a non-Claude harness — plus a structural guard test that keeps the runbook's derivable claims from rotting.

**Architecture:** The runbook is prose, so the only claims a test *can* mechanically hold are structural ones: that it covers all six phases, that each phase carries a pass criterion, that every committed repo path it cites exists, that it names the full generated-agent set derived from the `agents/docket-*.md` glob, and that it spells the facade the way the convention does. Everything a test cannot prove — whether the *expected outcomes* are correct — is settled by Daniel actually running the runbook and recording results. That split mirrors the closest precedent in this repo, `docs/cursor/permissions.md` + `tests/test_cursor_permissions_docs.sh`, which this plan follows deliberately.

**Tech Stack:** Bash (POSIX-leaning, GNU/BSD-portable), markdown. Tests are standalone `tests/*.sh` scripts that print `ok - …` / `NOT OK - …` and exit 0/1. No CI — the suite is the gate.

## Global Constraints

Copied verbatim from the reconciled spec (`.docket/docs/superpowers/specs/2026-07-15-codex-cli-validation-runbook-design.md`) and this repo's LEARNINGS. Every task's requirements implicitly include this section.

- **This change validates the NATIVE Codex path only.** Codex runs docket skills; their bash reaches scripts through the `docket.sh` facade (`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <op>`, change 0068) under **Codex's own sandbox**. Change 0079's `runner-dispatch` / `scripts/runners/codex.sh` is the **opposite direction** (a Claude-Code *parent* offloading onto a `codex exec` *child*) and is **out of scope** — the runbook must not conflate them.
- **Script paths are repo-root:** `sync-agents.sh`, `link-skills.sh`, `install.sh`, `migrate-to-docket.sh`. **Not** `scripts/`. Neither `sync-agents.sh` nor `link-skills.sh` has a co-located `.md` contract.
- **Never hardcode a Codex model slug as fact.** `config.yml.example`'s codex block is explicitly labelled "The IDs here are UNVALIDATED examples." The runbook must instruct the operator to derive slugs via `codex debug models | jq -r '.models[] | .slug'` (as `docs/codex/setup.md` already does). Prose restating a configurable value is a drift surface (LEARNINGS, verify-the-claim (d) — a README asserting per-agent model tiers shipped factually FALSE with every grep green).
- **`agents:` keys are bare, un-prefixed names** (`status`, `adr`, `implement-next`), harness-first under `agents.codex.<name>`. The generated file is `.codex/agents/docket-<name>.toml`.
- **Docket `effort:` maps to the TOML key `model_reasoning_effort`, verbatim.** There is no `max`→`xhigh` remap in the TOML emitter; that remap exists only in the runner adapter (`scripts/runners/codex.sh`), which is out of scope here.
- **The runbook must extend `docs/codex/setup.md`, never duplicate it.** setup.md owns static setup (opt-in scopes, pinning, restart-after-regenerating); the runbook owns live execution and evidence capture.
- **Phase 4 must produce the evidence ADR-0036 deferred to this change** — whether Codex honors the project-level `AGENTS.md` dispatch block (automatic / prompted / refused) and whether a user-level `~/.codex/AGENTS.md` is needed. Acting on that finding is a follow-up, never this change.
- **A failing phase is a valid outcome.** The validation passes when phases 1–3 and 6 pass and phases 4–5 have a definitive observed answer. Every gap becomes a follow-up `proposed` stub; none block writing results.
- **Test style** (from `tests/test_cursor_permissions_docs.sh`): `set -uo pipefail` (never `-e`); `REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"`; `ok()`/`no()` helpers with a `fail` accumulator; `exit $fail`.
- **Portability + anti-vacuity rules** (LEARNINGS, hard-won):
  - Always `grep -qF -- "$pat"` or `grep -E -e "$pat"` — a pattern leading with `--` is otherwise parsed as an option (exit 2), and a leading `!` inverts that error into a green pass.
  - Never pipe a producer into an early-exiting consumer (`grep -q`, `head`) under `pipefail` — capture to a variable, then match with a here-string.
  - **Derive, never retype:** build asserted literals from the authoritative source (the convention, the glob), so the assert cannot drift from the artifact.
  - **A tokenizer must prove it sees the corpus:** assert the *count* of units found, not just the verdict — a scan that parses nothing passes everything.
  - **Beware substring satisfaction:** `docket-auto-groom` is a prefix of `docket-auto-groom-critic`; a plain `grep -qF` for the former is satisfied by the latter. Match with a trailing non-name-character boundary.

---

## File Structure

| File | Responsibility |
|---|---|
| `docs/codex/validation-runbook.md` | **Create.** The deliverable: a six-phase, operator-executed checklist. Sits beside `docs/codex/setup.md` (static setup) as its live-execution counterpart, mirroring `docs/cursor/permissions.md`. |
| `tests/test_codex_runbook.sh` | **Create.** Structural guard: phase coverage, per-phase pass criteria, cited-path existence, generated-agent-set completeness (derived from the glob), canonical facade spelling, discoverability links. |
| `docs/codex/setup.md` | **Modify.** Its `## Verifying it works` section gains a pointer to the runbook (the deep, live version of that section's three static checks). |
| `README.md` | **Modify.** Link the runbook so it is discoverable, mirroring the existing `](docs/cursor/permissions.md)` link. |

Two tasks. Task 1 builds the runbook and the guard that holds its derivable claims. Task 2 wires discoverability and extends the guard to cover it — a reviewer could reasonably accept the runbook while rejecting how it is surfaced, which is exactly where the boundary belongs.

### What this build does NOT produce (and why that is correct)

The spec lists three deliverables; **this build produces only the first**. That is not a gap in the plan — it is the nature of the change:

| Spec deliverable | Produced by |
|---|---|
| 1. The runbook | **This build** (Task 1). |
| 2. A results doc (`docs/results/…-codex-validation-results.md`) — per-step pass/fail, observed behavior, environment versions | **Daniel executing the runbook**, at/after the merge gate. It requires an interactive Codex CLI session and real OpenAI billing; no autonomous agent can produce it. |
| 3. Follow-up stubs, one per gap found | **Daniel executing the runbook** — the gaps do not exist until it runs. |

So the merge gate for this change is *"is this runbook correct and executable?"*, not *"did Codex pass?"*. The build's own results file must say so explicitly, so the human knows execution is theirs.

---

### Task 1: The runbook and its structural guard

**Files:**
- Create: `/Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook/tests/test_codex_runbook.sh`
- Create: `/Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook/docs/codex/validation-runbook.md`

**Interfaces:**
- Consumes: `agents/docket-*.md` (the authoritative generated-agent set — 9 files today: `docket-adr`, `docket-auto-groom`, `docket-auto-groom-critic`, `docket-brainstorm-consultant`, `docket-finalize-change`, `docket-implement-next`, `docket-integration-repair`, `docket-rebase-resolver`, `docket-status`); `skills/docket-convention/SKILL.md` (the canonical `${DOCKET_SCRIPTS_DIR:?run docket/install.sh}` token).
- Produces: `docs/codex/validation-runbook.md` with `## Phase 1 — …` … `## Phase 6 — …` headings and one `**Pass when:**` line per phase. Task 2 relies on that exact file path and on `tests/test_codex_runbook.sh` existing with a `fail` accumulator and `ok`/`no` helpers to extend.

- [ ] **Step 1: Write the failing test**

Create `tests/test_codex_runbook.sh`:

```bash
#!/usr/bin/env bash
# tests/test_codex_runbook.sh — guards for the Codex live-validation runbook (change 0078).
# Structure only: these prove the runbook covers every phase, stamps a pass criterion on each,
# cites only real committed paths, and names the COMPLETE generated-agent set derived from the
# glob. They CANNOT prove an expected-outcome claim is TRUE — that is established by Daniel
# executing the runbook in Codex CLI and recording the results doc (LEARNINGS, verify-the-claim
# family: a doc sentinel proves a sentence still EXISTS, never that it is still TRUE).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
RUNBOOK="$REPO/docs/codex/validation-runbook.md"
CONV="$REPO/skills/docket-convention/SKILL.md"
fail=0
ok(){ echo "ok - $1"; }
no(){ echo "NOT OK - $1"; fail=1; }

# --- Assertion 0: the runbook exists ----------------------------------------------------------
if [ -f "$RUNBOOK" ]; then ok "runbook exists"; else no "runbook exists"; exit 1; fi

# --- Assertion 1: all six phases present, each stamped with a pass criterion -------------------
# Count equality, not presence: a phase silently dropped (or added unstamped) must redden.
NPHASES="$(grep -cE '^## Phase [1-6] — ' "$RUNBOOK")"
if [ "$NPHASES" = "6" ]; then ok "runbook covers 6 phases"; else no "runbook covers 6 phases (found $NPHASES)"; fi
NSTAMPS="$(grep -cF -- '**Pass when:**' "$RUNBOOK")"
if [ "$NSTAMPS" = "$NPHASES" ] && [ "$NPHASES" != "0" ]; then ok "every phase ($NPHASES) carries a pass criterion ($NSTAMPS)"; else no "phase/pass-criterion mismatch: $NPHASES phases, $NSTAMPS criteria"; fi

# --- Assertion 2: the generated-agent set is COMPLETE, derived from the glob -------------------
# The enumerated-floor trap (LEARNINGS): a hand-listed agent set goes stale the moment a 10th
# wrapper lands. Derive from the authoritative glob and require the runbook to name every member.
AGENT_SET="$(cd "$REPO/agents" && ls docket-*.md 2>/dev/null | sed 's/\.md$//' | sort)"
NAGENTS="$(grep -c . <<<"$AGENT_SET")"
if [ "$NAGENTS" -ge 9 ]; then ok "agent set derivable from glob ($NAGENTS agents)"; else no "agent set derivable from glob (found $NAGENTS, expected >= 9)"; fi
missing=""
while IFS= read -r a; do
  [ -z "$a" ] && continue
  # Boundary-matched: bare -F would let `docket-auto-groom-critic` satisfy `docket-auto-groom`.
  grep -qE -- "${a}([^A-Za-z0-9_-]|$)" "$RUNBOOK" || missing="$missing $a"
done <<<"$AGENT_SET"
if [ -z "$missing" ]; then ok "runbook names every generated agent"; else no "runbook missing agents:$missing"; fi

# --- Assertion 3: every committed repo path the runbook cites actually EXISTS ------------------
# This is the guard for the error class that bit this change's own spec: it cited
# `scripts/sync-agents.sh`, which does not exist (the script is repo-root). Scope is deliberate —
# only paths under committed dirs. Generated/user paths (`.codex/…`, `~/.codex/…`,
# `.docket.local.yml`) are excluded by construction: the runbook TEACHES the operator to create
# them, so they cannot exist before it runs. Glob'd tokens carry `*`, which the char class
# excludes, so they never enter the scan.
CITED="$(grep -oE '`[A-Za-z0-9_./-]+`' "$RUNBOOK" | tr -d '`' | sort -u | grep -E '^(scripts|agents|skills|docs|tests)/' )"
NCITED="$(grep -c . <<<"$CITED")"
# Prove the tokenizer SEES the corpus before trusting its verdict (LEARNINGS: a scan that parses
# nothing passes everything). Floor is the count of paths Task 1 Step 3 REQUIRES the runbook to
# cite, minus headroom — high enough to catch a blind tokenizer, low enough never to false-red.
if [ "$NCITED" -ge 4 ]; then ok "path scan found $NCITED cited repo paths"; else no "path scan found only $NCITED cited repo paths (tokenizer likely blind)"; fi
badpaths=""
while IFS= read -r p; do
  [ -z "$p" ] && continue
  [ -e "$REPO/$p" ] || badpaths="$badpaths $p"
done <<<"$CITED"
if [ -z "$badpaths" ]; then ok "every cited repo path exists"; else no "runbook cites nonexistent paths:$badpaths"; fi

# --- Assertion 4: root-level scripts the runbook drives are named at their REAL location -------
# The repo-root scripts have no `scripts/` prefix; assert each is cited and each exists.
for s in install.sh migrate-to-docket.sh sync-agents.sh link-skills.sh; do
  if [ ! -f "$REPO/$s" ]; then no "root script exists: $s"; continue; fi
  if grep -qF -- "\`$s\`" "$RUNBOOK"; then ok "runbook names root script: $s"; else no "runbook names root script: $s"; fi
done
# ...and never at a fabricated `scripts/` path.
if grep -qF -- 'scripts/sync-agents.sh' "$RUNBOOK"; then no "runbook cites fabricated scripts/sync-agents.sh"; else ok "runbook does not cite fabricated scripts/sync-agents.sh"; fi

# --- Assertion 5: canonical facade spelling, DERIVED from the convention -----------------------
# Phase 2 is the bash-under-sandbox smoke test; it must spell the facade the way the convention
# does. Derive the token rather than retyping it, so the assert cannot drift from the contract.
CANON="$(grep -oE '\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}' "$CONV")"
CANON="${CANON%%$'\n'*}"   # first match; no pipe-to-head (pipefail-safe)
if [ -n "$CANON" ]; then ok "canonical facade token derivable from convention"; else no "canonical facade token derivable from convention"; fi
if [ -n "$CANON" ] && grep -qF -- "$CANON" "$RUNBOOK"; then ok "runbook carries canonical facade spelling"; else no "runbook carries canonical facade spelling"; fi

# --- Assertion 6: no hardcoded Codex model slug presented as fact ------------------------------
# config.yml.example labels its codex IDs "UNVALIDATED examples"; setup.md points at the live
# source. The runbook must teach derivation, not pin a slug.
if grep -qF -- 'codex debug models' "$RUNBOOK"; then ok "runbook derives model slugs via codex debug models"; else no "runbook derives model slugs via codex debug models"; fi

# --- Assertion 7: the native/runner-delegation boundary is stated ------------------------------
# 0079's runner delegation is the opposite direction and out of scope; conflating them is the
# single likeliest misreading of this runbook.
if grep -qF -- 'runner-dispatch' "$RUNBOOK"; then ok "runbook states the runner-dispatch boundary"; else no "runbook states the runner-dispatch boundary"; fi

exit $fail
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook && bash tests/test_codex_runbook.sh; echo "exit=$?"
```
Expected: `NOT OK - runbook exists` then `exit=1` (Assertion 0 exits early — the runbook does not exist yet).

- [ ] **Step 3: Write the runbook**

Create `docs/codex/validation-runbook.md`. Write it to satisfy every assertion above **and** to be genuinely executable by a human — the assertions are a floor, not the goal.

Two assertions constrain the runbook's *wording*, so satisfy them deliberately rather than discovering them at Step 4:

1. **Assertion 4 requires each root script named in backticks, in prose** — not only inside a fenced command. Phase 1 must carry a sentence such as: "``sync-agents.sh``, ``link-skills.sh``, ``install.sh`` and ``migrate-to-docket.sh`` live at the **repo root** of your docket clone — there is no ``scripts/`` prefix." That sentence is also substantively load-bearing: citing them at a fabricated `scripts/` path is the exact error this change's own spec shipped.
2. **Assertion 3 requires ≥ 4 cited repo-relative paths, and every one must exist.** The runbook must cite at least: `docs/codex/setup.md` (the static counterpart), `docs/cursor/permissions.md` (the Cursor precedent named in Phase 2), `scripts/runners/codex.sh` (named only to mark it out of scope), and `docs/results/` (where the execution record lands). Do not invent paths to pad the count — an unchecked citation is what the assertion exists to catch.

Required structure:
- Title + a lead paragraph stating: this is the live-execution counterpart to `docs/codex/setup.md`; it validates docket running **natively inside Codex CLI**; change 0079's `runner-dispatch` (Claude parent → `codex exec` child) is the opposite direction and **out of scope**.
- An **Environment** capture block the operator fills in first: Codex CLI version, model used, date, OS.
- Six `## Phase N — <name>` sections, each ending in a `**Pass when:**` line, each step giving an exact command/prompt and an observable expected outcome with a `- [ ]` box.
- A `## Pass criteria` section: passes when phases 1–3 and 6 pass and 4–5 have definitive observed answers; every gap becomes a follow-up `proposed` stub; a "no" is a valid, recordable outcome.
- A `## Recording results` section pointing at `docs/results/` for the close-out doc.

Phase content (exact commands — do not paraphrase):

**`## Phase 1 — Setup (outside Codex)`**
1. Create a fixture repo with an origin, then install docket into it:
   ```sh
   bash /path/to/docket/install.sh
   cd /path/to/fixture && bash /path/to/docket/migrate-to-docket.sh
   ```
2. Opt the fixture in to the Codex harness — machine-local, so the fixture's committed config stays clean:
   ```sh
   printf 'agent_harnesses: [claude, codex]\n' > .docket.local.yml
   ```
   Note in the runbook: **this opt-in is required**; with `agent_harnesses` unset, `sync-agents.sh` early-returns and generates nothing, and `sync-agents.sh --check` is a **vacuous exit 0** — so a green `--check` alone proves nothing here. Assert artifacts directly.
3. Generate:
   ```sh
   bash /path/to/docket/link-skills.sh
   bash /path/to/docket/sync-agents.sh
   ```
4. Assert on disk:
   ```sh
   ls .codex/agents/docket-*.toml | wc -l          # expect the full built-in set
   grep -c 'docket:dispatch:start' AGENTS.md        # expect 1
   ls ~/.codex/skills                               # expect docket skill symlinks
   bash /path/to/docket/sync-agents.sh --check; echo "exit=$?"   # expect exit=0
   ```
   The runbook must name the expected agents explicitly (this is what Assertion 2 checks): `docket-adr`, `docket-auto-groom`, `docket-auto-groom-critic`, `docket-brainstorm-consultant`, `docket-finalize-change`, `docket-implement-next`, `docket-integration-repair`, `docket-rebase-resolver`, `docket-status` — and instruct the operator to compare against `ls agents/docket-*.md` in the docket clone rather than trusting the printed list, since the set grows.
   Note `~/.codex/skills` is only populated if `~/.codex` already exists (`link-skills.sh` only links into a harness whose parent dir is present).
   **Pass when:** every `.toml` in the built-in set exists and parses, `AGENTS.md` carries exactly one dispatch block, `~/.codex/skills` holds the docket links, and `--check` exits 0.

**`## Phase 2 — Skills load and scripts run`**
- In Codex CLI, opened in the fixture: ask Codex to list its available skills, then ask it to run `docket-status`.
- Expected: the docket skills are listed; the convention loads; the skill's bash reaches the scripts through the facade — `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` — and it executes under **Codex's own sandbox**. This is the bash-compat smoke test; Cursor needed a sandbox/permissions guide at exactly this step (`docs/cursor/permissions.md`).
- Record verbatim: every approval prompt, sandbox denial, or path/permission error, and what (if anything) had to be allowed. That transcript is the raw material for a Codex analogue of the Cursor permissions guide.
- **Pass when:** docket skills load and `docket.sh preflight` runs to a `BOOTSTRAP=PROCEED` block without an unrecoverable sandbox denial.

**`## Phase 3 — Agents load`**
- Ask Codex to list its available agents (or inspect its `/agent` surface).
- Expected: the `docket-*` agents from the generated `.toml` files are visible by name.
- **Pass when:** every agent generated in Phase 1 is listed by name.

**`## Phase 4 — Dispatch honored`**
- **Restart the Codex session first** — Codex registers agents at process start, so a session open since before Phase 1 holds stale definitions (see setup.md's *Restart after (re)generating*).
- Directly invoke a skill with a pinned wrapper (`docket-status`).
- Expected: Codex delegates to the matching `docket-status` agent per the `AGENTS.md` dispatch block instead of running the skill inline — the Cursor inline-quirk test replayed for Codex.
- **Record the answer explicitly — this phase exists to settle a deferred decision.** ADR-0036 deferred the **user-level `~/.codex/AGENTS.md` dispatch** question to this validation; only the project-level `<repo>/AGENTS.md` block exists today. Capture: (a) is delegation automatic, prompted, or refused? (b) does dispatch work for a skill invoked outside the fixture repo, or is a user-level `~/.codex/AGENTS.md` needed for globally-scoped agents? A "user-level dispatch is needed" answer becomes a follow-up stub — **do not implement it here**.
- **Pass when:** there is a definitive, recorded answer to both (a) and (b) — "Codex refuses to delegate" is a passing outcome for this phase.

**`## Phase 5 — Pin honored`**
- Discover real Codex model slugs — never assume one; `config.yml.example`'s codex IDs are explicitly unvalidated examples:
  ```sh
  codex debug models | jq -r '.models[] | .slug'
  ```
- Pin one agent to a distinctive slug + effort in the fixture's `.docket.local.yml` (keys are bare, un-prefixed, harness-first):
  ```yaml
  agents:
    codex:
      status: { model: <slug-from-codex-debug-models>, effort: xhigh }
  ```
- Regenerate and confirm the values reached the wrapper, then restart Codex and dispatch the agent, asking it to report its own model identity:
  ```sh
  bash /path/to/docket/sync-agents.sh
  grep -E 'model|model_reasoning_effort' .codex/agents/docket-status.toml
  ```
- Expected: the `.toml` carries `model = "<slug>"` and `model_reasoning_effort = "xhigh"` (docket's `effort:` maps to that key **verbatim** — there is no `max`→`xhigh` remap at this layer), and the spawned agent reports the pinned model.
- Note: with **no** `agents.codex.*` override the wrappers carry the built-in **Claude** model IDs, which Codex cannot run — `sync-agents.sh` warns about exactly this. A pin is therefore mandatory for this phase to mean anything.
- **Pass when:** there is a definitive recorded answer to "does the spawned agent run the pinned model/effort?" — including a negative.

**`## Phase 6 — End-to-end metadata write`**
- From inside Codex, run a trivial `docket-new-change` stub through to completion.
- Expected: the change-file commits land on `origin/docket`; the Board pass reports `board inline changed pushed`; the board renders the new stub. Verify against the remote, not the local tree:
  ```sh
  git -C .docket log --oneline -3 origin/docket
  ```
- This proves the whole producer loop — preflight, metadata worktree sync, must-land push, board render — works end to end under a non-Claude harness.
- **Pass when:** the stub's commits are on `origin/docket` and the Board pass reported `board inline changed pushed`.

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook && bash tests/test_codex_runbook.sh; echo "exit=$?"
```
Expected: every line `ok - …`, `exit=0`.

- [ ] **Step 5: Mutation-test the two load-bearing guards**

A guard is code: prove each can redden, or it is decoration (LEARNINGS, guards-are-code). Run each mutation, confirm `NOT OK`, then **revert it**.

```bash
cd /Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook
# (a) Agent-set completeness must catch a NEW agent the runbook does not name.
touch agents/docket-zzz-probe.md
bash tests/test_codex_runbook.sh | grep -E 'missing agents|names every generated agent'
rm agents/docket-zzz-probe.md

# (b) Cited-path existence must catch a fabricated path.
cp docs/codex/validation-runbook.md /tmp/rb.bak
printf '\nBogus: `docs/codex/does-not-exist.md`\n' >> docs/codex/validation-runbook.md
bash tests/test_codex_runbook.sh | grep -E 'nonexistent paths|every cited repo path exists'
cp /tmp/rb.bak docs/codex/validation-runbook.md

# (c) Substring boundary: naming ONLY the -critic agent must NOT satisfy docket-auto-groom.
#     Verify the boundary regex by hand — expect no match (exit 1).
printf 'docket-auto-groom-critic\n' | grep -qE -- 'docket-auto-groom([^A-Za-z0-9_-]|$)'; echo "boundary-check exit=$? (expect 1)"

# Confirm clean revert.
bash tests/test_codex_runbook.sh; echo "exit=$? (expect 0)"
```
Expected: (a) prints `NOT OK - runbook missing agents: docket-zzz-probe`; (b) prints `NOT OK - runbook cites nonexistent paths: docs/codex/does-not-exist.md`; (c) prints `boundary-check exit=1`; final run `exit=0`.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook
git add docs/codex/validation-runbook.md tests/test_codex_runbook.sh
git commit -m "feat(0078): Codex CLI live-validation runbook + structural guard

Six-phase operator checklist proving docket works natively under Codex CLI.
Guard derives the agent set from agents/docket-*.md and asserts every cited
committed path exists (the error class that bit this change's own spec).
Scoped to the native path; 0079 runner-dispatch is out of scope."
```

---

### Task 2: Discoverability — surface the runbook from setup.md and README

**Files:**
- Modify: `/Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook/docs/codex/setup.md` (the `## Verifying it works` section)
- Modify: `/Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook/README.md`
- Modify: `/Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook/tests/test_codex_runbook.sh`

**Interfaces:**
- Consumes: `docs/codex/validation-runbook.md` (Task 1) and `tests/test_codex_runbook.sh`'s `ok`/`no` helpers + `fail` accumulator (Task 1).
- Produces: nothing later tasks depend on — this is the final task.

A doc nobody can find is not shipped (LEARNINGS #49: a knob that merely *works* is not done — surface it end-to-end). `README.md` already links the Cursor guide as `](docs/cursor/permissions.md)`; the runbook gets the same treatment.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_codex_runbook.sh`, immediately **before** the final `exit $fail` line:

```bash
# --- Assertion 8: the runbook is discoverable -------------------------------------------------
SETUP="$REPO/docs/codex/setup.md"
README="$REPO/README.md"
# setup.md is the runbook's sibling; link it relatively from the section it deepens.
if grep -qF -- '](validation-runbook.md)' "$SETUP"; then ok "setup.md links the runbook"; else no "setup.md links the runbook"; fi
if grep -qF -- '](docs/codex/validation-runbook.md)' "$README"; then ok "README links the runbook"; else no "README links the runbook"; fi
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook && bash tests/test_codex_runbook.sh; echo "exit=$?"
```
Expected: `NOT OK - setup.md links the runbook` and `NOT OK - README links the runbook`, `exit=1`. Every earlier assertion still `ok`.

- [ ] **Step 3: Add the setup.md pointer**

In `docs/codex/setup.md`, find the `## Verifying it works` section (it lists three static checks and then describes what `sync-agents.sh --check` validates). Append this paragraph at the end of that section, immediately before the `## Restart after (re)generating` heading:

```markdown
Those three checks are the static ones. To validate the whole loop live — skills loading, docket's
scripts running under Codex's sandbox, agents listing, dispatch and the model pin actually being
honored, and metadata writes landing on `origin/docket` — work through the
[Codex live-validation runbook](validation-runbook.md), which drives each of these end to end in a
fixture repo and records the observed outcome.
```

- [ ] **Step 4: Add the README link**

In `README.md`, locate the existing Codex prose (the section that already discusses the Codex harness / `agent_harnesses`, near the `](docs/cursor/permissions.md)` link to the Cursor guide). Add the runbook alongside the existing Codex setup reference so both Codex docs are reachable from one place:

```markdown
See [Codex setup](docs/codex/setup.md) to enable the harness, and the
[Codex live-validation runbook](docs/codex/validation-runbook.md) to verify the whole loop live.
```

Match the surrounding sentence style rather than pasting verbatim if the section reads differently — the assertion only requires the `](docs/codex/validation-runbook.md)` link target to be present.

- [ ] **Step 5: Run test to verify it passes**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook && bash tests/test_codex_runbook.sh; echo "exit=$?"
```
Expected: all `ok - …`, `exit=0`.

- [ ] **Step 6: Run the WHOLE suite**

Never trust only the tests this plan enumerated — an out-of-goal regression is exactly what the tests outside the goal set exist to catch (LEARNINGS, enumerated-floor (c): a change reddened a pre-existing sentinel in a file its plan never named). Run **one foreground call**:

```bash
cd /Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook && fail=0; for f in tests/*.sh; do out="$(bash "$f" 2>&1)"; if [ $? -ne 0 ]; then echo "=== FAIL $f"; printf '%s\n' "$out" | grep '^NOT OK' ; fail=1; fi; done; echo "SUITE fail=$fail"
```
Expected: `SUITE fail=0`.

Two suite files are the likeliest to notice this change and must be read if they redden:
- `tests/test_script_contracts_coverage.sh` — asserts every `scripts/*.sh` has a co-located `.md`. This change adds no script, so it must stay green; if it reddens, something was added under `scripts/`.
- `tests/test_cursor_permissions_docs.sh` — the precedent guard; a README edit that disturbs the `](docs/cursor/permissions.md)` link reddens it.

If any test fails, fix the cause — never weaken the assertion.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/codex-cli-validation-runbook
git add docs/codex/setup.md README.md tests/test_codex_runbook.sh
git commit -m "docs(0078): surface the Codex validation runbook from setup.md + README

A runbook nobody can find is not shipped; guard both links."
```

---

## Notes for the implementer

- **The runbook's audience is one person running it once, carefully.** Favor exact, copy-pasteable commands and unambiguous expected output over prose. Every step needs a `- [ ]` box and an observable outcome.
- **A "no" is data, not failure.** Phases 4 and 5 pass by producing a definitive answer. Do not write the runbook as though Codex is expected to succeed at everything — its whole purpose is to find out.
- **Do not fix anything the runbook implies is broken.** Gaps become follow-up stubs at close-out. That boundary is in the spec's *Out of scope* and in the change's.
- **Do not touch docket metadata from this worktree** — no change files, no `BOARD.md`, no ADRs. This branch carries only the plan, the runbook, the test, and the two doc edits.
