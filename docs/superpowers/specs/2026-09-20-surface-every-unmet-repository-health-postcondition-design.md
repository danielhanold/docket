<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0418 — Surface every unmet repository health postcondition](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0418-surface-every-unmet-repository-health-postcondition.md)**
<!-- docket:backlink:end -->

# Report every applicable unmet repository health condition

Change: 0418. Design settled with the human on 2026-09-20; broader reporting scope explicitly selected.

## Intent

Users of `repository check` must be able to identify every applicable unmet health condition from one report, including failures hidden behind `postconditions-unmet` and independent failures masked by an earlier specific diagnosis. The reported missing `.opencode/agents/docket-*.md` entry is a regression case, not the definition of the problem. Preserve the current health policy, classification precedence, exit codes, and read-only behavior.

## Assumptions and approved scope

The human chose the broader reporting scope: supplement existing specific diagnoses with other applicable unmet health conditions. Keep the existing classifier's ordered state selection and reason tokens; completeness of reporting must not become a new authorization rule. Fresh/legacy repositories and operations that fail before gathering facts retain their existing reports, because the check does not gather the metadata-health postconditions in those cases. Unknown or inapplicable dependent observations are not fabricated failures.

Two required examples are a dirty metadata worktree plus a missing committed ignore entry, and pending setup edits plus hooks not disabled on the metadata worktree. The report must expose both issues in each example without changing the selected state or exit code.

This is a non-trivial, bounded diagnostic change with a written design. It adds neither a new workflow nor a new repository-health policy. No dependency or stack parent is needed.

## Evidence

The source examined is main at `57794104aef05dc02ca7e3a7f0d1658ee067234d`.

- `RunRepositoryCheck` gathers facts, calls `augmentCheckFacts`, then passes the classification and facts to `EvaluateHealth`. `RepositoryCheckResult.HumanText` and JSON already expose the same `Finding` values.
- `Classify` has an ordered ladder ending with a conjunction that proves health. Every input reaching that conjunction that fails it returns the sole reason `postconditions-unmet`. `findingFor` turns that token into a generic message and a circular remedy to resolve unspecified issues.
- `committedIgnorePresence` reads `.gitignore` from the pinned integration commit. It reduces a read failure to Unknown, a valid canonical block to Present, and all other readable inputs to Absent. It currently loses missing-entry and malformed-block detail.
- `ValidGitignoreBlock` accepts an exact, line-anchored canonical byte sequence. `GitignoreBlock` owns the expected bytes; `gitignoreMarkersMalformed` already checks marker balance for the current and legacy generations. Validation is not a general Git-ignore semantics check.
- `pendingReviewPaths` can select needs-review when the working-tree block is fixed but the committed block is invalid. The reported fallback regression must use a fixture without that earlier condition.
- #352 introduced the shared classifier and ordered findings. #378 established ownership verification and unknown-evidence safety. #383 removed a dead metadata-fetch diagnostic append from a copied context; restoring that append would not provide a reporting path. #403 established propagation through the existing findings shape and shared human renderer. All are done.
- ADR-0020 owns the managed ignore block and machine-local artifact policy; ADR-0025 owns disabled hooks on metadata worktrees; ADR-0099 preserves the sole metadata topology. Their policies are unchanged.
- Nearby active #350 concerns transaction-error propagation, and #320 concerns fixture ignore rules. Neither overlaps this implementation or blocks it.

This evidence comes from tracing source and existing tests. The original real-world missing-entry incident has not been reproduced during grooming.

## Design

### Evaluate existing health conditions independently of state precedence

Factor the terminal healthy-conjunction evaluation into a small pure helper in `internal/reposetup`, which returns the unmet condition identifiers in a fixed order. `Classify` uses its empty result to select healthy, preserving every earlier branch and retaining `postconditions-unmet` as the terminal reason. `EvaluateHealth` uses the same evaluation to supplement all applicable reports after collecting their existing reason-based findings. This avoids maintaining a separate diagnostic predicate list that can drift from the healthy decision. The helper belongs to the existing reposetup package; it is not a configurable rule registry or a new subsystem.

The evaluation must cover the full existing conjunction: remote metadata presence; verified metadata ownership; local metadata presence; metadata worktree presence, registration, foreign status, cleanliness, synchronization, and hooks configuration; committed ignore validity; absence of the live integration surface and legacy config key; primary cleanliness, integration-branch attachment, and equality with the pinned remote tip; authorized surface agreement; and pending review paths. Do not add conditions inferred from historical prose that the current classifier does not enforce.

