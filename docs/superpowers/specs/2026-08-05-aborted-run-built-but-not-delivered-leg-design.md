<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0211 — aborted-run is blind to a run that stops after the build: commits on an unpushed branch, every field coherent](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-05-0211-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com.md)**
<!-- docket:backlink:end -->

# aborted-run leg C — built but not delivered

Design doc for change 0211.

## Problem

`aborted-run` (change 0113) has two legs, and both are structurally blind to the abort signature
observed on 2026-08-05 (the 0206 run, stopped at the Step 5/6 boundary):

- **Leg A** keys on manifest/git *incoherence* (`plan:`/`results:` unset while the branch carries the
  artifact). That run dropped no bookkeeping write — it dropped two whole steps — so every field
  leg A inspects was coherent.
- **Leg B** keys on a `claimed_at` older than 12h. 0113's own heartbeat rider re-stamps `claimed_at`
  at every phase boundary, so a run that dies immediately *after* a metadata commit starts leg B's
  countdown from the freshest possible stamp. Leg B is at its blindest exactly when a run has just
  completed a step.

The state that *was* visible: four build commits on `feat/…`, no remote tracking ref, `pr:` empty.
"Built but not delivered" — evidence of a different kind from either existing leg.

## Approach

Add **leg C** to the existing `aborted-run` block in `scripts/board-checks.sh`, inside the same
`status: in-progress` scope, after leg B. No new check-id (`aborted-run` stays the single id, so the
four-place `BOARD_CHECK_IDS` pinning is untouched), advisory and warn-only like the rest of the file,
never `EXPLAINED`, never a status or claim write.

### Predicate

Evaluated only when `status: in-progress`, in this order — cheapest gate first:

1. `pr:` (anchored `fm_field`) is **empty**. Non-empty ⇒ skip leg C entirely, zero further git calls.
   A change whose PR is recorded has delivered; "unpushed branch with a recorded PR" is an
   incoherent state leg C is not the oracle for.
2. `branch:` (anchored `fm_field`) resolves. Leg C sits after leg B, **outside** leg A's
   `if ar_ref="$(branch_ref …)"`, where a failed `branch_ref` leaves `ar_ref` *set but empty* — so
   reuse requires an explicit `[ -n "$ar_ref" ] || skip`, never the bare variable. Without it the
   idle floor evaluates TRUE for a branchless change (`git log -1 --format=%ct ""` is empty, and
   `$(( NOW - "" ))` is `NOW`). The value is reused, the guard is not optional.
3. **Idle floor**: the branch's newest commit is older than `ABORTED_RUN_IDLE_SECS`
   (`"$GIT" -C "$CHANGES_DIR" log -1 --format=%ct "$ar_ref"`, one call).
4. **Ahead of BOTH integration bases**: the branch carries a commit reachable from neither the local
   integration ref nor its remote-tracking twin. Feature branches are cut from
   `origin/<integration_branch>` while this script's `INTEGRATION_BRANCH` names the *local* ref, and
   in this repo local `main` routinely lags origin (`sync-integration-branch.sh` is FF-only and
   best-effort). Comparing against the local ref alone makes a freshly-cut, nothing-built branch look
   arbitrarily far "ahead", with an arbitrarily old newest commit — it would sail through the idle
   floor and fire leg C on the 0109 signature with a fabricated commit count. So the probe excludes
   both bases:

   ```sh
   ar_bases=()
   for ar_b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do
     "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "$ar_b" && ar_bases+=( "$ar_b" )
   done
   # No base resolves => no comparison is possible => SILENT (no positive evidence), the same
   # posture leg B takes for an unparseable claimed_at. Never "ahead of nothing".
   [ "${#ar_bases[@]}" -gt 0 ] || skip leg C
   "$GIT" -C "$CHANGES_DIR" rev-list -n 1 "$ar_ref" --not "${ar_bases[@]}" 2>/dev/null
   ```

   **Both** bases are `show-ref`-verified, not just the remote one: an absent
   `refs/heads/$INTEGRATION_BRANCH` makes `rev-list` exit 128 with empty stdout, and since the
   predicate reads "empty ⇒ not ahead", an asymmetric guard would silently turn the whole leg into a
   no-op with no diagnostic. The empty-array case is gated explicitly (`set -u` would otherwise trip
   on `"${ar_bases[@]}"` under an older bash). Every git call in this leg goes through the
   file's documented `GIT` mock seam and the `-C "$CHANGES_DIR"` anchor, matching `branch_ref` and
   `stale-in-progress`.

All four true ⇒ emit exactly one finding, with the message chosen by *which* delivery step is
missing:

