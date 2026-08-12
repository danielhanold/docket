# board-checks.sh — mechanical docket-status health checks

## Purpose

Performs the deterministic git-only health checks over the change files
(`active/` and `archive/`) and cross-references integration-branch commit subjects against them
and emits one TAB-separated finding per line on stdout.
It is the sole mechanical checker; the caller (`docket-status`) surfaces the findings
and owns human-facing display. The one judgment-bearing check — `blocked_by:` re-examination
— stays model-driven in the skill and is NOT performed here. Introduced in change 0023.

## Usage

```
board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
                 [--lease-ttl-hours N] [--adrs-dir DIR] [--terminal-publish]
                 [--results-dir REPO-RELATIVE-DIR]
```

| Flag | Required | Description |
|---|---|---|
| `--changes-dir DIR` | yes | Path to the directory that contains `active/` and `archive/` subdirectories. |
| `--metadata-branch BR` | yes | The branch (e.g. `docket` or `main`) against which spec paths are resolved via `git cat-file -e`. |
| `--integration-branch BR` | yes | The branch against which `plan:` / `results:` paths for `done` changes are resolved. |
| `--strict` | no | Exit 1 if any finding is emitted (a CI gate). Default: exit 0 regardless of findings. |
| `--lease-ttl-hours N` | no | Claim-lease TTL (hours) for the `stale-in-progress` check's `claimed_at:` signal. Default `72` when absent, so standalone use stays sane. |
| `--adrs-dir DIR` | no | Path to the flat ADR directory. Enables the `adr-unpublished` check (with `--terminal-publish`). A supplied path that does not exist is an error (exit 2), never a silent skip. |
| `--results-dir REPO-RELATIVE-DIR` | no | Repo-relative results directory scanned by `aborted-run`'s leg A. Defaults to `docs/results` (the convention's own default) so a hand-run stays sane. Unlike `--changes-dir` and `--adrs-dir` this is a **repo-relative** path, not a filesystem path: it is addressed through `<ref>:<path>` and `ls-tree --full-tree`, which are worktree-root-relative. |
| `--terminal-publish` | no | Opens the `adr-unpublished` gate. The caller passes it only when `terminal_publish: true` **and** docket-mode; absent, the check emits nothing. |

**Output format:** every finding is `<check-id>\t<change-id>\t<message>` on stdout, sorted
by `(check-id asc, change-id numeric asc)`. A clean tree produces no output.

**Mock seams:** `GIT="${GIT:-git}"` and `NOW="${NOW:-$(date +%s)}"` — override in tests
for hermetic staleness checks and git injection.

## Behavior

### Check enumeration

The script walks every `*.md` file under `active/` and `archive/` (sorted), sources
`lib/docket-frontmatter.sh`, and calls `resolve_deps` once to populate the dependency
state maps. Then it runs the following named checks:

**`broken-spec`** — The change has a non-empty `spec:` field, `trivial: false` is not set,
and the spec path is absent on `--metadata-branch` (checked via
`git cat-file -e <metadata-branch>:<path>`). Changes with `trivial: true` are exempt even
if they carry an unresolvable spec path (carve-out).

**`broken-plan-results`** — The change has `status: done` and at least one of its `plan:` or
`results:` paths is absent on `--integration-branch`. Carve-out: changes at `status:
implemented` are never flagged — their build artifacts still live on the unmerged feature
branch and are not yet on the integration branch.

**`stale-in-progress`** — The change has `status: in-progress`. Two independent signals feed
this check (change 0089); at most one finding is emitted per change:

- **Branch idle >3 days.** `branch:` is set and a `feat/<slug>` ref resolves (`refs/heads/<branch>`
  or `refs/remotes/origin/<branch>`), and its newest commit is older than 3 days (compared against
  `$NOW`). Message: `branch <branch> idle >3 days (last commit <N>d ago)` — unchanged from before
  0089.
- **Claim lease expired.** `claimed_at:` is set, parses via `iso_to_epoch`, and
  `NOW - claimed_at > --lease-ttl-hours * 3600`. This is the signal that catches the
  **crashed-before-branch** blind spot the branch-age signal misses (a claim can expire before any
  branch is ever pushed). Its message depends on whether a branch ref exists:
  - **No branch ref** (the reclaimable case): `claim lease expired <N>h ago; no feature branch —
    self-heal with docket.sh reclaim-claims [reclaimable]`. The trailing **`[reclaimable]`** token
    is a **stable, machine-readable suffix** — `docket-status` keys on its literal presence to
    decide whether to print a reclaim-sweep remedy. Do not reword or relocate it.
  - **Branch ref exists**: `claim lease expired <N>h ago; branch <branch> exists — needs your
    review (not auto-reclaimable)`. A live branch means a human should look before anything
    auto-reclaims, so this case never carries `[reclaimable]`.

