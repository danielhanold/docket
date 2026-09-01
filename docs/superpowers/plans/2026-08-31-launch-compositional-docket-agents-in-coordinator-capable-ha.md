<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0384 — Launch compositional Docket agents in coordinator-capable harness contexts](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-01-0384-launch-compositional-docket-agents-in-coordinator-capable-ha.md)**
<!-- docket:backlink:end -->
# Launch Compositional Docket Agents in Coordinator-Capable Harness Contexts — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove, then encode, the native Codex invocation that starts a registered Docket agent in a coordinator-capable context, so a root → coordinator → named-leaf composition completes with a consumed sentinel on both supported entry paths in a fresh Codex process.

**Architecture:** A disposable two-role Codex fixture proves the launch primitive first; only a fixture-proven mechanism is then encoded at the narrowest harness boundary (`internal/harness/codex` for wrapper properties, the Codex-side dispatch surface for parent invocation shapes), with the caller skills and `agents/docket-*.md` role bodies staying harness-neutral. Live fresh-process certification — not generator tests — is the acceptance evidence.

**Tech Stack:** Go (internal/harness adapter layer, golden tests), Codex CLI 0.151.0 (`multi_agent = true`, shared app-server daemon, native TOML agents under `~/.codex/agents/`), the repo suite gate `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-design.md` (synchronized metadata copy; the spec is the authority — this plan encodes its ordering constraint and its evidence bar).

## Global Constraints

- **HARD ORDERING (spec Design §1):** no production launch mechanism may be selected or encoded until the disposable two-role fixture proves it — the grandchild actually starts and the coordinator consumes its exact sentinel. Inspecting a schema or a tool list is NOT success evidence. Tasks 3–5 are gated on Task 2's decision record.
- **Stop-and-report, never fallback:** if no accessible native path preserves coordination, record the finding and take the Blocked Path (after Task 2) — a parent relay, generic-agent substitute, shell runner, subprocess Codex session, or cross-harness fallback is out of scope and must not be silently admitted.
- **Acceptance evidence is live:** a fresh-process Codex certification (root → coordinator → named leaf → unique sentinel consumed) across BOTH entry paths — (A) repository managed-dispatch surface, (B) direct registered-agent invocation. Automated Go tests cannot substitute (spec §6). The change is not `done` without it; a review PR with a clearly-reported external limitation is permitted.
- **Capability-evidence rule (ADR-0059; learnings `capability-absence-needs-a-failed-attempt`):** only a failed attempted dispatch or an explicit policy denial establishes unavailability. Never treat a missing tool NAME or an unobserved result as absence; never make a specific tool spelling load-bearing in recorded prose.
- **Version scoping (learnings `harness-behavior-is-mode-and-version-scoped`):** every probe result and the certification are scoped to Codex CLI 0.151.0 with `multi_agent = true` on this machine's shared app-server; record version + configuration with every observation. Do not claim other versions behave the same.
- **Fresh process (learnings `generated-artifact-loaded-at-process-start`):** Codex loads agent definitions at process start. Every probe or certification run after regenerating wrappers requires a genuinely fresh Codex process; a new conversation in a running process certifies nothing.
- **Boundary discipline (spec §2):** workflow edges stay in caller skills, role behavior stays in `agents/docket-*.md`, native launch mechanics live only in `internal/harness/codex` and its generated surfaces. No handwritten agent-by-harness dispatch matrix.
- **Byte identity:** claude, cursor, and opencode adapter outputs must remain byte-identical (their `testdata/golden` trees unchanged); any intentional change to a shared parent surface needs focused ownership/coexistence tests for every harness reading that file.
- **Keep change-0365 negative-evidence coverage:** `TestCodexNestedDispatchBoundary` (internal/harness/codex/codex_test.go) — its claim that nested-inventory absence cannot prove dispatch unavailability must survive (updated to new spelling if the boundary prose changes, never deleted).
- **ADR posture:** ADR-0036 (repo-owned machine-neutral dispatch block), ADR-0059, ADR-0060 (wrapper conforms to target-harness contract), ADR-0094 (pinned plan-writer) are in force. If the proven launch needs Codex-specific parent syntax that ADR-0036's shared byte-identical block cannot express, flag `adr_required: yes` in the decision record with the narrow superseding decision drafted — never smuggle vendor syntax into the shared interior. The workflow records the ADR through the docket-adr contract; the build's job is to flag and draft it in the decision record and keep vendor syntax out of shared prose.
- **Mutation probes defeat the cache:** every Go mutation probe and manual re-verification runs `go test -count=1 …` (learnings `cached-runner-serves-a-mutated-tree`). Golden regeneration uses `-update` deliberately and only for the codex package.
- **Full gate:** `go run ./cmd/docket development test` from the feature worktree, whole suite, budget clause lines inspected (`SERIAL CONFIRMED OVER BUDGET:` is an authoritative breach).
- **Out of scope:** changing composition topology, payloads, return protocols, model/effort pins, worktree scopes, Tier assignments, `auto` authorization; reworking change-0359 run-gate state; touching halted change 0364 (it is resumed only after this fix merges and installs — post-merge, not in this branch).