- no `refs/remotes/origin/<branch>` (one `show-ref --verify --quiet`, run only at this point; the
  `origin` remote name is hardcoded, inheriting `branch_ref`'s existing convention rather than
  inventing one — and a *stale* remote-tracking ref left by a remote-side branch deletion reads as
  "pushed" and yields the other message, which is acceptable for an advisory finding whose remedy in
  both cases is "look at this run"):
  `"N commits on <branch> ahead of <integration>, branch never pushed and pr: is unset (last commit Nh ago) — the run stopped before it opened its PR; push and open it, or re-run the step"`
- remote ref present, `pr:` empty:
  `"<branch> is pushed but pr: is unset (last commit Nh ago) — the run stopped between the push and the PR record; open the PR or record it"`

The two are mutually exclusive by construction, so the emit site is a single `if/else` and exactly
one finding is possible per change.

The reported commit count is a display value only: `rev-list --count "$ar_ref" --not "${ar_bases[@]}"`
runs **only on the firing path**, where its cost is irrelevant and the count is what makes the finding
actionable.

Leg A and leg C can both fire on one change, and they already share the `aborted-run` id — so
`docket-status` prints two lines with different remedies for one change. That is the same shape legs
A and B already produce (the existing "BOTH legs" fixture), and the remedy text in each message is
self-contained, so no caller change is needed; the leg-C messages are worded to stand alone beside a
leg-A line.

### The idle floor

`ABORTED_RUN_IDLE_SECS=$(( 2 * 3600 ))` — a new hardcoded constant beside `ABORTED_RUN_STALE_SECS`,
same precedent as `FINALIZE_BLOCKED_STALE_SECS` and stale-in-progress's `3*86400`. The derivation
belongs in the comment: after the last build commit a healthy run still has
review, any ADR, the ~10-minute suite, and the push to get through — and a review-driven fix commits
and **resets the clock**, so the exposure is that tail, not the whole build. 2h covers it with room,
is six times tighter than leg B's 12h (the same ratio leg B took against the 72h lease), and the
comment must also state the residual: a marathon tail with no post-review commit will fire leg C on a
healthy run. That finding is free and self-clearing; a floor that never misfires would have to be so
loose it stops detecting.

The floor is keyed on the **branch's newest commit**, never on `claimed_at` — the heartbeat rider
makes `claimed_at` unusable here, which is the whole reason leg B misses this signature.

### Cost

Leg C adds **at most five** `git` invocations on the firing path (two base `show-ref`s, `log -1`, the
`rev-list -n 1` probe, and — only when it fires — the branch `show-ref` plus `rev-list --count`; the
message-selecting `show-ref` is genuinely new, because `branch_ref` skips the remote probe whenever
the local ref resolved), and **at most three** on any non-firing path, only for changes that are `in-progress` with an empty
`pr:` — the population legs A and B already walk. `rev-list -n 1` bounds output, not traversal; if a
cheaper form is wanted the honest one is `merge-base --is-ancestor "$ar_ref" <base>`, but it needs one
call per base and loses the count, so `rev-list` is kept. The common repo state (0-2 in-progress
changes, most with a recorded PR or no branch) adds **zero**. Ordering is deliberate: the free
frontmatter read gates the git calls, the reused `ar_ref` costs nothing, and the remote-ref probe
runs only after the idle and ahead gates have already decided the leg fires. This keeps the added
per-invocation cost bounded in the path change 0176 established as cost-sensitive.

### Tests (`tests/test_board_checks.sh`)

New ARM fixtures numbered from **227** upward (0202's spec claims 224-226; the build's reconcile
re-checks what has landed). Fixtures need two capabilities the existing `ar_branch` helper does not
have, so add a sibling helper rather than widening it:

- commits at a controlled age — set `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` from `NOW_EPOCH` offsets,
  the same way `AR_STALE_CLAIM`/`AR_FRESH_CLAIM` are derived;
- a *pushed* branch — `git -C "$repo" push -q origin <branch>`, which creates
  `refs/remotes/origin/<branch>` in the fixture's own bare origin (`new_repo` already repoints it).

**The clock skew in the existing fixtures is load-bearing and must be neutralized, not relied on.**
`NOW_EPOCH` is 1750000000 (2025-06-15) while `ar_branch`'s commits carry real wall-clock dates
(2026-08), so `NOW - ts` is hugely negative and *every* existing ARM fixture is silent for leg C only
by accident. Two consequences the build must handle:

- the existing single-finding asserts (e.g. "aborted-run fires exactly ONCE for id 201") are, after
  leg C, guarded by nothing but that skew — re-run them and, where a fixture's branch is ahead with
  `pr:` absent, pin the intent explicitly rather than leaving it to the delta's sign;
