---
name: docket-convention
description: Use when any docket skill runs — docket-new-change, docket-groom-next, docket-implement-next, docket-status, docket-finalize-change, and docket-adr load this first (their blocking Step 0) — or when you need to understand how docket tracks work. The shared contract — .docket.yml configuration, directory layout, the change manifest and lifecycle, ADR format, build-readiness and selection, the bootstrap guard, and the branch model. Pure reference — defines the convention; performs no reads, writes, or git operations.
---

# docket-convention — the shared contract (pure reference)

This skill defines the docket convention and does nothing else: no procedure, no reads or writes, no git.

## Convention

docket tracks planned work as **changes** — one markdown file each, roughly one PR — and records architecture decisions as **ADRs**. This skill is the single source of the convention; the operating skills (docket-new-change, docket-groom-next, docket-implement-next, docket-status, docket-finalize-change, docket-adr) load it at startup as their blocking Step 0, use its vocabulary, and never restate it.

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
change_types: [chore, docs, feat, fix, refactor, perf]  # a higher layer REPLACES this list, never merges
auto_capture:                # autonomous mid-run capture of discovered work into stubs
  enabled: false             # breaking: the old scalar `auto_capture: true` is now a hard error
  types: all                 # `all` or a change_types subset; leaves resolve independently
board_surfaces: [inline]     # which derived board view(s) to render: inline (BOARD.md) and/or github; [] = none
terminal_publish: false      # false (default) = terminal records (change file, spec, Accepted ADRs)
                             # stay on the metadata branch. true = ALSO copy them onto the
                             # integration branch at close-out — a direct commit to the code line,
                             # so opt in deliberately. Per-repo-only (coordination-key fenced).
finalize:                    # merge gate: rebase onto base + re-test before merge
  gate: local                # local (default, on) | ci | both | off  — off = pre-0015 (trust the PR's CI)
  test_command:              # OPTIONAL; unset => finalize auto-detects the suite
