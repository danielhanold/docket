<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0464 — Align the human-facing docs and example config with the docket binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0464-align-guide-install-docs-and-docket-example-yml-with-the-go.md)**
<!-- docket:backlink:end -->
# Align the human-facing docs and example config with the docket binary — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`: one `### Task N` per tier
> worker, strictly sequential, one commit per task. Steps use checkbox (`- [ ]`) syntax for
> tracking. Every task's requirements include this plan's **Global Constraints** and **Verified
> facts** sections — read both before you edit anything.

**Goal:** Every living human-facing doc (README, guide, install, concepts, reference) and
`.docket.example.yml` describes only what the `docket` binary does today, and a repo test keeps
change citations and unsupported config keys out of those docs.

**Architecture:** Four small code tasks come first (the `docket run` help text, deleting dead
runner scripts, re-cutting the harness-defaults fixture after a comment rewrite, and a schema
accessor plus the example-config test rework and rewrite). Then a new repoguard docs guard lands,
covering only `docs/concepts` at first. Each later doc task aligns its pages and adds them to the
guard's root list, so every commit stays green and every doc task has a mechanical check.
The last task closes the list to the canonical roots, sets the population floor, and runs a
one-off link check.

**Tech Stack:** Go (`internal/config`, `internal/repoguard`, `internal/cli`, `internal/assets`),
Markdown, YAML.

**Spec:** `docs/superpowers/specs/2026-10-04-align-guide-install-docs-and-docket-example-yml-with-the-go-design.md`
on the `docket` metadata branch (local copy:
`/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-align-guide-install-docs-and-docket-example-yml-with-the-go-design.md`).
Read the spec's *Alignment rules*, *Terminology*, and *Facts* sections once.

**Plan file (this file):**
`docs/superpowers/plans/2026-10-04-align-guide-install-docs-and-docket-example-yml-with-the-go.md`
in the feature worktree.

## Global Constraints

- **Describe only current behaviour.** The binary is the oracle: `internal/config/schema.go` and
  `capability.go` for configuration, `go run ./cmd/docket <verb> --help` and
  `go run ./cmd/docket capabilities --json` for commands, the Go code for mechanisms. Verify a
  claim before you write it.
- **Removed or refused things never appear.** Delete any setting the schema marks obsolete,
  inert, inert-companion, deferred, or deferred-active, along with retired features, deleted
  commands, flags, scripts, and checks, and the old names of renamed terms. No "this used to…",
  no tombstone paragraph, no "not supported, remove it" note, no table of refused keys.
- **No citations of individual changes or PRs** ("change 0363", "changes 0064/0084",
  "change-0365", "pre-0051", "since 0392", "PR #344", "#0471"). When an example id is needed,
  use `<id>` or put it inside a code span.
- **ADR citations** are fine where the ADR's decision is still how docket works. A relative ADR
  link must resolve on the integration branch (`docs/adrs/` there holds ADR-0001…0096). Cite a
  higher ADR as plain text ("ADR-0129"), never as a link.
- **Point-in-time records are untouched:** `docs/changes/`, `docs/results/`, `docs/adrs/`,
  `docs/superpowers/`, `docs/comparison/`, `docs/codex/fixtures/`, `testdata/**` frozen trees.
