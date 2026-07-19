# Selection-order backlog digest — design

Change #0094. Groomed 2026-07-19 (interactive, human-attended).

## 1. Problem

`docket-implement-next` Step 1 selects by having the **model** walk `active/`: read every change
file's frontmatter, filter to build-ready, rank by priority → created → id. The cost grows with the
backlog, and the ranking is model-executed rather than deterministic.

**#0088 (merged 2026-07-18) multiplied that cost.** Its loop-continuation contract is driver-agnostic
prose, and `/loop docket-implement-next` — the recommended driver, confirmed working at CC 2.1.214 —
re-forks the skill per iteration with **fresh context**. Draining N changes now re-walks `active/` N
times. 0088 shipped no digest consumption of any kind; Step 1 is byte-unchanged in how it acquires
state.

#0069 already ships the digest projection (`render-board.sh --format digest`: `backlog <status>
<count>` rollups + one `change <id> <status> <readiness> <slug>` per active change), and #0093
shipped archive decay. What is missing is **selection order** — and the plumbing for any skill to
read the digest at all.

Two blockers found while scoping:

- **`render-board` is not reachable from a skill.** The `docket.sh` facade exposes `preflight env
  bootstrap docket-status board-refresh archive-change …` — no `render-board`.
- **The one path that emits the digest, `docket-status --board-only`, also commits and pushes
  `BOARD.md`.** A selection read must not be a write.

## 2. Design decisions

### 2.1 The digest is an accelerator, not the sole selection channel

Step 1 uses the digest to obtain an **ordered candidate set**, then confirms build-readiness by
reading that one change file before claiming. The change files stay authoritative.

This was chosen over making the digest authoritative. The failure mode is what decides it: a stale or
wrong digest costs a **re-pick**, never a bad build, and Step 2's claim CAS re-read already backstops
the whole path. Making the digest authoritative would trigger the full `sole-channel` re-proof burden
(totality across every failure path, ordering against every mutating pass, plus a designed fallback
for a failed digest) to buy a marginal additional saving over "one read instead of N."

### 2.2 Order is expressed as one line, not N

`render-board.sh --format digest` emits one additional line after the existing `change …` lines:

```
ready <id> <id> <id>
```

— build-ready ids in selection order (priority → created → id).

- The build-ready set is exactly what `digest_readiness()` already computes as the `build-ready`
  token. **No new readiness logic, no new file reads.**
- Sort keys `priority` and `created` are static frontmatter. **No wall-clock read**, so
  `render-board`'s determinism and the golden byte-compare hold.
- Existing `change` lines are **untouched** and stay id-ascending: the human-facing report is
  unchanged and no #0069 consumer contract moves.
- Cost is one line regardless of backlog size — consistent with the change's own token-reduction
  purpose. Per-entry `ready <rank> <id> <slug>` lines were rejected for duplicating ids and slugs
  already present; reordering the `change` lines in place was rejected for breaking an existing
  contract and forcing priority/created onto all N lines.

### 2.3 The `ready` line is ALWAYS emitted

Bare (no ids) when the queue is empty. **Absence therefore means no queue was produced** — an older
`render-board`, or a render failure — and never "nothing is ready."

This is the `sole-channel` totality lesson applied directly: a channel where "no line" is
indistinguishable from a legitimate outcome has merely moved the silence somewhere quieter. It also
preserves #0069's "stdout is never empty" invariant for free, even on an empty backlog.

### 2.4 The read-only entry point is a `docket-status` flag

`docket-status.sh --digest-only`: resolve config, emit the backlog rollups + `change` lines + `ready`
line, exit. No sweep, no health checks, no learnings pass, no board render, no commit, no push.

Chosen over exposing `render-board` in the `docket.sh` facade, which would also hand callers the
**markdown** projection; since `render-board.sh` emits to stdout, `docket.sh render-board > BOARD.md`
becomes an easy way to bypass `board-refresh.sh`'s surface gate — a known footgun in this repo. The
flag is write-free by construction and resolves config itself, so the calling skill passes no paths.
A dedicated new script was rejected as a contract file's worth of ceremony for one flag of behavior.

- Emits **no `board …` line**, so the report-line classifier cannot mistake it for a Board pass. The
  classifier already ignores non-`board` lines (`docket-status.sh:107`), so `ready` needs no
  classifier change.
- `--digest-only` with `--board-only` is an argument error (exit 2) — opposite postures.

### 2.5 Claim-age is out

The original scope carried the in-progress `claimed_at:`/`updated:` date as a claim-age signal. It is
**dropped**: it has no named consumer, and `board-checks.sh` (stale-in-progress, change 0089) and
`reclaim-claims.sh` already own the claim-lease signal, both reading the files directly and both
computing against a wall-clock `render-board` may not have. Shipping it would repeat the exact
"no adopter" mistake that deferred this change the first time. The digest's existing
`change <id> in-progress …` line already tells a selector to skip it.

## 3. Step 1 acquisition path

1. Run `docket.sh docket-status --digest-only` — **after** Step 0's `docket-status` subagent dispatch
   and metadata re-sync, never before. The digest is a snapshot; taken pre-sweep it would list
   already-merged changes (`sole-channel` lesson (a)).
2. Candidate order = the `ready` line, filtered by the id allowlist when one is given.
3. Read the top candidate's change file and confirm build-readiness before claiming. If the file
   disagrees with the digest, drop it, take the next, and **report the disagreement** — digest/file
   drift is a signal, not noise.
4. Bare `ready`, or nothing in scope, → the `drained` disposition.
5. **No `ready` line at all** → fall back to walking `active/` exactly as today, and say so in the run
   report. Per `skill-fallback-degrades-discipline`, that warning is a defect to investigate, not
   boilerplate.

Skip reasons for allowlisted-but-not-ready ids come from the `change` lines' readiness tokens, which
already carry `needs-brainstorm` / `waiting` / `auto-groom-blocked`.

## 4. What the prose must not lose

Per `consolidation-flattens-caller-variance`, this rewrites prose carrying real posture, and the
sentinel tests cannot see posture — no test fails when a contract silently inverts. Two things are
load-bearing and survive in meaning:

- **The convention's *Build-readiness & selection* definition stays the authority.** Step 1 gains an
  *acquisition path*; it does not delegate the *definition* to a script.
- **The id-allowlist paragraph's posture is untouched** — "a filter, never a dependency override,"
  skipped-with-reason, "never aborts the run." The allowlist filters the `ready` line; it never
  overrides what is on it.

The build must include a human diff read of Step 1 before/after against these two points.

## 5. Testing

**`render-board.sh`**
- Ordering: priority beats age; age beats id; exact ties fall to lowest id.
- Empty build-ready set emits the bare `ready` line.
- Re-run byte-identical (determinism); golden fixture updated.

**`docket-status.sh --digest-only`**
- Writes nothing: `BOARD.md` byte-unchanged, working tree clean, `HEAD` unmoved.
- Emits no `board …` line.
- `--digest-only --board-only` exits 2.
- stdout non-empty on an empty backlog.

**Skill**
- `tests/test_skill_size_budgets.sh` row for `skills/docket-implement-next/SKILL.md` raised in the
  same diff (the guard permits an in-diff raise). Current headroom is 12 words — 129/2833 against
  140/2845 — so a raise is effectively certain, not contingent.

## 6. Out of scope

- ADR index titles and learnings-index hooks in the digest — they require reading sources
  `render-board` must not own (ADR-0012) and would *add* per-digest tokens.
- Replacing `BOARD.md`; a committed or cached digest artifact; semantic relevance ranking.
- Rewiring any skill other than `docket-implement-next`. The full `docket-status` report picks up the
  `ready` line automatically (it runs the same projection), which is a free readability win, but no
  other skill's selection path changes here.

## 7. Side effects

The full `docket-status` report also runs `render-board --format digest`, so the human-facing status
report gains the drain order at no extra cost and with no additional call site.