learnings:                   # the build-loop memory subsystem (change 0067)
  enabled: true              # default. false = whole subsystem off (read/write gate, never a purge)
  cap: 300                   # default. active-finding count past which the harvest flags "needs curation"
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
```

`.docket.yml` lives on the repo's **default branch (`origin/HEAD`)**, NOT on the integration branch — `integration_branch` is a value *read from* the file, so the file cannot be located *by* it. `metadata_branch` resolves where PM commits land; `integration_branch` (default `auto` → `origin/HEAD`, fallback `main`; explicit `main`/`develop` verbatim) resolves where code lands — feature branches always cut from `origin/<integration_branch>`. A genuinely absent file ⇒ defaults apply; an unreachable `origin` is never silently treated as "file absent." **Backward-compatible opt-out:** pinning `metadata_branch: main` (with `integration_branch: main`) reproduces single-branch behavior exactly — no `docket` branch, no `.docket/` worktree.

**Config layers.** Two more optional layers: a **user-level** `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` (full `.docket.yml` schema; every repo on this machine) and a **machine-local** `<repo>/.docket.local.yml` (gitignored; this repo, this machine only). Every key resolves **per-field**: **repo-local > repo-committed > global > built-in** (map-valued `skills:`/`agents:` merge field-by-field). **Coordination-key fence:** a key whose effect writes shared, non-re-derivable state (`metadata_branch`, `integration_branch`, `changes_dir`/`adrs_dir`/`results_dir`, `github_project`, `terminal_publish`, and `board_surfaces`' `github` token) is per-repo-only — set in either machine-scoped file it is loudly warned-and-ignored, never honored, never fatal (ADR-0019). Everything else is global-able. The per-key classification table and the misplaced/malformed-file postures are authoritative in [`scripts/docket-config.md`](../../scripts/docket-config.md); the legacy `agents.yaml` auto-migration is owned by `sync-agents.sh`.

This resolution — repair `origin/HEAD`, read `.docket.yml` authoritatively, apply every default, resolve `integration_branch` — is performed deterministically by the config resolver (**`docket-config.sh --export`**), reached in skill runtime through the `docket.sh preflight`/`env` verbs (see the *Step-0 preamble*); interface and mechanics live in [`scripts/docket-config.md`](../../scripts/docket-config.md).

**Reaching the helper scripts (`DOCKET_SCRIPTS_DIR`).** Every helper script this convention names lives in the docket clone's `scripts/` directory, NOT in the consuming repo; a skill invokes every docket helper through the single facade `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <op>` (op = the wrapped helper's basename; the `preflight`/`env` verbs are the two exceptions). `install.sh` injects the variable into the shell profile and Claude Code's user-level `settings.json` `env` (mechanics: `scripts/ensure-docket-env.md`); the `:?` makes a missing/incomplete install **fail loud** — stop and fix the install, never silently degrade to hand-worked operations. Every env var docket introduces is **DOCKET_-namespaced**.

**Script contracts (`scripts/<name>.md`).** Every `scripts/<name>.sh` has a co-located `scripts/<name>.md` contract — its authoritative spec (Purpose / Usage / Behavior / Exit codes / Invariants). Read it for a script's internals; reach it from a consuming repo the same way as the script. (`docket-convention/github-board-mirror.md` is skill-reference, not a single-script contract.)

**`board_surfaces` — the board as 0..n derived views.** The board is a *derived view* over the change files; `board_surfaces` lists which surfaces to render: `inline` (the committed, offline-safe `BOARD.md`) and `github` (the one-way Issues + Projects v2 mirror, see *GitHub board mirror*); default `[inline]`, `github` strictly opt-in. **`[]` disables the board entirely** — the change files plus git history remain fully authoritative. An unknown token is warned-and-ignored (a typo must never abort a build); a non-GitHub remote silently drops `github`; `github_project` is consulted only when `github` is enabled, minted-and-written-back on first sync if unset.

**`finalize` — the rebase-retest merge gate.** `finalize.gate` (`local` default · `ci` ·
`both` · `off`) governs `docket-finalize-change`'s merge step: before docket merges, it rebases
the feature branch onto `origin/<integration_branch>` and re-validates, merging only if green —
`local` runs the repo's suite locally (auto-detected, or `finalize.test_command`); `ci` polls
GitHub checks; `both` requires both; **`off`** merges trusting the PR's own CI. **Finalize-only**
— the `docket-status` sweep never merges. The gate flow and its agents live in
`docket-finalize-change`.

### Step-0 preamble (every operating skill)

Every operating skill starts identically; skill bodies compress to a pointer here plus one line naming where their writes land.

1. Load this convention (blocking).
2. Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` **as its own Bash call** — never compounded with other commands — then read the printed `KEY=value` block off stdout and carry those values forward as literals in later commands (no `eval`, no `source`). `preflight` resolves config, enforces the bootstrap verdict **fail-closed**, and ensures + syncs the metadata working tree (docket-mode: the persistent `.docket/` worktree, parked on `docket`, shared hooks disabled; main-mode: the primary tree). On success it prints the block; on any verdict other than `PROCEED` it exits non-zero with a stderr diagnostic instead.
3. Act on the verdict: `PROCEED` → continue. `STOP_MIGRATE` → refuse and point at `migrate-to-docket.sh` (a human-initiated setup script, never an agent runtime invocation). `CREATE_ORPHAN` (fresh repo, once, human-attended) → run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh bootstrap`, then re-run `docket.sh preflight`.

All metadata reads and writes happen in the metadata working tree on `metadata_branch`, pushed to its remote immediately. Every mid-run metadata re-sync — pre-read syncs and **push-retry CAS loops alike** — is a fresh `docket.sh preflight` run (for a CAS loop: re-run `docket.sh preflight`, then retry the push); plain git plumbing (`git add`/`commit`/`push`, `git -C` forms) stays direct.

### Harness-native recovery after sandbox or permission denial

On host **sandbox** or **permission** denial of a required `docket.sh` facade or direct Git command, retry the **exact command** once through the host harness's native approval/escalation mechanism. Do not change arguments or broaden the session sandbox. No shell-level elevation, including `sudo`, is permitted. If approval is unavailable or denied, or the retry fails, preserve the diagnostic and follow the caller's **existing failure posture**. Ordinary Git failures do not qualify. In Step 0, retry the outer `docket.sh preflight`, never an inner fetch. Normative skill prose is **harness-neutral**: never name or invoke product-specific retry syntax.

### Agent layer — model/effort-pinned subagents (change 0016)

Each **autonomous** docket skill can run as a model/effort-pinned **subagent** instead of inline at the session model. Five skills get a wrapper — `docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`; the two **interactive** skills (`docket-new-change`, `docket-groom-next`) stay inline and only surface an **advisory** recommended model/effort at startup (a skill cannot force the session model). `docket-convention` is not an agent — it is injected into every wrapper via `skills:`.

A wrapper is a thin generated file: it pins `model` + `effort` and injects the skill via `skills: [<skill>, docket-convention]`; the skill body stays the single source of behavior. Because a subagent cannot pause to ask a human, every autonomous wrapper carries an **abort-and-report** rule: an unmet precondition or blocking ambiguity is surfaced and stopped on — never turned into an interactive prompt. Wrappers are generated by `sync-agents.sh` from the layered config (see *Config layers*; no entry in any layer ⇒ `model: inherit`, no `effort`). A **directly-invoked** autonomous skill is still routed to its pinned wrapper by a harness dispatch mechanism (Cursor's generated `docket-dispatch.mdc` rule, or Claude Code's native `context: fork` + `agent:` frontmatter), so the pin holds either way; mechanics live in [`references/agent-layer.md`](references/agent-layer.md).

**Composition (change 0017).** `docket-implement-next` dispatches the `docket-status` subagent (step 0) and the `docket-adr` subagent (step 6); `docket-auto-groom` dispatches the `docket-auto-groom-critic` subagent for its adversarial gate. These dispatches are **foreground** (the parent suspends until the child returns) and **unconditional**; their contract is **git state** on `origin/docket`, re-read after a re-sync — never an in-context return. **Foreground means the parent *actively blocks* on the child's return — it may never background a dispatched or forked child and *yield* to await a task-notification** (a forked/subagent skill has no channel to receive one, ADR-0024): yielding returns a **half-done run that the caller reads as `completed`**. Reciprocally, a caller must **not** read a bare `completed` as proof the child finished: it verifies the child's git-state transition and **never adopts or commits a child's uncommitted working-tree files**. `docket-finalize-change` dispatches the `docket-rebase-resolver` subagent on a merge-gate rebase conflict and the `docket-integration-repair` subagent on a red rebased suite — also foreground, but their reports flow **back to finalize in-context** to gate the merge, acting in the feature worktree. Each dispatched agent runs at the model/effort its own wrapper resolves — literal tiers are never restated in dispatch prose. Four of the **nine** generated wrappers wrap **no skill** — `docket-auto-groom-critic`, `docket-rebase-resolver`, `docket-integration-repair` (each loads only `docket-convention`, inheriting no caller bias), and `docket-brainstorm-consultant`, which loads **no convention either** — it authors prose and performs zero docket operations (ADR-0022). All are auto-discovered by `sync-agents.sh`'s `agents/docket-*.md` glob. (Five *skills* get a wrapper — nine wrappers, five skills.)

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

- **Passthrough.** A value is passed verbatim to the Skill tool — never validated against a registry (ADR-0015); any third-party or in-repo skill plugs in. Unknown *role keys* are warned-and-ignored.
- **`auto` sentinel.** No skill is invoked; the running agent does the step itself. The per-role fallback defines only the **final artifact / stop-point** (column 4) — never the method.
- **Missing-skill rule — degrade to auto + warn** prominently (run output and, for plan/build/review/finish, the PR body) when the resolved skill cannot be invoked at runtime. Softer than abort-and-report because skill availability is per-machine, not repo state.
- **Resolution** is deterministic via `docket-config.sh --export`, which emits `SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH` (defaulted when unset); skill bodies read the variable, never re-parse YAML. `finalize.gate` still validates regardless of the resolved build method.
- **Autonomy precedence — pre-specified at the call site.** An invoked skill's interactive step never outranks the caller's autonomy contract. An **autonomous** caller (a skill with a generated wrapper, carrying its abort-and-report rule) states the outcome up front in its direction to a role skill — the house marker is `DIRECTED to:` — and answers any choice the sub-skill poses internally from already-resolved config, emitting one run-output line naming the role and skill **only when** a hand-off was actually met and suppressed. Phrase the direction by **shape** ("any execution-mode or option choice it poses"), never by citing a vendored heading a plugin upgrade would silently stale. This paragraph is durability for future bindings, **not** the enforcement — what beats a specific instruction read at the moment of invocation is a specific counter-instruction at that same moment, so a future slim must not keep it and drop the call-site directions. Interactive skills — those with no generated wrapper — are unaffected: their prompts are the product. `docket-finalize-change`'s human-present close-out is the one exception inside an autonomous file, stated as an explicit condition.

### Directory layout (paths relative to the configured knobs)

```
<changes_dir>/            # default docs/changes/
  active/                 # every NON-terminal change:   <id>-<slug>.md            (id zero-padded to 4 digits)
  archive/                # the two terminal outcomes:    <YYYY-MM-DD>-<id>-<slug>.md
  BOARD.md                # generated board (NEVER hand-edited); spans active + archive
  README.md               # small static blurb linking to BOARD.md (NOT generated)
  LEARNINGS.md            # pointer stub → learnings/ (the pre-0067 single-file ledger)
  learnings/              # curated build-loop findings; harvested at close-out (see "Learnings ledger")
    <slug>.md             # one finding per lesson/family — living files, extended on re-hit
    README.md             # GENERATED index (render-learnings-index.sh); never hand-edited