Priority when both signals fire on the same change (branch exists, idle >3 days, AND the lease is
separately expired): the branch-idle message wins and is the only finding emitted — idle-branch
evidence is the older, more specific signal and is preserved unchanged.

**`merge-gate-stall`** — The change is build-ready (`status: proposed` with a spec or
`trivial: true`) and `resolve_deps` determined it is blocked because its worst-unmet
dependency is stuck at `implemented` (needs your merge). The finding message names the
blocking dependency ID.

**`stale-finalize-blocked`** — The change has `status: implemented` and carries the
`## Finalize blocked` body section (`finalize_blocked`), and that marker has outlived a fixed
staleness horizon (`FINALIZE_BLOCKED_STALE_SECS`, hardcoded 72 h). Marker age is the change file's
last-commit timestamp (`git log -1 --format=%ct -- <file>`) — the marker heading is deliberately
undated and its in-body date is model-authored, so git's clock is the tamper-proof signal. The
finding names the age in hours and advises re-running finalize with the id. Git-only and warn-only:
it cannot probe whether the underlying cause still holds (that needs `gh`/network, forbidden here),
so it fires on **any** marker past the horizon — a still-blocked marker that old is itself worth a
human glance. It never mutates the change file or auto-clears the marker; that stays
`docket-finalize-change`'s job. The horizon is a hardcoded constant (mirroring `stale-in-progress`'s
own `3*86400` branch-idle horizon), not a config knob.

**`publish-deferred`** — The change carries the `## Publish deferred` body section
(`publish_deferred`), written by `mark-publish-deferred.sh` when a terminal close-out's publish
step was **expected** (`terminal_publish: true`, docket-mode) but consciously deferred or blocked.
The finding names the integration branch the record never reached and the metadata branch it is
still confined to. **No status gate and no directory gate:** the marker is written on the
*archived* file, so gating on a lifecycle status would make it unreadable exactly where it is
written; presence is the entire state, and `terminal-publish.sh` removes the marker on a
successful publish (so a marker in the tree always means a pending deferral). It reads the marker
in the change file, **never** a `git cat-file -e origin/<integration>:<path>` set-diff — a
branch-set diff would reintroduce the standing detector change 0083 deliberately declined, fire
forever under `terminal_publish: false`, and break this script's git-only/offline invariant.
**Residual, stated plainly:** this check reads a marker that only a *compliant driver* writes, so a
deferral nobody marked stays as invisible as it was before change 0083 — the check raises the floor
for drivers that follow the rule, it does not detect the gap independently. Warn-only; it never
mutates the change file.

**`adr-unpublished`** — An ADR whose publish onto the integration branch was due but did not
happen. Gated: emits nothing unless `--adrs-dir` is supplied **and** `--terminal-publish` is
passed. Walks `<adrs-dir>/*.md` (excluding `README.md` and any file whose basename yields no
padded id), and for each resolves the blob on `--metadata-branch` and on `--integration-branch`
via `git rev-parse --verify -q <ref>:<path>` — local refs only, no network. The due rule: a
standalone `Accepted` ADR is due immediately; a `change:`-tied ADR is due once that change reaches
`done` or `killed`; an ADR already present on the integration branch is due always, whatever its
status. An ADR that is neither `Accepted` nor already published is never expected there, and an
unresolvable `change:` stays silent. The change-id column carries the validated `change:` id when
there is one and `?` otherwise (ADR-0049); the ADR number is always named in the message. Report
only — this check writes nothing and heals nothing.

Two arms share the one check-id (the `stale-in-progress` precedent): **missing** — due but absent
on the integration branch; and **stale** — present on both branches with differing blob SHAs, the
un-re-published status flip. An ADR present on the integration branch but not committed on the
metadata branch has nothing to compare against and stays silent.

**`merged-orphan`** — A change id is referenced by a commit *subject* on `--integration-branch`
while the change is still non-terminal (a file under `active/`, not yet archived). This is the
classic orphan: work merged, but the docket record was never closed out. It is a git-history
signal that complements the PR-status sweep — it catches orphans the sweep structurally cannot
(squash-merge under a differently-named branch, an unrecorded `pr:`, or a sweep that never ran).
The message names the evidence commit (short sha + subject). Warn-only; a legitimately
just-merged change has already been archived by the time health checks run (they run after the
sweep), and a transient orphan from a skipped sweep self-clears next pass.

