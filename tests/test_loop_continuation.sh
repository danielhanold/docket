#!/usr/bin/env bash
# tests/test_loop_continuation.sh — guards change 0088 (loop continuation: docket-implement-next as
# a driver-agnostic re-invocation contract). Asserts the four-disposition terminal contract, the
# per-step-exit mappings, id-set scoping (SKILL.md), and the README /loop drain-pattern doc.
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review; this test does not replace it.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if ( eval "$2" ); then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# --- Matchers ------------------------------------------------------------------------------------
# Every matcher this file mutation-proves is defined HERE, once, and called from BOTH the live
# assert and its probe. A probe that restated its matcher as a fresh literal would prove nothing
# about the assert that actually guards the file: edit one twin and the probe stays green while the
# live assert silently stops matching (guards-are-code; mirrored-guard-enforces-its-own-property one
# layer in — the probe must be evidence about the live matcher).
#
# Each matcher reads its haystack on STDIN, so the live call is `m < "$IMPL"` (or `m <<<"$impl_flat"`
# for the whitespace-collapsed phrase asserts) and the probe is the same call over the probe file.
# STDIN, not an argument, so no call site needs a pipe — AGENTS.md *Shell* forbids piping a producer
# into `grep -q` under `set -o pipefail`.
#
# Backticks live inside these function bodies and never inside an assert argument: assert() evals its
# argument, so a backtick there would re-parse as command substitution. A function body is parsed
# once, at definition, and eval never re-reads it.
#
# The backtick itself comes from `$BT` rather than being written into the double-quoted pattern: no
# backtick may sit inside double quotes in test source, because bare it runs when the shell reads
# the line and backslash-escaped the escape is consumed there, sending a bare backtick on to the
# next evaluation (change 0221, scripts/check-test-source-hygiene.sh). An expansion's result is
# never re-scanned for command substitution, so the concatenation is inert.
BT='`'
has_code_token(){ grep -qF -- "$BT$1$BT"; }
has_postconditions_heading(){ grep -qF -- "### Step postconditions"; }
table_row(){ grep -F -- "| $1 |"; }              # the extractor…
has_table_row(){ table_row "$1" > /dev/null; }   # …and the presence test over the same literal
row_names_field(){ grep -qF -- "$1"; }
has_step_certificate_rule(){ grep -qF -- "certify a **step**, never the run"; }
has_step7_completes_run(){ grep -qF -- "the only postcondition that also completes the run is Step 7's"; }
# Finding 5: matched as the surrounding phrase, not the bare word `cumulative`. The bare word is
# common enough that a 160-line file could satisfy it from prose asserting the opposite; this pins
# the claim itself. It is NOT subsumed by has_scoped_cumulativity below — that one stays green if the
# claim is reworded away while some scoping clause survives within 200 chars of the word.
has_cumulative_claim(){ grep -qF -- "conditions are **cumulative**"; }
has_scoped_cumulativity(){ grep -qE "cumulative.{0,200}as of the close of its own step"; }
has_non_git_admission(){ grep -qF -- "is an in-context artifact, not a git object"; }
has_head_sha_qualification(){ grep -qE "in-context artifact, not a git object.{0,80}== HEAD conjunct is a git fact"; }
has_field_write_conjuncts(){ grep -qE "field-write rule.{0,120}claimed_at.{0,160}Artifacts.{0,40}block in the same commit"; }
has_postcondition_pointer(){ grep -qE "git-state postcondition.{0,120}Step postconditions"; }

IMPL="$REPO/skills/docket-implement-next/SKILL.md"

# --- SKILL.md: the four-disposition terminal contract ---
assert "SKILL has a Terminal disposition section" 'grep -Eqi "Terminal disposition" "$IMPL"'
for d in advanced contended drained halted; do
  assert "SKILL names disposition $d (code-formatted)" 'has_code_token "$d" < "$IMPL"'
done
# The binary driver rule — both halves must be present (non-vacuous).
assert "SKILL states continue-on advanced/contended" 'grep -Eqi "continue on .{0,4}advanced" "$IMPL"'
assert "SKILL states stop-on drained/halted" 'grep -Eqi "stop on .{0,4}drained" "$IMPL"'
# Skipped-with-reasons enumeration.
assert "SKILL enumerates skipped-with-reason" 'grep -Eqi "skipped with (its|the) reason" "$IMPL"'

