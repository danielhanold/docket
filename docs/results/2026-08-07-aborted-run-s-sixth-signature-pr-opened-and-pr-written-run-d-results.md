<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0219 — aborted-run's Step 7 seam — a fourth git-only leg, plus GitHub enrichment for leg C](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0219-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d.md)**
<!-- docket:backlink:end -->

# aborted-run's Step 7 seam — results

Change: #0219 · Branch: feat/aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d · Plan: docs/superpowers/plans/2026-08-07-aborted-run-step-7-seam.md · ADRs: 72

## Verify (human)

- [ ] **Leg D's real-world population is narrow by design — confirm you still want it.** The suite
      proves the predicate, but leg D's honest yield is *uncommitted partial edits in the shared
      `.docket` worktree, plus non-compliant drivers* that write `status:` and `pr:` separately.
      `board-checks.sh` reads change files off the filesystem, not out of a git blob, and
      `docket-implement-next` writes both fields in one stroke. It was built as a cheap, additive
      completeness guarantee over the Step 7 seam, not because it is a frequent signature.
- [ ] **The GitHub enrichment against a live repository.** Every test drives a stubbed `gh`. No
      automated test can reach the real API, so the first live pass is the only place the actual
      `gh pr list --repo … --state open --json number,headRefName` response shape is exercised —
      including whether `headRefName` matching behaves as expected against this repo's real branches.
- [ ] **The `--limit 200` ceiling and its `orphan-pr-skipped pr-list-truncated` guard.** The value is
      justified in a comment (gh's default of 30 truncates silently; 200 is two API pages at fixed
      cost) but it is a judgment call about *this* repo's open-PR volume. Confirm 200 is right, and
      that you are content with a truncated listing going quiet rather than guessing.

## Findings

**Review returned 7 findings (2 blocker, 2 important, 3 minor). All 7 were fixed in-branch.** The
two blockers are the ones worth reading, because both were cases where the code *looked* like it
honored a stated invariant and did not.

1. **Blocker — the enrichment's gate was not leg C's gate**, despite the spec, the code comment, and
   `docket-status.md` all selling "the two findings always agree" as load-bearing. The first
   implementation reused leg C's 2h idle floor and nothing else. It dropped the
   ahead-of-both-show-ref-verified-bases guard, so for a claimed change whose run died before its
   first commit, `git log -1` returned the *base* commit's date — past 2h — and the leg fired on the
   0109 "stopped with nothing built" signature that leg C deliberately stays silent about. It also
   asserted the word "pushed" without ever probing a remote ref. Neither defect was visible to the
   fixtures: the test helper gave every branch a real own-commit, and no fixture had a remote at all,
   so "is pushed" was being asserted true against branches that were never pushed. Fixed by mirroring
   leg C's whole predicate; the leg now emits **three** messages rather than two (the new arm is
   `<branch> was never pushed … the run stopped before pushing it`).

2. **Blocker — unanchored frontmatter reads** (`field` rather than `fm_field`) for the optional
   `pr:` and `branch:` keys in `detect_orphan_pr` — the ADR-0057 hazard, in a repo whose change
   bodies routinely open lines with `pr:`. Leg D on the same branch honored the rule and its
   complement did not, so the two legs could disagree about the same file. Notably, *no existing
   fixture could tell the two implementations apart*: every orphan fixture carried both keys in
   frontmatter. Fixed, with one absent-key fixture and one mutation arm per read.

3. **Important — a resolved `repo` was computed and discarded.** `gh repo view` was shelled and its
   result never spent: the `gh pr list` call passed no `--repo`, so it inferred the repository from
   cwd. Under a `--repo`-scoped pass this leg queried a different repository than the rest of the
   pass, and the documented `repo-unresolved` skip reason was unreachable. Fixed, with
   `detect_merged`'s owner/name validation adopted so the reason is genuinely reachable.

4. **Important — one network round-trip per candidate**, on a path this branch's own documentation
   repeatedly calls cost-sensitive, next door to a `detect_merged` that batches deliberately.
   Collapsed to a single `gh pr list` matched locally by `headRefName`.

5–7. **Minors** — leg D's code had been inserted *inside* leg C's comment block, leaving ~25 lines of
   leg-C rationale reading as preamble to leg D (the `shared-resource-keeps-first-owner-assumptions`
   shape in prose); two retargeted asserts were negative-only and would have passed vacuously if the
   finding vanished entirely; and `sweep-skipped` was being emitted by two unrelated subsystems onto
   one stdout, so an enrichment skip read as a merge-sweep skip. All fixed — the last by giving the
   enrichment its own `orphan-pr-skipped` token.

**ADR-0072 — leg C's predicate is duplicated by value across two scripts, never shared.** Recorded
because the decision *grew* during the build: the spec anticipated duplicating only the 2h constant,
and blocker 1 forced duplication of the whole predicate (ref resolution, the ahead-of-bases guard
with its empty-array count gate, the pushed/unpushed discrimination). The reason is
`board-checks.sh`'s independence — it must stay runnable offline with no dependency on
`docket-status.sh`. **The accepted cost is drift**: nothing links the two implementations, so a
future change that retunes leg C's floor or its base handling and forgets `detect_orphan_pr` breaks
the agreement silently, and no test will say so. Today's only mitigation is prose at each site
naming the other.

### Plan deviations worth knowing

- **The plan asserted leg C's fixtures would be unchanged byte-for-byte by the `ar_pr` hoist. That
  was wrong**, and four pre-existing asserts had to be retargeted — fixtures 235 and 243 plus
  mutations I and L — because each used *plain `aborted-run` silence* as a proxy for *leg C silence*
  on fixtures where leg D now fires by design. Each was narrowed onto leg C's exclusive
  `pr: is unset` clause. Leg C's behavior is genuinely unchanged; only these oracles moved. Review
  checked this specifically and confirmed no guard was weakened, though it did find two of the
  retargets still needed positive companions (minor 6, fixed).
- **Two plan fixtures could not discriminate and were replaced.** The "no candidate ⇒ never calls
  `gh`" witness was written against stdout, which is permanently vacuous because `gh repo view`'s
  output is captured into a variable and never printed; it became a side-effect witness with a
  companion assert proving the witness is not itself vacuous. This is the
  `green-suite-untested-branch` shape twice over.
- **Commit-message repair.** Four fix commits were authored with `(0227)` in their subject instead of
  `(0219)`. Corrected by a message-only rewrite before the branch was pushed; the tree hash was
  verified identical before and after, so no content changed.
- The plan document itself still quotes the pre-fix two-message shape and the per-candidate `--head`
  call. Left as a point-in-time record per `AGENTS.md`.

## Follow-ups

- **#0239 (auto-captured, `fix`)** — `detect_merged`'s own `gh pr list` fallback ignores `--repo`,
  the identical latent shape finding 3 fixed one function away. Under a `--repo`-scoped pass the two
  arms of one function query different repositories. Pre-existing, outside this change's diff, and
  deliberately not repaired here.
- **The duplication drift risk in ADR-0072 has no automated guard.** A correspondence test asserting
  the two scripts agree on the idle floor and the base-handling shape would close it. Not filed as a
  change — worth a human's judgment on whether it earns one.
- **Advisory over-budget in the suite runner** (`test_sync_agents*`) is pre-existing and untouched by
  this branch; the run exits 0 on it by design.
