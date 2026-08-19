#!/usr/bin/env bash
# tests/test_finalize_gate.sh — run: bash tests/test_finalize_gate.sh
# Sentinels for the finalize rebase-retest merge gate (change 0015). Sentinels are
# sampling, not parsing — paired with the whole-branch review. Each assert is written
# to flip to NOT OK if the clause it guards is removed (non-vacuous).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

FIN="$REPO/skills/docket-finalize-change/SKILL.md"
CONV="$REPO/skills/docket-convention/SKILL.md"
STAT="$REPO/skills/docket-status/SKILL.md"
GF="$REPO/skills/docket-finalize-change/references/gate-failure.md"   # change 0260: the abort set's canonical home
DYML="$REPO/.docket.yml"
EXAMPLE="$REPO/.docket.example.yml"   # change 0101: the canonical all-keys config reference

# ---- Config parse: the nested finalize.gate key, four modes + default ----------
# Block-scoped awk (the sync-agents.sh idiom), SIGPIPE-safe (capture, no producer|grep).
# Default is `local` (gate on by default); `off` is the documented opt-out.
gate_of(){  # $1 = path to a .docket.yml
  local v
  v="$(awk '
    /^finalize:[[:space:]]*$/{f=1;next}
    f&&/^[^[:space:]#]/{f=0}
    f&&/^[[:space:]]+gate[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/.*gate[[:space:]]*:[[:space:]]*/,"",line);
      gsub(/[[:space:]]/,"",line); print line; exit
    }' "$1" 2>/dev/null)"
  printf '%s' "${v:-local}"
}
TMPC="$(mktemp -d)"
printf 'finalize:\n  gate: local\n'  > "$TMPC/local.yml"
printf 'finalize:\n  gate: ci\n'     > "$TMPC/ci.yml"
printf 'finalize:\n  gate: both\n'   > "$TMPC/both.yml"
printf 'finalize:\n  gate: off\n'    > "$TMPC/off.yml"
printf 'metadata_branch: docket\n'   > "$TMPC/absent.yml"   # no finalize: block
assert "config-parse: gate local"            '[ "$(gate_of "$TMPC/local.yml")" = "local" ]'
assert "config-parse: gate ci"               '[ "$(gate_of "$TMPC/ci.yml")"    = "ci" ]'
assert "config-parse: gate both"             '[ "$(gate_of "$TMPC/both.yml")"  = "both" ]'
assert "config-parse: gate off (opt-out)"    '[ "$(gate_of "$TMPC/off.yml")"   = "off" ]'
assert "config-parse: absent block => local" '[ "$(gate_of "$TMPC/absent.yml")" = "local" ]'
rm -rf "$TMPC"

# ---- approval gating is owned by Go, not restated in the skill (0316) ----------
# RETIRED (0316, category (a)): the old Bash skill carried a `require_pr_approval: false # default
# false` config block, prose calling it "the human sign-off gate", and the GraphQL
# `reviewDecision != APPROVED` predicate. Approval is now owned by `internal/app/finalize_merge.go`
# — `ApprovalSatisfied: in.explicitID || !in.requireApproval` — enforced as a merge conjunct
# ("approval satisfied", SKILL step 8); the skill expresses an unapproved PR as the
# `approval-required` closed skip reason (finalize_context.go), never a restated GraphQL query.
# Authority #2: finalize_merge.go / finalize_context.go own approval. The config KEY still ships in
# .docket.example.yml (asserted below); only the skill's restatement of it is retired. Guard
# re-pointed at the surviving substance.
assert "SKILL enforces approval as a Go-owned merge conjunct, not restated GraphQL" \
  'grep -Eqi "approval satisfied" "$FIN" && grep -Eqi "approval-required" "$FIN"'

# ---- the canonical example carries the knob, active, at its default ----------
# Change 0101 moved the user-facing config documentation out of this repo's own .docket.yml
# (now values-only) and into .docket.example.yml, where every key ships ACTIVE at its default.
# So the discoverability assert follows the documentation.
#
# task-3 review (finding 4): this section used to route its default-value check through an
# rpa_of() awk YAML parser mirroring gate_of()'s idiom. rpa_of is now removed — it was dead
# weight, not real coverage: require_pr_approval is resolver-read in product code
# (scripts/docket-config.sh, layer-tested in tests/test_docket_config.sh), .docket.example.yml is
# pure documentation no docket tooling reads (see test_docket_example_yml.sh's header), and
# rpa_of's only call sites were self-built fixtures round-tripping through rpa_of itself plus this
# one documentation-only file — no product code path was ever exercised by it. The value check
# below now greps the example directly, same shape as its neighboring active-key check.
assert "the example mentions require_pr_approval (discoverability)" \
  'grep -q "require_pr_approval" "$EXAMPLE"'
