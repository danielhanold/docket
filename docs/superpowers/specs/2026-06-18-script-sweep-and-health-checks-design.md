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

The helper is **defined and built by 0022** (see its spec §2); 0023 only
*consumes* it — no new parser here. The pinned interface this change relies on:

- `field FILE KEY`, `list_field FILE KEY`, `has_section FILE STR` — frontmatter
  accessors.
- `resolve_deps CHANGES_DIR` — scans once and **populates global associative
  arrays** `STATUS_OF[id]`, `DEP_STATE[id]` (`clear`|`waiting`), `DEP_REASON[id]`
  (worst unmet: `needs your merge` > `not yet built`). `board-checks.sh` reads
  `DEP_REASON` for the merge-gate-stall check and `STATUS_OF` for the others.

0022 also migrates `github-mirror.sh` onto this helper, so the "compute
dependency resolution once, one parser, one resolver" invariant already holds by
the time 0023 lands. If `board-checks.sh` needs a helper not yet present, it is
added to `lib/docket-frontmatter.sh` (not re-implemented locally).

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

| Health check | Verdict | Rationale |
|---|---|---|
| Broken `spec:` link (vs `metadata_branch`) | **script** | path resolution: `git cat-file -e <branch>:<path>` |
| Broken `plan:`/`results:` on `done` (vs the integration branch) | **script** | same, against the integration branch |
| `depends_on` cycles | **script** | graph walk over frontmatter |
| Stale `in-progress` (branch exists, no commit in 3 d) | **script** | `git`/`git log` probes |
| Human-merge-gate stall | **script** | falls straight out of `resolve_deps` |
| Inline board/source drift | **→ 0024** | hinges on 0022 making rendering deterministic; decide there |
| `blocked_by:` blocker may have cleared | **model** | judgment — reading free text to infer whether an external issue/PR is resolved |

#### `scripts/board-checks.sh` — contract

**CLI:** `board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]`

- Sources `lib/docket-frontmatter.sh` (0022); calls `resolve_deps DIR` once.
- **Git-only** (no `gh`, no network). Mock seam `GIT="${GIT:-git}"` — the only
  external dependency, so tests inject a temp repo (§7).
- **Output:** one finding per line on **stdout**, TAB-separated
  `<check-id>\t<change-id>\t<message>`, where `<check-id>` ∈ `{broken-spec,
  broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall}`. Clean
  tree ⇒ **no output**. Findings are sorted by `(check-id, change-id)` for
  determinism. **Warn-only — never auto-fixes** (unchanged contract).
- **Exit code:** `0` normally (findings go to stdout; the caller surfaces them);
  `--strict` ⇒ exit `1` if any finding (for a future CI gate). In `main`-mode
  `metadata_branch == integration_branch`; the script takes both verbatim and the
  two link checks resolve on the same branch with no special-casing.

#### Per-check predicates (with the SKILL's carve-outs, encoded)

- **broken-spec** — for each change with `spec:` non-empty **and** not
  `trivial: true`: finding iff `$GIT cat-file -e <metadata-branch>:<spec-path>`
  fails. (Skip `trivial` changes — they have no spec.)
- **broken-plan-results** — for each `status: done` change, for each of `plan:`/
  `results:` that is **set**: finding iff it does not resolve on the integration
  branch (link-rot). **Carve-out:** never flag a `plan:`/`results:` on an
  `implemented` change — those files legitimately still live on the unmerged
  feature branch.
- **dep-cycle** — DFS over the `depends_on` graph (built from `list_field`); on a
  cycle, emit one finding **per change in the cycle** (each node, so the human
  sees the whole loop).
- **stale-in-progress** — for each `status: in-progress` change with `branch:` set:
  if `$GIT rev-parse --verify <branch>` **fails**, skip (**carve-out:** a
  just-claimed change whose branch is not yet created is *not* stale — and "branch
  gone after creation" is indistinguishable from "not yet created" via git alone,
  so we conservatively do not flag it). If the branch **exists**, finding iff its
  newest commit (`$GIT log -1 --format=%ct`) is older than **3 days** from now.
  (3 d is the current fixed default — a future `.docket.yml` knob, out of scope.)
- **merge-gate-stall** — straight from `resolve_deps`: a build-ready change
  (`proposed`, `spec`-or-`trivial`) whose `DEP_REASON[id]` is `needs your merge`.
  The message names the blocking dep (re-walk that change's `depends_on` for the
  one at `implemented`). Surfaces "a single merge unblocks downstream work."

The **`blocked_by:` re-examination** stays model-driven (judgment); it is **not**
in `board-checks.sh`.

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
- `scripts/board-checks.sh` — the mechanical health checks (§5a contract).
- Wire `docket-status`'s health-check pass to invoke the script, then run the
  `blocked_by` judgment check in-model on top (§6a).
- Record the §2 boundary as an ADR.
- `tests/test_board_checks.sh` (§7).

### 6a. `docket-status` wiring (SKILL edit)

In `docket-status`'s **Health checks** section, replace the five mechanical
bullets (broken `spec:`, broken `plan:`/`results:`, `depends_on` cycles, stale
`in-progress`, human-merge-gate stall) with: "invoke `scripts/board-checks.sh
--changes-dir <metadata tree> --metadata-branch <mb> --integration-branch <ib>`
and surface each finding line as a warning." **Keep model-driven, unchanged:** the
`blocked_by:` re-examination bullet, and — until change 0024 lands — the inline
board/source-drift bullet (0023 does not touch it). **Keep:** the "do not auto-fix
unless asked" stance and the "share the one dependency-resolution pass" note (it
is now literally `resolve_deps`, run by the script). The `github`-surface
mirror-reachability flag is unaffected.

**Out of scope**
- The `inline` board render — change 0022.
- Inline board/source-drift retirement — change 0024.
- Scripting the merge sweep — deferred (§5b); entangled with `docket-finalize-change`.
- The `github` surface — already scripted.
- Adopting `yq` — change 0018.

## 7. Test plan

`tests/test_board_checks.sh` — unlike `test_github_mirror.sh` (a mocked-`gh`
fixture), these checks probe real git, so the harness builds a **temp git repo**
(`GIT_COMMITTER_DATE` to age commits), matching the `mktemp -d` + `trap rm`
idiom. Assert each finding fires and a clean tree is silent:

- **broken-spec** — a change citing a `spec:` path absent on `metadata_branch` ⇒
  one `broken-spec` line; a `trivial: true` change with no spec ⇒ silent.
- **broken-plan-results** — a `done` change whose `results:` path is absent on the
  integration branch ⇒ one finding; the **same** missing field on an
  `implemented` change ⇒ silent (carve-out).
- **dep-cycle** — `A→B→A` ⇒ a finding for **each** node.
- **stale-in-progress** — an `in-progress` change whose feature branch's last
  commit is 4 days old ⇒ finding; a freshly-claimed one whose `branch:` is set but
  the branch **doesn't exist** ⇒ silent (carve-out); a branch with a commit today
  ⇒ silent.
- **merge-gate-stall** — a build-ready change `depends_on` a change at
  `implemented` ⇒ a finding naming that dep.
- **clean tree** ⇒ empty stdout, exit `0`; `--strict` on a finding ⇒ exit `1`.

Plus regression: `tests/test_github_mirror.sh` stays green after `github-mirror.sh`
migrates onto the shared helper (per 0022 §5) — no behavior change.
