---
slug: defaulting-an-unset-collection-skips-the-leg
hook: "Under set -u, ${LIST:-} turns a crash into a silently skipped leg — and the helper you call to populate LIST may itself read another unset variable."
topics: [shell, defaults, validation]
changes: [207, 211]
created: 2026-08-05
updated: 2026-08-05
promotion_state: retained
promoted_to:
---

## Apply
When code that walks a collection reaches a path where that collection was never populated, `set -u`
aborts. There are two remedies and they are not interchangeable.

**`${LIST:-}` is almost always the wrong one.** It silences the abort by substituting an empty list,
and an empty list makes the loop body execute zero times — so the leg the code exists to perform is
**skipped in silence**, at exit 0. That converts a loud failure into an invisible one, and it is
strictly worse when the loop is a *validation* leg: the gate reports clean precisely because it
checked nothing. Reach for it only when "empty" is a genuine, intended state, and never when the
population step simply has not run yet.

**Establish the value instead — and follow the chain to the bottom.** The populating helper is
itself code with preconditions, and on a path where your variable is unset its inputs are usually
unset too. Calling the helper is not automatically a fix; you have to verify that *it* can run in
the state you are in. The shape that works is a guarded chain — establish the prerequisite only if
absent, then populate:

```sh
[ -n "${PREREQ_SET:-}" ] || resolve_prereq
compute_the_list
```

Note the one legitimate `:-` in that idiom: it tests **whether** something is set, which is exactly
what `:-` is for, rather than papering over the fact that it is not.

The general rule, which outlives the shell specifics: **when you suppress an error, ask what the
suppressed state makes the code *do*, not just what it stops it from *printing*.** An unset scalar
that defaults to empty and gets compared is usually harmless; an unset *collection* that defaults to
empty and gets iterated silently deletes work. Same syntax, opposite consequences. This is the
collection-shaped sibling of [[best-effort-helper-on-a-sole-deliverable-path]] — a soft failure on a
path where the output *is* the deliverable.

A second-order caution: this hazard is concentrated on the **secondary** entry point — the
`--check`/`--dry-run`/validate-only mode that skips the resolution steps the main path performs. That
mode is also the one fixtures cover least, so the crash and its wrong fix can both ship green
([[green-suite-untested-branch]]).

## War story
- 2026-08-05 (#207, PR #159) — `sync-agents.sh` gained a pre-flight gate that walks every
  (pass, agent, harness) triple before generating any wrapper. The plan anticipated that
  `$USER_TARGETS` is unset on the `--check` path and prescribed calling `compute_user_targets`. The
  build found the plan's remedy was insufficient by one level: `compute_user_targets` itself reads
  `$USER_HARNESSES_SET`, which is *also* unset on `--check`, because `resolve_global_agent_harnesses`
  never runs there. The plan's fix would have died under `set -u` at the next line down. Resolved as
  `[ -n "${USER_HARNESSES_SET:-}" ] || resolve_global_agent_harnesses` followed by
  `compute_user_targets`. `${USER_TARGETS:-}` was considered and **explicitly rejected** — it would
  have made the gate's entire user-level leg iterate zero times and report clean, which is exactly
  the under-enumeration the gate was built to prevent, and that leg guards `~/.claude/agents`, the
  widest blast radius of the original bug. The fix is real but unverified: every `runner:` fixture in
  the suite lives in `.docket.yml`, so nothing reddens if the whole block is deleted.
- 2026-08-05 (#211, PR #160) — **The containment case: `set -u` inside `$( … )` does not abort the
  script, it aborts the subshell.** `board-checks.sh`'s new leg C guards an empty base-set before
  expanding `${ar_bases[@]}`. A review finding predicted that deleting the guard would kill the whole
  per-change walk and exit non-zero. It does not — the expansion lives inside a command
  substitution, so the failure kills only that subshell; the parent walk continues, every later
  change still reports, and the script exits **0**. The visible symptom is not a crash but a
  *misfire*: on bash ≥ 4.4 the leg emits with an empty base label, and on 4.0–4.3 the diagnostic
  appears on stderr only. So the same defect this finding names — a `set -u` failure that costs you
  a leg instead of the run — arrives here without anyone writing `:-` at all: **`$( … )` is an
  implicit error-suppressor for everything expanded inside it**, and the wider the surrounding loop,
  the more thoroughly it hides. When auditing where a collection can be unset, treat every command
  substitution boundary as a place the abort you were counting on will be swallowed.
