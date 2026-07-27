<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0117 — Deferred ADR-publish visibility — detect an unpublished ADR with a computed board-checks finding](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0117-deferred-adr-publish-visibility-decide-whether-docket-adr-s.md)**
<!-- docket:backlink:end -->

# Unpublished-ADR detection (`adr-unpublished`) — results

Change: #0117 · Branch: feat/deferred-adr-publish-visibility-decide-whether-docket-adr-s · Plan: `docs/superpowers/plans/2026-07-27-unpublished-adr-check-plan.md` · ADRs: 0061

## Verify (human)

The hermetic suite sees only its fixtures and the integration-branch checkout — it cannot see the
real ADR ledger, which lives on `docket`. The real-corpus behavior was therefore verified at build
time against the live tree. These are the observations to re-confirm at the merge gate if you want
them first-hand; each command is read-only.

- [ ] **Zero findings on the real corpus.** Expect no `adr-unpublished` line:

      bash scripts/board-checks.sh \
        --changes-dir /Users/homer/dev/docket/.docket/docs/changes \
        --adrs-dir /Users/homer/dev/docket/.docket/docs/adrs \
        --terminal-publish \
        --metadata-branch origin/docket --integration-branch origin/main

      Measured at build time: 60 ADRs on `docket`, 58 on `main`, zero byte drift among the 58
      present on both. The two absent from `main` — ADR-0023 (`change: 44`, `blocked`) and
      ADR-0060 (`change: 135`, `implemented`) — are both correctly not-due under the due rule.
      ADR-0061, minted by this change, is a third correctly-not-due case (`change: 117`,
      `in-progress`), so the count is now 61/58 and the expected output is still empty.

- [ ] **The zero is not a swallowed no-op.** Both positive controls fired at build time, on real
      bytes, with distinct messages. Re-run either if you want to see it:
      - *missing arm* — flip change 0135 to `done` in a throwaway copy of `docs/changes`, keep
        `--adrs-dir` pointed at the real (git-resident) ADR dir:
        `adr-unpublished<TAB>135<TAB>ADR-0060 is due on origin/main but absent — publish it (docket.sh terminal-publish --adr 0060)`
      - *stale arm* — cut a scratch branch off `origin/main`, append a line to ADR-0051 there,
        and pass that branch as `--integration-branch`:
        `adr-unpublished<TAB>83<TAB>ADR-0051 differs between origin/docket and <branch> — re-publish it (docket.sh terminal-publish --adr 0051)`
      Note the `--adrs-dir` must sit inside a git worktree — the check derives its repo-relative
      path with `rev-parse --show-prefix` and now exits 2 rather than silently mis-resolving.

## Findings

**The spec's `<change-id>` open question (§5), resolved.** ADR-0049 constrains that column to
script-derived or shape-validated values. The build kept the rule rather than widening the column:
a change-tied ADR emits its validated `change:` id, a standalone ADR emits `?` — the fallback
`padded_id_from_file` already uses for an unusable id — and the ADR number always rides the
**message** column, which is the last field the caller's `read` splits and therefore cannot shift a
field. No new precedent was minted.

**ADR-0061 — detect vs. mark.** The boundary the spec anticipated is recorded: *detect where there
is no marker seam and no healer; mark where a conscious human deferral is the failure mode.* It
narrows ADR-0051 rather than reversing it, and corrects one line of the record — `board-checks.sh`'s
inline comment had claimed a set-diff would "break the script's git-only/offline invariant", which
does not hold, since the script already runs `git cat-file -e <ref>:<path>` for both link checks.

**Four defects the green suite could not see, all found by review.** Worth recording because they
share one shape: every one lives in a repo state this repo does not have.

1. *(Critical)* `ADRS_DIR` is never empty — `docket-config.sh` always defaults and exports it — so
   `health_checks` forwarded `--adrs-dir` unconditionally, `board-checks.sh` exited 2 on a repo
   whose `docs/adrs` does not exist, and because the caller pipes into a `while read` loop that
   exit produced **zero** check lines. Every health check silently vanished, on a path
   `migrate-to-docket.sh` explicitly leaves reachable ("a fresh repo may lack adrs/"). Fixed at the
   caller with a `-d` guard; `board-checks.sh`'s exit-2 rule stays intact for hand-run callers,
   where a typo'd path must not be a silent skip.
2. *(Important)* `for af in "${ADR_FILES[@]}"` on an **empty** array throws `unbound variable` under
   `set -u` on bash 4.0–4.3. The repo's enforced floor is bash **major 4**
   (`ensure-docket-env.sh` checks the major only), not 4.4, so an ADR directory that exists but is
   empty — the normal state of a repo that opted in before writing its first ADR — aborted the
   script before its `FINDINGS` print. The same shape was fixed one file over in the same branch;
   both now use `${arr[@]+"${arr[@]}"}`.
3. *(Important)* The missing arm ignored `m_blob`, so an ADR present only in the working tree was
   reported as a publish gap — and the remedy the message printed would then fail, because
   `terminal-publish.sh` reads its copy-set from `$REMOTE/$META_BRANCH`. Reachable in practice: the
   health pass runs against the shared `.docket` worktree, which routinely holds another session's
   uncommitted work. Now silent, mirroring the stale arm's existing "nothing to compare against"
   reasoning.
4. *(Important)* An unchecked `rev-parse --show-prefix` fell back to an empty prefix, which would
   have made every ref lookup miss and reported **every** ADR as unpublished — the same
   silent-skip failure the neighbouring exit-2 guard exists to prevent, applied asymmetrically.

**A test that proved nothing, caught by mutation.** Task 1's first round claimed an `adr_pub`
fixture published to both branches; none existed, so `i_blob` was empty on every iteration and the
whole present-on-integration branch never executed — deleting it left the suite green. The fixture
was added and the mutation re-run: the same deletion now reddens three asserts. Recorded because
the comment describing the fixture was written before the fixture was, and the suite could not tell
the difference.

**`grep -E` with an escaped `\?` is a silent vacuity trap.** `\?` in an ERE is POSIX-undefined; where
it degrades, `^adr-unpublished\t?\t` matches *every* `adr-unpublished` line and the assert built on
it becomes vacuous. This file's own `has_finding` header already documented the trap. All such
asserts were normalized to fixed-string matching.

## Follow-ups

Two stubs minted (auto-capture), both discovered from this change:

- **#0144** (`chore`) — a `board-checks.sh` non-zero exit silently voids the entire health pass.
  Finding 1 above fixed the *trigger*; the swallowing remains, and the regression test written for
  it cannot see the failure it was written about (the mock exits 0 whatever its arguments, so the
  "still emits check lines" leg passes against fixed and unfixed code alike). Needs a mock that
  exits non-zero, and a decision on whether the best-effort posture is still the right contract now
  that a whole-pass loss can hide behind it.
- **#0145** (`docs`) — `skills/docket-status/SKILL.md` restates "Five mechanical checks" and lists
  five of the now-thirteen check-ids. Already stale before this change. It is not one of the four
  surfaces change 0111's correspondence guard pins, so every future check-id will drift there too
  while the guard stays green.

Reported, not minted: there is no bash 4.0–4.3 in this environment, so finding 2 was fixed on
code-identity with an already-proven fix rather than on a live repro. A version matrix would close
that class of gap, but it is a build-infrastructure question well beyond this change and no repo
state currently demands it.
