<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0154 — Remove stale Bash instructions and duplicated runtime contracts from Docket skills](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-09-0154-audit-skill-bodies-for-the-stale-restatement-class-change-01.md)**
<!-- docket:backlink:end -->

# Remove stale Bash instructions and duplicated runtime contracts from Docket skills

## Design status

Regroomed on 2026-09-08 at the user's request after a fresh necessity review. This design supersedes
the [2026-08-07 design](2026-08-07-audit-skill-bodies-for-the-stale-restatement-class-change-01-design.md)
for change 0154. The earlier file remains historical context; its named Bash files, guards,
dispositions, and exemptions are no longer implementation instructions.

Source baseline: `d73634925bc442d9c3437237e86ac01aca75b43a` on `main`. Recheck the actual checkout
at implementation time. This is a documentation and contract cleanup using existing Go behavior.

## Problem and goal

Skills are executable instructions for agents. A copied list, count, or explanation can become
wrong while tests of the actual implementation remain green. Change 0145 repaired one instance;
0154 was intended to audit the remaining skills. The Go migration removed the old owners but
left skill prose that still names them, so the problem remains and now includes dead instructions.

The outcome is a complete, recorded audit of maintained Markdown under `skills/`, with stale
runtime claims removed or corrected and duplicated mechanics replaced by usable references.
Agents must be able to perform the supported workflow from installed skills and the running
binary, without searching Docket's source checkout or interpreting historical Bash reports.

## Current evidence and required dispositions

These are verified seeds, not an exhaustive list or a replacement for the inventory.

1. **Status report contract.** `skills/docket-status/SKILL.md`, especially “Overview,”
   “Read the report,” “Judgment follow-ups,” and “Final summary,” still teaches the old
   `backlog`/`change`/`ready` lines, `pass ok`, `harvest`, and mirror-mint reports.
   The maintained workflow requests JSON, and `internal/app/status_result.go:StatusResult`
   plus the schema operation describe the current payload; `StatusResult.HumanText` in
   `internal/app/status_human.go` also has a different human report.
   Replace the old report grammar with instructions to validate and interpret the actual
   status/sweep payloads. Keep only fields or tokens needed for an explicit caller decision.
   Do not “repair” the historical missing `health checks failed <exit>` line from absorbed 0159.

2. **Sweep posture and deleted owners.** The status skill points at missing
   `scripts/docket-status.md`, `scripts/board-checks.md`, `scripts/board-refresh.md`,
   `board-refresh.sh`, `render-board.sh`, and `github-mirror.sh`. Its “Sweep posture”
   paragraph describes Bash-era failure stages and retry behavior.
   Remove those dependencies and the old reason-by-reason narrative. Retain current obligations:
   distinguish the read from explicit maintenance; observe a sweep's terminal envelope; inspect
   individual problem entries even when the overall result is applied; preserve unknown state;
   surface findings without unsolicited repairs; and keep sweep recovery distinct from finalize's
   merge authorization. Preserve the existing scope, completion, and integration-sync contracts.
   Derive retry advice from the current operation and its reason, never from the former
   `swept`/`harvest` cross-check.

3. **Board and GitHub mirror claims.** The status skill says a disabled board “must not exist,”
   while the convention's “Board refresh on status writes” says a pre-existing board is left
   untouched. The Go app gates board inclusion and renders it inside the metadata transaction
   (`internal/app/planning.go:fenceBoardSurface` and
   `internal/app/derived_views.go:includeBoard`).
   State the current property: disabling rendering does not authorize deleting a pre-existing
   board; status remains read-only and the report is the summary source.
   The convention, its `github-board-mirror.md` reference, and status's write-back instructions
   still describe an active Issues/Projects mirror. `internal/config/capability.go` classifies a
   repository request for the `github` board surface as unsupported and mutation-blocking.
   Remove active mirror/write-back instructions. If a compatibility explanation is needed, keep
   one concise supported/deferred/obsolete description grounded in the resolver; do not restore
   the feature or remove historical `issue:` data.

4. **Configuration copies.** The convention's “Config layers” paragraph copies the
   coordination-key list, and its configuration sketch/`board_surfaces` paragraph still
   describe retired behavior. Keep the concept of configuration ownership and the supported
   board model, but remove copied key inventories and defaults where a current owner can be read.
   Verify against `internal/config/schema.go`, `internal/config/capability.go`, the resolved
   `diagnostic.config` result, and the shipped `.docket.example.yml`.
   The sketch is now in scope: the old blanket exemption cannot justify false executable advice.
   Preserve useful minimal examples only when they match the current supported configuration.
   Do not describe every parseable key as an active feature.