<adrs_dir>/               # default docs/adrs/  — flat; ADRs are NEVER archived
  <NNNN>-<slug>.md        # immutable once Accepted (only its status: line ever changes)
  README.md               # generated ADR index
<results_dir>/            # default docs/results/  — optional close-out artifacts (feature-branch build files; NEVER archived)
  <YYYY-MM-DD>-<slug>-results.md
```

The `archive/` filename date prefix is **UTC**: the **merge commit's** date for `done`, the **kill commit's** date for `killed`.

In `docket`-mode all of the above lives on the `docket` branch, written through the persistent, gitignored **`.docket/` metadata worktree** (deliberately not under `.worktrees/` — slug collisions, prune blast radius; see *Branch model*).

### Change manifest (frontmatter at the top of each change file)

```yaml
---
id: 7                     # integer; zero-padded to 4 digits in the filename
slug: quicklook-interactions
title: Quick Look interactions — external links + local images
status: proposed          # proposed | in-progress | blocked | deferred | implemented | done | killed
priority: medium          # low | medium | high | critical   (default: medium)
type: feat                # a configured change_type; set at creation. `all`/`untyped` are reserved
created: 2026-05-30
updated: 2026-05-30
depends_on: [4]           # change ids that must reach `done` (PR merged) first
related: [4, 6]           # cross-links the reconcile pass reads
discovered_from: [62]     # change id(s) whose work surfaced this one; informational like related:, never a readiness gate
adrs: [24]                # ADRs this change cites or produces
spec:                     # superpowers design doc path; set at brainstorm (propose) time, on metadata_branch
plan:                     # plan FILE lives on the feature branch; this FIELD is set in the main tree at build time
results:                  # results FILE on the feature branch; this FIELD set in the main tree at close-out (optional)
trivial: false            # true = no spec needed (small mechanical change); still build-ready
auto_groomable:           # tri-state: unset ⇒ inherit the repo's auto_groom; true/false ⇒ explicit override
branch:                   # planned feat/<slug> name, set on claim; branch itself created at build (step 4)
claimed_at:               # UTC ISO-8601 claim lease (YYYY-MM-DDTHH:MM:SSZ); stamped at claim, refreshed at phase boundaries, cleared on leaving in-progress
pr:                       # set when the PR is opened
issue:                    # GitHub mirror issue number; minted on first `github` sync (one-way), shape of pr:
blocked_by:               # free text; set only when status: blocked
reconciled: false         # set true after the just-in-time reconcile pass
---
```

### Change body sections

- `## Artifacts` — **first body section** (immediately after the frontmatter closing `---`, above `## Why`). Marker-bounded (`<!-- docket:artifacts:start (generated — do not hand-edit) -->` / `<!-- docket:artifacts:end -->`); rendered by `render-change-links.sh` from frontmatter; **never hand-edited** — the renderer is the sole writer. The change template seeds it empty; field-writing skills regenerate it after every frontmatter update. Its **reciprocal** is the `docket:backlink` block (markers `<!-- docket:backlink:start … -->` / `<!-- docket:backlink:end -->`) stamped at the TOP of each artifact (spec, plan, results, PR body) pointing home to the change on `metadata_branch`, written solely by `render-artifact-backlink.sh` (change 0136; ADRs excluded, back-referenced by `change:`).
- `## Why` — the motivation, as detailed as warranted (no length limit).
- `## What changes` — scope of the work.
- `## Out of scope` — explicit non-goals.
- `## Open questions` — unknowns to resolve during reconcile/design.
- `## Reconcile log` — dated entries appended by the implementer's reconcile pass.
- `## Reclaim log` — dated entries appended by `reclaim-claims.sh` when an expired-lease, no-branch claim self-heals back to `proposed`.
- `## Auto-groom blocked` — dated abstain record appended by `docket-auto-groom`; contents and lifecycle (including removal on re-arm) are defined by the *Autonomous grooming* shared definition below.
- `## Publish deferred` — dated record appended by `mark-publish-deferred.sh` (change 0083) when a terminal close-out's publish step was **expected** (`terminal_publish: true`, docket-mode) but consciously deferred or blocked, so the archived record never reached the integration branch. **Presence-encoded state:** `board-checks.sh`'s `publish-deferred` check surfaces it as a finding, and `terminal-publish.sh` **removes it automatically** on a successful publish — so a backfill self-heals the marker for free. Never written when the publish is legitimately suppressed (`terminal_publish: false`, or `main`-mode), where a skipped publish is success rather than a deferral. Written and removed by the script; never hand-authored.
- `## Finalize blocked` — dated record appended by `docket-finalize-change` when a gate failure leaves a change needing a human; presence drives the board's `finalize blocked — needs you` cell and makes later **auto-detect** finalize runs skip the change. A human retries a marked change by **naming its id**, which overrides the skip (no manual delete needed). The clearing rule — when the section is removed, and why archiving does not require it — is owned by `docket-finalize-change`'s `## Finalize blocked` section and is not restated here.
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
     │                    ├──blocker──▶ blocked ──clears──▶ in-progress
     │                    └──lease expired, no branch (reclaim)──▶ proposed
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

