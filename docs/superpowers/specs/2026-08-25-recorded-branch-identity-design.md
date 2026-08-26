<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0347 — Honor recorded branch names instead of reconstructing feat/<slug>](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-26-0347-honor-recorded-branch-over-feat-prefix.md)**
<!-- docket:backlink:end -->

# Recorded branch identity and per-change branch prefixes — design (change 0347)

## Problem

Docket records a change's real feature branch in `branch:`, but the Go runtime repeatedly ignores
that record and reconstructs `feat/<slug>`. The two names happen to agree for Docket's own current
claims, which let the defect hide. They diverge in repositories using Git Flow (`feature/...`), on
human-minted long-lived stack branches, and after an approved branch rename.

The live failure was an implemented change whose record and open GitHub PR both named a
`feature/...` head. `docket context finalize` nevertheless searched for an open PR under
`feat/<slug>`, found none, and classified the real open PR as closed. Explicit selection could not
recover it. The same reconstruction pattern exists beyond the initial probe: implementation
context, workspace preparation and resume, finalize rebase/block/publish/merge/retarget/closeout,
and cleanup can each act on a branch name invented from the slug rather than the name Docket
already persisted.

There is a second naming problem at claim time. A single `feat/` default is wrong for a manifest
whose change types already distinguish `fix`, `chore`, `docs`, `refactor`, and other work. The
normal prefix should be the change's type, while a human must be able to request a different prefix
for one change during creation.

## Governing rules

1. **Mint once, record once, consume the record.** Claim constructs a full branch name and writes
   it to `branch:`. Every post-claim operation consumes that recorded name. It never reconstructs a
   name from the slug, type, or prefix.
2. **The normal mint prefix is the change type.** A `fix` change mints `fix/<slug>`, a `chore`
   change mints `chore/<slug>`, and a configured custom type behaves the same way.
3. **A per-change prefix may override the type.** Optional `branch_prefix:` is durable human input.
   When populated, claim mints `<branch_prefix>/<slug>` instead of `<type>/<slug>`.
4. **PR identity comes from `pr:`, not branch discovery.** Finalize reads the exact PR number
   recorded in `pr:` and compares GitHub's reported head with `branch:`. It never searches for a PR
   or branch that merely looks as if it might belong to the change.
5. **Unresolved identity authorizes no effects.** Rebase, force-push, merge, retarget, closeout,
   workspace mutation, and cleanup all stop until missing or conflicting identity is repaired and
   freshly re-probed.

These rules complement ADR-0092: stack resolution already follows the branch where a parent's
commits actually live. They also extend ADR-0097's exact-PR-number identity and ADR-0035's
fail-closed cleanup posture to branch identity.

## Manifest and creation contract

Add this optional scalar to the change manifest and templates:

```yaml
branch_prefix:
```

`docket-new-change` accepts a natural-language instruction such as "use the `hotfix/` prefix" and
records the normalized scalar `branch_prefix: hotfix`. Human prose is the input surface; the
structured field is the durable handoff. Claim never searches the change body for naming clues.

The stored prefix is exactly one unqualified branch-path component: non-empty, not `refs/...`, no
slash, and valid under the repository's existing branch/ref validation. The creation writer may
normalize one presentation-only trailing slash from human dialogue before validation; it never
repairs a malformed embedded path or invents a substitute. The change type used as the normal
prefix must pass the same component validation.

`branch_prefix:` remains on the record after claim but becomes informational and inert. This is
intentional:

- creation and claim may be separated by days and different agents;
- an expired, branchless claim that is reclaimed to `proposed` must retain the human's override;
- the record explains why a change's initial branch prefix differed from its type.

There is no continuing equality constraint between `branch_prefix:` and `branch:`. A later
human-approved rename may legitimately make them differ. Once `branch:` is populated, it alone
controls operations.

No repository-wide prefix configuration is added.

## Claim-time construction

Replace the slug-only `BranchForSlug` constructor with a claim-specific constructor over the
change's type, optional prefix override, and slug. Its result is:

