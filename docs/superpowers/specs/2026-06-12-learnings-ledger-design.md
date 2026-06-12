# Design: learnings ledger — build-loop memory that close-outs feed and future work reads

**Status:** design (groomed 2026-06-12 via `docket-groom-next`)
**Change:** 0006
**Related:** change 0001 (results artifact — the harvest's richest input), change 0012 (`docket-groom-next` — gains the read), ADR-0003 (convention is reference-loaded; the ledger rules land there once)

## 1. Context / problem

Every `docket-implement-next` run starts cold. PR review feedback, merge-gate corrections, and recurring build findings are recorded per-change (results files, PR threads) but never carried forward — the same class of mistake can recur build after build. CLAUDE.md holds durable project conventions, but there is no low-ceremony, docket-owned place where the *build loop itself* deposits and consumes lessons.

Inspiration (AgentRQ review, 2026-06-11): a per-workspace note agents append learned preferences to, and an append-only security-lessons file a review bot consults — both translated here to one curated markdown file in git.

## 2. The file

`<changes_dir>/LEARNINGS.md` on `metadata_branch`. No new `.docket.yml` knob — the path derives from `changes_dir`, exactly like `BOARD.md`. Like the board it is a **live working document that is never published to the integration branch** (terminal-publish never copies it); unlike the board it is hand-curated prose, not generated output — it is edited only by the harvest/distill procedure, never regenerated wholesale.

Structure: a short header comment stating the contract (what belongs here, the CLAUDE.md boundary, the distill rule), then flat dated entries, **newest first**. Entry format — one to three lines, with provenance and an actionable phrasing:

```markdown
- 2026-06-12 (#12, PR #7) — Verify review findings against the artifact before acting; one
  reviewer claim cited a sentence that did not exist in the file. Apply: byte-diff the artifact
  against its canonical source before implementing review feedback.
```

Decision record (stub's open question 1): fixed category sections were rejected (every harvest would make a classification call, and miscategorized lessons hide); one-file-per-lesson ADR-style was rejected (ceremony would discourage harvesting). Categories can emerge later via a distill pass if growth demands them.

## 3. Writing — the harvest (single writer, single moment)

Decision record (clarified at groom): **close-out harvest only.** Build-time findings already flow through results files (change 0001); a single harvest point keeps the ledger curated rather than chatty.

- The procedure is **single-sourced in `docket-finalize-change`** as a new close-out step, run per finalized change after the merge is verified: distill that change's PR review comments (`gh pr view --comments`), merge-gate feedback, and results-file findings into **zero or more** ledger entries. Zero is normal and explicitly fine — harvest is curation, not ceremony. Commit on `metadata_branch` (separate from the archive commit; the archive commit must stay byte-identical across concurrent archivers).
- Following the terminal-publish precedent, **`docket-status`'s merge sweep invokes the same harvest procedure** (referenced, never restated), best-effort and non-interactive, so changes merged via the GitHub button are not second-class. A change harvested by both (finalize racing the sweep) must be a no-op the second time — the harvester checks the ledger for an existing entry citing that change id and skips if present (entries citing `(#<id>` are the idempotency probe).
- **Kills are not harvested** — `## Why killed` on the archived change already records the rationale.

## 4. Reading

- `docket-implement-next`: reads `LEARNINGS.md` at **plan time** (step 4, alongside the spec read) and again at its **review step** (step 6) — past lessons shape the build and the self-review.
- `docket-groom-next`: reads it in its **scan-related-context step** (step 2) — lessons shape design conversations too.
- No other skill reads it (decision record: "every operating skill" was rejected — status/finalize/adr have no use for build lessons; finalize only writes).

Each is a one-line addition to an existing read list; no new mechanics.

## 5. Distillation (soft cap ~300 lines)

Append-only by default. When the file exceeds **~300 lines** (decision record: 150 was proposed, the human set 300), the **next harvest also distills**: merge near-duplicate lessons, drop entries that have since been promoted to CLAUDE.md or the convention, keep the file readable in one sitting. Git history preserves everything dropped — distillation is compression, not destruction. The boundary rule, stated in the file header: **the ledger is build-loop memory; durable project conventions belong in CLAUDE.md** — promotion during distill removes the entry here.

## 6. Build deliverables

1. **`docket-convention`** — a short *Learnings ledger* subsection: the file path (under `changes_dir`), never-published rule, entry format, harvest/read/distill rules, the 300-line soft cap, and the CLAUDE.md boundary. Single-sourced here per ADR-0003; operating skills reference, never restate.
2. **`docket-finalize-change`** — the harvest procedure (its single source), as a close-out step with the idempotency probe and the distill trigger.
3. **`docket-status`** — the sweep invokes the harvest procedure by reference (same pattern as its terminal-publish invocation).
4. **`docket-implement-next`** — read lines at plan time and the review step.
5. **`docket-groom-next`** — read line in scan-related-context.
6. **Retro-seed** (decision record: ship non-empty): the build's first act creates `LEARNINGS.md` and seeds it from the already-archived changes' results files — five exist on the integration branch (changes 0001, 0002, 0003, 0005, 0012; verified at reconcile 2026-06-12) — giving the entry format worked examples and the first reader immediate value. Not every results file must yield an entry; the zero-is-fine harvest rule applies retroactively too.
7. **Tests** — extend the existing structural pattern: the ledger file exists with its header contract; the five touched skills carry their harvest/read references; sentinel-style assertions that no skill restates the ledger rules (the convention subsection is the single source).

## 7. Out of scope

- Automatic promotion of ledger entries into CLAUDE.md (a human/distill judgment, possibly a later change).
- Harvesting killed changes, build-time mid-change appends, or write access for any skill beyond the harvest procedure.
- Any new `.docket.yml` knob, frontmatter field, or lifecycle status.
- Publishing the ledger to the integration branch.

## 8. Testing

Structural, following the suite's pattern: assert `LEARNINGS.md` is created with its header contract by the retro-seed; assert the harvest procedure lives only in `docket-finalize-change` (referenced elsewhere); assert the read lines exist in `docket-implement-next` and `docket-groom-next`; assert the convention carries the *Learnings ledger* subsection. Behavioral verification of a real harvest is a merge-gate concern for the implementer's results file.
