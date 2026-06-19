# Script the mechanical health checks (`board-checks.sh`) — results
Change: #23 · Branch: feat/script-sweep-and-health-checks · PR: <pending> · Plan: docs/superpowers/plans/2026-06-19-script-sweep-and-health-checks.md · ADRs: 12

## Verify (human)

No interactive/manual checks required — the five checks are covered hermetically by
`tests/test_board_checks.sh` (31 assertions; bare-origin git fixtures, no `gh`/network) and the
full suite is green (17/17 files PASS, incl. `test_render_board.sh` + `test_github_mirror.sh` on the
shared helper). The items below are merge-gate **decisions**, not manual test steps.

- [ ] Accept the two non-blocking nuances under **Follow-ups** (or open them as changes), then merge.

## Findings

- **ADR-0012 recorded** — the script-vs-model-driven boundary for `docket-status` passes
  ("mechanical & side-effect-free ⇒ script; judgment or shared terminal-transition ⇒ agent-prose"),
  generalizing `Accepted` ADR-0007 from the GitHub-mirror surface to the whole skill family. This was
  the spec's §2 deliverable. Accepted; publishes to `main` with this change's merge.
- **Reconcile narrowed the scope to health-checks-only.** Change **0025** had landed (`done`, PR #36)
  between this change's 2026-06-18 brainstorm and its build — it already scripted the merge sweep's
  close-out (`archive-change.sh`/`terminal-publish.sh`/`cleanup-feature-branch.sh`) and rewired the
  sweep + both kill paths. So the sweep is entirely out of 0023's scope (spec §5b superseded). The
  residual sweep step — the merged-PR `gh` probe — was kept model-driven (trivial, interleaved with
  the sweep's per-change rebase/re-read). The sweep prose was not touched.
- **`github-mirror.sh` migration dropped from scope** — 0022 already migrated it onto the shared
  helper (`scripts/lib/docket-frontmatter.sh` sources `field`/`list_field`/`has_section`/`resolve_deps`),
  so the spec's §3/§6/§7 "migrate github-mirror.sh" was already-done work. 0023 only *consumes* the
  helper; no new parser. The helper also exposes a bonus `DEP_ON[id]` (worst-unmet dep id), so
  `merge-gate-stall` names the blocking `#N` straight from `DEP_ON` rather than re-walking `depends_on`
  (a simplification over spec §5a).
- **Test-hardening during review (worth noting):** the `broken-spec` trivial carve-out fixture was
  initially not mutation-genuine (an empty `spec:` short-circuited `[ -n "$spec" ]` before the
  `trivial` guard); it was changed to an *absent* spec path + `trivial: true` so removing the guard
  would flip the assertion (commit `7ae4c77`). Reinforces LEARNINGS #25/#20 — a green carve-out test
  must flip under mutation.

## Follow-ups

- **merge-gate-stall message can over-claim (Minor, no behavior change here).** The check fires when
  `DEP_REASON[id] == "needs your merge"`, which the helper sets whenever *any* dep is at `implemented`
  — even if a co-existing dep is still `not yet built`. The message "a single merge unblocks downstream
  work" then over-claims. This faithfully matches spec §5a (DEP_REASON-based) and is zero behavior
  change from the prior model-prose. Future tightening option: soften the wording to "unblocks one of
  its dependencies," or only fire when the `implemented` dep is the *sole* unmet one.
- **stale-in-progress absent-branch carve-out is double-guarded (Minor).** The `rev-parse --verify`
  short-circuit and the `[ -n "$ts" ]` check both keep a not-yet-created branch silent; neither is
  independently mutation-isolable (an absent branch always yields empty `ts`, and empty-operand
  arithmetic errors rather than flips). Reviewed (incl. opus final review) as genuine defense-in-depth
  guarding different failure modes — not a LEARNINGS #21-style vacuous double-guard. No action needed
  unless a future refactor wants single-guard clarity.
- **dep-cycle could over-mark in a multi-cycle interleave (Minor).** The cycle-marking walks
  `PATH_STACK` from the back-edge target to the top; on a graph with interleaved cycles it can mark a
  cycle-adjacent node that is not strictly on that specific cycle. It only ever *over*-warns (never
  misses a real cycle), and `depends_on` graphs are tiny. Add a multi-cycle fixture if the graph ever
  grows. (The existing tests cover A↔B, self-loop, DAG, and dangling edges.)