# --- SKILL.md: the closing obligation on the AGENT (change 0212) ---
# The pre-0212 section was guidance to a DRIVER only; nothing bound the running agent to declare a
# disposition, and the 0206 run closed with a step-scoped "build disposition" instead. These asserts
# pin the obligation itself, not the table it sits under.
assert "SKILL obliges the agent to declare before the run ends" \
  'grep -qF -- "the run does not end until" "$IMPL"'
assert "SKILL names a non-conforming vocabulary an aborted run" \
  'grep -qF -- "is by construction an aborted run" "$IMPL"'
# advanced is gated on Step 7 BY POINTER — 0212 must not define Step 7's postcondition (that is
# change 0203's surface), so assert the pointer and not any postcondition wording.
assert "SKILL gates advanced on Step 7's postcondition by pointer" \
  'grep -Eqi "advanced.{0,80}Step 7|Step 7.{0,80}advanced" "$IMPL"'

# --- SKILL.md: the per-step postcondition table (change 0203) ---
# 0113 added the clause "the step is not complete until its git-state postcondition holds" and
# defined no postcondition anywhere; the term occurred exactly once in skills/. These asserts pin
# both halves of the repair: the DEFINING section, and — separately — that Step 5's producer-side
# clause actually points at it. A file-level "the term occurs more than once" count would be the
# 0199 co-occurrence weakness: it proves nothing about Step 5, which is where the reader hits the
# term. (It would also be false here: the section does not repeat the term, so the file still
# carries exactly one occurrence — the proximity assert below is the only thing pinning that it is
# no longer orphaned.)
#
# Phrase asserts read a whitespace-collapsed haystack so a pure re-flow of the table's prose never
# reddens a policy assert (learnings: phrase-grep-over-wrapped-prose, change 0218). The idiom is
# the same one-liner as the `flatten` helper in tests/test_docket_review.sh — deliberately
# re-stated rather than extracted, at 1 line it is cheaper to read here than to follow a pointer.
#
# DEVIATION from the plan's literal block, required by AGENTS.md *Shell*: the plan wrote every
# phrase assert as `flatten < "$FILE" | grep -q …`. Under this file's `set -o pipefail` that is a
# producer piped into an early-exiting consumer — `tr` takes SIGPIPE, the pipeline returns 141, and
# the assert goes intermittently red. Flattening is therefore done once into a variable via command
# substitution and matched with a here-string, exactly as tests/test_docket_review.sh does it.
flatten(){ tr -s '[:space:]' ' '; }

# Non-vacuity anchor #1: the file every assert below reads must exist and be non-empty, or the
# whole block passes for reasons unrelated to the property (style borrowed from
# tests/test_role_skill_self_description.sh, NOT from this file's own thinner precedent).
assert "SKILL.md exists and is non-empty" '[ -s "$IMPL" ]'
impl_flat="$(flatten < "$IMPL")"

# The defining section.
assert "SKILL states a Step postconditions section" 'has_postconditions_heading < "$IMPL"'

# Every step the table claims to cover has a row. A missing row is the exact 0113 defect (Step 4
# received no rider) and is what this loop exists to catch.
for row in "2 Claim" "3 Reconcile" "4 Worktree + plan" "5 Build" "6 Review + ADRs" "7 PR + stop"; do
  assert "the postcondition table has a row for Step $row" 'has_table_row "$row" < "$IMPL"'
done

# Row 2 must name ALL FOUR fields Step 2's procedure sets. The pre-fix row omitted `updated:`,
# making the certificate weaker than the step it certifies — the same defect class as the missing
# field-write conjuncts below, just inside a single row.
row2="$(table_row "2 Claim" < "$IMPL")"
for f in "status: in-progress" "branch:" "updated:" "claimed_at:"; do
  assert "row 2 names the $f field Step 2 writes" 'row_names_field "$f" <<<"$row2"'
done

# The governing sentence — a step certificate is NEVER a run certificate. This is the half the
# 0206 evidence bought: that run satisfied Step 5's postcondition at the moment it died.
assert "SKILL states the postconditions certify a step, not the run" \
  'has_step_certificate_rule <<<"$impl_flat"'
assert "SKILL states only Step 7's postcondition also completes the run" \
  'has_step7_completes_run <<<"$impl_flat"'
assert "SKILL states the conditions are cumulative" \
  'has_cumulative_claim <<<"$impl_flat"'