**Reclaim edge (`in-progress → proposed`).** An `in-progress` change whose claim lease (`claimed_at:` + `reclaim.lease_ttl`) has expired AND that has no feature branch is flipped back to `proposed` by `reclaim-claims.sh` (opt-in via `reclaim.auto` or an explicit `docket.sh reclaim-claims`), clearing `branch:`/`claimed_at:` and resetting `reconciled: false` so a fresh reconcile runs on re-claim. The has-branch case is never auto-reclaimed (it may carry real work) — it stays flagged for a human.

**Board refresh on status writes.** Any skill that writes a change's `status:` refreshes the board immediately after — the **Board pass** — by invoking the one facade call `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only`. That orchestrator owns the whole decision: it resolves config itself (fail-closed), gates on the enabled surfaces, renders the `inline` surface through the gated `board-refresh.sh` writer, runs the `github` mirror upsert (best-effort), and commits + pushes `BOARD.md` on `metadata_branch` **only if it actually changed** — a separate commit from the `status:` write, with its own rebase-retry. **No surfaces value is ever passed by a skill**: the caller never resolves, spells, or forwards one, which is what makes an unresolved config impossible to mistake for a disabled board. The pass reports its outcome on a single stdout report line, and **callers key on that line, never on the exit code** — the full report-line vocabulary and retry classification live in the script contract (`scripts/docket-status.md`). A must-land caller passes `--must-land`; the bounded retry and the exit-code mapping live in that same script contract. A must-land caller STOPs and surfaces that failure (abort-and-report — these are autonomous skills with no human to prompt); a best-effort caller logs it and continues to its own next step. A repo with `board_surfaces: []` renders and commits nothing, and a pre-existing `BOARD.md` is left untouched rather than truncated. The board is a derived view and must never trail the change files.

