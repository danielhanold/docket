# Terminal-publish gap — mark the deferral, stop the checker lying — design

**Change:** #0083 · **Status:** proposed · **Date:** 2026-07-18 · **Related:** #0033, #0043, #0064, #0095 · **Cites:** ADR-0001

## 1. Problem

Change #0043's terminal record — its archived change file and `spec:` — was archived on
`docket` on 2026-07-08 but never copied onto `main`. Nothing noticed for eight days; it
surfaced only incidentally while killing #0033, when an ad-hoc diff of the archive sets
across branches showed `main` at 69 records and `docket` at 71. No docket health check
caught it.

**Part 1 (root-cause #0043) is answered — decisively, and it overturns this stub's own
premise.** The abstain investigation (preserved in git history; the change's
`## Auto-groom blocked` section as of commit before this groom) pulled the original
2026-07-08 kill session and established:

- `terminal-publish.sh` was **never executed** in that session (0 invocations).
- The agent wrote a 4-step plan whose step 4 was *"terminal-publish 43's kill to `main` …
  I'll pause and confirm before that push."* It ran steps 1–3 (`archive-change.sh` at
  01:37:44Z), then surfaced the publish as *"one deferred step (your call) … since 43 was
  a never-shipped proposal, I'd leave `main` clean … say the word if you'd rather publish
  it."* The maintainer later replied *"43 is already published to main"* (which was not
  true); the agent re-checked, confirmed it absent, and **re-asked**. The thread moved on
  unanswered.

So #0043 was **not** a silent mechanism failure. It was a **correctly planned,
consciously deferred, human-gated close-out step that was recommended against and asked
about twice, and never answered.** The stub's three candidate causes — (a) classifier
denial, (b) a hand-driven skipped step, (c) a kill-path bug — are all **falsified**: the
publish was never *attempted*, and the publish step had been in the proposed-kill path
since `9d38434` (2026-06-19), well before #0043's kill. A fourth candidate (stale local
skill checkout) is falsified too (local `main` fast-forwarded 12 minutes before the kill).

That reframes the whole change. The decision in *What changes* part 2 — detector /
healer / both / **neither** — is no longer a technical default an agent can pick: it is a
call about how the maintainer's own unanswered approval should be treated, which is why
autonomous grooming abstained. **The maintainer's groom decision (2026-07-18): mark the
deferral, do not build a branch-diff detector; and fix the checker that certified the gap
clean.** This spec designs to exactly that.

### 1a. Why not a detector/healer — the policy-wall reading

Part 2 tempts a standing detector (diff the archived set on `metadata_branch` against the
published set on `integration_branch`) and/or a healer (re-publish what is missing). Two
reasons this spec deliberately declines that shape:

- **The realized gap was a conscious human deferral, not a fault to heal.** A healer that
  auto-republishes would have *reversed a choice the agent recommended and the maintainer
  never overrode.* (The 2026-07-16 backfill `c0d6c04` that "repaired" #0043 may itself
  have reversed that deliberate choice — see §5.)