# The key must be present and ACTIVE, not merely mentioned in prose.
assert "the example ships require_pr_approval as an active key" \
  'grep -qE "^[[:space:]]+require_pr_approval[[:space:]]*:" "$EXAMPLE"'
assert "the example leaves require_pr_approval at default false" \
  'grep -Eq "^[[:space:]]+require_pr_approval[[:space:]]*:[[:space:]]*false([[:space:]]|$)" "$EXAMPLE"'

# ---- selection is owned by Go; the explicit-id approval override survives (0316) ----
# RETIRED (0316, category (a)): the §4.1 ambiguity-only PROMPTING matrix (one-eligible→no-prompt,
# many-eligible→prompt, surface-don't-merge an un-mergeable/unapproved candidate) was the Bash
# procedure's interactive selection UI. Selection is now owned by `internal/app/finalize_context.go`
# — `SelectFinalizeQueue` orders the queue, `FinalizeCandidateReport.Band`/`.SkipReason` classify
# each candidate, and every skipped candidate surfaces its closed skip-reason token (no interactive
# prompt). Authority #2: finalize_context.go owns selection eligibility, ordering, and skip reasons.
assert "SKILL surfaces every skipped candidate with its closed skip-reason token (no prompt)" \
  'grep -Eqi "surfaced with its closed skip-reason token" "$FIN"'
# The one member that SURVIVES is the explicit-id / allowlist authorization overriding the
# approval-required skip (`ApprovalSatisfied: in.explicitID || !in.requireApproval`, finalize_merge.go).
assert "selection: an explicit id overrides the approval-required skip" \
  'grep -Eqi "named id .{0,4}is.{0,4} the human authorization|A named id overrides the .approval-required" "$FIN"'

# ---- the gate is composed into the Go rebase verb; multi-mode gate config is deferred (0316) ----
# RETIRED (0316): the `finalize.gate` config and its four modes (local/ci/both/off) were the Bash
# gate's dispatch table. The local gate is now COMPOSED into `docket finalize rebase`/`rebase-continue`
# (SKILL step 4), and *Out of scope* defers "CI/combined gates" — so `ci`/`both` do not ship and
# there is no `off` no-rebase mode (the sequencer always rebases and gates). Authority #1 (Out of
# scope: deferred CI/combined gates) + Authority #3 (the skill states the gate is composed into the
# rebase verb). The "names all four gate modes" assert went with them — three of the four modes no
# longer exist. Guard re-pointed at the surviving composition.
assert "SKILL composes the local gate into the Go rebase verb" \
  'grep -Eqi "gate is composed into .finalize rebase|composed into .{0,3}docket .?finalize rebase" "$FIN"'

# ---- dispatches the two agents at the right triggers --------------------------
assert "finalize dispatches docket-rebase-resolver on conflict" 'grep -q "docket-rebase-resolver" "$FIN"'
assert "rebase-resolver dispatch is tied to a rebase conflict" \
  'grep -Eqi "conflict[^.]*docket-rebase-resolver|docket-rebase-resolver[^.]*conflict" "$FIN"'
assert "finalize dispatches docket-integration-repair on red tests" 'grep -q "docket-integration-repair" "$FIN"'
# RE-KEYED (0316, category (c)): the dispatch is still tied to a red suite — step 5 is headed
# "Repair a red gate" and dispatches docket-integration-repair on "A red suite after the rebase".
# The old `[^.]*` window broke on the sentence boundary the rewrite inserted ("…regardless of
# cause. Dispatch `docket-integration-repair`"); key on the section header plus the red-suite
# dispatch sentence instead.
assert "integration-repair dispatch is tied to a red/failed suite" \
  'grep -Eqi "Repair a red gate" "$FIN" && grep -Eqi "red suite.{0,80}[Dd]ispatch .docket-integration-repair" "$FIN"'

# ---- the rebased-head push is the Go publish verb's lease, not a hand force-push (0316) ----
# RETIRED (0316, category (a)): the Bash skill force-pushed with `--force-with-lease` after a local
# validation, and this block asserted that ordering by LINE NUMBER. Publishing the rebased head is
# now `docket finalize publish`, which "pushes exactly `head` under the receipt's exact old-value
# lease" AFTER the gate is composed into `finalize rebase` — the ordering (validate, then push) is
# owned by the Go verb sequence, not a skill line order. Authority #2: `finalize publish` receipt
# lease. Guard re-pointed at the surviving substance.
assert "SKILL publishes the rebased head under the publish verb's exact lease" \
  'grep -Eqi "finalize publish" "$FIN" && grep -Eqi "exact old-value lease|under the receipt.s exact" "$FIN"'

