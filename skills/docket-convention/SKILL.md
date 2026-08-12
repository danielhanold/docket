---
name: docket-convention
description: Use when any docket skill runs — docket-new-change, docket-groom-next, docket-implement-next, docket-status, docket-finalize-change, and docket-adr load this first (their blocking Step 0) — or when you need to understand how docket tracks work. The shared contract — .docket.yml configuration, directory layout, the change manifest and lifecycle, ADR format, build-readiness and selection, the bootstrap guard, and the branch model. Pure reference — defines the convention; performs no reads, writes, or git operations.
---

# docket-convention — the shared contract (pure reference)

This skill defines the docket convention and does nothing else: no procedure, no reads or writes, no git.

## Convention

docket tracks planned work as **changes** — one markdown file each, roughly one PR — and records architecture decisions as **ADRs**. This skill is the single source of the convention; the operating skills load it at startup as their blocking Step 0, use its vocabulary, and never restate it.

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
terminal_publish: false      # false (default) = terminal records stay on the metadata branch;
                             # true = ALSO copy them onto the integration branch at close-out.
                             # Per-repo-only (coordination-key fenced).
finalize:                    # merge gate: rebase onto base + re-test before merge
  gate: local                # local (default, on) | ci | both | off  — off = pre-0015 (trust the PR's CI)
  test_command:              # OPTIONAL; unset => finalize auto-detects the suite
  skip_results_only_delta: false  # arms the gate's docs-only post-gate skip. Per-repo-only (fenced)
learnings:                   # the build-loop memory subsystem (change 0067)
  enabled: true              # default. false = whole subsystem off (read/write gate, never a purge)
  cap: 300                   # default. active-finding count past which the harvest flags "needs curation"
github_project:              # {owner, number} of the auto-managed Projects v2 board; unset ⇒ auto-create on first github sync
agent_harnesses: [claude]    # harnesses the per-repo agent pass generates wrapper files for;
                             # default [claude], e.g. [claude, cursor] for a Cursor repo.
agents:                      # harness-first per-skill subagent model/effort — write values unquoted and space-free, no `#` inside the `{…}` flow map; see "Agent layer" below
skills:                      # pluggable workflow skills; unset key = the superpowers default shown
  brainstorm: superpowers:brainstorming
  plan:       superpowers:writing-plans
  build:      docket-build   # e.g. `auto` to build inline with no fan-out
  review:     docket-review
  finish:     superpowers:finishing-a-development-branch
