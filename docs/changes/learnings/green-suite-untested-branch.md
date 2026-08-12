---
slug: green-suite-untested-branch
hook: "Green tests are not proof the hard branch was exercised — a mock that omits the tool routes every test through the degrade path."
topics: [testing, fixtures, mocks]
changes: [16, 22, 25, 26, 35, 58, 62, 69, 91, 93, 117, 126, 186, 207, 219, 298]
created: 2026-07-11
updated: 2026-08-12
promotion_state: retained
promoted_to:
---

## Apply
A tool-output mock must mirror the real tool's response shape, nesting and all — and
when the code under test has a best-effort/degrade branch, a mock that OMITS the tool silently routes
every test through that branch, so at least one fixture must carry the REAL tool (and its `lib/`).
Fixtures need real-SHAPED field values and PLURALITY (≥2 of every kind rendered as a list); smoke
against real data inside a real worktree before merge; to cover a conflict/CAS path the competing
writer must DIVERGE the same contended path (mutation-confirmed); give a tool writing to BOTH a user
and a project location SEPARATE dirs; keep fixture stderr 0-byte. A fixture pinning a
*read-modify-write* guarantee must carry a value that DIFFERS from the default the code would
otherwise write — when the stub value equals the fallback, the assertion passes identically against
the blind-set implementation the guarantee exists to forbid. When one change adds TWO independent
filters over the same data, a fixture that keeps them agreeing tests neither's independence — build
the crossed case where one filter's decision contradicts the other's. Green tests ≠ the hard branch
was exercised.

## War story
- 2026-07-11/13 (#58 PR #65; #69 PR #77; #16 PR #30; #22 PR #35; #25 PR #36; #26 PR #38; #35 PR #44
  — merged, one green-suite-untested-branch family) — Seven green suites that never exercised the
  branch they existed to cover. **Mock fidelity:** (a) a `gh api graphql` jq path read one level too
  shallow (`.data.pN.mergedAt` vs `.data.pN.pullRequest.mergedAt`), and the bug hid because the mock
  returned a *flattened* JSON shape `gh` never emits; (b) worse, every full-pass `docket-status`
  fixture pointed `SCRIPTS_DIR` at a mock dir containing **no `render-board.sh` at all** — and because
  the new digest call is best-effort, the missing tool degraded silently on every full-pass test, so
  the change's two headline claims had ZERO real coverage. **Fixture realism:** (c) a golden fixture
  used `pr: 142` where real changes store a full URL and had a single `done` change, so neither the
  URL-format path nor the multi-id concatenation bug was hit; (d) a generator test set
  `DOCKET_HARNESS_ROOT` to the repo root, so the user-level and project-level passes wrote ONE dir and
  an "unlisted skill gets no project file" assertion passed vacuously; (e) a CAS conflict-retry branch
  shipped uncovered because the competing-writer test touched an *unrelated* file, hitting only the
  clean if-branch; (f) a renderer branching on git-remote resolution was smoked in a `/tmp` fixture
  with no origin, so only the degraded bare-path fallback ran; (g) fixtures cloning a fresh
  `init --bare` origin emit `warning: You appear to have cloned an empty repository`, leaving noisy
  stderr.
