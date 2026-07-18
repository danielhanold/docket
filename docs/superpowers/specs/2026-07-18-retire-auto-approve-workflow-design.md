# Retire the auto-approve workflow — design

**Change:** #0095 · **Status:** proposed · **Date:** 2026-07-18 · **Reverses:** ADR-0042 (change 0062)

## 1. Problem

Changes 0015 (rebase-retest gate), 0021 (consent model), and 0062 (auto-approve
workflow) shared one north star: run `docket-finalize-change` **in one swoop** — gate,
merge, close out — without a human and without being blocked. Change 0062's mechanism
for the *merge authorization* half was a repo-installed GitHub Actions workflow
(`docket-approve.yml`) that approves the PR with the built-in `GITHUB_TOKEN`, so branch
protection's required review is satisfied without `--admin` and the merge is "not without
review" (ADR-0042).

**In practice the workflow is a full failure.** Claude Code's auto-mode classifier
soft-denies the `gh workflow run docket-approve.yml` dispatch that the finalize gate must
issue — reproduced on the 2026-07-18 finalize of change 0088: the dispatch (and even a
post-merge `gh pr view`) were denied by the classifier in an attended auto-mode session.
The bot-approval chain can never complete when its very first step is blocked, so the
capability ADR-0042 was built to deliver does not exist on Claude Code as run. ADR-0042's
own go/no-go spike had observed *no* headless denial under CC 2.1.211 and explicitly
pinned that classifier behavior is version- and mode-scoped; the behavior we now observe
is the version-dependence that ADR foresaw, landing on the failure side.

**The solution was found empirically, and it is much simpler.** The maintainer changed
this repo's branch protection to **require a pull request but require zero approvals**
(`required_approving_review_count: 0`, `enforce_admins: false`). With that, a plain
`gh pr merge --rebase` satisfies branch protection with **no `--admin`, no bot, and no
classifier-blocked dispatch** — which is exactly why the 0088 finalize succeeded end to
end (the first finalize run-through that worked). For a single-maintainer repo — docket's
primary use case, where the maintainer structurally cannot approve their own PR — this is
strictly better than the bot workflow: fewer moving parts, no Actions dependency, no
`can_approve_pull_request_reviews` repo setting, nothing for the classifier to deny.

The bot-approval subsystem is now dead machinery declared a failure. This change retires
it, records the reversal as a new ADR, and documents both the classifier behavior and the
branch-protection solution in the repo README so the next maintainer does not re-derive
this the hard way.

## 2. Goals / non-goals

**Goals**
- Fully decommission change 0062's `auto_approve` subsystem (code, config, docs, tests).
- Author a new **Accepted** ADR that reverses ADR-0042 and records the branch-protection
  solution as the supported single-maintainer merge path.
- Document in `README.md`: (a) the Claude Code auto-mode classifier behavior — cleanly,
  as *why the bot approach failed*; (b) the single-maintainer hands-off recipe (branch
  protection = require PR, 0 required approvals).
- **Preserve** the human-approval merge path for approval-required repos: a real human
  (or co-maintainer) approval on GitHub satisfies both branch protection and
  `require_pr_approval: true`, and finalize merges without `--admin`.

**Non-goals**
- Touching `finalize.gate` (the rebase-retest **correctness** gate, change 0015) — stays
  intact.
- Touching `require_pr_approval` (the human-authorization **policy** gate on the
  auto-detect path, change 0021 / ADR-0011) — stays intact; see §5.
- Reversing or editing ADR-0011 (it stands; the new ADR only removes the
  `auto_approve`-specific relaxation ADR-0042 layered on top of it).