- **mutation 1's blast radius is wider than one fixture**: dropping the idle-floor comparison makes
  every skewed ARM branch fixture a leg-C candidate. Assert the mutation's effect on the named
  fixture and re-check every count-based assert in the ARM mutation repo.

Fixtures:

- **unpushed, idle 3h, ahead** — FIRES, message names "never pushed".
- **unpushed, idle 30m, ahead** — SILENT (the live-run window; this is the fixture that proves the
  floor is real).
- **pushed, `pr:` empty, idle 3h, ahead** — FIRES, message names the push/PR seam.
- **pushed, `pr:` set, idle 3h, ahead** — SILENT (the delivered state).
- **branch exists, zero commits ahead, idle 3h, `pr:` empty** — SILENT (the 0109 signature; leg B's
  territory, not leg C's).
- **stale local integration ref** — `checkout -b tmp main`, commit through the **dated** helper, then
  `push -q origin tmp:main`, which advances the fixture's own bare origin *and* its
  `refs/remotes/origin/main` without moving local `main` (no `fetch` needed; `new_repo`'s template
  already carries both refs). Cut the change's branch from `refs/remotes/origin/main` with no commits
  of its own, `pr:` empty. SILENT. The advancing commits **must** be dated relative to `NOW_EPOCH` —
  the branch tip *is* one of them, so with real wall-clock dates the idle floor is false and mutation
  2b could never fire. This is the fixture the
  original single-base predicate would have failed, and `new_repo`'s always-current `main` means it
  must be built deliberately — no existing fixture reaches this state.
- **leg C alongside leg A on one change** — both findings emitted, proving the legs stayed
  independent (the existing "BOTH legs" fixture's pattern).

Mutation tests, one mutation per predicate, each asserted landed by a `grep -c` before/after
transition per the file's house rule:

1. drop the idle-floor comparison ⇒ the 30m fixture starts firing;
2. drop the ahead-of-bases test ⇒ the zero-commits fixture starts firing;
2b. drop the remote-tracking base from `ar_bases` (single-base comparison) ⇒ the stale-local-`main`
    fixture starts firing;
3. drop the `pr:`-empty gate ⇒ the pushed-with-`pr:` fixture starts firing;
4. swap the remote-ref probe's sense ⇒ the two firing fixtures swap messages;
5. swap leg C's `fm_field "$f" pr` to `field` ⇒ a fixture whose **body prose** contains a `pr:` line
   with frontmatter `pr:` empty goes silent (the ADR-0057 anchored-read pin, the same shape 0202
   adds for `results`).

Mutation asserts run `armrun` against the single shared `$ARM` repo, so **every** leg-C fixture the
mutations reference (30m, zero-ahead, pushed-with-`pr:`, both firing fixtures, stale-local-`main`)
must be duplicated into `$ARM` with its own pushes and origin advance, following the existing 220-223
duplication pattern. Advancing `$ARM`'s origin `main` is safe for the other checks in that repo — leg
A, `merged-orphan`, and `unknown-commit-ref` all read the local `INTEGRATION_BRANCH`.

On `grep`: the machine's PATH `grep` is ugrep, which accepts constructs BSD grep rejects (bounded
repetition among them). This change introduces no such construct — the new asserts are literal
`-qF`/`-cF` matches and `grep -cE` with an embedded TAB, the same shapes the file already uses at
lines 929 and 1206 — so no double-run under `/usr/bin/grep` is required. The file's one existing
`/usr/bin/grep` call is a plain `-c .` line count, not a portability hedge for this class.

Also extend `scripts/board-checks.md` (the `aborted-run` section) with leg C and the new constant.

## Verification

`bash tests/test_board_checks.sh` and the full suite green. Every new predicate mutation-tested with
a landed-mutation assert.

## Out of scope

- Retuning leg B's 12h horizon or the heartbeat rider.
- Any status flip, claim release, or file write — `board-checks.sh` stays a pure reader.
- The prose half of the failure (change 0212).
- A config knob for the idle floor; hardcoded, promoted to a flag only if tuning is ever wanted.
- **A sixth abort signature: PR opened, `pr:` written, run dies before `status: implemented`.** Leg
  C's `pr:`-empty gate makes it invisible here and leg B catches it at 12h. Named deliberately as a
  follow-up rather than folded in — its evidence is a manifest/GitHub comparison, which is a
  different oracle (and `board-checks.sh` is git-only, no `gh`, by contract).

## Assumptions

Every decision below was defaulted autonomously; this is the audit trail.

1. **Idle floor over pure advisory self-clearing.** Chosen: a branch-idle floor. Rejected: the
   advisory/self-clearing posture leg A takes, because leg A's false-positive window is *seconds*
   (an artifact commit to its field write) while leg C's would be the **entire build span** — and
   `board-checks.sh` runs on every Board pass, including passes inside the very run being built. A
   check that fires on every healthy build is noise, and 0113's stated value is credibility.
2. **The floor is 2h.** Rejected 45m (a quiet review + full-suite stretch after the last build commit
   is plausibly longer) and 12h (that is leg B; a third leg with leg B's horizon adds nothing over
   leg B). 2h detects within a `/loop` iteration or two and keeps the same 6× tightening ratio leg B
   used. Hardcoded, per this file's three existing horizon constants.
3. **Floor keyed on the branch's newest commit, not `claimed_at`.** The stub names this constraint;
   the heartbeat rider re-stamps `claimed_at`, so any claim-keyed floor reproduces leg B's blindness.
4. **One leg with two messages, not two legs.** The expensive half (branch resolution, idle, ahead)
   is shared, and the two disjuncts are mutually exclusive once ordered, so a single emit site with
   a message branch gives the distinct diagnostics the stub asks for without duplicating the probe
   or splitting the check-id. Rejected: two independent legs, which would double the git calls in a
   cost-sensitive path and could emit two findings for one fact.
5. **`pr:` non-empty short-circuits the whole leg.** Rejected: keeping the unpushed-branch disjunct
   live when `pr:` is set. That combination means the PR record and the remote disagree, which is a
   different defect with a different remedy; leg C would be a misleading oracle for it, and gating on
   the free frontmatter read is what keeps the common case at zero git calls.
6. **Both integration bases, local AND remote-tracking.** An earlier draft used `INTEGRATION_BRANCH`
   verbatim on the grounds that a stale local ref "only makes the branch look more ahead" — which is
   exactly the false-positive direction, and neither the idle floor nor the `pr:` gate blocks it: the
   inherited commits are old, so the floor *helps* the misfire. Since feature branches are cut from
   `origin/<integration_branch>` and local `main` routinely lags it here, excluding both bases is the
   only correct predicate. Both bases are `show-ref`-verified symmetrically — guarding only the
   remote one would let an absent local `refs/heads/<integration>` turn the whole leg into a silent
   no-op (`rev-list` exits 128 with empty stdout, which the predicate reads as "not ahead") — and
   "no base resolves" is silence, never a finding.
7. **No new check-id.** `aborted-run` is already declared in `BOARD_CHECK_IDS` and pinned in four
   places; leg C is more evidence for the same conclusion ("this run stopped mid-step"), so a new id
   would buy a four-file edit and a second remedy vocabulary for nothing.
8. **Fixture ids start at 227.** 0202's spec claims 224-226 and lands first; the build's reconcile
   re-checks the actual high-water mark.
9. **`depends_on: [202]`, not merely `related:`.** 0202 rewrites `branch_only_artifact` (NUL-delimited
   `ls-tree` via a process-substituted redirect) and adds ARM fixtures in the same two files leg C
   touches. Leg C does **not** call `branch_only_artifact`, so this is not a code dependency — but
   landing 0202 first means leg C is added to hardened predicates and its fixture numbering is
   settled, and the stub's own framing asks for that order. Recorded as `depends_on` per the
   dispatching instruction; `related: [113, 212]` records the origin and the prose half. Forward link
   only — the reciprocals on 0113/0202/0212 are not written.
10. **New fixture helper rather than widening `ar_branch`.** `ar_branch` is called by every existing
    ARM fixture; adding date control and a push to it would change their behavior. A sibling helper
    keeps them byte-identical — which is **not** the same as unaffected: leg C changes what those
    fixtures could emit, and the *Tests* section above states the re-check that byte-identity does
    not buy.
11. **`depends_on` is a hard readiness gate.** Satisfied only when 0202 reaches `done`, so if 0202
    stalls or is killed, this high-priority fix is unbuildable until a human edits the field. Accepted
    over `related:` because the two changes edit the same block of `board-checks.sh` and the same
    fixture region, and building on hardened predicates is the point.
12. **Leg C re-guards `ar_ref` explicitly** rather than trusting leg A's assignment. A failed
    `branch_ref` leaves the variable set-but-empty, and leg C runs outside leg A's `if`; the empty
    value makes the idle floor evaluate TRUE. The value is reused for the call it saves, the
    emptiness check is not optional.
13. **Every git call uses the `GIT` mock seam and the `-C "$CHANGES_DIR"` anchor**, per the file's
    documented seams; `origin` is hardcoded in the branch probe, inheriting `branch_ref`'s convention.