### Build-readiness & selection (shared definition)

A change is **build-ready** — eligible for `docket-implement-next` — only when it is `proposed`, has a `spec:` **or** `trivial: true`, and all `depends_on` are satisfied (`done`). A `proposed` change with neither a spec nor `trivial: true` is **needs-brainstorm** (not build-ready). The implementer's deterministic selection order is `priority` (`critical` > `high` > `medium` > `low`) → age (`created`) → **lowest `id`**. A `created:` that is missing or malformed (not a well-formed `YYYY-MM-DD`) sorts last within its priority band — unknown age never preempts dated work.

### Autonomous grooming (shared definition)

A change's **effective auto-groomable** value is its `auto_groomable:` override when explicitly set, else the repo's `auto_groom` knob (default `false`). The field is human input with one exception: `docket-auto-groom`'s abstain is the single agent write (it flips the override to `false`).

A stub is **autonomous-eligible** — selectable by `docket-auto-groom` — when it is needs-brainstorm (`proposed`, no `spec:`, not `trivial: true`) AND effective auto-groomable. Unsatisfied `depends_on` does NOT exclude it (the same design-ahead rule as interactive grooming; the implementer's reconcile re-validates at build time). Ranking is the same deterministic selection order as build-ready selection.

**Abstain rule.** When autonomous grooming cannot safely default a decision, it emits NO spec; it flips `auto_groomable: false` and appends a dated `## Auto-groom blocked` body section. The stub stays needs-brainstorm — out of the autonomous queue, still in the interactive one. Re-arm = a human supplies the missing context, flips the flag back to `true`, and DELETES the `## Auto-groom blocked` section (git history keeps it) — the section's presence is what drives the board's needs-you cell and `docket-groom-next`'s first band, so a stale section would mislabel a re-armed stub. Kill and defer are never autonomous: they surface inside the blocked section as recommendations.

**Interactive selection bands.** `docket-groom-next` still sees every needs-brainstorm stub, but its default order prefers stubs that need a human: (1) abstained (`## Auto-groom blocked` present), (2) effective `auto_groomable: false`, (3) effective auto-groomable — flagged "docket-auto-groom will handle it unless you want it now." Within each band, the deterministic selection order applies. The board renders abstained stubs as **auto-groom blocked — needs you**, distinct from plain needs-brainstorm.

### Auto-capture (shared definition)

`auto_capture` (a map: `enabled` default `false`, `types` default `all`; global-able) governs what
an **autonomous** skill does with genuine follow-up work it discovers mid-run. Disabled, the model
reports it in prose. Enabled, it **classifies** the work and — only if that type is admitted — mints
an ordinary `proposed` needs-brainstorm stub with `discovered_from:` and `type:` set. Capture
fidelity, **not** autonomy: every stub still waits at the human's groom gate.

**Mint sites** are the autonomous *single-change* skills: `docket-implement-next` (reconcile and
review) and the `docket-finalize-change` / `docket-status` harvest. **`docket-auto-groom` is never a
mint site** — a minted stub is itself autonomous-eligible, so minting would break its
provable-termination invariant and make `auto_groom` × `auto_capture` a backlog-growth loop.
**Interactive skills need no auto-capture path** — a human is present to decide what gets filed.

**Per discovery** (after the materiality bar): assign exactly one type from `CHANGE_TYPES` — the
model classifies, the script never infers (ADR-0012). `AUTO_CAPTURE_ENABLED: false` ⇒ report, mint
nothing. Enabled but the type is outside `AUTO_CAPTURE_TYPES` (the literal `all`, or a subset) ⇒
mint nothing, report it as **policy-suppressed**. Enabled and admitted ⇒ `mint-stub --type`. Every
outcome keeps ADR-0045's best-effort posture. **Type filtering runs before the cap is consumed** —
a suppressed candidate must never spend a mint slot; dedup stays after admission.

**Materiality bar** — mint only for *actionable follow-up work that would be its own change / PR*
("would a human file this as a `docket-new-change`?"). A build lesson → the **learnings** harvest;
drift inside the current change → the **reconcile log**; a bare observation → the run report.

**The mint itself is deterministic** (ADR-0012 — the model judges *what*, the script does the mint):
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh mint-stub --changes-dir .docket/<changes_dir>
--title <title> --type <type> --body-file <file> --discovered-from <this change's id> --minted <n so far>` (in
`docket`-mode; in `main`-mode, `--changes-dir <changes_dir>` — the metadata worktree IS the primary
tree) — one stub per call, `--body-file` **must start with `## Why`**, contract in
`scripts/mint-stub.md`. **`<n so far>` is the running count across the whole run on a single
change, never reset per mint site** — a skill with two mint sites (`docket-implement-next`'s
reconcile and review) carries the total forward. (`docket-status`'s sweep scopes it per swept
change — see its SKILL.md.) It owns dedup, id allocation, the template write, and the CAS push;
**exit 3** = duplicate skipped, **exit 4** = cap (3) reached, **exit 1** = a real error (push
failure, malformed body, retry exhaustion). Every skip, overflow, and exit-1 failure is **surfaced
in the run report, never silently dropped** — but none is fatal: **auto-capture is best-effort and
must never abort the change being built**, because capture is a courtesy while the change is the
job. Minting is a metadata-worktree write only — it never touches the running change's own
claim/branch/PR state.

