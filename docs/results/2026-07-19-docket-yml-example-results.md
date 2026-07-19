# .docket.yml.example — the canonical all-comprehensive config reference — results
Change: #101 · Branch: feat/docket-yml-example · PR: <pending> · Plan: docs/superpowers/plans/2026-07-19-docket-yml-example.md · ADRs: 19, 39, 48

## Verify (human)

<!-- The suite is hermetic and cannot see the metadata branch or a real global-config layer.
     These are the checks only a human on a real machine can make. -->

- [ ] **Copy-paste safety, the file's core promise.** In a scratch repo, copy `.docket.yml.example`
      verbatim to `.docket.yml`, commit on the default branch, run
      `"$DOCKET_SCRIPTS_DIR"/docket.sh env` and confirm the export block is byte-identical to the
      no-config run. The suite proves this against a fixture; confirm it on a real repo once.
- [ ] **The machine-scoped warning path.** Copy the same file to `~/.config/docket/config.yml` and
      confirm you get exactly seven `per-repo-only … ignored` warnings (metadata_branch,
      integration_branch, changes_dir, adrs_dir, results_dir, github_project, terminal_publish) and
      that `--export` output is still eval-clean. This is the header's new documented exception.
- [ ] **`install.sh` on a machine with an existing global config.** Confirm the new pointer-only
      scaffold in `scripts/ensure-global-config.sh` does not clobber your real
      `~/.config/docket/config.yml`. Pinned byte-exactly in two tests, but this one is worth seeing
      on your own machine given the recorded near-miss (config-layer-write-and-read-hazards).
- [ ] **ADR-0039 status flip publishes.** At merge, confirm terminal-publish carried BOTH ADR-0048
      and ADR-0039's `status: Superseded by ADR-48` line onto `main`. The manifest lists
      `adrs: [19, 39, 48]` specifically so this lands atomically — the known superseded-ADR
      Accepted-gate hazard silently skips a non-Accepted ADR otherwise.

## Findings

**Two Critical review findings, both the same false claim, both about `github_project`.** In a
change whose entire deliverable is documentation accuracy, the shipped prose asserted a behavior
that does not exist: that the first `github` sync mints a Projects v2 board and writes the resolved
`{owner, number}` back over `auto`. Nothing reads `github_project` from config at all —
`github-mirror.sh` resolves its board only from `--project` / `--auto-create-project`, and
`docket-status.sh` populates those only from CLI flags no skill passes. The plan had established
this correctly at line 35 ("documented-but-unwired key … the `auto` sentinel is
documentation-only") and the prose was then written against the *old* `.docket.yml`'s claim instead
of against the plan's own finding. Fixed in `.docket.yml.example`, `scripts/github-mirror.md`, and
`scripts/docket-config.md` with explicit NOT-WIRED annotations. This is the repo's documented
`verify-the-claim` failure mode reappearing inside the change built to end it — worth recording
because the correct fact was already written down in this change's own plan.

**The completeness guard was one-directional.** `(2a)` drives its loop off the resolver's actual
export surface (mutation-proven: a new export key fails until documented) and `(2b)` allowlists the
four non-exported schema keys — together they prove every key the code reads is documented. Neither
proved the converse. A phantom key added to the example passed all three existing guards: `(2a)`
iterates export keys rather than example keys, the fidelity diff is blind because the resolver
simply ignores unknown keys, and the scope-tag awk was satisfied by a neighbouring key's comment
window. Closed by a new `(2c)` orphan-key check anchored on the consuming scripts rather than a
hand-maintained allowlist. Mutation-verified.

**Four retargeted asserts, two of which passed vacuously after the retarget.** Commit `c5b6ae4`
moved four asserts off this repo's `.docket.yml` (which stopped being the user-facing documentation
surface) onto `.docket.yml.example`. A vacuity pass found `rpa_of` returns the default `"false"` for
an *absent* key — so asserting `"false"` proved nothing — and the learnings `enabled`/`cap` regexes
were unanchored, so they matched anywhere in the file. Strengthened with an explicit active-key
assert and a block-scoped awk; the review independently re-derived and mutation-confirmed all four.

**ADR-0048 supersedes ADR-0039.** ADR-0039's decision was not wrong, but it was stated over two
artifacts this change deletes (`config.yml.example`, `tests/test_config_example.sh`). The mirror
rule survives relocated — the example's commented `agents.claude` block mirrors the wrapper
frontmatter, and the wrappers still lead — joined by two new invariants: example = resolver defaults
(test-enforced), and the must-update rule (every new config flag lands in the example, value plus
docs plus scope tag, in the same PR).

**Residual risk, stated rather than solved.** Key *presence* and exported-key *values* are
mechanically enforced; the ~300 lines of surrounding English are not. Every documented default and
behavioral claim in the example is an assertion about `scripts/docket-config.sh` or a skill body,
and a grep can only prove a sentence still exists, never that it is still true. ADR-0048 records
this as a known residual risk of the hand-maintained-mirror trade-off.

## Follow-ups

- **#0103 (minted this run)** — wire the `github_project` config read end to end, or decide the key
  should not exist. Documented, fenced, and described in the convention as minted-and-written-back;
  read by nothing.
- **#0102 (pre-existing)** — `finalize.require_pr_approval` has no working layer resolution: absent
  from `scripts/docket-config.sh`, no row in `scripts/docket-config.md`, missing from the fence loop
  at `docket-config.sh:169`. Its only consumer, `skills/docket-finalize-change/SKILL.md`, reads
  `.docket.yml` alone, so a value in `.docket.local.yml` or the global config is silently ignored
  with no warning. The example annotates this accurately instead of applying either standard scope
  tag. Same shape as #0103 — the two could be groomed together as one documented-but-unwired sweep.
- **Plan deviation, corrected during review.** The spec's *Consolidation edits* said setup step 2
  **and the Configuration section** retarget to the example; the plan narrowed Task 6 Step 4 to the
  two spots literally containing the string `config.yml.example`, and the build followed the plan.
  That left the README's full commented `.docket.yml` sample alive as a fourth all-keys surface —
  already drifted (no `learnings:` block, no mention of the new `auto` sentinel), i.e. the exact
  defect this change exists to end. Restored to the spec's intent: a five-key illustrative snippet
  plus a pointer to the example. Nothing tests the README against the example, so this remains a
  drift surface by construction.
- **No two-layer test for the `auto` sentinel.** `test_docket_config.sh` section S covers the
  committed layer in isolation. The property that after-resolution placement actually buys — a
  *higher* layer's `auto` masking a *lower* layer's real `test_command` — has a corrected comment
  (`docket-config.sh:197`) but no fixture pinning it.
- **Rebase deferred to the merge gate.** The branch is 5 commits behind `origin/main` (all change
  0099). The three-dot diff is clean and 0099 touches no file this change touches, so the rebase is
  conflict-free; `finalize.gate: local` rebases and re-runs the suite before merging by design.
