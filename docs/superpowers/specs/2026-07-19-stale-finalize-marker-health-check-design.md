# Stale `## Finalize blocked` marker — a git-only staleness health check — design

**Change:** #0098 · **Status:** proposed · **Date:** 2026-07-19 · **Related:** #0087 · **Auto-groomed** (default-biased self-brainstorm; assumptions below are the deferred human audit trail)

## 1. Problem

Change #0087 added the `## Finalize blocked` marker (a presence-encoded body section on an
`implemented` change) and its board cell `finalize blocked — needs you`, but
`scripts/board-checks.sh` gained no check for it. The marker's only clearing path is a run of
`docket-finalize-change` itself: a successful finalize, or an already-merged PR that the sweep
archives. It does **not** clear when a human fixes the underlying cause *out of band* — resolves the
conflict, approves, or otherwise unblocks the PR — without re-running finalize with the id named. In
that case the marker sits on the board indefinitely with no advisory, exactly the "legitimate
briefly, suspicious once it persists" shape that `merge-gate-stall` and `stale-in-progress` already
guard against.

## 2. Decision

Add a **git-only, warn-only, time-based** health check to `scripts/board-checks.sh` — new check-id
**`stale-finalize-blocked`** — that flags an `implemented` change carrying the `## Finalize blocked`
section whose marker has outlived a fixed staleness horizon, surfaced through the same
`docket-status` needs-you finding channel `merge-gate-stall` uses. It advises; it never mutates the
change file and never auto-clears the marker.

### 2.1 Where it lives and what it may touch

In `board-checks.sh`, alongside `merge-gate-stall` and `stale-in-progress`, inside the existing
per-file `FILES` walk. It uses only the tools already in that script's contract: `GIT` (mock seam),
the sourced `lib/docket-frontmatter.sh` helpers (`field`, `finalize_blocked`), and the `NOW` clock
seam. **No `gh`, no network** — that is the script's stated invariant ("Git-only (no gh, no
network) and warn-only", header lines 4–6), and this check honors it.

### 2.2 Trigger

For a file with `status == implemented` (read via `field`) AND `finalize_blocked "$f"` true:
compute the marker's age from the change file's **last-commit timestamp** in the metadata worktree —

```
ts="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct -- "<worktree-relative path to $f>" 2>/dev/null)"
```

— and emit when `NOW - ts > STALE_SECS`. `STALE_SECS` is a hardcoded named constant
`FINALIZE_BLOCKED_STALE_SECS=$(( 72 * 3600 ))` (72 h), mirroring how `stale-in-progress` hardcodes
its `3*86400` branch-idle horizon. The finding message names the age, e.g.:

```
emit stale-finalize-blocked "$id" "## Finalize blocked marker set ${age_h}h ago — resolve and re-run finalize <id>, or it will sit on the board"
```

### 2.3 Surfacing

`board-checks.sh` already prints all findings TAB-separated and `docket-status` relays them as
needs-you advisories; `stale-finalize-blocked` rides that path with no new plumbing. The new
check-id is added to the header's documented `check-id ∈ {…}` set. `--strict` (the future CI gate)
counts it like any other finding, unchanged.

## 3. Out of scope

- **Auto-clearing the marker.** A health check advises; mutation stays with `docket-finalize-change`.
- **Distinguishing "cause resolved, marker stale" from "cause still holds, genuinely blocked."** A
  git-only check structurally cannot make this distinction — it requires probing live PR state
  (`gh`, network), which `board-checks.sh` forbids by contract. See Assumption A3.
