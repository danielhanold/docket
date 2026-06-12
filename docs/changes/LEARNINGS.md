<!-- LEARNINGS.md — the learnings ledger (contract: docket-convention, "Learnings ledger").
     Build-loop memory: lessons harvested at change close-out, read at groom, plan, and review
     time. Durable project conventions belong in CLAUDE.md — promotion during a distill removes
     the entry here. Newest first. Soft cap ~300 lines; the first harvest past the cap also
     distills (compression, not destruction — git history keeps whatever is dropped). -->

- 2026-06-12 (#12, PR #7) — A code-review finding cited a sentence that did not exist in the
  reviewed file. Apply: verify review claims against the artifact (byte-diff against canonical
  content) before implementing fixes; reject false positives with evidence.
- 2026-06-12 (#12, PR #7) — link-skills.sh needed no edit for a new skill — it globs skills/*/.
  Apply: at reconcile, check whether plumbing auto-discovers before planning an edit to it.
- 2026-06-10 (#5, PR #6) — A full convention restatement hid in paraphrase ("satisfied = done")
  where fixed-string sentinels could not see it. Apply: sentinel greps are sampling, not parsing;
  pair them with a whole-branch review that reads for meaning.
- 2026-06-10 (#5, PR #6) — YAML frontmatter: an unquoted scalar value cannot contain ": "
  (colon-space). Apply: reword with an em-dash or quote the scalar in skill descriptions.
- 2026-06-04 (#2) — A backward-compat test assertion was vacuous (any "main-mode" mention
  satisfied it). Apply: prove each assertion non-vacuous — deleting the clause it guards must
  flip the test to NOT OK.
- 2026-06-02 (#1) — Fragmenting a tightly-coupled single-artifact edit across subagents risks
  inconsistent edits to shared content. Apply: build inline when tasks share one artifact; fan
  out only for genuinely independent tasks.
