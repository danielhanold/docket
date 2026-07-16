# Codex CLI live-validation runbook — results

Change: #78 · Branch: feat/codex-cli-validation-runbook · PR: <url> · Plan: docs/superpowers/plans/2026-07-16-codex-cli-validation-runbook.md · ADRs: none

**This change ships a runbook, not a validation.** The merge gate here is *"is this runbook correct and executable?"* — **not** *"did Codex pass?"*. Executing it needs an interactive Codex CLI session and real OpenAI billing, so no autonomous agent can do it. Spec deliverables 2 (the execution results doc) and 3 (follow-up stubs per gap) are produced by **you executing the runbook**, at or after the merge gate — that split is intentional and recorded in the plan.

## Verify (human)

- [ ] **Execute the runbook** — `docs/codex/validation-runbook.md`, all six phases, in Codex CLI against a fresh fixture. This is the change's whole purpose.
- [ ] **Record the execution results** in a separate doc (`docs/results/<date>-codex-validation-results.md`) — per-step pass/fail, observed behavior, Codex CLI version, model used, date. A "no" is a valid outcome; the goal is confirmed knowledge.
- [ ] **File one `proposed` stub per gap found**, linked from that results doc.
- [ ] **Phase 4 settles ADR-0036.** That ADR deferred the user-level `~/.codex/AGENTS.md` dispatch decision to this validation. Phase 4 must produce a definitive answer (delegation automatic / prompted / refused; and whether user-level dispatch is needed for globally-scoped agents). Acting on the answer is a follow-up, not this change.
- [ ] **Decide what to do about your global `~/.config/docket/config.yml`** — see Findings, first item. This is the one item that bites outside this change.

## Findings

- **Your global `~/.config/docket/config.yml` carries a live `agents.codex` block of UNVALIDATED example slugs** (`status: gpt-5.6-luna`, `adr: gpt-5.6-terra`, six × `gpt-5.6-sol`) — sitting directly under its own comment reading *"The IDs here are UNVALIDATED examples."* Because the global layer is in the precedence chain (`sync-agents.sh:655`/`:721` pass `$GLOBAL_CFG` into `resolve_agent_layers`), resolving `(codex, <agent>)` finds a harness-specific line there, sets `RES_MODEL_FROM_HARNESS=1`, and `warn_fallback_model` returns early at `sync-agents.sh:591` — so **any** repo you opt into the codex harness generates wrappers carrying fake model IDs **silently, with zero warnings**. A warning-free `sync-agents.sh` run is therefore not evidence the models are real. This is a machine-config issue, not a repo defect, but it directly threatens the runbook's own execution — hence the Verify item above.

- **Two Critical defects were caught pre-merge, and both originated in the plan, not the implementation.** Recorded because the class matters more than the instances:
  - **Wrong script for the job.** Phase 1 told the operator to run `migrate-to-docket.sh` on a *fresh* fixture. That script migrates an *existing* repo and dies at `migrate-to-docket.sh:243` (`nothing to seed`), exit 1 — at the runbook's very first command. It also never writes `.docket.yml` (`CONFIG_FILE` is read-only via `yaml_get`). The fresh-repo tool is `docket.sh bootstrap` (the `CREATE_ORPHAN` path), which creates exactly two things: an empty orphan `docket` branch on origin and the managed `.gitignore` block.
  - **The pin arrived too late.** The model pin landed at Phase 5, so Phases 3–4 observed wrappers carrying model IDs Codex cannot spawn. A refusal there is indistinguishable from *"Codex does not honor the AGENTS.md dispatch block"* — which would have settled ADR-0036 on a **fabricated** finding. Fixed by moving slug discovery + the whole nine-agent `agents.codex` pin into Phase 1. (`agents.default.*` is not a shortcut: `RES_MODEL_FROM_HARNESS` is set only by a harness-specific line, so a default block would still warn *and* leak into claude.)

- **A guard-blindness class this repo has not recorded before: "path-correct, purpose-wrong."** Assertion 4 proves `migrate-to-docket.sh` is cited at its *real* path (the no-`scripts/`-prefix rule) — and never that it is the *right script for the job*. The guard was green across the entire first Critical. A docs deliverable invites exactly this: spelling is checkable, semantics are not. Candidate for the LEARNINGS harvest at close-out.

- **A verbatim regression of an already-fixed bug shipped, and the ledger caught it.** Task 1's Assertion 5 guarded only the inner `${DOCKET_SCRIPTS_DIR:?…}` token, leaving the `"…"/docket.sh` decoration unguarded — the exact defect change 0073 diagnosed, fixed, and wrote up three days earlier (`docs/results/2026-07-14-cursor-sandbox-permissions-guide-results.md`). The runbook itself was simultaneously teaching the unquoted form, and the guard was green on it. Fixed by asserting the full decorated spelling built from the derived token.

- **A reviewer's own factual claim was false, and was correctly overridden.** The whole-branch review instructed the fixer to warn that "~18 `WARN … may not be a valid model ID` lines are expected pre-pin." The fixer refused and verified why: the count is a function of which `~/.<harness>/agents` dirs exist (45 on an isolated fixture), and on a machine with the global codex block it is **zero**. The runbook now states the rule and both states, never a number. This is the verify-the-claim family firing on a *review* rather than a spec.

- **Deliberately left as-is, per change 0073's recorded precedent:** the `**Pass when:**` check is sum-equality rather than per-phase pairing (two stamps in one section and zero in another sums 6:6 and stays green), and the two link assertions are location-blind (degrading the setup.md link to an HTML comment stays green). 0073 assessed the identical pattern as "pathological, not exercised"; consistency with that precedent beat a bespoke exception.

## Follow-ups

- **Every gap the runbook finds** becomes a `proposed` stub when you execute it — including, if Phase 4 says so, user-level `~/.codex/AGENTS.md` dispatch (ADR-0036's deferred decision).
- **Consider a repo-level defense for the silent-fake-slug hazard** in the first finding: a harness-specific model that resolves from the *global* layer is currently indistinguishable from a validated one. Options range from a note in `docs/codex/setup.md` to a `sync-agents.sh --check` leg. Worth a stub only if executing the runbook confirms it bites in practice.
- **`config.yml.example`'s codex block is copy-paste bait.** It ships commented, but the machine config proves it gets uncommented verbatim, slugs and all. A stub to make the example self-defeating (obviously-invalid placeholder slugs rather than plausible-looking ones) may be worth more than the doc warning it already carries.
