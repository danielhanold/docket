# stack-closeout.sh — the idempotent stack close-out

## Purpose

When a stack **root**'s code reaches the integration branch, every descendant that has been parked at
`stacked-merged` becomes reachable from it too — and by the governing invariant (`done` means the
change's code is reachable from the integration branch) those changes are now `done`. This script is
what moves them: nothing else in docket does. The merge sweep flips a child *into* `stacked-merged`;
only this pass takes it out.

For each descendant it runs the **shared terminal close-out sequence** documented in the docket
convention's `references/terminal-close-out.md` — archive → re-render `## Artifacts` as a separate
follow-on commit → terminal-publish → cleanup — and then regenerates the root's **Stack carried**
table on the root's archived record.

Invoked by `scripts/docket-status.sh`'s merge sweep after a root is swept to `done`, and runnable by
hand on any archived root.

## Usage

```
stack-closeout.sh \
  --changes-dir DIR \
  --root-id N \
  --date YYYY-MM-DD \
  --integration-branch B \
  --metadata-branch M \
  --adrs-dir REL \
  --terminal-publish true|false \
  [--remote R]
```

- `--changes-dir` — the changes directory (parent of `active/` and `archive/`) **in the metadata
  working tree**; the git tree it lives in is where every commit this script makes lands.
- `--root-id` — the stack root whose merge triggered the pass, padded or bare (`0298` and `298` are
  the same change; canonicalized with `10#` at the argument boundary).
- `--date` — the root's **UTC merge date**, the terminal date every descendant is archived with.
  Never `now()`: the pass is re-run, and a clock-derived date makes two runs disagree about the same
  change's archive filename.
- `--adrs-dir` — the **repo-relative** ADRs directory, `terminal-publish.sh`'s shape. The artifacts
  re-render is handed the absolute form, composed from the worktree root.
- `--terminal-publish` — the resolved `TERMINAL_PUBLISH` knob, passed straight through to
  `terminal-publish.sh --enabled`. Required, with no default: this script never decides the knob.
- `--remote` — the remote whose metadata branch the idempotency probe reads (default `origin`).

**Mock seams:** `GIT="${GIT:-git}"`, `SCRIPTS_DIR` (the close-out helper directory), and
`DOCKET_BASH_PATH` (the configured runtime every nested script launch names).

The whole flag set is validated before any work runs.

## Behavior

**1. Snapshot the graph.** `stack_descendants` (in `scripts/lib/docket-stack.sh`) is called **once,
at call time**, before any promotion — never from an earlier scan. That is what lets a child swept to
`stacked-merged` moments earlier *in the same sweep pass* be promoted in that same pass. The walk is
breadth-first, so parents are promoted before their children.

**2. Promote each descendant**, in emitted order, printing exactly one verdict per descendant:

| Report line | Meaning |
|---|---|
| `promoted <id>` | The full close-out sequence ran. |
| `promote-skipped <id> already-archived` | The idempotency probe fired — see below. |
| `promote-skipped <id> not-stacked-merged` | Not this pass's business (a still-`implemented` child). |
| `promote-skipped <id> change-file-missing` | The graph named an id with no change file. |
| `promote-failed <id> archive` | `archive-change.sh` exited non-zero. |
| `promote-failed <id> archived-file-not-found` | The archive succeeded but its file could not be located. |
| `promote-failed <id> render-change-links` | The `## Artifacts` re-render failed; **the publish is skipped**. |
| `promote-failed <id> terminal-publish` | The publish exited non-zero. |

A per-descendant failure **never abandons its siblings**: each promotion is independently
re-runnable, and a stalled sibling helps nobody. Cosmetic failures — a failed follow-on commit or
push of the re-rendered block, a failed cleanup — are logged to stderr and do not change the verdict,
matching the sweep's carve-out in the shared sequence.

**3. Write the root's Stack carried table** into the root's **archived** record: a marker-bounded
block holding one row per descendant, `| #<padded id> | <title> | <pr> |`. Reported as
`stack-carried <root> <count>`, or `stack-carried-failed <root> <reason>` with reason
`root-not-archived`, `markers-unbalanced`, `render-failed`, `commit-failed`, or `push-failed`. A root
with **no** descendants has nothing to say, so its record is left untouched and no line is printed.

## Exit codes

| Code | Meaning | What it obliges the caller to do |
|------|---------|----------------------------------|
| 0 | Every descendant reached a verdict — **including failed ones**. | Relay the report lines. A `promote-failed` line is a finding, not an abort: the sweep's posture is log-and-continue, and the next pass resumes what can be resumed. |
| 1 | The **pass** could not run: no changes directory, not a git worktree, or an unknown `--root-id`. | Fix the invocation's target; nothing was attempted. |
| 2 | Usage error: a missing or unknown flag, a non-numeric `--root-id`, or a `--terminal-publish` that is neither `true` nor `false`. | Fix the invocation. |

## Invariants

- **The idempotency key is the state the close-out PROMISED — the descendant's archived file on the
  metadata branch — never a local proxy.** Above all, never "the working tree is clean": a run that
  promoted half a stack and died leaves precisely a dirty tree behind, so that proxy is false exactly
  when resumption matters and true exactly when it does not. The probe reads the branch after a
  read-only `fetch`, so a promotion another agent or an earlier crashed run already landed is seen
  even when this worktree has not pulled it.
- **A `fetch`, never a `pull`.** The probe needs fresh remote state; a pull would mutate the working
  tree, and the half-finished state this script exists to resume is precisely a tree that cannot take
  one.
- **An archived record with an outstanding publish is RESUMABLE.** Archiving is the sequence's first
  step, so a run that died at the publish leaves the change archived — indistinguishable, on the
  archived file alone, from one that completed. The durable `## Publish deferred` marker (ADR-0051)
  is the difference: present, the sequence is re-run from the top, which is safe because every step
  of it is idempotent (`archive-change.sh`'s own reuse-existing probe makes step 1 a no-op); absent,
  the descendant is closed out and the pass is silent about it. Under suppression nothing is ever
  marked, so a re-run correctly skips and the residual is a stale `## Artifacts` block — the same
  residual `scripts/docket-status.md` documents for the sweep.
- **Ordering inside the sequence is load-bearing.** `terminal-publish.sh` copies the change file
  *from the metadata branch*, so the re-render's follow-on commit must land first or the publish
  copies the stale block onto the exact surface the re-render targets. A failed re-render therefore
  skips the publish.
- **An abandoned but EXPECTED publish is marked durably.** Once archived, a change leaves `active/`
  and no sweep resumes it — the gap would be permanent and invisible. Both abandoning legs write the
  `## Publish deferred` marker, gated so it is **never** written under suppression (`--terminal-publish
  false`, or a metadata branch equal to the integration branch), where a skipped publish is success
  rather than a deferral. The mark is best-effort toward the report stream and transactional toward
  the shared worktree: a failed `add`/`commit` restores the path to `HEAD`, a failed push retains the
  commit.
- **Marker order and balance are validated before the Stack carried block is rewritten.** Dangling,
  out-of-order or nested markers make the script **refuse and leave the record byte-identical**:
  presence alone is not enough, because an unbounded range consumes to EOF and eats the record.
- **The block is rendered to a temp file beside its destination and moved with `mv -f`** — never a
  redirect into the file being generated — gated on both the renderer's exit status and non-empty
  output. The temp name is dot-prefixed so a crashed run's residue is invisible to every `*.md` glob.
- **Every commit stages by explicit path.** The metadata working tree is shared; an unscoped `add`
  sweeps a concurrent agent's staged work in under this run's message.
- **The descendant scan prefilters on the KEY's shape, never a value shape.** `^stacked_on:` is a
  strict superset of what `fm_field` can answer non-empty for, so it narrows the work and never the
  decision. It is not a micro-optimization: one `fm_field` per change file costs ~1s over a
  300-change tree against ~0.07s prefiltered, and this runs per sweep pass.
- **The scan terminates on a cyclic graph** and emits each descendant once: the visited set is seeded
  with the root. A cycle is a data defect the health checks name; hanging the sweep is not an
  acceptable way to report it.
- **`stacked_on:` is read with the anchored `fm_field`; `status:`, `id:`, `slug:` and `title:` with
  `field`.** The first is an optional key, and an unanchored read of an absent key falls through to
  body prose — ordinary content in a repo whose subject matter *is* these field names. See the
  selection rule in `scripts/lib/docket-frontmatter.sh`, censused by
  `tests/test_frontmatter_read_shapes.sh`.