# ...and that cumulativity is READ-SCOPED to each row's own step. Unqualified cumulativity makes
# Step 7 unsatisfiable on the Step-6.5 path: the results commit moves branch HEAD after the
# evidence was minted, so Steps 5/6 `head_sha` == HEAD is false at Step 7 and `advanced` — which
# only Step 7's row licenses — could never be declared on any run that writes a results file.
# references/edge-paths.md already states the opposite rule ("a stale `head_sha` on that path is
# EXPECTED, not a defect"); this assert pins the two files agreeing. Proximity-scoped so a
# scoping clause parked in an unrelated paragraph cannot satisfy it.
assert "SKILL scopes cumulativity to each row's own step" \
  'has_scoped_cumulativity <<<"$impl_flat"'
# ...and the "read from git" rule must NAME its one exception. The build-evidence record rows 5-6
# turn on is NOT a git artifact: docket-build emits it as output, its default BUILD_CHECKPOINT:
# false "persist[s] nothing", and even the true-path ledger lives under `.superpowers/`, which
# .gitignore excludes. An unqualified universal here is simply false, and the honest reading is what
# makes the head_sha == HEAD conjunct load-bearing rather than decorative. Proximity-scoped so the
# admission has to sit inside the governing rule, not in some distant paragraph.
assert "SKILL names the build-evidence record a non-git artifact" \
  'has_non_git_admission <<<"$impl_flat"'
assert "SKILL pins head_sha == HEAD as the record's only git fact" \
  'has_head_sha_qualification <<<"$impl_flat"'

# ...and the metadata-commit rows (3/4/6/7) must inherit the field-write rule's OWN conjuncts. Those
# rows name only the field they set, but the field-write rule mandates two more git-checkable facts
# on the same commit: the claimed_at re-stamp, and — for a link-bearing field — the regenerated
# ## Artifacts block. Without this clause a step is declarable complete on a commit missing the
# regen, which is the "row weaker than the procedure it certifies" failure the section exists to
# prevent. Stated ONCE (shared clause) rather than per row, so this assert is proximity-scoped
# across the three parts instead of looping over rows.
assert "SKILL binds the metadata rows to the field-write rule's own conjuncts" \
  'has_field_write_conjuncts <<<"$impl_flat"'

# PROXIMITY-SCOPED producer assert (learnings: specified-but-unreachable). The contract's producer
# is Step 5's clause; anchoring only on the defining section would leave the term orphaned exactly
# where a reader meets it. Extract the Step 5 region with awk — the extractor MUST keep newlines
# or the slice becomes the whole file — then flatten only the slice.
step5_region(){ awk '/^### Step 5 — Build/{f=1; next} f && /^### /{exit} f' "$IMPL"; }
s5="$(mktemp)"; step5_region > "$s5"
# Non-vacuity anchor #2: a live PRESENCE assert through the same extraction. If the Step 5 heading
# is renamed, this reddens instead of the proximity assert below going quietly green on an empty
# slice.
assert "the Step 5 region extracts and is non-empty" '[ -s "$s5" ]'
assert "the Step 5 region is really Step 5 (names the build role)" \
  'grep -qF -- "SKILL_BUILD" "$s5"'
s5_flat="$(flatten < "$s5")"
assert "Step 5's git-state postcondition clause points at the table" \
  'has_postcondition_pointer <<<"$s5_flat"'
rm -f "$s5"

# --- Mutation proofs: one per matcher introduced above (guards-are-code;
# assert-detects-removal-not-replacement; mirrored-guard-enforces-its-own-property). A matcher
# that has never been shown to go RED against the state it forbids is untested code. Every probe
# below calls THE SAME matcher function the live assert calls — never a re-typed literal — so the
# proof is evidence about the guard that actually reads SKILL.md.
probe="$(mktemp)"
# (a) the heading matcher fires on the heading and not on a near-miss.
printf '%s\n' '### Step postcondition' > "$probe"   # singular — a real typo shape
assert "the heading matcher rejects a singular near-miss" '! has_postconditions_heading < "$probe"'
printf '%s\n' '### Step postconditions' > "$probe"
assert "the heading matcher fires on the real heading" 'has_postconditions_heading < "$probe"'
# (b) the row-label matcher fires only on a table row, not on prose naming the step.
printf '%s\n' 'Step 4 Worktree + plan is where the plan file is written.' > "$probe"
assert "the row matcher ignores prose that merely names a step" '! has_table_row "4 Worktree + plan" < "$probe"'
printf '%s\n' '| 4 Worktree + plan | something holds. |' > "$probe"
assert "the row matcher fires on a real table row" 'has_table_row "4 Worktree + plan" < "$probe"'
# (c) the flattened phrase matchers survive a line wrap — the property flatten() exists for — and
# still reject the phrase's absence.
printf 'These\ncertify a **step**,\nnever the run.\n' > "$probe"
assert "the flattened phrase matcher survives a hard wrap" \
  'has_step_certificate_rule <<<"$(flatten < "$probe")"'
