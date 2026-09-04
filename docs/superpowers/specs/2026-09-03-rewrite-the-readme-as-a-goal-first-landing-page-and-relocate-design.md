<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0400 — Rewrite the README as a goal-first landing page and relocate its technical body to docs/](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-04-0400-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate.md)**
<!-- docket:backlink:end -->

# Rewrite the README as a goal-first landing page — design

**Change:** 0400 · **Type:** docs · **Date:** 2026-09-03

## Goal

Replace the 1,075-line mechanism-organised `README.md` with a short landing page that answers
"what does docket solve, and where do I stay in control?" in the vocabulary of Anthropic's
AI-Native SDLC Playbook, relocate the current README body verbatim into `docs/` so nothing is
lost, keep every repo guard green by repointing it at the relocated file, and commit the
comparison research the new README rests on.

This change is the **first of two**. The follow-on change splits the relocated body into
goal-organised technical pages and rewrites it; it depends on this one and is out of scope here.

## Research basis

The design rests on a capability-by-capability comparison of docket (main @ `ac782038`,
2026-09-02) against the playbook (https://claude.com/blog/the-ai-native-sdlc-playbook). The
full comparison is reproduced in the **Appendix** below and is committed by this change as a
docs page. Its conclusions, which the README must carry:

1. **The spine is shared.** Intent → spec → plan → diff → reviewed PR → merged commit, each
   committed before the next step reads it, with humans at the judgement points. docket
   implements that spine as a state machine with a board, an atomic claim, and an autonomous
   drainer; the playbook describes it as practices a team adopts one at a time.
2. **docket stops at the merge.** Stage 6 (production monitoring, security scans, incident
   channels) and enterprise governance (hooks, managed settings, sandboxing, Claude in CI) have
   no docket equivalent. The README must say this plainly rather than imply it.
3. **docket's depth is where the playbook is silent.** Just-in-time reconcile, model tiering
   per task, a supervised test gate with build-evidence, an in-branch fix loop, a
   rebase-retest-merge sequencer with conflict and repair agents, stacked changes, an ADR
   ledger, four harnesses with runner delegation. Reconcile is the single strongest point and
   leads the README.
4. **Five documented features are deferred from Go v1** and must be labelled as such wherever
   they are named: `auto_capture`, `terminal_publish`, automated learnings harvest/index/
   promotion, `dummy_mode`, `github_project`.

## Decisions (settled with the human, 2026-09-03)

| Question | Decision |
|---|---|
| Where the current README body goes in this change | **Relocated verbatim** to one docs page; the follow-on change splits and rewrites it. |
| Framing against the playbook | **Name it once with a link**; organise by its stage vocabulary; docket = stages 1–5, git-native, harness-neutral. |
| The comparison brief | **Committed** as a dated docs page and linked from the README. |

## What is built

### 1. `docs/guide/README.md` — the relocated body

- Create `docs/guide/README.md` containing the **current** `README.md` body from the
  `## Table of contents` heading to the end of the file, **byte-for-byte** (a `git mv`-shaped
  move followed by a new README write is the reference shape; the builder proves
  byte-equality of the moved region with a diff against the pre-change README in the PR body).
- Prepend exactly one authored block above the moved content:
  - the H1 `# docket — technical guide`,
  - one paragraph stating this is the former README body relocated unchanged by change 0400,
    pending the goal-organised split, with a link back to `../../README.md`.
- The relative links inside the moved body are re-targeted so they resolve from the new
  location (e.g. `.docket.example.yml` → `../../.docket.example.yml`, `docs/codex/setup.md` →
  `../codex/setup.md`, `skills/docket-build/SKILL.md` → `../../skills/docket-build/SKILL.md`,
  `agents/harness-defaults.yml` → `../../agents/harness-defaults.yml`). **Path rewriting is the
  only permitted edit inside the moved body**; intra-page `#anchor` links are unchanged.
- The `<!-- docket:config-fence: values -->` marker moves with its fence, untouched.

### 2. The new `README.md`

Target length ≤ 200 lines. Section order and content sources:

1. **Title + one-paragraph thesis.** docket keeps a backlog of planned work as markdown in your
   repo and ships skills that drain it to open pull requests. One sentence names the playbook:
   *docket is a repository-level implementation of the playbook's Plan, Design, Build, Test, and
   Deploy stages — git-native, harness-neutral, with the human at the merge* — with the link.
2. **What you get** — five bullets, outcome-first (durable backlog, hands-off implementation,
   the human merge gate, no new infrastructure, the right model for each step). Source: current
   README bullets, tightened. The phrase `The right model for each step.` is a pinned guard
   phrase today; after relocation its row points at `docs/guide/README.md`, so the new README
   may reword it freely.
3. **The committed artifact chain** — a compact table, stage by stage: what docket commits and
   where it lives (change file on `docket`; spec on `docket`; reconcile log + plan on the
   feature branch; build-evidence + results + review disposition in the PR; archived record +
   cleaned branch at close-out). Source: Appendix, "The committed artifact chain".
4. **Why docket: plans rot** — the reconcile step in three short paragraphs (problem, what
   docket does, the stance). Source: current README "The reconcile superpower", condensed.
5. **Where you decide** — the human judgement points as a short list: creating and grooming a
   change, merging the PR, finalize's confirmations, promoting a learning, filing discovered
   work. States explicitly that plan approval is *not* a human point by design, and that docket
   ends at the merge (no deploy, no production feedback).
6. **Install and the daily loop** — the three-command install and the five-step loop, each
   step one line, linking to the guide sections. Source: current Install + Quickstart, cut to
   the commands and the skill names.
7. **Documentation map** — links: the technical guide (`docs/guide/README.md`) with its major
   headings, the harness pages (`docs/cursor/`, `docs/codex/`, `docs/opencode/`), the
   comparison page, `.docket.example.yml`, the ADR index, `tests/README.md`.
8. **Status** — two sentences: docket-mode is the default; main-mode is the opt-out; what is
   deferred from Go v1 (the five features above, named).

Constraints on the new README:

- No YAML fence except the one `.docket.yml` example under *Install* (ADR-0053's fence guard
  was retired with the Bash suite, but the example stays value-equal to `.docket.example.yml`
  defaults by convention).
- No line-number cross-references (AGENTS.md rule).
- The playbook is named **once**; every other stage reference uses the vocabulary only.

### 3. `docs/comparison/ai-native-sdlc-playbook.md`

A dated Markdown page carrying the Appendix of this spec verbatim (the same headings, the six
stage matrices, the artifact chain, cross-cutting, the docs map, and the drift list). It opens
with the date and the docket revision it describes so it can go stale honestly. The README's
documentation map links it.

### 4. Guard repointing — `internal/repoguard/prose_contracts_test.go`

Change the `file` field from `README.md` to `docs/guide/README.md` on exactly these rows,
leaving `present`/`absent` phrases unchanged:

| sentinel | phrases (unchanged) |
|---|---|
| `test_consultant_brainstorm` | `brainstorm: docket-brainstorm` |
| `test_skill_fork_dispatch` | `completed (forked execution)`, `The right model for each step.` |
| `test_readme_finalize_docs` | `auto-mode classifier`, `Fork-exclusion principle` |
| `test_readme_skill_catalog` | present `## Skills`; absent `#the-eight-skills` |
| `test_cursor_permissions_docs` | `](docs/cursor/permissions.md)` — **becomes** `](../cursor/permissions.md)` because the link is path-rewritten in the move |
| `test_typed_changes_docs` | `untyped set can only shrink` |

Add **one** new row for the landing page, so the README's map cannot silently lose its two
load-bearing links:

```go
{sentinel: "change_0400_readme_landing", file: "README.md",
    present: []string{"](docs/guide/README.md)", "](docs/comparison/ai-native-sdlc-playbook.md)"}},
```

Mutation test (AGENTS.md rule): delete each link from the README and watch the row redden;
restore the old `file: "README.md"` on one repointed row and watch it redden against the new
README.

Note the comment block at the top of that file says `docs/` is read by path, fail-closed —
the moved file is read by its new path, which is exactly the intended coupling.

### 5. In-repo links that target the README

Derive the set with a whole-repo grep for `README.md#` and for bare `README.md)` links in
maintained source (skills, agents, docs/, `.docket.example.yml`, `tests/README.md`,
`AGENTS.md`); **never hand-list them**. For each hit whose anchor lives in the moved body,
retarget it to `docs/guide/README.md#<anchor>` (relative to the linking file). Point-in-time
records (`docs/changes/archive/`, `docs/results/`, specs, plans, Accepted ADRs) are **not**
edited — AGENTS.md's cross-reference rule keeps them as written.

## Acceptance

1. `go run ./cmd/docket development test` is green (whole suite, not a subset).
2. `README.md` ≤ 200 lines; names the playbook exactly once; contains both map links.
3. `docs/guide/README.md` contains the pre-change README body byte-identical apart from
   relative-path rewrites, proven by a diff in the PR body.
4. Every relative link in `README.md`, `docs/guide/README.md`, and
   `docs/comparison/ai-native-sdlc-playbook.md` resolves to an existing path (a one-off shell
   check in the results file; not a new permanent test).
5. The six repointed guard rows and the one new row are present; the mutation checks above are
   recorded in the results file.
6. No skill, agent, CLI, or config file changes.

## Open questions carried into build

- Whether `docs/guide/` is the right directory name for the follow-on split to grow into. The
  builder keeps it; the follow-on change may rename with a redirect stub.

---

## Appendix — docket against the AI-Native SDLC Playbook

*Compared on 2026-09-03: the playbook as published at
https://claude.com/blog/the-ai-native-sdlc-playbook against docket `main` @ `ac782038`
(2026-09-02). Sources: README, `skills/docket-convention/SKILL.md`, all twelve skills,
`.docket.example.yml`, `docket --help` for every noun, the 96 ADRs.*

**Legend.** *Both* — described in the playbook and present in docket. *Playbook only* — no
docket equivalent. *docket only* — not described in the playbook. *docket · deferred* —
documented and parseable in docket, but activates nothing in Go v1.

### Verdict

The playbook and docket agree on the spine: intent, then spec, then plan, then diff, then a
reviewed PR, each committed to git before the next begins, with humans holding the judgement
points. docket implements that spine as a state machine with a board, an atomic claim, and an
autonomous drainer. They diverge at the edges: the playbook reaches into production and into
enterprise governance; docket stops at the merge and keeps governance inside configuration and
fail-closed operations. In return docket has depth the playbook never mentions.

- **Both:** committed artifact chain, brainstorm-to-spec, plan-before-code, worktree
  parallelism, subagents, agent self-verification with pasted evidence, severity-ranked AI
  review, memory promoted into CLAUDE.md, humans at plan/PR/policy points.
- **Playbook only:** policy skills at spec time, deterministic hooks, managed settings and
  sandboxing, continuous evals in CI, @claude on PR comments, Claude in the pipeline, MCP deploy
  tooling with tiered autonomy, production monitoring and incident channels, a measurement
  framework.
- **docket only:** eight-state backlog with a board, atomic claims for parallel drains,
  just-in-time reconcile, autonomous grooming with an adversarial critic, profile-routed build
  workers with escalation, a supervised gate with build-evidence, an in-branch fix loop, a full
  finalize sequencer, stacked changes, an ADR ledger, four-layer config with a capability
  catalog, four harnesses and runner delegation.

### Stage 1 — Plan: capture the idea as a committed artifact

| Capability | Playbook | docket | Status |
|---|---|---|---|
| Version-controlled intent artifact | `intent.md` with a fixed section set; git carries author and time. | The change file: `## Why` / `## What changes` / `## Out of scope` / `## Open questions` plus `id`, `priority`, `type`, `depends_on`, `related`, `discovered_from`, `adrs`. On the orphan `docket` branch, always pushed. | Both |
| Brainstorm with the model | Originator talks to Claude; Claude drafts. | `docket-new-change` runs the pluggable `brainstorm` role inline with the human. | Both |
| Human correction before commit | Product owner reviews and corrects. | The brainstorm is interactive by construction. | Both |
| Backlog with lifecycle states | Not described. | Eight states, one physical move to `archive/`, `BOARD.md` rendered inside every status-writing commit. | docket only |
| Dependency and relationship graph | Not described. | `depends_on` gates on `done`; board distinguishes *needs your merge* from *not yet built*; `stacked_on`; cycles are health findings. | docket only |
| Type taxonomy and priority | Not described. | `change_types`; priority → age → lowest id selection. | docket only |
| Scan a project for candidate work | Not described. | Opt-in scan mode mints `proposed` stubs. | docket only |
| Automatic capture of discovered work | Stage 6 findings re-enter as `intent.md`. | `auto_capture` parseable but inert; runs report follow-up work, a human files it. | docket · deferred |
| Measurement | Time to committed artifact; survival into design. | None; derivable from `created:` and the spec commit. | Playbook only |

### Stage 2 — Design: collapse requirements and design into one session

| Capability | Playbook | docket | Status |
|---|---|---|---|
| Spec committed beside the intent | `spec.md` beside `intent.md` as the audit record. | Spec on the metadata branch; `spec:`, `## Artifacts`, and the backlink written in one commit. Build-ready = spec or trivial. | Both |
| Flagged concerns resolved before build | Policy concerns resolved with policy owners. | Consultant critique concerns; auto-groom critic verdicts (sound / fixable / needs-human). Design concerns, not organisational policy. | Both (different source) |
| Organisational policy skills at spec time | Brand, security, compliance, UX skills guide the session. | docket ships no policy skills; the harness's own may fire. | Playbook only |
| Autonomous design with an adversarial gate | Not described. | `docket-auto-groom` + isolated critic; one revision round; abstain writes `## Auto-groom blocked`; kill and defer never autonomous. | docket only |
| Pinned-model spec author | Not described. | `docket-brainstorm` dispatches `docket-brainstorm-consultant` once; capture-then-groom pins the whole conversation. | docket only |
| Trivial path | Not described. | `trivial: true` keeps a small change build-ready with no spec. | docket only |
| Grooming queue and selection bands | Not described. | Deterministic next-stub selection; abstained, then human-only, then auto-groomable. Grooming takes no claim. | docket only |
| Measurement | Intent-to-spec time; rework by spec commits after the first plan commit. | None; reconcile-log entries make rework derivable. | Playbook only |

### Stage 3 — Build: plan before code, then implement in parallel

| Capability | Playbook | docket | Status |
|---|---|---|---|
| Plan committed before code | Plan-mode interview; `plan.md` committed. | `docket-plan-writer` authors via the `plan` role, commits on the feature branch with a trailer; the parent verifies from git facts; `change attach-plan` records it. | Both |
| Human approves the plan | The engineer validates before code. | Deliberately absent; the human's checkpoint is the PR. | Playbook only (by design) |
| Refresh a stale change before planning | Not described. | The reconcile pass after claim, before the worktree: re-reads against related/archived changes, ADRs, and code; rewrites scope; `## Reconcile log`; kills obsolete, halts invalidated. | docket only |
| Institutional knowledge file | `CLAUDE.md`; mistake twice → into the file. | `AGENTS.md` is the promotion destination for learnings; criterion *will the agent know to search for this?*; human-gated. | Both |
| Skills as institutional knowledge | Policy skills, centrally owned. | docket is a skill set plus a pluggable `skills:` map; workflow skills, not policy skills. | Both (workflow vs policy) |
| Deterministic hooks as guardrails | Protected paths, formatters, credential checks. | None; docket disables git hooks on its metadata worktree; its own guards are tests. | Playbook only |
| Parallel work in worktrees | Multiple instances in separate worktrees. | `.worktrees/<slug>` per change; compare-and-swap claim; `/loop` drains. | Both |
| Subagents with scoped tools | Verification, simplification, exploration. | 17 generated wrappers, each pinned to a model and effort. | Both |
| Model and effort matched to the task | Not described. | Economy / standard / premium / max, rubric-routed, one escalation; `max` reserved for irreversible work. | docket only |
| TDD per task with one commit | Failing test first for bug fixes. | `docket-build-task`: baseline, failing test, implementation, self-review, one commit; SHA verified as an ancestor. | Both |
| More than one harness | Claude products throughout. | Claude Code, Cursor, Codex, opencode; runner delegation. | docket only |
| Stacked changes | Not described. | `stacked_on`, `stacked-merged`, promoted when the root lands. | docket only |
| Measurement | First-pass merge share; rework; plan/diff alignment. | None; escalation lines and the disposition table make rework derivable. | Playbook only |

### Stage 4 — Test: a feedback loop, and verification as part of done

| Capability | Playbook | docket | Status |
|---|---|---|---|
| Agent verifies before human review | Tests, builds, screenshots; quantifiable targets. | The build gate runs `build.test_command` once after all tasks; unconfigured halts. | Both |
| Verification output as proof of done | Output pasted before "complete". | Build-evidence record (command, result, head SHA, time) minted from the run directory, written into the PR body, required by the reviewer, read by finalize. | Both |
| Suite outlives one foreground call | Not described. | `docket gate drive` slices; `gate_observation_budget` fails closed; forked children block. | docket only |
| Red suite repaired in-loop | Implied. | One synthetic integration-repair task, one rung above default. | Both |
| Block test edits during a fix | A hook forbids editing tests in fix tasks. | Prose contract only. | Playbook only |
| Visual comparison for UI work | Screenshot against mocks. | None. | Playbook only |
| Continuous evals gating agent config | 20–50 tasks re-run on config changes; incidents become evals. | docket tests its own skills and config; a release-candidate workflow smokes four runners. Nothing evaluates a consuming repo. | Playbook only |
| Suite runner with budgets | Not described. | `docket development test`: fail-closed discovery, isolation, screen-then-confirm budgets, typed exit codes. | docket only (own suite) |
| Measurement | First-pass CI success; review time; change failure rate. | None. | Playbook only |

### Stage 5 — Deploy: review, approval gates, and getting to merged

| Capability | Playbook | docket | Status |
|---|---|---|---|
| AI review with severity-ranked findings | `REVIEW.md` passes; ranked; humans on behaviour and risk. | `docket-review`: blocker / important / minor; anchored on symbol or quoted clause; never re-litigates what the green suite proves. | Both |
| Review policy as a versioned file | `REVIEW.md` by the tech lead. | The review contract is docket's skill; `skills.review` is rebindable. | Playbook only (rebindable) |
| Review rung chosen by rule | Not described. | Lean / standard / deep from the highest build profile; bumped above 1500 changed lines; refuses without green evidence at HEAD. | docket only |
| Findings fixed by the agent | `@claude` on a PR comment pushes a fix. | Fixed before the PR opens by the fix loop (`review.min_fix_severity`, `review.max_fix_tasks`, blockers always); no PR-comment loop. | Both (pre-PR vs on-PR) |
| Findings feed institutional memory | Into `CLAUDE.md`. | Learnings findings with promotion to `AGENTS.md`; harvest and index are human curation today. | docket · deferred |
| Human merge gate | Code owner approves. | The implementer never merges; finalize merges only when authorised; branch-protection recipe. | Both |
| Rebase-and-retest before merge | Not described. | `finalize.gate` local / ci / both / off; evidence-based skip only on a no-op rebase. | docket only |
| Conflict and repair agents at the gate | Not described. | `docket-rebase-resolver` (≤2), `docket-integration-repair` (≤2); unattended repair blocks for sign-off. | docket only |
| Exactly-once, verified merge | Not described. | Every conjunct rechecked at the moment of effect; one permitted method; merge commit proven reachable. | docket only |
| Hooks as approval gates | Allow / ask / block scripts; release-manager authorisation; logged. | Typed operations refusing with closed reason tokens and durable markers. | Playbook only (different mechanism) |
| Managed settings, sandboxing, deny lists | Central permissions, OS sandbox, credential stripping. | Documented Cursor/Codex/opencode fragments; nothing central or enforced. | Playbook only |
| Claude in the CI pipeline | Failure triage, changelogs; scoped credentials. | None. | Playbook only |
| Deployment via MCP with tiered autonomy | Deploy / status / rollback per environment. | None; docket ends at the merge. | Playbook only |
| Hands-free close-out drain | Not described. | `/loop docket-finalize-change`: one merge per iteration, selection by mergeability. | docket only |
| Measurement | Time to first review; comments resolved without a human; escapes. | None; the disposition table records outcomes per finding. | Playbook only |

### Stage 6 — Maintain: close the loop from production back to intent

| Capability | Playbook | docket | Status |
|---|---|---|---|
| Production monitoring triggers the agent | Control-band breaches; tiered response; findings re-enter as intents. | None. | Playbook only |
| Scheduled security scans | Claude Security; validated findings; patches through the PR gate. | None. | Playbook only |
| Incident channels and ticket triage | Claude in Slack/Teams as first responder. | None. | Playbook only |
| Findings re-enter the pipeline | From production and scans. | From builds and close-outs, reported; a human files with `docket change create`. | Both (different source, manual capture) |
| Backlog hygiene and self-healing | Not described. | `docket status` (read-only), `maintenance sweep`, reclaim of expired branchless claims, `change resume-halted`. | docket only |
| Health checks with typed findings | Not described. | Validation, artifact integrity, derived-view drift, topology, prepare-time refusals; two judgement checks warn-only. | docket only |
| Decision ledger | Not described. | Immutable numbered ADRs; supersede and reverse as new records; 96 today. | docket only |
| Build-loop memory with relevance-gated reads | `CLAUDE.md` always in context. | Findings read per relevance at groom, plan, review; only unprompted rules promoted. Harvest and index deferred. | docket · deferred |
| Measurement | Breach-to-intent time; findings merged; repeat incidents. | None. | Playbook only |

### The committed artifact chain

| Stage | Playbook artifact | docket artifact |
|---|---|---|
| 1 | `intent.md` | Change file on `docket`: frontmatter + Why / What changes / Out of scope / Open questions; board row. |
| 2 | `spec.md` | Spec file on `docket` with a backlink header; `spec:` and `## Artifacts` on the change; or `trivial: true`. |
| 3 | `plan.md` + the diff | `## Reconcile log` and `reconciled: true`; plan on the feature branch (frozen once merged); one verified commit per task. |
| 4 | PR with test results | Build-evidence record in the PR body; results file; review disposition table. |
| 5 | Merged PR / commit | Merge proven reachable; change archived as `archive/<date>-<id>-<slug>.md` with optional `## Closeout notes`; branch and worktree cleaned; board re-rendered. |
| 6 | Incident record → new `intent.md` | ADRs; learnings findings; discovered work in the run report → human-filed change. Nothing from production. |

The reconcile step is docket's one addition to the chain itself: the playbook assumes the spec
is current when build starts; docket assumes it is not.

### Cross-cutting

| Principle | Playbook | docket | Status |
|---|---|---|---|
| Governance as configuration | Skills, hooks, managed settings; `CLAUDE.md` and `REVIEW.md` reviewed like code. | Four config layers per key; coordination-key fence; every key documented in `.docket.example.yml` or the suite fails; the capability catalog is the only hard-coded CLI spelling. | Both (different content) |
| Human judgement points | Intent, plan approval, design review, PR approval, findings triage, policy changes. | Change creation and grooming, the PR merge, finalize confirmations and repair sign-off, learnings promotion, filing discovered work, abstained stubs. Plan approval deliberately not a human point. | Both |
| Audit trail | Git history; logged hook decisions; incident conversations. | Every transition a metadata commit; presence-encoded sections; receipts and leases; frozen plan/results records; ADR supersessions as new files. | Both |
| Legacy tracker integration | One system of record per artifact. | One-way GitHub Issues mirror; Projects v2 fenced but unwired. | Both (GitHub issues only) |
| Autonomy without a human channel | Scoped subagents; gates ask a human. | Abort-and-report wrappers; autonomy precedence; forked children never yield; four dispositions drive any loop. | docket only |
| Run-gate bracketing for dispatched runs | Not described. | `run gate-before` / `gate-verdict` / `gate-claim`; retry-once accounting in durable records. | docket only |
| Persona-calibrated human prose | Not described. | `dummy_mode`; rejected at the config gate today. | docket · deferred |
| Terminal records on the code branch | Everything on one branch. | `terminal_publish`; parseable, fenced, inert. | docket · deferred |
| Metrics framework | Leading and lagging indicators; DORA. | None; most leading indicators derivable from existing timestamps. | Playbook only |

**Gaps the comparison surfaces (candidates, not commitments):** a PR-comment fix loop after the
PR opens; policy skills or a per-repo review policy the reviewer loads; a deterministic
no-test-edits guard during repair; an intake path from production or scanner findings into
`docket change create`; a metrics digest computed from artifacts docket already writes.

### A goal-first docs map (for the follow-on change)

| Page (goal) | Stage | Content | From today's README |
|---|---|---|---|
| README — what docket automates, and where you stay in control | Landing | Thesis in the playbook's vocabulary; artifact chain; reconcile; install; daily loop; map | Intro, How it works, Why docket, Quickstart |
| Capturing work that outlives the session | 1 | Change file and manifest; lifecycle and board; priorities, types, dependencies, stacking; scan mode, trivial, discovered work | Lifecycle, convention manifest, auto_capture / change_types |
| Designing before building | 2 | Interactive grooming; autonomous grooming and the critic; consultant specs | Quickstart step 2, consultant brainstorm |
| Building without supervision | 3 | implement-next; reconcile; plan authoring; build profiles and escalation; worktrees, claims, /loop, dispositions | Why docket, docket-build, draining hands-free |
| Proving the build | 4 | Build gate and evidence; gate driver and budgets; integration repair; configuring the suite | docket-build gate paragraphs, gate references, tests/README |
| Reviewing before the human does | 5 | Reviewer contract and rungs; fix loop and disposition table; why the suite runs in the build gate | docket-review section, fix-loop reference |
| Landing changes safely | 5 | Finalize end to end; blocked and identity repair; branch protection; /loop finalize | Closing out hands-free, hands-off finalize |
| Keeping the backlog honest | 6 | Status vs sweep; health codes; reclaim; halted recovery | Reclaiming stale claims, docket-status |
| Remembering why | 6 | ADRs; learnings and promotion; what is human-curated today | Learnings, docket-adr |
| Governing through configuration | cross | Layers and the fence; every config block by purpose; skills map; capability catalog; dummy_mode | Configuration, convention config |
| Running on your harness | cross | Install and update; model pins; invocation paths; Cursor/Codex/opencode; runner delegation | Install, Updating, Tuning, Runner delegation, docs/* |
| Where the metadata lives | cross | Two-branch model; artifact locations; GitFlow; migration and bootstrap guard; hooks; terminal_publish | docket-mode, Migration, Status |
| Reference | appendix | CLI by noun and verb; manifest and ADR fields; dispositions, reason tokens, health codes; skill and agent inventory | `docket --help`, convention |

### Drift to fix alongside the rewrite (separate change)

Several skills still name the retired Bash control plane (`board-refresh.sh`,
`render-board.sh`, `github-mirror.sh`, `render-change-links.sh`, `render-artifact-backlink.sh`,
`stack-base.sh`, `reclaim-claims.sh`, `disable-worktree-hooks.sh`) and their `scripts/<name>.md`
contracts, while `scripts/` now holds only the release smoke and runner adapters. The README's
install section still leads with `install.sh` while describing the Go engine underneath.
