# Design: scripting vs model-driven for the merge sweep & health checks (change 0023)

**Status:** design (brainstormed 2026-06-18)
**Change:** 0023
**Depends on:** 0022 — introduces `scripts/render-board.sh` and the shared frontmatter / dependency-resolution helper this change reuses.
**Spins out:** 0024 — retire/downgrade the inline board/source-drift health check once rendering is deterministic.

---

## 1. Context

`docket-status` runs three passes: the `inline` **board render**, the **merge
sweep**, and the **health checks**. Change 0022 moves the board render — pure,
judgment-free transformation — out of the model and into `render-board.sh`. This
change settles the other two: **per pass, script it or keep it model-driven**,
then implement that decision.

The work split from 0022 deliberately: the board render is unambiguously
mechanical, while the sweep carries terminal-transition side effects entangled
with `docket-finalize-change`, and one health check is genuinely judgment-bearing.
Those deserved their own deliberation rather than blocking the clean extraction.

## 2. Guiding principle (to be recorded as an ADR)

> A `docket-status` pass moves into a **script** when it is **mechanical and free
> of shared terminal-transition side effects**. It stays **agent-prose** when it
> needs **judgment**, or when it drives **terminal-publish / harvest** that is
> shared with `docket-finalize-change` and must not diverge.

This generalizes ADR-0007's GitHub-mirror boundary ("deterministic external-write
mechanics live in a script; the rest stays agent-prose") into a rule for the whole
skill. The implementer records it as a new ADR; `adrs:` stays `[]` until accepted.

## 3. The shared helper (decision — resolves an 0022/0023 open question)

Dependency resolution and frontmatter parsing live in **one sourced helper
script**, never duplicated inline. 0022 introduces it as its first consumer
(working name `scripts/lib/docket-frontmatter.sh`); this change reuses and, if
needed, extends it. Contents:

- `field FILE KEY`, `list_field FILE KEY`, `has_section FILE STR` — lifted
  **verbatim** from the proven copies in `scripts/github-mirror.sh`.
- `resolve_deps` — the convention's dependency-resolution pass (per change:
  `satisfied` / `needs your merge` / `not yet built`, and `dependency-clear`
  vs `dependency-waiting` with the worst-unmet reason). The convention's "computed
  **once** per run, consumed by both the board and the health checks" invariant
  becomes literal: one parser, one resolver, sourced by **all three** scripts.

`github-mirror.sh` **migrates onto the helper**, deleting its private `field`/
`list_field`/`has_section` copies — so the extraction reduces total code rather
than adding a fourth parser. (This migration may land with 0022, whichever
extracts the helper first; 0023 must not leave two parsers behind.)

## 4. Frontmatter parsing — yq assessment (decision: stay hand-rolled)

The board, sweep, and checks read only **flat top-level scalars** (`id`, `status`,
`priority`, `spec`, `trivial`, `pr`, `updated`, archive-filename merge date) and
**single-line flow lists** (`depends_on: [4, 6]`, `related`, `adrs`). The two
existing helpers already cover all of it. `yq` would **not** simplify this
greatly: it cannot parse markdown-with-frontmatter natively (you `sed` out the
`---` block first regardless), and its real wins — flow-vs-block, quoting, nested
maps — address a shape this frontmatter does not have. The only dense parser in
the repo is `sync-agents.sh`'s **nested** `agents:` config, which is **0018's**
scope and already decided "keep as-is." 0023 therefore stays hand-rolled and
**does not depend on 0018** (`related`, not `depends_on`).

## 5. Per-pass decision

### 5a. Health checks → script the mechanical ones; keep `blocked_by` model-side

A new **`scripts/board-checks.sh`** (sources the §3 helper) runs every check that
is pure state inspection and **prints findings to stdout**; it never auto-fixes
(unchanged contract). `docket-status` invokes it, then the model layers on the one
judgment-bearing check.

| Health check | Verdict | Rationale |
|---|---|---|
| Broken `spec:` link (vs `metadata_branch`) | **script** | path resolution: `git cat-file -e <branch>:<path>` |
| Broken `plan:`/`results:` on `done` (vs `origin/<integration>`) | **script** | same, against the integration branch |
| `depends_on` cycles | **script** | graph walk over frontmatter |
| Stale `in-progress` (branch gone, or no commit in 3 d) | **script** | `git branch`/`git log` probes |
| Human-merge-gate stall | **script** | falls straight out of `resolve_deps` |
| Inline board/source drift | **→ 0024** | hinges on 0022 making rendering deterministic; decide there |
| `blocked_by:` blocker may have cleared | **model** | judgment — reading free text to infer whether an external issue/PR is resolved |

### 5b. Merge sweep → stays model-driven (deferred, with cause)

The sweep's only purely-mechanical step is the `gh` is-merged probe. Its archive
add (`git mv active→archive`, UTC merge date, reuse-existing-file idempotency,
change-file-only commit) is **the exact same primitive** `docket-finalize-change`
performs, and the convention requires the two to be **byte-identical and never
diverge**. Scripting it for the sweep alone would create a second implementation
of that primitive — precisely the divergence the convention forbids. Doing it
right means routing **both** the sweep and finalize through one shared archive
helper: a larger blast radius into `docket-finalize-change` that is out of scope
here. The sweep also drives **terminal-publish + branch/worktree cleanup +
learnings harvest**, all shared with finalize — agent-prose by the §2 principle.

**Decision:** keep the merge sweep model-driven. Revisit only as part of a
deliberate "extract the shared terminal-archive primitive" change covering
finalize too. (The is-merged probe is too trivial to script in isolation.)

## 6. Scope

**In scope**
- Reuse/extend the §3 shared helper; migrate `github-mirror.sh` onto it.
- `scripts/board-checks.sh` — the mechanical health checks (§5a), printing findings.
- Wire `docket-status`'s health-check pass to invoke the script, then run the
  `blocked_by` judgment check in-model on top.
- Record the §2 boundary as an ADR.
- `tests/test_board_checks.sh` — fixtures producing each finding (matches
  `tests/test_github_mirror.sh`).

**Out of scope**
- The `inline` board render — change 0022.
- Inline board/source-drift retirement — change 0024.
- Scripting the merge sweep — deferred (§5b); entangled with `docket-finalize-change`.
- The `github` surface — already scripted.
- Adopting `yq` — change 0018.

## 7. Test plan

`tests/test_board_checks.sh`: build a fixture `changes/` tree exercising each
scripted finding — a broken `spec:` link, a `done` change with a missing
`results:`, a `depends_on` cycle, a stale `in-progress` (branch absent), and a
merge-gate stall — and assert `board-checks.sh` emits exactly those findings and
no false positives on a clean tree. Confirm `github-mirror.sh`'s existing tests
still pass after it migrates onto the shared helper (no behavior change).