## File Structure

- `docs/codex/fixtures/nested-launch/` — NEW. The disposable-but-committed prototype-regression fixture: two synthetic role definitions, a probe procedure, probe logs, the decision record, and the certification record. Committed because the spec's Testing section requires a deterministic fixture that reproduces the old invocation and validates the new one.
  - `README.md` — what the fixture is, how to stage it, sentinel definitions, restart discipline.
  - `probe-coordinator.toml`, `probe-leaf.toml` — the two synthetic Codex agents.
  - `probe-log.md` — dated, version-stamped observations (failed-current first, then candidates).
  - `decision.md` — the machine-checkable gate for Tasks 3–5.
  - `certification.md` — the completed live-certification record (Task 6) the change's results file will carry forward.
- `internal/harness/codex/codex.go` — MODIFIED (Task 3, gated): encode the proven launch (wrapper setting, boundary-prose correction, or entry-operation emission — per `decision.md`).
- `internal/harness/codex/codex_test.go`, `internal/harness/codex/testdata/golden/` — MODIFIED (Tasks 3, 5).
- `internal/harness/inventory.go`, `internal/harness/inventory_test.go`, `agents/docket-*.md` — MODIFIED ONLY IF Task 4 fires (launch-posture metadata; skipped under a universal rule).
- `docs/codex/setup.md`, `docs/codex/validation-runbook.md`, `skills/docket-convention/SKILL.md` — MODIFIED (Task 7): proven mechanics, reworked Phase 7 fixture, three-boundary separation prose.

Everything in this plan happens in the feature worktree on branch `fix/launch-compositional-docket-agents-in-coordinator-capable-ha`. All Codex probes run against a scratch fixture repo (create under `${TMPDIR:-/tmp}/codex-nested-probe.XXXXXX` via `mktemp -d` with a template), never against the live docket backlog.

---

### Task 1: Disposable two-role fixture + failed-current baseline

Reproduce the observed failure with the CURRENT launch shape, in a synthetic fixture with none of Docket's role prose, on both entry paths. This is the spec's "failed-current" half of the comparison and the behavioral baseline the Task 6 mutation check re-derives.

**Files:**
- Create: `docs/codex/fixtures/nested-launch/README.md`
- Create: `docs/codex/fixtures/nested-launch/probe-coordinator.toml`
- Create: `docs/codex/fixtures/nested-launch/probe-leaf.toml`
- Create: `docs/codex/fixtures/nested-launch/probe-log.md`

Suggested build profile: premium (live multi-process work; observations must be recorded with discipline, but no irreversible decisions).

**Interfaces:**
- Produces: the two synthetic role definitions and `probe-log.md` §"Failed-current baseline" that Task 2 extends and Task 6's mutation check must reproduce. Sentinel format (Task 2/6 depend on these exact spellings): leaf returns `LEAF_SENTINEL=<uuid>`; coordinator returns `COORDINATOR_CONSUMED=<same uuid>` where `<uuid>` is minted fresh per run (`uuidgen`), so a pass can never be replayed from a stale transcript.

- [ ] **Step 1: Write the two synthetic agents.**

`probe-leaf.toml`:

```toml
name = "probe-leaf"
description = "Synthetic leaf for the docket nested-launch fixture: returns the sentinel it is given and touches nothing."
developer_instructions = """
You are probe-leaf. Your task message contains a line `SENTINEL=<value>`.
Do not read or write any file, run any command, or start any agent.
Reply with exactly one line: LEAF_SENTINEL=<value>
"""
```

`probe-coordinator.toml`:

```toml
name = "probe-coordinator"
description = "Synthetic coordinator for the docket nested-launch fixture: starts probe-leaf in the foreground and relays its sentinel."
developer_instructions = """
You are probe-coordinator. Your task message contains a line `SENTINEL=<value>`.
Your only operation: start the registered agent named probe-leaf as a foreground
child with the task message `SENTINEL=<value>`, block for its return, and then
reply with exactly one line: COORDINATOR_CONSUMED=<value-from-the-leaf-reply>
If you cannot start probe-leaf, reply with exactly one line:
COORDINATOR_BLOCKED=<verbatim error or denial you received from an ATTEMPTED start>
Never answer COORDINATOR_BLOCKED from a tool listing alone — attempt the start first.
Do not read or write any file and do not run shell commands.
"""
```

