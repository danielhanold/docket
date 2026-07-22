# Typed changes, selective auto-capture, and backlog filters — results

Change: #127 · Branch: feat/typed-changes-selective-auto-capture · Plan: docs/superpowers/plans/2026-07-22-typed-changes-selective-auto-capture.md · ADRs: 56, 57

## ⚠️ Required at the merge gate — this repo's own config breaks on merge

`auto_capture` is now a map, and the scalar form has **no compatibility shim**. This machine's
`/Users/homer/dev/docket/.docket.local.yml` still carries `auto_capture: true`, so **every docket
skill will hard-fail on the next run after this merges** until that file is edited. The committed
`.docket.yml` is clean — only the gitignored machine-local layer is affected, and it is outside the
repo, so this PR cannot fix it.

The resolver already prints the exact remedy, naming the offending file:

```
auto_capture:
  enabled: true
  types: all
```

The second merge-gate action is the one-time backlog categorization, which is deliberately **not**
in this PR: change files live on the `docket` metadata branch (the feature branch never writes
docket metadata), and the mapping needs a human's single-decision approval.

```bash
docket.sh docket-status --type untyped      # the exact inventory
# an agent proposes a complete id -> type mapping; you approve it as one decision
docket.sh backfill-change-types --changes-dir .docket/docs/changes --map 7=feat,8=feat,...
```

Note #0127 itself already carries `type: feat` (written at authoring time), so it is **not** in the
untyped migration set — the helper refuses to overwrite an existing non-empty type.

## Findings

- **The spec's `auto_capture.enabled` was structurally blocked by an existing guard.**
  `tests/test_docket_example_yml.sh` forbade duplicate leaf key names, and `learnings.enabled`
  already owned `enabled` — the guard's own comment had predicted this exact collision. Rather than
  rename the leaf or weaken the guard, keys are now qualified by their full ancestor path
  (`runners.codex.sandbox` keeps both parents) and the duplicate floor is re-derived from the
  resolver's **read shape**: block-scoped leaves are read inside their own `yaml_block_body` and are
  genuinely unambiguous, while flat-read keys (every top-level key plus the `finalize.*` leaves)
  must stay globally unique or `yaml_get`'s `head -n1` mis-resolves them. Recorded as **ADR-0056**.
- **A real read bug, caught by the backfill's own anchor fixture.** `field()` returns the first
  match *anywhere* in the file. That is safe only for keys always present in frontmatter; for an
  **absent** key it falls through into the body, so an untyped change whose body opened a line with
  `type:` rendered that prose as its Type and made the backfill refuse the record. Fixed by adding
  `fm_field` (first frontmatter block only) and routing every `type:` read through it. Recorded as
  **ADR-0057**; the residual audit of the other `field()` call sites is auto-captured as **#134**.
- **A guard that mutation-testing proved vacuous.** The backfill's refusal cases originally asserted
  only "non-zero exit + nothing written". Removing the explicit conflicting-overwrite guard left the
  whole block green, because the downstream post-write verification caught it too. Each refusal is
  now pinned to its **own diagnostic**, so the guard — not merely some backstop — is what the suite
  holds in place.
- **A review finding: an invalid filter was silently swallowed.** `backlog_pass` is best-effort by
  design, so leaving `--type`/`--priority` validation to `render-board` meant `--board-only --type
  Bogus` printed a board and quietly omitted the backlog the caller asked to filter. Keying on
  `render-board`'s exit 2 does not work either — it also spends that code on "changes dir not
  found", which the full pass legitimately tolerates. Validation moved up front in
  `docket-status.sh`, using the **shared predicates it already sources**: one rule, two call sites.
- **Mutation coverage.** Every guard added by this change was mutation-tested in both directions
  where applicable: the taxonomy set-equality pin (remove an arm / add a phantom arm / drift the
  reserved set), the qualified extraction and the flat-read floor, the mint's `type:` write and
  reserved-value gate, the digest/writer boundary (letting the markdown writer consult the filter
  reddens the byte-identity assert), the `untyped` fallback, and the backfill's all-or-nothing
  staging, write anchoring, and conflict refusal.
- **The convention's word budget was raised deliberately**, 5689 → 5850, in the same diff — the
  mechanism that guard documents (precedent: change 0102). The section was compressed by ~150 words
  first; the residual is normative text with no other home. The line budget was **not** raised.

## Follow-ups

- **#134** — Audit `field()` call sites for frontmatter-anchored reads (auto-captured from this
  change; `blocked_by:`, `pr:`, `spec:`, `plan:`, `results:`, `branch:`, `issue:` are all optional
  and several appear in docket's own body prose).
- Not minted, reported only: the GitHub board mirror carries no type. The spec explicitly defers
  this as optional ("a later change **may** add a `docket:type/<type>` label"), so it is a design
  option rather than committed work.

## Plan deviations

- **Task 8's live migration was reduced to a runbook.** The plan already anticipated this; stated
  here as the deviation it is. Categorizing this repository's active backlog writes to the `docket`
  metadata branch and needs human approval of the mapping, so it is a merge-gate action, not a
  feature-branch commit.
- **Task 2 grew beyond "requalify the arms".** The extraction had to track a full indent stack
  (`runners.codex.sandbox` is three levels deep), the three resolver/consumer greps had to reduce a
  qualified key back to its leaf, and `AUTO_CAPTURE_TYPES` had to be restructured to be assigned
  from its own leaf read so the correspondence check could tie export to key.
- **`fm_field` was not in the plan.** It was added in Task 7 in response to a real defect the plan's
  own fixture design surfaced.
- **No subagents were used.** The configured build and review skills normally delegate per task; no
  nested dispatch tool was available in this execution context, so implementation, TDD, mutation
  testing, and the whole-branch review ran inline. The `docket-status` Step-0 dispatch was likewise
  replaced by an inline run of the same deterministic orchestrator (its contract is git state).

## Test results

Full suite: **59/59 passing** (`for t in tests/*.sh; do GIT_EDITOR=true bash "$t"; done`).
No pre-existing baseline failures were observed on this branch. `tests/test_docket_status.sh`
requires `GIT_EDITOR=true` in a non-interactive run — its rebase-conflict fixture otherwise launches
an editor.

One suite failed mid-build and is worth recording because the guard was doing its job:
`test_comment_anchor_style.sh` caught a line-number cross-reference (`docket-config.sh:201`) written
into a new test comment, which ADR-0054 forbids; it was re-anchored on a symbol name.
