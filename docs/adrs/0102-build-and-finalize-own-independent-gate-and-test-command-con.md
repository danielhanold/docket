---
id: 102
slug: 'build-and-finalize-own-independent-gate-and-test-command-con'
title: 'Build and finalize own independent gate and test-command configuration'
status: 'Accepted'
date: '2026-09-01'
supersedes: [63]
reverses: []
relates_to: [74, 95, 99]
change: 374
---

## Context

ADR-0063 established docket's own build role: a controller skill bound through `skills.build`, a compact per-task worker contract, profile-routed named agents, one bounded escalation per task, no per-task review, and a single whole-suite gate at the end of the build. Those decisions have held.

Its Decision 5 also settled *where the build gate's command comes from*: the end-of-build suite run was "derived from `finalize.test_command` or finalize's existing auto-detection rather than a second, driftable test-command key." The reasoning was drift avoidance — one command, one place to change it, no chance of a build gate and a merge gate silently testing different things.

Operating the shared key exposed three defects that the drift argument does not cover.

First, the two gates are not the same gate. The build's suite run is a fast in-loop signal over a work-in-progress branch, run repeatedly during a build; finalize's is a slow pre-merge verdict over a rebased head. A single command forces one to be wrong: sized for the merge gate, the in-loop run is too slow to run at the cadence a build needs; sized for the build, the merge gate under-tests what is about to land. Repos with a fast unit target and a slow full suite had no way to express that at all.

Second, the shared key made `build.gate: off` unrepresentable in evidence. With no command of its own, a build whose gate was off had nothing to record, and the evidence record could not distinguish "the suite ran and passed" from "no suite was run" — a disabled gate read as green, which is exactly the failure ADR-0074 forbids for the *tri-state verdict* and which the same reasoning forbids here for the evidence.

Third, the `auto` sentinel resolved test-command discovery at *runtime*, inside the run that depends on it. Discovery is a setup-time question about the repository, and answering it mid-run meant every build and every finalize paid the probe, could disagree about the answer, and could fail in a place where the correct posture (a halt, per ADR-0074) is expensive and confusing rather than in a place where a human is present to fix the configuration.

Drift between the two gates remains a real concern. What ADR-0063 got wrong was the remedy: it prevented drift by forbidding a second key, when drift is better handled by *detecting* it at the moment it matters — when finalize is deciding whether it can trust an existing build result.

## Decision

Build and finalize own **independent** `gate` and `test_command` configuration pairs. ADR-0063's Decision 5 shared-command rule is replaced by the following; every other ADR-0063 build-role decision is carried forward unchanged.

1. **Two independent pairs.** `build.gate` / `build.test_command` govern the end-of-build suite gate; `finalize.gate` / `finalize.test_command` govern the pre-merge gate. Each pair is resolved on its own.

2. **No cross-fallback.** Neither gate falls back to the other's command. An unset `build.test_command` does not silently borrow `finalize.test_command`, and the reverse never happens either. A gate that is on with no resolvable command is a configuration gap, handled per ADR-0074 — a halt, never a red suite.

3. **`build.gate: off` produces truthful evidence.** A build whose gate is off records its suite outcome as **skipped** — a distinct, first-class evidence value. It is never recorded as green, and no downstream consumer may read a skipped outcome as a passing one.

4. **The runtime `auto` sentinel is removed.** Test-command discovery moves to setup time via `docket repository configure-tests`, which probes the repository once, with a human present, and writes the resolved commands into configuration. Runtime resolution reads configuration only; it never probes.

5. **Evidence reuse is conjunctive.** Finalize may reuse a build's suite evidence in place of running its own gate only when **all three** hold: the evidence is **green**; its recorded `head_sha` matches the feature head finalize is about to gate; and its recorded command **byte-matches** the resolved `finalize.test_command`. If any conjunct fails — including a skipped or red outcome, a stale head, or a command that differs by so much as a flag — finalize runs its own gate. This conjunction is where drift is caught, and it is what makes two independent keys safe.

**ADR-0074's tri-state verdict rule remains in force and is unaffected**: configuration gaps and launch failures are halts, never red suites. Nothing here converts a missing command, an unresolvable configuration, or a runner that failed to start into a test failure.

## Consequences

- A repo can size each gate for its job: a fast in-loop build command and a thorough pre-merge command, expressed directly instead of compromised into one value.
- `build.gate: off` becomes honest. Evidence says `skipped`, the board and finalize can both see it, and no consumer can mistake an unrun suite for a passing one.
- Discovery failures move to a setup command run with a human present, where the fix is cheap, instead of surfacing mid-build as a halt.
- Drift is now detected rather than prevented by construction. The byte-match conjunct is deliberately strict: a command differing by a single flag forces finalize to run its own gate. That trades some redundant test time for the guarantee that a reused green never stands in for a run that was not actually equivalent.
- **What is given up: the single-key simplicity of ADR-0063's Decision 5.** There are now two commands to configure and keep coherent, and a repo that wants one command must write it twice. In exchange, the common case — both keys set to the same command over an unchanged head — still reuses build evidence and runs the suite once, so the shared-key benefit survives where it was actually load-bearing.
- Existing repos relying on the implicit fallback must set `build.test_command` explicitly (or run `docket repository configure-tests`); the fallback's removal is visible as a configuration-gap halt rather than a silent behavior change.

This ADR **supersedes ADR-0063**. Every ADR-0063 build-role decision other than its Decision 5 — profile routing across the economy/standard/premium/max profiles, one bounded escalation per task, no per-task review with a single whole-branch review, model and effort as properties of named agents, and a single whole-suite gate at the end of the build — is carried forward unchanged.

## Alternatives considered

- **Keep the shared key and add a build-only override.** Rejected: an override is a second key with a fallback, which reproduces the silent-borrow failure the no-cross-fallback rule exists to remove, while keeping the ambiguity about which value actually ran.
- **Keep `auto` but resolve it once per run and cache it.** Rejected: caching does not make a mid-run probe a good place to discover a configuration answer, and it leaves the two gates able to disagree across runs.
- **Let finalize always run its own gate and never reuse build evidence.** Rejected as needlessly expensive: when the head and the command match exactly, a second identical run adds no information. The three-way conjunct is the smaller price.
- **Reuse build evidence on head match alone.** Rejected: without the command byte-match, a build gated by a fast subset would silently satisfy a merge gate configured for the full suite — precisely the drift ADR-0063 set out to prevent.