Retain the generic summary finding when the classifier actually selects that fallback, and accompany it with specific findings explaining the unmet conditions. Do not add a generic summary to reports that already select a specific reason. Use the existing category ordering and `Finding` fields (`code`, `severity`, `ref`, `message`, `remedy`). Give each condition a stable code, and distinguish an unresolved observation from a proven wrong state in its message. Existing specific codes may be reused where their meanings match exactly; do not call an unknown ownership proof foreign or an unknown hook setting enabled. Keep the existing reason-based findings and avoid adding a second finding for a condition they already explain. Where an existing finding covers several alternatives, enrich its message to identify the alternatives actually observed. Dedupe by the condition explained, not merely by state, category, or the existence of any error. A dirty-worktree finding cannot suppress a missing ignore entry, and a pending-review finding cannot suppress a hooks failure. Retain committed-ignore detail even when `.gitignore` is also a pending review path; naming a pending path does not explain its committed defect.

For an actual `Classify` result, the generic finding must never stand alone. Preserve healthy output and all existing state/exit mappings, including supplemental Unknown values that currently reach conflict rather than the earlier unknown-authority branch. The change explains those values; it does not reclassify them.

### Applicability and dependent observations

Reuse the existing probe boundaries rather than introducing a fourth probe state, a run-wide trace, or new Git reads:

- Supplemental metadata-health reporting runs only when remote metadata is proven present, matching `RunRepositoryCheck`'s existing augmentation boundary. A fresh/legacy repository is not reported as having missing worktree hooks, ignore entries, or local metadata that this check never inspected.
- Report unresolved ownership and local metadata presence independently once metadata is present. An unknown ownership proof does not imply foreign ownership. Other successfully observed failures remain visible alongside it.
- Committed ignore validity, the obsolete config key, the live integration surface, and primary equality with the remote tip require a resolved pinned integration commit. When that prerequisite is unresolved, keep its existing diagnostic; do not describe skipped reads as missing files or separately failed child probes. When a committed read was attempted and failed, report that specific condition as unverified.
- A missing, unreadable, or foreign `.docket` path explains why its dependent registration/cleanliness/hooks facts cannot establish health. Report that blocking observation without a cascade claiming disabled hooks or cleanliness were independently checked. With a present, non-foreign path, report an unresolved registration as such. The existing cleanliness/hooks probes are attempted for a present path, so their observed failures may be reported there; do not relabel an unresolved registration as proven foreign. Synchronization detail requires both branch presences and tips to be known; otherwise its missing prerequisite already explains the unproved relationship.
- Surface agreement matters only when `SurfacesAuthorized` is true. Keep the current surface probe behavior, including its present implementation limitation; adding a new drift detector is outside this change. Primary cleanliness/branch attachment and nonempty pending paths use the observations the existing gatherer supplies.
- Existing partial/migration diagnoses explain their corresponding live-surface or attach condition but do not suppress independent committed-ignore, hooks, or primary-worktree issues. Unknown-authority reports may include independently established defects where their prerequisites are met, while remaining unknown with the same exit code.

A dependent condition suppressed for lack of its prerequisite must be represented by that prerequisite's concrete finding. This is explanatory grouping, not permission to drop unexplained failures. The raw healthy-conjunction evaluation remains complete and unchanged; applicability only controls how existing observations are presented.

### Preserve ignore details at the existing read boundary

Extend the existing committed-ignore probe result to carry the small amount of diagnostic data it currently discards, alongside the unchanged three-valued presence. Carry that data into the existing facts-to-findings path. No second file read, raw-file payload, new public result array, or general probe-error framework is needed.

Use a pure explanatory helper beside `ValidGitignoreBlock`, reusing `GitignoreBlock` and the marker routines. The existing validity predicate remains authoritative: inputs it accepts today remain accepted. Only rejected inputs need explanatory detail.

Distinguish these cases:

- The committed `.gitignore` file or current managed block is absent.
- Only legacy markers are present.
- The marker structure is malformed, naming the observed structural defect.
- A well-formed current block lacks one or more canonical entries. Derive the expected entries from the canonical block, and report all missing entries in canonical order. An entry elsewhere in the file does not satisfy membership in the managed block.
- The block contains all expected entries but fails the exact canonical representation, such as reordered lines, extra lines, or newline differences. Explain that mismatch rather than inventing a missing entry.
- The committed blob cannot be read. Keep Unknown and report inability to verify it; do not claim the file or an entry is missing.