# ---- §6 sign-off: attended prompt vs autonomous halt --------------------------
assert "finalize documents repair sign-off" 'grep -qi "sign-off" "$FIN"'
# RE-KEYED (0316, category (c)): the sign-off flow is preserved in step 6 — an ATTENDED run prompts
# the human before merging; an AUTONOMOUS run records the block (repair-needs-signoff) and halts.
# The skill uses "Attended run:"/"Autonomous run:" rather than the old "interactive"/"autonomous".
assert "finalize: attended sign-off prompts before merge" \
  'grep -Eqi "[Aa]ttended run:.{0,200}before merging" "$FIN"'
assert "finalize: autonomous repair records the block and halts" \
  'grep -Eqi "[Aa]utonomous run:.{0,200}repair-needs-signoff" "$FIN"'

# ---- §7 abort-and-report set (the full list of stop points) -------------------
# RE-KEYED (0316, category (c)): the full abort-and-report SET moved into gate-failure.md (the skill
# points at it as a blocking read); count the enumeration in GF, where it now lives.
ab="$(grep -ci "abort-and-report" "$GF")"
assert "gate-failure names abort-and-report multiple times (the enumeration lives here)" '[ "$ab" -ge 3 ]'
assert "abort path: ambiguous rebase conflict"     'grep -Eqi "ambiguous[^.]*conflict|conflict[^.]*ambiguous" "$FIN"'
assert "abort path: no detectable test suite"      'grep -Eqi "no[^.]*suite|suite[^.]*not[^.]*found|no[^.]*test_command" "$FIN"'
assert "abort path: cannot reach green in <=2"      'grep -Eqi "two attempts|<=2|cannot reach green|stuck" "$FIN"'
# RE-KEYED (0316, category (a)): the force-with-lease-rejected / concurrent-push abort member is
# obsolete — the rebased-head push is now `docket finalize publish`, whose un-certifiable push
# surfaces as `rewrite-unknown`/`pr-probe-failed` in the GF abort set, not a hand force-push
# rejection. Keyed on the surviving publish-abort member (which now lives in gate-failure.md).
assert "abort path: a publish that cannot certify (rewrite-unknown/pr-probe-failed)" \
  'grep -Eqi "publish cannot certify|rewrite-unknown|pr-probe-failed" "$GF"'

# ---- LEARNINGS #17: no model/effort literal in the dispatch prose -------------
assert "finalize body restates NO model alias literal" '! grep -qiE "\b(opus|sonnet|haiku|fable)\b" "$FIN"'
assert "finalize body restates NO effort literal" '! grep -qiE "\bxhigh\b" "$FIN"'
assert "finalize names the wrapper as the tier source" 'grep -Eqi "model/effort its wrapper resolves|its wrapper resolves" "$FIN"'

# ---- docket repo dogfoods the gate -------------------------------------------
assert "repo .docket.yml sets finalize gate to local" \
  '[ "$(gate_of "$DYML")" = "local" ] && grep -Eq "^finalize:" "$DYML" && grep -Eq "^[[:space:]]+gate[[:space:]]*:[[:space:]]*local" "$DYML"'

# ---- 0190/0326: the repo DISARMS the results-only skip, and the invisibility guard stands anyway --
# Change 0326 defers `finalize.skip_results_only_delta` for Go v1, so this repo now explicitly
# disarms it: docket's own finalize is governed by the equality-only post-gate delta predicate of
# change 0170, not by the results-only skip. Asserting the value is explicitly `false` (not merely
# "absent" or "any value") keeps this an anti-regression check — it reddens if the key is dropped
# or silently re-armed.
assert "repo .docket.yml disarms finalize.skip_results_only_delta" \
  'grep -Eq "^[[:space:]]+skip_results_only_delta[[:space:]]*:[[:space:]]*false([[:space:]]|#|$)" "$DYML"'
# With the skip disarmed, the invisibility guard is no longer this key's justification. It is
# retained here as a STANDING INVARIANT in its own right: the property it proves — that no
# executable component of this suite reads the results tree as a content source — is a fact about
# this repo's suite that holds independent of the skip, and asserting the guard still ships keeps it
# from being deleted silently and re-armable-unnoticed later.
assert "0190: the results-tree invisibility guard ships in the suite as a standing invariant" \
  '[ -f "$REPO/tests/test_skip_allowlist_invisibility.sh" ] && grep -q "results_dir" "$REPO/tests/test_skip_allowlist_invisibility.sh"'