- **The clearing-rule wording** (change #0099) and **mirror readiness parity** (change #0097).
- Any config knob for the horizon (see A4).

## 4. Assumptions (deferred human audit trail)

Each is a decision an interactive brainstorm would have raised; the conservative default is chosen
and the rejected alternatives recorded.

**A1 — Time-based, not cause-re-probing. [chosen: time-based]**
The stub floats cause-re-probing (re-check whether the PR is still unmerged/unapproved/conflicting)
as "precise where a timer is only a heuristic." Rejected: it requires `gh` + network, which
`board-checks.sh` forbids as a core invariant (git-only, warn-only). The stub's own primary
instruction — "surface it through the same needs-you channel `merge-gate-stall` uses" — points at
`board-checks.sh`, and `merge-gate-stall`/`stale-in-progress` are the cited precedents, both
git-only time signals. A cause-re-probing check would belong in the *skill's* model-driven layer
(where the `blocked_by:` re-examination already lives — board-checks.sh header line 5), a materially
larger and differently-surfaced design the stub does not scope. Conservative default: the git-only
time check that matches the precedent.

**A2 — Marker age = change file's last-commit timestamp, not the in-body date. [chosen: git ct]**
The marker heading is deliberately bare/undated; the date lives inside the section body as
*model-authored* prose (finalize SKILL §"`## Finalize blocked`"). Parsing that date would key a
guard on an untrusted, format-unguaranteed value (learnings: `model-authored-values-are-untrusted-input`).
The git commit timestamp is tamper-proof and matches `stale-in-progress`'s own use of
`git log --format=%ct`. An `implemented` + finalize-blocked change's file is quiescent except for
re-marks (reconcile touches only in-progress builds), so last-commit-ct ≈ marker age; a re-mark that
replaces the section resets the clock, which is correct (a fresh mark restarts the plausible
lifetime). Rejected: pickaxe (`-S'## Finalize blocked'`) for first-appearance — more precise but a
same-commit replace nets zero string delta and can miss; last-commit-ct is simpler and adequate.

**A3 — Drop the resolved-vs-still-blocked distinction. [chosen: fire on any marker past the horizon]**
The stub asks whether the check should stay quiet when the cause still holds. A git-only check
cannot know whether the cause holds (that needs network probing — see A1), so it fires on *any*
marker older than the horizon. This is acceptable because the finding is **warn-only advisory**: a
marker still genuinely blocked past 72 h is itself worth a human glance (a PR blocked for days
deserves revisiting), so a "false" nag on a still-blocked marker is low-cost and arguably desirable.
Rejected: gating the fire on cause-still-holds — impossible under A1's constraint.

**A4 — Horizon is a hardcoded 72 h constant, no config knob. [chosen: hardcode + promote-later comment]**
Rejected: (a) a new `--finalize-blocked-ttl-hours` flag — speculative config surface (YAGNI);
promote to a knob only if tuning is ever felt. (b) reusing the existing `--lease-ttl-hours` flag —
it would silently broaden that flag's documented meaning ("the claim-lease TTL") to a second
concept. A hardcoded named constant mirrors `stale-in-progress`'s own `3*86400` branch-idle horizon,
adds zero config surface, keeps the lease flag's contract pure, and is trivially tunable in one line.
72 h chosen to match the lease-TTL default's sense of "a few days is normal, longer is suspicious."
A code comment notes it can be promoted to a knob if a human ever wants to tune it independently.

**A5 — Dependency state.** `depends_on: []`; no unmet dependency. `related: [87]` (#0087 is `done`,
archived 2026-07-19). No design-ahead gating.

## 5. Test plan (for the builder)

`tests/test_board_checks.sh` (the script's existing test) gains cases:
- An `implemented` change with `## Finalize blocked` and a last-commit older than the horizon (drive
  via the `NOW` seam and a fixture commit) ⇒ one `stale-finalize-blocked` finding.
- The same change with a recent last-commit (within the horizon) ⇒ no finding.
- An `implemented` change **without** the marker ⇒ no finding.
- A non-`implemented` change carrying a stray `## Finalize blocked` (should not occur, but guard) ⇒
  no finding (the `status == implemented` gate).
Use `GIT`/`NOW` mock seams exactly as the existing staleness cases do; no network.