```text
(branch_prefix when present, otherwise type) + "/" + slug
```

Claim validates all inputs before its metadata transaction, then atomically stamps the resulting
full name into `branch:` together with the existing claim fields. Existing claimed changes are
unaffected because their recorded branch wins. Proposed but unclaimed changes use the new
type-derived rule on their next claim.

Reclaim remains a deliberate exception to the no-reconstruction consumer rule. It performs
read-only, conservative orphan detection: probe the recorded branch when present and the branch
that a fresh claim would mint from `type:` / `branch_prefix:` / slug. It does not select either
name for a Git mutation. Reclaim preserves `branch_prefix:` when returning a branchless expired
claim to `proposed`.

## Recorded branch as an explicit target

Make the feature branch an explicit validated input to the workspace target constructor. The
constructor still consumes the change id and slug for workspace identity/path purposes and the
resolved effective base for base identity, but it no longer derives `FeatureRef` from the slug.
It validates the supplied short branch and qualifies it once as `refs/heads/<branch>`.

Every post-claim caller reads the current change record and supplies its non-empty `branch:`:

- implementation context and workflow reports;
- workspace prepare, inspect, publish, resume, rewrite, and cleanup;
- PR publication and mark-implemented verification;
- finalize context, block, rebase, publish, merge, retarget, closeout, and cleanup;
- stack parent/child operations and run verification.

The build must derive this population from a whole-repository search for the old constructor and
for hand-built feature refs, then classify each site as minting, conservative detection, or
post-claim consumption. A static guard prevents the retired slug-only constructor or equivalent
`"feat/" + slug` reconstruction from returning in executable code.

A post-claim operation outside interactive finalize that sees an absent, empty, or invalid
`branch:` returns invalid state and performs no mutation. Autonomous workflows halt with the
evidence for a human; they do not choose a likely branch.

## Exact PR probing

Add or expose one GitHub read operation that fetches a PR by repository identity and exact positive
PR number and returns its full normalized facts, including state, head branch, head object id, base
branch, and merge facts where applicable. Reuse the existing authoritative PR snapshot parsing
rather than creating a second JSON interpretation.

Finalize resolves the number only from the recorded `pr:` using the parser governed by ADR-0097,
then performs the exact-number read. `FindOpenPullRequestsByHead` is not used to identify that
change's PR. A clean exact read may establish open, merged, or closed-unmerged; a transport or
decode failure remains unknown and never becomes absence.

For every candidate, finalize compares the exact PR's reported head branch with recorded
`branch:`. Equality is required before the candidate can authorize effects. Stack child probes use
each child's exact recorded PR and recorded branch under the same rule.

## Interactive identity repair

Finalize reports structured identity evidence rather than folding a mismatch into `pr-closed` or
generic `pr-unknown`. The evidence includes the change id and version, recorded PR, exact PR
number/state, recorded branch, reported PR head, and relevant remote/workspace observations.

### Recorded branch and PR head differ

The human receives three choices:

1. **Trust the PR.** Adopt the exact PR's reported head as `branch:`.
2. **Trust the record.** Supply the correct PR reference; adopt it as `pr:` only after an exact read
   proves that PR's head equals the recorded branch.
3. **Abort.** Preserve all state.

### Recorded branch is absent or empty

Finalize uses only the exact recorded PR's reported head as a candidate. It does not search remote
branches by slug/prefix and does not search for another PR. Before offering the candidate, it must
prove the corresponding remote branch exists. The human may confirm the candidate or abort.

### Repair transaction and restart

A repair is a version-pinned metadata mutation. Its request carries the exact evidence the human
approved so a changed PR head, PR number, or change version loses the race rather than applying a
stale choice. After the repair commits, finalize reloads authoritative metadata and repeats the
exact PR probe from scratch. `--id` selects the change but never overrides unresolved identity.

The interactive `docket-finalize-change` workflow owns the question. Non-interactive callers return
the same structured checkpoint and halt for a human.