- **Plain register.** Don't use jargon such as "Go v1" or Bash-era vocabulary.
- **Terminology:**
  - "the daily loop" becomes **"the five steps"**. The page is `docs/guide/five-steps.md`, titled
    "Using docket: the five steps" (capture, groom when needed, build, review and merge, close
    out). Drop "daily".
  - "Loop" means only `/loop`, the harness command that re-runs a workflow until its queue
    drains. For the autonomous builder ("the loop", "the autonomous loop", "hand work to the
    loop"), write **implement-next** or **a build run**: one run builds one change.
  - "fix loop" becomes **fix pass**. "build loop" / "build-loop memory" become **builds** /
    **the learnings ledger**. "both halves of the daily loop" becomes **both drains** (the
    implement-next drain and the finalize drain).
  - Do NOT rename the skill file `skills/docket-implement-next/references/fix-loop.md` and do not
    edit it. Skills belong to the follow-on change #502.
- **Out of scope (never edit):** `skills/**`, `agents/docket-*.md`, `AGENTS.md`, `CLAUDE.md`,
  `cursor-rules/**`, and anything that changes binary behaviour (including deleting
  schema rows). The only Go behaviour change is the `docket run` short help string.
- **Same-commit test coupling.** Before editing any file, grep `internal/` and `tests/` for its
  repo-relative path and for every phrase you remove (`grep -rn -- "<path or phrase>" internal tests`).
  Update or remove every assert that pins wording on the file **in the same commit** as the text
  it guards. If a pinned phrase's claim is still true, repoint the row to the current clause that
  states it. If the claim is gone, remove the row and say so in your report. Never re-add deleted
  text to keep a grep green.
- **Focused test command shape:** `timeout --kill-after=10s 10m go test ./<pkg>/ -run '<Name>' -count=1`
  (`-count=1` defeats the result cache; use `gtimeout` on macOS when `timeout` is absent).
- **Full suite (build gate):** `go run ./cmd/docket development test`. docket-build runs it once
  at the end; don't run it per task.
- **Embedded assets:** whenever `.docket.example.yml` or `agents/harness-defaults.yml` changes,
  run `go generate ./internal/assets/` and commit the regenerated `internal/assets/embedded/`
  files in the same commit (`TestEmbeddedMatchesAuthored` byte-compares them).
- **Shell rules (AGENTS.md):** never pipe a producer into `grep -q`/`head` under pipefail; template
  `mktemp` as `"${TMPDIR:-/tmp}/<name>.XXXXXX"`; `grep` here is ugrep, so use `/usr/bin/grep`
  for portable regex checks; mutation-test restores come from a backup copy, never
  `git checkout --` (that discards the uncommitted work under test).
- **Commits:** stage explicit paths only (never `git add -A` / `git add .`); use `git rm` for
  deletions. One commit per task.

## Verified facts (the rewrite relies on these — re-verify the one you use)

- **Supported configuration keys.** These are the only keys any doc may name:
  - The four shared-setting-guarded, repository-only keys: `integration_branch`, `changes_dir`,
    `adrs_dir`, `results_dir`.
  - `finalize.gate` (`local` | `off`), `finalize.test_command`, `finalize.require_pr_approval`,
    `finalize.resolver_max_attempts` (10), `finalize.repair_max_attempts` (6).
  - `build.gate` (`local` | `off`), `build.test_command`, `build.max_attempts` (4).
  - `run.max_attempts` (2).
  - `review.min_fix_severity` (`minor`), `review.max_fix_tasks` (10).
  - `reclaim.lease_ttl` (72), `reclaim.auto` (false).
  - `learnings.enabled` (true).
  - `gate_observation_budget` (30).
  - `board_surfaces` (`[inline]`), `board.section_order`, `board.sorting.<section>.by|direction`.
  - `change_types`, `agent_harnesses`.
  - `agents.<harness>.<agent>.model|effort`, honoured **from the global config only**.

  Everything else in the schema is unsupported and is never mentioned.
- **Layers.** There are four:
  - global `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`
  - committed `.docket.yml`
  - machine-local `.docket.local.yml`
  - the built-in defaults

  Precedence is local > committed > global > built-in. Agent model/effort overrides are honoured
  from the global file only. A malformed file, an unknown key, or a bad value in **any** layer
  makes the whole configuration invalid; there is no per-layer fallback. An unreadable file is a
  load error. `docket diagnostic config --repo-dir .` shows the resolved configuration and
  anything that blocks writes. Nothing writes or maintains the global file for you, and there is
  no `agents.yaml` migration.
- **`agent_harnesses`** has no default; when absent, install touches no repository surface.
  - Only a `.docket.yml` / `.docket.local.yml` declaration authorizes repository surfaces. The
    installer ignores a global value, and an unknown token is an error.
  - It selects only the parent-facing dispatch surfaces: the managed block in `CLAUDE.md` /
    `AGENTS.md` (with a `CLAUDE.md` → `AGENTS.md` symlink when Codex or OpenCode is also opted in)
    and `.cursor/rules/docket-dispatch.mdc`.
- **Agent wrappers** are user-level only: `~/.claude/agents/`, `~/.codex/agents/`,
  `~/.cursor/agents/`, `${XDG_CONFIG_HOME:-~/.config}/opencode/agents/`. Each harness gets 17
  agents. No per-repository wrapper files exist. The built-in model/effort table is compiled into
  the binary; `agents/harness-defaults.yml` is its shipped mirror.
- **Install.**
  - `install.sh` is a bootstrapper. It forwards only `--bin-dir` and `--harness` to
    `docket development install --source <dir>`.
  - `docket install check` checks this machine's installation only; it is not a repository CI
    gate.
  - The `.gitignore` managed block is written by `docket repository init` / `migrate` and checked
    by `docket repository check`.
- **Repository setup.**
  - A repository that never used docket runs `docket repository init`.
  - `docket repository migrate` converts only a legacy single-branch layout.
  - `check` reports health.
  - `repair` previews a fix and applies it only with a human's `--yes`.
  - `configure-tests` sets the test commands.
  - There is one metadata layout: the `docket` branch through the `.docket/` worktree.
- **Status and maintenance.**
  - `docket status` is read-only.
  - Sweeping merged changes to done, cleanup retries, and lease reclaim are
    `docket maintenance sweep`. The status skill runs it on an explicit refresh; implement-next
    runs `docket maintenance preflight`.
  - With `reclaim.auto: false`, an eligible change is reported as skipped
    (`reclaim-auto-disabled`). Reclaim requires an expired lease, no feature branch, and no
    workspace.
  - Status health findings are structural: `artifact-missing`, `change-reference-dangling`,
    `change-dependency-cycle`, `branch-malformed`, plus configuration and parse diagnostics.
    There is no stale-claim or stalled-dependency check.
  - A halted change stays `in-progress`. Recovery is `docket change resume-halted` and
    `docket run start implement-next --resume <id>`.
- **Lifecycle.**
  - A dependency is satisfied only at `done`.
  - `done` is written by `docket finalize closeout` after it proves the merge; the maintenance
    sweep is the safety net.
  - A stacked change's base is its parent's branch while the parent is live, and the integration
    branch once the parent is `done`.
  - Statuses include `blocked`, `deferred`, `killed`, and `stacked-merged`.
  - Branches are `<type>/<slug>` or `<branch_prefix>/<slug>`.
- **Finalize.**
  - The sequence:
    1. Rebase onto the change's effective base. A conflict goes to the resolver.
    2. Retest. A red suite goes to repair, capped by `finalize.repair_max_attempts`.
    3. `finalize publish` pushes the rebased head and updates the PR's build-evidence block.
    4. Merge with the first method the repository permits: rebase, merge commit, or squash.
    5. Closeout archives the change on the `docket` branch only and retargets backlinks.
    6. Cleanup.
  - Stacked children are retargeted (`finalize retarget-children`).
  - An autonomously authored repair halts for human sign-off
    (`finalize block --reason repair-needs-signoff`), cleared with `finalize clear-block`.
  - `finalize.gate: off` skips rebase and retest. A no-op rebase with exact-head green evidence
    and the same command skips the suite.
  - A failed backlink retarget is `final-backlink-pending`, repaired by `finalize cleanup`.
  - Nothing is copied to the integration branch at close-out.
- **Build gate and evidence.**
  - The build gate runs `build.test_command`. Red runs become repair tasks (premium → max → halt)
    until `build.max_attempts` is spent.
  - `build.gate: off` runs nothing and records `skipped` / `build-gate-off` evidence.
  - An empty `build.test_command` under `local` halts with the remedy
    `docket repository configure-tests`.
  - Build evidence is an immutable record minted by `docket evidence record` from a passed run
    and checked by `docket evidence verify`. It lives in the PR body's build-evidence block and is
    never committed.
  - Per-test-file wall-clock budgets (`BUDGET WATCH`, `SERIAL CONFIRMED OVER BUDGET`) belong to
    docket's own contributor suite (`docket development test`), not to the build gate.
- **Review.**
  - The review tier maps same-level from the build tier: economy → lean, standard → standard,
    premium/max → deep, with a bump for very large diffs.
  - Out-of-branch follow-up work is reported in the run's final report for a human to capture;
    nothing is minted automatically.
- **Run tracker.**
  - `docket run start` writes the run record and launches nothing; the parent dispatches.
  - `docket run verdict` always exits 0 and prints one decision line:
    - `run-done`
    - `run-retry-once`
    - `run-stop` (a halt is `run-stop … run-halted`)
    - `run-continue` (nonterminal; it also authorizes re-dispatch)
    - `run-observe` (unattributed)
  - `run start --resume` refuses with `resume-active-run` or `cancellation-pending`, or returns
    the reserved replacement (`resume-replacement-reserved`).
  - `run cancel` reports `cancelled`, `cancellation-pending`, `already-cancelled`, or `refused`.
  - `run-untracked` lets a run be dispatched without a key, and `run verify` checks a change's
    run against its postconditions.
  - Gate-run exit codes are 0/1/2, and a halted gate run exits like a failed one.
- **Learnings.**
  - `docket learning record` / `docket learning update` (JSON request) write findings.
  - `learnings.enabled: false` refuses them and turns reads off.
  - Nothing harvests, promotes, re-indexes, or counts findings automatically, and the index file
    is never regenerated.
- **Outcomes vocabulary.**
  - Every result envelope carries `result`: `applied`, `no-op`, `contended`, `invalid-input`,
    `invalid-state`, `blocked`, `unsupported-config`, `gate-failed`, `external-failed`,
    `interrupted`, or `internal-error`.
  - Each operation has its own disposition vocabulary, listed by `docket schema`.
  - `applied` / `no-op` / `refused` / `error` is `repository prepare`'s disposition set only.
- **Default workflow roles:** superpowers for brainstorm / plan / finish (`superpowers:brainstorming`,
  `superpowers:writing-plans`, `superpowers:finishing-a-development-branch`); docket's own for
  build and review (`docket-build`, `docket-review`). Roles are fixed; nothing rebinds them.

## Review Focus

These are the five input classes the spec implies but no worklist item exercises, most likely
first. The owning task pins each one.

- A **plain-English sentence that ends a word with a colon** ("two kinds of skills: …") must not
  trip the unsupported-key guard. Only a line-start YAML key or a code span opening with the key
  counts. Pinned in Task 5's non-vacuity cases.
- A **current per-change field whose name extends a refused key** (`auto_groomable:`) and
  **year-like or vendor numbers** ("since 2024", `deepseek-v4-flash-0731`, `ADR-0071`, a heading
  anchor `#1-capture`) must pass the guard. Pinned in Task 5.
- A **verbatim copy of `.docket.example.yml`** as `.docket.yml` must resolve with no warnings, no
  write blockers, and effective values equal to the built-in defaults. Someone who copies it into
  a machine layer must be told which keys are repository-only. Pinned in Task 4: the
  verbatim-copy assert plus the header wording.
- **Links and anchors after renames and deletions.** The renamed page (`five-steps.md`), the
  deleted pages, the renamed glossary entry (`#fix-pass`), and removed sections must leave no
  dangling relative link or anchor. Checked by Task 14's one-off link check.
- **The guard's file population can silently shrink**, for example a renamed root or an
  unterminated code fence that masks the rest of a file. The guard fails closed on a missing root
  and on an unterminated fence, and Task 14 adds the population floor. Pinned in Task 5 and
  Task 14.

---

### Task 1: Correct the `docket run` short help

**Build tier:** economy

**Files:**
- Modify: `internal/cli/run.go` (the `Short:` of the `run` group command in `newRunCommand`)
- Test: `internal/cli/run_test.go`

**Interfaces:**
- Consumes: `runCLI(t, args...)` from `internal/cli/root_test.go`.
- Produces: the `run` group's short help. `docs/reference/cli.md` mirrors it later (Task 12).

- [ ] **Step 1: Write the failing test.** Append to `internal/cli/run_test.go`:

```go
// TestRunGroupShortHelpIsNotReadOnly pins the run group's one-line summary:
// run start, run cancel, run continue, and run verdict write local run state,
// so the group must not advertise itself as read-only.
func TestRunGroupShortHelpIsNotReadOnly(t *testing.T) {
	out, errS, code := runCLI(t, "run", "--help")
	if code != 0 {
		t.Fatalf("run --help exit %d, stderr %q", code, errS)
	}
	first := strings.SplitN(out, "\n", 2)[0]
	if strings.Contains(first, "read-only") {
		t.Errorf("run group summary still claims read-only: %q", first)
	}
	const want = "Track dispatched runs and verify a change's claim-to-implemented run"
	if first != want {
		t.Errorf("run group summary = %q, want %q", first, want)
	}
}
```

Add `"strings"` to the file's imports if it is not already imported.

- [ ] **Step 2: Run it and confirm it fails.**
  Run: `timeout --kill-after=10s 10m go test ./internal/cli/ -run TestRunGroupShortHelpIsNotReadOnly -count=1`
  Expected: FAIL, with the summary reported as
  `"Report on a change's claim-to-implemented run (read-only)"`.

- [ ] **Step 3: Implement.** In `internal/cli/run.go`, change the `run` group's
  `Short: "Report on a change's claim-to-implemented run (read-only)",` to
  `Short: "Track dispatched runs and verify a change's claim-to-implemented run",`.
  Leave every subcommand's `Short` untouched.

- [ ] **Step 4: Check for other pins on the old string, then re-run.**
  Run `grep -rn -- "claim-to-implemented run (read-only)" internal cmd tests`. Expected: no hit in
  Go or tests. (`docs/reference/cli.md` still carries it; Task 12 fixes that page.)
  Run: `timeout --kill-after=10s 10m go test ./internal/cli/ -count=1`
  Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/cli/run.go internal/cli/run_test.go
git commit -m "fix(cli): docket run help no longer claims read-only"
```

---

### Task 2: Delete the dead runner scripts

**Build tier:** economy

**Files:**
- Delete: `scripts/runners/codex.sh`, `scripts/runners/codex.md`, `scripts/runners/cursor.sh`,
  `scripts/runners/cursor.md`, `scripts/runners/opencode.sh`, `scripts/runners/opencode.md`
- Modify: `internal/repoguard/contracts_test.go` (`TestScriptContractsCoverage`, its comment block)
- Modify: `internal/repoguard/absence_test.go` (header comment sentence naming
  "the surviving scripts/runners/*.md prose")

**Interfaces:** none produced. The generated-output ban lists that forbid the token
`scripts/runners` stay unchanged and must keep passing: `runnerEraTokens` in
`internal/harness/native_dispatch_test.go`, the banned list in `internal/harness/dispatch_test.go`,
and the token list in `internal/reposeed/plan_test.go`. The temp-dir fixtures in
`internal/repoguard/repoguard_test.go` and `absence_test.go` that *write* a fake
`scripts/runners/codex.sh` also stay; they exercise walk logic, not the real files.

- [ ] **Step 1: Prove nothing invokes them.**
  Run `grep -rn -- "scripts/runners" --include='*.go' --include='*.sh' --include='*.yml' --include='*.json' . | grep -v '^./docs/'`
  Expected: only the test files listed above, plus the six runner files themselves. If any
  non-test caller appears, stop and report BLOCKED.

- [ ] **Step 2: Delete the files.** `git rm scripts/runners/codex.sh scripts/runners/codex.md scripts/runners/cursor.sh scripts/runners/cursor.md scripts/runners/opencode.sh scripts/runners/opencode.md`

- [ ] **Step 3: Run the coverage guard and confirm the floor reddens.**
  Run: `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestScriptContractsCoverage -count=1`
  Expected: FAIL with `population floor: only 2 scripts/*.{sh,md} scanned (expected >= 4)`.

- [ ] **Step 4: Fix the guard.** In `TestScriptContractsCoverage`:
  - Delete the `scriptRunners` regexp, the `runners` map, its loop branch, and the
    `pairCoverage("scripts/runners", runners)` call. Nothing under `scripts/runners/` exists any
    more.
  - Set the floor to `if scanned < 2` and replace the floor comment with: "Population floor:
    scripts/release-smoke.{sh,md} is the surviving pair; a collapse below it means the scan
    stopped reaching scripts/."
  - Update the section banner comment `scripts/<name>.sh <-> scripts/<name>.md coverage (top level and runners/)`
    to drop "and runners/".
  - In `absence_test.go`'s header, rewrite the sentence about "the surviving
    scripts/runners/*.md prose that names `runner-dispatch.sh` descriptively" so it no longer
    claims such prose exists. Keep the documented-removal-sentence clause.

- [ ] **Step 5: Mutation-check the floor.** Back up `internal/repoguard/contracts_test.go` to a
  `mktemp` file. Temporarily change `scriptTop` to a pattern that matches nothing (for example
  `^scripts/NOPE/([^/]+)\.(sh|md)$`), run the test, and expect FAIL on the floor. Then restore
  from the backup with `cp -f`.

- [ ] **Step 6: Run the package.**
  Run: `timeout --kill-after=10s 10m go test ./internal/repoguard/ ./internal/harness/... ./internal/reposeed/ -count=1`
  Expected: PASS.

- [ ] **Step 7: Commit.**

```bash
git add internal/repoguard/contracts_test.go internal/repoguard/absence_test.go
git commit -m "chore: delete dead scripts/runners files and lower the script-contract floor"
```

(The `git rm` in Step 2 already staged the deletions.)

---

### Task 3: Rewrite the `agents/harness-defaults.yml` comments and cut a new frozen sidecar

**Build tier:** standard

**Files:**
- Modify: `agents/harness-defaults.yml` (comments only; every key and value byte-identical)
- Create: `testdata/repositories/v0.9.11/agents-harness-defaults.yml` (byte copy of the edited live file)
- Create: `testdata/repositories/v0.9.11/PROVENANCE.md`
- Modify: `internal/config/defaults_test.go` (`sidecarPath` constant and its comment)
- Modify: `internal/config/defaults.go` (the file-header comment naming the v0.9.9 tree)
- Modify: `internal/repoguard/absence_test.go` (the header paragraph that records the
  harness-defaults comment as known-stale)
- Regenerate: `internal/assets/embedded/` (`go generate ./internal/assets/`)

**Interfaces:** none. `TestBuiltinAgentsParityWithFrozenSidecar` keeps its name and logic; only
`sidecarPath` moves.

Never edit any file under `testdata/repositories/v0.9.2`–`v0.9.10`. Frozen trees are immutable
(`testdata/README.md`). A new upstream state gets a new tree.

- [ ] **Step 1: Confirm the values are untouched before and after.** Save the value lines:
  `vals_before=$(/usr/bin/grep -vE '^[[:space:]]*(#|$)' agents/harness-defaults.yml)`. Keep the
  variable for Step 4.

- [ ] **Step 2: Rewrite the header comment block (everything above `agents:`)** so it states only
  current facts:
  - This file is docket's shipped per-harness agent model/effort table. The binary's compiled
    built-in agent table (`internal/config/defaults.go`) mirrors it, and a test keeps the two
    equal.
  - Only the global configuration (`${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`) overrides
    it. Overrides are per agent and per field, and the repository layers never override agent
    pins.
  - Shape rules that still hold: entries nest under a concrete harness (no harness-neutral
    `default:` block); keys are wrapper short names (`build-economy`, not
    `docket-build-economy`); each listed entry supplies both model and effort; every shipped
    harness (claude, cursor, codex, opencode) carries a complete 17-agent block.
  - Model IDs and effort tokens are opaque passthrough values (ADR-0015 may stay cited).

  Delete every reference to `scripts/lib/harness-defaults.sh`, `sync-agents.sh`,
  `HD_SHIPPED_HARNESSES`, `hd_validate`, the Bash reader ("the reader consumes everything up to
  the next `,` or `}`"), the bare-scalar/clipping rationale tied to that reader, and the
  "`runner:` is forbidden — delegation…" bullet. Also check the ADR-0016 citation: keep it only
  if ADR-0016's decision (layered agent resolution) still matches the global-only override rule.
  Otherwise drop it.

- [ ] **Step 3: Fix in-block comments.** Delete the change citations "(change 0169)" and
  "(change 0192)" in the codex and opencode block comments. Keep the substantive tiering
  rationale. Don't change any `{ model: …, effort: … }` line.

- [ ] **Step 4: Prove values are unchanged.**
  `vals_after=$(/usr/bin/grep -vE '^[[:space:]]*(#|$)' agents/harness-defaults.yml)`, then
  `[ "$vals_before" = "$vals_after" ] && echo SAME`. Expected: `SAME`. If not, restore the value
  lines and repeat.

- [ ] **Step 5: Watch the drift guard redden.**
  Run: `timeout --kill-after=10s 10m go test ./internal/config/ -run TestBuiltinAgentsParityWithFrozenSidecar -count=1`
  Expected: FAIL from `assertFrozenCopyMatchesLive` ("The shipped agent defaults changed…").

- [ ] **Step 6: Cut the new tree.**
  `mkdir -p testdata/repositories/v0.9.11 && cp -f agents/harness-defaults.yml testdata/repositories/v0.9.11/agents-harness-defaults.yml`.
  Write `testdata/repositories/v0.9.11/PROVENANCE.md` following `v0.9.9/PROVENANCE.md`'s shape:
  - Source repo `danielhanold/docket`; commit "this change's feature commit (the tree equals
    `agents/harness-defaults.yml` at that commit)"; date 2026-10-04; redaction none.
  - Cut from `v0.9.9/agents-harness-defaults.yml`. It differs in comments only (the header now
    describes the compiled built-in table and global-only overrides, and the deleted Bash tooling
    references are gone). No key or value changed, so `internal/config/defaults.go`'s table is
    unchanged.
  - Only the parity oracle's `sidecarPath` advances to this tree. Older trees are immutable.

  PROVENANCE.md is a point-in-time record, so a change citation is allowed there, but use
  "this change" instead.

- [ ] **Step 7: Repoint the oracle.** In `internal/config/defaults_test.go`, set
  `sidecarPath = "../../testdata/repositories/v0.9.11/agents-harness-defaults.yml"` and fix the
  comment above the constants: drop "the file sync-agents actually ships" and say "the file
  docket ships". In `TestBuiltinAgentsParityWithFrozenSidecar`'s doc comment, replace "what
  sync-agents ships today" with "what docket ships today". In `internal/config/defaults.go`'s
  header, replace "`testdata/repositories/v0.9.9/` (re-cut comment-only by change 0473)" with
  "`testdata/repositories/v0.9.11/` (re-cut comment-only)". Keep the rest of the sentence.

- [ ] **Step 8: Fix the absence-test header.** In `internal/repoguard/absence_test.go`, the
  paragraph beginning "Config YAML (.yml): most .yml is out of population, but
  agents/harness-defaults.yml IS scanned…" says the header comment names the deleted
  `scripts/lib/harness-defaults.sh` and that the fix is deferred. Rewrite it: the file is
  scanned, carries no retired reference, and is byte-pinned by
  `TestBuiltinAgentsParityWithFrozenSidecar` (currently v0.9.11) and the embedded copy. Keep the
  sentence about root `.docket.yml` naming run-tests.sh only if it is still true. Check with
  `grep -n run-tests .docket.yml`.

- [ ] **Step 9: Regenerate embedded assets.** `go generate ./internal/assets/`

- [ ] **Step 10: Run.**
  Run: `timeout --kill-after=10s 10m go test ./internal/config/ ./internal/assets/ ./internal/repoguard/ -count=1`
  Expected: PASS.

- [ ] **Step 11: Commit.**

```bash
git add agents/harness-defaults.yml testdata/repositories/v0.9.11/agents-harness-defaults.yml testdata/repositories/v0.9.11/PROVENANCE.md internal/config/defaults_test.go internal/config/defaults.go internal/repoguard/absence_test.go internal/assets/embedded
git commit -m "docs(agents): harness-defaults comments describe the compiled table; re-cut sidecar v0.9.11"
```

---

### Task 4: Schema support accessor, example-config test rework, and a copy-safe `.docket.example.yml`

**Build tier:** standard

**Files:**
- Modify: `internal/config/schema.go` (add `SettingPath`, `SettingPaths`, `dispositionSupported`)
- Create: `internal/config/setting_paths_test.go`
- Modify: `internal/config/example_correspondence_test.go`
- Rewrite: `.docket.example.yml`
- Modify: `internal/repoguard/prose_contracts_test.go` (the `test_docket_example_yml` row, only if
  its phrases change)
- Regenerate: `internal/assets/embedded/`

**Interfaces:**
- Produces (Task 5 consumes):

```go
// in package config (internal/config/schema.go)
type SettingPath struct {
	Path      string // dotted; dynamic segments spelled "*"
	Supported bool
}
func SettingPaths() []SettingPath // every registry row, registry order
```

- [ ] **Step 1: Write the accessor test (fails: undefined).** Create
  `internal/config/setting_paths_test.go`:

```go
package config

import (
	"sort"
	"testing"
)

// TestSettingPathsSupportSplit pins the registry's support verdict to the
// supported key list the documentation promises. The expected supported set is
// written out because it is the contract docs and .docket.example.yml are held
// to; the unsupported side is checked by sampling every disposition family.
func TestSettingPathsSupportSplit(t *testing.T) {
	paths := SettingPaths()
	if len(paths) != len(registry()) {
		t.Fatalf("SettingPaths returned %d rows, registry has %d", len(paths), len(registry()))
	}
	var supported []string
	unsupported := map[string]bool{}
	for _, p := range paths {
		if p.Supported {
			supported = append(supported, p.Path)
		} else {
			unsupported[p.Path] = true
		}
	}
	want := []string{
		"integration_branch", "changes_dir", "adrs_dir", "results_dir",
		"finalize.gate", "finalize.test_command", "finalize.require_pr_approval",
		"finalize.resolver_max_attempts", "finalize.repair_max_attempts",
		"build.gate", "build.test_command", "build.max_attempts",
		"run.max_attempts",
		"review.min_fix_severity", "review.max_fix_tasks",
		"reclaim.lease_ttl", "reclaim.auto",
		"learnings.enabled",
		"gate_observation_budget",
		"board_surfaces", "board.section_order",
		"change_types", "agent_harnesses",
		"agents.*.*.model", "agents.*.*.effort",
	}
	for _, s := range BoardSectionTokens {
		want = append(want, "board.sorting."+s+".by", "board.sorting."+s+".direction")
	}
	sort.Strings(supported)
	sort.Strings(want)
	if len(supported) != len(want) {
		t.Fatalf("supported paths:\n got %v\nwant %v", supported, want)
	}
	for i := range want {
		if supported[i] != want[i] {
			t.Fatalf("supported paths:\n got %v\nwant %v", supported, want)
		}
	}
	// One sample per unsupported disposition family.
	for _, p := range []string{
		"metadata_branch", "runtime.bash", // obsolete
		"learnings.cap", "delegation_observation_budget", "github_project", // inert
		"build.checkpoint", "terminal_publish", "auto_groom", "finalize.skip_results_only_delta", // deferred
		"auto_capture.types", "dummy_mode.persona", "runners.*.shim_model", // inert companion
		"skills.build", "agents.*.*.runner", // deferred-active
	} {
		if !unsupported[p] {
			t.Errorf("%s should be unsupported", p)
		}
	}
}
```

- [ ] **Step 2: Run and confirm it fails to compile.**
  Run: `timeout --kill-after=10s 10m go test ./internal/config/ -run TestSettingPathsSupportSplit -count=1`
  Expected: FAIL, `undefined: SettingPaths`.

- [ ] **Step 3: Implement in `internal/config/schema.go`** (place it after `func registry()`):

```go
// SettingPath is one configuration path from the registry (dynamic segments
// spelled "*") and whether docket supports it today. Repository guards read it
// so documentation checks derive the unsupported key set from this registry
// rather than re-listing it.
type SettingPath struct {
	Path      string
	Supported bool
}

// SettingPaths returns every registry path, in registry order, with its
// support verdict.
func SettingPaths() []SettingPath {
	out := make([]SettingPath, 0, len(registryTable))
	for _, s := range registryTable {
		out = append(out, SettingPath{Path: s.path, Supported: dispositionSupported(s.disp)})
	}
	return out
}

// dispositionSupported reports whether a disposition family is supported
// configuration. finalize.gate (refused only for some values), board_surfaces
// (one token dropped), and the agents model/effort leaves (honoured only from
// the global layer) are supported keys whose individual values or layers may
// still be refused. Obsolete, inert, inert-companion, deferred, and
// deferred-active paths are not supported. A new disposition defaults to
// unsupported until it is added here.
func dispositionSupported(d disposition) bool {
	switch d {
	case dispSupported, dispDeferredByValue, dispSupportedOrDropped, dispAgentsLeaf:
		return true
	}
	return false
}
```

  Run the Step 2 command. Expected: PASS. If the supported set differs from `want`, the code is
  the oracle. Stop and report BLOCKED with the diff; never edit the schema.

- [ ] **Step 4: Rework `TestExampleSchemaCorrespondence` (red against the current example).**
  Edit `internal/config/example_correspondence_test.go`:
  - Header comment: drop every change citation and the "retired Bash test" history. State that
    the guard checks three things: every documented key is a schema path (A); every **supported**
    schema path is documented (B); no **unsupported** path is documented as an active or
    scope-tagged key (C). It also checks that a verbatim copy as `.docket.yml` resolves cleanly
    (D). The `activeKeyRe` comment drops its "change 0367" citation.
  - `scopeTagRe`: the alternation becomes `(repo-only|any layer|global-only)` (`local-only` is
    gone).
  - Population openers loop: `for _, k := range []string{"agents", "agent_harnesses"}`.
  - Extract the direction checks into pure functions so mutations can call them:

```go
// undocumentedSupported returns every supported registry path the documented
// set does not cover: a static path by its exact key, a dynamic path by any
// documented ancestor.
func undocumentedSupported(documented map[string]bool) []string {
	var out []string
	for _, spec := range registry() {
		if !dispositionSupported(spec.disp) {
			continue
		}
		segs := splitPath(spec.path)
		if strings.Contains(spec.path, "*") {
			hit := false
			for k := range documented {
				if matchAt(segs, splitPath(k)) {
					hit = true
					break
				}
			}
			if !hit {
				out = append(out, spec.path)
			}
			continue
		}
		if !documented[spec.path] {
			out = append(out, spec.path)
		}
	}
	sort.Strings(out)
	return out
}

// documentedUnsupported returns every documented key that names only
// unsupported registry paths (exactly or as a block header), such as `skills`
// or `learnings.cap`. A key that also heads a supported path (`agents`,
// `finalize`) is not flagged.
func documentedUnsupported(documented map[string]bool) []string {
	var out []string
	for k := range documented {
		ks := splitPath(k)
		matched, supported := false, false
		for _, spec := range registry() {
			if matchAt(splitPath(spec.path), ks) {
				matched = true
				if dispositionSupported(spec.disp) {
					supported = true
					break
				}
			}
		}
		if matched && !supported {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// verbatimCopyProblems resolves content as a committed .docket.yml on its own
// and reports what would make a verbatim copy unsafe: a resolve error, a
// warning or error diagnostic, a mutation-preflight blocker, or effective values
// that differ from the built-in defaults.
func verbatimCopyProblems(content string) []string {
	ctx := ResolveContext{DefaultBranch: "main"}
	snap, diags, err := Resolve([]Source{{Layer: LayerRepository, Name: ".docket.yml", Data: []byte(content)}}, ctx)
	if err != nil {
		return []string{fmt.Sprintf("resolve: %v (diagnostics %+v)", err, diags)}
	}
	var problems []string
	for _, d := range snap.Diagnostics {
		if d.Severity == SeverityWarning || d.Severity == SeverityError {
			problems = append(problems, fmt.Sprintf("diagnostic %s at %s: %s", d.Code, d.Path, d.Message))
		}
	}
	if dec := PreflightMutation(snap); !dec.Allowed {
		for _, b := range dec.Blockers {
			problems = append(problems, "mutation blocker: "+b.Path)
		}
	}
	base, _, err := Resolve(nil, ctx)
	if err != nil {
		return append(problems, fmt.Sprintf("resolve built-ins: %v", err))
	}
	if got, want := effectiveValuesOnly(snap.Effective), effectiveValuesOnly(base.Effective); !reflect.DeepEqual(got, want) {
		problems = append(problems, fmt.Sprintf("effective values differ from built-in defaults:\n got: %v\nwant: %v", got, want))
	}
	return problems
}

// effectiveValuesOnly renders an Effective through JSON and drops every
// provenance and explicit marker, leaving only resolved values.
func effectiveValuesOnly(e Effective) any {
	b, err := json.Marshal(e)
	if err != nil {
		return "marshal: " + err.Error()
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return "unmarshal: " + err.Error()
	}
	return dropProvenance(v)
}

func dropProvenance(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if k == "provenance" || k == "explicit" {
				continue
			}
			out[k] = dropProvenance(val)
		}
		return out
	case []any:
		for i := range x {
			x[i] = dropProvenance(x[i])
		}
		return x
	}
	return v
}
```

  - In the test body:
    - Keep Direction A as is.
    - Replace the inline Direction B with
      `if u := undocumentedSupported(documented); len(u) != 0 { t.Errorf(...) }`.
    - Add Direction C: `if u := documentedUnsupported(documented); len(u) != 0 { t.Errorf("unsupported keys documented in .docket.example.yml:\n%s", strings.Join(u, "\n")) }`.
    - Add D: `if p := verbatimCopyProblems(string(b)); len(p) != 0 { t.Errorf("a verbatim copy of .docket.example.yml as .docket.yml is unsafe:\n%s", strings.Join(p, "\n")) }`.
    - The floors count **supported** rows (`dispositionSupported`), not "non-obsolete" ones.
      Leave the floor numbers as they are for now; Step 8 recomputes them.
    - Add imports `encoding/json`, `fmt`, `reflect` as needed.
  - Replace `non_vacuity` with real mutation calls (the old Direction B "shrunk" block never called
    a detector):

```go
	t.Run("non_vacuity", func(t *testing.T) {
		for _, k := range []string{"finalize.gate", "finalize.repair_max_attempts", "run.max_attempts", "board.sorting.in-progress.by", "agent_harnesses", "agents"} {
			if !documented[k] {
				t.Errorf("extraction missed documented key %q", k)
			}
		}
		if knownDocumented("finalize.bogus_setting") || knownDocumented("no_such_top_level_key") {
			t.Errorf("Direction A admitted a bogus key")
		}
		if !knownDocumented("finalize.require_pr_approval") || !knownDocumented("finalize") || !knownDocumented("agents") {
			t.Errorf("Direction A rejected a real key/header")
		}
		// matchAt cases (keep the four existing asserts unchanged).
		// Direction B: strip a supported key -> reported.
		shrunk := map[string]bool{}
		for k := range documented {
			if k != "finalize.gate" {
				shrunk[k] = true
			}
		}
		if u := undocumentedSupported(shrunk); len(u) != 1 || u[0] != "finalize.gate" {
			t.Errorf("Direction B missed a stripped supported key: %v", u)
		}
		// Direction C: plant refused keys -> reported, via the real extractor.
		planted := exampleDocumentedKeys(string(b) + "\n# scope: any layer\nskills:\n  build: docket-build\nlearnings:\n  cap: 300\n")
		got := strings.Join(documentedUnsupported(planted), ",")
		for _, want := range []string{"skills", "skills.build", "learnings.cap"} {
			if !strings.Contains(","+got+",", ","+want+",") {
				t.Errorf("Direction C missed planted %q (got %s)", want, got)
			}
		}
		// D: a blocking active line makes the verbatim copy unsafe.
		if p := verbatimCopyProblems(string(b) + "\nterminal_publish: true\n"); len(p) == 0 {
			t.Errorf("verbatim-copy check admitted a blocking line")
		}
		if p := verbatimCopyProblems(string(b) + "\nskills:\n  build: docket-build\n"); len(p) == 0 {
			t.Errorf("verbatim-copy check admitted a skills binding")
		}
	})
```

  `matchAt`, `splitPath`, `exampleDocumentedKeys`, and `knownDocumented` keep their names. Keep
  the four existing `matchAt` asserts inside `non_vacuity`.

- [ ] **Step 5: Run and confirm it fails on the current example.**
  Run: `timeout --kill-after=10s 10m go test ./internal/config/ -run TestExampleSchemaCorrespondence -count=1`
  Expected: FAIL. Direction C lists `metadata_branch`, `skills`, `runners`, `runtime`,
  `auto_capture`, `dummy_mode`, `terminal_publish`, `auto_groom`, `github_project`,
  `delegation_observation_budget`, `learnings.cap`, `build.checkpoint`, and others. D reports
  blockers. `non_vacuity` passes.

- [ ] **Step 6: Rewrite `.docket.example.yml`.** Keep its role as the one-place reference with a
  short plain comment per key: what it does and its default. Follow this layout:
  - **Header:**
    - This file is documentation; docket never reads it. It lists every supported key with its
      built-in default.
    - The four layers and their precedence, as in Verified facts.
    - **Safe to copy:** copied verbatim as `.docket.yml`, it behaves exactly like having no file
      (every active value is the built-in default).
    - For `.docket.local.yml` or the global config, copy only `scope: any layer` keys: the four
      `scope: repo-only` keys are ignored with a warning there.
    - The three scope tags:
      - `# scope: repo-only (shared-setting guarded)`: committed `.docket.yml` only.
      - `# scope: any layer`.
      - `# scope: global-only (global config.yml)`.
    - `docket diagnostic config --repo-dir .` shows the resolved configuration.

    Delete:
    - the `tests/test_docket_example_yml.sh` references
    - the "standing rule" paragraph's script name (keep one plain sentence: a new key lands here
      with its documentation in the same change, and a test enforces it)
    - the leaf-merge list naming `runtime:`/`skills:`/`runners:`
    - the "eight repo-only keys" count (it is four)
    - every change citation
  - **Active keys at their built-in defaults, grouped as today:**
    - Branch model & layout: `integration_branch: auto`, `changes_dir`, `adrs_dir`,
      `results_dir`, each `scope: repo-only`.
    - `finalize:`: `gate: local` (values `local` | `off`), `test_command: ""`,
      `require_pr_approval: false`, `resolver_max_attempts: 10`, `repair_max_attempts: 6`.
    - `build:`: `gate: local`, `test_command: ""`, `max_attempts: 4`.
    - `run:`: `max_attempts: 2`.
    - `review:`: `min_fix_severity: minor`, `max_fix_tasks: 10`.
    - `reclaim:`: `lease_ttl: 72`, `auto: false`.
    - `learnings:`: `enabled: true`.
    - `gate_observation_budget: 30`.
    - `board_surfaces: [inline]`.
    - `board:`: `section_order` plus all six `sorting.<section>.by/direction` entries.
    - `change_types: [chore, docs, feat, fix, refactor, perf]`.

    Read each default from `internal/config/schema.go`, never from this list.
  - **Commented keys**, each preceded on the line directly above by its scope tag (the extractor
    relies on that adjacency):
    - `# agent_harnesses: [claude]` with `# scope: any layer`. It has no default; it opts this
      repository's dispatch surfaces in; a global value is ignored by the installer.
    - The `# agents:` block with `# scope: global-only (global config.yml)`. Model/effort pins
      are honoured only from the global config; the block shows the shipped values as a
      starting point. Keep the four harness sub-blocks you have now, but make sure no row
      carries `runner:`.
  - **Delete entirely:** `runtime` / `runtime.bash`, `metadata_branch`, `finalize.skip_results_only_delta`,
    `learnings.cap`, `build.checkpoint`, `delegation_observation_budget`, `github_project` (and
    the `github` token commentary under `board_surfaces`), `terminal_publish`, `auto_groom`,
    `auto_capture`, `dummy_mode`, `runners`, and `skills`. Also delete every Bash-era remark: the
    line-oriented reader, in-map `#` truncation, `docket install check` as a CI gate,
    per-repository wrapper files, "Stage 3 bootstrap guard", `g ls-tree`, and main-mode.
  - No line mentions an unsupported key name, even in a comment.

- [ ] **Step 7: Make the test pass.** Run the Step 5 command. Expected: PASS. Fix the example
  (never the test) until A, B, C, and D are all clean.

- [ ] **Step 8: Recompute the floors.** Temporarily print `len(documented)`, the supported static
  count, and the supported dynamic count (`t.Logf`). Set each floor to the measured value (for
  example `if len(documented) < <measured>`) and remove the log. Expected numbers are about
  35 static and 2 dynamic, but use what you measure.

- [ ] **Step 9: Mutation checks** (back up both files with `mktemp`, then restore with `cp -f`):
  - (a) Delete the `  gate: local` line under `finalize:` in the example. The test reddens
    (Direction B).
  - (b) Append `learnings:\n  cap: 300` inside the learnings block. It reddens (C and D).
  - (c) Make `dispositionSupported` return `true` for everything. `TestSettingPathsSupportSplit`
    reddens.

  Restore everything and re-run until green.

- [ ] **Step 10: Acceptance probe.** Verbatim copy through the real binary, with an isolated
  global layer:

```bash
scratch=$(mktemp -d "${TMPDIR:-/tmp}/example-copy.XXXXXX")
xdg=$(mktemp -d "${TMPDIR:-/tmp}/example-xdg.XXXXXX")
git -C "$scratch" init -q
cp -f .docket.example.yml "$scratch/.docket.yml"
XDG_CONFIG_HOME="$xdg" go run ./cmd/docket diagnostic config --for-mutation --repo-dir "$scratch" --default-branch main
```

  Expected: mutation allowed, no blockers, no warnings. Paste the output into your report;
  implement-next records it in the results file.

- [ ] **Step 11: Other pins.** The `test_docket_example_yml` row in
  `internal/repoguard/prose_contracts_test.go` requires `board_surfaces`, `agent_harnesses`, and
  `finalize:`. Keep all three present. Run `grep -rn -- ".docket.example.yml" internal tests` and
  check each hit still holds (most are fixture writes of `version: 1`, which are unaffected).

- [ ] **Step 12: Regenerate and run.** `go generate ./internal/assets/`, then
  `timeout --kill-after=10s 10m go test ./internal/config/ ./internal/assets/ ./internal/repoguard/ ./internal/install/ -count=1`.
  Expected: PASS.

- [ ] **Step 13: Commit.**

```bash
git add internal/config/schema.go internal/config/setting_paths_test.go internal/config/example_correspondence_test.go .docket.example.yml internal/assets/embedded internal/repoguard/prose_contracts_test.go
git commit -m "docs(config): .docket.example.yml lists only supported keys and is safe to copy"
```

---

### Task 5: Add the living-docs guard (seeded with `docs/concepts`)

**Build tier:** standard

**Files:**
- Create: `internal/repoguard/docs_alignment_test.go`

**Interfaces:**
- Consumes: `config.SettingPaths()` / `config.SettingPath` (Task 4); `guardRoot`, `readMaintained`
  (existing repoguard test helpers).
- Produces: `var livingDocRoots []string`. Tasks 6–13 append entries to it and Task 14 closes
  it. Also `TestLivingDocsAlignment`.

The guard checks two defect classes over the living docs: change/PR citations and unsupported
config keys. It is seeded with only `docs/concepts`, which is already clean for both classes.
Every commit stays green; each later doc task extends the list to cover the pages it aligned.

- [ ] **Step 1: Write the guard with its non-vacuity cases.** Create
  `internal/repoguard/docs_alignment_test.go`:

```go
package repoguard

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

// TestLivingDocsAlignment keeps two kinds of drift out of docket's living,
// human-facing documentation (the roots in livingDocRoots; docs/release/ is
// deliberately excluded because dated release evidence lands there):
//
//  1. Citations of individual changes or PRs ("change 0363", "changes 0064/0084",
//     "change-0365", "pre-0051", "since 0392", "PR #344", "#0471"). ADR citations
//     are allowed. Matched case-insensitively on shape, with no exception list.
//  2. Configuration keys the schema registry marks unsupported (obsolete, inert,
//     inert-companion, deferred, deferred-active), derived from
//     config.SettingPaths at test time — never hand-listed.
//
// # Limits (pinned by non_vacuity, not merely stated)
//
//   - Citations inside fenced code blocks and inline code spans are not checked:
//     sample output and placeholder ids live there. A citation written inside a
//     code span therefore escapes.
//   - A four-digit "since"/"pre-" number is a citation only when zero-padded
//     (0NNN), so years ("since 2024") pass.
//   - A key is matched as a dotted path (`skills.build`, `learnings.cap`,
//     `agents.<h>.<a>.runner`), a dotted child of an all-unsupported block
//     (`skills.<role>`), a YAML key at a line start or opening a code span
//     (`skills:`, `terminal_publish:`), or — inside a block that also holds
//     supported keys — an unsupported leaf key at a line start or inside a flow
//     mapping (`checkpoint:`, `cap:`, `{ …, runner: … }`). A bare
//     mention without the dot or colon (`terminal_publish` in prose) is not
//     matched, and a wrapper's own `skills` frontmatter field must be described
//     without the `skills:` spelling.
//   - Refused VALUES of supported keys (`finalize.gate: ci`, the `github` board
//     token) are out of reach; review catches those.
//   - An unterminated code fence is reported, so it cannot mask the rest of a file.
func TestLivingDocsAlignment(t *testing.T) {
	root := guardRoot(t)
	files, err := livingDocFiles(root, livingDocRoots)
	if err != nil {
		t.Fatalf("%v (fail closed)", err)
	}
	shapes := unsupportedKeyShapes(config.SettingPaths())
	if len(shapes) == 0 {
		t.Fatalf("no unsupported-key shapes derived from the schema registry")
	}
	for _, rel := range files {
		content := readMaintained(t, root, rel)
		for _, v := range scanCitations(rel, content) {
			t.Error(v)
		}
		for _, v := range scanUnsupportedKeys(rel, content, shapes) {
			t.Error(v)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		for _, s := range []string{
			"see change 0363 for", "Changes 0064/0084 split it", "the change #502 work",
			"the change-0365 guard", "a pre-0051 repo", "since 0392 the install",
			"PR #344 landed", "tracked as (#0471).", "fixed in #502.", "see change\n0363",
		} {
			if len(scanCitations("x.md", s)) == 0 {
				t.Errorf("citation shape not flagged: %q", s)
			}
		}
		for _, s := range []string{
			"ADR-0071 decides it", "since 2024", "[step one](#1-capture)",
			"run `docket change show --id 0042`", "an em dash &#8212; here",
			"model `deepseek-v4-flash-0731`", "docs/changes/active/0412-<slug>.md",
			"```\nchange 0042\n```\n",
		} {
			if got := scanCitations("x.md", s); len(got) != 0 {
				t.Errorf("non-citation flagged: %q -> %v", s, got)
			}
		}
		if len(scanCitations("x.md", "intro\n```yaml\nkey: 1\n")) == 0 {
			t.Errorf("unterminated fence not reported")
		}

		for _, s := range []string{
			"bind `skills.build` to", "raise learnings.cap", "set `skills:` to",
			"skills:\n  build: x\n", "  terminal_publish: true", "# auto_groom: false",
			"agents.claude.build-max.runner", "{ model: x, runner: codex }",
			"runners.codex.shim_model", "`dummy_mode.persona`", "build:\n  checkpoint: true\n",
			"the skills.<role> map",
		} {
			if len(scanUnsupportedKeys("x.md", s, shapes)) == 0 {
				t.Errorf("unsupported key not flagged: %q", s)
			}
		}
		for _, s := range []string{
			"auto_groomable: true", "two kinds of skills: workflow and worker",
			"finalize.gate: local", "board_surfaces: [inline]", "agents.claude.status.model",
			"learnings.enabled", "enabled: true", "docs/install/codex.md", "the skills.",
			"see docket-skills.md", "plan: docs/superpowers/plans/x.md",
			"build:\n  gate: local\n", "review:\n  max_fix_tasks: 10\n", "types: [feat]",
		} {
			if got := scanUnsupportedKeys("x.md", s, shapes); len(got) != 0 {
				t.Errorf("supported text flagged: %q -> %v", s, got)
			}
		}
		// Registry-derived population: every unsupported path, spelled as a
		// config key, is caught — so a new unsupported row is guarded with no
		// edit here.
		for _, p := range config.SettingPaths() {
			if p.Supported {
				continue
			}
			spelled := strings.ReplaceAll(p.Path, "*", "x")
			probe := "set `" + spelled + "` here"
			if !strings.Contains(p.Path, ".") {
				probe = spelled + ": x\n"
			}
			if len(scanUnsupportedKeys("x.md", probe, shapes)) == 0 {
				t.Errorf("registry path %s not caught by %q", p.Path, probe)
			}
		}

		tmp := t.TempDir()
		for _, f := range []string{"a/one.md", "a/fixtures/skip.md", "a/testdata/skip.md", "a/notes.txt", "top.md", "b/notes.txt"} {
			if err := os.MkdirAll(filepath.Join(tmp, filepath.Dir(f)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(tmp, f), []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, err := livingDocFiles(tmp, []string{"a", "top.md"})
		if err != nil || strings.Join(got, ",") != "a/one.md,top.md" {
			t.Errorf("livingDocFiles = %v, %v; want [a/one.md top.md]", got, err)
		}
		if _, err := livingDocFiles(tmp, []string{"missing"}); err == nil {
			t.Errorf("a missing root must fail closed")
		}
		if _, err := livingDocFiles(tmp, []string{"b"}); err == nil {
			t.Errorf("a root yielding no files must fail closed")
		}
	})
}

// livingDocRoots lists the living documentation the guard covers: files or
// directories, slash paths relative to the repo root. Doc tasks add their
// pages as they align them.
var livingDocRoots = []string{
	"docs/concepts",
}

// livingDocFiles returns every .md file under roots (a root may be a file),
// sorted. Directories named "fixtures" or "testdata" below a root are pruned.
// A missing root, or a root that yields no .md file, is an error.
func livingDocFiles(base string, roots []string) ([]string, error) {
	var out []string
	for _, r := range roots {
		abs := filepath.Join(base, filepath.FromSlash(r))
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("living doc root %s: %w", r, err)
		}
		before := len(out)
		if !info.IsDir() {
			out = append(out, r)
			continue
		}
		err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if p != abs && (d.Name() == "fixtures" || d.Name() == "testdata") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			rel, rerr := filepath.Rel(base, p)
			if rerr != nil {
				return rerr
			}
			out = append(out, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk living doc root %s: %w", r, err)
		}
		if len(out) == before {
			return nil, fmt.Errorf("living doc root %s holds no .md file", r)
		}
	}
	sort.Strings(out)
	return out, nil
}

var citationShapes = []struct {
	name string
	re   *regexp.Regexp
}{
	{"change N", regexp.MustCompile(`(?i)\bchanges?\s+#?[0-9]{1,4}\b`)},
	{"change-N", regexp.MustCompile(`(?i)\bchange-[0-9]+\b`)},
	{"pre-NNNN", regexp.MustCompile(`(?i)\bpre-0[0-9]{3}\b`)},
	{"since NNNN", regexp.MustCompile(`(?i)\bsince\s+0[0-9]{3}\b`)},
	{"PR #N", regexp.MustCompile(`(?i)\bPRs?\s+#[0-9]+\b`)},
	{"#N", regexp.MustCompile(`(?:^|[^&\w/#])#[0-9]+(?:[^\w-]|$)`)},
}

var codeSpanRe = regexp.MustCompile("`[^`\n]*`")

// maskCode blanks fenced code blocks and inline code spans (keeping line
// structure) and reports whether a fence was left unterminated.
func maskCode(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	inFence, marker := false, ""
	for i, ln := range lines {
		trimmed := strings.TrimLeft(ln, " \t")
		if inFence {
			if strings.HasPrefix(trimmed, marker) {
				inFence = false
			}
			lines[i] = ""
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence, marker = true, trimmed[:3]
			lines[i] = ""
			continue
		}
		lines[i] = codeSpanRe.ReplaceAllStringFunc(ln, func(s string) string { return strings.Repeat(" ", len(s)) })
	}
	return strings.Join(lines, "\n"), inFence
}

func lineOf(content string, idx int) int { return strings.Count(content[:idx], "\n") + 1 }

// scanCitations reports every change/PR citation outside code.
func scanCitations(rel, content string) []string {
	masked, open := maskCode(content)
	var v []string
	if open {
		v = append(v, fmt.Sprintf("%s: unterminated code fence", rel))
	}
	for _, s := range citationShapes {
		for _, m := range s.re.FindAllStringIndex(masked, -1) {
			v = append(v, fmt.Sprintf("%s:%d: %s citation %q — cite no individual change or PR", rel, lineOf(masked, m[0]), s.name, strings.TrimSpace(masked[m[0]:m[1]])))
		}
	}
	return v
}

type keyShape struct {
	name string
	re   *regexp.Regexp
}

const keySegClass = `[A-Za-z0-9_<>*-]+`

// unsupportedKeyShapes derives the key spellings to ban from the registry:
// each unsupported dotted path; for a top-level segment with no supported
// path beneath it, its YAML-key form and any dotted child; and, inside a
// block that also holds supported keys (finalize, learnings, build, agents),
// an unsupported leaf's YAML-key form when that leaf name is no segment of any
// supported path. The last restriction keeps `build:`, `review:`, and the
// change-frontmatter `plan:` (also leaf names under skills.*) from matching.
func unsupportedKeyShapes(paths []config.SettingPath) []keyShape {
	supportedTop, supportedSeg := map[string]bool{}, map[string]bool{}
	for _, p := range paths {
		if p.Supported {
			segs := strings.Split(p.Path, ".")
			supportedTop[segs[0]] = true
			for _, s := range segs {
				supportedSeg[s] = true
			}
		}
	}
	var shapes []keyShape
	seenTop, seenLeaf := map[string]bool{}, map[string]bool{}
	for _, p := range paths {
		if p.Supported {
			continue
		}
		segs := strings.Split(p.Path, ".")
		if len(segs) > 1 {
			parts := make([]string, len(segs))
			for i, s := range segs {
				if s == "*" {
					parts[i] = keySegClass
				} else {
					parts[i] = regexp.QuoteMeta(s)
				}
			}
			shapes = append(shapes, keyShape{p.Path, regexp.MustCompile(`(?:^|[^\w.-])` + strings.Join(parts, `\.`) + `(?:[^\w-]|$)`)})
		}
		top := segs[0]
		if !supportedTop[top] && !seenTop[top] {
			seenTop[top] = true
			q := regexp.QuoteMeta(top)
			shapes = append(shapes, keyShape{top + ":", regexp.MustCompile("(?m)(?:^[ \\t]*(?:#[ \\t]*)?(?:-[ \\t]+)?|`)" + q + ":")})
			if len(segs) > 1 {
				shapes = append(shapes, keyShape{top + ".<child>", regexp.MustCompile(`(?:^|[^\w.-])` + q + `\.` + keySegClass)})
			}
		}
		leaf := segs[len(segs)-1]
		if len(segs) > 1 && supportedTop[top] && leaf != "*" && !supportedSeg[leaf] && !seenLeaf[leaf] {
			seenLeaf[leaf] = true
			shapes = append(shapes, keyShape{leaf + ":", regexp.MustCompile(`(?m)(?:^[ \t]*(?:#[ \t]*)?|[{,][ \t]*)` + regexp.QuoteMeta(leaf) + `:`)})
		}
	}
	return shapes
}

// scanUnsupportedKeys reports every unsupported config key spelled in content
// (code included: config examples live in code).
func scanUnsupportedKeys(rel, content string, shapes []keyShape) []string {
	var v []string
	for _, s := range shapes {
		for _, m := range s.re.FindAllStringIndex(content, -1) {
			v = append(v, fmt.Sprintf("%s:%d: unsupported config key %s (%q) — describe only supported settings", rel, lineOf(content, m[0]), s.name, strings.TrimSpace(content[m[0]:m[1]])))
		}
	}
	return v
}
```

- [ ] **Step 2: Run.**
  Run: `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1 -v`
  Expected: PASS. `docs/concepts` is clean for both classes today; its other defects are
  Tasks 10–11's job.
  - If a `non_vacuity` case fails, the plan-supplied regex is wrong. Fix the regex, not the
    case, unless the case contradicts the spec. If you change any matcher or case, list it in
    your report.
  - If `docs/concepts` reports a real hit, leave the page alone. Remove `docs/concepts` from the
    seed, add only the clean concept files as individual entries, and say so in your report.

- [ ] **Step 3: Mutation checks** (back up the file with `mktemp`, restore with `cp -f`, and
  use `-count=1` throughout):
  - (a) Delete the `"#N"` entry from `citationShapes`. `non_vacuity` reddens on `fixed in #502.`.
  - (b) Make `unsupportedKeyShapes` return `nil`. The main test fatals.
  - (c) Change `maskCode` to never set `inFence`. The fenced `change 0042` case reddens.
  - (d) Append the line `See change 0363.` to `docs/concepts/README.md`. The main test reddens.
    Then restore the page.
  - (e) Append `skills:\n  build: x` to `docs/concepts/README.md`. It reddens. Restore.

  Record each mutation's result in your report.

- [ ] **Step 4: Comment-anchor check.** Run
  `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run 'TestCommentAnchorStyle|TestLivingDocsAlignment' -count=1`.
  Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/repoguard/docs_alignment_test.go
git commit -m "test(repoguard): guard living docs against change citations and unsupported config keys"
```

---

### Task 6: Front door and guide, part 1 (rename the daily loop to the five steps)

**Build tier:** standard

**Files:**
- Modify: `README.md`, `docs/README.md`, `docs/guide/README.md`
- Rename: `docs/guide/daily-loop.md` → `docs/guide/five-steps.md` (use `git mv`), then modify
- Modify: `docs/guide/capturing-work.md`, `docs/guide/designing-before-building.md`,
  `docs/guide/building-without-supervision.md`, `docs/guide/proving-the-build.md`,
  `docs/guide/reviewing-before-the-human.md`
- Modify: `internal/repoguard/docs_alignment_test.go` (`livingDocRoots`)
- Modify: `internal/repoguard/prose_contracts_test.go` (rows on these files, see Step 1)

**Interfaces:** consumes `livingDocRoots` (Task 5). Before editing, read this plan's Global
Constraints and Verified facts. The spec's *Front door* and *docs/guide* worklists are the
source; the bullets below copy them. Treat each bullet as a hypothesis: verify it against the
code first, then fix it, delete it, or note in your report why it was not a defect. After the
listed fixes, re-read each page for further instances of the same defects.

- [ ] **Step 1: Find every pin on these files.** Run
  `grep -rn -e 'README.md' -e 'docs/README' -e 'docs/guide/' internal tests | grep -v '"README.md' | grep -E 'docs/|"README'`
  and also `grep -rn -- 'docs/guide/\|"README.md"' internal/repoguard`. Known pins:
  - `prose_contracts_test.go`:
    - `README.md` must keep "The right model for each step." and the links
      `](docs/README.md)` and `](docs/comparison/ai-native-sdlc-playbook.md)`.
    - `docs/guide/designing-before-building.md` pins `brainstorm: docket-brainstorm`, a durable
      `skills:` binding that this task deletes. Repoint the row to a current clause on the page
      that names `docket-brainstorm-consultant`, or remove the row if the page no longer
      describes the consultant.
    - `docs/guide/capturing-work.md` pins "untyped set can only shrink". That sentence sits in
      the *Migrating to typed changes* section this task deletes. Remove the row, unless a
      current clause still states that every creation path writes a type, in which case repoint
      it there.
  - `license_test.go` keeps README's `## License` section and its links to `LICENSE`, `NOTICE`,
    and `CONTRIBUTING.md`.

- [ ] **Step 2: Add the pages to the guard and watch it redden.** In `livingDocRoots`, add
  `"README.md"`, `"docs/README.md"`, `"docs/guide/README.md"`, `"docs/guide/five-steps.md"`,
  `"docs/guide/capturing-work.md"`, `"docs/guide/designing-before-building.md"`,
  `"docs/guide/building-without-supervision.md"`, `"docs/guide/proving-the-build.md"`,
  `"docs/guide/reviewing-before-the-human.md"`. Do the `git mv docs/guide/daily-loop.md docs/guide/five-steps.md`
  first. Run `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1`.
  Expected: FAIL, listing citations and unsupported keys in these pages.

- [ ] **Step 3: Edit the pages.** Worklist:
  - `README.md`:
    - "no CLI to install" is wrong: docket ships a `docket` binary that `install.sh` installs.
    - Adopting docket in a repository is `docket repository init`, not `migrate`.
    - Rewrite *Status* to state what is supported today: no main-mode opt-out, no list of
      deferred features, no "Go v1".
    - Rename the heading "Install and the daily loop" to "Install and the five steps".
    - The Quickstart link goes to `docs/guide/five-steps.md`.
  - `docs/README.md`: drop the *Workflow roles* and *Delegation* entries (those pages are deleted
    in Task 8). Daily-loop links and wording become the five steps.
  - `docs/guide/README.md`: the daily-loop link and wording become the five steps.
  - `docs/guide/five-steps.md`:
    - Title it "# Using docket: the five steps". The steps are capture, groom when needed,
      build, review and merge, close out.
    - The closing "health checks that flag stale claims … stalled dependencies" becomes the real
      findings (`artifact-missing`, `change-reference-dangling`, `change-dependency-cycle`,
      `branch-malformed`, configuration and parse diagnostics).
    - "the loop grooms, builds, and closes out" becomes "docket grooms, builds, and closes out".
  - `docs/guide/capturing-work.md`:
    - Priorities are critical / high / medium / low.
    - The board is grouped into in-progress, built, blocked, groomed, proposed, deferred. The
      order is configurable with `board.section_order` / `board.sorting`. Every write re-renders
      it; there is no regenerate step.
    - Delete the auto-capture paragraph and its YAML, the `auto_capture.types` claims, the
      "query pseudo-values" claim, and the `--type untyped` / `--type all` / unconfigured-type
      claims (all are refused). Delete the whole *Migrating to typed changes* section.
    - Capture is the `docket-new-change` skill. `docket change create` needs `--request <file>`
      (verify with `go run ./cmd/docket change create --help`).
  - `docs/guide/designing-before-building.md`:
    - The consultant-written spec is a spoken per-run request only; delete the durable `skills:`
      config.
    - "agents not synced" becomes "installed by `docket install`".
    - Delete the whole *Shaping the conversation (dummy_mode)* section, including the persona
      gallery.
    - Delete the line-oriented-config-reader claim.
  - `docs/guide/building-without-supervision.md`:
    - docket requires its own `docket` binary.
    - Build and review default to docket's own skills.
    - Rename the "loop" senses (Global Constraints → Terminology). `/loop` drain wording stays.
  - `docs/guide/proving-the-build.md`:
    - Describe red-suite handling and `build.max_attempts` (Verified facts → Build gate).
    - Delete the `build.checkpoint` bullet and the `ci` / `both` gate values.
    - Delete the per-file budget prose (it describes docket's own contributor suite, which
      `tests/README.md` covers) and "in this repository both resolve…".
  - `docs/guide/reviewing-before-the-human.md`:
    - Delete the `skills: review:` opt-out.
    - The tier mapping is same-level (Verified facts → Review).
    - Delete "restores the older … escape hatch".
    - Follow-ups go to the final report.

  Across all nine pages:
  - Remove every change/PR citation.
  - Apply the loop terminology.
  - Delete removed features and old names.
  - Check every `docket …` command you keep against `go run ./cmd/docket <verb> --help`.

- [ ] **Step 4: Grep for leftover vocabulary** (these are hints; resolve or justify each hit):

```bash
/usr/bin/grep -nEi -e 'loop' -e 'daily' -e 'Go v1' -e 'sync-agents' -e 'board-refresh' -e 'stale[- ]claim' -e 'stalled' -e 'harvest' -e 'terminal[ _-]?publish' -e 'delegat' -e 'dummy' -e 'auto-capture' -e 'runner' -e 'main-mode' -e 'single-branch' -e 'metadata_branch' -e 'workflow-roles' -e 'daily-loop' README.md docs/README.md docs/guide/README.md docs/guide/five-steps.md docs/guide/capturing-work.md docs/guide/designing-before-building.md docs/guide/building-without-supervision.md docs/guide/proving-the-build.md docs/guide/reviewing-before-the-human.md
```

  `/loop` itself is fine. So is "single-branch" in the README's description of what `migrate`
  converts.

- [ ] **Step 5: Run the guard and the pinned tests.**
  `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run 'TestLivingDocsAlignment|TestProseContracts|TestLicenseReadmeSection' -count=1`
  Expected: PASS. (Use `grep -n "^func Test" internal/repoguard/prose_contracts_test.go` for the
  exact prose-contract test names and include them all.)

- [ ] **Step 6: Inbound links.** Run `grep -rn -- 'daily-loop' README.md docs/README.md docs/guide docs/install docs/concepts docs/reference`.
  Expected: no hits.

- [ ] **Step 7: Commit.**

```bash
git add README.md docs/README.md docs/guide/README.md docs/guide/five-steps.md docs/guide/capturing-work.md docs/guide/designing-before-building.md docs/guide/building-without-supervision.md docs/guide/proving-the-build.md docs/guide/reviewing-before-the-human.md internal/repoguard/docs_alignment_test.go internal/repoguard/prose_contracts_test.go
git commit -m "docs: README and guide part 1 describe the binary; the daily loop becomes the five steps"
```

---

### Task 7: Guide, part 2 (landing, backlog, learnings, metadata)

**Build tier:** standard

**Files:**
- Modify: `docs/guide/landing-changes.md`, `docs/guide/keeping-the-backlog-honest.md`,
  `docs/guide/remembering-why.md`, `docs/guide/where-the-metadata-lives.md`
- Modify: `internal/repoguard/docs_alignment_test.go` (`livingDocRoots`: collapse every
  `docs/guide/…` entry into the single entry `"docs/guide"`)
- Modify: `internal/repoguard/prose_contracts_test.go` if a pinned phrase moves

**Interfaces:** consumes `livingDocRoots`. Read Global Constraints and Verified facts first.
Treat each bullet as a hypothesis to verify against the code.

- [ ] **Step 1: Find pins.** `grep -rn -- 'docs/guide/landing-changes\|keeping-the-backlog-honest\|remembering-why\|where-the-metadata-lives' internal tests`.
  Known: `landing-changes.md` must keep "auto-mode classifier" (`test_readme_finalize_docs`).
  Keep the clause if it still describes current behaviour; otherwise repoint the row.

- [ ] **Step 2: Extend the guard and watch it redden.** Replace every `docs/guide/…` entry in
  `livingDocRoots` with `"docs/guide"`. Run
  `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1`.
  Expected: FAIL on these four pages.

- [ ] **Step 3: Edit.** Worklist:
  - `landing-changes.md`:
    - Delete *Selective publish on close-out* and the retired bot-approval paragraph.
    - The merge method order is rebase, merge commit, squash: the first one the repository
      permits.
    - Add to *When finalize is blocked* the repair sign-off block: an autonomously authored
      repair halts with `finalize block --reason repair-needs-signoff`, and a human clears it
      with `finalize clear-block`.
    - "both halves of the daily loop" becomes "both drains".
  - `keeping-the-backlog-honest.md`:
    - Rewrite the page around read-only `docket status` + `docket maintenance sweep`, the real
      health findings, the reclaim rules (`reclaim.auto`, `reclaim.lease_ttl`,
      `reclaim-auto-disabled`; expired lease, no feature branch, no workspace), and halted-run
      recovery (`docket change resume-halted`,
      `docket run start implement-next --resume <id>`).
    - Delete *Supported modes and the bootstrap guard*'s mode framing.
    - A new repository runs `docket repository init`.
  - `remembering-why.md`:
    - Learnings are written with `docket learning record` / `docket learning update`.
    - Delete close-out harvest, the generated index, the curation cap, and promotion proposals.
  - `where-the-metadata-lives.md`:
    - Rewrite around the one layout (the `docket` branch through `.docket/`) and
      `repository init` / `migrate` / `check` / `repair` / `configure-tests` (verify each with
      `go run ./cmd/docket repository <verb> --help`).
    - Delete every terminal-publish passage and table row, *Publishing archived records*,
      *Single-branch mode: the opt-out*, the allow-rule remnant, and *Carrying a pre-0051 repo
      forward*.
    - The `.gitignore` managed block is written by init/migrate.
    - `migrate` uses no worktree.

  Across all four pages: remove citations, apply the loop terminology, and verify every kept
  command.

- [ ] **Step 4: Grep for leftover vocabulary.** Run the same command as Task 6 Step 4 (copied
  here) over the four pages:

```bash
/usr/bin/grep -nEi -e 'loop' -e 'daily' -e 'Go v1' -e 'sync-agents' -e 'board-refresh' -e 'stale[- ]claim' -e 'stalled' -e 'harvest' -e 'terminal[ _-]?publish' -e 'delegat' -e 'dummy' -e 'auto-capture' -e 'runner' -e 'main-mode' -e 'single-branch' -e 'metadata_branch' -e 'publish' docs/guide/landing-changes.md docs/guide/keeping-the-backlog-honest.md docs/guide/remembering-why.md docs/guide/where-the-metadata-lives.md
```

  `finalize publish` (pushing the rebased head) is a current command, so keep it. Every
  close-out "publish" of archived records goes.

- [ ] **Step 5: Run.** `timeout --kill-after=10s 10m go test ./internal/repoguard/ -count=1`
  Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add docs/guide/landing-changes.md docs/guide/keeping-the-backlog-honest.md docs/guide/remembering-why.md docs/guide/where-the-metadata-lives.md internal/repoguard/docs_alignment_test.go internal/repoguard/prose_contracts_test.go
git commit -m "docs(guide): landing, backlog, learnings, and metadata pages describe the binary"
```

---

### Task 8: Install pages, part 1 (core pages, delete workflow-roles and delegation)

**Build tier:** standard

**Files:**
- Modify: `docs/install/README.md`, `docs/install/install.md`, `docs/install/keeping-current.md`,
  `docs/install/global-config.md`, `docs/install/config-layers.md`,
  `docs/install/models-and-effort.md`
- Delete: `docs/install/workflow-roles.md`, `docs/install/delegating-across-harnesses.md`
- Modify: `docs/reference/skills-and-agents.md` (gains the default-roles table)
- Modify: `docs/install/opencode.md`, but only the inbound link to the deleted delegation page
  (Task 9 rewrites the rest of that page)
- Modify: `internal/repoguard/docs_alignment_test.go` (`livingDocRoots`)
- Modify: `internal/repoguard/prose_contracts_test.go` (rows on these files)

**Interfaces:** consumes `livingDocRoots`. Read Global Constraints and Verified facts first.

- [ ] **Step 1: Find pins.** `grep -rn -- 'docs/install/' internal tests`. Known pins:
  - `models-and-effort.md` must keep "completed (forked execution)" (`test_skill_fork_dispatch`)
    and "Fork-exclusion principle" (`test_readme_finalize_docs`).
  - `install.md` section contracts:
    - `TestUninstallCollectionDocContracts` (`uninstallDocContracts`) needs the headings
      `## Uninstalling docket`, `## Reclaiming old version trees`, and
      `## Adopting docket in a repository`, plus three clauses.
    - `TestInstallPrerequisiteDocContracts` needs `## What you need first`,
      `## Install docket on your machine`, and two coreutils clauses.

  Keep those headings and clauses verbatim unless one is false. If it is false, fix the row in
  the same commit.

- [ ] **Step 2: Move the default-roles table and delete the two pages.** Copy the role → default
  skill table from `docs/install/workflow-roles.md` into `docs/reference/skills-and-agents.md`
  under a heading such as `## Default workflow roles`. The table states the fixed defaults:
  brainstorm `superpowers:brainstorming`, plan `superpowers:writing-plans`, build `docket-build`,
  review `docket-review`, finish `superpowers:finishing-a-development-branch`. Verify them
  against `skills/docket-convention/SKILL.md` *Skill layer* (read only). Drop any rebinding
  prose. The `test_readme_skill_catalog` row needs `## Skills` to stay in that file and
  `#the-eight-skills` to stay absent. Then
  `git rm docs/install/workflow-roles.md docs/install/delegating-across-harnesses.md`.

- [ ] **Step 3: Fix inbound links** to the two deleted pages in `docs/install/README.md`,
  `docs/install/install.md`, `docs/install/config-layers.md`, and `docs/install/opencode.md`.
  `docs/README.md` was already handled in Task 6; verify with
  `grep -rn -- 'workflow-roles\|delegating-across-harnesses' README.md docs/README.md docs/guide docs/install docs/concepts docs/reference`
  (expected: no hits after this step). A link to the roles table points at
  `../reference/skills-and-agents.md#default-workflow-roles`.

- [ ] **Step 4: Extend the guard and watch it redden.** Add `"docs/install/README.md"`,
  `"docs/install/install.md"`, `"docs/install/keeping-current.md"`, `"docs/install/global-config.md"`,
  `"docs/install/config-layers.md"`, `"docs/install/models-and-effort.md"` to `livingDocRoots`.
  Run `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1`.
  Expected: FAIL on these pages.

- [ ] **Step 5: Edit.** Worklist:
  - `README.md`:
    - Drop the *Workflow roles* and *Delegation* entries and the opencode "auto-approve grant"
      wording.
    - Fix the global-config bullet: the global config does not enable a harness.
  - `install.md`:
    - The harness roots are the four supported harnesses; no `.kiro` / `.windsurf`.
    - The default roles are as in Verified facts.
    - `install.sh` flags are `--bin-dir` / `--harness` (verify in `install.sh`).
    - Nothing writes `config.yml`.
    - A repository enables a harness with `agent_harnesses` in `.docket.yml`.
    - Delete the stale project-level-wrapper callout.
    - Under *Adopting docket in a repository*, a repository that never used docket runs
      `docket repository init`; a legacy single-branch repository runs `migrate`.
  - `keeping-current.md`: delete the managed-global-config bullet and refresh the example tag
    (use the latest tag from `git tag --sort=-v:refname`).
  - `global-config.md`:
    - Say what belongs there: agent model/effort pins and the other any-layer keys.
    - Delete the `skills:` default binding, the installer-writes-config claims, the global
      `agent_harnesses` instructions, and the `agents.yaml` migration.
  - `config-layers.md`:
    - Describe the layers and precedence as in Verified facts.
    - Delete the `skills:` / repo-layer `agents:` merge prose, the `runtime.bash` paragraph,
      `metadata_branch` and the mode opt-out, and *Migrating from agents.yaml*.
    - The shared-setting guard covers the four repository keys.
    - A malformed layer invalidates the whole configuration.
    - The `.gitignore` block is written by init/migrate.
    - Point to `docket diagnostic config --repo-dir .`.
  - `models-and-effort.md`:
    - Pins live in the global config only.
    - The dev install command is `docket development install --source <dir>`.
    - List the wrapper roots, with opencode under `${XDG_CONFIG_HOME:-~/.config}`.
    - Delete per-repository wrapper generation, "Generated per-repo agent files…", the retired
      clone-identical note, and the `install check` CI-gate claims.
    - Keep the two pinned phrases.

  Across all pages: remove citations, apply the loop terminology, and verify every kept command
  (`go run ./cmd/docket install --help`, `install check --help`,
  `development install --help`, `diagnostic config --help`).

- [ ] **Step 6: Grep for leftover vocabulary.**

```bash
/usr/bin/grep -nEi -e 'loop' -e 'daily' -e 'Go v1' -e 'sync-agents' -e 'agents\.yaml' -e 'kiro' -e 'windsurf' -e 'runtime' -e 'bash' -e 'delegat' -e 'runner' -e 'main-mode' -e 'metadata_branch' -e 'skills:' -e 'per-repo' -e 'CI gate' docs/install/README.md docs/install/install.md docs/install/keeping-current.md docs/install/global-config.md docs/install/config-layers.md docs/install/models-and-effort.md docs/reference/skills-and-agents.md
```

  (`bash` legitimately appears in a `curl … | bash` install line; judge each hit.)

- [ ] **Step 7: Run.** `timeout --kill-after=10s 10m go test ./internal/repoguard/ -count=1`
  Expected: PASS.

- [ ] **Step 8: Commit.**

```bash
git add docs/install/README.md docs/install/install.md docs/install/keeping-current.md docs/install/global-config.md docs/install/config-layers.md docs/install/models-and-effort.md docs/install/opencode.md docs/reference/skills-and-agents.md internal/repoguard/docs_alignment_test.go internal/repoguard/prose_contracts_test.go
git commit -m "docs(install): core install pages describe the binary; drop workflow-roles and delegation pages"
```

(The `git rm` in Step 2 already staged the deletions.)

---

### Task 9: Harness install pages and the harness reference

**Build tier:** standard

**Files:**
- Modify: `docs/install/claude-code.md`, `docs/install/codex.md`, `docs/install/cursor.md`,
  `docs/install/opencode.md`
- Delete: `docs/reference/harness/validation-runbook.md`, the whole
  `docs/reference/harness/fixtures/` directory (`nested-launch/*`),
  `docs/reference/harness/permissions.example.json`, `docs/reference/harness/sandbox.example.json`
- Modify: `docs/reference/harness/README.md`, `docs/reference/harness/validation.md`
- Modify: `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/root_entry_dispatch_test.go`
- Modify: `internal/repoguard/docs_alignment_test.go` (`livingDocRoots`)

**Interfaces:** consumes `livingDocRoots`. Read Global Constraints and Verified facts first.

- [ ] **Step 1: Pins on these files.**
  - `prose_contracts_test.go`:
    - `test_cursor_permissions_docs` on `docs/install/cursor.md` needs
      `permissions.example.json` and `](../reference/harness/`. The example JSON is deleted, so
      repoint the row to the inline `{"terminalAllowlist": ["docket"]}` fragment that stays on
      the page. If the page still links the harness reference, keep `](../reference/harness/`.
    - `test_codex_runbook` and `change_0393_feature_child_entry` target the deleted runbook.
      Remove both rows. The metadata-child native-dispatch invariant stays covered by the
      `docs/install/codex.md` row in `TestCodexLaunchMatrixOperatorProse`
      ("unmarked metadata-scoped ordinary child →\nnative named-agent dispatch").
    - `test_cursor_contract_docs` on `validation.md` needs `## The PR-handoff obligation`. Keep
      that heading.
  - `root_entry_dispatch_test.go` `TestCodexLaunchMatrixOperatorProse`:
    - Remove the `docs/reference/harness/validation-runbook.md` entry.
    - **Don't rewrap** the `docs/install/codex.md` paragraph whose three clauses the test pins
      with their exact line breaks
      (`"`[docket launch: root-coordinator]` → foreground `agent.enter` at the\ncaller's cwd"`,
      the `[docket worktree: feature]` clause, and the `unmarked metadata-scoped ordinary child →\nnative named-agent dispatch`
      clause). Edit around them.

- [ ] **Step 2: Delete the files.**
  `git rm -r docs/reference/harness/validation-runbook.md docs/reference/harness/fixtures docs/reference/harness/permissions.example.json docs/reference/harness/sandbox.example.json`

- [ ] **Step 3: Extend the guard and watch it redden.** Add `"docs/install/claude-code.md"`,
  `"docs/install/codex.md"`, `"docs/install/cursor.md"`, `"docs/install/opencode.md"`, then
  collapse every `docs/install/…` entry into `"docs/install"`. Also add
  `"docs/reference/harness"`. Run
  `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1`.
  Expected: FAIL on these pages.

- [ ] **Step 4: Edit.** Worklist:
  - `claude-code.md`: add what install writes for Claude: user-level skills and agents, and,
    with `agent_harnesses` including `claude`, the managed dispatch block in `CLAUDE.md`.
  - `codex.md`:
    - Wrappers are user-level only, and pins are global-only.
    - Fix the opt-in table and the gotcha per Verified facts → `agent_harnesses`.
    - Delete the upgrade note, the `skills:` binding remark (including "flipping a workflow's
      `skills:` binding to `auto`"), "delegation prose", "install or sync", and the
      "(change 0351)" citation.
    - Drop the link to the deleted Codex runbook and to `nested-launch`.
    - Don't rewrap the pinned paragraph.
  - `cursor.md`:
    - Delete "delegation target", the allowlist bullets about status publishing, terminal
      publish, the GitHub mirror, and `cleanup-feature-branch`.
    - Bare `preflight` becomes `docket repository prepare` / `docket maintenance preflight`.
    - `docket board-refresh` / `docket env` become real commands; check
      `go run ./cmd/docket --help`.
    - Correct the "no run operation" remark: `docket run` is the run tracker, not an exec.
    - Keep the inline `{"terminalAllowlist": ["docket"]}` fragment. Delete the pointer to the
      example JSON copies and the sandbox read-path fragment.
    - Add what install writes for Cursor: user-level agents and, with `cursor` opted in,
      `.cursor/rules/docket-dispatch.mdc`.
  - `opencode.md`:
    - The wrapper and skills roots are under `${XDG_CONFIG_HOME:-~/.config}/opencode/`, with
      17 agents.
    - Pins are global-only, and the effort drop is silent.
    - Delete *Delegating Claude Code agents to opencode* and its permissions table, and the
      "(change 0351)" citation.
  - `docs/reference/harness/README.md`: list only what remains (`validation.md`); no links to
    deleted files.
  - `docs/reference/harness/validation.md`:
    - `sync-agents.sh` becomes `docket install --harness cursor`.
    - Delete the nonexistent test path.
    - There are 17 wrappers.
    - The default build role is always `docket-build`, so Phase 7 always applies. Retitle it
      without `skills.build`.
    - Correct the pinned-model exception sentence.
    - Phase 6 describes docket-build's tiers, not subagent-driven development.
    - Delete the "(change 0351)" citation.
    - Line 74's "no `skills:` key" (wrapper frontmatter) must be reworded without the
      `skills:` spelling, for example "no `skills` field". The guard bans that spelling.

  Verify every wrapper root and generated path against `internal/harness/*` (for example
  `grep -rn 'agents' internal/harness/opencode/*.go`) and `go run ./cmd/docket install --help`.

- [ ] **Step 5: Grep for leftover vocabulary.**

```bash
/usr/bin/grep -nEi -e 'loop' -e 'Go v1' -e 'sync-agents' -e 'runbook' -e 'nested-launch' -e 'example\.json' -e 'delegat' -e 'runner' -e 'board-refresh' -e 'docket env' -e 'terminal' -e 'mirror' -e 'per-repo' -e 'sdd' -e 'subagent-driven' docs/install/claude-code.md docs/install/codex.md docs/install/cursor.md docs/install/opencode.md docs/reference/harness/README.md docs/reference/harness/validation.md
```

  `terminalAllowlist` is the kept Cursor key.

- [ ] **Step 6: Run.** `timeout --kill-after=10s 10m go test ./internal/repoguard/ -count=1`
  Expected: PASS.

- [ ] **Step 7: Commit.**

```bash
git add docs/install/claude-code.md docs/install/codex.md docs/install/cursor.md docs/install/opencode.md docs/reference/harness/README.md docs/reference/harness/validation.md internal/repoguard/prose_contracts_test.go internal/repoguard/root_entry_dispatch_test.go internal/repoguard/docs_alignment_test.go
git commit -m "docs(install): harness pages describe the binary; drop the Codex runbook and stale Cursor examples"
```

---

### Task 10: Concepts, part 1 (README, build tiers, lifecycle, config layers, memory, reconcile)

**Build tier:** standard

**Files:**
- Modify: `docs/concepts/README.md`, `docs/concepts/build-tiers-and-gate.md`,
  `docs/concepts/change-lifecycle.md`, `docs/concepts/config-layers.md`,
  `docs/concepts/memory.md`, `docs/concepts/reconcile.md`

**Interfaces:** `docs/concepts` is already in `livingDocRoots` (Task 5), so the guard covers these
pages throughout. Read Global Constraints and Verified facts first.

- [ ] **Step 1: Pins.** `grep -rn -- 'docs/concepts' internal tests`. Expected: none besides the
  guard root. If you find any, keep them true in this commit.

- [ ] **Step 2: Apply the `## Decided in` rule to every page in this task.**
  - The sections stay. Drop a bullet when its ADR's decision is removed or refused behaviour.
    At least these go: ADR-0002 (superseded), ADR-0005 (close-out harvest), ADR-0018 (pluggable
    skill rebinding), ADR-0045 (auto-capture), ADR-0046 (destructive reset precondition),
    ADR-0051 and ADR-0090 (publish-deferred marker), and ADR-0080 (detached delegation).
  - Judge every other bullet the same way: read the ADR's *Decision* in `docs/adrs/` (on the
    metadata branch at `/Users/homer/dev/docket/.docket/docs/adrs/` for numbers above 0096).
  - Reword a kept bullet that describes its ADR in retired terms.
  - A relative ADR link must resolve in this worktree's `docs/adrs/`. Cite a higher ADR as
    plain text.
  - `docs/concepts/README.md` keeps its four-section promise.

- [ ] **Step 3: Edit.** Worklist:
  - `README.md`: the finalize summary includes publish and cleanup.
  - `build-tiers-and-gate.md`:
    - Evidence is a PR-body record and is never committed.
    - Per-file budgets are not the build gate's.
    - Add `build.gate: off` (runs nothing; records `skipped` / `build-gate-off`) and the
      empty-command halt (remedy `docket repository configure-tests`).
  - `change-lifecycle.md`:
    - Say how `done` is reached: `docket finalize closeout` after proving the merge, with the
      maintenance sweep as the safety net.
    - Dependencies are satisfied at `done`.
    - State the stacked base rule.
    - Delete best-effort follow-up capture.
    - The diagram includes `blocked`, `deferred`, `killed`, and `stacked-merged`. Verify the
      status set against `internal/domain` (`grep -rn '"stacked-merged"' internal/domain`).
  - `config-layers.md`:
    - Delete the bash and `metadata_branch` rationale and the "external objects" invariant.
    - Model pins are global-only, and a malformed layer invalidates the configuration.
    - Give the global path and say the built-in table is compiled in.
    - Add how to inspect the resolved configuration (`docket diagnostic config --repo-dir .`).
  - `memory.md`:
    - Learnings are written by `docket learning record` / `update`.
    - Delete close-out harvest.
    - Scope the comment-anchor invariant to docket's own source, or drop it.
  - `reconcile.md`: only the `Decided in` judgement.

  Across all pages: apply the loop terminology and delete removed features.

- [ ] **Step 4: Grep for leftover vocabulary.**

```bash
/usr/bin/grep -nEi -e 'loop' -e 'Go v1' -e 'bash' -e 'metadata_branch' -e 'harvest' -e 'promot' -e 'committed evidence' -e 'budget' -e 'follow-up' -e 'external object' docs/concepts/README.md docs/concepts/build-tiers-and-gate.md docs/concepts/change-lifecycle.md docs/concepts/config-layers.md docs/concepts/memory.md docs/concepts/reconcile.md
```

- [ ] **Step 5: Run.** `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1`
  Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add docs/concepts/README.md docs/concepts/build-tiers-and-gate.md docs/concepts/change-lifecycle.md docs/concepts/config-layers.md docs/concepts/memory.md docs/concepts/reconcile.md
git commit -m "docs(concepts): lifecycle, build gate, config layers, and memory describe the binary"
```

---

### Task 11: Concepts, part 2 (finalize, run tracker, dispatch, two branches)

**Build tier:** standard

**Files:**
- Modify: `docs/concepts/finalize-sequencer.md`, `docs/concepts/run-tracker.md`,
  `docs/concepts/skills-agents-dispatch.md`, `docs/concepts/two-branches.md`

**Interfaces:** `docs/concepts` is already guarded. Read Global Constraints and Verified facts
first.

- [ ] **Step 1: Apply the `## Decided in` rule.** The rule is the same as in Task 10, copied
  here:
  - Drop a bullet whose ADR decision is removed or refused behaviour. At least these go:
    ADR-0002, ADR-0005, ADR-0018, ADR-0045, ADR-0046, ADR-0051, ADR-0090, ADR-0080.
  - Judge the others by reading the ADR's *Decision*.
  - Reword kept bullets that use retired terms.
  - Relative ADR links must resolve in this worktree's `docs/adrs/`; cite a higher ADR as
    plain text.

- [ ] **Step 2: Edit.** Worklist:
  - `finalize-sequencer.md`:
    - Redraw the sequence: rebase onto the effective base (conflict → resolver), retest (red →
      repair capped by `finalize.repair_max_attempts`), `finalize publish`, merge (rebase,
      merge commit, squash), closeout (archive on the `docket` branch, retarget backlinks),
      cleanup.
    - Include `finalize retarget-children`, the repair sign-off block (`finalize block --reason
      repair-needs-signoff` / `finalize clear-block`), `finalize.gate: off`, the no-op-rebase
      suite skip, and `final-backlink-pending` repaired by `finalize cleanup`.
    - Delete archive-to-integration-branch and `## Publish deferred`.
    - Verify each verb with `go run ./cmd/docket finalize --help`.
  - `run-tracker.md`:
    - Delete "launches, then observes" and history framing ("no longer", "as before").
    - Cover the verdict lines and exit codes from Verified facts → Run tracker. `run-continue`
      also authorizes re-dispatch.
    - Add the resume refusals (`resume-active-run`, `cancellation-pending`,
      `resume-replacement-reserved`), the cancel dispositions, `run-untracked`, and
      `run verify`.
    - Verify with `go run ./cmd/docket run start --help`, `run verdict --help`,
      `run cancel --help`.
  - `skills-agents-dispatch.md`:
    - Skills are fixed default roles (Verified facts → Default workflow roles).
    - Wrappers come from the built-in table plus the global config.
    - The dispatch return is not the only channel: ADR, status, and plan-writer results arrive
      as git state.
    - Fix the fallback description and the precedence diagram: no repository layer for agent
      pins.
    - If you describe a wrapper's preload frontmatter, call it the `skills` field. Never write
      `skills:`, which the guard bans.
  - `two-branches.md`:
    - Delete archived-record copying, "Docket-mode is the default", and the destructive-reset
      invariant.
    - `.docket/` paths follow `changes_dir` / `adrs_dir`.
    - A fresh repository is refused, with `docket repository init` as the remedy.

  Across all pages: apply the loop terminology.

- [ ] **Step 3: Grep for leftover vocabulary.**

```bash
/usr/bin/grep -nEi -e 'loop' -e 'Go v1' -e 'no longer' -e 'as before' -e 'launches' -e 'publish deferred' -e 'archive.*integration' -e 'docket-mode' -e 'reset' -e 'rebind' -e 'skills\.' -e 'delegat' docs/concepts/finalize-sequencer.md docs/concepts/run-tracker.md docs/concepts/skills-agents-dispatch.md docs/concepts/two-branches.md
```

- [ ] **Step 4: Run.** `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1`
  Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add docs/concepts/finalize-sequencer.md docs/concepts/run-tracker.md docs/concepts/skills-agents-dispatch.md docs/concepts/two-branches.md
git commit -m "docs(concepts): finalize, run tracker, dispatch, and two-branch pages describe the binary"
```

---

### Task 12: Reference pages (not the glossary) and the release acceptance checklist

**Build tier:** standard

**Files:**
- Modify: `docs/reference/README.md`, `docs/reference/cli.md`, `docs/reference/config-keys.md`,
  `docs/reference/outcomes.md`, `docs/reference/skills-and-agents.md`, `docs/reference/fields.md`
- Modify: `docs/release/four-harness-acceptance.md` (fix in place; not guarded)
- Modify: `internal/repoguard/docs_alignment_test.go` (`livingDocRoots`)

**Interfaces:** consumes `livingDocRoots`. Read Global Constraints and Verified facts first.

- [ ] **Step 1: Pins.** `grep -rn -- 'docs/reference/\(cli\|config-keys\|outcomes\|skills-and-agents\|fields\|README\)\|four-harness-acceptance' internal tests`.
  Known: `skills-and-agents.md` keeps `## Skills` and must not contain `#the-eight-skills`.

- [ ] **Step 2: Extend the guard and watch it redden.** Add `"docs/reference/README.md"`,
  `"docs/reference/cli.md"`, `"docs/reference/config-keys.md"`, `"docs/reference/outcomes.md"`,
  `"docs/reference/skills-and-agents.md"`, `"docs/reference/fields.md"`. Run
  `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1`.
  Expected: FAIL on `config-keys.md` at least.

- [ ] **Step 3: Edit.** Worklist:
  - `cli.md`:
    - Add `docket agent`.
    - `docket repository` also prepares, repairs, configures tests, and syncs the integration
      branch.
    - `docket run` is not read-only. Mirror the new short help, "Track dispatched runs and
      verify a change's claim-to-implemented run" (Task 1).
    - Fix either the "never lists flags" intro or the flag mentions so the two agree.
    - Check the verb list against `go run ./cmd/docket --help`.
  - `config-keys.md`:
    - List supported keys only, per Verified facts, including the missing `run` block and the
      finalize attempt caps.
    - Delete the `local-only` scope tag and every unsupported row (including `skills`).
    - Describe `agent_harnesses` as the dispatch opt-in.
    - Remove change citations.
    - Read each default from `internal/config/schema.go`.
  - `outcomes.md`: the result and disposition vocabulary from Verified facts → Outcomes. Verify
    with `go run ./cmd/docket schema --help` and the `result` enum in Go
    (`grep -rn '"unsupported-config"' internal/app`).
  - `skills-and-agents.md`: confirm the default-roles table from Task 8 is accurate and that the
    page names 17 agents; fix any stale rebinding wording.
  - `README.md` and `fields.md`: remove citations and stale claims, if any.
  - `docs/release/four-harness-acceptance.md`:
    - Remove the change citations and the "change 0317 checklist" framing.
    - `docket install claude` becomes `docket install --harness claude`.
    - Delete the runner shim / runner fallback wording.
    - This file stays out of the guard.

- [ ] **Step 4: Grep for leftover vocabulary.**

```bash
/usr/bin/grep -nEi -e 'loop' -e 'Go v1' -e 'read-only' -e 'local-only' -e 'skills' -e 'runner' -e 'shim' -e 'change 0' docs/reference/README.md docs/reference/cli.md docs/reference/config-keys.md docs/reference/outcomes.md docs/reference/skills-and-agents.md docs/reference/fields.md docs/release/four-harness-acceptance.md
```

  `skills` legitimately appears on `skills-and-agents.md` as a noun. Only config-key uses of it
  go.

- [ ] **Step 5: Run.** `timeout --kill-after=10s 10m go test ./internal/repoguard/ -count=1`
  Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add docs/reference/README.md docs/reference/cli.md docs/reference/config-keys.md docs/reference/outcomes.md docs/reference/skills-and-agents.md docs/reference/fields.md docs/release/four-harness-acceptance.md internal/repoguard/docs_alignment_test.go
git commit -m "docs(reference): CLI, config keys, outcomes, and acceptance checklist describe the binary"
```

---

### Task 13: Glossary

**Build tier:** standard

**Files:**
- Modify: `docs/reference/glossary.md` (about 2,500 lines)
- Modify: `internal/repoguard/docs_alignment_test.go` (`livingDocRoots`: collapse every
  `docs/reference/…` entry, including `docs/reference/harness`, into `"docs/reference"`)

**Interfaces:** consumes `livingDocRoots`. Read Global Constraints and Verified facts first.
`internal/repoguard/gatedrive_task_absence_test.go` names `docs/reference/glossary.md` only as an
excluded-corpus probe, so no wording is pinned there. Confirm with
`grep -rn glossary internal tests`.

- [ ] **Step 1: Extend the guard and watch it redden.** Collapse the reference entries into
  `"docs/reference"`. Run `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1`.
  Expected: FAIL with roughly 4 citation hits and about 20 key hits in `glossary.md`.

- [ ] **Step 2: Edit.** Worklist:
  - Delete the *Obsolete terms* section, its TOC item, and its index lines.
  - Delete the tombstone entries: *Docket-mode / single-branch mode*, *GitHub board mirror /
    github_project*, and the `launch-unconfirmed` tail of *worktree-busy*. Delete the old-name
    index aliases (*Change version / entity version*, *Selective publish*).
  - Remove every change citation.
  - Rewrite or delete every entry that presents a refused or inert setting:
    - the *Auto-groom* repo knob (the per-change `auto_groomable` field stays)
    - *Workflow role* rebinding
    - the `skills.*` mentions under *docket-build* / *docket-review* / *docket-brainstorm*
    - *Dispatch fallbacks*' explicit-`auto` path
    - *Observation budget*'s delegation half (`delegation_observation_budget`)
    - *Finalize gate*'s `ci`/`both`
    - *Auto-capture*, *build.checkpoint*, *Dummy mode*, *finalize.skip_results_only_delta*,
      *learnings.cap*
    - the `local-only` scope tag
    - *Reserved type tokens*' scalar remark
    - *Learnings index*'s deferral note
  - Correct these entries:
    - *Feature branch*: status records carry no branch or PR.
    - *Finding / finding code*.
    - *Result / disposition* (Verified facts → Outcomes).
    - *Install check*: machine-only, not a CI gate.
    - *Managed global config* / *Keeping docket current*: nothing manages the global file.
    - *Tri-state verdict / halt exit code*: exit codes 0/1/2; a halted gate run exits like a
      failed one.
    - *Marker section*.
    - *docket-status*: read-only status plus `maintenance sweep` on refresh.
    - *Worktree / feature workspace*: "identity check" becomes worktree-changed; verify the
      finding name with `grep -rn 'worktree-changed' internal`.
    - *Diagnostic runtime*.
  - Rename *Fix loop* to **Fix pass** (it runs once: findings become fix tasks, then one
    full-suite run). Update its index line and every in-page anchor (`#fix-loop` →
    `#fix-pass`), and grep the other in-scope docs for `glossary.md#fix-loop`.
  - Apply the loop terminology everywhere else in the page.

- [ ] **Step 3: Grep for leftover vocabulary.**

```bash
/usr/bin/grep -nEi -e 'obsolete' -e 'loop' -e 'Go v1' -e 'docket-mode' -e 'single-branch' -e 'github_project' -e 'launch-unconfirmed' -e 'entity version' -e 'selective publish' -e 'delegat' -e 'runner' -e 'dummy' -e 'auto-capture' -e 'checkpoint' -e 'local-only' -e 'harvest' -e 'identity check' -e 'sync-agents' docs/reference/glossary.md
```

  Also check that every in-page anchor link resolves:
  `/usr/bin/grep -noE '\]\(#[^)]+\)' docs/reference/glossary.md`. Spot-check each one against a
  heading.

- [ ] **Step 4: Run.** `timeout --kill-after=10s 10m go test ./internal/repoguard/ -count=1`
  Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add docs/reference/glossary.md internal/repoguard/docs_alignment_test.go
git commit -m "docs(glossary): only current terms; fix loop becomes fix pass"
```

---

### Task 14: Close the guard to the canonical roots, set the floor, and check every link

**Build tier:** standard

**Files:**
- Modify: `internal/repoguard/docs_alignment_test.go`
- Modify: any in-scope doc a residual hit or broken link names

**Interfaces:** consumes everything above.

- [ ] **Step 1: Close the root list.** Set exactly:

```go
var livingDocRoots = []string{
	"README.md",
	"docs/README.md",
	"docs/guide",
	"docs/install",
	"docs/concepts",
	"docs/reference",
}
```

  and update its doc comment to "the living documentation the guard covers. `docs/release/` is
  deliberately excluded (dated release evidence)." Remove the "Doc tasks add their pages"
  sentence.

- [ ] **Step 2: Add the population floor.** In `TestLivingDocsAlignment`, right after
  `livingDocFiles`, temporarily `t.Logf("%d files", len(files))` and run the test to measure the
  count (expected about 43). Then add:

```go
	// Population floor: a renamed or emptied root must not let the guard pass
	// over a shrunken surface.
	const livingDocFloor = <measured count>
	if len(files) < livingDocFloor {
		t.Fatalf("population floor: only %d living doc files scanned (expected >= %d)", len(files), livingDocFloor)
	}
```

  Then remove the log line.

- [ ] **Step 3: Run the guard and fix any residue.**
  `timeout --kill-after=10s 10m go test ./internal/repoguard/ -run TestLivingDocsAlignment -count=1 -v`
  Expected: PASS. Any hit is a page an earlier task missed. Fix the page, never the guard.

- [ ] **Step 4: Mutation-check the floor.** Back up the test file, set
  `livingDocRoots = []string{"README.md"}`, run, and expect FAIL on the floor. Restore with `cp -f`.

- [ ] **Step 5: Repo-wide terminology and dead-page sweep** over the in-scope tree:

```bash
/usr/bin/grep -rnEi -e 'daily loop' -e 'daily-loop' -e 'fix loop' -e 'build loop' -e 'the loop' -e 'autonomous loop' -e 'Go v1' -e 'sync-agents' -e 'board-refresh' -e 'workflow-roles' -e 'delegating-across-harnesses' -e 'validation-runbook' -e 'nested-launch' -e 'permissions\.example' -e 'sandbox\.example' README.md docs/README.md docs/guide docs/install docs/concepts docs/reference docs/release/four-harness-acceptance.md
```

  Expected: no hits, except a `references/fix-loop.md` skill-file link if one exists (that file
  is renamed by #502, which retargets the link). Fix anything else.

- [ ] **Step 6: One-off relative-link check.** This is not committed; paste the output into your
  report for the results file. Save this script to a `mktemp` file and run it with `python3`
  from the worktree root:

```python
import os, re, sys, unicodedata
roots = ["README.md", "docs/README.md", "docs/guide", "docs/install", "docs/concepts", "docs/reference", "docs/release/four-harness-acceptance.md"]
files = []
for r in roots:
    if os.path.isfile(r):
        files.append(r); continue
    for d, ds, fs in os.walk(r):
        ds[:] = [x for x in ds if x not in ("fixtures", "testdata")]
        files += [os.path.join(d, f) for f in fs if f.endswith(".md")]
def slugs(path):
    out, seen, fence = set(), {}, False
    for ln in open(path, encoding="utf-8"):
        if ln.lstrip().startswith(("```", "~~~")):
            fence = not fence; continue
        m = None if fence else re.match(r"^#{1,6}\s+(.*?)\s*#*\s*$", ln)
        if not m: continue
        s = re.sub(r"[^\w\- ]", "", m.group(1).strip().lower().replace("`", "")).replace(" ", "-")
        n = seen.get(s, 0); seen[s] = n + 1
        out.add(s if n == 0 else f"{s}-{n}")
    return out
bad = 0
link = re.compile(r"\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
for f in sorted(files):
    text = open(f, encoding="utf-8").read()
    text = re.sub(r"```.*?```", "", text, flags=re.S)
    for m in link.finditer(text):
        t = m.group(1)
        if re.match(r"^[a-z]+:", t): continue
        path, _, anchor = t.partition("#")
        target = f if path == "" else os.path.normpath(os.path.join(os.path.dirname(f), path))
        if not os.path.exists(target):
            print(f"BROKEN {f}: {t}"); bad += 1; continue
        if anchor and target.endswith(".md") and anchor not in slugs(target):
            print(f"ANCHOR {f}: {t}"); bad += 1
print(f"{bad} problem(s) in {len(files)} files")
sys.exit(1 if bad else 0)
```

  Expected: `0 problem(s)`. A link into `docs/adrs/` must resolve in this worktree; cite a higher
  ADR as plain text. Fix every BROKEN/ANCHOR line in the doc that holds it and re-run until
  clean.

- [ ] **Step 7: Run the touched packages.**
  `timeout --kill-after=10s 10m go test ./internal/repoguard/ ./internal/config/ ./internal/cli/ ./internal/assets/ -count=1`
  Expected: PASS. (docket-build then runs the full suite `go run ./cmd/docket development test`.)

- [ ] **Step 8: Commit.**

```bash
git add internal/repoguard/docs_alignment_test.go
# plus every doc path you fixed in Steps 3, 5, and 6, listed explicitly
git commit -m "test(repoguard): living-docs guard covers every living doc root with a population floor"
```

---

## Self-review notes (plan author)

- **Spec coverage.** Each spec section maps to a task:
  - Front door → T6.
  - docs/guide → T6–T7.
  - docs/install → T8–T9.
  - docs/concepts → T10–T11.
  - docs/reference → T9 (harness), T12, and T13 (glossary).
  - docs/release → T12.
  - `.docket.example.yml` and its test → T4.
  - harness-defaults → T3.
  - Dead scripts → T2.
  - `docket run` help → T1 (and the `cli.md` mirror in T12).
  - New docs guard → T5 + T14.
  - Pinned-wording tests → updated per task.
  - Acceptance: the verbatim copy is T4 Step 10, the link check is T14 Step 6, and the full
    suite is the docket-build gate.
- **Interfaces.** `config.SettingPaths` / `config.SettingPath` (T4) are consumed by T5.
  `livingDocRoots` (T5) is extended by T6–T13 and closed by T14. Test helper names
  (`undocumentedSupported`, `documentedUnsupported`, `verbatimCopyProblems`, `scanCitations`,
  `scanUnsupportedKeys`, `unsupportedKeyShapes`, `livingDocFiles`) are used consistently.
- **Ordering.** The guard covers only pages already aligned, so every intermediate commit is
  green. The deleted pages' inbound links are fixed in the same task that deletes them (T8, T9).
  `docs/README.md`'s entries for those pages are dropped earlier (T6).
