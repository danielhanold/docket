---
slug: enumerated-floor
hook: "Every hand-written enumeration is a floor, not the set — derive the sites from a whole-repo grep, then treat that grep as a floor too."
topics: [process, inventory, review]
changes: [14, 32, 42, 52, 54, 56, 64, 67, 71, 74, 84]
created: 2026-06-12
updated: 2026-07-16
promotion_state: candidate
promoted_to:
---

## Apply
Never hand-list the sites of a literal, a count, or an operation you are gating — derive them
from a grep of the WHOLE repo (case-INSENSITIVE, every file type, never `--include="*.sh"`; the
variable is uppercase in prose and lowercase in YAML), then sort them into prose vs executable,
because only the executable ones can violate a gate. But treat even that grep as a floor: when a
change NARROWS an exception, the sites that matter assert the OLD exception was the only one, and
that shape carries none of your keywords — only a semantic whole-branch read finds it, so budget for
one. Let reconcile pin the inventory before the build, and never let a reconcile grep OVERRIDE a spec
that named a site — re-read the file. Guard the list's completeness with a structural sentinel —
whose corpus is itself an enumeration, so derive that too. Name every dimension you need audited as
an explicit goal. And run the WHOLE suite at the merge/build gate, never only the tests the spec
enumerated.

## War story
- 2026-06-12 → 2026-07-16 (#14 PR #10; #32 PR #43; #42 PR #52; #56 PR #68; #64 PR #75; #52 PR #61;
  #54 PR #66; #71 PR #81; #74 PR #82; #84 PR #90; #67 PR #91 — merged, one enumerated-floor family) — **Every
  hand-written enumeration is a floor, not the set** — of sites, of audit dimensions, of tests — and
  the miss always lands in the surface that mattered most.
  (a) **Sites.** A hand-listed "everywhere X appears" undercounted again and again: 4 test assertions
  still hardcoding old model aliases plus two un-named id-read sites; a 9th generated wrapper leaving
  the convention's "eight wrappers" line, one `test_finalize_gate` assertion and seven
  `test_sync_agents` assertions stale at once; an earlier skill leaving "six skills" in README and the
  convention; and 0064's gating knob listing every *prose* site while missing `scripts/docket-status.sh`,
  the one **executable** invocation, in the headless merge sweep — precisely the agent the gate exists
  to serve. The same undercount hits a sentinel's own CORPUS: #71's structural sentinel scanned a file
  set that omitted `skills/docket-convention/github-board-mirror.md` — the one reference doc *about*
  board surfaces (widened in review to `skills/*/*.md` + `skills/*/references/*.md`). #67 hit the same
  shape **three times in one plan**, which is the useful part — the miss is not rare, it is the
  default: the plan listed **3** learnings-reader sites and a whole-repo grep found **5**
  (`docket-auto-groom` and `docket-brainstorm` would have been left silently reading a ledger that had
  become a pointer stub — a dead read, shipped green); it listed **12** finding families for the
  migration where the real derivation found **14**; and it listed **no** co-located script contracts
  when **three** (`docket-config.md`, `docket.md`, `docket-status.md`) were stale and needed fixing.
  A plan's enumeration is a floor even when the plan is fresh, specific, and written by someone who
  just read the code.
  (b) **The grep that derives the inventory is itself a floor.** #84 inverted 0064's shape: its four
  EXECUTABLE default sites were derived by whole-repo grep and held exactly, while ~10 PROSE sites
  went unfixed — because the inventory grep used `--include="*.sh"` for the `TERMINAL_PUBLISH:-`
  pattern and a case-sensitive lowercase `terminal_publish` for `.md` files, so an uppercase
  `${TERMINAL_PUBLISH:-true}` quoted inside `scripts/docket-status.md` fell through both filters.
  Reconcile then "cleared" that file on the strength of the bad grep, overriding a spec that had
  correctly listed it. Worse, the dominant defect shape was keyword-INVISIBLE: prose naming `main`-mode
  as the **sole** no-op exception to publishing — exhaustive and true *before* the change, exactly half
  the gate after it — carries neither "default" nor "true", so every keyword search was structurally
  blind to it. Nine live sites had it, including one *instruction* (`docket-convention/SKILL.md`) that
  would have made a parent agent read a healthy opted-out run as a failure. Found only by the final
  whole-branch semantic read.
  (c) **Audit dimensions.** A goal-scoped rewrite only examines the dimensions in its goal set; anything
  outside it passes unaudited even when every claim it makes is TRUE — a README rewrite audited hard
  against its three named goals shipped clean yet stayed Claude-centric for a tool that first-classes
  three harnesses; the owner caught it at the merge gate.
  (d) **Tests.** A behavior-neutral slim passed its own goal-scoped review, yet finalize's FULL suite
  caught a regression its 7 enumerated sentinels missed. #74 re-hit it from the other direction: its
  edits reddened a *pre-existing* sentinel in a file its plan never enumerated, in a change whose
  subject was a different file entirely. The blast radius of retiring a string is every guard keyed on
  that string, repo-wide.
