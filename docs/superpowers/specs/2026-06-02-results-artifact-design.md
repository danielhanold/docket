# docket — change *results* artifact — design spec

**Date:** 2026-06-02
**Status:** Draft (awaiting review)
**Author:** Daniel Hanold
**Relates to:** `docs/superpowers/specs/2026-05-30-docket-design.md` (supersedes its line-401 "results folded into the body" intent)

---

## 1. Problem

Implementing a change sometimes produces a **close-out document** — what was hand-verified at the merge gate, what was discovered during the build, what follow-ups it spawned. docket has **no concept of such a file**: its convention defines change files, specs, plans, ADRs, and `BOARD.md`, nothing else. So when the Markhaus onboarding change (#0004) produced one, it was written ad-hoc and had nowhere defined to land:

```
markhaus/docs/2026-05-31-onboarding-results.md   # orphaned
```

Two concrete failures:

1. **Orphaned** — nothing links the file to the change that produced it. (#0004's frontmatter has no field for it.)
2. **Clutters top-level `docs/`** — with no home, these files accumulate loose in `docs/` (markhaus already has `plan-3-results.md`, `plan-4-results.md`, `plan-5-results.md`, `preferences-pane-results.md`, `spike-results.md`, … from the pre-docket era — the same pattern).

The original docket design (the 2026-05-30 spec, line 401) anticipated such content only obliquely, prescribing that migrated results be *"folded into the body"* of the `done` change. That conflicts with docket's own rule that the change body is a **PM-altitude proposal** (intent + scope) — detailed build evidence and human QA checklists do not belong there. So the right fix is neither "fold into the body" nor "leave it loose," but a **first-class, linked, optional artifact** with defined rules.

## 2. Decision

Introduce the **results artifact**: a change's optional close-out file, modelled as a **true twin of the plan** — a feature-branch build artifact, linked from the change by a new `results:` frontmatter field.

> A change's **results file** is the build's hand-off note: the human's merge-gate verification checklist, the build's findings, and any follow-ups. It is authored in the feature worktree alongside the plan, merges to `main` with the PR, and is linked from the change by `results:`. It is **optional** — written only when warranted (§5), never by default.

This mirrors the existing artifact triad and inherits its already-proven rules:

| Artifact | Produced at | Lives on | Linked by | Archived? |
|---|---|---|---|---|
| spec | brainstorm (propose) | `metadata_branch` | `spec:` | no |
| plan | build (plan step) | feature branch | `plan:` | no |
| **results** | **build (close-out)** | **feature branch** | **`results:`** | **no** |

## 3. Locked decisions

These are the outcomes of the brainstorm, each with its rationale (the spec's mini-ADRs).

1. **Results is a *feature-branch build artifact*, a true twin of the plan — not change metadata.** The bulk of its content (what was verified, what was discovered) is generated *during* the build, so it is authored where the build happens: the feature worktree. It merges to `main` with the PR. This reuses the plan's exact pattern — the file lives on the feature branch; the `results:` **field** is set separately in the main tree (docket's iron rule: the feature branch never edits `docs/changes/` metadata). Rejected alternative — *metadata twin of the change file* (authored in the main tree on `metadata_branch`): would force the build agent to write build-content outside the worktree, against the grain of where the content is produced.

2. **Home is `docs/results/`, a docket-owned dir — exposed as a `results_dir` knob.** *Not* `docs/superpowers/results/`: that namespace is for superpowers-skill outputs (`brainstorming`→spec, `writing-plans`→plan); the results file is a docket concept and mislabelling it there is a category error. *Not* under `changes_dir` (`docs/changes/`) either: the feature branch must never touch the metadata tree, and results is a feature-branch artifact. `docs/results/` sits as a clean sibling of the existing `docs/changes/`, `docs/adrs/`, `docs/superpowers/` subdirs. Because the other two docket-owned dirs (`changes_dir`, `adrs_dir`) are `.docket.yml` knobs, results gets the same treatment: `results_dir`, default `docs/results`.

3. **The link field is `results:`, a single path** (not a list). It slots directly after `plan:` with a parallel comment. A change is ≈ one PR, so ≈ one results doc; a scalar mirrors `spec:`/`plan:`. (A YAML list remains forward-compatible if a change ever needs two, but the documented shape is a single path.)

4. **Optional, written only when warranted — never by default.** The orphan problem was caused by *no rule*; the fix is a *concrete trigger* (§5), not "always write one." Forcing a file per change would trade orphaning for noise and inflate the document count. When no trigger fires, `results:` stays empty and **the PR description + green CI are the receipt.**

5. **Not archived, linked by path** — exactly like the plan and the spec. When the change moves to `archive/` (date-prefixed rename), the results file stays put in `docs/results/`; the archived change links to it by repo path. This is the payoff of the linked-file model over directory-per-change: the change archives without dragging files around.

6. **`results:` resolves on `main` only after merge** — identical to `plan:`. `docket-status` already tolerates an unresolved `plan:` on an `implemented` change (the file is on the unmerged feature branch); `results:` gets the same tolerance, so a populated-but-unmerged `results:` is **not** flagged as a broken link.

## 4. Data model

### `.docket.yml` (one added knob)

```yaml
metadata_branch: main          # unchanged
changes_dir: docs/changes      # unchanged
adrs_dir: docs/adrs            # unchanged
results_dir: docs/results      # NEW — default; docket-owned build-artifact dir
```

Absent ⇒ all defaults (including `results_dir: docs/results`), so existing repos need no edit.

### Change manifest (one added field)

```yaml
plan:                    # plan FILE on the feature branch; this FIELD set in the main tree at build time
results:                 # results FILE on the feature branch; this FIELD set in the main tree at close-out
```

### Directory layout (one added entry)

```
<results_dir>/           # default docs/results/  — feature-branch build artifact; NEVER archived
  <YYYY-MM-DD>-<slug>-results.md
```

Filename: `<YYYY-MM-DD>-<slug>-results.md`, UTC authoring date — mirrors `docs/superpowers/plans/<YYYY-MM-DD>-<slug>.md` exactly.

## 5. The "warranted" trigger

The implementer writes a results file **only if at least one** is true:

- **(a) Human verification** — there are interactive/manual checks the human must run at the merge gate, beyond automated tests (e.g. a first-launch UX flow, a cross-process behaviour outside unit-test reach).
- **(b) Findings** — the build surfaced discoveries worth recording, including any that became ADRs.
- **(c) Follow-ups / deviations** — there are deferred items, spawned proposals, or notable departures from the plan to capture.

Otherwise: **skip it.** Leave `results:` empty; the PR description + green CI are the receipt. This keeps the document count low by construction.

## 6. Content template (`results-template.md`)

Lean and purpose-built — each section *is* a reason for the file to exist. Build-receipt detail ("what shipped", full test tables) is intentionally **omitted**: that belongs in the PR description, not duplicated here.

```markdown
# <title> — results
Change: #<id> · Branch: feat/<slug> · PR: <url> · Plan: <path> · ADRs: <ids>

## Verify (human)
Interactive/manual checks for the merge gate. Each item PENDING until checked.
- [ ] …

## Findings
Discoveries during the build; note which became ADRs.

## Follow-ups
Deferred items / new proposed changes. (Optional — omit if none.)
```

The header line back-links to the change (and PR/plan/ADRs), so the link is navigable in both directions even though only the change carries the machine-readable `results:` field.

## 7. Lifecycle

1. **Author (build time).** During `docket-implement-next` close-out (after build + review), the implementer evaluates the §5 trigger. If warranted, it writes `docs/results/<date>-<slug>-results.md` **in the feature worktree** and commits it on the feature branch with the code.
2. **Link (main tree).** Back in the main tree at step 7, alongside setting `status: implemented` + `pr:`, the implementer sets `results:` to the file path and commits on `metadata_branch`. (Never edits the file or field from the feature worktree — the field is metadata.)
3. **Merge.** The file merges to `main` with the PR; `results:` resolves on `main` from then on.
4. **Post-merge append.** The human / `docket-finalize-change` may append interactive-verification **outcomes** and late findings to the file **on `main`** after merge — which is exactly how the Markhaus onboarding file actually evolved (build receipt + pending checklist first, outcomes and the QL-sandbox finding → ADR-0031 later).
5. **Archive.** On the terminal move, the change file is renamed into `archive/`; the results file stays in `docs/results/`, still linked by path.

## 8. Implementation scope (touch-points)

1. **Convention block** — canonical in `docket-new-change/SKILL.md`, then propagated by `sync-convention.sh` to the other four skills (byte-identical):
   - add `results_dir: docs/results` to the `.docket.yml` example;
   - add the `<results_dir>/` entry to the directory layout;
   - add the `results:` field (with its parallel comment) to the change manifest;
   - update the Branch-model one-liner: the feature branch adds "the plan **+ results** + code" and still never modifies docket metadata.
2. **`docket-implement-next`** — add the conditional close-out sub-step (steps 6–7): evaluate the §5 trigger; if warranted, author the file in the worktree and set the `results:` field in the main tree.
3. **`docket-finalize-change`** — note the human's option to append interactive outcomes / late findings to the results file on `main` post-merge.
4. **`docket-status`** — extend its "unresolved `plan:` on `implemented` is OK" rule to cover `results:` (no broken-link warning until merged).
5. **`docket-adr`** — no behavioural change (it already carries the convention block; it gets the synced edits).
6. **Design spec** (`2026-05-30-docket-design.md`) — add a locked decision for the results artifact and reconcile line 401 (its "folded into the body" intent is superseded by the linked-file model).
7. **README** — document the artifact set including results (if/where it enumerates artifacts).
8. **New `results-template.md`** — ship the §6 template (location mirrors `docket-new-change/change-template.md`; the implementer is its consumer).

## 9. Out of scope

- **Retrofitting other repos.** Migrating Markhaus's existing orphans (`2026-05-31-onboarding-results.md`, `plan-N-results.md`, `spike-results.md`, …) into `docs/results/` and back-linking them via `results:` is a **markhaus-side** cleanup, tracked separately. This change defines the convention; it does not reach into other repos.
- **Directory-per-change** restructuring (considered and rejected — heavy: touches the board glob, id-scan, and the archive move; most folders would hold a single file).
- **Folding results into the change body** (the prior intent — superseded: build evidence and QA checklists violate the body's PM-altitude rule).
- **Multi-file `results:` lists** — forward-compatible but not implemented; documented shape is a single path.

## 10. Open questions

None outstanding — naming (`results:`), cardinality (single path), home (`docs/results/` via `results_dir`), and the optional/triggered model are all resolved above.
