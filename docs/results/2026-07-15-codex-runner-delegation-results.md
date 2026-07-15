# Cross-harness runner delegation (first runner: Codex) — results
Change: #79 · Branch: feat/codex-runner-delegation · PR: (see change file `pr:`) · Plan: docs/superpowers/plans/2026-07-15-codex-runner-delegation.md · ADRs: 37, 38

## Verify (human)

- [ ] Optional live re-check on your machine: `bash scripts/docket.sh runner-dispatch --runner codex --agent status -- "Run the board-only pass only"` relays a Codex-authored backlog digest and exits 0. (The build already ran this end-to-end successfully — codex-cli 0.144.4, ChatGPT auth, board-only pass, `board inline clean`, digest relayed on stdout, rc=0. Re-run only if you want your own receipt.)
- [ ] If you intend to delegate an **orchestrator** (implement-next / auto-groom) rather than a leaf agent: set `[features] multi_agent = true` in `~/.codex/config.toml` first — it is NOT currently set on this machine, and Codex-side SDD fan-out needs it. Leaf agents (status, adr, the report-leaves) run without it.

## Findings

- **Live pair verified claude→codex** (spec §7's smoke): preflight (real `codex login status`), skill discovery from `~/.codex/skills`, sandbox `workspace-write` + network flags, foreground blocking, and `--output-last-message` relay all exercised against the real repo; the child ran the docket-status board-only pass correctly.
- **Open question 1 resolved at reconcile:** codex-cli 0.144.4 has `--output-last-message <FILE>`; effort maps via `-c model_reasoning_effort=…` (docket `max` → codex `xhigh`); sandbox/network via `--sandbox` + `-c sandbox_workspace_write.network_access=true`.
- **Two decisions became ADRs:** ADR-0037 (explicit `runner:` switch, never model-ID sniffing — ADR-0015 passthrough preserved end-to-end) and ADR-0038 (delegation rides the generated shim wrapper body through one `emit_wrapper` chokepoint shared by both generation passes and `--check` leg (c) — no skill-body branching).
- **A mutation survivor exposed a weak fixture** (LEARNINGS guards-are-code, re-hit): the `--check`-flags-de-shimmed-wrapper test originally de-shimmed with junk bytes, which drift under EITHER emission path — the leg-(c)-bypasses-`emit_wrapper` mutant stayed green. Fixed by de-shimming with the exact native-emitted bytes (plus a fixture-sanity assert that shim ≠ native); the mutant now reddens. All four planned mutations (M1–M4) confirmed red.
- **Review finding fixed pre-PR:** the facade now rejects path-traversing `--runner` names (`../x`) — the runner name becomes a path component, and the docket.sh facade family is a finite table, never an escape hatch (charset guard + test).
- **Concurrent overlap with #0077** (codex-harness-toml-agents, `implemented` mid-build): both changes add registry scaffolding + tests to `sync-agents.sh`. This change's edits were kept additive (new functions + three call-site swaps); whichever merges second resolves hunks by intent — a mechanical rebase, no semantic conflict (0077 guarantees a byte-identical non-codex markdown emitter path).

## Follow-ups

- **Spec open question (policy, unresolved by design):** whether delegating `docket-finalize-change` to Codex sidesteps the merge-without-review classifier — interacts with #0062; documented as "never a policy bypass" in README/ADR-0037, but the authorization story itself belongs to #0062.
- **Accepted limitation:** per-agent model pins do not carry into a delegated orchestrator's child-side sub-dispatches. Whether #0077's `.codex/agents/docket-*.toml` files soften this is verifiable only after 0077 merges (its PR is open).
- **Method note:** built inline in the invoking session at the human's explicit direction (no subagents) — plan/build/review roles ran via `superpowers:writing-plans` inline, `superpowers:executing-plans` (the plan's sanctioned inline alternative to SDD), and an inline whole-branch review in place of `superpowers:requesting-code-review`'s reviewer dispatch. Disclosed here and in the PR body per the Missing-skill/degrade rule's spirit; the TDD + mutation-test discipline was followed task-by-task.
