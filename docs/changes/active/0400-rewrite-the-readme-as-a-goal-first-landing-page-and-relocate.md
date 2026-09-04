---
id: 400
slug: 'rewrite-the-readme-as-a-goal-first-landing-page-and-relocate'
title: 'Rewrite the README as a goal-first landing page and relocate its technical body to docs/'
status: 'implemented'
priority: 'medium'
type: 'docs'
created: '2026-09-03'
updated: '2026-09-04'
depends_on: []
stacked_on:
related: [283, 385]
discovered_from: []
adrs: [53]
spec: 'docs/superpowers/specs/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-design.md'
plan: 'docs/superpowers/plans/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate.md'
results: 'docs/results/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'docs/rewrite-the-readme-as-a-goal-first-landing-page-and-relocate'
pr: 'https://github.com/danielhanold/docket/pull/274'
blocked_by:
reconciled: true
claimed_at: '2026-09-03T23:15:33Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-design.md) |
| Plan | [2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate.md) |
| Results | [2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-results.md) |
| ADRs | [ADR-0053](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0053-readme-yaml-fences-guarded-by-default-opt-out-marker-grammar.md) |
<!-- docket:artifacts:end -->

## Why

The README is a 1,075-line page organised by mechanism — install, config, docket-mode, tuning, skills, customisation — with the reason to adopt docket buried under the install and configuration surface. A reader arriving cold cannot answer "what does this solve, and where do I stay in control?" without reading most of it.

A comparison of docket against Anthropic's AI-Native SDLC Playbook (the six-stage model: Plan → Design → Build → Test → Deploy → Maintain, each stage ending in a committed artifact the next reads) shows that docket is a repository-level implementation of stages 1–5 with a spine the playbook never describes: a state-machine backlog, just-in-time reconcile, profile-routed builds, a supervised test gate with build-evidence, an in-branch fix loop, and a full rebase-retest-merge sequencer — while stopping deliberately at the merge and at repo-level governance. That research is the basis for a README that leads with outcomes: what docket automates, which artifacts it commits at each stage, and the two points where a human decides.

The technical content the README carries today is still needed; it moves, unchanged, into a docs/ page so the follow-on change that splits it into goal-organised technical docs starts from one relocated file rather than from the README.

## What changes

- `README.md` becomes a short landing page organised by outcome, in the playbook's stage vocabulary: what docket automates and where you stay in control; the committed artifact chain (change → spec → plan → verified diff with build-evidence → reviewed PR → archived record); reconcile as the thesis; install and the daily loop in a few lines; a link map into the technical docs. It names the AI-Native SDLC Playbook once, with a link, and frames docket as stages 1–5, git-native and harness-neutral.
- The current README body is relocated **verbatim** into a single docs/ page (a mechanical move — no rewording — so the follow-on change owns every editorial split).
- The six `internal/repoguard` prose-contract rows that pin README phrases (`test_consultant_brainstorm`, `test_skill_fork_dispatch`, `test_readme_finalize_docs`, `test_readme_skill_catalog`, `test_cursor_permissions_docs`, `test_typed_changes_docs`) are repointed at the relocated file in the same commit, so the guards follow the content.
- The comparison brief is committed as a dated docs/ page (stage-by-stage Both / Playbook-only / docket-only matrices, the artifact-chain mapping, and the goal-first docs map) and linked from the README.
- Every in-repo link that targets a README anchor (skills, agents, docs/ pages, `.docket.example.yml`) is re-targeted to the relocated page.

## Out of scope

- Splitting the relocated technical body into goal-organised pages, or rewording any of it — that is the follow-on technical-docs change, which depends on this one.
- Any change to skills, agents, the CLI, or `.docket.example.yml`; this change touches prose and the guard table only.
- Fixing the documentation drift the comparison surfaced inside skill bodies (references to the retired Bash control-plane scripts) — a separate change.
- New product features suggested by the comparison (PR-comment fix loop, policy skills, no-test-edits guard, production intake, metrics digest).

## Reconcile log

### 2026-09-03

2026-09-03 — Reconciled at claim. The change was drafted and its spec settled earlier the same day, so current reality still matches it: README.md is 1,075 lines and mechanism-organised; ADR-0053 (the retired README YAML-fence guard) is cited unchanged; the six `internal/repoguard/prose_contracts_test.go` README rows named by the spec all exist with the exact `present`/`absent` phrases the spec's table pins (`test_consultant_brainstorm`, `test_skill_fork_dispatch`, `test_readme_finalize_docs`, `test_readme_skill_catalog`, `test_cursor_permissions_docs`, `test_typed_changes_docs`); and both related changes (283 slim-AGENTS.md, 385 cursor-permissions-docs) remain `proposed` and unmerged, so neither collides with this prose-and-guard move. No scope change, no new constraint, no dependency to fold in. Relations (`related: [283, 385]`, `adrs: [53]`) left as authored. Proceeding to plan and build as specified.