5. **Remaining legacy explanations and counts.** The convention still advertises single-branch
   main-mode, co-located Bash contracts, and a “Derived-view script family.” It also says “nine”
   exceptions and then “Those eight.” Audit these and analogous passages across all skill
   references. Apply ADR-0099's current metadata topology and point at existing Go operations.
   Prefer removing redundant cardinalities and enumerations. The original 0170 wrapper-count
   exemption is withdrawn: any surviving exemption must cite a current, substantive Go guard
   whose invariant still matters, not a deleted Bash test.

## Scope and boundaries

In scope: every tracked Markdown file under `skills/`, including templates and references.
Discover this population from the checkout; do not hard-code a file count or stop at the seeds.

The implementation may also touch directly dependent Go tests, required small corrections to an
existing reference chosen as an owner, and generated installation assets. Any outside-`skills/`
edit must name the in-scope edit that requires it. No general documentation rewrite is implied.

Historical specs, archived changes, merged plans/results, Accepted ADR bodies, and frozen fixture
corpora are evidence rather than cleanup targets. Do not update them to modernize old wording.
A fixture refresh required by an existing correspondence guard follows that guard's supported
regeneration procedure and must be explained separately; it is not a license to rewrite history.

Runtime behavior, CLI/schema/config interfaces, defaults, workflows, permissions, and feature
support stay unchanged. Do not resurrect the GitHub mirror, publication, main-mode, or Bash
tooling. Do not implement gaps found in the runtime or add a new documentation framework.

## Ownership and reference rule

Use this preference for each hit:

1. **Delete and point** when the binary or a current reference already owns the contract.
2. **Compress to the caller's judgment plus a pointer** when mechanics and agent obligations are
   mixed. Retain prerequisites, authority checks, failure posture, and required follow-up at the
   place where they affect the decision.
3. **Keep and prove correspondence** only when a literal set genuinely must remain in the loaded
   instructions. Name the current owner and non-vacuous protection; no exemption by old test name.

A single operation name, a token that a specific branch tests, a minimal usage example, and
convention-owned definitions are not automatically duplication. Completeness is needed only where
the prose claims a complete set. Never delete an essential branch predicate merely to avoid a token.

The runtime reference channels are distinct:
- Resolve argv, flags/signatures, and effects through the validated capability catalog.
- Resolve described request/result shapes and named vocabularies through the catalog-resolved
  schema operation, on demand for the operation in use.
- Resolve actual configuration values and capability classifications through existing supported
  configuration/context operations. A payload schema is not automatically a full configuration
  manual or a description of retry semantics.

During development, Go source and tests establish truth; they are not a new runtime discovery
instruction for an agent working in a consuming repository. The schema has a deliberate opaque
type boundary (ADR-0109), so “ask schema” is not an adequate replacement for information it omits.
Where needed information has no usable owner, preserve its unique supported meaning in one small,
installed skill reference, then point there. Do not relocate the entire stale narrative.

Every new pointer must resolve to an actual operation, file, and section/symbol as applicable, and
be reachable in the installed skill layout. A repository-local `docs/` or `internal/` path is
insufficient as the sole operational reference for consuming repositories. Verify a proposed
owner before linking to it; existing documentation can also be stale.

Preserve caller-specific differences. A maintenance pass's independent-item continuation and
a finalize controller's merge gate must not be flattened into one generic error rule.

## Inventory and implementation evidence

At build time, record the baseline revision and discover every in-scope file. Perform mechanical
searches plus a full manual read. Mechanical seeds include operation names/signatures from the
catalog, available vocabularies from schema, configuration classifications from their owners,
references into the removed script tree, copied count words, and detailed output/retry narratives.
Derive token populations from their owners rather than writing another hand-maintained registry.

The plan/results must account for every reviewed file and every hit. For each hit record its
path plus section/symbol or quoted clause, current owner, evidence, disposition, surviving caller
obligation, dependent test coverage, and any residual work. Record “no hit” files too, so the
whole-skill audit is reviewable. Known seeds may be already fixed at build time; record the
replacement and its evidence rather than reapplying an obsolete edit.

Before each deletion or compression, search the entire current test surface for the affected
clauses, including whitespace-normalized searches where prose wraps. A guard over a surviving
property moves to its new owner; establish and mutation-test replacement coverage before removing
the old protection. An assert whose only subject is a retired copy may be removed with its premise
and rationale recorded. Do not preserve wrong prose or weaken assertions just to make tests green.

## Guard and validation strategy