```

`.docket.yml` lives on the repo's **default branch (`origin/HEAD`)**, NOT on the integration branch — `integration_branch` is a value *read from* the file, so the file cannot be located *by* it. `metadata_branch` resolves where PM commits land; `integration_branch` (default `auto` → `origin/HEAD`, fallback `main`; explicit values verbatim) resolves where code lands. A genuinely absent file ⇒ defaults apply; an unreachable `origin` is never silently treated as "file absent." **Backward-compatible opt-out:** pinning `metadata_branch: main` (with `integration_branch: main`) reproduces single-branch behavior exactly — no `docket` branch, no `.docket/` worktree.

**Config layers.** Two more optional layers: a **user-level** `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` (full `.docket.yml` schema; every repo on this machine) and a **machine-local** `<repo>/.docket.local.yml` (gitignored; this repo, this machine only). Every key resolves **per-field**: **repo-local > repo-committed > global > built-in** (map-valued `skills:`/`agents:` merge field-by-field). **Coordination-key fence:** a key whose effect writes shared, non-re-derivable state (`metadata_branch`, `integration_branch`, `changes_dir`/`adrs_dir`/`results_dir`, `github_project`, `terminal_publish`, `finalize.skip_results_only_delta`, and `board_surfaces`' `github` token) is per-repo-only — set in either machine-scoped file it is loudly warned-and-ignored, never honored, never fatal (ADR-0019). Everything else is global-able. The per-key classification table and the misplaced/malformed-file postures are authoritative in [`scripts/docket-config.md`](../../scripts/docket-config.md); the legacy `agents.yaml` auto-migration is owned by `sync-agents.sh`.

This resolution — repair `origin/HEAD`, read `.docket.yml` authoritatively, apply every default, resolve `integration_branch` — is performed deterministically by the config resolver (**`docket-config.sh --export`**), reached in skill runtime through the `docket.sh preflight`/`env` verbs (see the *Step-0 preamble*); interface and mechanics live in [`scripts/docket-config.md`](../../scripts/docket-config.md).

**Reaching the helper scripts (`DOCKET_SCRIPTS_DIR`).** Every helper script this convention names lives in the docket clone's `scripts/` directory, NOT in the consuming repo; a skill invokes every docket helper through the single facade `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <op>` (op = the wrapped helper's basename; the `preflight`/`env` verbs are the two exceptions). `install.sh` injects the variable into the shell profile and Claude Code's user-level `settings.json` `env` (mechanics: `scripts/ensure-docket-env.md`); the `:?` makes a missing/incomplete install **fail loud** — stop and fix the install, never silently degrade to hand-worked operations. Every env var docket introduces is **DOCKET_-namespaced**.

**Script contracts (`scripts/<name>.md`).** Every `scripts/<name>.sh` has a co-located `scripts/<name>.md` contract — its authoritative spec (Purpose / Usage / Behavior / Exit codes / Invariants). Read it for a script's internals; reach it from a consuming repo the same way as the script. (`docket-convention/github-board-mirror.md` is skill-reference, not a single-script contract.)

**`board_surfaces` — the board as 0..n derived views.** The board is a *derived view* over the change files; `board_surfaces` lists which surfaces to render: `inline` (the committed, offline-safe `BOARD.md`) and `github` (the one-way mirror, see *GitHub board mirror*); default `[inline]`. **`[]` disables the board entirely** — the change files plus git history remain fully authoritative. An unknown token is warned-and-ignored (a typo must never abort a build); a non-GitHub remote silently drops `github`; `github_project` is consulted only when `github` is enabled, minted-and-written-back on first sync if unset.

**`finalize` — the rebase-retest merge gate.** `finalize.gate` governs `docket-finalize-change`'s
merge step — rebase onto `origin/<integration_branch>`, re-validate, merge only if green: `local`
(default) runs the repo's suite locally, `ci` polls GitHub checks, `both` requires both, **`off`**
merges trusting the PR's own CI. **Finalize-only** — the `docket-status` sweep never merges. The
gate flow and its agents live in `docket-finalize-change`.

### Step-0 preamble (every operating skill)

Every operating skill starts identically; skill bodies compress to a pointer here plus one line naming where their writes land.

1. Load this convention (blocking).
2. Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` **as its own Bash call** — never compounded with other commands — then read the printed `KEY=value` block off stdout and carry those values forward as literals in later commands (no `eval`, no `source`). `preflight` resolves config, enforces the bootstrap verdict **fail-closed**, and ensures + syncs the metadata working tree (docket-mode: the persistent `.docket/` worktree, parked on `docket`, shared hooks disabled; main-mode: the primary tree). On success it prints the block; on any verdict other than `PROCEED` it exits non-zero with a stderr diagnostic instead.
3. Act on the verdict: `PROCEED` → continue. `STOP_MIGRATE` → refuse and point at `migrate-to-docket.sh` (a human-initiated setup script, never an agent runtime invocation). `CREATE_ORPHAN` (fresh repo, once, human-attended) → run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh bootstrap`, then re-run `docket.sh preflight`.

All metadata reads and writes happen in the metadata working tree on `metadata_branch`, pushed to its remote immediately. Every mid-run metadata re-sync — pre-read syncs and **push-retry CAS loops alike** — is a fresh `docket.sh preflight` run (for a CAS loop: re-run `docket.sh preflight`, then retry the push); plain git plumbing (`git add`/`commit`/`push`, `git -C` forms) stays direct — and stages by explicit path: that tree is SHARED, so `git add -A`, `git add .`, or `commit -a` there sweeps up whatever another agent had staged at that instant, committing it under your message and pushing it (observed live, change 0247 — a groom's three staged files landed in two unrelated autonomous commits, and its own commit then reported "nothing to commit"). Stage by explicit path.

### Harness-native recovery after sandbox or permission denial

On host **sandbox** or **permission** denial of a required `docket.sh` facade or direct Git command, retry the **exact command** once through the host harness's native approval mechanism — no changed arguments, no broadened sandbox, no shell-level elevation (`sudo` included). If approval is unavailable or the retry fails, preserve the diagnostic and follow the caller's **existing failure posture**. Ordinary Git failures do not qualify. In Step 0, retry the outer `docket.sh preflight`, never an inner fetch. Normative skill prose is **harness-neutral**: never name product-specific retry syntax.

### Dispatch-capability resolution (change 0137)

A dispatch-dependent step — a composition dispatch, or a role skill that dispatches internally — may be declared unavailable only after the agent has, in order: (1) **resolved** a subagent-dispatch mechanism, **including searching any deferred or lazily-loaded tool surface** the harness exposes, since a partially-loaded tool set makes absence observable without anything having been resolved; and (2) **attempted one trivial dispatch**, if resolution was inconclusive. Only a **failed attempt** or an explicit **policy denial** establishes unavailability. **The absence of a specifically-named tool never does** — the rule is stated by capability, and a tool name is a diagnostic string, **never a decision input**. A failure diagnostic MAY report what was searched for; naming it there commits docket to nothing — such a name is an observed internal, not an interface.

When dispatch is genuinely unavailable the kinds are **not** equivalent, so the posture is tiered:

| Tier | Dispatch | Posture |
|---|---|---|
| **A — deterministic** | the `docket-status` and `docket-adr` composition dispatches | Run the same work **inline** — a **first-class equivalent path**, neither a degradation nor a warning, because the contract is git state on `metadata_branch`, not an in-context return. Every obligation holds unchanged: re-sync before reading, derive from fresh origin, never adopt or commit another agent's uncommitted files. |
| **B — adversarial** | the `docket-auto-groom-critic` gate | **Abstain**, per *Autonomous grooming*. Self-critique by the agent that drafted the spec is not an adversarial gate, and the abstain is a path that skill already owns. |
| **C — discipline** | the `build` and `review` role skills, plus the in-branch fix workers — which run the `build` role's own contract, so `skills.build: auto` authorizes inline fixing as well as inline building and there is no separate `skills.fix` key | **Authorized-or-halt.** An **explicitly configured `auto` is the human's** authorization to run inline; any other resolved value that cannot dispatch is **abort-and-report**, leaving the change `in-progress` with `claimed_at` refreshed and the halt reason recorded. **No new status, no new field** — the reclaim lease self-heals an abandoned claim. |

**Outside the table — the `carve-out` (change 0260).** `docket-finalize-change`'s two merge-gate dispatches — `docket-rebase-resolver` and `docket-integration-repair` — sit **outside** this taxonomy rather than in a row of it, because their contract is an **in-context report** gating the merge, not git state on `metadata_branch`. Neither tier posture can be borrowed for them: Tier A's first-class-equivalent inline path presupposes a git-state transition to reproduce, and Tier C's authorized-or-halt presupposes a `skills:` role whose resolved value could carry a human's `auto` authorization — these dispatches have neither. When dispatch is genuinely unavailable for either — established per the resolution rule above, never from a tool name — the posture is finalize's own pre-existing **abort-and-report**: the gate stops, the PR stays open, the change stays `implemented`, and the reason is recorded through the three channels `docket-finalize-change`'s failure reference owns. **Inline substitution is forbidden** for both, and that is the point of carving them out rather than tiering them: reconciling the conflict, or authoring the repair, inside the very agent that would then merge that work is the same self-approval shape Tier B rejects for the critic.

Tier C neither replaces nor softens the *Skill layer*'s **missing-skill rule**: a skill that cannot be **invoked** still degrades to `auto` + warn, while a skill that was invoked and then cannot **dispatch** is Tier C. Two conditions, two postures, one symptom in the run log.

### Agent layer — model/effort-pinned subagents (change 0016)

Each **autonomous** docket skill can run as a model/effort-pinned **subagent** instead of inline at the session model. Seven skills get a wrapper — `docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`, `docket-build-task` (shared by its four profile agents), and `docket-review` (shared by its three rung wrappers); the two **interactive** skills (`docket-new-change`, `docket-groom-next`) stay inline and only surface an **advisory** recommended model/effort at startup. `docket-convention` is not an agent — it is injected via `skills:` into every wrapper except eight: `docket-brainstorm-consultant` (ADR-0022), the four `docket-build-*` profile workers, and the three `docket-review-*` rung wrappers. Those **seven** wrapper-bearing exceptions perform no docket metadata operations — a boundary each worker contract's own *Scope* states normatively; the consultant wraps nothing at all (see *Composition*).

A wrapper is a thin generated file: it carries the resolved `model` and `effort`, when they resolve, and injects the skill via `skills: [<skill>, docket-convention]`; the skill body stays the single source of behavior. Because a subagent cannot pause to ask a human, every autonomous wrapper carries an **abort-and-report** rule: an unmet precondition or blocking ambiguity is surfaced and stopped on — never turned into an interactive prompt. `sync-agents.sh` resolves each field from the config layers over docket's shipped `agents/harness-defaults.yml`; an unmapped harness/agent pair is generated **unpinned**. A **directly-invoked** autonomous skill is still routed to its pinned wrapper by a harness dispatch mechanism, so the pin holds either way; mechanics live in [`references/agent-layer.md`](references/agent-layer.md).

**Composition (change 0017).** `docket-implement-next` dispatches the `docket-status` subagent (step 0) and the `docket-adr` subagent (step 6). These dispatches are **foreground** (the parent suspends until the child returns) and **unconditional**; their contract is **git state** on `origin/docket`, re-read after a re-sync — never an in-context return. `docket-auto-groom` dispatches the `docket-auto-groom-critic` subagent for its adversarial gate — foreground and unconditional on the same terms, but its verdict flows back to the groom **in-context as the dispatch's return** — never via git state and never via agent messaging, because a dispatched groom is not registered under its skill name and no name-addressed delivery to it resolves (change 0281). **Foreground means the parent *actively blocks* on the child's return — it may never background a dispatched or forked child and *yield* to await a task-notification** (a forked/subagent skill has no channel to receive one, ADR-0024): yielding returns a **half-done run that the caller reads as `completed`**. Reciprocally, a caller must **not** read a bare `completed` as proof the child finished: it verifies the child's git-state transition and **never adopts or commits a child's uncommitted working-tree files**. For an interactive dispatch, that verification obligation has a mechanical form: the managed `docket:dispatch` block each harness's parent-facing instructions file carries runs `docket.sh verify-run` on the returning run and re-dispatches an incomplete one exactly once (change 0242). `docket-finalize-change` dispatches the `docket-rebase-resolver` subagent on a merge-gate rebase conflict and the `docket-integration-repair` subagent on a red rebased suite — also foreground, but their reports flow **back to finalize in-context** to gate the merge, acting in the feature worktree. Each dispatched agent runs at the model/effort its own wrapper resolves — literal tiers are never restated in dispatch prose. Four of the **sixteen** generated wrappers wrap **no skill** — `docket-auto-groom-critic`, `docket-rebase-resolver`, `docket-integration-repair` (each loads only `docket-convention`), and `docket-brainstorm-consultant`, which loads **no convention either** — it authors prose and performs zero docket operations (ADR-0022). All are auto-discovered by `sync-agents.sh`'s `agents/docket-*.md` glob. (Sixteen wrappers: five wrap the five autonomous skills, four share the `docket-build-task` contract, three share the `docket-review` contract, four wrap none.)

**Configuring the layer** — the `agents:` blocks, `agent_harnesses` scoping, model IDs (ADR-0015), generation and dispatch-rule mechanics — is a separate read: **read [`references/agent-layer.md`](references/agent-layer.md) now (blocking) before configuring `agents:`/`agent_harnesses:` or running/debugging `sync-agents.sh`.**

### Skill layer — pluggable workflow skills (change 0049)

docket's five workflow steps are **pluggable roles**: the optional `skills:` map rebinds each to any skill name, or to the sentinel `auto`. An unset key defaults to the skill shown — superpowers for `brainstorm`/`plan`/`finish`, docket's own for `build`/`review` (change 0193).

| Role | Default skill | Invoked by | `auto` / fallback artifact — stop-point |
|---|---|---|---|
| brainstorm | `superpowers:brainstorming` | `docket-new-change` §2, `docket-groom-next` | a spec file at the configured spec path; stop at the spec |
| plan | `superpowers:writing-plans` | `docket-implement-next` §4 | a plan file on the feature branch, recorded in `plan:` |
| build | `docket-build` | `docket-implement-next` §5 | the plan executed on the feature branch |
| review | `docket-review` | `docket-implement-next` §6 | a whole-branch review before the PR opens, over a branch whose build evidence is green |
| finish | `superpowers:finishing-a-development-branch` | `docket-implement-next` §7; `docket-finalize-change` close-out | a pushed feature branch + open PR — never merged; stop |

- **Passthrough.** A value is passed verbatim to the Skill tool — never validated against a registry (ADR-0015); any third-party or in-repo skill plugs in. Unknown *role keys* are warned-and-ignored.
- **`auto` sentinel.** No skill is invoked; the running agent does the step itself. The per-role fallback defines only the **final artifact / stop-point** (column 4) — never the method.
- **Missing-skill rule — degrade to auto + warn** prominently (run output and, for plan/build/review/finish, the PR body) when the resolved skill cannot be invoked at runtime. Softer than abort-and-report because skill availability is per-machine, not repo state.
- **Resolution** is deterministic via `docket-config.sh --export`, which emits `SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH` (defaulted when unset); skill bodies read the variable, never re-parse YAML. `finalize.gate` still validates regardless of the resolved build method.
- **Autonomy precedence — pre-specified at the call site.** An invoked skill's interactive step never outranks the caller's autonomy contract. An **autonomous** caller (a skill with a generated wrapper, carrying its abort-and-report rule) states the outcome up front in its direction to a role skill — the house marker is `DIRECTED to:` — and answers any choice the sub-skill poses internally from already-resolved config, emitting one run-output line naming the role and skill **only when** a hand-off was actually met and suppressed. Phrase the direction by **shape** ("any execution-mode or option choice it poses"), never by citing a vendored heading a plugin upgrade would silently stale. This paragraph is durability for future bindings, **not** the enforcement — what beats a specific instruction read at the moment of invocation is a specific counter-instruction at that same moment, so a future slim must not keep it and drop the call-site directions. Interactive skills — those with no generated wrapper — are unaffected: their prompts are the product. `docket-finalize-change`'s human-present close-out is the one exception inside an autonomous file, stated as an explicit condition.
- **Role skill self-description.** A role skill body names its `skills.<role>` binding key, never
  whether that binding is the default — this section's table and `README.md` own that.

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
status: proposed          # proposed | in-progress | blocked | deferred | implemented | stacked-merged | done | killed
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

**Merged plans and results are frozen build records.** Once a change's PR merges, its `plan:` and
`results:` files are never hand-edited again — not to correct a stale line reference, not to update a
superseded instruction. The one writer allowed to touch them afterward is `render-artifact-backlink.sh`,
re-stamping the generated `docket:backlink` block at terminal publish; authored content never changes.
They record what a build was *told* to do at the time it ran, which is the
only thing that makes a completed run auditable; editing one destroys that record while silently
changing what a re-read of the artifact would say the build was asked for. Corrections go in a new
change, never in the merged artifact.

### Change body sections

- `## Artifacts` — **first body section** (immediately after the frontmatter closing `---`, above `## Why`). Marker-bounded (`<!-- docket:artifacts:start (generated — do not hand-edit) -->` / `<!-- docket:artifacts:end -->`); rendered by `render-change-links.sh` from frontmatter; **never hand-edited** — the renderer is the sole writer. Seeded empty by the template; regenerated after every frontmatter field write. Its **reciprocal** is the `docket:backlink` block (markers `<!-- docket:backlink:start … -->` / `<!-- docket:backlink:end -->`) stamped at the TOP of each artifact (spec, plan, results, PR body) pointing home to the change on `metadata_branch`, written solely by `render-artifact-backlink.sh` (change 0136; ADRs excluded, back-referenced by `change:`).
- `## Why` — the motivation, as detailed as warranted (no length limit).
- `## What changes` — scope of the work.
- `## Out of scope` — explicit non-goals.
- `## Open questions` — unknowns to resolve during reconcile/design.
- `## Reconcile log` — dated entries appended by the implementer's reconcile pass.
- `## Reclaim log` — dated entries appended by `reclaim-claims.sh` when an expired-lease, no-branch claim self-heals back to `proposed`.
- `## Auto-groom blocked` — dated abstain record appended by `docket-auto-groom`; contents and lifecycle (including removal on re-arm) are defined by the *Autonomous grooming* shared definition below.
- `## Publish deferred` — dated record appended by `mark-publish-deferred.sh` (change 0083) when a terminal close-out's publish step was **expected** (`terminal_publish: true`, docket-mode) but consciously deferred or blocked. **Presence-encoded state:** the `publish-deferred` health check surfaces it, and `terminal-publish.sh` **removes it automatically** on a successful publish. Never written when the publish is legitimately suppressed (`terminal_publish: false`, or `main`-mode) — a skipped publish there is success, not a deferral. Script-written and script-removed; never hand-authored.
- `## Finalize blocked` — dated record appended by `docket-finalize-change` when a gate failure leaves a change needing a human; presence drives the board's `finalize blocked — needs you` cell and makes later **auto-detect** finalize runs skip the change. A human retries a marked change by **naming its id**, which overrides the skip. The clearing rule is owned by `docket-finalize-change` and not restated here.
- `## Run halted` — record appended (heading **bare**, never dated — the reader is a whole-line match, so the date belongs inside the body) by an autonomous run that stops needing a human (the `halted` disposition). **Presence-encoded state**, in the same family as `## Auto-groom blocked` and `## Finalize blocked`: the run clears `verify-run`'s gate by *writing this section and committing it*, which is what makes a `halted` disposition verifiable in git rather than a claim in a completion report. Removal is owned by `docket-implement-next`'s Step 2 claim — the only transition back into a live run — and is stated there, not restated here.
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

