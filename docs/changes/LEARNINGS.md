<!-- LEARNINGS.md — the learnings ledger (contract: docket-convention, "Learnings ledger").
     Build-loop memory: lessons harvested at change close-out, read at groom, plan, and review
     time. Durable project conventions belong in CLAUDE.md — promotion during a distill removes
     the entry here. Newest first. Soft cap ~300 lines; the first harvest past the cap also
     distills (compression, not destruction — git history keeps whatever is dropped). -->

- 2026-06-16 (#11, PR #11) — A test piped a live-producing script straight into `grep -q`; grep
  exits on first match, the still-writing producer takes SIGPIPE, and `pipefail` turned that 141
  into an intermittent failure. Apply: capture a producer's output into a variable first, then
  grep the variable — never `producer | grep -q` under `set -o pipefail`.
- 2026-06-16 (#11, PR #11) — A derived-surface mirror keyed idempotency on a persisted change-file
  field but did no git writes itself, so a bare run (outside the orchestrating pass that records
  the field) re-created every time — and it was pointed at the integration checkout where `active/`
  is pruned, so it only saw archived changes. Apply: a script that reads change files must read the
  metadata working tree (guard the pruned tree) and is idempotent only via the orchestrating pass's
  write-back — drive it through that pass, never bare.
- 2026-06-16 (#11, PR #11) — First-sync close-state keyed on the *pre-existing* id field (empty on
  a fresh mint), so an already-terminal item was created open and only closed on a later pass.
  Apply: when a create-and-set-state pass mints an id, key the state write on the effective id
  (existing OR just-minted), not the stored field.
- 2026-06-12 (#14, PR #10) — A plan's order assertion compared `grep -n` line numbers, which
  cannot order two phrases inside one paragraph; the implementer "satisfied" it by splitting a
  sentence mid-paragraph. Apply: order-assert with byte offsets (`grep -ob`) when both anchors
  can share a line — and treat an implementer contorting the artifact to pass a test as a signal
  the assertion itself is wrong.
- 2026-06-12 (#14, PR #10) — Two views keyed off a body section's *presence* (board cell,
  selection band), but the state transition out (re-arm) didn't remove the section — a re-armed
  stub stayed mislabeled. Apply: when state is encoded by an artifact's presence, every
  transition out of that state must remove the artifact.
- 2026-06-12 (#14, PR #10) — Adding a member to an enumerated set left two stale counts ("six
  skills" in README from 0012, "six operating skills" in the convention from 0014's own edit).
  Apply: when a set gains a member, grep the repo for the old count word and the enumeration.
- 2026-06-12 (#13, PR #9) — A sentinel test guarded every clause of an ordered procedure but
  would have false-passed had the clauses been reordered; review caught it. Apply: when order is
  part of the contract, assert it — compare `grep -n` line numbers, don't just grep for presence.
- 2026-06-12 (#6, PR #8) — The spec asked for a test asserting a metadata-branch file exists,
  but the suite runs against the integration-branch checkout where that file never lives. Apply:
  when specifying tests for metadata-branch artifacts, verify them at build time and record in
  the results file instead — repo tests can only see the integration branch.
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
