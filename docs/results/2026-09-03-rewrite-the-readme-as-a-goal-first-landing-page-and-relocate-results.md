<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0400 — Rewrite the README as a goal-first landing page and relocate its technical body to docs/](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0400-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate.md)**
<!-- docket:backlink:end -->
# Rewrite the README as a goal-first landing page — results
Change: #400 · Branch: docs/rewrite-the-readme-as-a-goal-first-landing-page-and-relocate · PR: (see change record) · Plan: docs/superpowers/plans/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate.md · ADRs: 53 (cited)

## Verify (human)

<!-- Genuinely manual checks for the merge gate — things no automated test reaches. -->
- [ ] On GitHub, open the new `README.md` and confirm the goal-first framing reads well and that both documentation-map links navigate correctly: **Technical guide** → `docs/guide/README.md`, **How docket maps to the six-stage model** → `docs/comparison/ai-native-sdlc-playbook.md`.
- [ ] Skim `docs/guide/README.md` on GitHub to confirm the relocated body renders intact (the moved `## Table of contents` links and the `.docket.example.yml` / skills / agents links resolve from the new two-directories-deeper location).

## Findings

Automated acceptance evidence recorded here (spec Acceptance 3–5); the whole-suite gate (`go run ./cmd/docket development test`) is green and its build-evidence block rides in the PR body.

- **Acceptance 3 — byte-verbatim relocation.** `docs/guide/README.md`'s body from `## Table of contents` to EOF is 1061 lines on both sides. The relocation diff shows **26 changed lines (13 hunks), every one carrying a `](` link target** (no-prose-corruption count = 0); the `<!-- docket:config-fence: values -->` marker moved intact. Only relative link targets were rewritten (`](docs/`→`](../`, `](.docket.example.yml`→`](../../.docket.example.yml`, `](skills/`→`](../../skills/`, `](agents/`→`](../../agents/`); intra-page `#anchor` links and prose path mentions untouched. The full relocation diff is the PR-body byte-equality proof.
- **Acceptance 5 — guard repoint + mutation probes.** Six `internal/repoguard/prose_contracts_test.go` rows repointed from `README.md` to `docs/guide/README.md`; one new row `change_0400_readme_landing` pins the two README map links. Mutation probes (each: `cp` backup, `/usr/bin/grep -cF` landing check, `go test -count=1`, restore):
  - Probe A — delete `](docs/guide/README.md)` from README (landing count → 0): FAIL naming `change_0400_readme_landing`. Restored (count 1).
  - Probe B — delete `](docs/comparison/ai-native-sdlc-playbook.md)` from README (count → 0): FAIL naming `change_0400_readme_landing`. Restored (count 1).
  - Probe C — restore `file: "README.md"` on `test_typed_changes_docs` (mutation landed, count 1): FAIL ("untyped set can only shrink" no longer in README.md, proving the phrase moved to the guide). Restored.
  - Final `go test -count=1 ./internal/repoguard/`: PASS.
- **Acceptance 4 — link resolution.** All relative links + anchor fragments across `README.md`, `docs/guide/README.md`, and `docs/comparison/ai-native-sdlc-playbook.md` resolve. NOTE: the plan's one-off shell link-check emits four spurious `BROKEN:` lines for README's intra-guide `path#anchor` links because `[ -e "$d/$t" ]` stats the literal `path#anchor` string as a filename; an anchor-aware resolver confirms `broken=0` (each target heading — `## Install`, `## Migration`, `## Quickstart: the daily loop`, `## Tuning agent models & effort` — is present in the guide). No maintained-source file references a moved `README.md#anchor` (link-retarget sweep was an honest no-op — no commit).
- **Plan/spec defect corrected during build (guard table).** The spec/plan's guard table (Task 3 Step 1) repointed the whole `test_skill_fork_dispatch` row (two phrases) to `docs/guide/README.md`. After the split those two phrases live in **different** files: `completed (forked execution)` in the relocated guide body, `The right model for each step.` in the README header (line 11, above the `## Table of contents` boundary — never eligible for the moved body — and retained in the new landing README's "What you get"). `scanProse` is AND-over-`present`, so the single wholesale repoint would have reddened the suite. Corrected by **splitting the row in two**, each phrase pinned to the file that contains it — faithful to the spec's stated intent ("guards follow the content") and keeping the byte-equality constraint intact. This is the only deviation from the plan's literal instructions.
- **Review.** `docket-review-deep` (rung: base `standard` from Task 3's build profile, bumped one step by the 3003-changed-line whole-branch diff). Verdict: 0 blocker, 0 important, 1 minor. The minor (a brief "Deploy stage" vs "does not deploy" wording tension) was fixed in-branch — see the PR-body disposition table.

## Follow-ups

Reported for deliberate human capture (`docket change create`); nothing minted by this run.

- **Follow-on technical-docs split (already tracked).** Change **#0402** (created by a concurrent session, `depends_on: 400`) owns splitting the relocated `docs/guide/README.md` body into goal-organised technical pages and rewriting it — out of scope here by design.
- **Doc drift (separate change).** Several skills still name the retired Bash control plane (`board-refresh.sh`, `render-board.sh`, `github-mirror.sh`, `render-change-links.sh`, `render-artifact-backlink.sh`, `stack-base.sh`, `reclaim-claims.sh`, `disable-worktree-hooks.sh`) and their `scripts/<name>.md` contracts, while `scripts/` now holds only the release smoke and runner adapters; the relocated guide's install section still leads with `install.sh`. Surfaced by the comparison research; explicitly out of scope for this change.
- **Comparison gaps (candidates, not commitments).** The playbook comparison surfaced product gaps docket does not yet cover: a PR-comment fix loop after the PR opens, policy skills / a per-repo review policy the reviewer loads, a deterministic no-test-edits guard during repair, a production/scanner intake into `docket change create`, and a metrics digest over artifacts docket already writes.
