<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0226 — Reframe auto-capture as capability discovery with strict admission gates](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0226-reframe-auto-capture-as-capability-discovery-with-strict-adm.md)**
<!-- docket:backlink:end -->

# Reframe auto-capture as capability discovery with strict admission gates — design

## Problem

Auto-capture reads as a suppression mechanism. Change 0218 made that correct but one-sided: it
established that work the current run will fix in-branch fails the materiality bar, and the
reference's prose now leads with reasons *not* to mint. Nothing in the convention directs an agent
to actively look for the thing auto-capture exists for — genuinely new capability discovered while
doing something else.

The suppression is load-bearing and stays. What is missing is the positive half: an instruction to
search for independently valuable work, and an explicit set of gates that work must clear.

## Design

### 1. The reference is reframed as a discovery pipeline

`skills/docket-convention/references/auto-capture.md` leads with intent — auto-capture discovers
independently valuable capability — then gates hard.

**Discovery categories** the agent is told to look for: reusable capabilities; new product or
workflow features; missing policy or lifecycle behavior; tooling opportunities; architectural gaps
or boundaries; improvements with value beyond the active change.

**Admission gates** — capture only when the discovery:

1. falls outside the scope of the active change;
2. has independently valuable outcomes;
3. is more than a defect or review finding in the current implementation;
4. has a clear, defensible boundary;
5. is concrete enough to describe as a separate change;
6. cannot reasonably be completed on the active branch without expanding its intended scope.

**Suppression rules are preserved verbatim in effect.** Never mint for: review findings about the
active diff; bugs or regressions introduced by the active change; work `implement-next` is expected
to fix in the current branch; minor cleanup or refactoring without independent value; documentation
needed to complete the active change; vague ideas without a clear outcome or boundary.

### 2. Capture requirements fit under `## Why`

A captured discovery explains **Trigger**, **Opportunity**, **Independent value**, **Boundary**, and
**Reason for deferral**.

`scripts/mint-stub.sh` hard-rejects any `--body-file` whose contents do not **start with `## Why`**
(validated before any write; exit 1). The five fields are therefore specified as labelled lines or a
sub-block **under** a leading `## Why` heading — never as five top-level `##` sections.
`scripts/mint-stub.md` and `mint-stub.sh` are not modified.

### 3. Routing is site-dependent, not uniform

Four routes: **fix-in-branch**, **record-as-learning**, **report-only**, **capture-as-new-change**.

Fix-in-branch exists when and only when the site has an open branch and a live fix loop:

| Site | Branch + fix loop | Routing |
|---|---|---|
| A — `docket-implement-next` reconcile | yes | all four; discoveries are usually drift → reconcile log |
| B — `docket-implement-next` review | yes | fix loop is the **default** consumer (`REVIEW_MIN_FIX_SEVERITY` gates entry, blockers regardless); capture is the narrow exception |
| C — `docket-finalize-change` / `docket-status` harvest | **no** | fix-in-branch **unavailable**; the other three are the whole space |

The `--minted` count carries forward A → B: one budget of 3 across the whole run on a single change,
never reset per site. (`docket-status`'s sweep scopes it per swept change.)

**Site C keeps 0218's exemption.** The existing text already carves it out — no open branch, no fix
loop, so no run there fixes anything in-branch. At Site C the admission bar stays the existing
*would a human file this as a `docket-new-change` / own change-or-PR* test, **not** the stricter
capability-discovery gates. Failure mode if this is flattened: cheap-to-fix work found at harvest —
which the current text calls out as "exactly what nothing else picks up" — gets suppressed as "not
an independently valuable capability" and is lost. The reframe carries the exemption forward.

### 4. The convention summary is reframed under progressive disclosure

`skills/docket-convention/SKILL.md`'s `### Auto-capture (shared definition)` currently carries the
suppression-first framing, and it is what a mint site reads inline before deciding whether to drill
down. It is reframed to carry:

- the **intent sentence** — auto-capture is capability discovery gated by strict admission controls;
- the existing config/classification/policy-suppression mechanics;
- the existing mint-site enumeration;
- the existing **blocking drill-down pointer** to `references/auto-capture.md`.

It does **not** carry the six discovery categories, the six admission gates, the five capture
fields, or the routing table. Those live only in the reference.

## Constraints

**Size budgets** (`tests/test_skill_size_budgets.sh`):

- `references/auto-capture.md` measures **51 lines / 544 words** against its row `55 600`. The
  rewrite exceeds both. The row must be raised — target ≈ `100 1150` — **with** the dated
  justification comment block that file's convention requires for every raise (see its existing
  blocks near lines 198 and 354). Without this the suite goes red.
- `docket-convention/SKILL.md` measures **339 lines / 5804 words** against its row `345 5850` —
  6 lines and 46 words of headroom. The summary reframe is therefore an approximately byte-neutral
  **in-place rewrite**. Raise that row only if a genuinely minimal rewrite does not fit, and if
  raised, add the dated justification block.

**Guard block.** `tests/test_docket_review.sh` ~394–422 is change 0218's assertion block. It greps
**exact prose** from `auto-capture.md`: "work the current run will fix in-branch fails the bar", the
"no branch, no fix loop" exemption rationale, the *Materiality bar* section non-vacuity anchor, and
a `>= 20` line floor. Those assertions **must survive** — re-anchored to the new wording where prose
moves, never deleted or weakened. The line floor is raised to match the larger file.

**File collision.** Active change **0204** (Restore dropped doc rationale) also edits
`references/auto-capture.md` and the convention's auto-capture summary. Whichever lands second
rebases onto the first; neither is blocked on the other.

## Out of scope

Does not: relax suppression of review findings; change how `implement-next` fixes findings in the
active branch; change stub-minting mechanics or modify `scripts/mint-stub.sh` / `mint-stub.md`;
alter deterministic naming, numbering, or creation behavior; turn auto-capture into an
implementation or review loop; require capture of speculative or weakly defined ideas; change
`AUTO_CAPTURE_TYPES` filtering, its ordering before the cap, or the per-invocation cap of 3.

The capability-discovery framing biases discoveries toward `feat`, so a repo whose
`AUTO_CAPTURE_TYPES` excludes `feat` will see more policy-suppressed reports. Expected; not to be
"fixed" during the build.

## Acceptance criteria

- `auto-capture.md` leads with the intent to discover independently valuable capabilities.
- It explicitly instructs agents to search for new features, workflow improvements, policy gaps,
  tooling opportunities, and architectural capabilities.
- All existing suppression rules remain in effect.
- Findings about the active change continue to be fixed in the active branch, never captured.
- The five capture fields are specified as living under a leading `## Why`, satisfying
  `mint-stub.sh`'s body contract.
- The routing section states the four routes and marks fix-in-branch conditional on an open branch
  plus fix loop, with Site C's exemption and its own admission bar preserved.
- The convention summary is reframed to intent + mechanics + drill-down pointer, with no
  duplication of the reference's detail.
- Deterministic change-creation behavior is unchanged.
- `tests/test_skill_size_budgets.sh`: the `auto-capture.md` row is raised with a dated
  justification block; the `SKILL.md` row is raised only if needed, likewise justified.
- `tests/test_docket_review.sh`'s 0218 guard block still passes, re-anchored rather than removed,
  with its line floor raised.
- Tests cover **both** a discovery that qualifies as a new change and a current-branch finding that
  must not become one.