Note the coordinator's own contract already enforces ADR-0059: `COORDINATOR_BLOCKED` requires an attempted start's error, and the prohibition names which return it maps to (learnings `prohibition-needs-a-return-value`).

- [ ] **Step 2: Write `README.md`** covering: purpose (prototype regression for change 0384), how to install the two TOMLs (`cp probe-*.toml ~/.codex/agents/` — note they sit beside, and must never overwrite, the generated `docket-*.toml` wrappers), the fresh-process restart requirement (link `docs/codex/setup.md` "Restart after (re)generating"), how to stage a scratch fixture repo containing an `AGENTS.md` whose dispatch block routes a prose request to `probe-coordinator` (copy the managed-dispatch block shape the Codex install renders — same heading `## Docket agents — dispatch, don't run inline` semantics, with the probe roles named for the fixture), the sentinel protocol from Interfaces above, teardown (`rm ~/.codex/agents/probe-*.toml`, remove the scratch repo), and the scope stamp: every recorded observation carries `codex --version` output and the `multi_agent` setting.

- [ ] **Step 3: Reproduce the failure — entry path B (direct registered-agent invocation).** Install the TOMLs, start a fresh Codex process, invoke `probe-coordinator` through Codex's supported direct agent entry surface (the same surface the 0364 run used to start `docket-implement-next`) with `SENTINEL=<fresh uuid>`. Expected under the current launch: `COORDINATOR_BLOCKED=<...>` carrying a real attempted-start error, or an equivalent observed inability — the grandchild does not start. Record verbatim: the exact invocation used, whether the coordinator's top-level surface contained a collaboration control, the attempted start and its rejection text, and the final line.

- [ ] **Step 4: Reproduce the failure — entry path A (repository managed-dispatch surface).** In the scratch fixture repo, in a fresh root Codex session, issue the plain-prose request the dispatch block routes to `probe-coordinator`, same sentinel protocol. Record the same fields. If path A actually PASSES under the current launch (root-level routing may keep the coordinator at the top level), that is a finding, not a failure — record it precisely; the defect may be scoped to path B's entry shape, which narrows Task 2.

- [ ] **Step 5: Write `probe-log.md` §"Failed-current baseline"** with the Codex version line, both entry-path records, and the explicit statement of what the failure IS (coordinator entered without a working named-child start) and what it is NOT (an inference from a tool list). One observation rule at the top of the file: a candidate counts only when the grandchild starts and the sentinel round-trips; schema/tool-list inspection is never success evidence.

- [ ] **Step 6: Commit.**

```bash
git add docs/codex/fixtures/nested-launch/
git commit -m "test(0384): disposable two-role Codex fixture reproduces the coordinator-to-child launch failure"
```

---

### Task 2: Native coordinator-launch investigation → decision record (GATE)

Find the native launch that gives the registered coordinator working named-child dispatch, on this exact Codex build. This task's output — `decision.md` — is the gate every encoding task consumes. Do not write a line of production Go in this task.

**Files:**
- Modify: `docs/codex/fixtures/nested-launch/probe-log.md`
- Create: `docs/codex/fixtures/nested-launch/decision.md`