# ---- convention documents the gate + the two new wrappers --------------------
assert "convention documents finalize.gate" 'grep -Eqi "finalize\.gate|finalize:" "$CONV" && grep -qi "gate" "$CONV"'
assert "convention names the four gate modes" \
  'grep -Eqi "local[^.]*ci[^.]*both[^.]*off|gate.*off.*opt" "$CONV"'
assert "convention names docket-rebase-resolver" 'grep -q "docket-rebase-resolver" "$CONV"'
assert "convention names docket-integration-repair" 'grep -q "docket-integration-repair" "$CONV"'
assert "convention count prose says seventeen wrappers" 'grep -qi "seventeen" "$CONV"'
assert "convention count prose no longer says thirteen wrappers" '! grep -qi "thirteen" "$CONV"'
assert "convention names the no-convention consultant wrapper" 'grep -q "docket-brainstorm-consultant" "$CONV"'
# Non-vacuous count guard: the "seven skills get a wrapper" language must stay exact.
assert "convention keeps 'seven skills get a wrapper' exact" 'grep -qiE "\bseven\b.*skills.* get a wrapper" "$CONV"'

# ---- docket-status notes the gate is finalize-only ---------------------------
assert "status notes the rebase-retest gate is finalize-only" \
  'grep -Eqi "finalize-only|the sweep[^.]*never merges|only archives already-merged" "$STAT"'

# --- close-out and the feature worktree (0075 posture now Go-owned) -----------------------------
# RETIRED (0316, category (a)): the durable-root posture — naming a `REPO_ROOT` literal and running
# the close-out from it, so a `git worktree remove` could not strand the agent's CWD — was a
# skill-side Bash concern. Close-out is now `docket finalize closeout`, a Go transaction that
# reloads metadata, reprobes, and commits by explicit path; the skill runs no bash from a durable
# root, so REPO_ROOT and the "durable root" instruction are gone. Authority #2: Go transactions own
# committing/closeout (the skill no longer commits). The anti-pattern negative and the
# feature-worktree assert below are PRESERVED — the gate's suite still runs in the feature worktree.
assert "0075: finalize does NOT derive the root as dirname of the metadata worktree" \
  '! grep -qE "dirname .*METADATA_WORKTREE" "$FIN"'
# RE-KEYED (0316, category (c)): the gate suite still runs in the feature worktree — step 5 launches
# it with `--cwd <feature worktree>` and both agents are dispatched "naming the feature worktree".
assert "0075: finalize still runs the merge-gate suite in the feature worktree" \
  'grep -qiE "cwd <feature worktree>|naming the feature worktree" "$FIN"'

# --- merge authorization after 0095 (auto_approve retired) ---
assert "0095: no live auto_approve/docket-approve reference in the finalize skill" \
  '! grep -Eqi "auto_approve|docket-approve|setup-auto-approve" "$FIN"'
# RETIRED (0316, category (a)): the auto-detect approval gate no longer restates the GraphQL
# `reviewDecision`/`APPROVED` predicate — approval is a Go merge conjunct (`ApprovalSatisfied`,
# finalize_merge.go) surfaced as the `approval-required` skip reason (see the approval retirement
# near the top of this file for the full cite). The "approved PR merges without --admin" invariant
# is preserved as the positive that `--admin` is honored ONLY to force past a required review and is
# never inferred — so a normal approved merge needs none.
assert "0095: --admin is honored only to force past a required review, never inferred" \
  'grep -Eqi ".--admin. is honored .{0,12}only.{0,160}required review" "$FIN" && grep -Eqi "never inferred from an approval absence" "$FIN"'

# Adjacency, not mere co-occurrence: two independent existence checks are vacuous here —
# `explicit id` is independently satisfied by the Selection section (lines ~40, 58, 86) and
# `--admin` by the flow's step 6 opener, so deleting the actual escape-hatch clause left both
# conjuncts green (mutation-proven). Flatten newlines first — the real clause ("`--admin`
# remains available only on the pre-existing explicit-id / attended paths...") wraps across two
# physical lines in the source, and grep does not match across lines by default.
admin_flat="$(tr '\n' ' ' < "$FIN")"
assert "0095: --admin survives only as the explicit-id / attended escape hatch (adjacent)" \
  'grep -Eqi -- "admin.{0,60}(explicit[- ]id|attended)" <<<"$admin_flat"'
