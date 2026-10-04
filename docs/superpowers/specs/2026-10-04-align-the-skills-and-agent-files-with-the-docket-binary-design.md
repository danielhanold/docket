<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0502 — Align the skills and agent files with the docket binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0502-align-the-skills-and-agent-files-with-the-docket-binary.md)**
<!-- docket:backlink:end -->

# Align the skills and agent files with the docket binary — design

## Goal

Agents take their instructions from docket's skills, the skill references, the agent wrappers, and
`AGENTS.md`. Today that text tells them to read configuration values that never arrive, to use
features the binary refuses, to run scripts and health checks that do not exist, and in four places
to do something that fails outright. Because agents follow the text literally, the drift is not
cosmetic: it produces real failures.

The outcome: every in-scope file describes only what the binary does **today**; the four bugs are
fixed; and the living-docs guard that #464 added covers these files too, so the drift cannot
quietly come back. This is the agent-facing half of the alignment #464 did for the human-facing
docs, and #464's rules carry over unchanged.

## Alignment rules (apply to every in-scope file)

1. **Describe only current behaviour.** The binary is the oracle: `internal/config/schema.go` +
   `capability.go` for configuration; `docket capabilities --json`, `docket schema`, and each verb's
   `--help` for operations; the Go code for mechanisms.
2. **Removed or refused things never appear.** A setting the schema marks obsolete, inert,
   inert-companion, deferred, or deferred-active; a retired feature; a deleted command, flag,
   script, or check; and the old name of a renamed term are deleted — no "this used to…", no
   tombstone paragraph, no "not supported" note.
3. **No citations of individual changes or PRs**, including line-wrapped ones (`since change` at a
   line end, `0392` on the next), and no "Go v1" / "deferred from Go v1" history narration. Keep
   the rule a citation carried; drop only the citation.
4. **ADR citations stay only where the ADR's decision is still how docket works.** ADR-0018
   (pluggable skill passthrough) describes a capability the binary defers, so skill prose stops
   citing it. ADR-0022 (consultant-authored brainstorm) stays current as a per-run human request.
5. **Point-in-time records are untouched:** archived changes, results, Accepted ADRs, specs,
   plans, `testdata/` (including `internal/render/testdata/`), `internal/install/legacydata/`.
6. **Skill bodies ship into other repositories** (learning `distributed-body-has-no-local-repo`):
   a sentence true only in docket's own repository stays conditioned on that.

### Terminology

- **"Loop" is reserved for `/loop`.** Every other sense is renamed:
  - fix loop → **fix pass**; `skills/docket-implement-next/references/fix-loop.md` →
    `fix-pass.md`, with every link updated;
  - build loop / build-loop memory → **builds** / **the learnings ledger**;
  - resolver loop → resolver rounds; repair loop → repair attempts; retry / push-retry loop →
    retries; poll / observe loop → polling; drain loop → drain; auto-groom's "loop to step 1" →
    "return to step 1";
  - `skills/docket-build/references/gate-caller-loop.md` → `gate-driver.md` (its title is already
    "Gate driver contract"), with every link updated.
- **One metadata layout.** `metadata_branch`, "docket-mode", and "repo-mode" become **the `docket`
  branch** and **the `.docket/` worktree** (ADR-0099).
- **`auto-or-halt` becomes `halt`.**

## Scope

**In scope:** `skills/**` (all twelve skills and their references, including the two templates
being deleted); `agents/*.md`; `AGENTS.md` outside its generated dispatch block; the Go comments
in `internal/render/record.go` that name a deleted template; `docs/release/four-harness-acceptance.md`
(it receives the run-boundary probe scenarios); the regenerated embedded assets; the guard
extension; and every repo test that pins wording on these files.

