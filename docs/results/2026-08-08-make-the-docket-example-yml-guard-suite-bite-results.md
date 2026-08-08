<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0246 — Make the docket-example-yml guard suite bite](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0246-make-the-docket-example-yml-guard-suite-bite.md)**
<!-- docket:backlink:end -->

# Make the docket-example-yml guard suite bite — results
Change: #0246 · Branch: feat/make-the-docket-example-yml-guard-suite-bite · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-08-make-the-docket-example-yml-guard-suite-bite.md · ADRs: none

## Verify (human)

- [ ] Nothing manual. Every claim in this change is a guard, and every guard was mutation-proven in
      both directions inside the build; the suite is the verification. Read the fix commits in the
      diff — that is the merge gate this change asks for.

## Findings

**The truncation was not grep.** The stub framed #0178 as a grep-portability defect. It is a bash
version defect: under `/bin/bash` 3.2 the file's `$(...)`-containing-a-heredoc construct cannot be
parsed, so everything from the heredoc to EOF died and 290 of 393 asserts silently never ran
(exit 2, cryptic parse error). `scripts/run-tests.sh` was already protected by its Bash 4.3+
re-exec, so the exposed path was direct invocation — the file's own documented run line. The gate
now fails loudly instead.

**A guard suite about stale literals contained stale literals.** Three separate instances surfaced,
and all three were only visible because something else forced a look:

- The mirror slice terminator was prefix-weak — `claude-opus-5` is a prefix of cursor's
  `claude-opus-5-high` — so deleting claude's own `build-max` row closed the range on *cursor's*
  block while the terminator guard stayed green, because the over-wide slice's last line is still a
  `build-max:` line.
- The round-trip slice's end anchor was a hand-written cursor `finalize-change` literal sitting
  above cursor's own build rows and above the entire opencode block. Change 0192 shipped opencode
  and nothing noticed that sixteen rows stopped reaching the real resolver.
- `tests/test_skip_allowlist_invisibility.sh` pinned its probe of `test_grep_portability.sh`'s
  whole-index walk to hard-coded **line 102**. This change added 27 lines above that walk, moving it
  to 129, and the full-suite gate went red — the only red of the build. Its diagnostic even said
  "if it moved, re-find it and re-point the probe", which is what the repair did, onto a derived
  slice anchor (`-t ALL_FILES`) rather than a new line number.

The third one is the useful one: the branch's own thesis reddened a guard in a file the change was
not otherwise touching, and the correct fix was to apply the thesis there too.

**The word-boundary ban covers a spelling, not the defect.** Review finding 1. In bash, `"\\b"` and
`"\b"` deliver the identical byte pair to grep, and the scanner is a pure byte-pattern with no quote
awareness — so the double-quoted single-backslash form is unguarded. Rather than expand scope
mid-review, the header was corrected to state the limitation honestly, the limitation was asserted
rather than merely commented, and the residual work was filed (see Follow-ups). The count in that
header is now **computed**, not written: the review estimated ~42 surviving sites, the measured
figure was 47 before this change's own additions and 48 after — which is itself the argument for
computing it.

**The `elsewhere:` anchor was unfalsifiable for one of its six entries.** Review finding 2. The
shape predicate carried no left boundary, so `agents` was satisfied by `sync-agents.sh` containing
its own hyphenated *name* (`"sync-agents: $*"`). Strip every `agents:` reader out of the consumer
and the guard stayed green. Fixed with a left boundary class of `[^[:alnum:]_-]` — note the hyphen,
which the reviewer's own suggested class would have admitted.

**The reverse mirror loop had the same blind spot it was written to close.** Review finding 3. Its
population came from the `build-max`-terminated slice, and `build-max` is the *last* row of every
block — so an orphan row appended after it, the most natural place to append one, was invisible to
both the orphan check and the arity check. Replaced with a shape-detected harness partition of the
whole commented block.

**No ADR.** Every decision here is scoped to the internals of two test files — the exemption
mechanism, the derived terminators, the shape set — and none of them establishes a boundary another
part of the system must respect.

## Follow-ups

- **#0262** (minted this run) — ban the single-backslash word-boundary form too, not just its
  escaped spelling. 48 computed sites across the tracked tree must be converted or blessed; two
  files already carry the defect in the surviving spelling and pass today's class clean
  (`tests/test_docket_metadata_branch.sh:112`, `tests/test_cursor_dispatch_rule.sh:38` and `:93`).
- **#0150** (pre-existing, still `proposed`) — the suite-wide toolchain pin/report decision. Its
  groomed spec touches `tests/test_grep_portability.sh`'s prologue, which this change also extended.
  Disjoint regions; whichever builds second absorbs a reconcile-time collision. No dependency was
  added.
- **Plan defect worth knowing about if this plan is ever re-run**: Task 4 Step 2's mutation command
  inserts its phantom row *after* the `build-max:` line — i.e. outside the slice — so as written it
  produces no red. The worker ran the equivalent in-slice mutation. Task 5 Step 8's third check is
  also self-contradictory: the replacement comment quotes the stale wording it replaces, so the
  grep that is supposed to find nothing necessarily finds that quotation.
- **Deliberate deviation from plan-supplied code**: `code_shaped_mention` was written capture-first
  (`body="$(grep -vE ... )" || true` then `grep -qE ... <<<"$body"`) instead of the plan's
  `grep -v ... | grep -q ...`. The file runs under `set -uo pipefail` and AGENTS.md bans
  `producer | early-exiting-consumer` there — the producer takes SIGPIPE and the pipeline's 141
  would intermittently invert a real match into "not code-shaped". Behavior is identical; the
  flakiness is not.