# RETIRED (0316, category (a)): terminal publishing is DEFERRED — 0316's *Out of scope* names
# "terminal publishing" among the deferred capabilities, so the skill carries no `terminal_publish`
# degradation prose. Authority #1 (Out of scope: terminal publishing). Inverted guard proving the
# deferred surface stayed out, preceded by a non-vacuity anchor so an empty file cannot pass. When
# the Out-of-scope set is lifted (a later change), restore a real degradation assert.
assert "SKILL is the Go sequencer (non-vacuity anchor)" 'grep -qF "docket finalize" "$FIN"'
assert "SKILL carries no deferred terminal_publish degradation prose" \
  '! grep -qi "terminal_publish" "$FIN"'

# --- change 0260: the two new abort-and-report members live in gate-failure.md ------------------
# The enumeration is one long line today, but guard it through a whitespace-collapsed haystack
# anyway: a future re-flow must not redden asserts about policy that did not change (learnings:
# phrase-grep-over-wrapped-prose).
gf_flat="$(tr '\n' ' ' < "$GF" | tr -s '[:space:]' ' ')"
assert "0260: gate-failure.md is reachable (non-vacuity for the flattened asserts below)" \
  '[ "${#gf_flat}" -gt 500 ] && grep -qF -- "abort-and-report points" <<<"$gf_flat"'

# SECTION-scoped, not file-scoped, for the two membership asserts: the site-marker paragraph added
# by this same change says "the dispatch itself is unavailable ... the `carve-out`" in the section
# ABOVE, so a file-wide grep for those two claims is satisfied whether or not the enumeration ever
# lists the reason — which is precisely the property being guarded ("a LISTED reason, not an
# implied one"). Mutation-proven: deleting the enumeration member leaves a file-scoped grep green.
abort_flat="$(awk '/^## abort-and-report points/{f=1;next} f&&/^## /{exit} f' "$GF" | tr '\n' ' ' | tr -s '[:space:]' ' ')"
assert "0260: the abort-set section extractor still reaches the enumeration (non-vacuity)" \
  '[ "${#abort_flat}" -gt 200 ] && grep -qF -- "ambiguous rebase conflict" <<<"$abort_flat"'

# Member 1 (push mechanics) — RETIRED (0316, category (a)). Change 0260's two push-related abort
# members — a harness/permission DENIAL of the post-rebase `--force-with-lease` push (conditioned on
# Harness-native recovery), and the distinct concurrent-push lease rejection — are obsolete. The
# rebased-head push is now `docket finalize publish`, a Go verb that owns the lease; an
# un-certifiable push surfaces as the "rewrite the publish cannot certify" abort member
# (`rewrite-unknown`/`pr-probe-failed`), and there is no hand force-push for a harness to deny.
# Authority #2: `finalize publish` owns the lease push. Guard re-pointed at the surviving
# publish-abort member in the enumeration (over the same abort_flat haystack).
assert "0316: the abort set names an un-certifiable publish (rewrite-unknown/pr-probe-failed)" \
  'grep -qE -- "publish cannot certify|rewrite-unknown|pr-probe-failed" <<<"$abort_flat"'

# Member 2 — the carve-out's posture pointer must resolve to a LISTED reason, not an implied one.
assert "0260: abort set names dispatch-unavailability for the gate agents" \
  'grep -qE -- "dispatch[^.]{0,80}unavailable|unavailable[^.]{0,80}dispatch" <<<"$abort_flat"'
assert "0260: the dispatch-unavailable member points at the carve-out" \
  'grep -qE -- "(dispatch|unavailable)[^.]{0,120}carve-out" <<<"$abort_flat"'

# RETIRED (0316, category (a)): the old GF enumeration ended with a count sentence ("flatten the N
# distinct abort reasons"), and this assert pinned it de-numeralized. The Go-sequencer rewrite
# dropped the count sentence entirely — the abort set is now a plain bulleted list under
# "## abort-and-report points (the full set)" with no running tally — so there is no numeral to
# guard. Authority #3: the rewritten GF states the set with no count sentence. Inverted guard (the
# count sentence must be ABSENT) with a non-vacuity anchor on the section header.
assert "0316: the abort set is a plain enumeration with no running count sentence" \
  '! grep -qiE "the (two|three|four|five|six|seven|eight|nine|ten|[0-9]+) distinct abort reasons" <<<"$gf_flat" && grep -qF -- "abort-and-report points (the full set)" <<<"$gf_flat"'

exit $fail