- Building a finalize driver / loop (that is #0087's territory) or the attended
  finalize-merge path work (#0086). This change only removes the failed mechanism and
  documents the working one.
- Changing the `--admin` attended/explicit-id escape hatch — it remains available.

## 3. What survives, and why (the reader's first question)

Two finalize knobs are **orthogonal** to the classifier/approval problem and are kept:

- **`finalize.gate`** (0015) — the **correctness** gate. Before merge, finalize rebases
  `feat/<slug>` onto `origin/<integration_branch>` and re-runs the suite
  (`local`/`ci`/`both`), merging only if green. Catches "green on the branch, broken once
  rebased onto latest main" — a semantic conflict a stale PR CI never sees. It is what ran
  clean on 0088 (rebase no-op + 50/50 suites). Nothing to do with approvals.
- **`require_pr_approval`** (0021 / ADR-0011) — the **human-authorization policy** gate,
  governing only the auto-detect path. `true` ⇒ auto-detect finalize refuses to merge a PR
  that is not `reviewDecision: APPROVED`, surfacing it. An explicit id always overrides it.
  It exists for **team** repos that want a real human bar; default `false` for the
  single-maintainer case.

Removing `auto_approve` in fact *cleans up* `require_pr_approval`: ADR-0042 point 1 had
made `require_pr_approval: true` **bot-satisfiable** under `auto_approve`. With
`auto_approve` gone, `require_pr_approval: true` reverts to its pure ADR-0011 meaning —
"a human authorized the merge," no longer satisfiable by a bot.

## 4. Decommission scope (the removal work-list)

Everything below is change 0062's footprint; each item is removed unless marked *prune*.

**Workflow + template**
- `.github/workflows/docket-approve.yml` — delete (the installed workflow).
- `scripts/templates/docket-approve.yml` — delete (the static template).

**Setup script + facade + its docs**
- `scripts/setup-auto-approve.sh` and `scripts/setup-auto-approve.md` — delete.
- `docs/auto-approve-setup.md` — delete (its salvageable branch-protection guidance moves
  into the README, §6).
- `scripts/docket.sh` + `scripts/docket.md` — remove the `setup-auto-approve` facade op
  (dispatch arm + wrapped-ops inventory + contract row).

**Config knob + resolver**
- `.docket.yml` — remove the `finalize.auto_approve` key (and its comment block).
- `scripts/docket-config.sh` + `scripts/docket-config.md` — remove the
  `FINALIZE_AUTO_APPROVE` export, its parse, its coordination-key-fence classification
  row, and any default. `finalize.gate` / `finalize.test_command` parsing is untouched.

**Skill prose**
- `skills/docket-finalize-change/SKILL.md` — remove the gate's **step 6 (Approve)**
  entirely; simplify **step 7 (merge)** to: `gh pr merge` without `--admin` on the
  human-approved / 0-required-approval paths, `--admin` retained only on the attended /
  explicit-id path where a required review is otherwise unsatisfiable. Remove the
  `auto_approve` row from the `finalize:` YAML block, the auto_approve abort-and-report
  bullets, and the ADR-0042 cross-reference; keep the `require_pr_approval` and gate prose.
  Update the change-body/`.docket.yml` example if it lists `auto_approve`.
- `docket-convention` — confirm no residual `auto_approve` mention (the convention's
  `.docket.yml` example does not carry it today; verify and scrub if present).

**ADR ledger**
- New Accepted ADR (§5) with `reverses: [42]`; flip `ADR-0042.status` →
  `Reversed by ADR-00NN` (the one mutable line); re-render the ADR index. ADR-0042's body
  is immutable and stays as the historical record.

**Tests**
- Delete `tests/test_auto_approve_docs.sh`, `tests/test_docket_approve_template.sh`,
  `tests/test_setup_auto_approve.sh`.
- *Prune* the `auto_approve` assertions from `tests/test_finalize_gate.sh`,
  `tests/test_docket_config.sh`, `tests/test_docket_facade.sh` — keep every assertion
  guarding `finalize.gate`, `require_pr_approval`, and the surviving facade ops
  (mutation-test the pruned files still redden on a real regression of what remains).
- Add a **README doc-sentinel** (new `tests/test_readme_finalize_docs.sh` or fold into an
  existing docs test): assert the README documents the classifier behavior and the
  branch-protection recipe, keyed on the load-bearing phrases, non-vacuously.

## 5. The new ADR (reverses ADR-0042)

- **Context:** the `docket-approve.yml` dispatch is classifier-blocked in practice
  (2026-07-18, change 0088 finalize); the bot-approval chain cannot complete. Branch
  protection requiring a PR with **0 required approvals** is the simpler working path.
- **Decision:** retire the bot-approval mechanism (change 0062) entirely. The
  single-maintainer hands-off merge path is branch-protection configuration
  (`required_approving_review_count: 0`, `enforce_admins: false`) → `gh pr merge --rebase`
  with no `--admin`, no bot, no dispatch. Removing `auto_approve` restores
  `require_pr_approval: true` to its ADR-0011 "a human authorized the merge" meaning.
- **Consequences:** `enables` a genuinely working one-swoop finalize for the
  single-maintainer default with far less machinery; `costs` losing the "required-review is
  satisfied *and* recorded as an approval" property (a 0-required-approvals repo merges
  with no recorded review) — acceptable and explicit for a solo maintainer. Team /
  approval-required repos keep `require_pr_approval: true` with a **real human** reviewer
  (§ below). Classifier behavior is version/mode-scoped, so this is an empirical decision,
  not a permanent contract. Sets `ADR-0042.status: Reversed by ADR-00NN`.

## 6. README documentation (the two deliverables)

Add a focused finalize/merge section to `README.md`:

1. **Claude Code auto-mode classifier — what it blocks.** In interactive auto-mode the
   classifier soft-denies capability-granting / merge-adjacent `gh` actions — notably
   `gh workflow run` and `gh pr merge` on an unreviewed PR (and occasionally a post-merge
   `gh pr view`). `permissions.allow` cannot clear a soft-deny; behavior is version- and
   mode-scoped and differs headless. This is *why* the change-0062 bot-approval workflow
   failed — its first step (`gh workflow run`) is exactly what gets denied.
2. **Single-maintainer hands-off finalize (recipe).** Set branch protection on the
   integration branch to **require a pull request** but **require 0 approvals**
   (`required_approving_review_count: 0`; leave `enforce_admins` off). Then
   `docket-finalize-change` runs its rebase-retest gate and merges via plain
   `gh pr merge --rebase` — no `--admin`, no bot, nothing for the classifier to deny.
   (Salvaged from the deleted `docs/auto-approve-setup.md`.)
3. **Approval-required repos (human sign-off preserved).** For repos with
   `required_approving_review_count ≥ 1`: a human approves the PR on GitHub — a
   co-maintainer, or the human running finalize if they are a reviewer — so
   `reviewDecision: APPROVED` satisfies both branch protection and `require_pr_approval:
   true`, and finalize merges with **no `--admin`**. The attended explicit-id `--admin`
   path remains the escape hatch when a sole maintainer chooses to force past an
   unsatisfiable required review. The implementer must confirm the finalize Selection
   matrix's "approved ⇒ eligible" behavior and the gate's APPROVED-satisfies-merge prose
   survive the `auto_approve` removal unchanged.

## 7. Risks / edge cases

- **Vacuous test pruning.** Removing `auto_approve` assertions could silently gut a shared
  test file. The implementer mutation-tests each pruned file: strip a surviving feature,
  confirm it still reddens (`guards-are-code` discipline).
- **Dangling references.** `grep -rn` for `auto.?approve|docket-approve|FINALIZE_AUTO_APPROVE|
  setup-auto-approve` across the repo after removal must return only historical
  archive/ADR-0042/spec-and-results provenance — no live script, skill, config, or test
  path. `docs/adrs/README.md` re-renders to reflect the reversed status.
- **Reversed ADR immutability.** Only ADR-0042's `status:` line changes; its body is
  untouched (the historical record of why the bot approach was tried).
- **Self-referential.** This change edits `docket-finalize-change/SKILL.md`, the skill
  used to close it out. Safe: the edit lands on the feature branch; the installed copy
  driving the eventual finalize is untouched mid-run.
- **Size budgets.** Trimming finalize SKILL.md step 6 *reduces* its size; the README grows.
  Adjust any `tests/test_skill_size_budgets.sh` rows in the same diff if a bound is crossed
  (the guard permits in-diff raises/lowers).

## 8. Acceptance

- No live (non-archive/non-ADR-0042) reference to the `auto_approve` subsystem remains.
- New Accepted ADR present with `reverses: [42]`; ADR-0042 marked `Reversed by ADR-00NN`;
  ADR index re-rendered.
- README documents the classifier behavior, the single-maintainer recipe, and the
  approval-required human path; the doc-sentinel test guards it non-vacuously.
- `finalize.gate` and `require_pr_approval` behavior is unchanged; full `tests/test_*.sh`
  suite green.