There is no new general “no duplicated prose” or closed-token ban. Such a test cannot distinguish
a necessary decision token from a copied inventory without accumulating its own sanction list.

Reuse `internal/repoguard/prose_contracts_test.go`,
`internal/repoguard/capability_surface_test.go`, and the applicable existing absence/config/asset
guards. Preserve each still-valid invariant. Add or strengthen narrowly scoped checks only for
concrete instruction regressions changed here (for example, the JSON report contract or the
prohibition on reintroducing an active mirror recipe). Bind checks to the subject and obligation,
not merely a phrase appearing anywhere. Any new or changed guard must reject a mutation of the
specific regression it claims to prevent and permit legitimate token references and reflow.
For operation/literal scans, derive the population and match syntactic shape rather than a list of
retired spellings. State where prose needs manual review; never claim the existing fenced-code
absence seal proves all prose current.

Regenerate the embedded bundle using the existing `internal/assets` generator when authored
skills change; verify it matches the authored tree. If a reference is moved or retired, verify
all maintained incoming links and its installed presence/absence. Do not hand-edit generated
copies. Respect the existing word and runtime budgets; read budget advisories even on a green run.

Run the complete build gate from the feature source checkout using the command resolved from
`build.test_command`, through `internal/suiterunner`; the current resolved command is
`go run ./cmd/docket development test`. Record the actual result, relevant guard mutations, asset
correspondence, and budget findings. Finalize independently uses its resolved
`finalize.test_command`. Do not execute maintenance, mirror writes, or finalize as validation.

## Acceptance criteria

- Every Markdown file under the discovered `skills/` population is accounted for.
- All current seed defects have a verified disposition; no operational instruction depends on
  deleted Bash tooling or the old status report grammar.
- Status summaries use the current structured report, retain completion/per-entry checks, and
  do not infer board-file absence from disabled rendering.
- Skills describe the currently supported board/configuration behavior; no active mirror
  write-back or single-branch compatibility recipe remains.
- Copied inventories/counts are removed, reduced to owned judgment, or individually justified
  against a current owner and meaningful coverage. Old guard exemptions are not inherited.
- Required caller authority, stop/continue behavior, and safety checks survive compression.
- Replacement references are accurate and usable from installed skills in a consuming repository.
- Dependent guards remain substantive, relevant mutations fail as intended, generated assets
  match source, and the full configured suite passes with budget findings reviewed.
- The results record unresolved or out-of-scope findings honestly; no unrelated backlog work is
  silently marked complete.

## Related work and historical dispositions

Changes 0370, 0372, and 0377 removed/migrated the Bash surfaces; 0363 removed main-mode;
0394 and 0399 provide the current discovery channels. These predecessors are done, so this change
needs no new dependency or stacked base. Existing historical relations to 0111/0144/0157/0159 and
discovery from 0145 remain.

0159's omitted report-line case is obsolete as a literal repair and covered here by replacing the
report model. No Bash guard from 0111/0145/0170 is assumed to survive unchanged.

Deferred change 0257 names the convention's stale “superpowers default shown” sketch comment and
explicitly suggests consolidation into 0154. Correct or remove that comment as part of the sketch
audit and record that one item's disposition. Its remaining rationale restoration, shell guidance,
and broader rationale-loss work are outside this change; 0257 stays deferred and is not declared
fully absorbed. Other adjacent findings are reported with their existing change ids when known.

Applicable precedents: ADR-0003 (single ownership), ADR-0012's ownership boundary as evolved by the
Go migration, ADR-0054 (stable anchors), ADR-0099 (one metadata topology), ADR-0104 (capabilities),
and ADR-0109 (schema fidelity). This design requires no new architecture decision.

## Assumptions and alternatives

1. **Retain and regroom 0154.** Killing it was rejected because live contradictory instructions
   remain. Implementing the August design verbatim was rejected because its owners and guard
   assumptions have been deleted.
2. **One bounded skill audit.** Patching only the status skill misses the same class in the
   convention and references. A repository-wide prose cleanup would mix unrelated work.
3. **Prefer removal and usable references.** Pinning every duplicate increases the number of
   synchronized surfaces; blanket deletion would lose necessary caller judgment.
4. **No automatic exemption for a guarded copy.** A current guard can protect a false statement
   or only one phrase; its real invariant and coverage must be examined.
5. **No schema expansion.** Existing discovery and a small installed prose owner, where needed,
   suffice for this documentation change. A true interface gap is reported separately.
6. **Preserve runtime and historical records.** Correct the instructions to what the implementation
   supports; do not change the implementation to make old prose true.
