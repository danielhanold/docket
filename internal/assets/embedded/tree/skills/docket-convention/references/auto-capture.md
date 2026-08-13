# auto-capture — the full shared definition

Auto-capture exists to **discover independently valuable capability** — work worth its own change
that surfaces while you are doing something else — and to file it before it is forgotten. The gates
below are what keep that from becoming stub churn: the active change's own work never becomes a
stub. The mechanics behind the convention's *Auto-capture (shared definition)* summary — read
before minting or suppressing a discovered stub. Loaded on demand from `docket-convention/SKILL.md`;
sibling files are not auto-loaded with the skill.

## What to look for

Actively look for work that is worth its own change — this pass is the only one that will see it:

- **reusable capabilities** — a mechanism this repo would use again if it existed;
- **new product or workflow features** — behavior a user or an operating skill would ask for;
- **missing policy or lifecycle behavior** — a state, transition, or gate that nothing owns;
- **tooling opportunities** — a deterministic script that would replace repeated model judgment;
- **architectural gaps** — a boundary asserted in prose and owned by no code;
- **improvements whose value outlives the active change** — worth doing even with this change
  reverted.

Finding it is the point of the pass; admitting it is gated.

## Admission gates

Capture only when the discovery clears **all six**. It must:

1. fall **outside the scope** of the active change;
2. have **independently valuable** outcomes — they stand up with the active change reverted;
3. be **more than a defect** or review finding in the current implementation;
4. have a clear, defensible **boundary** — you can say where the work stops;
5. be concrete enough to describe as a **separate change** — a title, a why, an outcome;
6. be work that cannot reasonably be completed on the active branch **without expanding** that
   branch's intended scope.

**These six bind where a branch and a fix loop exist** — sites A and B. The `docket-finalize-change`
/ `docket-status` harvest is exempt: with neither, it keeps the *Materiality bar*'s own *would a
human file this as its own change / PR* test instead — see *Routing*.

**Never mint** for: a review finding about the active diff; a bug or regression the active change
introduced; work `docket-implement-next` is expected to fix in the current branch; minor cleanup or
refactoring with no independent value; documentation needed to complete the active change; a vague
idea with no clear outcome or boundary.

## Materiality bar

Mint only for *actionable follow-up work that would be its own change / PR*
("would a human file this as a `docket-new-change`?"). A build lesson → the **learnings** harvest;
drift inside the current change → the **reconcile log**; a bare observation → the run report.

**Work the current run will fix in-branch fails the bar** (change 0218). A review finding about the
diff currently on the branch is **never mintable** — it is fixed in-branch or recorded in the PR
body, per `docket-implement-next`'s fix loop. A stub costs a title, an id, a groom, a spec, a plan,
a branch, a PR, and a close-out; a dead line of code costs one deletion, and routing the second
through the machinery built for the first is what made the backlog self-generating. Minting from
review survives only for genuinely distinct, beyond-the-branch work that clears the bar on its own.

**That clause binds only where a branch and a fix loop exist** — `docket-implement-next`'s reconcile
and review mint sites. **The `docket-finalize-change` / `docket-status` harvest is exempt**: it runs
with no open branch and no fix loop, so no run there fixes anything in-branch. Cheap-to-fix work
found at harvest is exactly what nothing else picks up — judge it on the *own change / PR* test
above.

## Routing

Four routes for a discovery: **fix-in-branch**, **record-as-learning**, **report-only**, and
**capture-as-new-change**. Fix-in-branch exists only where the site has an open branch AND a live
fix loop, so the available space differs per site:

| Site | Branch + fix loop | Routing |
|---|---|---|
| A — `docket-implement-next` reconcile | yes | all four; a discovery here is usually drift → the **reconcile log** |
| B — `docket-implement-next` review | yes | the **fix loop is the default** consumer (`REVIEW_MIN_FIX_SEVERITY` gates entry; blockers regardless); capture is the narrow exception |
| C — `docket-finalize-change` / `docket-status` harvest | **no** | fix-in-branch **unavailable**; the other three are the whole space |

**Site C keeps its own admission bar** — the *would a human file this as its own change / PR* test
of the *Materiality bar* above, not the six gates. With no branch and no fix loop, applying the
stricter capability-discovery gates there would suppress the cheap-to-fix follow-up that nothing
else picks up.

## What a captured discovery says

`mint-stub.sh` rejects any `--body-file` whose contents do not **start with `## Why`** (validated
before any write; exit 1). The five required fields are therefore labelled lines *under* that one
leading heading — never five top-level sections:

```markdown
## Why

**Trigger** — what surfaced this, and while doing what.
**Opportunity** — the capability that does not exist today.
**Independent value** — what it is worth with the active change reverted.
**Boundary** — where the work stops, and what it deliberately leaves alone.
**Reason for deferral** — why it cannot ride the active branch without expanding its scope.
```

## Per discovery

**Per discovery** (after whichever bar the site applies — the six gates at A and B, the materiality
bar at the harvest): assign exactly one type from `CHANGE_TYPES` — the model classifies, the script
never infers (ADR-0012). `AUTO_CAPTURE_ENABLED: false` ⇒ report, mint nothing. Enabled but the type
is outside `AUTO_CAPTURE_TYPES` (the literal `all`, or a subset) ⇒ mint nothing, report it as
**policy-suppressed**. Enabled and admitted ⇒ `mint-stub --type`. Every outcome keeps ADR-0045's
best-effort posture. **Type filtering runs before the cap is consumed** — a suppressed candidate
must never spend a mint slot; dedup stays after admission.

## The deterministic mint

**The mint itself is deterministic** (ADR-0012 — the model judges *what*, the script does the mint):
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh mint-stub --changes-dir .docket/<changes_dir>
--title <title> --type <type> --body-file <file> --discovered-from <this change's id> --minted <n so far>` (in
`docket`-mode; in `main`-mode, `--changes-dir <changes_dir>` — the metadata worktree IS the primary
tree) — one stub per call, `--body-file` **must start with `## Why`**, contract in
`scripts/mint-stub.md`. **`<n so far>` is the running count across the whole run on a single
change, never reset per mint site** — a skill with two mint sites (`docket-implement-next`'s
reconcile and review) carries the total forward. (`docket-status`'s sweep scopes it per swept
change — see its SKILL.md.) It owns dedup, id allocation, the template write, and the CAS push;
**exit 3** = duplicate skipped, **exit 4** = cap (3) reached, **exit 1** = a real error (push
failure, malformed body, retry exhaustion). Every skip, overflow, and exit-1 failure is **surfaced
in the run report, never silently dropped** — but none is fatal: **auto-capture is best-effort and
must never abort the change being built**, because capture is a courtesy while the change is the
job. Minting is a metadata-worktree write only — it never touches the running change's own
claim/branch/PR state.