### Lifecycle — eight states

```
                         ┌──────────────── deferred ──────────────┐
                         │ (conscious shelve; revive → proposed)   │
                         ▼                                          │
  proposed ──claim──▶ in-progress ──PR open──▶ implemented ──merge+sweep──▶ done
     │                    │                              │                   (archive/)
     │                    │                              └──into stack parent──▶ stacked-merged ──root lands──▶ done
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
| `stacked-merged` | merged into its stack parent — awaiting the stack root | `active/` |
| `done` | PR merged, filed away (happy terminal) | `archive/` |
| `killed` | abandoned — obsolete or never shipped (sad terminal) | `archive/` |

**Rules.** `active/` holds every non-terminal status; `archive/` holds the two terminal outcomes. `stacked-merged` is non-terminal for that reason: merged into its stack parent, not the integration branch, so it stays in `active/` until the stack close-out promotes it when the stack root lands. The single physical move (`active/ → archive/`, date-prefixed) happens once on the terminal transition and is **idempotent**: re-pull, re-read `status` on `metadata_branch`, no-op if already terminal. `deferred` may be entered from `proposed` or `in-progress` (add `## Why deferred`) and revived to `proposed`; clearing a blocker or reviving is a one-line frontmatter edit, no move. A change whose `depends_on` is unsatisfied is *implicitly* blocked — the selector skips it and the board shows it **waiting on #N**. A dependency is **satisfied when it reaches `done`**; if `#N` is still `implemented` the board flags **waiting on #N — needs your merge**, distinct from **waiting on #N — not yet built**. Reserve explicit `blocked` for external blockers the system can't infer.

