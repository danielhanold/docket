---
name: docket-convention
description: Use when any docket skill runs — docket-new-change, docket-groom-next, docket-implement-next, docket-status, docket-finalize-change, and docket-adr load this first (their blocking Step 0) — or when you need to understand how docket tracks work. The shared contract — .docket.yml configuration, directory layout, the change manifest and lifecycle, ADR format, build-readiness and selection, the bootstrap guard, and the branch model. Pure reference — defines the convention; performs no reads, writes, or git operations.
---

# docket-convention — the shared contract (pure reference)

This skill defines the docket convention and does nothing else: no procedure, no reads or writes, no git. The seven operating skills load it as their blocking Step 0 and use its vocabulary without restating it.

## Convention

docket tracks planned work as **changes** — one markdown file each, roughly one PR — and records architecture decisions as **ADRs**. This skill is the single source of the convention; the operating skills (docket-new-change, docket-groom-next, docket-implement-next, docket-status, docket-finalize-change, docket-adr) load it at startup as their blocking Step 0 and never restate it.

### Configuration — `.docket.yml` (optional, committed on the default branch)

Read at startup by every docket skill. Absent ⇒ all defaults. It is **committed** (never gitignored), because it governs cross-agent coordination and must be identical for every clone, agent, and device.

```yaml
# .docket.yml — committed on the repo's DEFAULT branch (origin/HEAD); read by every docket skill at startup
metadata_branch: docket      # docket (default) | main  — where PM commits land (see "Branch model")
integration_branch: auto     # auto (→origin/HEAD, fallback main) | main | develop  — where code lands; feature branches cut from origin/<this>
changes_dir: docs/changes    # default
adrs_dir: docs/adrs          # default
results_dir: docs/results    # default  — close-out 'results' artifacts (build-time files, like plans)
auto_groom: false            # repo default for autonomous grooming; per-change auto_groomable overrides
board_surfaces: [inline]     # which derived board view(s) to render: inline (BOARD.md) and/or github; [] = none
finalize:                    # merge gate: rebase onto base + re-test before merge
  gate: local                # local (default, on) | ci | both | off  — off = pre-0015 (trust the PR's CI)
  test_command:              # OPTIONAL; unset => finalize auto-detects the suite
github_project:              # {owner, number} of the auto-managed Projects v2 board; unset ⇒ auto-create on first github sync
agent_harnesses: [claude]    # harnesses the per-repo agent pass generates machine-local wrapper
                             # files; default [claude]. e.g. [claude, cursor]
                             # for a Cursor repo.
agents:                      # harness-first per-skill subagent model/effort; see "Agent layer" below
skills:                      # pluggable workflow skills; unset key = the superpowers default shown
  brainstorm: superpowers:brainstorming
  plan:       superpowers:writing-plans
  build:      superpowers:subagent-driven-development   # e.g. `auto` to build inline without SDD
  review:     superpowers:requesting-code-review
  finish:     superpowers:finishing-a-development-branch
build:                       # per-role SDD build model IDs (change 0044); unset ⇒ SDD's own Model Selection
  implementer:                # <model-id> — per-task implementer + fix subagents
  reviewer:                   # <model-id> — task-reviewer + final code-reviewer
```

`.docket.yml` lives on the repo's **default branch (`origin/HEAD`)**, NOT on the integration branch — `integration_branch` is a value *read from* the file, so the file cannot be located *by* it. The file then **declares `integration_branch`**, which may differ from the default branch. `metadata_branch` resolves where PM commits land; `integration_branch` (default `auto` → `origin/HEAD`, fallback `main`; explicit `main`/`develop` verbatim) resolves where code lands — feature branches always cut from `origin/<integration_branch>`. A genuinely absent file ⇒ defaults apply (`metadata_branch: docket`, `integration_branch: auto`); an unreachable `origin` is never silently treated as "file absent." **Backward-compatible opt-out:** pinning `metadata_branch: main` (with `integration_branch: main`) reproduces today's single-branch behavior exactly — no `docket` branch, no `.docket/` worktree.

