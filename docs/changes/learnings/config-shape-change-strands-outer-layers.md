---
slug: config-shape-change-strands-outer-layers
hook: "A repo-committed config migration cannot reach the machine-local and global layers — change an existing key's SHAPE, or make an optional one REQUIRED, and every install with an override breaks the moment the PR merges."
topics: [config, migration, compat]
changes: [127, 205]
created: 2026-07-23
updated: 2026-08-05
promotion_state: retained
promoted_to:
---

## Apply
Adding a *new* key is safe: the layers that predate it simply do not set it. Changing an **existing**
key's shape — scalar to map, string to list, one enum to another — is a migration, and a migration
authored inside the repo can only reach the repo-committed layer. **So is tightening a rule about a
key whose shape never changes:** making a previously-optional key *required*, narrowing an enum, or
removing a documented fall-through strands the outer layers exactly the same way, because what
breaks is not the value's form but its newly-insufficient absence. Ask the question by consequence —
"does a config that was valid before the merge stay valid after it?" — never by whether a key was
added or its type edited. Machine-local
(`.docket.local.yml`) and user-global (`~/.config/docket/config.yml`) files are gitignored or wholly
outside the tree; no PR can edit them, no test fixture exercises them, and the change is green
end-to-end right up until it merges onto a machine that carries the old shape.

Before shipping a shape change to an existing key, pick one deliberately:

- **A compatibility shim** — accept the old shape and normalize it, ideally with a deprecation
  warning. The only option that keeps every existing install working through the merge.
- **A hard cut** — accept only the new shape, and treat the merge as a **breaking change for every
  human with an override**. Then it is not enough to be correct: the resolver's failure must name
  the offending file and print the exact replacement, the merge-gate action must be written into
  the results file, and whoever merges must fix their own machine before the next run.

The failure mode is unusually harsh because the broken layer sits *upstream of the toolchain that
would report it*: the first casualty is the resolver every skill calls at Step 0, so the whole
system stops rather than degrading. Weigh that against the cost of the shim before choosing the cut.

Related: [[config-knob-ship-end-to-end]] (a knob is not done when it merely works),
[[printed-remedy-state-validity]] (the remedy you print must be valid in the state that produced
it), [[config-layer-write-and-read-hazards]] (the hazards of introducing a layer in the first place).

## War story
- 2026-07-23 (#127, PR #123) — `auto_capture` was widened from a scalar to a map
  (`{enabled, types}`) with **no compatibility shim**. The committed `.docket.yml` was updated in
  the same PR and every test passed, but the machine that authored the change carried
  `auto_capture: true` in its gitignored `.docket.local.yml` — a file the PR could not touch. The
  build correctly identified this **before** the merge and wrote it into the results file as a
  required merge-gate action, naming the file and quoting the replacement block; the resolver also
  prints the same remedy on failure. Without that hand-off the next docket run on that machine —
  any skill, at Step 0 — would have hard-failed with no obvious cause, and the same would hold for
  every other clone with an override. Fixed by hand-editing the machine-local layer at close-out.
  Note the detection asymmetry: the suite is hermetic over fixtures, so **no test can see this**;
  only reasoning about the layers outside the repo catches it.
- 2026-08-05 (#205, PR #156) — Second hit, and the one that showed the trigger is not the *shape*.
  ADR-0067 made a `runner:`-bearing agent's `model:` **required**, replacing a documented silent
  fall-through to the child's own default (`scripts/runners/codex.md` had said "Omitted ⇒ the child's
  own default model"). No key was added, no type changed, and the committed `.docket.yml` needed no
  edit at all — yet every existing model-less codex or cursor delegation in a `.docket.local.yml` or
  `~/.config/docket/config.yml` became a **generation-time error** on merge, in files the PR could
  not reach. The hard cut was the right call: under OpenRouter the child's default is pay-per-token
  and of unknown identity, so the old behavior failed on the bill rather than in the run, and the
  rule was applied runner-wide so "is a model required?" is not an adapter-by-adapter fact learned
  twice. The cut's obligations were met — the error names the offending agent and says what to add,
  and the results file records the breaking change — but note the failure mode this one carries that
  #127's did not: the abort happens mid-loop *during wrapper generation*, so the affected agent is
  left with a zero-length wrapper and its siblings later in glob order go stale (captured as #0207).
  A hard cut is not done when the resolver refuses correctly; ask what partial state the refusal
  leaves behind.