### Learnings ledger

`<changes_dir>/learnings/` — the project's **build-loop memory** (change 0067): one curated finding
per file (a lesson or a consolidated family), on `metadata_branch` only, never published to the
integration branch. `LEARNINGS.md` remains as a pointer stub to the pre-0067 single-file ledger.
The finding files are curated prose, written only by the harvest and by human curation; the index
(`learnings/README.md`) is a **derived view**, rendered by `render-learnings-index.sh` (its sole
writer, ADR-0012) — readers pay for a small hint surface, not for history.

**Full mechanics — finding-file frontmatter, the harvest (create/extend), promotion, capacity, and
the off-switch — are in [references/learnings.md](references/learnings.md); read it before
harvesting, promoting, or curating findings.**

**Read contract — pay per relevance.** Gated on `learnings.enabled`; when `false`, readers perform
**zero** learnings reads:
1. Load `learnings/README.md` (the index) always — a small, grouped hint surface.
2. Read only the finding files whose index line (hook + topics) bears on the change at hand.

**Readers:** `docket-implement-next` at plan time and at review; `docket-groom-next` before a brainstorm; `docket-auto-groom` before its self-brainstorm. **Writer:** only the harvest at close-out (single source: the *Harvest learnings* step in `docket-finalize-change`; `docket-status`'s sweep invokes it by reference) — it creates or extends a finding, never merges two distinct ones.

Compressed rules (detail in the reference): the promotion tiering criterion is
*"will the agent know to search for this?"* — a rule that must fire unprompted graduates
(`promotion_state: retained | candidate | promoted`; promotion and consolidation are human acts);
`learnings.cap` counts **active findings** (`retained` + `candidate`), and past it the loop
flags needs-curation, never auto-merging its own memory; `learnings.enabled: false` is
a no-op **read/write gate, never a purge** — existing files stay byte-untouched, re-enabling resumes.

### GitHub board mirror (shared definition)

The `github` board surface mirrors each change to one GitHub issue (and one Projects v2 item) — **strictly one-way**: change files are the source of truth, the mirror is derived output that is **never read back**. It rides in the Board pass (`docket-status`) and is **best-effort** (network + `gh` auth; self-heals next pass; never aborts a build); its external-write mechanics are owned by the deterministic `github-mirror.sh` (not agent-constructed `gh` calls) — the Board pass only invokes it. **Full mechanics — the `issue:` upsert, the `docket:` label namespace, the status→issue mapping across all seven states, the issue body, and Projects v2 — are in [`github-board-mirror.md`](github-board-mirror.md); read it when `board_surfaces` includes `github`.**

**Derived-view script family.** The deterministic scripts producing derived views from the change files, each the sole writer of its output (the ADR-0012 script-vs-model boundary): `board-refresh.sh` (the gated `inline` board writer, wrapping the pure renderer `render-board.sh`), `github-mirror.sh` (the GitHub Issues/Projects mirror), `render-change-links.sh` (per-change `## Artifacts` link-block renderer; offline, falls back to bare code-formatted paths when no GitHub remote is detected) — field-writing skills call it immediately after every frontmatter field write — and `render-artifact-backlink.sh` (the reciprocal per-artifact `docket:backlink` renderer; offline, same fallback) — called by the skills/close-out that write each artifact.

### Bootstrap guard (`docket`-mode first-run safety)

At startup, after resolving config, when `metadata_branch == docket`, fetch origin and evaluate a 2×2 over two probes (both stated over the **same vocabulary**):

- **`DOCKET`** = the `docket` branch exists (origin OR local).
- **`LIVE`** = the **live planning surface** still sits on the integration branch — probe ONLY the pruned surface (`<changes_dir>/active`, `README.md`, `BOARD.md`) via `git ls-tree origin/<integration_branch>`; `archive/`, `<adrs_dir>/`, and pre-migration specs deliberately *stay* on integration, so probing them would read `LIVE` forever. An unresolvable `origin/<integration_branch>` is a **hard config error**, not `¬LIVE`.

| | `LIVE` | `¬LIVE` |
|---|---|---|
| **`¬DOCKET`** | existing single-branch repo → **STOP**, point to `migrate-to-docket.sh`; never auto-create or move data | fresh repo → create the empty orphan `docket`, push, **proceed** |
| **`DOCKET`** | **half-migrated** (interrupted run) → **STOP**, point back to `migrate-to-docket.sh` to finish its prune | migrated → **proceed** |

This 2×2 is the spec `docket-config.sh` implements, reported as its `BOOTSTRAP=` verdict — `PROCEED` (migrated or main-mode), `STOP_MIGRATE` (existing single-branch or half-migrated), or `CREATE_ORPHAN` (fresh) — which the *Step-0 preamble* acts on. Probe mechanics, the read-only `--export` default, and the guarded `--bootstrap` write path are in [`scripts/docket-config.md`](../../scripts/docket-config.md). The guard is a no-op in `main`-mode; the migration itself lives in the standalone `migrate-to-docket.sh`.

### Branch model

Metadata (change files, `BOARD.md`, ADRs, specs) commits to `metadata_branch` (default `docket`) via the **metadata working tree** — the primary working tree on the integration branch in single-branch (`main`) mode, the persistent `.docket/` worktree in `docket`-mode — and is **always pushed to its remote immediately** (the backlog, board, specs, and ADRs stay browsable on the remote at all times).

A change's `feat/<slug>` branch is **ALWAYS cut from `origin/<integration_branch>`** — `metadata_branch` only redirects bookkeeping commits, never where code branches start. The feature branch adds only the plan + results + code and **never modifies** docket metadata.

The `.docket` metadata worktree has the repo's shared git hooks disabled (worktree-scoped `core.hooksPath` → an empty docket-owned dir, via `disable-worktree-hooks.sh`), so machine-generated bookkeeping commits coexist with a hook framework on the integration branch; feature-branch code commits still run the team's hooks (change 0063).

On a terminal transition (`done` *or* `killed`), the driving skill runs the shared **terminal close-out** sequence — archive, re-render, **terminal-publish** (copying the archived change file + its `spec:` + the `Accepted` ADRs in `adrs:` from `origin/docket` onto the integration branch via `git checkout origin/docket -- <paths>`, never a `git merge docket` — the only flow of metadata onto the code line, also refreshing the integration-branch ADR index whenever the commit publishes an ADR), cleanup, board. Ordering, per-caller failure postures, and the `main`-mode degradation live in **[`references/terminal-close-out.md`](references/terminal-close-out.md) — read it before driving any terminal transition.** **`terminal_publish` is `false` by default** (per-repo-only; changes 0064/0084): without the opt-in the records stay on `metadata_branch` and the integration branch receives only code, plans, and results through the normal PR merge; **`terminal_publish: true`** accepts a direct machine commit onto the integration branch and gates both publish shapes (change close-out and `docket-adr`'s publish); inert in `main`-mode. After a merge lands, both merge sites run the best-effort, FF-only `sync-integration-branch.sh` once at end of run (a no-op in `main`-mode and on any non-FF/dirty/feature-branch tree).
