<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0247 — Make shared metadata worktree contention survivable and scope its commits](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0247-make-shared-metadata-worktree-contention-survivable-and-scop.md)**
<!-- docket:backlink:end -->

# Make shared metadata worktree contention survivable and scope its commits — design

Change: 0247 (consolidates killed #0110 and #0119; their analysis, including #0119's two-pass
critic-settled findings, carries over and is reused below rather than re-derived).

## Problem

Two halves of one concurrency defect in the shared `.docket` metadata worktree:

1. **Contention.** `scripts/lib/docket-preflight.sh` syncs with a bare
   `fetch && pull --rebase || return 1` — no retry, no discrimination. The shared worktree is
   dirty for another agent's entire multi-tool-call edit→commit window (seconds to minutes), so a
   concurrent preflight hard-fails on a perfectly normal transient state. Observed live (0109);
   reproducible by construction. No lock exists anywhere in `scripts/`.
2. **Blast radius — script channel.** Two pathspec-less `git commit` calls in `scripts/docket-status.sh` — the one
   inside `commit_and_push_generated` and the sweep's `"docket($id): refresh artifacts links"`
   commit — sweep up whatever another agent has staged at that instant, committing someone else's
   in-flight work under this run's message and push. (#0119's audit confirmed these are the only
   two exposed sites; every other shared-tree commit already carries a pathspec, and
   `terminal-publish.sh`'s `$pub` commit is exclusive-worktree **and** index-driven, so it stays
   out of scope — a pathspec there would change behavior, not harden it.)