The canonical-plus-extra-malformed-block case remains governed by today's acceptance predicate. Duplicate-block enforcement, negation analysis, and broader ignore validation are outside this change.

### Remedies and output

Every added finding identifies the affected condition and a known path/ref or setting where applicable. For `.gitignore`, explicitly say the defect is in the committed integration tree. An uncommitted local fix does not establish the guarantee.

Remedies must fit the observed state. Missing ignore entries can name the entries to restore and the need to review, commit, and publish the corrected managed block. Malformed markers require inspecting and correcting the reported structure first. Failed probes require restoring readable evidence and rechecking, without asserting an unavailable underlying cause. Repository-state remedies must preserve local work and cannot blindly prescribe initialization, migration, resets, or cleanup for every condition. Reuse existing setup/repair commands only where their actual admission rules support the diagnosed state.

Both output formats consume the same finding values through the established renderer and JSON serialization. Preserve the existing result envelope, revisions, and frontmatter/test-configuration findings. No renderer redesign is required.

## Verification and acceptance

Extend the existing classifier, health, gitignore, and repository-check tests. Keep most condition combinations and applicability/deduplication cases in fast pure tests; real-Git cases belong in the established integration partition.

1. Starting from the healthy facts fixture, vary every conjunct through its non-satisfying states. Assert unchanged classification precedence and exit behavior. Assert the concrete condition finding or its precise prerequisite explanation, rather than merely nonzero exit or finding count. Include earlier specific branches, not only terminal fallback results.
2. Use multiple simultaneous terminal failures and mixed earlier-branch failures to prove all independent causes survive and retain deterministic category ordering. Pin both approved examples: dirty metadata plus a missing ignore entry, and pending-review plus hooks not disabled. Add partial plus an independent ignore defect, unresolved ownership plus a known ignore defect, and a condition already explained by an existing reason to prove deduplication. Mutate away an individual diagnostic population step and prove its acceptance test fails; suppressing the generic summary alone is not sufficient coverage.
3. Add pure ignore cases for missing file/block, missing OpenCode entry, multiple missing entries, legacy markers, malformed structure, reordered/extra content, and canonical acceptance. Include accepted edge cases so explanatory work cannot silently tighten validity.
4. Add a committed-file integration regression deleting only the OpenCode entry from the canonical block, committing/pushing it, and leaving the working tree unchanged. Assert that human and JSON output name `.gitignore`, the exact missing entry, and its remedy, while state and exit remain the same.
5. Preserve the existing working-tree-versus-committed-file fixture and its needs-review precedence. Add a failed committed-blob read and retain the failed metadata-fetch fixture; neither may be labeled clean absence or foreign ownership.
6. Reuse the read-only snapshot checks and healthy baseline. Preserve state and exit behavior for fresh, legacy, partial, needs-review, specific conflict, and unknown-authority cases while asserting the new supplemental findings where applicable. Assert no dependent-failure cascade for a missing/foreign worktree, unknown integration authority, or an absent metadata branch; unauthorized surfaces must not produce findings. The previously healthy baseline must stay unchanged.
7. At implementation time run the complete source-tree build gate resolved from `build.test_command`, and handle budget findings under `tests/README.md`. Grooming itself does not run the build suite.

## Alternatives and exclusions

A `.gitignore`-only message leaves other causes behind the same generic fallback, so it does not fulfill the problem. A new health-rule registry or generalized probe-tracing system is unnecessary because the existing facts, classifier, finding categories, and output pipeline provide the required structure. Limiting expansion to the terminal fallback would still make users fix one issue and rerun to discover another. The human explicitly chose broader reporting, implemented as supplemental findings over the same observed facts and classifier conditions.

No new configuration, command, automatic repair, extra remote probe, metadata ownership rule, ADR, or implementation plan is introduced. Preserve the existing distinction between classification and operation-specific authorization.

## Metadata exit

Attach this spec with `change.groom`, keep `trivial: false` and `status: proposed`, and replace the stub's old small-propagation-fix rationale with the evidence-based scope. Use `related: [352, 378, 383, 403]`, `adrs: [20, 25, 99]`, no dependencies, and no stack parent. The transaction owns artifact links and board rendering. Stop at the spec.
