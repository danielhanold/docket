---
id: 284
slug: runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi
title: 'runner-dispatch --observe is sentinel-only: adopt 0282''s identity-checked liveness probe'
status: done
priority: high
type: fix
created: 2026-08-10
updated: 2026-08-11
depends_on: []
related: [208, 270, 277]
discovered_from: [282]
adrs: [87, 88]
spec: docs/superpowers/specs/2026-08-10-runner-dispatch-observe-liveness-probe-design.md
plan: docs/superpowers/plans/2026-08-11-runner-dispatch-observe-liveness-probe.md
results: docs/results/2026-08-11-runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi-results.md
trivial: false
auto_groomable:
branch: feat/runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi
pr: https://github.com/danielhanold/docket/pull/199
blocked_by:
claimed_at: 
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-10-runner-dispatch-observe-liveness-probe-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-10-runner-dispatch-observe-liveness-probe-design.md) |
| Plan | [2026-08-11-runner-dispatch-observe-liveness-probe.md](https://github.com/danielhanold/docket/blob/feat/runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi/docs/superpowers/plans/2026-08-11-runner-dispatch-observe-liveness-probe.md) |
| Results | [2026-08-11-runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi-results.md](https://github.com/danielhanold/docket/blob/feat/runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi/docs/results/2026-08-11-runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi-results.md) |
| PR | [#199](https://github.com/danielhanold/docket/pull/199) |
| ADRs | [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0088](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0088-halt-exit-code-is-a-property-of-run-state-not-discovery-path.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while reconciling change 0282 (the liveness-keyed launch-and-wait contract
for long-running child processes). 0282 excludes `runner-dispatch.sh` from its site rewiring as a
conscious decision — its surface is agent-dispatch-specific and change 0277 is actively reworking
it — and records the excluded gap as a named residual it is obliged to file rather than absorb.
Re-verified in current source at that reconcile: the only `kill -0` in `runner-dispatch.sh` is in
its give-up path, and `runner-dispatch.md` still states "The sentinel is the *only* source of
liveness — the facade never [probes the process]".

**Opportunity** — `runner-dispatch --observe` has no process-liveness probe at all. Its predicate
is "no sentinel ⇒ still running", so a delegated agent whose process died without writing its
sentinel reads as `running` for the entire `DELEGATION_OBSERVATION_BUDGET` (default 60 minutes)
before the budget bound fires. That is the same marker-keyed-versus-liveness-keyed defect 0282
exists to remove, in the dispatch lifecycle instead of the gate lifecycle, and at ten times the
worst-case latency. 0282 ships the correct predicate — an identity-checked process-group probe
with the terminal record outranking liveness on both sides — but deliberately does not apply it
here.

**Independent value** — stands with 0282 fully reverted: `runner-dispatch`'s dead-child latency
gap is a defect in its own contract, measured against its own documented budget, and closing it
would cut the worst-case detection time for a dead delegated agent from the observation budget to
one observation interval. The value is also independent in the other direction: if 0282 lands, the
gap is the one remaining place in the repo where a wait is still marker-keyed by design.

**Narrower than the stub first read** — `runner-dispatch.sh` **already owns** the identity
conjuncts, inside `terminate_dispatch`, where they gate the group kill on the give-up path. The gap
is not that no such check exists here; it is that the check is consulted only when the facade is
about to *signal*, never as a *verdict input* one lifecycle phase earlier.

**Boundary** — `runner-dispatch.sh`'s `--observe` verb and its `runner-dispatch.md` contract
section on liveness, plus the tests covering them, plus a new `scripts/lib/docket-liveness.sh` that
`gate-run.sh` is refactored onto so the predicate has one definition rather than two. In scope:
the identity-checked probe, the record-outranks-liveness read ordering, and a git-decided
disposition for a child that died without a sentinel — with the mutation tests that redden if any
is dropped. Deliberately out of scope: the observation budget's value (change 0273), reaping
orphaned children (reversing the refuse-to-signal-unprovable-ownership rule is an ADR-level
decision), the dispatch directory layout, the harness adapters, the brief-file format, the
`--launch` verb's detachment mechanism (ADR-0080 measured it and it is not in question), and
anything about the sentinel's role as the source of *correctness* — this is about liveness only,
and correctness still comes from git via `verify-run.sh`.

**Sequencing — no dependency; the stub's caution is superseded.** The stub proposed waiting for
change 0277's rework of the same file. Reading 0277's spec settles it the other way: 0277 declares
*"any change to launch/observe semantics, the run gate, or detachment"* **out of scope**, and the
other two active changes on this file — 0208 (input gates) and 0270 (config resolution) — sit on
the input-validation side. None touches the observe leg. 0277's own assumption 8 set the house
precedent for exactly this: file collisions on `runner-dispatch.sh` are recorded as `related:` and
reconciled at rebase by intent. Recorded as `related: [208, 270, 277]`; `depends_on:` stays empty,
so a `high`-priority fix does not queue behind a `medium` one.

## Reconcile log

### 2026-08-11 — reconciled against current `origin/main`

Spec dated 2026-08-10; both primary files were reworked and merged after it (0208 `07de6e55`,
0277 `6cc79e8b`, 0270 `d0197ee5`, 0286 `fc482699`). Re-read at `7245279f`. The design holds — the
gap is still real and still exactly as described — with four adjustments and one scope confirmation.

**1. `gate-run.sh`'s `identity_of` cannot be deleted; the spec's §1 wording is superseded.**
`tests/test_gate_run.sh` carries two *source-shape* asserts that pin the spellings verbatim —
`grep -qF -- "identity_matches \"\$RUN_DIR\""` and
`grep -qF -- "SPAWN_IDENT=\"\$(identity_of \"\$SPAWN_PID\")\""`. The spec's own § Testing rule
forbids editing that file ("an edit to either file is the tell" that the refactor was not
behaviour-preserving), so deleting the symbols would force the very edit the safety net exists to
detect. Resolution: `identity_of` and `identity_matches` **survive as thin delegations** onto the
new lib — the conjunct ladder moves out, only the call-site spellings stay, so the predicate still
has exactly one definition and both asserts stay green untouched. `group_alive_and_ours` collapses
into the lib call with `recorded_pgid`/`recorded_identity` as its arguments exactly as specified;
no test pins its spelling. `identity_matches` deliberately does **not** collapse into
`docket_group_alive_and_ours`: that would add a `kill -0` conjunct to gate-run's pre-signal
re-check, which is a re-specification, and the spec forbids re-specifying gate-run.

**2. The dead path's `build-*` disposition inherits 0208's two non-verdict legs.** §3's table names
only the `verify-run --build` call. The sentinel path (change 0208, ADR-0083) additionally reports
`task-unverifiable worktree-removed` when `ANCHOR_FALLBACK=1` and `task-unverifiable
launch-branch-missing` when the launch record carries no branch — precisely because `--observe` on
a removed worktree reassigns `ANCHOR` to the repo root, and verifying there answers a question
nobody asked. Reproducing §3's table literally would regress both onto the new leg. The dead path
takes the same three-way split, with the death stated first per §3's re-wording rule.

**3. `runner-dispatch.sh` has no test-only barrier hook today.** §Testing case 3 needs one to hold
the step-1/step-3 TOCTOU window open deterministically; `gate-run.sh` owns the reference shape
(`barrier`, env-gated on a point NAME, inert by default, bounded even when armed, two-way
rendezvous via `.reached`/`.release`). It is added to `runner-dispatch.sh` in that same shape, not
invented afresh.

**4. Cosmetic drift.** §1 says "seven libs today"; `scripts/lib/` holds eight since 0208 added
`docket-agent-scope.sh`. No design consequence.

**Scope confirmed unchanged against the three `related:` changes.** 0277 moved delegated briefs to
a `--brief-file` channel and raised `tests/test_runner_dispatch.sh`'s budget row to 20s — neither
touches the observe leg. 0270's runner-config locality fence is input-side. 0208's gate 3b is
input-side and is explicitly preserved, including its `ANCHOR_FALLBACK != 1` condition (see 2).
`depends_on:` stays empty. Additionally noted: 0286 (`fc482699`) fixed caller-authored
`gate-run --observe` poll loops and taught the executable-fence oracle technique in `gate-run.md`;
`runner-dispatch.md` carries **no** equivalent caller-loop fence (its caller loop is emitted by
`sync-agents.sh`'s `emit_shim`), so that technique has no target here and is out of scope. 0286's
*other* lesson — a guard can be vacuous because the document it reads discusses the literal the
guard greps for — **is** adopted: `runner-dispatch.md` will now discuss `cause=child-vanished`, so
any contract guard keyed on it matches comment-stripped lines.

**Auto-capture:** nothing independently valuable surfaced this pass; every finding above is
in-scope work on this change's own diff.
