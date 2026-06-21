# Artifact links — results

Change: #35 · Branch: feat/artifact-links · PR: <pending> · Plan: docs/superpowers/plans/2026-06-21-artifact-links.md · ADRs: 7, 12 (cited; no new ADR)

## Verify (human)

Automated tests + CI are the primary receipt; these are optional spot-checks at the merge gate.

- [ ] Skim the rendered `## Artifacts` block on this change file once it carries the block (it renders on the next field write; this change deliberately did not back-fill its own block — see Follow-ups).
- [ ] Confirm the GitHub-mode block on a merged change resolves: the real-data smoke below renders a `done` change's block with plan/results pinned to `main` and spec/ADRs pinned to `docket` — clickable on GitHub.

## Findings

- **Renderer is offline + deterministic + idempotent.** `scripts/render-change-links.sh` reads frontmatter only via the shared `field`/`list_field` helpers (command-substitution, so the trailing-newline hazard from LEARNINGS #22/#32 cannot bite), matches markers with `awk index()` (fixed-string, immune to the markers' regex metachars), and resolves ADR globs into a bash array (no `ls | head`, pipefail-safe). Same inputs ⇒ byte-identical output, verified by the golden + idempotency cases.

- **Real-data smoke (GitHub mode) passed the full row matrix.** Run against the real `done` change #0002 (copied inside the worktree so the origin remote resolves): Spec/ADRs pinned to `docket`, Plan/Results flipped to `main` (status `done`), PR rendered `[#3](…/pull/3)`, both ADR slugs (`0001`, `0002`) resolved from the live `.docket/docs/adrs`, second run byte-identical. The `/tmp`-based smoke in the build only exercised the bare-path fallback (no remote in `/tmp`); the in-worktree GitHub-mode run is the one that closes the LEARNINGS #22 "smoke against real data" loop.

- **Whole-branch review caught a publish-ordering bug (fixed).** As originally wired, `docket-finalize-change` invoked `terminal-publish.sh` **before** the renderer re-pointed plan/results. Because terminal-publish copies the archived change file *from `origin/docket`* onto the integration branch, it would have published the **stale** block (plan/results still pinned to the deleted feature branch) — defeating the re-point on exactly the public surface it targets. The `docket-status` sweep was already correct (renderer commits+pushes to `origin/docket` before terminal-publish). Fix: reordered finalize step 3 so the renderer commits the re-pointed block to `origin/docket` **before** terminal-publish, converging the two skills (commit `2d74b88`). This is a sequencing requirement of the existing terminal-publish "copies from `origin/docket`" contract, not a new architectural decision — no ADR.

- **Reconcile correction folded into the spec.** The spec said the renderer should reuse `github-mirror.sh`'s remote resolution; in fact `github-mirror.sh` takes `--repo` from its caller and does not self-derive. The inline origin-URL derivation lives in `render-board.sh` — the renderer reuses that pattern (plus an optional `--repo` override for test mocking). Branch refs come from `docket-config.sh --export`, as the spec said.

- **Non-URL `pr:` now fails closed (review finding M2, fixed).** A malformed bare-number `pr:` (e.g. `pr: 44`) previously rendered `[#44](44)` — a broken relative link. The renderer now only emits the `[#NN](url)` link form when `pr:` is an actual URL (an `is_url` scheme check); any non-URL value renders verbatim (no broken link), in both the PR row and the killed-change plan/results row. Covered by test cases L and M (mutation-checked).

- **No new ADR.** The spec scoped this as a direct application of **ADR-0007** (one-way, change-files-authoritative derived output) and **ADR-0012** (script-vs-model boundary). The build introduced no decision that contradicts or extends either, so `adrs:` stays `[7, 12]`.

## Follow-ups

These were deferred from the whole-branch review (Minor; outside the happy path). Capture as new proposed changes if wanted:

- **M3 — insert path can duplicate a `## Artifacts` heading on a legacy marker-less file that already has one.** The marker-insert path unconditionally adds a `## Artifacts` heading after the frontmatter. A legacy hand-edited change with a pre-existing `## Artifacts` section but no markers would get a second heading (idempotent thereafter). No current docket change has that shape, and the template ships markers, so new changes never hit it. Optional guard: skip insertion if a `## Artifacts` line already exists.

- **One-time back-fill (already out of scope per the spec).** Existing active changes get the block naturally on their next field write; a bulk back-fill pass is a separate change if desired.

- **Tidy (pre-existing, noted by the reviewer):** `docket-status`'s sweep does a manual archive (steps c–e) *and* re-invokes `archive-change.sh` (step f) — a redundant double-archive that is idempotent but convoluted. Not introduced by this change; a candidate for a future cleanup so the sweep delegates the archive entirely to the script.
