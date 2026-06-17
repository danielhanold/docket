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
finalize:                    # merge gate (change 0015): rebase onto base + re-test before merge
  gate: local                # local (default, on) | ci | both | off  — off = pre-0015 (trust the PR's CI)
  test_command:              # OPTIONAL; unset => finalize auto-detects the suite
github_project:              # {owner, number} of the auto-managed Projects v2 board; unset ⇒ auto-create on first github sync
agents:                      # per-skill subagent model/effort (change 0016); see "Agent layer" below
```

`.docket.yml` lives on the repo's **default branch (`origin/HEAD`)**, NOT on the integration branch — `integration_branch` is a value *read from* the file, so the file cannot be located *by* it. The default branch is discoverable with zero prior config, but `origin/HEAD` is not reliably populated, so skills **repair it first**: `git remote set-head origin -a`, then resolve `git symbolic-ref refs/remotes/origin/HEAD`. Read config authoritatively via `git show origin/HEAD:.docket.yml` (after a fetch); the working-tree copy is trusted only on the default branch's *primary* checkout. **A ref-unresolvable `origin/HEAD` ≠ a file-absent default branch:** if `origin/HEAD` resolves but the file is genuinely absent ⇒ defaults apply (`metadata_branch: docket`, `integration_branch: auto`); if `origin/HEAD` is unresolvable or `origin` is unreachable ⇒ do **not** assume defaults (abort with a clear error, keying on the `set-head`/fetch return code, never on `git show` — a cached `origin/HEAD` lets `git show` succeed with stale bytes). The file then **declares `integration_branch`**, which may differ from the default branch (default `main`, integration `develop`). `metadata_branch` resolves where PM commits land; `integration_branch` (default `auto` → `origin/HEAD`, fallback `main`; explicit `main`/`develop` verbatim) resolves where code lands — feature branches always cut from `origin/<integration_branch>`. **Backward-compatible opt-out:** pinning `metadata_branch: main` (with `integration_branch: main`) reproduces today's single-branch behavior exactly — no `docket` branch, no `.docket/` worktree.

**`board_surfaces` — the board as 0..n derived views.** The board is a *derived view* over the change files; `board_surfaces` lists which surfaces to render. Members: `inline` (the committed, offline-safe `BOARD.md`) and `github` (the one-way Issues + Projects v2 mirror, see *GitHub board mirror*). Default `[inline]` — backward-compatible, and `github` is strictly opt-in (an existing repo never starts minting issues until it asks). `[inline, github]` renders both; `[github]` is GitHub-only; **`[]` disables the board entirely** — no `BOARD.md`, no mirror, the change files plus git history remain the only (and fully authoritative) record. An unknown token is warned-and-ignored (a typo must never abort a build); a non-GitHub remote silently drops `github`. `github_project` is consulted only when `github` is enabled, and is minted-and-written-back on first sync if unset (see *GitHub board mirror*).

**`finalize` — the rebase-retest merge gate.** `finalize.gate` (`local` default · `ci` ·
`both` · `off`) governs `docket-finalize-change`'s merge step: before docket merges, it
rebases the feature branch onto `origin/<integration_branch>` and re-validates the merged
result, merging only if green. `local` runs the repo's suite locally (auto-detected, or the
`finalize.test_command` override); `ci` polls GitHub checks; `both` requires both; **`off`**
restores pre-gate behavior (merge trusting the PR's own CI). The gate is **finalize-only** —
the `docket-status` sweep never merges, so it has nothing to gate. Details: the gate flow and
its two judgment-tier agents live in `docket-finalize-change`.

### Agent layer — model/effort-pinned subagents (change 0016)

Each **autonomous** docket skill can run as a model/effort-pinned **subagent** instead of inline at the session model. Five skills get a wrapper — `docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`; the two **interactive** skills (`docket-new-change`, `docket-groom-next`) stay inline and only surface an **advisory** recommended model/effort at startup (a skill cannot force the session model). `docket-convention` is not an agent — it is injected into every wrapper via `skills:`.

A wrapper is a thin file: it pins `model` + `effort`, injects the skill via `skills: [<skill>, docket-convention]`, and adds a one-line directive. The skill body stays the single source of behavior. Because a subagent cannot pause to ask a human, every autonomous wrapper carries an **abort-and-report** rule: an unmet precondition or blocking ambiguity (e.g. finalize finding a PR not actually approved, a merge conflict, or a dirty worktree) is surfaced and stopped on — never turned into an interactive prompt.

**Layered config (precedence: per-repo > global > built-in).** Frontmatter is static, so configurability is a **generator** — `sync-agents.sh` — that resolves layers and writes agent files (generated copies it owns and overwrites, unlike `link-skills.sh`'s symlinks):

| Layer | Source | Generates |
|---|---|---|
| Built-in | `agents/docket-*.md` shipped in docket (each ships its default model/effort) | — |
| Global | `~/.config/docket/agents.yaml` (optional, XDG) | user-level `~/.claude/agents/docket-*.md` |
| Per-repo | `.docket.yml` `agents:` block (committed) | **project-level** `<repo>/.claude/agents/docket-*.md` |

```yaml
agents:
  implement-next: { model: opus,   effort: xhigh }
  status:         { model: sonnet, effort: medium }
  # unlisted -> built-in default; effort: auto (or omitted) -> omit the effort line (inherit model default)
