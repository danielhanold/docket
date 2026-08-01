<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0184 — Four-tier build profile ladder — low/medium/high/max replaces economy/standard/premium](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0184-four-tier-build-profile-ladder.md)**
<!-- docket:backlink:end -->

# Four-tier build profile ladder — design

**Change:** #184 · **Date:** 2026-08-01 · **Status:** approved design, awaiting build

## Problem

`docket-build` routes each plan task to one of three profiles — `economy`, `standard`,
`premium`. Two observed/structural problems:

1. **Direct routing catches too much.** The premium named-risk list (auth/security,
   migrations, concurrency, release infra, unresolved architecture) sweeps a lot of
   ordinary work into the most expensive profile.
2. **No room at the top.** With three levels there is no way to make the "risky work"
   tier routine-safe *and* keep a genuinely rare top tier: the single top slot has to
   serve both roles, so it normalizes.

A secondary problem: on the Claude harness all three profiles pin the *same model*
(Opus) at different efforts, so `economy` never delivered a genuinely cheap floor.

## Decision

Replace the three profiles with **four**, named **`low` / `medium` / `high` / `max`**.
Clean break: the old names are removed, not aliased.

### Routing rubric

The asymmetry principle is preserved — the cheap tier must be *positively established*,
named risk selects upward, uncertainty defaults to the middle:

- **`max`** — direct routing for **unresolved architecture** and **irreversible data
  changes/migrations** ONLY. Nothing else classifies here.
- **`high`** — the remaining current premium triggers demote here: authentication or
  security boundaries, concurrency or locking, release infrastructure, plus any other
  named consequential risk.
- **`medium`** — everything remaining; the default and the uncertainty sink (normal
  feature, integration, refactor, and debugging work).
- **`low`** — only when positively established: fully specified, localized to roughly
  one or two files, follows an established pattern, no consequential risk.

Plan override: `**Build profile:** <low|medium|high|max>`. The old tokens
(`economy`/`standard`/`premium`) are invalid values — the existing invalid-override
rule applies unchanged (halt as a plan contract error; remedy is editing the plan
line). No legacy aliases.

Rarity of `max` is enforced by three narrow doors: the two-item direct rubric, the
plan override, and the `high → max` escalation.

### Escalation ladder

One rung, at most once per task; all existing malformed-return, stray-commit, and
never-climb-twice rules carry over unchanged:

```text
low    -> one medium retry
medium -> one high retry
high   -> one max retry
max    -> halt
```

Note the structural effect: today a failed `standard` task escalates to the top tier;
after this change a failed `medium` lands on `high`, one notch below the top. The
escalation side door into the most expensive rung narrows to `high`-routed tasks that
fail.

### Integration-repair ladder

`high → max → halt` (replacing `standard → premium → halt`). Rationale: the repair
task is cross-task diagnosis — never routine work — and in Claude-harness pin terms
`high → max` is byte-identical to today's repair ladder (Opus/medium → Opus/high), so
repair strength does not regress while the rest of the ladder gets cheaper.

### Shipped pins (`agents/harness-defaults.yml`)

The compression intent: `max` inherits today's premium pin (no new headroom above);
every rung below is at or below today's cost; `medium` — the default tier — drops a
notch, which is where the bulk of the savings comes from.

```yaml
# claude
build-low:    { model: claude-sonnet-5, effort: low }
build-medium: { model: claude-opus-5,   effort: low }     # today's economy pin
build-high:   { model: claude-opus-5,   effort: medium }  # today's standard pin
build-max:    { model: claude-opus-5,   effort: high }    # today's premium pin

# cursor (effort auto throughout, per the existing cursor-block rule)
build-low:    { model: cursor-grok-4.5-low,    effort: auto }
build-medium: { model: cursor-grok-4.5-medium, effort: auto }  # today's economy
build-high:   { model: cursor-grok-4.5-high,   effort: auto }  # today's standard
build-max:    { model: claude-opus-5-high,     effort: auto }  # today's premium

# codex (model/effort PAIRS are model-specific roles, not cross-model ordinals)
build-low:    { model: gpt-5.6-luna,  effort: xhigh }   # unchanged — already the cheap floor
build-medium: { model: gpt-5.6-terra, effort: medium }  # one notch below today's standard
build-high:   { model: gpt-5.6-sol,   effort: low }     # strong model, modest reasoning
build-max:    { model: gpt-5.6-sol,   effort: medium }  # today's premium
```

On `build-low` (claude): Haiku (`claude-haiku-4-5-20251001`) was considered and
rejected as the *shipped* default — the worker contract is long and strict, and its
failure modes (malformed return, stray commit, unverifiable COMPLETE) **halt** the
build rather than escalate, so contract-fumble risk lands on the human. Haiku remains
the documented cost-aggressive **user-layer override** for repos that want it; document
this in the pins comment or skill prose, not as a shipped default.

## What this touches

- `skills/docket-build/SKILL.md` — profiles table (four rows), routing rubric,
  escalation ladder, repair ladder, frontmatter description.
- `skills/docket-build-task/SKILL.md` — "three profiles" language and any
  profile-name references.
- `agents/docket-build-*.md` — rename the three wrappers to `docket-build-low` /
  `-medium` / `-high`, add `docket-build-max`; each still preloads only
  `docket-build-task` (`skills: [docket-build-task]`), and each body's profile
  self-description updates to the new tier semantics (e.g. the max wrapper: rare
  extreme work; escalation destination from high; an initial-max escalation request
  halts).
- `agents/harness-defaults.yml` — all three harness blocks get complete 4-row build
  sets (the validator requires key-set equality with the `agents/docket-*.md` glob,
  so completeness is forced, not optional).
- `docket-convention` SKILL.md + agent-layer reference — wrapper counts ("twelve
  wrappers", "three are docket-build's task workers") become **thirteen / four**;
  any economy/standard/premium mention updates.
- `sync-agents.sh` / `install.sh` / dispatch-rule generation — expected to pick up
  the new set via the existing glob; verify no hardcoded profile names remain
  (tests included).
- Docs/README mentions of the three profiles.

**Not touched:** the build gate, the review boundary, the checkpoint ledger format
(it records whatever profile name was used), `docket-implement-next`, the escalation
protocol's rules themselves (only the rung names change).

## Compatibility

Clean break, called out as a release note:

- A pre-existing plan carrying an old explicit `**Build profile:**` token halts with
  the existing invalid-value diagnostic; remedy is editing the plan line.
- User config layers overriding `build-economy`/`build-standard`/`build-premium`
  resolve against nothing and are warned-and-ignored per the existing unknown-key
  posture; those machines fall back to shipped defaults until the user renames keys.
- A checkpoint ledger written under old profile names fails its resume validation
  conservatively (task re-verified from commits, per existing rules) — acceptable.

## Out of scope

- Any change to the worker contract (`docket-build-task`) beyond renaming profile
  references.
- Routing telemetry / cost accounting (a possible future change).
- Changing non-build agents' pins.

## Open design points settled during brainstorm

- Top-tier reachability: narrow two-item direct rubric + override + escalation
  (chosen over escalation-only).
- `build-low` claude pin: Sonnet/low over Haiku (contract-discipline risk halts
  runs; savings smallest at the bottom rung).
- Naming: `low/medium/high/max`, no legacy aliases.
- Repair ladder: `high → max`, preserving today's effective repair strength.