**`unknown-commit-ref`** — A change id is referenced by an `--integration-branch` commit subject
but no change file with that id exists under `active/` or `archive/` (a typo'd or deleted id).
The change-id column is the referenced id; the message names the evidence commit.

**Id-extraction grammar (both checks).** Ids are parsed from commit *subject* lines only, in
exactly two docket-convention forms: a numeric conventional-commit scope `<type>(<id>):`
(e.g. `docket(0085):`, `results(0085):`) and a `(change <id>)` tag (conventionally trailing,
matched anywhere in the subject; e.g. `… (change 0085)`).
Zero-padding is tolerated and normalized to the integer value. Bare `#NNNN` and body text are
deliberately excluded — `#NNNN` collides with PR numbers, and subject-only parsing drops free-text
mentions. The full integration-branch history is scanned on every run (stateless; no `--since`
window, no persisted cursor).

**`dep-cycle`** — A depth-first search (DFS) over `depends_on:` edges marks every node that
lies on a cycle (including both members of a mutual `A→B→A` loop and self-loops `C→C`).
Only edges to known change IDs (present in the file set) are followed; dangling references
to unknown IDs are silently skipped. Every node on a cycle is emitted as a separate finding.

**`field-domain`** — A frontmatter value that is well-formed *text* but outside its field's
*domain*. These are the four fields the board renderers consume; a value outside the domain does
not error, it silently drops the change's row from every board surface (`status`, `slug`) or
injects columns into it (`title`). One finding per violated field, per change.

| Field | Domain | Empty | Failure mode without the check |
|---|---|---|---|
| `status` | one of the lifecycle statuses (`DOCKET_STATUSES` in `lib/docket-frontmatter.sh`) | **fails** | The row is bucketed under an unrecognized key and never emitted, while the file is still counted in the board's total — the count line and the tables disagree. The change also vanishes from the digest's `ready` queue. |
| `slug` | `^[a-z0-9-]+$` — `slugify`'s own alphabet | **fails** | Leaks raw into the digest's space-joined `change` line. |
| `priority` | one of `low`, `medium`, `high`, `critical` | **legal** (`medium`) | Sorts as `medium` in the `ready` queue while rendering raw in the Priority cell. |
| `title` | contains no `|` | legal | Injects extra columns into the `BOARD.md` table row. |

`id` is deliberately **not** covered here — `malformed-id` already detects a non-integer id, and a
second overlapping check would double-report the same file. Every domain is a shape or membership
test; none enumerates bad values.

**`scalar-form`** — The well-formedness leg of the house yaml-scalar rule: a frontmatter scalar
that is *syntactically malformed* YAML even though today's grep/awk readers happen to tolerate it.
It covers the only two free-text string scalars docket reads that are not already gated — `title`
(via `field_raw`) and the optional `blocked_by:` (via `fm_field_verbatim`). This field set is a
**derivation**, never a hand-listed "bad fields" enumeration: the natively-boolean fields are
deliberately excluded (`trivial`, `auto_groomable`, `reconciled`) because a bare `true`/`false`
there is *correct* well-formed YAML, and the shape/domain-gated fields (`status`, `slug`,
`priority`, `type`, `id`) are already covered by `field-domain` or `malformed-id`. At most **one**
finding per field: the predicate reports the first matching leg and stops, since one reason is
enough to demand a quote.

The check reads the **raw** token — never the quote-unwrapped value, which could not tell a quoted
colon-space title from a bare one — and applies a skip leg, then the predicate's empty-value early
return, then **five** syntax legs, in that evaluation order:

- **Skip leg:** the raw value is empty, **or opens with `"` or `'`** — a quoted scalar is
  well-formed by definition (the 0190 quoted-title shape) and is never inspected further. This leg
  lives in the script, not in the predicate.
- **Empty-value early return:** the predicate's own first act, before any syntax leg — an empty
  scalar is well-formed bare. The skip leg above has already caught it for this checker.
- **Colon-space leg:** the unquoted raw value contains `: `.
- **Trailing-colon leg:** the unquoted raw value ends in `:`. This leg closes a real miss: an
  archived change whose title ended in `/ or :` sat unreported because the colon-space leg alone
  never sees a colon at end-of-value (change 0235).
- **Bare-boolean leg:** the unquoted raw value is, whole-value and case-insensitive, exactly one of
  `on`/`off`/`yes`/`no`/`true`/`false` (YAML 1.1).
- **Comment-introducer leg:** the unquoted raw value contains **whitespace** (the `[[:space:]]`
  class — a tab opens a comment exactly as a space does) followed by `#`, which opens a YAML comment
  and silently **truncates** the value rather than aborting the parse — the quieter, worse failure.
  The leg is deliberately as wide as the *reader* it warns about: `fm_field_raw`'s own inline-comment
  strip is `[[:space:]]+#`, so a narrower detector would stay silent about a truncation the reader
  performs. A `#` with no whitespace before it (`issue#3 reopened`) is part of the value.
- **Indicator leg:** the unquoted raw value opens with a YAML indicator character (`[`, `]`, `{`,
  `}`, `,`, `#`, `&`, `*`, `!`, `|`, `>`, `'`, `"`, `%`, `@`, a backtick, `?`, a leading `:`, or a
  leading `- `). A leading `#` is the **maximal** form of the comment-introducer leg above — the
  comment opens at character one, so the whole value parses to null rather than merely being
  shortened; it reaches this leg only when the value carries no `: ` and no ` #` of its own. A
  `[…]` or `{…}` is **not** exempt: the legs judge whether a value is well-formed
  as a bare **scalar**, and a flow collection is not one — bare, `[234]` does not read back as the
  string `[234]`. An exemption would have to run ahead of every other leg to protect that shape,
  and there it silenced them all (`[a title: with colon]` stopped reporting `colon-space`), while
  protecting nothing this checker reads: `title` and `blocked_by` are free text. A caller that
  means a value as a sequence or a map does not route it through a scalar predicate.

The five syntax legs live in **one** place — `lib/docket-frontmatter.sh`'s
`docket_scalar_quote_reason`, which returns a single leg token — so this checker and any future
consumer cannot drift into two copies of the same rule. Only the **skip leg** and the finding
**messages** stay in the script: the skip leg is the one leg that needs the *raw* token (a value
that logically *starts* with a quote character must be quoted, not skipped), and a finding's
wording is this script's output shape, not the predicate's. The check **id is unchanged** — it
gained legs, not a sibling check, so the `docket-status` check-id vocabulary is untouched. Note the
asymmetry with the **writer**: `mint-stub.sh` quotes `title` unconditionally and consumes no
predicate at all (ADR-0071); the checker needs one because it judges hand-authored scalars it did
not write. Guarantee vs. detect — two rules with different jobs.

The `blocked_by:` read is **anchored** to the first `---…---` block via `fm_field_verbatim`
(ADR-0057): the field is optional, so a change that omits it while its body happens to open a
`blocked_by:` line must stay silent — an unanchored read would mistake body prose for the field and
misfire. `fm_field_verbatim` is the anchored accessor that strips **neither** the quotes nor a
whitespace-preceded `#…`, and both halves are load-bearing here. Its sibling `fm_field_raw` strips
the inline comment before returning — deliberately, for the change-template's
`type: feat   # chosen at creation` line — and that strip *is* the truncation the comment-introducer
leg exists to report, so reading `blocked_by` through it would make that leg structurally
unreachable for this field (`blocked_by: PR #69 is stale …` would arrive as `PR` and pass silently).
`title` needs no such twin: `field_raw` has no comment strip.

The full **selection rule** for all four read shapes — which accessor a call site must use, keyed
on whether the key can be absent — is recorded once, canonically, in the
`scripts/lib/docket-frontmatter.sh` header (change 0244). It is not restated here: a copy would
accumulate its own guards and quietly become load-bearing. The rule is enforced by the
(accessor, key) census in `tests/test_frontmatter_read_shapes.sh`.

Warn-only, like every check here: it never marks `EXPLAINED`, never touches `board-row-dropped` (a
malformed *scalar* never drops a row), and never auto-fixes or rewrites the change file. Example
message: `title: unquoted scalar contains ': ' — quote it or reword (well-formed YAML)`.

**`stack-invalid`** — A non-terminal change carrying `stacked_on:` whose **effective base** does not
resolve: `stack_effective_base` (in `lib/docket-stack.sh`) exits `4`. Three data states reach that
exit and the message names all three, because the check cannot distinguish them without re-deriving
the resolver's own walk: the chain names a **missing** parent, the chain **closes a cycle**, or it
reaches a live parent whose `branch:` has **no ref under `refs/remotes/origin/`** — a branch that was
stamped into the manifest at claim time but never pushed. The last state is the common one and it is
not a nicety: cutting a child from an unpushed branch name silently produces a branch based on the
integration branch while everyone believes it is stacked. Remedy: repair the `stacked_on` id, break
the cycle, or push the parent's branch.

**`stack-parent-killed`** — A non-terminal change carrying `stacked_on:` whose chain reaches a
**killed** parent: `stack_effective_base` exits `3`. Separate from `stack-invalid` on purpose — the
remedies are different kinds of act. An invalid chain is a data repair anyone can make; a killed
parent is a **scoping decision** (rescope onto the integration branch, re-parent onto a live change,
or kill the child too) that only a human makes, and spec §9 forbids silently falling back to the
integration branch on its behalf. Merging the two ids would bury the decision inside a repair queue.

Both checks are scoped to **non-terminal** changes: a `done` or `killed` change's chain is history,
and neither re-parenting nor pushing a branch is something anyone can still do about it. Both read
`stacked_on:` with the **anchored** accessor (the key is optional; ADR-0057), and both come from a
**single** `stack_effective_base` call per file whose exit code selects the finding — one call, so
the two legs can never disagree about the same file. The resolver addresses its own
`git show-ref --verify` at the changes dir it is handed, like every other git call in this script,
so this check adds no `-C` wrapper of its own — a second one would compose relatively against the
first; it reads no remotes over the network.

**`aborted-run`** — An `in-progress` change whose autonomous run stopped mid-step: it completed the
visible artifact, narrated success, and dropped the metadata write. The oracle is deliberately
**external** — the agent that dropped the bookkeeping write is the least reliable narrator of
whether it dropped it, and the observed incidents produced confident, specific, wrong reports, so a
check keyed on hedging in the report catches nothing. Gated on `status: in-progress`. **Four**
independent legs; any emits, and more than one may emit on one change.

- **Leg A — manifest/git incoherence (time-free).** The change's `branch:` carries a file under
  `docs/superpowers/plans/` (or under `--results-dir`) that is **absent from the integration
  branch**, while `plan:` (resp. `results:`) is empty. This is the exact **inverse of
  `broken-plan-results`**, which catches *field set, file missing*; leg A catches *file present,
  field empty*. Same two fields, same two trees, opposite direction. An artifact the branch merely
  **inherits** from the integration branch is already-merged work and never fires. The only
  false-positive window is the seconds between an artifact commit and its field write — advisory
  and self-clearing, so the race costs nothing.
- **Leg B — run-scale stale claim (time-based).** `claimed_at:` older than **12 hours**. This
  catches the abort that leaves nothing in git at all (a plan written but never committed), which
  leg A structurally cannot see.
- **Leg C — built but not delivered (time-based, change 0211).** The branch named in `branch:`
  carries commits reachable from **neither** `refs/heads/<integration-branch>` **nor**
  `refs/remotes/origin/<integration-branch>`, its newest commit is older than **2 hours**, and
  `pr:` is empty. Catches the run that finished its build and stopped before delivering it —
  invisible to leg A (every field is coherent: `plan:` recorded, no results file yet) and to leg B
  (the `claimed_at` heartbeat was re-stamped at that very metadata write, so leg B's countdown
  starts from the freshest possible stamp). One leg, two messages, chosen by whether
  `refs/remotes/origin/<branch>` resolves: *branch never pushed* (with the ahead-count and the
  bases it was measured against) or *`<branch>` is pushed but `pr:` unset*.

  **Both messages hedge, and neither prescribes a state change.** They read
  `a run may have stopped before it pushed; verify it is not still building` and
  `a run may have stopped between its push and its PR record; verify the PR exists` — leg B's
  register (`a run may have stopped mid-step; verify it reached its PR`), applied to leg C's own
  seam. Two reasons, both structural rather than stylistic. The predicate fires on healthy runs by
  construction (see **Known residual** below), so asserting the abort as fact would be a claim the
  check cannot support. And a remedy phrased as "push it" or "open the PR", acted on against a run
  that is merely between commits, races the running agent on its own branch. Leg C deliberately
  does **not** reuse leg B's `mid-step`: that phrase stays leg-B-exclusive so a message-shape
  assert can tell the two legs apart. The clause `pr: is unset` is likewise leg-C-exclusive — leg A
  emits `plan: is unset` / `results: is unset`, leg B emits neither — and `tests/test_board_checks.sh`
  keys every leg-C presence and absence assert on it.

  Three design points worth stating, because each is a predicate someone will later be tempted to
  simplify:

  - **Both integration bases are excluded, each `show-ref`-verified.** Feature branches are cut
    from `origin/<integration-branch>` while a local integration ref routinely lags it, so a
    local-only comparison makes a freshly-cut, nothing-built branch look arbitrarily far ahead with
    arbitrarily old commits — firing leg C on a signature that belongs to leg B. No base resolving
    at all is **silence**, never "ahead of nothing": the verified bases are collected into
    `ar_bases`, and a count gate short-circuits the whole predicate when that array is empty. That
    gate is exercised, not asserted — a fixture run against an integration branch that resolves as
    neither ref pins the silence (and pins that a *later* change's finding still appears, so the
    silence is a decline and not a dead walk), and a mutation deleting the gate watches leg C start
    reporting a branch as ahead of an empty base label.
  - **The idle floor is keyed on the branch's newest commit, never on `claimed_at`** — the
    heartbeat rider makes `claimed_at` unusable here, which is precisely why leg B is blind.
  - **A non-empty `pr:` short-circuits the whole leg** before any git call. That keeps the common
    case free, and "unpushed branch with a recorded PR" is a different defect with a different
    remedy that leg C would be a misleading oracle for.

  **Cost:** at most four `git` invocations on a non-firing path (`log -1`, the two base `show-ref`s,
  and `rev-list -n 1`) and six on the firing path — five on the pushed arm, which skips the
  `rev-list --count` — and only for `in-progress` changes with an empty `pr:` — the population legs
  A and B already walk. A change with a recorded PR adds **zero**.

  **Known residual:** the floor keys on the branch tip, so **any** gap longer than 2 hours between
  commits fires leg C on a live, healthy run — a marathon post-build tail with no further commit,
  but equally a single long build task between two per-task commits. The finding is advisory and
  self-clearing once the next commit lands or the PR is recorded; a floor loose enough never to
  misfire would be loose enough to stop detecting.

  **Now covered, by leg D (change 0219):** the run that opens the PR, writes `pr:`, and dies before
  `status: implemented`. Leg C's `pr:`-empty gate still makes it invisible *here* — that gate is
  deliberate — but the state is manifest-internal and detectable git-only, so it needed no widening
  of leg C and no relaxation of this script's contract. Leg D is its complement on the same hoisted
  `pr:` read.

  **The surviving residual is offline, or `gh` unavailable.** Leg C's `pr:`-unset finding is
  *ambiguous by construction*: a PR that exists and merely went unrecorded, and a run that died
  before opening one, produce the identical evidence in git. Resolving them requires asking GitHub,
  which this script will not do — so the resolution lives in `docket-status.sh`'s
  `detect_orphan_pr` (change 0219), beside `detect_merged` where `gh` already lives. That leg mirrors
  this leg's whole gate — the 2h floor **and** the ahead-of-both-bases predicate, with no-base-resolving
  as silence — so the two findings always agree. When `gh` is unavailable it emits
  `orphan-pr-skipped` and goes quiet; leg C's finding still fires, and a human still resolves the
  ambiguity by hand. **That degradation is the design, not a defect:** the offline-safe check stays
  offline-safe.

  A **floor-free** check of the same postcondition does exist, but only where a board pass cannot
  reach: `verify-run` (change 0237) evaluates Step 7's postcondition with **no time floor at all**,
  because it is called at a **dispatch seam** where the child process has already returned and
  "stopped" is therefore unambiguous. That is why this script keeps its floors and is otherwise
  untouched — the two checks answer the same question from positions with different information.

- **Leg D — the Step 7 seam: `pr:` recorded, `status:` never advanced (time-free, change 0219).**
  `pr:` is non-empty while `status:` is still `in-progress`. `docket-implement-next` writes
  `status: implemented` **and** `pr:` in a single field-write and no script under `scripts/` writes
  `pr:`, so this state is an anomaly **by construction** rather than a run in flight — which is why
  the leg carries **no idle floor**. Leg A is the precedent, time-free for the same reason. The
  other three legs are all blind to it: leg A finds no incoherence (`plan:` and `results:` are both
  recorded by then), leg C **short-circuits on a non-empty `pr:`** by deliberate design, and leg B
  catches it only at 12h — the same lag change 0211 exists to close.

  The message names the recorded PR, the field that was never written, and a remedy that stays a
  verification: `pr: records <n> but status: is still in-progress — the run stopped before its final
  status write; verify the PR and set status: implemented`. It deliberately borrows neither leg C's
  exclusive `pr: is unset` clause nor leg B's exclusive `mid-step`, so message-shape asserts keep
  telling the four legs apart.

  **Cost: zero git invocations.** Leg D's predicate is a pure frontmatter test, and it shares its
  `pr:` read with leg C — one anchored read (`ar_pr`) serves both, since the two gates are exact
  complements. Adding a second read on this path would be a real regression (change 0176).

  **Known residual, and it is narrower than it looks.** This script reads change files off the
  filesystem, not out of a git blob. Combined with the single-stroke field-write, leg D's honest
  yield is *uncommitted partial edits in the shared `.docket` worktree, plus non-compliant drivers*
  that write the two fields separately — not a routine abort signature. It is worth having as a
  cheap, additive completeness guarantee over the Step 7 seam, not because it is frequent. No idle
  floor is constructible for an uncommitted edit, so no floor is correct here.

**A separate check-id, not a widened `stale-in-progress`.** That check keys on the same
`claimed_at:` field but at a *human-scale abandonment* horizon (the 72h lease TTL, plus a 3-day
branch-idle signal), with a different remedy — "this looks abandoned, reclaim it" — and a machine
contract `docket-status` keys on, the trailing `[reclaimable]` marker. `aborted-run`'s remedy is
"this run stopped mid-step, go look". Widening the incumbent predicate would silently change what
an already-written consumer sees.

The 12h leg-B window and the 2h leg-C branch-idle floor (`ABORTED_RUN_STALE_SECS` and
`ABORTED_RUN_IDLE_SECS`) are both **hardcoded**, matching `stale-finalize-blocked`'s 72h and
`stale-in-progress`'s 3-day branch-idle horizon; only the lease TTL is a knob.

**Not detected, deliberately:** *"a PR is open but `pr:` is empty"* would need a network probe,
which this script forbids by contract. *"Build commits present while `in-progress`"* is what a
healthy in-flight build looks like, not incoherence.

Warn-only, like every check here: it never marks `EXPLAINED`, never feeds `board-row-dropped`, and
never mutates the change file. Every field it reads (`branch`, `plan`, `results`, `claimed_at`) is
optional, so all four go through the **anchored** `fm_field` (ADR-0057) — an unanchored read would
take body prose for a set field and certify the very abort the check exists to catch.

**`board-row-dropped`** — Backstop for the count-vs-rows invariant, and the only check whose trigger
is **computed rather than enumerated**. A change file is *rendered* iff `int_field id` yields a
non-empty integer **and** its `status:` is a member of the status set the renderer iterates for
that file's directory: `DOCKET_STATUSES_ACTIVE` for `active/`, `DOCKET_STATUSES_TERMINAL` for
`archive/`. Anything else is counted in the board's `total` and rendered nowhere — the count line
and the tables disagree. The predicate (`renders_row` in the script) reads the *same arrays the
renderer's own section iteration uses*, so it is a mirror of the renderer's bucketing in both
directories, not a restatement of the causes the other checks name: a drop path added to either
side of the renderer starts reporting here with **no edit to `board-checks.sh`**.

Emitted **only when no finding already accounts for the drop**. Exactly two arms suppress it, both
already directory-agnostic, and both describe a row *disappearing*:

| Suppressing finding | Why it explains the drop |
|---|---|
| `malformed-id` | A non-integer `id:` — `render-board.sh` skips the row outright, in either directory. |
| `field-domain` on **`status`** | A status outside the `DOCKET_STATUSES` vocabulary is outside both the ACTIVE set and the terminal set, so the row buckets under a key nothing iterates. |

A `field-domain` finding on `slug`, `priority` or `title` does **not** suppress, on either side: none
of them drops a row (a piped `title` injects columns into a row that is still emitted; `priority`
renders raw; `slug` is not read by the markdown renderer at all). Were they to suppress, an
unrelated pipe in a change's title would silence the backstop on a row that vanished for a
different reason.

Live triggers today, one set per directory:

- `active/`:
  - A change file with **no `id:` field at all** — `malformed-id` requires a non-empty (if
    non-integer) value, so nothing else reports it.
  - An `active/` file carrying a **terminal status** (`done` / `killed`) — a *legal* status in the
    *wrong directory*. `field-domain` is correctly silent (`done` is in `DOCKET_STATUSES`) and the
    id is valid, so the computed invariant is the only thing that sees it. This state is reachable
    and documented: `docket-status`'s `sweep-failed <id> archive <reason>` is exactly "status
    flipped to `done`, archive move failed".
- `archive/`:
  - An `archive/` file carrying a **non-terminal** status — the symmetric case: a *legal* status in
    the *wrong directory*, reachable from the same interrupted operation as the active-side case —
    `archive-change.sh` does its `git mv` before the status flip and the commit, so a failure
    between them leaves the file moved but not re-statused.
  - A **terminal** `archive/` file with **no usable id** — same "no `id:` field at all" shape as the
    active side, evaluated after the file has already moved.

Beyond those, the remaining trigger on either side is a future renderer-added drop path.

One thing this predicate deliberately does **not** treat as a drop: the `ARCHIVE_RECENT` collapse.
The archive renderer's recency window redirects an older file's row into its per-month digest
instead of emitting it as a table row, but the file still joins the summary count (`ARC_COUNT`) it
is checked against. The predicate is written against that accounting — count vs. what the renderer
accounts for — not against verbatim per-row emission, so it is blind to the collapse by design, and
must stay blind to it: flagging every digest-collapsed file would make the check fire on every
healthy archive with more than a few months of history.

**`malformed-id`** — Guard/carve-out — it reports a malformed *file* rather than an unhealthy
*change* — but a first-class emitted check-id like the rest, and a full member of the closed
enumeration below. A change file whose `id:` field is non-empty but non-integer emits a
`malformed-id` finding. The change-id column
carries the **filename-derived** padded id (`?` when the filename yields none) — never the raw
frontmatter value, which is untrusted input and would shift the caller's TAB-separated fields; the
raw value appears in the message instead. The file is then skipped for all other checks.

### Sorting and strict mode

All findings are accumulated and sorted by `(check-id asc, change-id numeric asc)` before
output, ensuring deterministic ordering. With `--strict`, the script exits 1 if any findings
were emitted; otherwise it always exits 0.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | No findings (clean tree), or findings present without `--strict`. |
| 1 | One or more findings emitted and `--strict` was passed. |
| 2 | Missing or invalid argument (`--changes-dir` absent/not a directory, unknown flag). |

## Invariants

- **Git-only, offline.** No network calls, no `gh`. All checks use `git cat-file -e` or
  `git log`/`git rev-parse` against the local object store.
- **Warn-only, never auto-fixes.** The script emits findings and exits; it never modifies
  change files, the git index, or any branch.
- **STDOUT for findings, STDERR for errors.** Callers capture stdout to surface findings;
  usage errors and hard failures go to stderr.
- **Deterministic.** Same inputs produce identical output. Sorted by `(check-id, change-id)`
  so the caller can pipe or diff without ordering surprises.
- **`docket-status` owns display.** This script is an implementation detail of `docket-status`
  and surfaces nothing to the user directly — `docket-status` formats and presents the lines.
- **`blocked_by:` re-examination is model-driven.** The skill, not this script, evaluates
  whether a `blocked` change's blocking reason still holds. That judgment is intentionally
  outside the mechanical checker.
- **The findings channel's COLUMNS and RECORDS are not forgeable.** `emit` escapes TAB, CR and LF
  to the visible `\t` / `\r` / `\n` in both embedded columns, and the change-id column never
  carries a raw frontmatter value. The caller splits findings with `IFS=$'\t' read -r check_id
  change_id message`, so an un-escaped TAB in an untrusted value would shift every later field and
  an un-escaped LF would split one finding into two records. The LF case is not hypothetical: since
  change 0202 the `aborted-run` legs embed a git path read NUL-delimited, which may legally contain
  a newline (change 0200).
- **Message TEXT is untrusted; consumers must anchor on the check-id column.** The guarantee above
  is about column integrity, *not* content: `field-domain` messages quote raw frontmatter —
  including free-form `title` prose — verbatim by design, so any token a consumer keys on
  (`[reclaimable]`, say) can appear inside some *other* check's message. A consumer that
  substring-scans the whole findings blob is forgeable by anyone who can write a change file. Key on
  the check-id column: `docket-status.sh`'s `reclaim_pass` anchors its mutating gate at `^check
  stale-in-progress ` and requires the marker at end-of-line, so a marker inside a `field-domain`
  message can never satisfy it — that line begins with a different check-id.
- **The check-id vocabulary is closed, and guarded both ways.** `## Behavior`'s `### Check
  enumeration` is this file's completeness claim: every check-id `board-checks.sh` can emit has a
  section there, and every section there names a check-id it can emit. The set is declared as
  `BOARD_CHECK_IDS` in `lib/docket-frontmatter.sh` and pinned — in both directions — against the
  emitting code, this file, `board-checks.sh`'s `--help` header, and `docket-status.md`'s `check`
  report-line row by `tests/test_board_checks.sh`. Adding a check-id means editing the array plus
  the four surfaces it is pinned against.