**Config layers.** Two more optional layers: a **user-level** `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` (accepts the full `.docket.yml` schema; applies to every repo on this machine) and a **machine-local** `<repo>/.docket.local.yml` (gitignored; this repo, this machine only). Every key resolves **per-field**, four layers deep: **repo-local > repo-committed > global > built-in** (map-valued `skills:`/`agents:` merge field-by-field, the same resolver at each layer). **Coordination-key fence:** a key whose effect writes shared, non-re-derivable state (`metadata_branch`, `integration_branch`, `changes_dir`/`adrs_dir`/`results_dir`, `github_project`, and `board_surfaces`' `github` token) is per-repo-only — set in either machine-scoped file it is loudly warned-and-ignored, never honored, never fatal (the classification rule is ADR-0019). Everything else is global-able; `agent_harnesses`' per-pass scoping is in *Agent layer*. The per-key classification table and the misplaced/malformed-file postures are authoritative in the contract [`scripts/docket-config.md`](../../scripts/docket-config.md); the legacy `agents.yaml` auto-migration is owned by `sync-agents.sh` (ADR-0019).

This resolution — repair `origin/HEAD`, read `.docket.yml` authoritatively, apply every default, resolve `integration_branch` — is performed deterministically by **`docket-config.sh --export`** (invocation: see the *Step-0 preamble*). The prose in this section is the spec the script implements; its interface and mechanics are in its contract [`scripts/docket-config.md`](../../scripts/docket-config.md).

**Reaching the helper scripts (`DOCKET_SCRIPTS_DIR`).** Every helper script this convention names lives in the docket clone's `scripts/` directory, NOT in the consuming repo; a skill resolves each as `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh`. `install.sh` injects the variable into the shell profile and Claude Code's user-level `settings.json` `env` (mechanics: `scripts/ensure-docket-env.md`); the `:?` makes a missing/incomplete install **fail loud** — the executing agent stops and fixes the install instead of silently degrading to hand-worked operations. Every env var docket introduces is **DOCKET_-namespaced** to avoid collisions in the user's shared shell.

**Script contracts (`scripts/<name>.md`).** Every `scripts/<name>.sh` has a co-located `scripts/<name>.md` contract — its authoritative spec (Purpose / Usage / Behavior / Exit codes / Invariants). Read it for a script's internals; reach it from a consuming repo the same way as the script. (`docket-convention/github-board-mirror.md` is skill-reference, not a single-script contract.)

**`board_surfaces` — the board as 0..n derived views.** The board is a *derived view* over the change files; `board_surfaces` lists which surfaces to render. Members: `inline` (the committed, offline-safe `BOARD.md`) and `github` (the one-way Issues + Projects v2 mirror, see *GitHub board mirror*); default `[inline]`, and `github` is strictly opt-in (an existing repo never starts minting issues until it asks). **`[]` disables the board entirely** — the change files plus git history remain the only (and fully authoritative) record. An unknown token is warned-and-ignored (a typo must never abort a build); a non-GitHub remote silently drops `github`; `github_project` is consulted only when `github` is enabled, and is minted-and-written-back on first sync if unset.

**`finalize` — the rebase-retest merge gate.** `finalize.gate` (`local` default · `ci` ·
`both` · `off`) governs `docket-finalize-change`'s merge step: before docket merges, it
rebases the feature branch onto `origin/<integration_branch>` and re-validates the merged
result, merging only if green. `local` runs the repo's suite locally (auto-detected, or the
`finalize.test_command` override); `ci` polls GitHub checks; `both` requires both; **`off`**
restores pre-gate behavior (merge trusting the PR's own CI). The gate is **finalize-only** —
the `docket-status` sweep never merges, so it has nothing to gate. Details: the gate flow and
its two judgment-tier agents live in `docket-finalize-change`.

### Step-0 preamble (every operating skill)

Every operating skill starts identically; skill bodies compress to a pointer here plus one line naming where their writes land.

1. Load this convention (blocking).
2. Resolve config + the bootstrap verdict: `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"` (fail-closed; read-only). Act on `BOOTSTRAP` — `PROCEED` → continue; `STOP_MIGRATE` → refuse and point at `migrate-to-docket.sh`; `CREATE_ORPHAN` → opt into `docket-config.sh --bootstrap` (fresh repo only).
3. Ensure + sync the **metadata working tree**. In `docket`-mode: the persistent `.docket/` worktree parked on `docket` (state-specific create per *Branch model*, idempotent); **sync before any read** — `git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`; pushes target `origin/docket`. In single-branch/`main`-mode this degrades to the primary working tree on the integration branch (no `.docket/`): `git pull --rebase` and push on `origin/<metadata_branch>` (= `origin/<integration_branch>` there). Skill prose that says "`.docket/`" / "`origin/docket`" reads as the metadata working tree / `origin/<metadata_branch>` in `main`-mode.

All metadata reads and writes happen in that tree on `metadata_branch`, pushed to its remote immediately.

### Agent layer — model/effort-pinned subagents (change 0016)

Each **autonomous** docket skill can run as a model/effort-pinned **subagent** instead of inline at the session model. Five skills get a wrapper — `docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`; the two **interactive** skills (`docket-new-change`, `docket-groom-next`) stay inline and only surface an **advisory** recommended model/effort at startup (a skill cannot force the session model). `docket-convention` is not an agent — it is injected into every wrapper via `skills:`.

A wrapper is a thin generated file: it pins `model` + `effort` and injects the skill via `skills: [<skill>, docket-convention]`; the skill body stays the single source of behavior. Because a subagent cannot pause to ask a human, every autonomous wrapper carries an **abort-and-report** rule: an unmet precondition or blocking ambiguity (e.g. a PR not actually approved, a merge conflict, a dirty worktree) is surfaced and stopped on — never turned into an interactive prompt. Wrappers are generated by `sync-agents.sh` from the layered config (precedence: repo-local > repo-committed > global > built-in); an agent with no entry in any layer defaults to `model: inherit` with no `effort`.

**`build:` — build-phase dispatch model IDs (change 0044).** `build.implementer` / `build.reviewer` govern only `docket-implement-next`'s Step 5/6 build-phase dispatches: `implementer` covers the per-task implementer and fix subagents — SDD's (`superpowers:subagent-driven-development`) own already-required `model:` field; `reviewer` covers both the per-task task-reviewer (also SDD's `model:` field) and the Step 6 final whole-branch code-review dispatch, which `docket-implement-next` runs via the separately-resolved `$SKILL_REVIEW` rather than as an SDD sub-dispatch. Each value is a **direct model ID**, passed straight through to SDD's `model:` field — the same harness-neutral passthrough contract the `agents:` block uses, so under Claude Code it is a Claude alias/ID and under another harness (e.g. Cursor) one of that harness's own model IDs; docket does not interpret or validate the string. `build:` is global-able (not a coordination key) and layers like any other config key. An unset role — or an absent `build:` block entirely — defers to SDD's own Model Selection judgment, so this surface is purely additive: today's behavior is exactly the unset case.

**Composition (change 0017).** `docket-implement-next` dispatches the `docket-status` subagent (step 0) and the `docket-adr` subagent (step 6); `docket-auto-groom` dispatches the `docket-auto-groom-critic` subagent for its adversarial gate. These dispatches are **foreground** (the parent suspends until the child returns) and **unconditional**; their contract is **git state** on `origin/docket` (for adr, plus a published ADR on the integration branch), re-read after a re-sync — never an in-context return. `docket-finalize-change` dispatches the `docket-rebase-resolver` subagent when its merge gate hits a rebase conflict and the `docket-integration-repair` subagent when the rebased suite is red — also foreground, but their reports flow **back to finalize in-context** to gate the merge, and they act in the feature worktree, not on `origin/docket`. Each dispatched agent runs at the model/effort its own wrapper resolves — literal tiers are never restated in dispatch prose, so an override can never drift from the documentation. Three of the **eight** generated wrappers wrap **no skill** — `docket-auto-groom-critic`, `docket-rebase-resolver`, `docket-integration-repair`; each loads only `docket-convention`, so it inherits no caller bias, and all are auto-discovered by `sync-agents.sh`'s `agents/docket-*.md` glob. (Five *skills* get a wrapper; these three are wrappers that wrap no skill — eight wrappers, five skills.)

**Configuring the layer** — the harness-first `agents:` blocks (`default:` + per-harness keys), `agent_harnesses` scoping, harness-portable model IDs (ADR-0015), always-full-set generation, the Cursor dispatch rule, `effort: auto` vs omitted, and `sync-agents.sh` / `--check` mechanics — is a separate read: **read [`references/agent-layer.md`](references/agent-layer.md) now (blocking) before configuring `agents:`/`agent_harnesses:` or running/debugging `sync-agents.sh`.**

### Skill layer — pluggable workflow skills (change 0049)

docket's five workflow steps are **pluggable roles**: the optional `skills:` map rebinds each to any skill name, or to the sentinel `auto`. An unset key defaults to the superpowers skill — an absent map is byte-identical to pre-0049 behavior.

| Role | Default skill | Invoked by | `auto` / fallback artifact — stop-point |
|---|---|---|---|
| brainstorm | `superpowers:brainstorming` | `docket-new-change` §2, `docket-groom-next` | a spec file at the configured spec path; stop at the spec |
| plan | `superpowers:writing-plans` | `docket-implement-next` §4 | a plan file on the feature branch, recorded in `plan:` |
| build | `superpowers:subagent-driven-development` | `docket-implement-next` §5 | the plan executed on the feature branch |
| review | `superpowers:requesting-code-review` | `docket-implement-next` §6 | a whole-branch review before the PR opens |
| finish | `superpowers:finishing-a-development-branch` | `docket-implement-next` §7; `docket-finalize-change` close-out | a pushed feature branch + open PR — never merged; stop |

- **Passthrough.** A value is passed verbatim to the Skill tool — never validated against a registry (the ADR-0015 passthrough philosophy; exactly what lets any third-party or in-repo skill plug in). Unknown *role keys* are warned-and-ignored.
- **`auto` sentinel.** No skill is invoked; the running agent does the step itself at whatever model it already runs at. The per-role fallback defines only the **final artifact / stop-point** (column 4) — never the method.
- **Missing-skill rule — degrade to auto + warn.** If the resolved skill cannot be invoked at runtime, the invoking skill degrades to that role's `auto` fallback and warns prominently — in the run output and (for plan/build/review/finish) in the PR body. Softer than abort-and-report because skill availability is per-machine, not repo state.
- **Resolution** is deterministic via `docket-config.sh --export`, which emits `SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH` (defaulted when unset); skill bodies read the variable, never re-parse YAML. `docket-finalize-change`'s merge gate (`finalize.gate`) still validates regardless of the resolved build method.

### Directory layout (paths relative to the configured knobs)

```
<changes_dir>/            # default docs/changes/
  active/                 # every NON-terminal change:   <id>-<slug>.md            (id zero-padded to 4 digits)
  archive/                # the two terminal outcomes:    <YYYY-MM-DD>-<id>-<slug>.md
  BOARD.md                # generated board (NEVER hand-edited); spans active + archive
  README.md               # small static blurb linking to BOARD.md (NOT generated)
  LEARNINGS.md            # curated build-loop lessons; harvested at close-out (see "Learnings ledger")
<adrs_dir>/               # default docs/adrs/  — flat; ADRs are NEVER archived
  <NNNN>-<slug>.md        # immutable once Accepted (only its status: line ever changes)
  README.md               # generated ADR index
<results_dir>/            # default docs/results/  — optional close-out artifacts (feature-branch build files; NEVER archived)
  <YYYY-MM-DD>-<slug>-results.md
```

The `archive/` filename date prefix is **UTC**: the **merge commit's** date for `done`, the **kill commit's** date for `killed`.

In `docket`-mode all of the above lives on the `docket` branch, written through a persistent, gitignored **`.docket/` metadata worktree** parked on that branch (it is `.docket/`, **not** under `.worktrees/`, to avoid slug collisions and the ephemeral-worktree prune blast radius — see "Branch model").

### Change manifest (frontmatter at the top of each change file)

```yaml
---
id: 7                     # integer; zero-padded to 4 digits in the filename
slug: quicklook-interactions
title: Quick Look interactions — external links + local images
status: proposed          # proposed | in-progress | blocked | deferred | implemented | done | killed
priority: medium          # low | medium | high | critical   (default: medium)
created: 2026-05-30
updated: 2026-05-30
depends_on: [4]           # change ids that must reach `done` (PR merged) first
related: [4, 6]           # cross-links the reconcile pass reads
adrs: [24]                # ADRs this change cites or produces
spec:                     # superpowers design doc path; set at brainstorm (propose) time, on metadata_branch
plan:                     # plan FILE lives on the feature branch; this FIELD is set in the main tree at build time
results:                  # results FILE on the feature branch; this FIELD set in the main tree at close-out (optional)
trivial: false            # true = no spec needed (small mechanical change); still build-ready
auto_groomable:           # tri-state: unset ⇒ inherit the repo's auto_groom; true/false ⇒ explicit override
branch:                   # planned feat/<slug> name, set on claim; branch itself created at build (step 4)
pr:                       # set when the PR is opened
issue:                    # GitHub mirror issue number; minted on first `github` sync (one-way), shape of pr:
blocked_by:               # free text; set only when status: blocked
reconciled: false         # set true after the just-in-time reconcile pass
---
```

### Change body sections

- `## Artifacts` — **first body section** (immediately after the frontmatter closing `---`, above `## Why`). Marker-bounded (`<!-- docket:artifacts:start (generated — do not hand-edit) -->` / `<!-- docket:artifacts:end -->`); rendered by `render-change-links.sh` from frontmatter; **never hand-edited** — the renderer is the sole writer. The change template seeds it empty; field-writing skills regenerate it after every frontmatter update.
- `## Why` — the motivation, as detailed as warranted (no length limit).
- `## What changes` — scope of the work.
- `## Out of scope` — explicit non-goals.
- `## Open questions` — unknowns to resolve during reconcile/design.
- `## Reconcile log` — dated entries appended by the implementer's reconcile pass.
- `## Auto-groom blocked` — dated abstain record appended by `docket-auto-groom`; contents and lifecycle (including removal on re-arm) are defined by the *Autonomous grooming* shared definition below.
- `## Why deferred` / `## Why killed` — added when entering those states.

The change body is a **PM-altitude proposal** (intent + scope). Detailed design lives in the linked superpowers spec; the task breakdown in the linked superpowers plan. Different zoom levels, no duplication.

### ADR file (`<adrs_dir>/<NNNN>-<slug>.md`)

```yaml
---
id: 24                    # integer; zero-padded to 4 digits in the filename
slug: quicklook-interaction-limits
title: Quick Look interaction limits under sandbox
status: Accepted          # Accepted | Superseded by ADR-NN | Reversed by ADR-NN | Deprecated
date: 2026-05-20
supersedes: []            # ADR ids this replaces (sets the old one's status)
reverses: []              # ADR ids this undoes
relates_to: [22]          # cross-links
change: 4                 # back-link: the change that produced this decision, if any
---

## Context       — the forces / problem that prompted the decision
## Decision      — what was chosen, and the rule a reader needs to know
## Consequences  — what it enables, what it costs, what is given up
```

An `Accepted` ADR is immutable except its `status:` line; a non-reversing context change is appended as a dated `## Update` note, never an edit to the decision. A reversal/supersession is always a **new** ADR.

### Lifecycle — seven states

```
                         ┌──────────────── deferred ──────────────┐
                         │ (conscious shelve; revive → proposed)   │
                         ▼                                          │
  proposed ──claim──▶ in-progress ──PR open──▶ implemented ──merge+sweep──▶ done
     │                    │                                                  (archive/)
     │                    └──blocker──▶ blocked ──clears──▶ in-progress
     │
     └──── killed (obsolete — from proposed, or from in-progress via reconcile; → archive/) ────▶
```

| status | meaning | directory |
|---|---|---|
| `proposed` | drafted, awaiting work | `active/` |
| `in-progress` | claimed, being built | `active/` |
| `blocked` | external blocker (`blocked_by:`) | `active/` |
| `deferred` | consciously shelved, may revive | `active/` |
| `implemented` | built, PR open — **human merge gate** | `active/` |
| `done` | PR merged, filed away (happy terminal) | `archive/` |
| `killed` | abandoned — obsolete or never shipped (sad terminal) | `archive/` |

**Rules.** `active/` holds every non-terminal status; `archive/` holds the two terminal outcomes. The single physical move (`active/ → archive/`, date-prefixed) happens once on the terminal transition and is **idempotent**: re-pull, re-read `status` on `metadata_branch`, no-op if already terminal. `deferred` may be entered from `proposed` or `in-progress` (add `## Why deferred`) and revived to `proposed`; clearing a blocker or reviving is a one-line frontmatter edit, no move. A change whose `depends_on` is unsatisfied is *implicitly* blocked — the selector skips it (no status change) and the board shows it **waiting on #N**. A dependency is **satisfied when it reaches `done`**. If `#N` is still `implemented` (PR open, unmerged), the dependent is gated on a human merge — the board flags **waiting on #N — needs your merge**, distinct from **waiting on #N — not yet built**. Reserve explicit `blocked` for external blockers the system can't infer.

**Board refresh on status writes.** Any skill that writes a change's `status:` refreshes **each enabled board surface** (the Board pass) immediately after — the `inline` surface regenerates `BOARD.md` in a separate commit; the `github` surface runs the mirror upsert (best-effort); `board_surfaces: []` makes this a no-op. The board is a derived view and must never trail the change files.

### Build-readiness & selection (shared definition)

A change is **build-ready** — eligible for `docket-implement-next` — only when it is `proposed`, has a `spec:` **or** `trivial: true`, and all `depends_on` are satisfied (`done`). A `proposed` change with neither a spec nor `trivial: true` is **needs-brainstorm** (not build-ready). The implementer's deterministic selection order is `priority` (`critical` > `high` > `medium` > `low`) → age (`created`) → **lowest `id`**.

### Autonomous grooming (shared definition)

A change's **effective auto-groomable** value is its `auto_groomable:` override when explicitly set, else the repo's `auto_groom` knob (default `false`). The field is human input with one exception: `docket-auto-groom`'s abstain is the single agent write (it flips the override to `false`).

A stub is **autonomous-eligible** — selectable by `docket-auto-groom` — when it is needs-brainstorm (`proposed`, no `spec:`, not `trivial: true`) AND effective auto-groomable. Unsatisfied `depends_on` does NOT exclude it (the same design-ahead rule as interactive grooming; the implementer's reconcile re-validates at build time). Ranking is the same deterministic selection order as build-ready selection.

**Abstain rule.** When autonomous grooming cannot safely default a decision, it emits NO spec; it flips `auto_groomable: false` and appends a dated `## Auto-groom blocked` body section. The stub stays needs-brainstorm — out of the autonomous queue, still in the interactive one. Re-arm = a human supplies the missing context, flips the flag back to `true`, and DELETES the `## Auto-groom blocked` section (git history keeps it) — the section's presence is what drives the board's needs-you cell and `docket-groom-next`'s first band, so a stale section would mislabel a re-armed stub. Kill and defer are never autonomous: they surface inside the blocked section as recommendations.

**Interactive selection bands.** `docket-groom-next` still sees every needs-brainstorm stub, but its default order prefers stubs that need a human: (1) abstained (`## Auto-groom blocked` present), (2) effective `auto_groomable: false`, (3) effective auto-groomable — flagged "docket-auto-groom will handle it unless you want it now." Within each band, the deterministic selection order applies. The board renders abstained stubs as **auto-groom blocked — needs you**, distinct from plain needs-brainstorm.

### Learnings ledger

`<changes_dir>/LEARNINGS.md` — the project's **build-loop memory**: curated, hand-edited lessons, on `metadata_branch` only (never published to the integration branch; unlike the board it is prose, never regenerated). Flat dated entries, **newest first**, one to three lines with provenance: `- 2026-06-12 (#12, PR #7) — <what happened>. Apply: <the rule>.`

**Writing:** only the harvest at close-out appends (single source: the *Harvest learnings* step in `docket-finalize-change`; `docket-status`'s sweep invokes it by reference). Zero entries is normal; kills are not harvested. **Reading:** `docket-implement-next` at plan time and review; `docket-groom-next` before a brainstorm. **Distilling:** append-only until ~300 lines; the next harvest past the cap also distills — merge near-duplicates, drop entries promoted to CLAUDE.md or this convention. Distillation is **compression, not destruction** (git history keeps everything); durable conventions belong in CLAUDE.md, and promotion removes the entry here.

### GitHub board mirror (shared definition)

The `github` board surface mirrors each change to one GitHub issue (and one Projects v2 item) — **strictly one-way**: change files are the source of truth, the mirror is derived output that is **never read back**. It rides in the Board pass (`docket-status`) and is **best-effort** (network + `gh` auth; self-heals next pass; never aborts a build); its external-write mechanics are owned by the deterministic `github-mirror.sh` (not agent-constructed `gh` calls) — the Board pass only invokes it. **Full mechanics — the `issue:` upsert, the `docket:` label namespace, the status→issue mapping across all seven states, the issue body, and Projects v2 — are in [`github-board-mirror.md`](github-board-mirror.md); read it when `board_surfaces` includes `github`.**

**Derived-view script family.** The deterministic scripts that produce derived views from the change files: `render-board.sh` (the `BOARD.md` inline surface), `github-mirror.sh` (the GitHub Issues/Projects mirror), `render-change-links.sh` (per-change `## Artifacts` link-block renderer — sole writer of that block per ADR-0012 script-vs-model boundary; offline, falls back to bare code-formatted paths when no GitHub remote is detected). Each script is the sole writer of its output; field-writing skills call `render-change-links.sh` immediately after every frontmatter field write.

### Bootstrap guard (`docket`-mode first-run safety)

At startup, after resolving config, when `metadata_branch == docket`, fetch origin and evaluate a 2×2 over two probes (both stated over the **same vocabulary**):

- **`DOCKET`** = the `docket` branch exists (origin OR local).
- **`LIVE`** = the **live planning surface** still sits on the integration branch — probe ONLY the pruned surface (`<changes_dir>/active`, `README.md`, `BOARD.md`) via `git ls-tree origin/<integration_branch>`; `archive/`, `<adrs_dir>/`, and pre-migration specs deliberately *stay* on integration, so probing them would read `LIVE` forever. An unresolvable `origin/<integration_branch>` is a **hard config error**, not `¬LIVE`.

| | `LIVE` | `¬LIVE` |
|---|---|---|
| **`¬DOCKET`** | existing single-branch repo → **STOP**, point to `migrate-to-docket.sh`; never auto-create or move data | fresh repo → create the empty orphan `docket`, push, **proceed** |
| **`DOCKET`** | **half-migrated** (interrupted run) → **STOP**, point back to `migrate-to-docket.sh` to finish its prune | migrated → **proceed** |

This 2×2 is the spec the same `docket-config.sh` implements, reported as its `BOOTSTRAP=` verdict — `PROCEED` (migrated or main-mode), `STOP_MIGRATE` (existing single-branch or half-migrated), or `CREATE_ORPHAN` (fresh). The skill acts on the verdict: STOP and point at `migrate-to-docket.sh` on `STOP_MIGRATE`, or opt into the orphan-create write (`docket-config.sh --bootstrap`) on `CREATE_ORPHAN`. How the script probes `DOCKET`/`LIVE`, why the default `--export` is read-only, and the `--bootstrap` write path (guarded to the `¬DOCKET ∧ ¬LIVE` cell) are in its contract [`scripts/docket-config.md`](../../scripts/docket-config.md).

The guard is a no-op in `main`-mode (`DOCKET`/`LIVE` are only evaluated when `metadata_branch == docket`). The migration itself lives in the standalone `migrate-to-docket.sh`.

### Branch model

Metadata (change files, `BOARD.md`, ADRs, specs) commits to `metadata_branch` (default `docket`) via the **metadata working tree** — the primary working tree on the integration branch in single-branch (`main`) mode, the persistent `.docket/` worktree in `docket`-mode — and is **always pushed to its remote immediately** (the backlog, board, specs, and ADRs stay browsable on the remote at all times).

A change's `feat/<slug>` branch is **ALWAYS cut from `origin/<integration_branch>`** — `metadata_branch` only redirects bookkeeping commits, never where code branches start. The feature branch adds only the plan + results + code and **never modifies** docket metadata.

On a terminal transition (`done` *or* `killed`), the driving skill runs the shared **terminal close-out** sequence — archive, re-render, **terminal-publish** (copying the archived change file + its `spec:` + the `Accepted` ADRs in `adrs:` from `origin/docket` onto the integration branch via `git checkout origin/docket -- <paths>`, never a `git merge docket` — the only flow of metadata onto the code line, also refreshing the integration-branch ADR index whenever the commit publishes an ADR), cleanup, board. Ordering, per-caller failure postures, and the `main`-mode degradation live in **[`references/terminal-close-out.md`](references/terminal-close-out.md) — read it before driving any terminal transition.** After a merge lands, both merge sites run the best-effort, FF-only `sync-integration-branch.sh` once at end of run to fast-forward the clone's local `<integration_branch>` checkout (a no-op in `main`-mode and on any non-FF/dirty/feature-branch tree).