- **It smells like the second workaround for a wall the maintainer owns.** The learning
  `relax-the-policy-before-building-the-workaround` (landed 2026-07-18 from #0095) records
  docket spending *three* changes (0015/0021/0062) building machinery to route around
  `main`'s branch protection, when the fix was one console setting. The direct
  terminal-publish push to a protected `main` sits behind the same class of wall. Building
  a detector/healer to survive a wall the maintainer controls is that anti-pattern again.
  The honest, minimal response is to make the deferral **visible** and stop the checker
  from **lying** — not to automate around the wall.

## 2. Goals / non-goals

**Goals**
- **A durable marker.** When a terminal close-out's publish step is consciously deferred
  or blocked (expected but not completed), leave a self-describing marker *at the change
  file*, so the gap is visible where a human reads it — not derivable only by diffing two
  branches later.
- **Stop `board-checks.sh` certifying a pending deferral as clean.** Make the mechanical
  checker surface the marker as a finding, so a deferred publish can never again ride out
  a "done/clean" report unseen.
- **Honor `terminal_publish` (change 0064).** A marker means *expected-but-not-done*,
  never *policy-suppressed*: under `terminal_publish: false`, and in `main`-mode, no
  marker is ever written and no finding ever fires.
- **Presence-encoded state is honored.** When the deferred publish later completes, the
  marker is removed in the same act — no stale marker survives the state it encodes.

**Non-goals**
- **No standing branch-diff detector / audit** over the full archive set, and **no
  healer** that re-publishes missing records. Explicitly declined per §1a and the groom
  decision.
- **No change to the classifier, branch protection, or the `--admin` policy** on
  `danielhanold/docket` — the wall is out of scope (stub *Out of scope*).
- **No redesign of the `terminal_publish` knob** — consulted as a constraint only.
- **No re-litigation of the 2026-07-16 backfill** as code — it is noted (§5) as a
  reframing, not reversed here.

## 3. Design

### 3.1 The marker — `## Publish deferred`

A dated body section appended to the (archived) change file, structurally mirroring the
existing `## Auto-groom blocked` marker so readers and the board already have the pattern:

```
## Publish deferred

### 2026-07-08 — terminal-publish to `main` not completed

Close-out steps 1–2 (archive, artifacts re-render) landed on `docket`; the terminal-publish
step (copying the archived change file + `spec:` + Accepted ADRs onto `main`) did **not**
run — <reason: deferred pending human approval | blocked: direct push to protected `main`>.
The record is on `docket` only.

**Re-arm:** complete the publish (`docket.sh terminal-publish …`, or a human-approved
backfill). A successful publish removes this section automatically.
```

- **Name.** `## Publish deferred` — parallel to `## Auto-groom blocked`, and distinct from
  the lifecycle `## Why deferred` (which records the `deferred` *status*). *(Final section
  name is a one-line build-time call; `## Terminal-publish blocked` is the alternative if
  "deferred" reads as too close to the status. Recommend `## Publish deferred`.)*
- It is **generated, not hand-authored** — appended and removed by a script (ADR-0012
  script-vs-model boundary), exactly as `## Reclaim log` is owned by `reclaim-claims.sh`.
- The change body's section list in `docket-convention` gains an entry describing it and
  its lifecycle (added/removed), alongside `## Auto-groom blocked`.

### 3.2 When it is written / removed — the `terminal-publish.sh` seam

The deferral in #0043 happened *above* the script (the agent never invoked it), so the
close-out **sequence** owns the "did the publish complete?" decision, and a deterministic
writer owns the file edit. Design:

- **Write on defer/block.** In the terminal close-out sequence (step 3, `references/terminal-close-out.md`),
  when the publish is *expected* (`TERMINAL_PUBLISH=true`, docket-mode) but does **not**
  complete — an autonomous run hard-stops at a protected-branch wall, or an interactive
  run defers pending approval — the driver appends the marker before reporting. Autonomous
  callers still abort-and-report; the marker makes that abort **durable and self-describing**
  instead of living only in a chat thread.
- **Remove on completion.** `terminal-publish.sh`, on a **successful** publish, removes any
  existing `## Publish deferred` section from the archived change file in the same commit
  that publishes — the presence-encoded-state guarantee (`presence-encoded-state` learning:
  every transition *out* of the encoded state removes the artifact). A backfill is just a
  later successful publish, so it self-heals the marker for free.
- **Never write under suppression.** When `TERMINAL_PUBLISH=false` or in `main`-mode,
  `terminal-publish.sh` is already a no-op that exits 0; the marker is **not** written —
  a suppressed publish is legitimate success, not a deferral.

**Writer boundary (build-time call).** Two viable shapes; recommend the first:
1. A small dedicated writer (`mark-publish-deferred.sh` add/remove, or a new
   `terminal-publish.sh --mark-deferred` mode) invoked by the close-out drivers on the
   defer path; `terminal-publish.sh`'s success path calls the remove. Keeps the file edit
   deterministic and testable in one place.
2. Fold both add and remove into `terminal-publish.sh` (a `--deferred` invocation writes
   the marker and exits; the normal invocation publishes and clears it). Fewer scripts,
   but overloads the publish script with a "don't publish, just mark" mode.

Either way: the writer is deterministic and git-only; the model never hand-writes the
section.

### 3.3 `board-checks.sh` — surface the marker, don't diff branches

`board-checks.sh` "certified the gap clean" on 2026-07-08 because it has **no** check for
terminal-record state at all — it checks `broken-plan-results` (a `done` change's
`plan:`/`results:` on the integration branch) but nothing about whether the archived
*record* reached the integration branch. The fix that is consistent with "mark, don't
detect" and preserves the script's **git-only / offline** invariant:

- **New check `publish-deferred`.** Walking `archive/` (and `active/`, harmlessly), a
  change carrying a `## Publish deferred` section emits one finding:
  `publish-deferred \t <id> \t terminal-publish to <integration-branch> not completed — record on docket only`.
  This reads the marker in the change file (offline, no `origin/main` diff), so the checker
  can no longer report clean while a deferral is pending. `docket-status` surfaces it in
  its health-check display like any other finding.
- **Deliberately NOT** a `git cat-file -e origin/<integration>:<archived-path>` set-diff.
  That would (i) reintroduce the declined detector, (ii) fire forever under
  `terminal_publish: false`, and (iii) largely duplicate the marker. Keying on the marker
  gets the no-regret correctness win — a pending deferral is never certified clean — at a
  fraction of the surface.

**Residual, stated honestly.** A terminal record that goes missing via a path that writes
*no* marker (e.g. a hard crash between archive and publish, no deferral) is still not
caught by `board-checks.sh`. That is the accepted cost of "mark, don't detect": the marker
is written at the one seam where deferrals actually occur, and a general
"every-terminal-record-must-be-on-main" audit is the declined detector. If the maintainer
later wants belt-and-suspenders coverage, it is a *separate* change — noted, not built here.

## 4. Components / touch points

- `scripts/terminal-publish.sh` (+ `.md` contract) — remove-marker-on-success; possibly
  the deferral-mark mode (per §3.2 shape choice).
- (If shape 1) `scripts/mark-publish-deferred.sh` (+ `.md`) — the dedicated add/remove
  writer, and a `docket.sh` facade verb.
- `scripts/board-checks.sh` (+ `.md` contract) — the `publish-deferred` check; add it to
  the check enumeration and the sort ordering. `docket-status`'s display side surfaces it.
- `skills/docket-convention/references/terminal-close-out.md` — step 3 gains the
  write-marker-on-defer rule (and the "never under suppression" invariant).
- `skills/docket-convention/SKILL.md` — the *Change body sections* list gains
  `## Publish deferred` (added on a deferred/blocked publish; removed on completion).
- Tests: (a) a deferred publish writes the marker and `board-checks.sh` emits
  `publish-deferred`; (b) a subsequent successful publish removes it and the finding
  clears; (c) under `terminal_publish: false` and in `main`-mode, no marker and no finding;
  (d) the marker survives a re-run idempotently (writing twice does not duplicate the
  section).

## 5. Notes carried from the investigation (no code)

- **The 2026-07-16 backfill (`c0d6c04`) may have reversed a deliberate choice.** On this
  evidence there was no failure to repair — the absence of #0043's record on `main` was the
  outcome the agent *recommended* for a never-shipped proposal and the maintainer never
  overrode. This reframes the stub's *Why* ("symptom repaired, cause unknown"): the symptom
  may not have been a symptom. Left as-is (reverting a landed backfill is not worth it); the
  marker mechanism this change adds is what would have made the deferral legible in the
  first place.
- **Gap census (2026-07-17, re-verified in the abstain):** archived change files 74/74
  across `docket`/`main`; 0 spec gaps across 67 specs; the only Accepted ADR absent from
  `main` is 0023 (`change: 44`, still `blocked`) — legitimately pending its close-out, not
  a gap. #0043 was the only realized gap.
- **`docket-adr`'s publish path** sits behind the same protected-`main` wall. It is *not*
  wired into the marker here (its records are ADRs, not change files, and it has no archive
  seam); if the maintainer wants deferred-ADR-publish visibility too, that is a follow-on.
  Called out so the omission is deliberate, not forgotten.

## 6. Open questions (for the build's reconcile pass)

- Final marker section name: `## Publish deferred` (recommended) vs `## Terminal-publish blocked`.
- Writer boundary: dedicated `mark-publish-deferred.sh` (recommended) vs a mode on
  `terminal-publish.sh`.
- Whether the marker's reason line should be a fixed enum (`deferred` | `blocked`) or free
  text — recommend a short fixed prefix plus free text, matching `## Auto-groom blocked`.