```

User-level files are built-in ⊕ global; project-level files are built-in ⊕ per-repo. Claude Code applies **project-over-user precedence natively**, so the effective order is per-repo > global > built-in without the generator merging three layers per file — and because the per-repo overrides generate **committed** project-level files, the same autonomous change builds on the same model for every clone (the reproducibility guarantee). An agent with neither a built-in nor a config entry defaults to `model: inherit` with no `effort`.

`sync-agents.sh` runs **on demand** (install time, and after editing any config layer) — the same mental model as `link-skills.sh`; it does NOT hook session start (silently regenerating committed files out of band would race the commits that make overrides clone-identical). The drift backstop is **`sync-agents.sh --check`**, a CI gate that exits non-zero with a diff when committed project-level files fall out of sync with the resolved config.

**Composition (change 0017).** Nesting lets each whole-skill sub-invocation run at its own model. `docket-implement-next` **dispatches the `docket-status` subagent** at step 0 and the **`docket-adr` subagent** at step 6; `docket-auto-groom` **dispatches the dedicated `docket-auto-groom-critic` subagent** for its adversarial gate. `docket-finalize-change` **dispatches the `docket-rebase-resolver` subagent** when its merge gate hits a rebase conflict and the **`docket-integration-repair` subagent** when the rebased suite is red (change 0015) — both **foreground**, but their contract differs from the three above: the agent's report flows **back to finalize in-context** to gate the merge (continue, sign-off, or abort), and they act in the feature worktree, not on `origin/docket`. Each runs at the model/effort its own wrapper resolves through the layered config — the literal tiers are **never restated** in the dispatch prose, so a per-repo or global override can never drift from the documentation (the built-in defaults live only in `agents/docket-*.md`, per the Agent layer above). The `docket-status`, `docket-adr`, and `docket-auto-groom-critic` dispatches are **foreground** (the parent suspends until the child returns) and **unconditional** (baked into the skill body, so the sub-call gets its own model whether the parent ran as its wrapper subagent or as a plain inline skill); their contract is **git state** on `origin/docket` (and, for adr, a published ADR on the integration branch), re-read after a re-sync — never an in-context return. Three of the **eight** generated wrappers wrap **no skill** — `agents/docket-auto-groom-critic.md` (config key `auto-groom-critic`, attached to `auto-groom`), and `agents/docket-rebase-resolver.md` + `agents/docket-integration-repair.md` (config keys `rebase-resolver` / `integration-repair`, attached to `finalize-change`'s gate). Each loads only `docket-convention`, never a designer/driver skill body, so it inherits no caller bias; all are auto-discovered by `sync-agents.sh`'s `agents/docket-*.md` glob (no generator edit). (The "Agent layer" line above stays exact: **five *skills* get a wrapper**; these three are wrappers that wrap no skill — eight wrappers, five skills.)

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

`<changes_dir>/LEARNINGS.md` — the project's **build-loop memory**: a curated, hand-edited file of lessons the build loop taught, living on `metadata_branch` only (like `BOARD.md`, it is never published to the integration branch — but unlike the board it is curated prose, never regenerated). Flat dated entries, **newest first**, one to three lines each, with provenance and an actionable phrasing — e.g. `- 2026-06-12 (#12, PR #7) — <what happened, one clause>. Apply: <the rule to follow next time>.`

**Writing.** Entries are added only by the **harvest** at close-out (its procedural single source is the *Harvest learnings* step in `docket-finalize-change`; `docket-status`'s sweep invokes it by reference). Zero entries for a change is normal. Kills are not harvested — `## Why killed` already records the rationale.

**Reading.** `docket-implement-next` reads the ledger at plan time and again at its review step; `docket-groom-next` reads it before a brainstorm. No other skill reads it.

**Distilling.** Append-only until the file exceeds **~300 lines**; the next harvest past the cap also distills — merge near-duplicates and drop entries since promoted to CLAUDE.md or this convention. Distillation is **compression, not destruction**: git history keeps everything dropped. Boundary: the ledger holds lessons for the build loop; durable project conventions belong in CLAUDE.md — promotion removes the entry here.

### GitHub board mirror (shared definition)

The `github` board surface mirrors each change to one GitHub issue (and one Projects v2 item) — **strictly one-way**: change files are the source of truth, the mirror is derived output that is **never read back** (no comments, labels, assignments, or state flow into change files). It rides in the Board pass (`docket-status`) and is **best-effort**, identical to the inline board rule: it needs network + `gh` auth, it self-heals on the next pass, and it **never aborts a build**. The mirror's external-write mechanics are owned by the deterministic `scripts/github-mirror.sh` (not agent-constructed `gh` calls); the Board pass only invokes it.

**`issue:` field.** One issue per change, upserted idempotently on the per-change `issue:` field (shape of `pr:`), minted on first sync and persisted into the change file on `metadata_branch`.

**Status → issue mapping (all seven).** Active states (`proposed`, `in-progress`, `blocked`, `deferred`, `implemented`) keep the issue **open**; terminal states close it with the native reason — `done` → closed as **completed**, `killed` → closed as **not planned**. The sync is the **sole writer** of issue open/closed state and reason: a PR may *reference* its mirror issue (a plain `#N` link, for the linked-PR "awaiting merge" view) but never `Closes #N`, which would make GitHub a second writer that cannot express `killed → not planned`.

**Labels — `docket:` namespace only.** Mirror labels are prefixed `docket:` (`docket:status/<state>`, `docket:priority/<p>`, and the derived `docket:readiness/<needs-brainstorm|auto-groom-blocked|build-ready>` / `docket:waiting/<needs-your-merge|not-yet-built>`). docket creates/updates only labels inside that namespace and never touches a label it did not mint, so existing repo labels are collision-proof.

**Issue body.** A visibility pointer, never a second home for the content: a one-way banner, a one-line frontmatter digest, the `## Why` distilled to a sentence or two, and hrefs to every relevant artifact (the change file on `metadata_branch`, the `spec:`, each ADR in `adrs:`, and `plan:`/`results:` once those resolve on the integration branch).

**Projects v2.** The optional half of `github`. When `github_project` is unset, first sync mints a **private** Projects v2 board under the integration repo's owner (Status single-select seeded from the active statuses) and writes its `{owner, number}` back into `.docket.yml` on the default branch — a one-time config commit that keeps later runs idempotent. Missing `project` token scope or any GraphQL failure ⇒ skip Projects and still mirror Issues + labels.

### Bootstrap guard (`docket`-mode first-run safety)

At startup, after resolving config, when `metadata_branch == docket`, fetch origin and evaluate a 2×2 over two probes (both stated over the **same vocabulary**):

- **`DOCKET`** = the `docket` branch exists (origin OR local): `git rev-parse --verify --quiet refs/remotes/origin/docket || git rev-parse --verify --quiet refs/heads/docket`.
- **`LIVE`** = the **live planning surface** still sits on the integration branch: `git ls-tree origin/<integration_branch> -- <changes_dir>/active <changes_dir>/README.md <changes_dir>/BOARD.md` (non-empty ⇒ present). Probe **only** this pruned surface — `archive/`, `<adrs_dir>/`, and pre-migration specs deliberately *stay* on integration, so probing them would read `LIVE` forever. Use `git ls-tree`, never bare `<ref>:<path>`. If `origin/<integration_branch>` does not resolve, `ls-tree` exits ≠0 with empty stdout — treat that as a **hard config error**, not `¬LIVE`.

| | `LIVE` | `¬LIVE` |
|---|---|---|
| **`¬DOCKET`** | existing single-branch repo → **STOP**, point to `migrate-to-docket.sh`; never auto-create or move data | fresh repo → create the empty orphan `docket`, push, **proceed** |
| **`DOCKET`** | **half-migrated** (interrupted run) → **STOP**, point back to `migrate-to-docket.sh` to finish its prune | migrated → **proceed** |

The guard is a no-op in `main`-mode (`DOCKET`/`LIVE` are only evaluated when `metadata_branch == docket`). The migration itself lives in the standalone `migrate-to-docket.sh`.

### Branch model

Metadata (change files, `BOARD.md`, ADRs, specs) commits to `metadata_branch` (default `docket`) via the **metadata working tree** — which is the **primary working tree on the integration branch** in single-branch (`main`) mode, and the persistent **`.docket/` worktree** in `docket`-mode — and is **always pushed to its remote immediately** (a local-only orphan branch defeats the purpose; the backlog, board, specs, and ADRs stay browsable on the remote at all times). A change's `feat/<slug>` branch is **ALWAYS cut from `origin/<integration_branch>`** (default trunk `main`; `develop` for GitFlow) — `metadata_branch` only redirects bookkeeping commits, never where code branches start. The feature branch adds only the plan + results + code and **never modifies** docket metadata. On a terminal transition (`done` *or* `killed`), the driving skill runs the shared **terminal-publish** procedure: it **copies** the change's terminal records (the archived change file + its `spec:` if set + the **`Accepted`** ADRs in `adrs:`) from `origin/docket` onto the integration branch in one dedicated commit and pushes (`git checkout origin/docket -- <paths>`, never a `git merge docket`) — the only flow of metadata onto the code line. (In `main`-mode the metadata working tree *is* the integration branch, so terminal-publish is skipped — the archive move there is itself the terminal record.)
