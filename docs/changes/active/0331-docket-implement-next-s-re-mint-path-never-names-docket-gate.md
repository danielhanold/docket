---
id: 331
slug: 'docket-implement-next-s-re-mint-path-never-names-docket-gate'
title: 'docket-implement-next''s re-mint path never names docket gate launch, so a resumed run cannot produce the run directory evidence record requires'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-08-19'
updated: '2026-08-19'
depends_on: [316]
stacked_on:
related: [330]
discovered_from: [316]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

An autonomous docket-implement-next run resuming change 0316 wedged four times in a row and could not finish, burning roughly forty minutes on repeated whole-suite runs. Every cycle it ran the suite green, failed to record evidence, and parked. The cause is a two-sentence gap in its own skill.

`skills/docket-implement-next/SKILL.md` line 88 tells the agent what to do when the build's evidence is missing or stale: "re-run the full suite once to mint the record yourself rather than reviewing an uncertified branch." Line 90 then requires `docket evidence record --id <id> --run <absolute-run-dir> --head <feature head>`, sourced from "a **passed** terminal gate observation".

Neither line says how to produce that run directory. The only producer is `docket gate launch`, which allocates a supervised run slot; `docket evidence record` refuses without one. And `docket gate launch` is named nowhere in docket-implement-next — it appears in `docket-build/SKILL.md`, `docket-build/references/delegation-execution.md`, `docket-finalize-change/SKILL.md`, and `docket-finalize-change/references/gate-failure.md`, but not in the skill that carries the re-mint instruction.

So the natural reading of "re-run the full suite" is to invoke the suite command directly (`scripts/run-tests.sh`), which is exactly what the agent did on all four attempts. That produces a green suite and NO run slot, so `--run` has nothing to point at, `evidence record` cannot be satisfied, and the run cannot reach its Step 7 postcondition. Confirmed empirically: after four cycles no gate run directory existed anywhere on the machine (`~/.local/share/docket`, the worktree, or the temp tree). Recovery required a human to run `docket gate launch --root <dir> --cwd <worktree> -- scripts/run-tests.sh` by hand and pass the resulting slot to `evidence record`.

This is not a rare path. In the ordinary flow the build role launches the gate and implement-next reuses its run directory, which is why the gap stayed invisible. The re-mint path fires whenever the evidence is missing or stale relative to HEAD — most notably on the halt/resume flow that change 0316 just made first-class (`docket change halt` / `resume-halted`), where the original gate run is long gone and any review fix moves HEAD and invalidates prior evidence. Expect it to recur on every resumed run.

Note for scope: the sibling step is NOT defective. `docket pr publish` creates the GitHub PR but does not write the manifest; `docket change mark-implemented` does that, and line 110 documents it thoroughly. An agent that never reaches Step 7 simply never gets there. Only the run-directory gap needs fixing.

## What changes

Name `docket gate launch` as the producer of the run directory on docket-implement-next's re-mint path, at the point of use. The evidence step should read as a chain an agent can execute without inferring a missing verb: launch a supervised run, observe it to a terminal `passed` state, then record evidence against that run directory and the exact head.

State the required argument shape where the instruction lives, since both flags are non-obvious: `--root` (a directory holding run slots) and `--cwd` (the absolute feature worktree). Reuse the phrasing already landed in `docket-build/SKILL.md`, which describes the Go-v1 gate as the native `docket gate launch` / `observe <run-dir>` / `stop <run-dir>` supervisor, rather than inventing a second vocabulary for the same thing.

Add a guard asserting that wherever docket-implement-next instructs an agent to mint evidence itself, the run-directory producer is named in the same neighbourhood. Key it on shape rather than an exact sentence — per AGENTS.md, a guard keyed on a spelling breaks on the first rewording — and give it a non-vacuity anchor so a moved or emptied SKILL.md reddens rather than passing. Mutation-test it: remove the `gate launch` mention and confirm it reddens.

While in the file, check the same class elsewhere: any other step that consumes an artifact without naming the verb that produces it. Derive those from a read of the skill, not from this report.

## Out of scope

Changing `docket evidence record`'s interface or relaxing its run-directory requirement — the refusal is correct, the instruction is what is incomplete. Not `docket change mark-implemented`, which is already documented. Not the yield-versus-block behavior of dispatched children (docket-build's *Gate execution posture* point 4), which is a separate contributing factor to the same incident and is correctly documented today. Not the deferred learnings harvest that would normally have captured this lesson.