## Existing workspace safety

A Docket workspace is the separate Git checkout Docket created for one change, together with an
ownership manifest recording its target branch. Correcting metadata must not silently point that
checkout at a different branch.

Finalize may backfill or replace `branch:` only when no Docket workspace exists or the owned
workspace already targets the proposed branch. If an owned workspace still targets the old name,
finalize reports the conflict and stops before editing metadata. This change does not rename local
branches, switch an existing checkout, or rewrite a workspace ownership manifest.

The same fail-closed rule applies to contradictory local refs: a candidate remote branch may be
adopted only when local/workspace evidence is absent or already consistent. Ambiguous or
unobservable local state is retained for manual reconciliation.

## Failure vocabulary

Use distinct machine-readable outcomes so the interactive workflow can distinguish repairable
identity from unavailable evidence:

- missing recorded branch with an exact, verified PR-head candidate;
- recorded branch / exact PR-head mismatch;
- workspace or local-ref conflict preventing repair;
- exact PR unknown (transport/decode failure);
- candidate remote branch absent;
- malformed recorded branch or prefix;
- stale metadata/evidence contention during repair.

The first two may produce a human checkpoint. The remaining outcomes stop with diagnostics and no
mutation. No failure is translated into `pr-closed` unless the exact numbered PR was cleanly
observed closed and unmerged.

## Verification

### Claim and manifest

- Each built-in change type mints `<type>/<slug>`.
- A configured custom type behaves identically.
- `branch_prefix:` overrides type and survives reclaim.
- Creation normalizes a presentation-only trailing slash and rejects empty, nested, qualified, or
  malformed prefixes.
- Existing records with a populated nonconventional `branch:` never consult type or prefix.

### Workspace and consumers

- Workspace targets accept and preserve a valid explicit feature branch distinct from
  `<type>/<slug>` and `feat/<slug>`.
- Every post-claim consumer operates on a recorded `feature/...` or `hotfix/...` name.
- Missing/empty/invalid post-claim branches fail before Git or GitHub mutations.
- Stack parent and child operations use each record's branch independently.

### Finalize identity and repair

- An open exact PR whose head matches recorded `feature/...` is finalizable without head search.
- A mismatch emits both identities and cannot be bypassed by explicit id.
- Trust-PR and trust-record repairs each update only the approved field, reload, and re-probe.
- Missing-branch recovery offers only the exact PR head and requires its remote ref to exist.
- Abort, unknown exact PR, missing remote candidate, stale evidence, and workspace conflict write
  nothing and issue no finalize effects.
- A clean exact closed-unmerged result remains `pr-closed`; probe errors remain unknown.
- End-to-end coverage includes ordinary finalize, stack retarget/closeout, cleanup, and workspace
  resume with deliberately non-derived branch names.

Mutation-test the governing guards: replacing a recorded branch with a reconstructed name must
redden the relevant tests, deleting the exact-PR-number lookup must redden, and removing the
workspace-conflict refusal must redden while proving the conflicting fixture was actually reached.
Run the full suite through the configured build gate.

## Out of scope

- A repository-wide branch-prefix configuration.
- Inferring a branch or PR from slug similarity, prefix conventions, or repository-wide searches.
- Renaming local or remote branches, switching an existing workspace, or migrating workspace
  ownership manifests.
- Changing integration-branch or stacked merge-destination policy.
- Forcing consuming repositories onto one branch convention.
- Inventing a lifecycle state.

## Settled decisions

The human approved all of the following during grooming:

- recorded `branch:` is authoritative after claim;
- the default mint prefix is `type:`, with optional durable `branch_prefix:` override;
- no repository-wide prefix knob;
- exact-PR-number probing only, with no likely-match discovery;
- explicit, in-session human repair in both directions, followed by reload and re-probe;
- missing-branch recovery uses only the exact PR head and requires the candidate remote branch;
- workspace disagreement blocks repair; this change does not rename or migrate the checkout.

## Open questions

None remain.
