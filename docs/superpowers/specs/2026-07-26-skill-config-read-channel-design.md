<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0120 — docket-finalize-change claims integration_branch is read from .docket.yml, but it is an exported resolver key](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0120-docket-finalize-change-claims-integration-branch-is-read-fro.md)**
<!-- docket:backlink:end -->

# Skill config read-channel: correct the finalize provenance claim and guard the class

Change: #0120 · Groomed autonomously by `docket-auto-groom` (2026-07-26), one adversarial critic round applied.

## Problem

`skills/docket-finalize-change/SKILL.md` tells the agent that `<integration_branch>` is "resolved
from `.docket.yml`". It is not: `INTEGRATION_BRANCH` is emitted in the Step-0 `preflight` export
block, exactly like `FINALIZE_GATE` and `CHANGES_DIR`. ADR-0052 (change 0102) already states the
rule this violates — a documented key resolves through `docket-config.sh` and skills read the
exported value; a model-read of `.docket.yml` is not a supported shape.

This is the second occurrence of the class in the same file. 0102 fixed
`finalize.require_pr_approval`; nothing stops the third.

## Scope

Four pieces, in order:

1. **Correct the one false claim.** `skills/docket-finalize-change/SKILL.md`, step 1 of *Per-change
   steps*, the clause `(resolved from `.docket.yml`; not hard-coded `main`)` → name the exported
   `INTEGRATION_BRANCH` read from the Step-0 `preflight` block, keeping the "not hard-coded `main`"
   half. Match the phrasing 0102 established a few lines below, in *The rebase-retest merge gate*:
   "read from the Step-0 `preflight` export block … never by parsing `.docket.yml`".

2. **The audit is done — its result, with the counts verified.** A whole-repo grep of `skills/`
   for `.docket.yml` returns **16 occurrences across 5 files**:

   | Site | Count | Verdict |
   |---|---|---|
   | `docket-finalize-change/SKILL.md` — *Per-change steps* step 1 | 1 | **the bug** — fix |
   | `docket-finalize-change/SKILL.md` — merge-gate paragraph | 1 | correct: **negative** ("never by parsing") |
   | `docket-status/SKILL.md` — `github_project` write-back ×2 | 2 | correct: **write-back** (a write to the file, not a read channel) |
   | `docket-convention/github-board-mirror.md` — `github_project` write-back | 1 | correct: **write-back** |
   | `docket-convention/SKILL.md` | 6 | contract prose about the file itself — see §4 |
   | `docket-convention/references/agent-layer.md` | 5 | describes the config layering itself |

   The three other `<integration_branch>` occurrences in the finalize skill use it as a bare
   placeholder and assert no provenance. Nothing else needs an edit. **The implementer re-runs the
   grep at reconcile** and reconciles against what it actually finds — this table is a snapshot,
   not an oracle (learning: `verify-the-claim`).

3. **Guard the class with a sentinel** — new `tests/test_config_read_channel.sh` (the suite is
   glob-discovered as `tests/test_*.sh`; no registration needed). Every `.docket.yml` occurrence in
   the scanned population must be classified, and an **unclassified occurrence fails**.

   - **Population is computed, not hand-listed.** Follow the in-repo precedent at
     `tests/test_skill_size_budgets.sh` (its completeness guard auto-discovers `skills/**/*.md`):
     `find skills -name '*.md'`, then subtract a **declared, short exclusion list**. A new skill
     file — or a new template — is therefore scanned by default rather than silently exempt.
   - **Exclusions (declared, each with a reason in the test):** `skills/docket-convention/SKILL.md`
     and `skills/docket-convention/references/agent-layer.md` — see §4.
   - **Admissible classes, marked at the line, not inferred:** the guard reads an explicit
     line-level opt-out marker (an HTML comment carrying the class, e.g.
     `<!-- docket:config-read-channel: write-back -->` / `: negative`) or an equivalent declared
     `(file, verbatim-quoted clause, class)` manifest in the test, mirroring ADR-0052's own
     manifest pattern in `test_docket_example_yml.sh`. **Do not** infer the class from the line's
     wording: `docket-status/SKILL.md`'s second occurrence ("record back into the change file /
     `.docket.yml`") names no key at all, so any "the line must name the written key" rule reddens
     on correct pre-existing prose, and widening it to cover that spelling reintroduces the
     enumerated-spelling anti-pattern AGENTS.md forbids. Marking the **four** legitimate sites is
     part of this change's edit: `docket-finalize-change/SKILL.md`'s merge-gate line (`negative`),
     `docket-status/SKILL.md` ×2 and `github-board-mirror.md` ×1 (`write-back`).
   - Failure messages name the file, the line, and the line's text.