printf 'These certify a step, never the run.\n' > "$probe"   # unbolded — a near-miss
assert "the flattened phrase matcher rejects an unbolded near-miss" \
  '! has_step_certificate_rule <<<"$(flatten < "$probe")"'
printf 'the only postcondition that also completes the run is Step 6'"'"'s\n' > "$probe"
assert "the Step 7 phrase matcher rejects the wrong step number" \
  '! has_step7_completes_run <<<"$(flatten < "$probe")"'
# (c1) the cumulativity-claim matcher. It is phrase-scoped (finding 5), so its near-miss is prose
# that carries the word without making the claim — including prose asserting the OPPOSITE, which the
# pre-fix bare-word matcher accepted anywhere in the file.
printf 'the conditions are independent.\n' > "$probe"
assert "the cumulative matcher rejects prose that omits the claim" \
  '! has_cumulative_claim <<<"$(flatten < "$probe")"'
printf 'These conditions are not cumulative; nothing carries forward.\n' > "$probe"
assert "the cumulative matcher rejects prose asserting the opposite" \
  '! has_cumulative_claim <<<"$(flatten < "$probe")"'
printf 'The conditions\nare **cumulative**: each holds in addition to every earlier step'"'"'s.\n' > "$probe"
assert "the cumulative matcher ACCEPTS the claim (across a wrap)" \
  'has_cumulative_claim <<<"$(flatten < "$probe")"'
# (c2) the read-scoping matcher must go RED on the UNQUALIFIED governing sentence — the exact state
# that made Step 7 unsatisfiable on the Step-6.5 path — and green once the scoping clause is back.
printf 'The conditions are **cumulative**: each holds in addition to every earlier step'"'"'s.\n' > "$probe"
assert "the read-scoping matcher REJECTS unqualified cumulativity" \
  '! has_scoped_cumulativity <<<"$(flatten < "$probe")"'
printf 'The conditions are **cumulative**: each holds in addition to every earlier\nstep'"'"'s, each read **as of the close of its own step**.\n' > "$probe"
assert "the read-scoping matcher ACCEPTS the scoped sentence (across a wrap)" \
  'has_scoped_cumulativity <<<"$(flatten < "$probe")"'
# ...and is not satisfied by a scoping clause far from the cumulativity claim.
printf 'cumulative.%s as of the close of its own step\n' "$(printf ' x%.0s' $(seq 1 210))" > "$probe"
assert "the read-scoping matcher rejects a distant co-occurrence" \
  '! has_scoped_cumulativity <<<"$(flatten < "$probe")"'
# (c3) the evidence-qualification matcher must go RED on the UNQUALIFIED governing sentence — the
# state in which the section asserts a universal ("read from git, never from a sub-skill's report")
# that rows 5-6 cannot honour — and green once the exception is named, across a wrap.
printf '%s\n' 'Each step below is complete only when its row holds — read from **git**, never from a sub-skill'"'"'s report or its own narration.' > "$probe"
assert "the evidence-qualification matcher REJECTS the unqualified rule" \
  '! has_head_sha_qualification <<<"$(flatten < "$probe")"'
assert "the non-git-artifact matcher REJECTS the unqualified rule" \
  '! has_non_git_admission <<<"$(flatten < "$probe")"'
printf 'the build-evidence record of rows 5-6 is an in-context artifact, not\na git object, so only its head_sha == HEAD conjunct is a git fact.\n' > "$probe"
assert "the evidence-qualification matcher ACCEPTS the named exception (across a wrap)" \
  'has_head_sha_qualification <<<"$(flatten < "$probe")"'
assert "the non-git-artifact matcher ACCEPTS the admission (across a wrap)" \
  'has_non_git_admission <<<"$(flatten < "$probe")"'
# ...and is not satisfied by the two halves parked far apart.
printf 'in-context artifact, not a git object.%s == HEAD conjunct is a git fact\n' "$(printf ' x%.0s' $(seq 1 90))" > "$probe"
assert "the evidence-qualification matcher rejects a distant co-occurrence" \
  '! has_head_sha_qualification <<<"$(flatten < "$probe")"'
