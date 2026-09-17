# Native Codex runner completion — change 432

## Outcome

The compatibility objective was demonstrated on 2026-09-17: the real native
`docket-implement-next` acceptance run for change 431 reached `run-complete`.
The parent observed the original coordinator terminate before obtaining its keyed
`gate-done` verdict. This was successful completion with explicitly authorized human
recovery interventions, not proof of intervention-free operation.

The repair candidate was `e8b5391816e1f6eff22d60161d947c198f8907a4` on
`fix/complete-native-codex-runner`. Its repairs were applied to the preserved 431
checkout and delivered through [PR #309](https://github.com/danielhanold/docket/pull/309)
into change 425 at `8982b2873580f5677a0900abc94bfaef22403028`.
Comparison of those two source trees showed no differences outside `docs/`.
This documentation PR does not deliver the repair a second time, merge 425 into
main, or claim that the repair has shipped on main.

## Preserved records

- [Original implementation plan](../../superpowers/plans/2026-09-17-complete-native-codex-runner.md).
- [Original bootstrap results](../../results/2026-09-17-complete-native-codex-runner-results.md).
- [Historical investigation and handoff](0432-codex-runner-handoff.md).
- [Shared simplification findings](../shared-orchestration-simplification.md).
- [Supervisor findings for change 412](../0412-supervisor-findings.md).

The plan and bootstrap results are preserved byte-for-byte from the candidate above
at their original paths. Their unchecked tasks and pending-acceptance language are
historical checkpoint statements, not a current completion verdict. This report
supplies the subsequent outcome without rewriting that history.
The research notes originated on the `docket` metadata branch, as observed at
`e8aaab760ee334a4019fe265983a93706acd9abd`. Their relative links have been adjusted
for this source-tree location. Metadata originals remain intact pending merge;
they should subsequently become pointers rather than competing maintained copies.

## Verification and limitations

Acceptance preserved the original 431 plan and worker work, passed configured
certifications, obtained exact-HEAD evidence and native review, attached results,
and published PR #309. The native reviewer raised one blocker about replacement
admission; inspection showed the driver supplied the authenticated change identity,
so that finding was recorded and disposed of as a false positive. A prematurely
written halt was cleared through supported resume. The acceptance feature head was
`6cc7fd63936b587b2310f1848b3e999114c424fe` before finalization rebased it to
`f6695e5f8fbb198ba9458d40cfeb215a2af928bc`.

The operator explicitly authorized resetting the existing 431 build suite budget
from four spent attempts to zero. The prior run was cancelled first; a protected
before/after audit was retained. No budget ceiling or general reset policy changed.
After implementation completed, finalization was blocked by a stale execution
ownership label. Cancellation was confirmed and the operator authorized clearing
that specific label, with a protected audit. These interventions are limitations
of the demonstrated workflow, not capabilities added by the receipt repair.

Finalization passed 56 suite files and 460 assertions, but serial confirmation found
the closeout shard at 61 seconds against its 60-second limit. The operator waived
that specific timing finding. No blanket timing waiver or policy change followed.
PR #309 merged into 425 on 2026-09-17; 431 became `stacked-merged`.

Private gate keys, capabilities, ownership records and raw transcripts are excluded
from these documents. Protected raw evidence remains in the local 432 launch kit;
the sanitized conclusions here do not require exposing that kit's credentials.

## Remaining work

432's documentation closeout targets 425 and must not be marked `stacked-merged`
until its own PR actually merges and the lifecycle requirements are satisfied.
Its pre-created workspace lacks managed adoption, and its plan/results attachments
remain unresolved; creating a PR alone does not repair those metadata prerequisites.
No workspace manifest, evidence receipt or merge record should be fabricated.

Shared simplification, timing calibration and supervisor 412 remain deferred.
425's source review, integration preparation and the separately approved 424 dogfood
sequence remain distinct work. This report authorizes no further implementation,
acceptance dispatch, main merge, or global runtime replacement.
