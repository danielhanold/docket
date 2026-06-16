---
id: 8
slug: agent-layer-generated-subagents
title: Agent layer — generated subagent wrappers from layered config
status: Accepted
date: 2026-06-16
supersedes: []
reverses: []
relates_to: [1, 3]
change: 16
---

## Context

docket skills run inline at the session model/effort. The skills differ enormously
in blast radius — the fully autonomous, no-human-backstop skills (`implement-next`,
`auto-groom`) versus near-mechanical board rendering — yet all inherit one
undifferentiated session model. We want per-skill control of **model** and
**effort**, configurable without editing skill bodies, with sane built-in defaults.

Three forces constrain the design:

- **Subagent frontmatter is static** — no templating, no runtime config reads. So
  "configurable" requires a **generator** that writes agent files from resolved
  config; it cannot be a live `.docket.yml` read at invocation time.
- **Reproducibility** — docket's standing rule is that config is identical for
  every clone, agent, and device (the same rationale as [[0001]]'s metadata-branch
  model). The skills that most need pinning (the autonomous ones) are exactly the
  ones where "builds on Opus locally, Sonnet on CI" would be a silent correctness
  hazard.
- **Autonomy** — a subagent cannot pause to ask a human. A wrapped autonomous skill
  that hits an unmet precondition has no one to prompt.

## Decision

Introduce an **agent layer**: thin wrapper files that pin model+effort and inject
the existing skill via `skills:` (the skill body stays the single source —
[[0003]]). Five autonomous skills get wrappers (`implement-next`, `auto-groom`,
`finalize-change`, `status`, `adr`); the two interactive skills (`new-change`,
`groom-next`) stay inline with an **advisory** recommendation only (a skill cannot
force the session model). Four sub-decisions:

1. **A separate generator, `sync-agents.sh`, writing generated copies** — not an
   extension of `link-skills.sh`. `link-skills.sh` symlinks `skills/<name>`;
   agent files bake resolved model/effort, so they must be copies the generator
   owns and overwrites. The built-in defaults are the committed `agents/docket-*.md`
   source files (each ships its default model/effort); the generator globs them, so
   adding a wrapper needs no script edit.

2. **Two-layer generation leveraging Claude Code's native project-over-user
   precedence — never a 3-way merge.** User-level files (`~/.claude/agents/…`) are
   built-in ⊕ global (`~/.config/docket/agents.yaml`); project-level files
   (`<repo>/.claude/agents/…`, committed) are built-in ⊕ per-repo (`.docket.yml`
   `agents:`). The harness picks project-over-user for free, yielding the effective
   order **per-repo > global > built-in**. Crucially, the committed project-level
   files do **not** bake the per-machine global layer, so they stay clone-identical —
   this is what gives per-repo overrides their reproducibility guarantee.

3. **On-demand generation with a `--check` CI gate — explicitly NOT a session-start
   hook.** Generated copies can't symlink-track config the way skills do, but
   silently regenerating committed project files every session would mutate
   committed files out of band and race the commits that make overrides
   clone-identical. So generation runs on demand (install / config edit), and the
   drift backstop is `sync-agents.sh --check` (exits non-zero with a diff when
   committed project files fall out of sync with resolved config).

4. **Abort-and-report for autonomous subagents.** Because a subagent cannot ask,
   every autonomous wrapper carries an abort-and-report rule: an unmet precondition
   or blocking ambiguity (e.g. finalize finding a PR not actually approved, a merge
   conflict, or a dirty worktree) is surfaced and stopped on — never an interactive
   prompt. This is a named behavior change for finalize-as-subagent.

## Consequences

- **Enables** per-skill model/effort with reproducible per-repo overrides, and
  standalone subagent invocation of each autonomous skill at its pinned tier.
- **Harness-dir semantics diverge from `link-skills.sh`**: the generator checks the
  harness *root* (`.claude`) and creates the `agents/` subdir, rather than
  leaf-checking `.claude/agents`. `agents/` is docket-introduced and won't pre-exist
  even on a harness you use, so a leaf-check would make the feature dead on arrival.
  The cost: a present-but-unused harness gets an `agents/` dir on install — accepted,
  consistent with `link-skills.sh` populating every present harness.
- **The global file is per-machine convenience only** (it never reaches committed
  files), so it cannot break the reproducibility guarantee.
- **Config values are passed through unvalidated** — a typo (`model: gpt4`) emits an
  invalid wrapper. Deliberately not validated against an allowlist: the set of valid
  model aliases is a moving target (new aliases would trigger false warnings), and
  the convention documents the valid values. Reconsider if it proves a real footgun.
- **Composition is deferred to change 0017** (`depends_on: 16`): nested
  sub-invocations (`implement-next → status`/`adr`, `auto-groom → critic`) still run
  inline at the parent's model until 0017 rewires them. 0016 only establishes the
  wrappers, config, and generator so standalone invocation works.

## Update

**2026-06-16 (change 0017, [[0009]]).** The composition deferred in the final
consequence above is implemented in change 0017. The three nested sub-invocations
named there — `implement-next → status` (step 0) and `→ adr` (step 6), and
`auto-groom → critic` (step 3) — now dispatch **named, model/effort-pinned subagents**
**foreground**, with **git state on `origin/docket` as the contract**: the parent
suspends, re-syncs `.docket/`, and re-reads the child's commits rather than relying on
an in-context return (the `adr` dispatch additionally returns its number for the
`adrs:` write-back). The critic is materialized as the dedicated
`docket-auto-groom-critic` wrapper that loads only `docket-convention`; the rationale
for that isolation is its own decision, [[0009]]. The `Decision` above is unchanged —
this note only records that its deferred item has landed.