# (b2) the row-2 field matcher reddens on the pre-fix row — the one that dropped `updated:`.
printf '%s\n' '| 2 Claim | `status: in-progress` + `branch:` + `claimed_at:` committed on `metadata_branch`. |' > "$probe"
assert "the row-2 field matcher REJECTS the row that omits updated:" \
  '! row_names_field "updated:" <<<"$(table_row "2 Claim" < "$probe")"'
# (c4) the field-write-conjuncts matcher must go RED on a row that names the rule but carries none
# of its conjuncts — the state the finding reported — and green once both are named, across a wrap.
printf '%s\n' 'plan: landed on metadata_branch per the **field-write rule** — a two-tree conjunction.' > "$probe"
assert "the field-write-conjuncts matcher REJECTS a pointer with no conjuncts" \
  '! has_field_write_conjuncts <<<"$(flatten < "$probe")"'
printf 'the **field-write rule**'"'"'s conjuncts on every metadata commit they name:\nthe claimed_at re-stamp, and — for any link-bearing field write — the\nregenerated ## Artifacts block in the same commit.\n' > "$probe"
assert "the field-write-conjuncts matcher ACCEPTS the shared clause (across a wrap)" \
  'has_field_write_conjuncts <<<"$(flatten < "$probe")"'
# ...and half of it is not enough: dropping the Artifacts regen must redden.
printf '%s\n' 'the **field-write rule**'"'"'s conjuncts: the claimed_at re-stamp on every metadata commit.' > "$probe"
assert "the field-write-conjuncts matcher rejects the claimed_at half alone" \
  '! has_field_write_conjuncts <<<"$(flatten < "$probe")"'
# (d) THE load-bearing one: the proximity assert must go RED on the pre-0203 state — the clause
# present with no pointer. This is the state 0203 removed, not the wording it introduced.
printf '%s\n' 'the step is not complete until its git-state postcondition holds. docket-build routes each task.' > "$probe"
assert "the proximity matcher REJECTS the orphan clause (the pre-0203 defect)" \
  '! has_postcondition_pointer <<<"$(flatten < "$probe")"'
printf '%s\n' 'the step is not complete until its git-state postcondition holds — see *Step postconditions*.' > "$probe"
assert "the proximity matcher ACCEPTS the repaired clause" \
  'has_postcondition_pointer <<<"$(flatten < "$probe")"'
# (e) and it must not be satisfied by a far-apart co-occurrence elsewhere in the same region.
printf 'git-state postcondition holds.%s Step postconditions\n' "$(printf ' x%.0s' $(seq 1 130))" > "$probe"
assert "the proximity matcher rejects a distant co-occurrence" \
  '! has_postcondition_pointer <<<"$(flatten < "$probe")"'
# (f) the Step 5 extractor is really scoped — the postcondition TABLE's own prose must not leak
# into the slice, or the proximity assert would pass off the defining section rather than Step 5.
assert "the Step 5 extractor excludes the defining section" \
  '! has_postconditions_heading <<<"$(step5_region)"'
rm -f "$probe"

# --- SKILL.md: per-step-exit mappings ---
assert "SKILL ties a lost claim race to contended (Step 2)" 'grep -Eqi "claim (CAS|race)" "$IMPL"'
assert "SKILL ties the empty queue to drained (Step 1)" 'grep -Eqi "empty queue|no candidate|nothing .{0,20}build-ready" "$IMPL"'

# --- SKILL.md: id-set scoping ---
assert "SKILL documents an id allowlist" 'grep -Eqi "allowlist" "$IMPL"'
assert "SKILL shows the comma-separated id-set form" 'grep -Eq "docket-implement-next 90,92,94" "$IMPL"'
assert "SKILL states the allowlist is not a dependency override" 'grep -Eqi "never a dependency override" "$IMPL"'

README="$REPO/README.md"
wb='`/loop docket-implement-next`'

# --- README: the /loop drain-pattern doc ---
assert "README documents the /loop whole-backlog drain" 'grep -qF "$wb" "$README"'
assert "README documents the /loop id-set drain" 'grep -Eq "/loop docket-implement-next 90,92,94" "$README"'
assert "README states the driver never merges" 'grep -qiF "never merges** — the human merge gate is untouched" "$README"'
assert "README names all four dispositions" 'for d in advanced contended drained halted; do grep -qiF "$d" "$README" || exit 1; done'

# --- Non-vacuity / mutation proof: the code-formatted disposition grep actually bites. ---
probe="$(mktemp)"; printf 'plain advanced word, no code formatting\n' > "$probe"
assert "the code-formatted disposition grep is non-vacuous" '! has_code_token advanced < "$probe"'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