**Not in scope:** the human-facing docs (#464); changing what the binary does, including exporting
the values skills used to read or deleting obsolete/refused schema rows; point-in-time records;
generated `CLAUDE.md`/`AGENTS.md` dispatch blocks beyond what their generators produce;
`agents/harness-defaults.yml`.

## Decisions settled at grooming

| Question | Decision |
|---|---|
| How finalize re-gates a repaired head | The gate driver with `--owner finalize` (bug 3 below). The raw gate verbs stay operator-only. |
| In-session plain-language requests once dummy mode is gone | Deleted entirely; no replacement sentence. |
| `gate-execution-evidence.md` | Deleted, together with `gate-execution.md` and docket-build's blocking pointer to it. The run-boundary probe scenarios move to the release acceptance procedure. |
| Guarding against new drift | Extend #464's `TestLivingDocsAlignment` to `skills/`, `agents/`, and `AGENTS.md`. |
| `adr-template.md`, `change-template.md` | Deleted. No skill reads them; the Go renderers are the only writers. |
| Bug count | All four bugs are fixed. Bug 4 is wrong in finalize SKILL.md step 6 as well as `gate-failure.md`. |

## Worklist

The audit behind this spec ran over main @ `20bc0a36a` and was re-verified read-only at main @
`ecbb32f19` (2026-10-04). **Treat each item as a hypothesis to verify at build time**: re-check
it, then fix, delete, or record in the results file why it was not a defect. Line numbers drift; the
named phrase or section is the anchor. After the listed fixes, grep every in-scope file
(whitespace-collapsed, so wrapped phrases match) for further instances of each defect — the audit
counted examples, not every occurrence.

### Facts the rewrite relies on (verify once, use everywhere)

- **Prepare context.** `repository.prepare` returns the repo root, origin URL, the default,
  integration, and metadata branch names with their revisions, the `.docket` worktree path, the
  changes/ADRs/results dirs, `finalize` (`gate`, `test_command`, `require_pr_approval`,
  `resolver_max_attempts`, `repair_max_attempts`), `build` (`gate`, `test_command`,
  `max_attempts`), and an always-empty `skills` object (`internal/app/repository_prepare.go`:
  `PrepareContext`, `buildPrepareContext`). It exports no `SKILL_*`, `DUMMY_MODE_*`,
  `AUTO_CAPTURE_*`, `REVIEW_MIN_FIX_SEVERITY`, `REVIEW_MAX_FIX_TASKS`, `GATE_OBSERVATION_BUDGET`,
  or `BUILD_CHECKPOINT` value. The phrase "startup-check (config) export" goes everywhere.
- **Where the other values live.** `review.min_fix_severity` (default `minor`) and
  `review.max_fix_tasks` (default 10) are supported configuration, read through
  `diagnostic.config`. `gate_observation_budget` is enforced by the gate driver itself; no skill
  reads or passes it.
- **Configuration.** Supported keys: `integration_branch`, `changes_dir`, `adrs_dir`,
  `results_dir`; `finalize.gate` (`local` | `off`), `finalize.test_command`,
  `finalize.require_pr_approval`, `finalize.resolver_max_attempts` (10),
  `finalize.repair_max_attempts` (6); `build.gate` (`local` | `off`), `build.test_command`,
  `build.max_attempts` (4); `run.max_attempts` (2); `review.min_fix_severity`,
  `review.max_fix_tasks`; `reclaim.lease_ttl` (72), `reclaim.auto` (false); `learnings.enabled`
  (true); `gate_observation_budget` (30); `board_surfaces` (`[inline]`); `board.section_order`,
  `board.sorting.<section>.by|direction`; `change_types`; `agent_harnesses`;
  `agents.<harness>.<agent>.model|effort` **from the global config only** (set in `.docket.yml` or
  `.docket.local.yml`, an agent pin blocks writes). Any explicit `skills.*` value blocks writes,
  even one repeating a default (`dispDeferredActive`). Configuration is read from the primary
  worktree's `.docket.yml` and `.docket.local.yml` plus the global file — never from
  `origin/HEAD`. `integration_branch: auto` resolves origin's HEAD; an unresolvable remote HEAD is
  an error, not a fallback to `main`. Only `docket install` downgrades unknown keys to warnings
  (`TolerateUnknownKeys`); every other read treats an unknown key as an error.
- **Auto-groom selection.** Only a per-change `auto_groomable: true` makes a stub auto-groomable.
  `auto_groom` in configuration blocks writes, so there is no repository default to inherit; an
  unset `auto_groomable` means not auto-groomable.
- **Learnings.** `learning.record` and `learning.update` write findings; `learnings.enabled` gates
  reading and writing. There is no cap. Verify what (if anything) refreshes `learnings/README.md`
  and describe only that.
- **Plans.** `change.attach-plan` accepts only paths under `docs/superpowers/plans`
  (`internal/app/change_attach.go`, `plansPlanningRoot`).
- **`## Run halted`.** Written by `change.halt`, removed by `change.resume-halted`
  (`internal/app/change_halt.go`). Verify whether the Step 2 claim also removes it and state only
  what the code does.
- **Agent wrappers.** Installed at user level only; install writes no per-repository agent
  definitions (`internal/reposeed/plan.go`). The repository surfaces install writes are the managed
  dispatch blocks (`CLAUDE.md`, `AGENTS.md`, `.cursor/rules/docket-dispatch.mdc`) for the harnesses
  `agent_harnesses` opts into. `docket install check` is a machine-only report.
- **Findings and checks.** `status` reports configuration diagnostics, `parse-failed`, record
  validation codes, `artifact-missing`, and `branch-malformed`. `repository check` reports the
  repository-setup codes (`board-stale`, `adr-index-stale`, `artifact-links-stale`,
  `docket-worktree-hooks-*`, …). Stale claims surface as `maintenance sweep` reclaim items;
  stalled dependencies surface as the `waiting-dependency` readiness with `unmet_dependencies`.
- **Stacks.** Readiness token `stack-base-unresolved`; validation finding `change-stack-cycle`;
  the effective-base kinds in `internal/domain`. Stack worktrees come from `workspace.prepare`;
  child PRs are retargeted by `finalize.retarget-children`.
- **Hooks.** `repository prepare` turns shared hooks off in the `.docket/` worktree
  (`internal/gitcli`). No script does it.
- **Scripts.** `scripts/` holds only `release-smoke.sh` and its contract. No workflow script or
  `scripts/<name>.md` contract exists for a skill to point at.
- **Run tracker.** `run.verify` and `run.verdict` exist; `verify-run` does not. The halted verdict
  is `run-halted`.

### Roles are fixed defaults

brainstorm `superpowers:brainstorming`, plan `superpowers:writing-plans`, build `docket-build`,
review `docket-review`, finish `superpowers:finishing-a-development-branch`.

- Replace every `$SKILL_*` read with the named skill (convention, implement-next, groom-next,
  new-change, `agents/docket-plan-writer.md`).
- Delete the `auto` sentinel, custom-binding branches, the unvalidated-passthrough prose, the
  `skills:` merge across layers, and every "explicitly configured `auto` authorizes inline" clause
  (convention *Skill layer* and *Dispatch-capability resolution*, docket-build, `fix-loop.md`,
  plan-writer). The dispatch-capability table's third posture becomes **halt**: an undispatchable
  required dispatch leaves the change `in-progress` with the halt reason recorded.
- Keep the missing-skill rule, reworded without `auto`: a role skill that cannot be invoked (for
  example, the superpowers plugin is not installed) is done inline by the running agent, with a
  prominent warning in the run output and, for plan/build/review/finish, in the PR body.
- `docket-brainstorm` (consultant-authored spec) runs only when a human asks for it in that run.
- Keep "a role invokes a skill, never a same-name agent" and the autonomy-precedence rule
  (`DIRECTED to:` directions at the call site) where they still say something true. Delete the role
  self-description rule ("a role skill body names its `skills.<role>` binding key"): it names a
  refused key.

### Deleted outright

- **Dummy mode:** the convention's *Dummy mode* section, `references/dummy-mode.md`, every consumer
  pointer (grep `DUMMY_MODE` and "dummy" — groom-next, new-change, implement-next, finalize,
  status, auto-groom), and finalize's leftover paragraph.
- **Auto-capture:** the `AUTO_CAPTURE_*` exports and the narration of the deferred key. Keep the
  rule that work discovered mid-run goes in the run's final report for a human to capture.
- **Terminal publish:** all narration, and step 3 of `docket-convention/references/close-out.md`.
- **`build.checkpoint: true`** ledger mode (docket-build).
- **`finalize.skip_results_only_delta`** (convention, `edge-paths.md`).
- **`finalize.gate: ci` / `both`** (convention). Finalize's gate is `local` or `off`.
- **Repo-layer `agents:` pins**, per-repository wrapper generation, the Bash validator, the generic
  emitter, and the three-leg drift gate (`references/agent-layer.md`, convention).
- **`learnings.cap`** and the active-findings cap (convention, `references/learnings.md`).
- **The legacy `agents.yaml` auto-migration** (convention, agent-layer).
- **The convention's `.docket.yml` sample** is rewritten from the schema with supported keys only,
  so a verbatim copy never blocks writes.
- **The two templates:** `skills/docket-adr/adr-template.md` and
  `skills/docket-new-change/change-template.md`. The convention's change-manifest and ADR-format
  blocks are corrected to match `render.ChangeRecord` and `render.ADRRecord` (the ADR block gains
  `## Alternatives considered`, which `adr.record` requires; the manifest shows `branch_prefix`
  and only the sections `change.create` writes). The `internal/render/record.go` comments that say
  a renderer mirrors a template are reworded to name the renderer as the authority.
- **The gate-execution references:** `skills/docket-build/references/gate-execution.md` and
  `gate-execution-evidence.md`, plus docket-build's "read it now (blocking) before starting the
  gate" paragraph and `gate-driver.md`'s intro reference to them. The *Run-boundary continuation
  acceptance* material (the pending-rows table, the five probe scenarios, the standing rules) moves
  into `docs/release/four-harness-acceptance.md` as a section of the release procedure, without
  change citations. The measured verdicts and probe history stay in git history and ADR-0081.

### Bash-era names → typed operations

| Stale name (where) | Replace with |
|---|---|
| `stack-base.sh` (convention) | the `stack-base-unresolved` readiness token; the binary resolves the effective base |
| `disable-worktree-hooks.sh` (convention) | `repository prepare` turns hooks off |
| `scripts/<name>.md` contracts (convention, `close-out.md`) | delete; operations are reached through the capability catalog and `docket schema` |
| `verify-run` (convention, `stacked-changes.md`, `edge-paths.md`) | `run.verify` |
| `fm_field` (`stacked-changes.md`) | the record's fields from `status --json` |
| `reclaim-claims` (implement-next) | `change.reclaim` / the maintenance sweep's reclaim |
| `run-halt` (implement-next) | `run-halted` |
| checks `publish-deferred`, `adr-unpublished`, `stack-invalid`, `stack-parent-killed`; tokens `board off`, `promote-failed`, `stack-carried-failed` (convention, `close-out.md`, docket-adr, `stacked-changes.md`, status) | delete; name only findings the binary emits (facts above), `change-stack-cycle` for stacks |
| hand `git worktree add` (`stacked-changes.md`) | `workspace.prepare` |
| hand `gh pr edit` (`stacked-changes.md`) | `finalize.retarget-children` |
| "human curation only" learnings (convention, `learnings.md`, status) | `learning.record` / `learning.update` |

### Wrong facts to correct

- **Convention, configuration:** config read from `origin/HEAD`, `origin/HEAD` "repaired", and
  `auto` falling back to `main` → the facts above.
- **Convention, *Change body sections*:** the `## Run halted` paragraph (stale `verify-run`; who
  removes the section).
- **Plans location:** implement-next's "no directory allowlist" and plan-writer's "a custom skill
  decides the location" → plans live under `docs/superpowers/plans`. `agents/docket-plan-writer.md`
  drops `SKILL_PLAN` and invokes `superpowers:writing-plans`.
- **Agent layer:** rewrite `references/agent-layer.md` around user-level wrappers, the compiled
  built-in model/effort table, global-only overrides, and `agent_harnesses` as the opt-in for a
  repository's dispatch blocks. The convention's "per-repo agent pass generates wrapper files"
  goes. `docket install check` is described as machine-only.
- **docket-status** (SKILL.md and the `agents/docket-status.md` description): drop the stale-claim
  and dependency-stall health checks; say where those states actually surface.

### The four bugs

1. **auto-groom dirties `.docket/`.** It drafts the spec to `.docket/docs/superpowers/specs/…`, so
   the next `repository prepare` refuses with `metadata-worktree-dirty`. Fix: the draft lives in
   session scratch space and reaches `change.groom` as `spec_markdown` in the request file, as in
   groom-next. The contended path's "delete any just-drafted spec markdown" becomes "discard the
   draft".
2. **docket-review raises a false blocker.** Its evidence rule demands `result: green`, so under
   `build.gate: off` it reports `unverified-build-state` against implement-next's accepted
   `skipped` / `build-gate-off` evidence. Fix: when the resolved `build.gate` (prepare's `build`
   context) is `off`, the skipped / `build-gate-off` record is the expected build state, not a
   blocker; under `local` the green requirement stands.
3. **Finalize re-gates a repaired head two ways.** Decision: the gate driver. Finalize step 5,
   `gate-failure.md`'s two-agents flow, and `agents/docket-integration-repair.md` re-gate a
   repaired head with `gate.drive.start --repo-dir <feature worktree> --owner finalize --change-id
   <id> --run-root <dir> --json`, then `gate.drive.advance` slices until terminal, under
   docket-build's gate-run posture (a dispatched agent never yields). A `PASSED` drive whose head
   equals the repaired head feeds `evidence.record --id <id> --run <raw run dir from the PASSED
   document> --head <repaired head>`; `FAILED` returns to repair within
   `finalize.repair_max_attempts`; `HALTED` is `halted`. The `--owner finalize` drive resolves
   `finalize.test_command` and does not charge the build attempt budget (`internal/cli/gate.go`,
   `internal/app/gate_drive.go`). `gate-driver.md`'s caller list names this re-gate as a direct
   caller; its rule that the raw verbs are primitive/operator APIs stands unchanged. If
   `evidence.record` refuses a finalize-owned drive's run, stop and report it in the results file —
   never fall back to raw verbs.
4. **Finalize's sign-off advice is wrong.** SKILL.md step 6 (the autonomous bullet) and
   `gate-failure.md` (*Sign-off on auto-authored repairs*) both say the human re-runs finalize and
   the retry clears the `repair-needs-signoff` block. A re-run does not clear it, and a sign-off
   relayed through an agent's prompt is not authority. Fix both: the human reviews the pushed
   repair, signs off by running `finalize clear-block` (`--id --revision --head <repaired head>
   --pr-number`) themselves, then re-runs finalize. Verify that the autonomous path publishes the
   repaired head before it records the block (clear-block reprobes the published remote ref), and
   fix whichever text is wrong.

### Citations and history narration

About 55 "change NNNN" citations (convention about 24, implement-next 7, `close-out.md` 6,
`edge-paths.md` 5, agent-layer 4, results-template 2, others 1 each), 2 "#NNN" citations in
`learnings.md`, about 34 "Go v1" narrations, and `AGENTS.md`'s line-wrapped "since change 0392".
That `AGENTS.md` bullet stays — install does tolerate unknown keys — without the citation.

## Guards and pinned tests

- **Extend `TestLivingDocsAlignment`** (`internal/repoguard/docs_alignment_test.go`) to `skills/`,
  `agents/`, and `AGENTS.md`. The citation check applies as is. The unsupported-key check skips
  YAML frontmatter (agent wrappers carry a real `skills` field) and nothing else; prose that
  describes a wrapper's frontmatter avoids the `skills:` spelling, per the guard's stated limit.
  `AGENTS.md`'s generated dispatch block is scanned like the rest (it carries no citations today).
  Mutation-test the extension: a citation or a registry-derived key planted in a skill body and in
  an agent wrapper body reddens it; the same key in agent frontmatter does not; the population
  floor counts the new roots. #505 (in progress) moves the guard's key matcher into
  `internal/config`; extend whichever matcher is current at build time.
- **Re-key `TestSkillHandoffSites`** from `$SKILL_*` sigils to invocations of the fixed role-skill
  names, keeping its `DIRECTED to:` requirement and its population floors. Mutation-test it:
  stripping a `DIRECTED to:` reddens it.
- **Update every pinned row in the same commit as the text it pins**, and remove rows whose target
  is deleted: `TestProseContracts` (`test_dummy_mode`; `test_docket_metadata_branch`, which pins
  docket-adr's "adr-unpublished"; `test_role_skill_self_description`; `test_skill_handoff_precedence`;
  `test_learnings_ledger`; `test_change_types` on the change template; the fix-loop condensation
  row), `TestSkillSizeBudgets` (deleted files; `fix-pass.md` and `gate-driver.md` under their new
  names), `internal/app/results_contract_test.go` (results-template), `TestConfigReadChannel`
  markers, and `TestEmbeddedMatchesAuthored`. Grep `internal/` and `tests/` for every edited path
  before editing — this list is not exhaustive (learning `restatement-accumulates-its-own-guards`).
- **Regenerate the embedded assets** (`go generate ./internal/assets/`) in every commit that edits
  `skills/` or `agents/`.

## Acceptance

- Every worklist item is fixed, deleted, or recorded in the results file as not-a-defect with its
  evidence.
- A whitespace-collapsed grep of the in-scope files, recorded in the results file, finds no
  `SKILL_*`, `DUMMY_MODE_*`, `AUTO_CAPTURE_*`, `REVIEW_MIN_FIX_SEVERITY`, `REVIEW_MAX_FIX_TASKS`,
  `GATE_OBSERVATION_BUDGET`, or `BUILD_CHECKPOINT` reference; no `metadata_branch`, "docket-mode",
  or "repo-mode"; no name from the Bash-era table; and no "loop" except `/loop`.
- The extended living-docs guard and the re-keyed handoff test are green, and their mutations
  redden them.
- Every relative link in the in-scope files resolves, and nothing anywhere in the repository links
  to a deleted or renamed file (`dummy-mode.md`, `gate-execution.md`, `gate-execution-evidence.md`,
  `fix-loop.md`, `gate-caller-loop.md`, the two templates).
- The whole suite (`build.test_command`) is green, and its budget report carries no
  `SERIAL CONFIRMED OVER BUDGET:` line.