3. **Blast radius — agent channel** (added 2026-08-09). The same defect reaches the shared tree
   through a second, unguarded channel: **git commands an agent runs directly from skill prose.**
   Not one skill body instructs staging by pathspec. `docket-convention`'s Step-0 preamble grants
   the direct-git authority — *"plain git plumbing (`git add`/`commit`/`push`, `git -C` forms)
   stays direct"* — and constrains it not at all; the six metadata-writing skills then say only
   *"Commit the change-file edit + spec together in the metadata working tree"* (`docket-groom-next`
   Step 5) or the equivalent. The one place the discipline **is** written down,
   `docket-build-task` (*"Stage by explicit path — only paths your task changed. Never
   `git add -A`, `git add .`"*), governs feature-branch commits — a **private** worktree, where the
   hazard does not exist. The rule is present exactly where it is not needed and absent exactly
   where the shared tree makes it load-bearing.

   **Observed live, 2026-08-09.** During an interactive `docket-groom-next` of #0270, two
   concurrent autonomous commits each swept up the interactive session's staged files: first the
   staged rename (into `docs(0279): auto-groom to build-ready…`), then, after re-staging, all three
   files (into `docket: arm 0195 0265 0272 for auto-groom (wave 5, triage 2026-08-09)`). The
   groomer's own `git commit` returned **"nothing to commit, working tree clean."** Content survived
   and pushed; the commit message did not, so the groom's entire rationale existed only in the
   artifacts. Neither swallowing commit was a script commit, so Half 2's `scripts/**/*.sh` guard
   would have stayed green through both. The same session then hit Half 1's contention defect on a
   later `preflight` (`cannot pull with rebase: You have unstaged changes`) — independent
   corroboration that both halves are live, not theoretical.

## Requirements checklist

The normative summary — what a build must satisfy, each item specified in full in its half. The
rest of this document is the decision record and evidence trail behind these lines.

- No rebase when the fetched remote has not advanced; a dirty tree with nothing to pull never
  fails the sync (Half 1 item 1).
- Diverged history rebases local commits onto the fetched remote only when the tracked tree is
  clean; both branches of the sync function behave identically (Half 1 items 1–2).
- Bounded retry (5 attempts, 2/4/8/8s) is spent only on failure classes consistent with transient
  contention; a content conflict raised by this attempt's own rebase aborts and fails
  immediately; the exhaustion diagnostic names the last failure class (Half 1 items 2, 4).
- Never `--autostash` in any metadata-tree sync path, repo-grep asserted (Half 1 item 3).
- Untracked-only files never count as dirty (Half 1 item 4).
- The backoff sleep is injectable; fixture tests never wait real time (Half 1, tests).
- Both `docket-status.sh` commit sites carry `--` pathspecs; a wedged tree yields
  `blocked-wedged-tree`, which `--must-land` treats as not-landed (Half 2 items 1–2).
- Default-deny shape-keyed guard over pathspec-less commits in `scripts/**`: masked text,
  per-segment predicate, explicit driver set, keyed exceptions with existence floor,
  mutation-tested (Half 2 item 3).
- Every metadata-writing skill's commit instruction carries `Stage by explicit path`; the
  convention's grant sentence states the rule; coverage is derived from the `docket.sh preflight`
  command string; both guard groups mutation-tested, matching reflow-proof (Half 3).
- Skill size-budget raises satisfy change 0201's in-diff argument and change 0137's rounding
  (Half 3 item 4).

## Architecture decision — survivable, not impossible

**Keep the single shared `.docket` worktree; make collisions survivable with a bounded,
discriminating retry in the preflight sync.** Rejected alternatives:

- **Per-session metadata worktrees** (collisions impossible): a large architecture change needing
  a mint/lease/prune lifecycle, N checkouts, and a rewrite of every shared-tree invariant, guard,
  and learning built since (the `no-checkout-in-shared-worktree` family, ADR-0046's clean-tree
  gates, #0119's whole framing). Today's real concurrency level is "one interactive session
  overlapping one autonomous loop" — parallel fan-out is #0008, which is *deferred*. Buying the
  heavyweight answer ahead of the workload that would justify it is the wrong default; if #0008
  revives, this decision is its natural re-opening point (recorded in the ADR).
- **Advisory lock** around the write→commit→push critical section: the section spans multiple
  tool calls, so no single script can own the lock, and a crashed agent strands it — the
  lease/expiry machinery needed to fix that is most of the per-session cost with none of the
  benefit.
- **Shrink the dirty window** (direct skills to write-and-commit in one call): narrows but never
  closes the race, and is prose discipline rather than mechanism; worth doing opportunistically
  but not as the fix. **This rejection stands, and is narrower than it first reads** — it rejects
  prose as a *race* fix. It does not reach prose as a *blast-radius* fix, which is Half 3: bounding
  what a lost race can damage is the same correctness argument Half 2 already accepts, and staging
  by pathspec is as absolute in an agent's commit as in a script's. Half 3 was added 2026-08-09
  after the agent-authored channel was observed live (below); the original spec addressed only the
  script channel.

Record the decision as an ADR (survivable-over-impossible; correctness-over-availability posture
below) via the standard build-time `docket-adr` step; list it in `adrs:`.

## Half 1 — preflight sync: bounded, discriminating retry

In `scripts/lib/docket-preflight.sh`'s metadata-worktree sync (both the worktree and the
main-mode branch of the sync function):

**Invariant.** The sync may report success when local metadata is already current with, or ahead
of, the fetched remote. It must integrate remote changes only when the tracked tree is clean and
no git operation is in progress in the shared tree, and it must never mutate another agent's
in-flight state to get there (no autostash, no reset). Everything below implements this
invariant; review the implementation against it.

1. **Fetch first, then decide.** After `git fetch origin <metadata_branch>`, compare
   `HEAD` to the fetched remote ref. **If the local branch is already up to date (or ahead only),
   skip the `pull --rebase` entirely** — a dirty tree with no remote movement must never fail the
   sync. This alone removes the most common collision (the other agent has not pushed yet).
   **Diverged history** — local commits *and* remote movement — takes the rebase path in item 2:
   local commits rebase onto the fetched remote, under the same precondition (clean tracked
   tree). Both branches of the sync function must behave identically here; leaving it implicit is
   how they drift apart.
2. **If the remote moved and the rebase is needed**, attempt it; on failure, retry the whole
   fetch→compare→rebase step with backoff: **5 attempts, sleeping 2s, 4s, 8s, 8s between them
   (~22s total budget)**. The collision window is "another agent between edit and push"; most
   windows close in seconds once the other agent commits, and an autonomous caller re-running
   preflight later covers the long tail. The budget is a constant at the top of the function with
   a comment naming this rationale (per `tolerance-constant-calibrated-on-one-machine`: record the
   reasoning, not just the number).

   **Classify each failure before sleeping; spend retries only on classes that can self-heal.**
   Retryable: a dirty tracked tree (another agent mid-edit), an in-progress rebase/merge that
   *predates this attempt* (another agent mid-sync — transient unless it is a crashed agent's
   leftover, which only the exhaustion diagnostic can call wedged), and fetch/ref-lock races.
   Not retryable: a content conflict raised by *this attempt's own* rebase — deterministic, it
   fails identically on every retry — so `git rebase --abort` (restoring the pre-attempt state)
   and fail immediately with the conflict named, spending no further budget. Fetch failures retry
   undiscriminated: git's exit codes do not portably separate an auth or bad-remote failure from
   a transient network one, and stderr-pattern matching is locale/version-fragile — the
   diagnostic carries the last stderr instead (accepted limit, stated).
3. **Never `--autostash`** — on a shared tree it stashes another agent's in-flight edits (#0110).
   Assert this with a repo grep in the test file (no `--autostash` in any metadata-tree sync
   path).
4. **On exhaustion, fail with a discriminating diagnostic**, still `return 1` (fail-closed, as
   today): name whether the blocker was a dirty tracked tree (`git status --porcelain
   --untracked-files=no` non-empty — likely another agent mid-write, or a human's leftover; retry
   later or inspect) or an in-progress rebase/merge in the shared tree (wedged — needs a human),
   versus an ordinary fetch/network failure — **and always name the last attempt's failure
   class**, so the caller sees what actually blocked the sync, not just that five attempts died.
   Untracked-only files must not count as dirty (ADR-0046's two-sided lesson).

No lock file, no new state, no new config knob. **The backoff sleep must be injectable** — an
overridable function or env hook read by the retry loop — so fixture tests drive all five
attempts without real waiting; the suite's per-file wall-clock budgets make a real ~22s of sleeps
in a test a defect, not a style choice. Tests: a fixture-repo test exercising (a)
dirty-tree + no-remote-movement → success without rebase; (b) dirty-tree + remote moved →
retries, then succeeds once the fixture "other agent" commits; (c) exhaustion → non-zero with the
discriminating diagnostic naming the last failure class; (d) untracked-only file never fails the
sync; (e) a conflicting local commit → immediate abort-and-fail without burning the retry
budget, tree restored to its pre-attempt state.

## Half 2 — scope the two commits; wedged-tree posture

1. Scope both exposed sites: add `-- "$rel"` to `commit_and_push_generated`'s commit, and scope
   the sweep's refresh-artifacts-links add/commit pair with `-- "$archived"` (today its add
   carries a bare `"$archived"` without `--` and its commit carries no pathspec at all). The
   #0083 mark path in the same file is the local precedent idiom to copy.
2. **Wedged-tree posture: halt, with a new token — never overload `push-failed`.** A pathspec
   commit exits 128 mid-rebase/mid-merge where the old pathspec-less form exited 0 — but that old
   exit 0 *committed an interrupted rebase's staged content under a board-refresh message*, which
   is corruption, not availability. So: before committing, probe the shared worktree for an
   in-progress rebase/merge (`rebase-merge`/`rebase-apply`/`MERGE_HEAD` under `git -C "$mw"
   rev-parse --git-dir`); if wedged, `commit_and_push_generated` returns a **new report token
   `blocked-wedged-tree`** (no commit attempted). `docket-status.sh`'s report lines carry it
   through (`board inline blocked-wedged-tree`, same for the learnings index);
   `--must-land` treats it as not-landed (exit non-zero → the autonomous caller STOPs and
   abort-and-reports, per the convention); best-effort callers log and continue, as they do for
   `changed-push-failed` today. Update `scripts/docket-status.md`'s report-line vocabulary and
   exit-code mapping accordingly; `changed-push-failed` keeps its exact current meaning.
   Note Half 1 makes the wedged state rarer and shorter-lived at its source; this posture is the
   backstop, not the common path.
3. **Shape-keyed guard**, new `tests/test_shared_worktree_commit_scope.sh` (auto-discovered):
   default-deny — flag every `git … commit` in `scripts/**/*.sh` lacking a `--` pathspec, with a
   justified exception list for exclusive-worktree sites keyed `<basename>:<-C target var>`
   (existence floor so a stale exception reddens; never line numbers, ADR-0054). Build it per
   #0119's prototyped findings, which are adopted verbatim as requirements: run detection on
   **quoted-string-masked** text (raw text false-positives on `reclaim-claims.sh`'s `;`-bearing
   message and `mint-stub.sh`'s ` #$FROM`); evaluate the predicate **per `;`/`&`/`|` segment**,
   never whole-line (demonstrated false negative on the #0083 mark path); match `commit` as an
   exact-token git subcommand under an explicit driver set that includes `git`, `$GIT`, **and**
   `docket-config.sh`'s local `g` wrapper (which writes the metadata branch). Mutation-test the
   guard: strip a pathspec from one of the two fixed sites and watch it redden.

   **Contract boundary, stated:** the guard detects `commit` as an exact-token subcommand under
   the explicit driver set only — it is not, and must not grow into, a general shell parser. A
   commit issued through a driver spelling outside the set is outside the guard's contract
   (accepted limit; the set is small because the repo's metadata-writing drivers are), and
   introducing a new driver means extending the set in the same change — a review obligation,
   not something the guard infers.

## Half 3 — scope the agent-authored commits; state the rule where it is read

Same invariant as Half 2 — *no commit in the shared metadata worktree stages anything it did not
write* — applied to the channel a shell guard cannot see. Half 2 hardens the mechanism; Half 3
hardens the instruction.

**Marker.** Reuse the phrase already in the repo rather than minting a second idiom:
**`Stage by explicit path`** (from `docket-build-task`). One house token, greppable, already
carrying the meaning. The guard keys on this literal string.

1. **State the rule at the grant.** `docket-convention`'s Step-0 preamble sentence that authorizes
   direct git plumbing is where the authority is issued, so it is where the constraint belongs:
   direct `git add`/`commit` in the metadata working tree stages **by explicit pathspec**, never
   `-A`/`.`/`-a`, because the tree is shared and a bare add commits whatever another agent has
   staged at that instant. Name the observed consequence in one clause (another agent's work
   committed under your message) — a rule whose cost is stated survives a slim; a bare imperative
   does not.

2. **Carry the marker at every call site.** Each metadata-writing skill's commit instruction states
   `Stage by explicit path`. This is deliberately redundant with (1), and the redundancy is the
   point: **a standing instruction already in context demonstrably loses to a specific instruction
   at the moment of action.** That is not a guess — it is the finding
   `tests/test_skill_handoff_precedence.sh` was built on (its own header records run 40, where the
   wrapper's abort-and-report rule and §5's resolved-build statement were *both* in context and the
   sub-skill's prompt still won). A convention-only fix would repeat the mistake that guard exists
   to prevent.

3. **Coverage guard**, extending `tests/test_shared_worktree_commit_scope.sh` (same file as Half 2 —
   one invariant, two channels, one guard; a second file would split the exception lists). Built on
   the two-group shape of `test_skill_handoff_precedence.sh`:
   - **Group 1** — `docket-convention`'s Step-0 preamble states the rule: scope to the section
     (`awk` range on the heading, as the handoff guard does), then assert it names the pathspec
     requirement and the marker.
   - **Group 2 — coverage, sites DERIVED, never hand-listed** (AGENTS.md: enumerated floor). A
     skill is in scope iff its body **invokes `docket.sh preflight`** — the convention's Step-0
     preamble, which is what *makes* a skill an operating skill that reads and writes on
     `metadata_branch` — with `docket-convention` excluded as the rule's home. Verified 2026-08-09,
     that derivation yields exactly the right seven: `docket-implement-next`, `docket-groom-next`,
     `docket-auto-groom`, `docket-status`, `docket-new-change`, `docket-finalize-change`,
     `docket-adr` — and excludes `docket-build`/`docket-build-task` (feature worktree),
     `docket-review`, `docket-brainstorm`. Each in-scope body must carry the marker.

     **Key on the command string, not on prose describing it.** The obvious predicate — "the body
     names the metadata working tree" — yields the same seven today but is keyed on a *spelling*,
     which AGENTS.md forbids for exactly the reason visible in `docket-adr`: it already uses the
     variant "metadata tree" on **three** lines (four occurrences) against the canonical phrase on
     **two**, so an ordinary slim that normalizes its two canonical mentions to its own dominant
     idiom would silently drop it from coverage — a false green in the one channel Half 3 exists to
     guard. Nor is the Step-0 "All reads and writes land in the metadata working tree" line the
     structural anchor it looks like: only four of the seven carry any form of it. `docket.sh
     preflight` is a literal invoked command, immune to both reflow and rewording. It is not
     logically undroppable — `docket-finalize-change` and `docket-adr` carry it exactly once, in
     the Step-0 gloss — but it is the most stable anchor available: all seven carry it in that
     stereotyped line, five also carry it at mid-run re-sync sites as executable instruction, and
     unlike the prose predicate it has no observed drift. (`Step-0 preamble` as a phrase yields the
     identical set and may be asserted alongside it as a cheap second signal; the command string is
     the load-bearing one.)
   - Mutation-test both groups: strip the marker from one skill body and from the convention
     sentence; each must redden its own assert.

   **`docket-status` is in scope on purpose,** though its commits are made by
   `docket-status.sh` (Half 2's territory): the convention's Tier-A rule has the agent run that
   same work **inline** when dispatch is unavailable, so the prose must carry the discipline the
   script does.

   **Two accepted limits, stated rather than papered over.** (a) Only two of the seven skills have
   a commit-bearing heading (`### Step 5 — Commit, push, board`), so Group 2 is a **file-level**
   token check for the other five — the marker could sit anywhere in the file and pass. Scoping to
   a heading would silently skip five of seven, which is worse; the realistic drift (a marker
   deleted or reflowed away) is still caught. (b) A skill that grows a *second* commit site is
   covered by the file's single marker. Both need contrived prose to exploit.

4. **Skill size budgets — a required, verified part of this half.** `tests/test_skill_size_budgets.sh`
   fails any skill that grows past its row.

   **Re-measured 2026-08-11 (reconcile) — the 2026-08-09 figures below are superseded and the
   ranking inverted.** Eight changes merged in between and several raised these very rows, so the
   headroom moved in both directions. Current word headroom: `docket-auto-groom` **14** (was 32 —
   now the *tightest*), `docket-implement-next` **30** (was 11), `docket-new-change` **46** (was
   14), `docket-convention` **50** (was 14), `docket-finalize-change` **52**, `docket-groom-next`
   **53**, `docket-status` **58**, `docket-adr` **128**. Line headroom: `docket-auto-groom` and
   `docket-new-change` **4**, `docket-finalize-change` and `docket-groom-next` **5**,
   `docket-implement-next` **6**, `docket-adr` and `docket-convention` **8**, `docket-status` **16**.
   The build **re-measures again before setting any row** — these numbers are a planning input, not
   an authority, and nothing here may be copied into a budget row unmeasured.

   Prefer absorbing the marker into an existing line over adding one; every file's line headroom is
   in single digits except `docket-status`.

   **A raise must satisfy the test header's full rule, which is stricter than "edit the number."**

   Per change 0201 it must additionally **name the `references/` file the new prose was considered
   for and argue in-diff why it cannot live there** — "no other home" is argued, not asserted. Here
   that argument is available and should be made explicitly rather than waved at: the marker is *a
   rule that must intervene at the moment of action* — the header's own first example of prose that
   cannot be moved behind a pointer, and the whole basis of Half 3 item 2. Apply change 0137's
   rounding as well: lines to the next multiple of 5, words to the next multiple of 50, and if that
   lands within 25 words of the actual, take the multiple after it — near-zero headroom is the
   failure mode the table exists to forbid, not a tight fit to aim for.

## Out of scope

- **Enforcing the discipline on the agent at runtime.** Half 3 guards what the *instructions* say,
  not what a model does with them; no in-repo test can be the oracle for the latter. A pre-commit
  hook in the shared checkout was considered (2026-08-11 review) and rejected: a hook sees the
  index, not how it was staged, so a "suspicious broad staging" heuristic cannot be told apart
  from a legitimate multi-file human commit — and it is the runtime-machinery class the
  architecture decision declines. The tradeoff, bluntly: **the agent-facing layer (Half 3)
  reduces risk; the script layer (Half 2) provides the enforceable guarantee** — plus the fact
  that a correctly-scoped commit by every *other* agent already makes an unscoped one harmless
  to them.
- **Shrinking the write→commit window to a single tool call.** Rejected above and still rejected:
  it narrows a race rather than bounding damage, and unlike the pathspec rule it has no checkable
  shape. (The 2026-08-09 incident is not evidence for it — with pathspec-scoped commits on the
  other side, the window's width would not have mattered.)
- The sweep's skip-publish marking (#0118 — which edits adjacent `docket-status.sh` lines; build
  order should watch for the file collision, recorded in `related:`).
- Parallel backlog drain (#0008, deferred) — this change only makes today's
  interactive+autonomous overlap safe; #0008's revival is the trigger to revisit per-session
  worktrees.
- The push-side CAS loops (already correct) and feature-branch commit paths (per-change
  worktrees, not shared).

## Assumptions

Every decision an interactive brainstorm would have raised, the chosen default, and why — the
human's deferred audit trail.

1. **Core fork: survivable (retry) over impossible (per-session worktrees).**
   Chosen: bounded discriminating retry, shared tree kept. Rejected: per-session worktrees
   (heavyweight lifecycle machinery ahead of a workload that is deferred in #0008; invalidates the
   shared-tree guard/learning corpus), advisory lock (cannot span tool calls; strands on crash),
   dirty-window shrinking alone (doesn't close the race). This is the conservative default: the
   smallest mechanism that fixes the observed failure, reversible, and it leaves the per-session
   door explicitly open at #0008's revival. The stub's open question flagged this fork for Daniel;
   the stub was nonetheless armed `auto_groomable: true` at the same triage that wrote that line,
   which we read as authorization to attempt the conservative default with a full audit trail —
   if that reading is wrong, rejecting this spec (or answering the fork differently) reopens only
   Half 1's mechanism; Half 2 is valid under either fork until the shared tree is actually
   removed.
2. **Retry parameters: 5 attempts, 2/4/8/8s backoff (~22s), constant with rationale comment.**
   Rejected: unbounded retry (an autonomous loop must terminate), a config knob (no evidence yet
   that any repo needs a different budget — add the knob when a second calibration exists).
3. **Up-to-date fast path before rebasing.** Chosen: fetch, compare, skip the rebase when the
   remote hasn't moved. Rejected: always rebasing (fails on a dirty tree even with nothing to
   do — the most common collision). Low risk: pure narrowing of the failure surface.
4. **Wedged-tree posture: halt with a new `blocked-wedged-tree` token** (the #0119 abstain's
   decision 1). Chosen: correctness over availability. The "availability" the old behavior bought
   was committing a half-rebased shared tree's staged content under this run's message — itself a
   corruption bug, not a feature to preserve; the convention's own doctrine for an autonomous run
   meeting an abnormal precondition is abort-and-report; and Half 1 makes the state rare and
   self-healing at the source. A new token (not overloading `push-failed`) keeps
   `scripts/docket-status.md`'s existing vocabulary meanings intact — the #0119 abstain
   identified overloading as a documented-contract rewrite, so we don't. Rejected: skip-and-
   continue silently (a must-land board pass that silently didn't land violates the board's
   never-trails invariant), attempting the commit anyway and mapping 128 to `push-failed`
   (mislabels, and the abstain traced it to a confusing three-retry death anyway).
5. **Guard design adopted verbatim from #0119's settled critic findings** (masked text,
   per-segment predicate, explicit driver set including the local `g` wrapper, default-deny with
   keyed exception list + existence floor). These survived two adversarial passes already;
   re-deriving them would only lose fidelity.
6. **`terminal-publish.sh:$pub` stays unscoped and on the guard's exception list** — exclusive
   `mktemp -d` worktree, index-driven commit; a pathspec would change behavior (#0119, settled).
7. **One ADR** records the survivable-over-impossible choice and the halt posture; no new config
   knobs, no new lifecycle state. **Half 3 adds no second ADR** — it applies the blast-radius
   decision already recorded to a second channel; the ADR's *Consequences* names both channels.
8. **Couplings**: `related: [8, 118, 253]` (forward links only — #0008's revival re-opens the fork;
   #0118 collides on `docket-status.sh`; #0253 owns the prose-guard house pattern Half 3's Group 2
   must follow, see 14). #0110/#0119/#0130 are archived; `discovered_from:
   [110, 119]` already records lineage. No `depends_on:` — nothing must merge first.
9. **Half 3 folded in rather than filed separately** (2026-08-09, human decision). It is the same
   invariant, the same review, and it edits the same guard file; a separate change would split one
   invariant across two PRs and two exception lists. Cost accepted: this change grows from two
   halves to three, and now touches `skills/**` as well as `scripts/**` and `tests/**`.
10. **Marker is `Stage by explicit path`, reused from `docket-build-task`, not a new token.**
    Rejected: minting a `STAGE:`-style token parallel to `DIRECTED to:` (two idioms for one
    discipline; the existing phrase already reads as an instruction and is already greppable).
11. **Both the convention rule and the per-site marker, not either alone.** Rejected:
    convention-only (evidence-backed as insufficient — see Half 3 item 2 and run 40), and
    per-site-only (leaves the direct-git *grant* sentence still unconstrained, so a new skill
    inherits the authority without the limit). The redundancy is deliberate.
12. **Sites derived by the invoked command `docket.sh preflight`, not by prose, heading, or an
    enumerated list.** Rejected: **"names the metadata working tree"** — the first draft's
    predicate, and the one an implementer will reach for again, so it is recorded here as rejected
    rather than omitted: it yields the same seven today but is spelling-keyed, and `docket-adr`
    already carries the escaping variant ("metadata tree" 3× vs the canonical 2×), so a routine
    slim silently drops it (AGENTS.md: the spelling you miss is the target file's own house
    idiom). Also rejected: the Step-0 "All reads and writes land in…" sentence as the anchor (only
    4 of 7 carry any form of it); heading-keyed derivation (only 2 of 7 have a commit heading —
    it would false-green on five); and default-deny over every `commit` mention in `skills/**`
    (104 matching lines, overwhelmingly prose; the exception list would exceed the guarded set and
    rot immediately). Caught by the 2026-08-09 critic pass.
13. **`docket-build`/`docket-build-task` stay out of scope.** Their commits are feature-branch, in
    a per-change worktree that is not shared; `docket-build-task` already carries the discipline.
    Including them would imply the shared-tree hazard applies there and dilute the rule's reason.
14. **Half 3's marker check must be reflow-proof, and that is a live coupling to #0253.** A bare
    `grep -qF 'Stage by explicit path'` reddens the moment an editor rewraps the sentence across a
    line break — the exact fragility #0253 is settling with `flatten()` in a sourced
    `tests/lib/prose_guard.sh`. **If #0253 has merged, source that helper**; if not, flatten
    locally in the same single-gap shape #0253's house rule prescribes and leave a comment naming
    #0253 as the consolidation target, so the follow-up is mechanically findable. Do **not** add a
    `depends_on: [253]` — either order builds; only the implementation detail differs. Group 1's
    section-scoped asserts inherit the same requirement.
15. **2026-08-11 spec-review refinements (human-directed).** (a) Retry classification moved ahead
    of the sleep: only transient-contention classes spend budget, and this attempt's own rebase
    conflict aborts and fails immediately. The reviewer's "stop immediately on an in-progress
    merge/rebase" was narrowed: a *pre-existing* in-progress operation is another agent mid-sync —
    the transient state the retry exists for — so it retries, and only the exhaustion diagnostic
    calls it wedged. Auth/invalid-remote fetch classification was declined as
    locale/version-fragile; the diagnostic carries the last stderr instead. (b) The sync
    invariant is stated explicitly at the head of Half 1, and diverged-history behavior is
    written down for both sync-function branches. (c) The backoff sleep must be injectable for
    tests. (d) The guard's not-a-shell-parser contract boundary is stated. (e) Runtime
    enforcement (pre-commit hook / staging wrapper) declined — reasoning now recorded in Out of
    scope, with the risk-vs-guarantee split stated bluntly. (f) A requirements checklist heads
    the spec as the normative summary; the body remains the decision record.
16. **2026-08-11 reconcile against current `main` (build-time; eight changes merged since drafting).**
    Five findings, each a constraint on the build rather than a change of design:

    (a) **The new report token needs an explicit `case` arm, not just a new return value.**
    `board_pass_inline`'s result `case` ends in a `*)` catch-all that prints
    `board inline changed push-failed`, and `learnings_pass` carries the identical shape. A
    `blocked-wedged-tree` return that only travels out of `commit_and_push_generated` would be
    *silently relabelled by that catch-all* into the retryable push-failed token — the precise
    overloading Assumption 4 exists to forbid, reintroduced one layer up. Both call sites get an
    explicit arm ahead of the catch-all. `board_classify`'s own `*)` already maps an unrecognized
    `board …` line to `failed` (hence must-land non-zero, the halt Assumption 4 wants), so the
    classifier is correct by construction — but the token is named there explicitly anyway, because
    inheriting the right behaviour from a catch-all is not the same as documenting it.

    (b) **`worktree-scope:` (change 0208, ADR-0083) is the repo's settled scope vocabulary — do not
    mint a second one.** It is a declared frontmatter fact on `agents/docket-*.md` sources with
    exactly two values, `feature` and `metadata`. Half 3's Group 2 predicate is a *coverage*
    derivation over skill bodies, not a scope notion, and must not be described as one. It does gain
    a **cross-check floor** from 0208's fact: every `agents/*.md` source declaring
    `worktree-scope: metadata` whose `skills:` list names a docket operating skill must have that
    skill inside Group 2's derived set. Verified 2026-08-11 — that is `docket-adr`,
    `docket-auto-groom`, `docket-finalize-change`, `docket-implement-next`, `docket-status`: five of
    the seven, the remaining two (`docket-groom-next`, `docket-new-change`) being interactive and
    wrapper-less by construction. This reuses a declared fact instead of a parallel derivation and
    catches drift the `docket.sh preflight` predicate alone cannot see.

    (c) **The `docket.sh preflight` derivation re-verified 2026-08-11: still exactly the seven.**
    `grep -rl 'docket.sh preflight' skills/*/SKILL.md` yields those seven plus `docket-convention`
    (excluded as the rule's home) and nothing else. Assumption 12 stands unchanged.

    (d) **#0253 has NOT merged** — `tests/lib/prose_guard.sh` does not exist on `main`. Assumption
    14's second branch is therefore the live one: define `flatten(){ tr -s '[:space:]' ' '; }`
    locally, **byte-identical** to the three existing copies (`test_docket_review.sh`,
    `test_gate_execution_posture.sh`, `test_loop_continuation.sh`), with a comment naming #0253 as
    the consolidation target. A fourth local copy is the house idiom until #0253 lands, not a
    deviation from it.

    (e) **Change 0286's caller poll-loop was read and is not applicable — recorded so it is not
    re-litigated.** That shape governs observing a launched child through a closed vocabulary of
    printed `state=` lines under a minute-denominated budget. Half 1's retry is an in-function
    retry of a git operation with no child, no report line, and no observation budget; adopting the
    loop verbatim would be cargo-culting a shape whose every load-bearing part is absent. **One
    doctrine does transfer and is adopted**: the unknown arm is *terminal, never a retry*. So Half 1
    retries only the classes item 2 names explicitly, and a rebase failure matching none of them
    fails closed immediately rather than spending budget — which is also what item 2's own "spend
    retries only on classes that can self-heal" already implies, now stated rather than inferred.
    (Fetch failures keep their deliberate undiscriminated retry per item 2; that exception is
    argued there and is unaffected.)

