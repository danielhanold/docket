<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0331 — docket-implement-next's re-mint path never names docket gate launch, so a resumed run cannot produce the run directory evidence record requires](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-21-0331-docket-implement-next-s-re-mint-path-never-names-docket-gate.md)**
<!-- docket:backlink:end -->

# Docket implement-next evidence re-mint — complete the gate-to-evidence chain

**Change:** 0331 · **Date:** 2026-08-20 · **Type:** fix · **Priority:** high

## Problem

`docket-implement-next` Step 6 correctly refuses to review a branch whose build-evidence record is
missing, malformed, or stale. Its recovery instruction is incomplete, however: it tells the
controller to re-run the full suite and later requires `docket evidence record --run
<absolute-run-dir>`, but never names the operation that creates that run directory.

A direct invocation of the suite can pass while producing no supervised run slot. The controller
then has no valid `--run` value, evidence recording correctly refuses, and a resumed run halts after
paying for a full green suite. This happened four times while resuming change 0316. The ordinary
build path did not expose the gap because `docket-build` had already launched the suite through the
gate supervisor and could hand its run directory forward.

The Go command behavior is already correct and covered: `docket gate launch` creates a supervised
run, `docket gate observe` reports its durable state, and `docket evidence record` accepts only a
terminal passed run at the exact feature head. The defect is solely in the orchestration contract
that connects those operations on the controller-owned re-mint path.

## Decision summary

1. Replace Step 6's incomplete “re-run the suite” instruction with an executable
   **launch → observe → record → verify** chain at the point of use.
2. State the non-obvious `gate launch` argument shape locally, while reusing `docket-build`'s
   existing bounded observation posture rather than copying its polling policy.
3. Extend the existing Bash contract shard for Step 6 with a whitespace-tolerant, section-scoped,
   mutation-proven producer/consumer guard.
4. Regenerate the embedded skill bundle mechanically; make no Go runtime or CLI change.

## Design

### 1. Complete Step 6's re-mint flow

When the build-evidence record is missing, malformed, or stale, Step 6 launches the resolved full
suite through the native gate supervisor:

```text
docket gate launch \
  --root <absolute-run-root> \
  --cwd <absolute-feature-worktree> \
  -- <resolved-suite-command>
```

- `--root` is a writable absolute directory that holds supervised run slots. This change introduces
  no new canonical storage location; the controller may choose an existing task-local run root.
- `--cwd` is the absolute feature-worktree path, so the tested bytes are the branch being certified.
- The `--` separator is required and keeps the resolved suite command and arguments on the child
  side of the CLI boundary.
- The launch result supplies the absolute run directory used by every following operation. A direct
  suite invocation is not an equivalent path because it cannot supply that durable handle.

The controller then calls `docket gate observe <run-dir>` under the gate-execution posture already
owned by `docket-build`, including its bounded observation budget and terminal-state rules. Step 6
names that governing posture instead of restating its observation loop.

Only a terminal `passed` observation whose head equals the current feature head permits:

```text
docket evidence record --id <id> --run <run-dir> --head <feature-head>
docket evidence verify --record <request-file> --head <feature-head>
```

The resulting canonical record is the evidence supplied to review and later publication. The run
directory remains available until recording and verification finish; this change adds no cleanup
policy.

### 2. Failure and invalidation posture

The existing fail-closed behavior remains authoritative:

- A failed launch produces no handle and halts the run.
- Observation-budget exhaustion or a non-`passed` terminal state produces no evidence and halts.
- A vanished, malformed, stopped, failed, or head-mismatched run is never converted into evidence.
- Review never starts from an uncertified branch.
- Any review fix moves HEAD and invalidates prior evidence, so the controller repeats the same
  launch → observe → record → verify chain before publication.

The gate remains tri-state per ADR-0074; this change does not reinterpret native observation output
or invent a passed boolean. Evidence continues to certify the exact tested head per ADR-0066, using
the durable supervisor record established by ADR-0095.

### 3. Contract guard and mutation proof

Extend `tests/test_docket_review.sh`, which already extracts `docket-implement-next` Step 6 and
guards the build-evidence producer/consumer chain. The guard reads the authored
`skills/docket-implement-next/SKILL.md`, not the generated embedded copy.

Within a positively located, non-empty Step 6 section, assert these structural properties:

1. A `docket gate launch` invocation exists on the re-mint path.
2. Its command shape includes `--root`, `--cwd`, and the child-command `--` boundary.
3. Launch precedes observation; observation precedes evidence recording.
4. Evidence recording consumes the produced run directory and binds it to the exact feature head.
5. Evidence verification follows recording and checks the same head.

Collapse whitespace before matching prose-spanning relationships so Markdown reflow is not a
semantic failure. Key the guard on command and ordering shape, not an exact English sentence or a
whole-file token occurrence. Keep the Step 6 existence assertion separate so a renamed or empty
section cannot make a negative condition pass.

The committed test also proves its guard is load-bearing: copy the skill to a temporary file,
confirm the targeted `gate launch` occurrence exists, remove it, confirm the mutation landed, and
run the same structural checker against the mutated copy. The mutated copy must be rejected. The
probe never edits the real worktree, so it needs no destructive restoration.

A Go semantic test is deliberately not added. This contract's authoritative input is Markdown
outside a Go package, and `go test` caching does not track edits to `skills/`. The existing Go tests
continue to own gate/evidence command behavior, while the asset-drift shard owns authored-to-embedded
byte correspondence.

### 4. Audit and generated assets

Read all of `docket-implement-next` for the same local shape: an instruction consumes an artifact or
handle without naming the operation that produces it. Fix another occurrence in this change only
when it is the same bounded instructional omission and requires no new command or policy. A broader
workflow or runtime gap is separate work, not scope expansion.

After the authored skill edit, run the repository's existing asset generator so
`internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md` and the embedded manifest match
the source. Generated files are never hand-edited.

The authored skill currently has limited word-budget headroom. Tighten the existing Step 6 prose as
part of the replacement and keep the file within its standing budget; this focused fix does not
justify increasing the allowance.

## Verification

- The normal Step 6 source passes the structural chain guard.
- The test's confirmed temporary removal of `gate launch` is rejected by the same guard.
- `tests/test_docket_review.sh` passes through the suite runner.
- The skill-size budget test passes without raising `docket-implement-next`'s allowance.
- The embedded-asset drift test passes after regeneration.
- The configured whole suite passes, with every trailing `OVER BUDGET:` report investigated rather
  than ignored.
- The whole-file producer/consumer audit records whether any additional local omission was found.

## Out of scope

- Changing `docket gate`, `docket evidence record`, or `docket evidence verify` behavior or
  interfaces.
- Relaxing the passed-run, exact-head, or durable-run-directory requirements.
- Duplicating or changing `docket-build`'s observation-loop and yield/block policy.
- Changing `docket change mark-implemented`; its producer/consumer chain is already documented.
- Adding a general Markdown linter or a new production parser for skill contracts.
- Restoring deferred learnings harvest behavior from change 0316.