- 2026-07-17 (#62, PR #94) — `setup-auto-approve.sh` promised it would never *blind-set* the repo's
  `default_workflow_permissions`: it reads the current value and preserves it. The test that existed
  to pin that guarantee stubbed the API's current value as **the same value the fallback would have
  written**, so a blind-set implementation passed the assertion byte-for-byte — the guarantee was
  decoration and its test could not tell the two implementations apart. The fixture now stubs a
  non-default `write` and asserts it survives the round-trip. The discriminating input is the whole
  test: when the fixture value and the default coincide, there is no experiment.
- 2026-07-18 (#93, PR #96) — One change gave `render-board.sh` two independent output filters over the
  same archived-`done` set: a count-based recency window that COLLAPSES old dones out of the archive
  table, and a mermaid pruning rule that KEEPS a done node styled `:::done` when an active change's
  `depends_on` still references it. Each filter had its own assertions and the suite was green, but
  the large-archive fixture pointed its active dependency at a done id that was *inside* the verbatim
  window — so the two filters never disagreed, and the state that actually proves they are independent
  (a done collapsed out of the table yet still styled in the graph) was verified only by reading the
  code. Review caught it as Minor; the fixture now aims the dependency at a collapsed month. Two
  filters that always agree in the fixture are, as far as the suite knows, one filter.
- 2026-07-19 (#91, PR #104) — **A green suite hid two shipping-blockers because every fixture used
  tidy inputs.** `mint-stub.sh` mangled any title containing `&` and let a multi-line title inject
  frontmatter that bypassed the grooming gate ([[model-authored-values-are-untrusted-input]]) — both
  invisible because no fixture ever passed a title with punctuation. A third gap needed a *combined*
  state no fixture built: an empty `active/` together with a forced retry. The lesson is about
  fixture *input* realism rather than mock shape — when the value under test is free text a model
  writes, the fixture must carry the punctuation, the multi-line case, and the shell metacharacters
  that real prose carries. Tidy fixtures test the happy path you imagined, not the input you ship.
- 2026-07-28 (#117, PR #129 — merged) — The fixture the comment described was never written. A test
  claimed an `adr_pub` fixture published to both branches; none existed, so `i_blob` was empty on
  every iteration and the present-on-integration branch never executed — deleting that branch left
  the suite green. Caught only by mutation: after the fixture was added, the same deletion reddened
  three asserts. The comment describing the fixture was authored before the fixture was, and nothing
  in a green run can tell those two states apart. Mutation-test the branch, not the suite's color.
- 2026-07-28 (#126, PR #132 — merged) — **State leaked from the previous fixture makes the next
  block's assert vacuous.** Sequential resolver fixtures in one file share exported variables; when
  block P's resolver was sabotaged into aborting, its `eval` received an empty string and the assert
  silently read block **O's** stale `AUTO_GROOM=false` — printing `ok` for a resolver that never
  ran. Proven, not asserted: same assert, three states — clean `ok`, sabotage-only `ok` (vacuous),
  sabotage + an `AUTO_GROOM=__poison__` prelude `NOT OK` (caught). Poison every variable an assert
  will read, between evals. Two cautions the demonstration itself taught: sibling asserts in the
  same block redden for *unrelated correct* reasons under the sabotage, so read the target assert's
  own status rather than the suite's ok/notok deltas; and the site the stub originally named was
  the wrong one to demonstrate at — where the previous fixture happens to leave a *differing*
  value, the hazard is real but latent and the assert reddens honestly.
- 2026-08-01 (#186, PR #148 — merged) — **The environment, not the fixture, was the missing branch.**
  A rollback test forced a mid-loop install failure with `chflags uchg` and asserted the undo path;
  it passed for months. It passed only because nothing that ran it had a tty — BSD `mv` prompts on an
  unwritable destination, so on a terminal the test hangs and in a pipe it takes a *different* failure
  path than the one a real user hits (a silent zero-exit decline). Agent shells and the finalize gate
  are both tty-less, so no runner could tell the two apart. When a fixture's premise is "this
  operation will fail", check whether the *runner's* environment decides HOW it fails; cover the case
  under a pty (`script(1)`) rather than trusting the color of a non-interactive run.
- 2026-08-05 (#207, PR #159 — merged with the gap open, carried to #220) — **Every fixture for a
  rule lived in ONE config layer, so the sibling layer's code path was entirely
  mutation-survivable.** The change added a pre-flight gate with two legs — a project-level leg and
  a user-level leg over `$USER_TARGETS`. Every `runner:` fixture in every suite writes `.docket.yml`,
  the project-level file, because that is the convenient one to author. Result: the user-level block
  shipped green with no test that could redden if it were deleted — and that is the leg protecting
  `~/.claude/agents`, the **widest blast radius of the original bug**. The same untested block also
  held the build's subtlest fix: the plan predicted `$USER_TARGETS` would be unset under `set -u` on
  the `--check` path and prescribed calling `compute_user_targets`, but that helper itself reads
  `$USER_HARNESSES_SET`, which is *also* unset there — so the plan's remedy alone would still have
  died. The correct chain (`[ -n "${USER_HARNESSES_SET:-}" ] || resolve_global_agent_harnesses`, then
  `compute_user_targets`) is verified by nothing. Note the selection pressure: fixtures cluster in
  whichever layer is cheapest to write, so "which layer do all my fixtures use?" is a question worth
  asking directly — the answer names the branch with no coverage. Where a rule spans a user-level and
  a project-level location, at least one fixture must exercise **each**, and the way to prove it is
  deletion, not reading ([[config-layer-write-and-read-hazards]] on giving the two locations separate
  dirs).
- 2026-08-07 (#219, PR #171) — **A witness written against a stream the code never writes to.** A
  plan-supplied fixture pinned "no candidate ⇒ never calls `gh`" by asserting on stdout — but the
  only `gh` call on that path captures its output into a variable and prints nothing, so the assert
  was *permanently* vacuous: it passed identically whether the call happened or not. Rewritten as a
  side-effect witness (the stub records its invocation) **plus a companion assert proving the
  witness is not itself vacuous** — the second half matters, because a witness that never fires is
  the same green as a code path that never runs ([[plan-supplied-test-code-is-unverified]]). A
  second plan fixture failed to discriminate for the same reason and was likewise replaced. In the
  same branch, two of the change's blockers were invisible to their fixtures for the structurally
  identical reason (see [[frontmatter-anchored-read]] and
  [[duplicated-gate-copies-the-whole-predicate]]): a fixture population authored from tidy,
  well-formed inputs makes the correct and the incorrect implementation indistinguishable. Three in
  one change is the tell that "would this fixture redden if the code were wrong?" belongs in the
  build loop, not review.
- 2026-08-12 (#298, PR #203 — merged) — **Adding a flag to a mocked call can turn a whole fixture
  vacuously green rather than red.** The real fix was a cross-cutting one: `stack_effective_base`
  issued its `git show-ref --verify` with no `-C`, so it answered "is the parent's branch pushed?"
  from the *caller's cwd* instead of the repo under `--changes-dir` — and because that lookup is the
  **positive** conjunct of the resolver's rule 1, a failed lookup is indistinguishable from "not
  pushed", so the symptom is silent: every stacked change renders `stack base not built` and drops
  out of the ready queue. The part worth carrying forward is what the fix did to the fixtures. Two
  git stubs matched on `$1 = show-ref`; once `-C` arrived, `$1` became `-C` and both fell through to
  their catch-all `exit 0` — reporting **every** branch as pushed. The stubs would have kept the
  suite green while testing the opposite of the intended state. Rule: when a fix adds a flag to a
  call some stub intercepts, re-check every stub's dispatch on `$1` — a catch-all `exit 0` converts
  a missed match into a permissive answer, not a loud failure.