4. **`docket-convention` exclusions — examined, not assumed.** The two excluded files are the
   contract that *describes* `.docket.yml`, so a read-channel rule cannot apply to them as written.
   One line was specifically checked because it carries the guarded shape for this very key —
   `skills/docket-convention/SKILL.md`: "`integration_branch` is a value *read from* the file, so
   the file cannot be located *by* it." Read in context, that sentence is about **where
   `.docket.yml` lives** (it cannot be located by a value stored inside it), not an instruction to
   any agent to parse the file; the same file later attributes the read to the resolver explicitly
   ("read `.docket.yml` authoritatively" — the resolver's job, in the *Config layers* discussion a
   few paragraphs on). **The implementer
   re-reads that paragraph at reconcile and confirms this** before accepting the exclusion; if it
   reads instead as a read-channel instruction, rephrase it in the same change ("a value the
   resolver reads *from* the file"), which fits convention SKILL.md's budget.

## Guard discipline

Per AGENTS.md and the `marker-scoped-guard-needs-a-population-floor` /
`backstop-must-compute-not-reenumerate` learnings, the sentinel must:

- **Assert a population floor** — the discovered file set is non-empty and includes
  `skills/docket-finalize-change/SKILL.md`; a glob that matches nothing must not read as green.
  Assert too that at least one occurrence was actually classified, so a broken reader that finds
  zero occurrences does not pass.
- **Be mutation-tested in-suite, non-vacuously.** Against a tmpdir copy, never the real tree:
  (a) a fixture line carrying the bad clause ⇒ the classifier **rejects**;
  (b) a fixture line carrying a *marked* occurrence of each admissible class ⇒ **passes**;
  (c) the same occurrence with its marker stripped ⇒ **rejects**.
  A "corrected clause" fixture is explicitly NOT the positive test — the corrected clause contains
  no `.docket.yml` token, so it is not an occurrence and would pass while proving nothing.
- **Key on shape, not spelling** — the reject rule is "an unclassified `.docket.yml` occurrence".
  The admissible half is closed and marked in-line; nothing depends on how a sentence is phrased.

## ADR-0052

ADR-0052 is Accepted and its `## Enforcement` section names `tests/test_docket_example_yml.sh` as
the enforcer. This change adds a second one, so append a dated `## Update` note to ADR-0052 naming
`tests/test_config_read_channel.sh` and what it covers — a non-reversing context addition, the
sanctioned shape (no new ADR, no edit to the Decision). Set the change's `adrs: [52]` so the update
is delivered atomically with this change (learning: `adr-update-delivery`).

## Out of scope

- Changing what `integration_branch` means, how it resolves, or its ADR-0019 coordination fence.
- Extending the guard to the README, `.docket.example.yml`, or script contracts — those are
  documentation *about* the config file, and `tests/test_docket_example_yml.sh` already owns the
  example's honesty.
- Tightening ADR-0052's known-residual `elsewhere:` mention-check (that is change #0121).

## Verification

- The corrected clause is present and the false one is gone; the three legitimate sites carry their
  class markers.
- `bash tests/test_config_read_channel.sh` passes; all three mutation fixtures behave as specified.
- `bash tests/test_skill_size_budgets.sh` passes on **both** dimensions it pins for
  `docket-finalize-change/SKILL.md` — words (4131/4200 at grooming) and lines (189/193). The
  clause replacement is same-line, but the **four** marker comments each add a line, in
  `docket-finalize-change/SKILL.md` (189→190), `docket-status/SKILL.md` (107→109), and
  `github-board-mirror.md` (17→18) — all budgeted files. The mirror file has only **2 lines of
  headroom** (19), so confirm rather than assume.
- `bash tests/test_adr_checks.sh` (ADR-0052 update) and the whole suite at the build gate.

## Assumptions

Every decision below was defaulted autonomously. This block is the deferred audit trail. Items
2, 4, 6, and 9–12 were revised or added after the adversarial critic round.

1. **Fix scope: the one clause, not a rewrite of the paragraph.**
   Chosen: replace only the parenthetical. Rejected: rewriting step 1 to restate the whole export
   contract (duplicates the merge-gate paragraph ten lines down and spends scarce word budget);
   rejected: deleting the parenthetical entirely (loses the "not hard-coded `main`" warning, which
   is the reason it exists). Conservative because it is the minimal true edit.

2. **The sibling audit found no second false claim, so the change carries no second prose fix.**
   Counts re-verified after the critic caught arithmetic errors in the first draft: 16 occurrences
   across 5 files, distributed as the §2 table shows. Rejected: a broad sweep across
   README/scripts prose (outside the stub's scope; the example-yml suite already guards the
   documented-key surface). Risk if wrong: a missed site — mitigated by the sentinel, which fails
   on any unmarked site the grep missed the moment it runs.

3. **Build the sentinel rather than deferring it.** The stub said "consider". Chosen: build it —
   this is occurrence #2 of a class ADR-0052 already declared a rule, and a stated rule with no
   enforcement is what let occurrence #2 exist. Rejected: fix-only (leaves the class open);
   rejected: a general prose linter over all docs (false-positive farm, unbounded scope).

4. **Sentinel form: classify-every-occurrence via an explicit in-line class marker.**
   Chosen: an admissible-class allowlist over the full occurrence population, with the class
   **declared at the site** rather than inferred from wording. Rejected: a grep for "resolved from
   `.docket.yml`" (an enumerated spelling AGENTS.md forbids); rejected: the first draft's
   "admitted when the line names the written key" heuristic — the critic proved it reddens on
   `docket-status/SKILL.md`'s legitimate second occurrence, which names no key.

5. **Sentinel home: a new `tests/test_config_read_channel.sh`.**
   Chosen: new file. Rejected: extending `tests/test_docket_example_yml.sh` (1345 lines; its
   subject is the example config's fidelity, not skill prose); rejected: folding into
   `test_skill_facade_wiring.sh` (different contract).

6. **Population is auto-discovered minus a declared exclusion list; the two excluded
   `docket-convention` files are examined rather than waved through.**
   Chosen: `find skills -name '*.md'` minus two named files, following the live precedent in
   `test_skill_size_budgets.sh`. Rejected: the first draft's hand-listed glob set, which also left
   `skills/*/*-template.md` permanently exempt. The exclusion of `docket-convention/SKILL.md` and
   `references/agent-layer.md` rests on §4's reading of the one line that carries the guarded
   shape, and §4 makes the implementer re-confirm it at reconcile with a stated fallback edit.

7. **No new ADR.** ADR-0052 already states the rule; this change enforces it.

8. **Dependency state:** `depends_on` is empty and nothing here is gated. `related` is empty;
   `discovered_from: [102]` is informational. #0121 (the `elsewhere:` mention-check) is adjacent
   but independent and explicitly out of scope.

9. **ADR-0052 gets a dated `## Update`, not silence.** The critic flagged that its `## Enforcement`
   section would otherwise name one of two enforcers. Chosen: the convention's sanctioned
   append-an-Update path, delivered atomically via `adrs: [52]`. Rejected: leaving it stale
   (a known-wrong ledger entry); rejected: editing the Decision or Enforcement text (an Accepted
   ADR is immutable except its status line).

10. **The positive mutation fixture is a marked occurrence, not a corrected clause.** The corrected
    clause has no `.docket.yml` token, so it exercises nothing. Chosen: mark/strip-the-marker as
    the passing/failing pair. Rejected: the first draft's vacuous positive.

11. **Both size-budget dimensions are verified, not just words.** `test_skill_size_budgets.sh` pins
    lines as well; the four class markers add a line each across three budgeted files, the tightest
    being `github-board-mirror.md` at 2 lines of headroom. Chosen: name both dimensions and every
    affected file in Verification. Rejected: the first draft's word-only, three-marker check.

12. **`type: docs` stands.** The deliverable is a prose correction plus the test that keeps the
    prose honest — an enforcement guard for a documentation rule, which reads as `docs` on its own
    terms. (No sibling precedent is claimed: change 0102 predates the `type:` taxonomy and carries
    no `type:` field.) Rejected: reclassifying to `chore`/`fix` (metadata churn with no downstream
    effect here;
    `auto_capture` typing governs minted stubs, not this change's own build). Noted explicitly
    because the change does ship an executable file, so the classification is a real judgment
    rather than an oversight.