Suggested build profile: max (this is the change's unresolved-architecture moment: what is proven here decides everything downstream, and a wrong "proof" here poisons the branch).

**Interfaces:**
- Consumes: Task 1's fixture, sentinels, and failed-current baseline.
- Produces: `decision.md` with machine-checkable frontmatter (exact keys, closed vocabularies — Tasks 3–5 branch on these):

```markdown
---
mechanism: wrapper-setting | parent-invocation | role-entry | universal | blocked
needs_role_distinction: true | false
adr_required: yes | no
codex_version: "0.151.0"
---
```

  plus body sections: `## Proven invocation` (the exact native shape that passed, verbatim), `## Fixture evidence` (both entry paths, sentinel values, dates), `## Rejected candidates` (each with its failed attempt or denial), and — when `adr_required: yes` — `## Draft ADR` (the narrow superseding decision over ADR-0036, drafted for the workflow's docket-adr step). `mechanism` values map to spec §3's three encoding boundaries; `universal` means every registered agent can safely be coordinator-capable with no role classification (spec §4's preferred outcome); `blocked` routes the branch to the Blocked Path.

- [ ] **Step 1: Enumerate the accessible native surfaces** on Codex CLI 0.151.0, from its own installed material — `codex --help`, `codex agents --help`, the config schema `~/.codex/config.toml` accepts, the agent-TOML keys the local build documents, and the app-server surface the `codex agents` daemon exposes. Record the enumeration in `probe-log.md` with its evidence grade (executed run vs. `--help` text — the two are different grades; say which). This step produces CANDIDATES only, never conclusions.

- [ ] **Step 2: Probe candidates against the fixture, one at a time,** fresh process per probe, new uuid per run, both entry paths for any candidate that passes one. For each candidate record in `probe-log.md`: exact invocation/config change, entry path, whether the coordinator's active top-level surface held the collaboration control, whether `probe-leaf` actually started, and the final sentinel line. A candidate passes ONLY on `COORDINATOR_CONSUMED=<uuid>` with the run's own uuid. Probe order (cheapest real attempt first — each is an ATTEMPT, not a schema read): (a) a wrapper-TOML property on the coordinator's own definition, if the schema enumeration surfaced one; (b) a parent-invocation variant — flags/modes on the direct agent entry surface; (c) a root-session role-entry operation — the root adopting the registered role while keeping its own top-level surface; (d) any app-server-level configuration governing nesting depth or agent capability. Where the enumeration surfaced nothing for a category, one trivial attempt of the most plausible spelling is still the evidence standard for recording that category as unavailable — never the enumeration's silence alone.

- [ ] **Step 3: Verify the winner preserves the wrapper contract.** For the selected mechanism, confirm in the passing transcript that the coordinator ran AS the registered definition — its `developer_instructions` were in force (the fixture coordinator's constrained reply format is itself the probe: a coordinator answering in the exact `COORDINATOR_CONSUMED=` shape is running the wrapper's instructions). For the production encoding this must extend to model/effort pins, skill preload, and recursion guard (spec §3's "bypassing the wrapper is not equivalent"); record how the mechanism composes with those wrapper fields.

- [ ] **Step 4: Write `decision.md`** in the Interfaces shape. Choose `universal` over `needs_role_distinction: true` whenever the proven mechanism safely applies to every registered agent (spec §4 prefers the simpler rule). Set `adr_required: yes` only if the proven parent-side syntax cannot live inside ADR-0036's machine-neutral shared block and must instead be Codex-specific parent prose or structure; draft the narrow ADR in the body.

- [ ] **Step 5: Commit.**

```bash
git add docs/codex/fixtures/nested-launch/
git commit -m "feat(0384): prove the native Codex coordinator launch in the fixture and record the decision"
```

- [ ] **Step 6: Route.** If `mechanism: blocked` → jump to **Blocked Path** (below, after Task 8). Otherwise continue to Task 3, carrying `mechanism` and `needs_role_distinction` forward.

---

### Task 3: Encode the proven launch at the narrowest harness boundary (gated on Task 2)

Exactly one of the three encodings below, selected by `decision.md` `mechanism:`. All three share the same TDD shape: a failing inventory-derived test in `internal/harness/codex/codex_test.go` first, the render change second, a deliberate golden regeneration third. In every branch the shared `agents/docket-*.md` bodies and `internal/harness/dispatch.go`'s machine-neutral `dispatchPreamble` interior are untouched unless the branch explicitly says otherwise.

**Files:**
- Modify: `internal/harness/codex/codex.go`
- Modify: `internal/harness/codex/codex_test.go`
- Modify: `internal/harness/codex/testdata/golden/*` (regenerated)

Suggested build profile: premium (consequential but correctable; the mechanism is already proven).

**Interfaces:**
- Consumes: `decision.md` (`mechanism`, `## Proven invocation` verbatim shape).
- Produces: a `Plan()` output whose codex agent targets carry the proven launch; Task 5 hardens the guards, Task 6 certifies the installed result. If the boundary paragraph's prose changes, the new spelling becomes the one `TestCodexNestedDispatchBoundary`'s clause list pins (Task 5 updates the clauses; the negative-evidence CLAIM — nested-inventory absence proves nothing — must survive verbatim in meaning).

- [ ] **Step 1 (branch A — `mechanism: wrapper-setting` or `universal` via a wrapper key): write the failing test.** Model it on the existing `TestCodexNestedDispatchBoundary` pattern — plan from the real embedded inventory, decode every rendered `*.toml` agent target, and assert each carries the proven TOML key at top level (spec: coverage derives from the inventory, no hand-listed paths). Concrete shape (substitute the proven key/value from `decision.md` for the illustrative `allow_subagents = true`):

```go
// TestCodexCoordinatorLaunchSetting: every generated wrapper carries the
// fixture-proven coordinator-capability setting (change 0384, decision record
// docs/codex/fixtures/nested-launch/decision.md). Emitted unconditionally for
// the same reason as the dispatch boundary: composition is a property of the
// invoked skill and may change through configuration.
func TestCodexCoordinatorLaunchSetting(t *testing.T) {
	targets := planFromEmbedded(t) // reuse this file's existing plan helper
	saw := 0
	for _, tgt := range targets {
		if tgt.Role != "agent" {
			continue
		}
		saw++
		if !strings.Contains(string(tgt.Content), "allow_subagents = true") {
			t.Errorf("%s: rendered wrapper missing the proven coordinator-launch setting", tgt.Path)
		}
	}
	if saw < 5 {
		t.Fatalf("sampling floor: expected at least 5 agent targets, saw %d", saw)
	}
}
```

  Under `universal`, emit for every agent; under `wrapper-setting` with `needs_role_distinction: true`, Task 4's posture field decides which wrappers get it and this test splits into coordinator-carries / leaf-omits halves keyed on the parsed posture (Task 4 defines the field; write the split against it).

- [ ] **Step 1 (branch B — `mechanism: parent-invocation`): write the failing test** against the surface that renders the parent-side invocation. The proven entry shape is Codex-specific, so it must NOT enter `dispatchPreamble` (shared, machine-neutral, ADR-0036); encode it as a Codex-only addition rendered by the codex adapter's own material — e.g. a Codex-appended paragraph or key emitted alongside/into the wrapper TOMLs, or a documented-entry correction, per `## Proven invocation`. The test asserts the codex-rendered surface carries the proven shape and — coexistence — that claude/cursor/opencode outputs are byte-unchanged (Task 5 Step 3 pins this globally; here assert the shared-constant interior `harness.DispatchHeading`/preamble bytes are untouched). `adr_required: yes` is expected on this branch; confirm `decision.md` drafted the ADR.

- [ ] **Step 1 (branch C — `mechanism: role-entry`): write the failing test** asserting the codex adapter emits the proven role-entry operation explicitly (a rendered instruction/definition telling the root how to assume the registered role with its wrapper contract intact — model, effort, preload, recursion guard). Assert the emitted operation names the registered agent, not a workflow reconstruction, and contains no generic-agent substitute.

- [ ] **Step 2: Run the new test, verify it fails** for the right reason:

```bash
go test -count=1 ./internal/harness/codex/ -run TestCodexCoordinatorLaunch -v
```

Expected: FAIL with the "missing the proven coordinator-launch setting" (or branch-equivalent) message — not a compile error.

- [ ] **Step 3: Implement minimally in `renderAgent`/`Plan`** (branch A: write the key in `renderAgent` beside `name`/`description`; branches B/C: emit at the boundary the branch names). If `decision.md`'s proven invocation contradicts any sentence of the existing `codexDispatchBoundary` constant, correct THAT constant's wording here to match proven reality — keeping its negative-evidence sentence ("absence from such a nested inventory cannot establish dispatch unavailability; only a failed direct dispatch attempt or an explicit policy denial does") intact in meaning. Do not document any unproven spelling (spec Documentation rule).

- [ ] **Step 4: Regenerate the codex goldens deliberately, then verify.**

```bash
go test -count=1 ./internal/harness/codex/ -update
go test -count=1 ./internal/harness/... ./internal/... 2>&1 | tail -20
git diff --stat internal/harness/
```

Expected: PASS; the diff touches ONLY `internal/harness/codex/` (plus Task 4 files if that branch is live). Any diff under `internal/harness/claude|cursor|opencode` is a defect — stop and fix before committing.

- [ ] **Step 5: Commit.**

```bash
git add internal/harness/codex/
git commit -m "feat(0384): encode the fixture-proven Codex coordinator launch in the codex renderer"
```

---

### Task 4: Launch-posture metadata (CONDITIONAL — only if `needs_role_distinction: true`)

Skip this task entirely — and record "skipped: universal rule" in the build evidence — when `decision.md` says `needs_role_distinction: false`. The spec prefers the universal rule.

**Files:**
- Modify: `internal/harness/inventory.go` (parse + validate the field)
- Modify: `internal/harness/inventory_test.go`
- Modify: `agents/docket-*.md` (frontmatter field on coordinator roles)
- Modify: `internal/harness/codex/codex.go` (consume the field)

Suggested build profile: premium.

**Interfaces:**
- Consumes: Task 3's branch-A split test shape.
- Produces: `AgentSource.LaunchPosture string` (values `"coordinator"`, `"leaf"`; frontmatter key `launch-posture:`; absent defaults to `"leaf"`), and a parse error for any other value. Task 5's correspondence guard iterates it.

- [ ] **Step 1: Derive the coordinator population from a whole-repo scan — never a hand-list.** Grep the active dispatch-owning contracts: `grep -rlE "dispatch|Agent tool|subagent" agents/*.md skills/*/SKILL.md`, then sort hits into prose-only vs. genuinely dispatch-owning by reading each: the known dispatch owners are implement-next (status, plan-writer, adr, build, review edges), build (profile workers), auto-groom (critic), finalize (resolver, integration-repair), brainstorm (consultant) — plus any role whose configurable `skills:` binding can resolve to a skill whose own contract dispatches (the spec calls this out: a configurable binding makes the ROLE dispatch-owning). Record the scan command and the sorted result in the task's commit message body.

- [ ] **Step 2: Failing tests.** In `inventory_test.go`: (a) parsing a synthetic agent source with `launch-posture: coordinator` yields `LaunchPosture == "coordinator"`; absent yields `"leaf"`; (b) `launch-posture: orchestrator` (unknown value) is a parse ERROR naming the closed vocabulary — extend `TestParseInventoryRejects`'s table. (c) The correspondence guard, run over the real embedded inventory, in BOTH directions (learnings `correspondence-guard-runs-one-way`): forward — every agent whose BODY matches the dispatch-owning shape (derive the predicate from a shape grep of the parsed `Body`, e.g. containing "dispatch" as an operation it performs, refined from Step 1's reading — anchor on the consuming reality, not an allowlist) is declared `coordinator`; reverse — every declared `coordinator` has a matching dispatch-owning body. Mutation-test both directions before trusting them: flip one real coordinator's frontmatter to leaf → forward loop reddens; add posture to a synthetic leaf fixture → reverse loop reddens (`-count=1`).

- [ ] **Step 3: Implement** — add `LaunchPosture` to `agentFrontmatter`/`AgentSource` in `inventory.go` with the closed-vocabulary validation in `parseAgentSource`; stamp `launch-posture: coordinator` into the Step-1 population's frontmatter; make `renderAgent` key its Task-3 emission on it. Unaffected harnesses ignore the field — verify their goldens are byte-unchanged (`git diff` empty outside codex + agents + inventory).

- [ ] **Step 4: Run and commit.**

```bash
go test -count=1 ./internal/harness/... && git add internal/harness/ agents/ && git commit -m "feat(0384): closed launch-posture field with two-way correspondence guard"
```

---

### Task 5: Guard hardening — byte identity, 0365 retention, mutation matrix (gated on Task 3)

**Files:**
- Modify: `internal/harness/codex/codex_test.go`
- Verify (no edit expected): `internal/harness/cross_harness_test.go`, `internal/harness/native_dispatch_test.go`, `internal/harness/claude|cursor|opencode/testdata/golden/`

Suggested build profile: standard.

**Interfaces:**
- Consumes: Task 3's test and constant names.
- Produces: the guard set Task 8's gate run exercises; the mutation ledger recorded in the build evidence.

- [ ] **Step 1: Reconcile `TestCodexNestedDispatchBoundary` with the new reality.** If Task 3 reworded `codexDispatchBoundary`, update the test's clause list to the new spelling while preserving the negative-evidence clauses' MEANING (nested-inventory absence proves nothing; only failed attempt/policy denial does). The test must keep failing when the boundary paragraph is deleted from `renderAgent` — that is the 0365 retention requirement.

- [ ] **Step 2: Mutation matrix — run each, watch the named assert redden, restore, re-verify green. Restore via a backup copy of your edited file, never `git checkout --` over uncommitted work (learnings `mutation-restore-needs-a-backup-copy`).** All runs `go test -count=1 ./internal/harness/...`:
  1. Delete Task 3's launch emission from `renderAgent` → `TestCodexCoordinatorLaunchSetting` (or branch-equivalent) reddens. If it stays green, the guard is decoration — a defect until explained.
  2. Delete the `codexDispatchBoundary` emission → `TestCodexNestedDispatchBoundary` reddens.
  3. Hand-corrupt one byte of a codex golden → `TestCodexGoldenAgents` reddens (proves the regenerated goldens are actually tied).
  4. If Task 4 fired: both correspondence directions (already proven there; re-run both here post-integration).
  Record each mutation, the reddening assert, and the restore in the build evidence.

- [ ] **Step 3: Byte-identity check for unaffected harnesses.**

```bash
go test -count=1 ./internal/harness/... && git diff --stat main...HEAD -- internal/harness/claude internal/harness/cursor internal/harness/opencode
```

Expected: tests PASS and the diff is EMPTY. `TestAdapterRenderByteStable` and `TestNoCrossHarnessDelegation` must be green unmodified — if either needed editing, that is a scope violation to justify explicitly or revert.

- [ ] **Step 4: Commit** (test-file changes from Steps 1–2):

```bash
git add internal/harness/codex/codex_test.go
git commit -m "test(0384): retention + mutation-proven guards for the coordinator launch"
```

---

### Task 6: Live fresh-process certification matrix + prototype regression mutation (gated on Task 3)

The acceptance evidence. Nothing in Tasks 3–5 substitutes for this.

**Files:**
- Create: `docs/codex/fixtures/nested-launch/certification.md`
- Modify: `docs/codex/fixtures/nested-launch/probe-log.md` (fixed-new + mutation entries)

Suggested build profile: max (the run that decides whether the change can ever be `done`; misreported evidence here is not walk-back-able).

**Interfaces:**
- Consumes: Task 1's fixture + sentinel protocol; the branch's built state.
- Produces: `certification.md` — the completed certification section the change's results record must carry (spec §6): Codex version, both entry paths, selected launch shape, coordinator sentinel, child sentinel, failed-current/fixed-new comparison, and the mutation check.

- [ ] **Step 1: Install the branch and restart.**

```bash
docket development install --source /Users/homer/dev/docket/.worktrees/launch-compositional-docket-agents-in-coordinator-capable-ha
```

Verify a generated wrapper on disk carries the Task-3 emission (read `~/.codex/agents/docket-plan-writer.toml`). Then start a FRESH Codex process. Reinstall the fixture probe TOMLs updated to the proven launch shape if the mechanism is per-wrapper (mirror what the generator now emits into `probe-coordinator.toml`/`probe-leaf.toml`, and commit that update).

- [ ] **Step 2: Fixed-new fixture matrix — both entry paths.** Fresh uuid each: (A) prose request through the scratch repo's managed dispatch surface → `probe-coordinator` → `probe-leaf`; (B) direct registered-agent invocation of `probe-coordinator`. PASS = `COORDINATOR_CONSUMED=<run's own uuid>` in both. Record verbatim invocations, sentinels, and `codex --version` in `certification.md`.

- [ ] **Step 3: Mutation check (spec Testing "Mutation requirement").** Force the generator/routing surface back to the old launch shape: create a scratch copy of `internal/harness/codex/codex.go`, remove the Task-3 emission, reinstall from the worktree, fresh process, rerun entry path B with a fresh uuid. Expected: the nested sentinel check FAILS at the coordinator-to-child edge (`COORDINATOR_BLOCKED=` with a real attempted-start rejection, matching Task 1's baseline). Restore `codex.go` from the scratch copy (never `git checkout --` if any uncommitted work exists), reinstall, fresh process, rerun → PASS again. Record both runs in `probe-log.md` and summarize the comparison in `certification.md`.

- [ ] **Step 4: One real composition edge, without mutating the active backlog.** In a scratch fixture repo (never the live docket metadata), exercise one genuine Docket edge through the fixed launch — e.g. a prose "refresh the docket board" routed to the registered `docket-status` agent in a docket-initialized scratch repo, or a staged fixture change driven far enough for `docket-implement-next` to dispatch `docket-plan-writer` and consume a return. The bar is spec §5's: a named registered child starts and its return is consumed. Change 0364 itself is NOT resumed on this branch (post-merge only) — state that explicitly in `certification.md`.

- [ ] **Step 5: Write `certification.md`** with every spec-§6 field, each entry either COMPLETED with evidence or explicitly NOT-RUN with the reason — no unchecked checklist items presented as done. Note the standing rule: the change is not finalized `done` without the successful nested sentinel; the results file must carry this section forward.

- [ ] **Step 6: Commit.**

```bash
git add docs/codex/fixtures/nested-launch/
git commit -m "test(0384): live fresh-process certification — both entry paths, sentinel consumed, mutation reddens"
```

---

### Task 7: Documentation at the three boundaries

**Files:**
- Modify: `docs/codex/setup.md` ("Two invocation paths — one contract" + a nested-launch mechanics passage)
- Modify: `docs/codex/validation-runbook.md` (Phase 7)
- Modify: `skills/docket-convention/SKILL.md` ("Dispatch-capability resolution" section)

Suggested build profile: standard.

**Interfaces:**
- Consumes: `decision.md` (the only source of native spellings), `certification.md`.
- Produces: docs the next operator follows; guarded by the suite's existing doc-shape tests (fix any that redden at the gate by reconciling meaning, never by weakening the guard).

- [ ] **Step 1: `docs/codex/setup.md`** — extend "Two invocation paths — one contract" with the PROVEN entry and nested-launch mechanics from `decision.md`, verbatim spellings only for what the fixture passed (spec: never document an unproved `@agent`/fork/spawn/tool spelling as supported). Keep the restart section's authority; scope claims to Codex 0.151.0.

- [ ] **Step 2: `docs/codex/validation-runbook.md` Phase 7** — rework it around the committed two-role fixture: point at `docs/codex/fixtures/nested-launch/README.md`, name the expected sentinels, version capture, fresh-process restart, and the failed-current/fixed-new comparison as the recorded shape. Keep the negative-evidence discipline item verbatim in meaning.

- [ ] **Step 3: `skills/docket-convention/SKILL.md`** — in/beside the "Dispatch-capability resolution" section, add a compact statement of the three-boundary separation (workflow edge in the caller skill; role behavior in `agents/docket-*.md`; harness launch in the adapter and its generated surfaces), harness-neutral — no Codex spellings here. This is the "agent-layer reference" home the spec's Documentation section names.

- [ ] **Step 4: Commit.**

```bash
git add docs/codex/setup.md docs/codex/validation-runbook.md skills/docket-convention/SKILL.md
git commit -m "docs(0384): proven Codex launch mechanics, fixture-based Phase 7, boundary separation"
```

---

### Task 8: Full suite gate

Suggested build profile: standard.

- [ ] **Step 1:** From the feature worktree run `go run ./cmd/docket development test`. Whole suite, never a subset.
- [ ] **Step 2:** Inspect the budget clause lines: `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` are screening findings to note in build evidence; any `SERIAL CONFIRMED OVER BUDGET:` is an authoritative breach to act on. Nothing else surfaces them.
- [ ] **Step 3:** Fix any red honestly (root-cause, never weaken a guard), rerun to green, commit fixes with focused messages.

---

## Blocked Path (only when Task 2 records `mechanism: blocked`)

The spec permits a review PR that clearly reports an external limitation; the change is not `done` without the live sentinel. Land a branch that is honest about that:

- [ ] **B1.** Ensure `decision.md` records every candidate's ATTEMPTED failure or explicit policy denial verbatim (ADR-0059 grade — an enumeration's silence blocks nothing), scoped to Codex 0.151.0 / `multi_agent = true`, and states what a future Codex build would need to expose. Commit.
- [ ] **B2.** Skip Tasks 3–5 (no production encoding without proof; no relay/fallback — spec forbids silently admitting one). Verify the change-0365 surface is untouched: `TestCodexNestedDispatchBoundary` green, `git diff main...HEAD -- internal/` empty.
- [ ] **B3.** Run Task 7 Step 2 only, adding to the runbook a clearly-marked "known external limitation (change 0384)" note pointing at `decision.md` — no unproven mechanics documented. Run Task 8 (full gate).
- [ ] **B4.** Write `docs/codex/fixtures/nested-launch/certification.md` as an explicitly INCOMPLETE certification: failed-current baseline recorded, fixed-new impossible, limitation named — so the results record and PR report the exact unmet acceptance conjunct instead of success-shaped prose.

---

## Self-Review (performed against the spec)

- **Ordering constraint (§1):** Tasks 3–5 are gated on Task 2's `decision.md`; Task 2's pass bar is the started grandchild + consumed sentinel; the Blocked Path forbids relay/fallback. Covered.
- **Three sources of truth (§2):** Global Constraints boundary-discipline line; Task 3 keeps `dispatchPreamble` and agent bodies untouched; Task 7 Step 3 writes the separation prose. Covered.
- **Narrowest boundary (§3):** Task 3's three branches map one-to-one onto the spec's three cases, with wrapper-contract retention checked at Task 2 Step 3. Covered.
- **Metadata only if required (§4):** Task 4 is conditional, population is scan-derived, correspondence guard is two-way and mutation-proven, universal rule preferred. Covered.
- **Native + foreground (§5):** no new dispatch machinery anywhere; Task 6 Step 4 exercises an existing edge. Covered.
- **Live certification as evidence (§6) + Testing sections:** Task 1 (failed-current), Task 6 (fixed-new both paths + mutation), Task 5 (generator tests inventory-derived, byte identity, 0365 retention), Task 8 (full gate). Covered.
- **Type/name consistency:** `LaunchPosture`/`launch-posture` (Task 4) match between inventory and codex-test usage; sentinel spellings `LEAF_SENTINEL=`/`COORDINATOR_CONSUMED=`/`COORDINATOR_BLOCKED=` are uniform across Tasks 1, 2, 6. Checked.
- **Placeholder scan:** the only intentionally open values are the proven native spellings, which CANNOT be known before Task 2 — each consuming step names `decision.md` as its exact source and provides the concrete surrounding code. No TBDs otherwise. Checked.
