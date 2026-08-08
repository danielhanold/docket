---
id: 266
slug: deterministic-frontmatter-field-writer-no-skill-hand-rolls-a
title: 'Deterministic frontmatter field writer — no skill hand-rolls a manifest edit'
status: proposed
priority: medium
type: feat
created: 2026-08-08
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [140]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced during change 0140's Step 7, writing `status:`/`pr:`/`results:` into a change manifest. The field write used `perl -0pi -e 's/(^---.*?^)claimed_at: .*$/.../ms'`. Under `/s` the `.` in `.*$` matches newlines, so the pattern was greedy to **end of file**, not end of line: the substitution deleted the closing frontmatter fence, the generated `## Artifacts` block, the entire change body, and the whole reconcile log. The truncated file was committed and pushed to `origin/docket` (`973bba5a`) before anyone noticed, because the post-write spot-check read only the frontmatter lines the edit targeted — the damage was entirely *below* the window being inspected. Repaired in `1ba594b1` by restoring from the prior commit and re-applying the writes in line mode. Nothing was permanently lost only because git history held the intact copy.

**Opportunity** — docket has **no deterministic frontmatter field writer**. There is no `set-field` script, no shared library helper, and no facade verb; the facade exposes only `bootstrap`, `env`, and `preflight`. Yet the *field-write rule* obliges every operating skill to write `status:`, `branch:`, `claimed_at:`, `plan:`, `adrs:`, `pr:`, and `results:` on essentially every run — so each of those writes is hand-rolled, per run, per agent, in whatever regex dialect that agent reaches for. This is exactly the boundary ADR-0012 draws: deterministic mechanics belong in a script, and the model's job is deciding *what* to write, never *how* to splice it into YAML.

The gap is already partly acknowledged elsewhere. `AGENTS.md` carries two hand-written rules that exist only because there is no writer to enforce them — "anchor a frontmatter-field edit to the first `---…---` block, never a bare column-0 line match" and the YAML-scalar quoting rule — and `mint-stub.sh` is cited there as "the reference" for quoting at the write boundary precisely because it is the one place a script owns the write. Prose rules in an always-in-context file are what docket uses when it cannot mechanically check something; here it *could*.

**Independent value** — stands entirely with change 0140 reverted. 0140 is a runner-adapter fix and touches none of this. The value is that a whole defect class disappears: no field write can truncate a file, escape its frontmatter block, or emit an unquoted scalar, because no agent is writing the bytes. It also makes the field-write rule enforceable — a guard can assert the skills call the writer rather than that they hand-roll it correctly.

**Boundary** — one script plus its co-located contract (`scripts/set-field.sh` / `.md`), reached through the `docket.sh` facade like every other helper: read a change or ADR file, set one or more named frontmatter fields, anchored to the first `---…---` block, quoting scalars per the AGENTS.md rule, refusing to write on a malformed or missing fence, and leaving the body byte-identical. Then repoint the skills' field-write rule at it. It deliberately does **not** own the `## Artifacts` regeneration (`render-change-links.sh` is already the sole writer of that block), does not own archiving or status *policy* (which fields may move to which values, and when, stays with the skills), and introduces no new frontmatter field.

**Reason for deferral** — cannot ride 0140's branch without destroying it. 0140 is scoped to the `inherit` sentinel across `runner-dispatch.sh` and three runner adapters; adding a metadata-write helper used by every docket skill would expand a four-file adapter fix into a change touching the write path of the whole system, and would need its own spec, its own guard design, and a migration of every existing hand-rolled call site. It also warrants a design pass on the multi-field-per-invocation and ADR-vs-change-file question that grooming should settle, not a build-time side-quest.