**Reclaim edge (`in-progress → proposed`).** An `in-progress` change whose claim lease (`claimed_at:` + `reclaim.lease_ttl`) has expired AND that has no feature branch is flipped back to `proposed` by `reclaim-claims.sh` (opt-in via `reclaim.auto` or an explicit `docket.sh reclaim-claims`), clearing `branch:`/`claimed_at:` and resetting `reconciled: false` so a fresh reconcile runs on re-claim. The has-branch case is never auto-reclaimed (it may carry real work) — it stays flagged for a human.

**Board refresh on status writes.** Any skill that writes a change's `status:` refreshes the board immediately after — the **Board pass** — by invoking the one facade call `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only`. That orchestrator owns the whole decision: it resolves config itself (fail-closed), gates on the enabled surfaces, renders the `inline` surface through the gated `board-refresh.sh` writer, runs the `github` mirror upsert (best-effort), and commits + pushes `BOARD.md` on `metadata_branch` **only if it actually changed** — a separate commit from the `status:` write, with its own rebase-retry. **No surfaces value is ever passed by a skill**: the caller never resolves, spells, or forwards one — so an unresolved config can never be mistaken for a disabled board. The pass reports its outcome on a single stdout report line, and **callers key on that line, never on the exit code** — the report-line vocabulary, `--must-land`'s bounded retry, and the exit-code mapping live in the script contract (`scripts/docket-status.md`). A must-land caller passes `--must-land` and STOPs and surfaces a failure (abort-and-report — these are autonomous skills with no human to prompt); a best-effort caller logs it and continues to its own next step. A repo with `board_surfaces: []` renders and commits nothing, and a pre-existing `BOARD.md` is left untouched rather than truncated. The board is a derived view and must never trail the change files.

### Build-readiness & selection (shared definition)

A change is **build-ready** — eligible for `docket-implement-next` — only when it is `proposed`, has a `spec:` **or** `trivial: true`, and all `depends_on` are satisfied (`done`). A `proposed` change with neither a spec nor `trivial: true` is **needs-brainstorm** (not build-ready). The implementer's deterministic selection order is `priority` (`critical` > `high` > `medium` > `low`) → age (`created`) → **lowest `id`**. A `created:` that is missing or malformed (not a well-formed `YYYY-MM-DD`) sorts last within its priority band — unknown age never preempts dated work.

### Autonomous grooming (shared definition)

A change's **effective auto-groomable** value is its `auto_groomable:` override when explicitly set, else the repo's `auto_groom` knob (default `false`). The field is human input with one exception: `docket-auto-groom`'s abstain is the single agent write (it flips the override to `false`).

A stub is **autonomous-eligible** — selectable by `docket-auto-groom` — when it is needs-brainstorm (`proposed`, no `spec:`, not `trivial: true`) AND effective auto-groomable. Unsatisfied `depends_on` does NOT exclude it (the same design-ahead rule as interactive grooming; the implementer's reconcile re-validates at build time). Ranking is the same deterministic selection order as build-ready selection.

**Abstain rule.** When autonomous grooming cannot safely default a decision, it emits NO spec; it flips `auto_groomable: false` and appends a dated `## Auto-groom blocked` body section. The stub stays needs-brainstorm — out of the autonomous queue, still in the interactive one. Re-arm = a human supplies the missing context, flips the flag back to `true`, and DELETES the `## Auto-groom blocked` section (git history keeps it; the section's presence drives the board's needs-you cell, so a stale one would mislabel a re-armed stub). Kill and defer are never autonomous: they surface inside the blocked section as recommendations.

**Interactive selection bands.** `docket-groom-next` still sees every needs-brainstorm stub, but its default order prefers stubs that need a human: (1) abstained (`## Auto-groom blocked` present), (2) effective `auto_groomable: false`, (3) effective auto-groomable — flagged "docket-auto-groom will handle it unless you want it now." Within each band, the deterministic selection order applies. The board renders abstained stubs as **auto-groom blocked — needs you**, distinct from plain needs-brainstorm.

### Auto-capture (shared definition)

Auto-capture is **capability discovery under strict admission gates**: an autonomous skill actively
looks for independently valuable work it discovers mid-run — and files it only if that work clears
every gate. `auto_capture` (a map: `enabled` default `false`, `types` default `all`; global-able —
resolved as `AUTO_CAPTURE_ENABLED` / `AUTO_CAPTURE_TYPES`) governs what happens then. Disabled, the
model reports it in prose; enabled, it **classifies** the work and — only if that type is admitted —
mints an ordinary `proposed` needs-brainstorm stub (`mint-stub --type`, one per call) with
`discovered_from:` and `type:` set. Capture fidelity, **not** autonomy: every stub still waits at
the human's groom gate. A type outside policy is reported as **policy-suppressed**, never minted —
and type filtering runs **before the cap** is consumed.

**Mint sites** are the autonomous *single-change* skills: `docket-implement-next` (reconcile and
review) and the `docket-finalize-change` / `docket-status` harvest. **`docket-auto-groom` is never a
mint site** — a minted stub is itself autonomous-eligible, so minting would break its
provable-termination invariant. **Interactive skills need no auto-capture path** — a human is
present to decide what gets filed.

**At each mint site — on arrival, before anything has surfaced — and again on any discovered
follow-up work mid-run → read [`references/auto-capture.md`](references/auto-capture.md) now
(blocking)** before minting or suppressing — it owns what to look for, the admission gates and
the suppression list, the materiality bar, the per-site routing, what a captured discovery must say,
and the deterministic mint with its exit codes and cross-site `--minted` carry-forward.

### Dummy mode (shared definition)

Docket's **human-facing** prose can be calibrated to a reader the repo describes. `dummy_mode`
(a map: `enabled` default `false`, `persona` free text, `surfaces` default `all`; global-able —
resolved as `DUMMY_MODE_ENABLED` / `DUMMY_MODE_PERSONA` / `DUMMY_MODE_SURFACES`) governs it. Five
surfaces are eligible: `dialogue` and `reports` are **replaced** — written calibrated to the
persona; `results`, `change-sections`, and `pr` are **additive** — the technical content is
untouched and an authored `### In plain terms` block is written alongside it, in the same commit.
**Apply a surface only when it is in `DUMMY_MODE_SURFACES`** — the literal `all` matches every
token, an empty value matches none — no matter which surfaces a skill's own pointer names.
`DUMMY_MODE_PERSONA` always carries a persona (the shipped default when none is configured), so no
skill special-cases an empty one.

**Agent-safety rule:** an `### In plain terms` block is written for the human and is **never a
decision input** — reconcile, review, planning, and every worker read the technical content only.
Agent-facing artifacts (plans, the spec file, learnings, build evidence) are never simplified;
simplifying them would degrade the build loop itself.

**When `DUMMY_MODE_ENABLED` is `true`, or a human asks for dummy mode in-session, and you are about
to author any of the five surfaces → read [`references/dummy-mode.md`](references/dummy-mode.md)
now (blocking)** — it owns the token table, the replace/additive mechanics, ad-hoc session
enablement, the not-eligible list, and the authoring guidance.

### Learnings ledger

`<changes_dir>/learnings/` — the project's **build-loop memory** (change 0067): one curated finding
per file, on `metadata_branch` only, never published to the integration branch.
`LEARNINGS.md` remains as a pointer stub to the pre-0067 single-file ledger. The finding files are
written only by the harvest and by human curation; the index (`learnings/README.md`)
is a **derived view**, rendered by `render-learnings-index.sh` (its sole writer, ADR-0012).

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

The `github` board surface mirrors each change to one GitHub issue (and one Projects v2 item) — **strictly one-way**: change files are the source of truth, the mirror is derived output that is **never read back**. It rides in the Board pass (`docket-status`) and is **best-effort** (self-heals next pass; never aborts a build); its external writes are owned by the deterministic `github-mirror.sh`, never agent-constructed `gh` calls. **Full mechanics — the `issue:` upsert, the `docket:` label namespace, the status→issue mapping across all eight states, the issue body, and Projects v2 — are in [`github-board-mirror.md`](github-board-mirror.md); read it when `board_surfaces` includes `github`.**

**Derived-view script family.** The deterministic scripts producing derived views from the change files, each the sole writer of its output (the ADR-0012 script-vs-model boundary): `board-refresh.sh` (the gated `inline` board writer, wrapping the pure renderer `render-board.sh`), `github-mirror.sh` (the GitHub Issues/Projects mirror), `render-change-links.sh` (per-change `## Artifacts` link-block renderer; offline-safe) — called immediately after every frontmatter field write — and `render-artifact-backlink.sh` (the reciprocal per-artifact `docket:backlink` renderer; offline-safe) — called by the skills/close-out that write each artifact.

### Bootstrap guard (`docket`-mode first-run safety)

At startup, after resolving config, when `metadata_branch == docket`, fetch origin and evaluate a 2×2 over two probes:

- **`DOCKET`** = the `docket` branch exists (origin OR local).
- **`LIVE`** = the **live planning surface** still sits on the integration branch — probe ONLY the pruned surface (`<changes_dir>/active`, `README.md`, `BOARD.md`) via `git ls-tree origin/<integration_branch>`; `archive/`, `<adrs_dir>/`, and pre-migration specs deliberately *stay* on integration, so probing them would read `LIVE` forever. An unresolvable `origin/<integration_branch>` is a **hard config error**, not `¬LIVE`.

| | `LIVE` | `¬LIVE` |
|---|---|---|
| **`¬DOCKET`** | existing single-branch repo → **STOP**, point to `migrate-to-docket.sh`; never auto-create or move data | fresh repo → create the empty orphan `docket`, push, **proceed** |
| **`DOCKET`** | **half-migrated** (interrupted run) → **STOP**, point back to `migrate-to-docket.sh` to finish its prune | migrated → **proceed** |

This 2×2 is the spec `docket-config.sh` implements, reported as its `BOOTSTRAP=` verdict — `PROCEED` (migrated or main-mode), `STOP_MIGRATE` (existing single-branch or half-migrated), or `CREATE_ORPHAN` (fresh) — which the *Step-0 preamble* acts on. Probe mechanics, the read-only `--export` default, and the guarded `--bootstrap` write path are in [`scripts/docket-config.md`](../../scripts/docket-config.md). The guard is a no-op in `main`-mode; the migration itself lives in the standalone `migrate-to-docket.sh`.

### Branch model

Metadata (change files, `BOARD.md`, ADRs, specs) commits to `metadata_branch` (default `docket`) via the **metadata working tree** — the primary working tree on the integration branch in single-branch (`main`) mode, the persistent `.docket/` worktree in `docket`-mode — and is **always pushed to its remote immediately** (the planning surface stays browsable on the remote).

A change's `feat/<slug>` branch is **ALWAYS cut from `origin/<integration_branch>`** — `metadata_branch` only redirects bookkeeping commits, never where code branches start. The feature branch adds only the plan + results + code and **never modifies** docket metadata.

The `.docket` metadata worktree has the repo's shared git hooks disabled (worktree-scoped `core.hooksPath` → an empty docket-owned dir, via `disable-worktree-hooks.sh`), so machine-generated bookkeeping commits coexist with a hook framework on the integration branch; feature-branch code commits still run the team's hooks (change 0063).

On a terminal transition (`done` *or* `killed`), the driving skill runs the shared **terminal close-out** sequence — archive, re-render, **terminal-publish** (copying the archived change file + its `spec:` + the `Accepted` ADRs in `adrs:` from `origin/docket` onto the integration branch via `git checkout origin/docket -- <paths>`, never a `git merge docket` — the only flow of metadata onto the code line), cleanup, board. Ordering, per-caller failure postures, and the `main`-mode degradation live in **[`references/terminal-close-out.md`](references/terminal-close-out.md) — read it before driving any terminal transition.** **`terminal_publish` is `false` by default** (per-repo-only; changes 0064/0084): without the opt-in the records stay on `metadata_branch`; **`terminal_publish: true`** accepts a direct machine commit onto the integration branch and gates both publish shapes (change close-out and `docket-adr`'s publish); inert in `main`-mode. After a merge lands, both merge sites run the best-effort, FF-only `sync-integration-branch.sh` once at end of run (a no-op in `main`-mode and on any non-FF/dirty/feature-branch tree).
