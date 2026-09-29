<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0468 — Rename colliding docket terms and retire obsolete glossary entries](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-29-0468-rename-colliding-docket-terms-and-retire-obsolete-glossary-e.md)**
<!-- docket:backlink:end -->
# Rename Colliding Docket Terms and Retire Obsolete Glossary Entries Implementation Plan

> **For agentic workers:** Execution is via the `docket-build` role (task-by-task through its profile agents under the `docket-build-task` contract). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the prose-only renames of spec rows 60–65 across docket's maintained sources, and move the four retired features to a new "Obsolete terms" section of the glossary (row 66).

**Architecture:** This is a one-time prose sweep with no runtime behavior change. It has three kinds of edit: (1) the glossary restructure (Task 1); (2) one task per rename family over docs, skills, agents, cursor rules, the example config and Go comments/messages (Tasks 2–6), each deriving its sites from a whitespace-tolerant whole-repo scan rather than a hand list, then regenerating the embedded asset mirror (`go generate ./internal/assets/`) and the harness goldens (`-update`) in the same commit when it touches an authored root; (3) a residual audit plus the full suite (Task 7). No new guard is added (spec, *Testing*).

**Tech Stack:** Markdown docs and skill bodies; `.docket.example.yml` comments; Go comments and message strings; `cmd/genassets` (via `go generate ./internal/assets/`); golden tests in `internal/harness/{claude,codex,cursor,opencode}`; the Go suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-09-29-rename-colliding-docket-terms-and-retire-obsolete-glossary-e-design.md` on the `docket` metadata branch (readable at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-29-rename-colliding-docket-terms-and-retire-obsolete-glossary-e-design.md`). The rows this plan carries out are in its *0468 itself — prose only* table (rows 60–66). Section *2. Prose-only renames (0468)* defines the scope.

## Global Constraints

Worktree (every command runs here; use absolute paths): `/Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e`. Base commit: `4b318461c4b0d28916c3acffc2add03f7272fff4`.

- **Rows owned by this change, and nothing else:**
  - 60: coordination-key fence → **shared-setting guard**
  - 61: human merge gate → **PR handoff**
  - 62: "merge gate" and "rebase-retest gate" (aliases of the finalize gate) → dropped; **finalize gate** stays
  - 63: "test gate" (alias of the suite gate) → dropped; **suite gate** stays
  - 64: terminal status / terminal record / terminal sweep / terminal close-out (change lifecycle) → **final status / archived record / merged-PR sweep / close-out**
  - 65: autonomous-eligible → folded into **auto-groomable**
  - 66: runner delegation, runner shim / `runners` block, `runtime.bash`, terminal publish → the glossary's **Obsolete terms** section
- **Family-owned terms stay untouched** (rows 1–59 belong to 0471–0474). Do not rename: run gate, gate key, arm/arming, run epoch, epoch fence, dispatch context, `gate-*` tokens, `--run-epoch`, `--version`, change version, build profile, review rung, dispatch tiers, re-arm / `rearm`, `nothing-to-rearm`, `fenced-setting-ignored`, `terminal-backlink-pending`, `terminal-notes-frozen`, `change-terminal-claim-stamp`, `drop-terminal-claimed-at`, `not-terminal`, `skipped-terminal`, `adr-update-after-terminal`. Go identifiers that carry these (for example `CodeFencedIgnored`, `applyFence`, `scopeRepoFenced`, `closeoutNotesMatchTerminal`, `CodeChangeTerminalClaimStamp`) also stay.
- **`internal/config/**` is out of scope.** Its "fence" / "coordination fence" comments describe the `fenced-setting-ignored` machinery and its `*Fence*` identifiers, which change 0474 (row 56) renames together. Leave that prose there so comments never disagree with the identifiers beside them.
- **Process-level "terminal" stays** (spec Decision 6). This covers a run's or process's finished state: a gate run's `terminal.json` / "durable terminal record", "terminal result", "terminal disposition", "terminal verdict", "nonterminal", `record-terminal`, `terminal_receipt`. Only the **change-lifecycle** sense of "terminal" (a change in `done`/`killed`, its archived file, the sweep, the close-out) is renamed.
- **"Terminal publish" / "terminal publication" stay as the retired feature's name.** They match the config key `terminal_publish`, and config keys are never renamed (spec Decision 9). Row 66 only relocates the glossary entry.
- **"Terminal half"** (`docket finalize` / `docket maintenance` Cobra `Short:` help text, `docs/reference/cli.md`, `docs/reference/config-keys.md`, `skills/docket-finalize-change/SKILL.md`, Go comments) is **not** a row-64 phrase. Leave it; Task 7 reports it as a residual.
- **Never rename files or paths.** Keep `skills/docket-convention/references/terminal-close-out.md`, `docs/concepts/build-profiles-and-gate.md`, `docs/guide/proving-the-build.md`, and every ADR filename (e.g. `0010-finalize-merge-gate-split-agents.md`, `0019-global-config-fence-classification.md`). Link text may change; link targets may not.
- **Never edit point-in-time records or frozen fixtures:** `docs/changes/**`, `docs/results/**`, `docs/superpowers/**` (other than this plan), `docs/adrs/**`, `testdata/**`, any `**/testdata/**` path (except the regenerated `internal/harness/*/testdata/golden`), `internal/install/legacydata/**`, `docs/codex/fixtures/**`, `docs/reference/harness/fixtures/**`.
- **Do not write the ADR.** The parent records "Collision-free docket vocabulary" through `docket-adr`. No task touches `docs/adrs/`.
- **The embedded mirror is generated, never hand-edited.** After editing any file under `skills/`, `agents/`, `cursor-rules/`, or `.docket.example.yml`, run `go generate ./internal/assets/` from the worktree root and commit the regenerated `internal/assets/embedded/**` in the same commit (`TestEmbeddedMatchesAuthored` enforces this).
- **Harness goldens are regenerated, never hand-edited.** After editing an agent description or a cursor dispatch rule, and *after* `go generate`, run `go test -count=1 ./internal/harness/claude/ ./internal/harness/codex/ ./internal/harness/cursor/ ./internal/harness/opencode/ -update`. Then re-run the same packages without `-update` and confirm they pass.
- **Skill size budgets:** after editing any `skills/**` file, run `go test -count=1 ./internal/repoguard/ -run 'TestSkillSizeBudgets|TestDispatchBlockBudget'`. The renames are word-neutral or shorter, so a budget breach means an edit grew prose it should not have.
- **Keep an edit minimal.** Change the phrase and whatever grammar it forces, and nothing more. Do not re-flow untouched paragraphs. Re-wrap an edited line only when it becomes longer than the surrounding lines. The learning `phrase-grep-over-wrapped-prose` explains why.
- **Shell hygiene** (learning `agent-shell-noop-reads-as-success`): run every scan under an explicit `bash -c`, use `command git`, and judge a scan by the lines it prints, never by its exit status. A scan that prints nothing must be confirmed by its positive control first.
- **Staging:** `git add` explicit paths only, never `-A` or `.`. Commit messages end with `(change 0468)`.
- **Final gate:** the whole suite through `build.test_command`: `go run ./cmd/docket development test`, entered from the worktree.

### The scan helper (used by every task)

Every task finds its sites with this whitespace-tolerant, case-insensitive scan, which prints `file:line: match` for each hit. A phrase that wraps across a line break is still found. Paste it verbatim; do not shorten it.

```bash
bash -c '
cd /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e || exit 2
RE="$1"
command git ls-files -z -- . \
  ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" \
  ":!internal/assets/embedded" ":!testdata" ":!**/testdata/**" \
  ":!internal/install/legacydata" ":!docs/codex/fixtures" ":!docs/reference/harness/fixtures" \
  | RE="$RE" xargs -0 perl -0777 -ne '"'"'
      while (/$ENV{RE}/gi) {
        my $l = 1 + (substr($_, 0, $-[0]) =~ tr/\n//);
        (my $m = $&) =~ s/\s+/ /g;
        print "$ARGV:$l: $m\n";
      }'"'"'
' scan '<PERL-REGEX>'
```

Positive control: before trusting an empty result, run the helper once with `human\s+merge\s+gate`. At base it must print at least `docs/reference/glossary.md` and `skills/docket-implement-next/SKILL.md`.

## Review Focus

1. **A renamed glossary heading leaves a dangling in-page link.** For example, `#human-merge-gate`, `#suite-gate--test-gate`, or `#coordination-key--scope-tag--fence` may still be linked from the Gate entry, the Epoch fence entry, or the alphabetical index. A reader clicking the link expects to land on the entry. Pinned by the anchor checker in Task 1, Steps 1 and 6, and re-run in Task 7.
2. **A process-level "terminal" rewritten as "final".** For example, a gate run's "durable terminal record" or a "terminal disposition" in `skills/docket-build/**`, `internal/process`, or `internal/gatedrive`. A reader expects process vocabulary to be unchanged (Decision 6). Pinned by the explicit site lists in Tasks 5–6, the protected-token count check in Tasks 5–7, and Task 7's per-residual classification.
3. **A family-owned token renamed by accident.** Examples: a `terminal-notes-frozen` code, `fenced-setting-ignored`, `rearm` / re-arm, run gate, the `terminal_publish` key. The binary and the family changes expect these spellings unchanged until 0471–0474 land. Pinned by the protected-token count comparison against the base commit, run in Tasks 2–7.
4. **Authored and embedded copies drifting apart, or stale goldens.** This happens when an agent description or cursor rule changes without `go generate` and `-update`. `docket install` would then ship old wording. Pinned by `TestEmbeddedMatchesAuthored` and the golden tests, which run in each task that touches an authored root (Tasks 2–5) and again in the full suite (Task 7).
5. **A wrapped old phrase survives a line-oriented grep.** For example, `human merge\ngate`. Pinned by the whitespace-collapsing scan helper that every task and Task 7 use.

### Protected-token check (used by Tasks 2–7)

Compares match counts at the base commit with the working tree. Every listed token must keep its count.

```bash
bash -c '
cd /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e || exit 2
PS=(. ":!docs/changes" ":!docs/results" ":!docs/superpowers" ":!docs/adrs" ":!testdata" ":!**/testdata/**" ":!internal/install/legacydata" ":!docs/codex/fixtures" ":!docs/reference/harness/fixtures")
fail=0
for tok in terminal-backlink-pending terminal-notes-frozen change-terminal-claim-stamp drop-terminal-claimed-at not-terminal skipped-terminal adr-update-after-terminal fenced-setting-ignored terminal_publish rearm re-arm record-terminal "run gate" "epoch fence" "terminal half" "terminal publication"; do
  base=$(command git grep -i -F -o -e "$tok" 4b318461c4b0d28916c3acffc2add03f7272fff4 -- "${PS[@]}" | wc -l | tr -d " ")
  now=$(command git grep -i -F -o -e "$tok" -- "${PS[@]}" | wc -l | tr -d " ")
  if [ "$base" -ne "$now" ]; then echo "CHANGED $tok: $base -> $now"; fail=1; else echo "ok $tok $now"; fi
done
[ "$base" -gt 0 ] || echo "WARNING: last token counted zero at base — check the pathspec"
exit $fail
'
```

Expected: every line starts `ok` and the exit status is 0. `git grep` against the working tree counts tracked files with unstaged edits, so the check works before a commit too.

---

## File Structure

- `docs/reference/glossary.md`: every glossary change (Task 1 only; later tasks do not touch it).
- `docs/**` guides, concepts, install, reference, README pages; `README.md`; `tests/README.md`: prose renames (Tasks 2, 3, 5).
- `skills/**`, `agents/docket-implement-next.md`, `agents/docket-auto-groom.md`, `cursor-rules/dispatch/*.md`, `.docket.example.yml`: authored roots whose mirror is `internal/assets/embedded/**` (Tasks 2–5).
- `internal/harness/*/testdata/golden/*`: regenerated goldens (Tasks 3, 4).
- `.docket.yml`: this repo's own config comments (Task 5; not an authored root, no regeneration).
- Go comments and message strings in `internal/app`, `internal/domain`, `internal/render`, `internal/repository`, `internal/evidence`, `internal/reposetup`, plus the `tests/test_go_toolchain.sh` comment (Tasks 3, 6).

---

### Task 1: Glossary: rename rows 60–65 entries and add the "Obsolete terms" section

**Build profile:** standard

**Files:**
- Modify: `docs/reference/glossary.md`

**Interfaces:**
- Consumes: nothing.
- Produces: these glossary anchors, which later docs may link to: `#pr-handoff`, `#suite-gate`, `#finalize-gate` (unchanged), `#archived-record`, `#auto-groom--auto-groomable`, `#coordination-key--scope-tag--shared-setting-guard`, `#status-vs-the-merged-pr-sweep`, `#obsolete-terms`, `#runner-delegation`, `#runner-shim--runners-block` (unchanged), `#runtimebash`, `#terminal-publish`.

- [ ] **Step 1: Build the anchor checker and prove it on the untouched file (positive control)**

Save this as a scratch script. It is not committed, and no guard is added.

```bash
CHK="$(mktemp "${TMPDIR:-/tmp}/glossary-anchors.XXXXXX")"
cat > "$CHK" <<'EOF'
#!/usr/bin/env bash
# usage: glossary-anchors <file>  — prints DANGLING lines for in-page links with no heading.
perl -CSD -ne '
  if (/^#{1,6}\s+(.*?)\s*$/) { my $h = lc $1; $h =~ s/[^\p{L}\p{N}\s_-]//g; $h =~ s/ /-/g; $seen{$h} = 1 }
  while (/\]\(#([^)\s]+)\)/g) { push @refs, [$., $1] }
  END {
    my $bad = 0;
    for my $r (@refs) { unless ($seen{$r->[1]}) { print "DANGLING line $r->[0]: #$r->[1]\n"; $bad++ } }
    print "checked ", scalar(@refs), " in-page links, $bad dangling\n";
    exit($bad ? 1 : 0);
  }' "$1"
EOF
chmod +x "$CHK"; echo "$CHK"
bash "$CHK" /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e/docs/reference/glossary.md
```

Expected: `checked N in-page links, 0 dangling` with N > 200. Then mutation-test the checker. Copy the glossary to a backup (`cp docs/reference/glossary.md "$CHK.bak"`), change the heading `### Human merge gate` to `### Human merge gateX` in the working copy, and re-run. It must print `DANGLING` lines for `#human-merge-gate` and exit 1. Restore with `cp -f "$CHK.bak" docs/reference/glossary.md` and prove the restore with `cmp "$CHK.bak" docs/reference/glossary.md`.

- [ ] **Step 2: Record the failing state (the scan finds the old phrases)**

Run the scan helper, restricting its output to the glossary, once for each regex:
`coordination(-key)?[\s-]+fenc`, `merge[\s-]+gate`, `rebase-retest(\s+\w+)?\s+gate`, `\btest\s+gate`, `autonomous[\s-]+eligible`, `terminal\s+(status(es)?|records?|sweep|close-?out|transition|outcomes?)`, `(happy|sad)\s+terminal`, `non-terminal`.
Expected: hits in `docs/reference/glossary.md`. This is the red state. Also note the ones the edits below must leave in place: line ~1000 `Non-terminal:` (a run-gate verdict, process-level) and lines ~1133/1136 `exact terminal record` (native supervisor, process-level).

- [ ] **Step 3: Apply the entry edits**

Make exactly these edits, in file order.

1. **Jump list** (top of the file): after `13. [Operations and the CLI protocol](#operations-and-the-cli-protocol)`, add `14. [Obsolete terms](#obsolete-terms)`.

2. **Repository and branches → `### Terminal record / terminal publish`**: delete the whole entry, from the heading down to the line before `### Worktree / feature workspace`. Then insert this new entry between `### Adopting docket in a repository`'s entry and `### Bootstrap guard` (entries in each group are alphabetical):

   ```markdown
   ### Archived record

   A change's archived file (plus its results) once it reaches a final status, `done` or `killed`. It
   stays on the metadata branch; the integration branch gets code, plans, and results through PRs alone.

   **Used for:** historical browsing. `archive/` on the metadata branch keeps every closed change.
   ```

3. **Backlog**: `it means the non-terminal changes in` → `it means the changes not yet in a final status, in`.

4. **Change**: `terminal ones in` → `final ones (\`done\`, \`killed\`) in`.

5. **Change lifecycle and statuses intro**: after the `Statuses: …` paragraph (ending `(closed vocabulary \`statuses\` in \`docket schema\`).`), add a blank line and then: `` `done` and `killed` are the two **final statuses**; every other status is non-final.``

6. **Lifecycle table**:
   - `| Built, PR open — the human merge gate |` → `| Built, PR open — the PR handoff |`
   - `(happy terminal)` → `(happy final status)`
   - `(sad terminal)` → `(sad final status)`

7. **Archive**: `a terminal transition (\`done\` or \`killed\`)` → `a final transition (\`done\` or \`killed\`)`.

8. **`### Human merge gate`**: delete the whole entry (heading through its `**Used for:**` paragraph). Insert this entry between the `### Owned sections / section intents …` entry and `### Readiness: build-ready / needs-brainstorm / not-proposed`:

   ```markdown
   ### PR handoff

   The stop at `implemented`: implement-next opens the pull request and ends there. It never merges; a human (or a
   finalize run they start) takes the change the rest of the way. It is a handoff to a person, not a
   [gate](#gate): no suite runs at this stop.

   **Used for:** keeping merge a human decision. After review, the next step is `docket-finalize-change`.
   ```

9. **`### Auto-groom / auto-groomable / autonomous-eligible`** → heading `### Auto-groom / auto-groomable`. Replace the body's first paragraph with:

   ```markdown
   **Auto-groom** grooms stubs with no human, gated by an adversarial **critic**. A stub's **effective
   auto-groomable** value is its `auto_groomable:` override when set, or else the repo's `auto_groom`
   knob. A stub is **auto-groomable** (selectable by auto-groom) when it is needs-brainstorm *and* that
   effective value is `true`.
   ```

   Keep the `**Used for:**` paragraph unchanged.

10. **`### Gate`** list:
    - `- [Finalize gate](#finalize-gate) — the rebase-retest (merge) gate that re-runs the suite before merging.` → `- [Finalize gate](#finalize-gate) — re-runs the whole suite on the rebased branch before merging.`
    - Delete the bullet `- [Human merge gate](#human-merge-gate) — the \`implemented\` stop where a person merges.`
    - The closing paragraph `The build and finalize gates both run the whole suite. The page that covers them is\n[Suite gate / test gate](#suite-gate--test-gate).` becomes:

      ```markdown
      The build and finalize gates both run the whole suite. The page that covers them is
      [Suite gate](#suite-gate). The `implemented` stop where a person merges is the
      [PR handoff](#pr-handoff), not a gate.
      ```

11. **Suite command entry**: `finalize's merge gate` → `the finalize gate`.

12. **`### Suite gate / test gate`** → `### Suite gate` (body unchanged).

13. **`### Epoch fence`**: the `**Used for:**` paragraph `making a cancel stick while teardown finishes. This is not the config\n[coordination fence](#coordination-key--scope-tag--fence), which is a different mechanism.` becomes `**Used for:** making a cancel stick while teardown finishes.` Leave the heading and every other line of the entry alone; family (a) owns them.

14. **Closeout**: `**Closeout** is the terminal transition that archives` → `**Closeout** is the final transition that archives`.

15. **`### Finalize gate`**: the first paragraph's opening `How finalize validates the rebased branch before merging; the guide and skills also call it the\n**rebase-retest gate** or the **merge gate**. \`finalize.gate\` is` → `How finalize validates the rebased branch before merging: it rebases onto the integration branch and\nre-runs the suite. \`finalize.gate\` is`. Keep the rest of the paragraph.

16. **`### Policy gate (\`require_pr_approval\`)`**: replace the phrase `rebase-retest correctness gate` with `[finalize gate](#finalize-gate)`. Keep the sentence grammatical (read it whole first).

17. **`### Status vs the terminal sweep`** → `### Status vs the merged-PR sweep`. In its body, `The **terminal sweep** is close-out` → `The **merged-PR sweep** is close-out`, and in its code block `# the terminal sweep (writes)` → `# the merged-PR sweep (writes)`.

18. **docket-auto-groom** entry: `It drains every autonomous-eligible stub` → `It drains every auto-groomable stub`.

19. **`### Runner / delegation`**, **`### Runner shim / \`runners\` block`**: cut both entries from *Skills, agents, and harnesses* (they move in item 22).

20. **`### Coordination key / scope tag / fence`** → `### Coordination key / scope tag / shared-setting guard`. In the body, `The **fence** ignores (with a warning)` → `The **shared-setting guard** ignores (with a warning)`.

21. **`### \`finalize.skip_results_only_delta\`**: `It is repo-only (coordination-fenced)` → `It is repo-only (shared-setting guarded)`. **GitHub board mirror**: `only its\ncoordination fence runs` → `only its\nshared-setting guard runs`.

22. **`### \`runtime.bash\` (obsolete)`**: cut the entry from *Configuration*. Then insert a new group immediately before the `## Alphabetical index` heading. Put a `---` separator line and a blank line before the new group, matching how the other groups are separated:

    ```markdown
    ## Obsolete terms

    Retired features. Their config keys are still recognised, so a stale file gets a warning or a refusal
    instead of being silently accepted; nothing in current docket uses them.

    ### Runner delegation

    <the body of the old "Runner / delegation" entry, verbatim>

    ### Runner shim / `runners` block

    <the body of the old "Runner shim / `runners` block" entry, verbatim>

    ### `runtime.bash`

    <the body of the old "`runtime.bash` (obsolete)" entry, verbatim>

    ### Terminal publish

    Terminal publish (also called **selective publish on close-out**) was the opt-in copying of archived
    records onto the integration branch. It is deferred from Go v1: `terminal_publish: false` is inert,
    and `true` blocks every repository mutation until you remove it.

    **Used for:** nothing in Go v1. The integration branch gets code, plans, and results through PRs alone.
    ```

    "Verbatim" means moving the bodies byte-for-byte. The old Runner delegation body's link `[Runner shim / \`runners\` block](#runner-shim--runners-block)` keeps resolving because that heading is unchanged.

- [ ] **Step 4: Update the alphabetical index**

In `## Alphabetical index`, keep strict alphabetical order (the existing list's case-insensitive order):
- add `- [Archived record](#archived-record)` directly after `- [Archive](#archive)`;
- `- [Auto-groom / auto-groomable / autonomous-eligible](#auto-groom--auto-groomable--autonomous-eligible)` → `- [Auto-groom / auto-groomable](#auto-groom--auto-groomable)`;
- `- [Coordination key / scope tag / fence](#coordination-key--scope-tag--fence)` → `- [Coordination key / scope tag / shared-setting guard](#coordination-key--scope-tag--shared-setting-guard)`;
- `- [Effective auto-groomable](#auto-groom--auto-groomable--autonomous-eligible) — see Auto-groom / auto-groomable / autonomous-eligible` → `- [Effective auto-groomable](#auto-groom--auto-groomable) — see Auto-groom / auto-groomable`;
- delete `- [Human merge gate](#human-merge-gate)`, `- [Merge gate](#finalize-gate) — see Finalize gate`, and `- [Rebase-retest gate](#finalize-gate) — see Finalize gate` (spec Decision 10: no old→new mapping in the glossary);
- add `- [PR handoff](#pr-handoff)` in alphabetical position (next to the existing `PR publish` line);
- `- [Runner / delegation](#runner--delegation)` → `- [Runner delegation](#runner-delegation)`;
- `- [runtime.bash (obsolete)](#runtimebash-obsolete)` → `- [runtime.bash](#runtimebash)`;
- `- [Selective publish](#terminal-record--terminal-publish) — see Terminal record / terminal publish` → `- [Selective publish](#terminal-publish) — see Terminal publish`;
- `- [Status vs the terminal sweep](#status-vs-the-terminal-sweep)` → `- [Status vs the merged-PR sweep](#status-vs-the-merged-pr-sweep)`;
- `- [Suite gate / test gate](#suite-gate--test-gate)` → `- [Suite gate](#suite-gate)`;
- `- [Terminal record / terminal publish](#terminal-record--terminal-publish)` → `- [Terminal publish](#terminal-publish)`.

- [ ] **Step 5: Re-run the Step 2 scans on the glossary**

Expected output, and only this:
- `terminal\s+(status…)` still hits the process-level lines ~1133/1136 (`exact terminal record`, `the terminal record is written`), which stay.
- `non-terminal` still hits the run-gate verdict row (`Non-terminal: the same attempt…`), which stays.
- Every other regex prints nothing for `docs/reference/glossary.md`.

Also run `grep -n -E "^## " docs/reference/glossary.md`. `## Obsolete terms` must appear directly before `## Alphabetical index`. `grep -c "^### Runner delegation$\|^### Terminal publish$\|^### \`runtime.bash\`$\|^### PR handoff$\|^### Archived record$\|^### Suite gate$" docs/reference/glossary.md` must print `6`.

- [ ] **Step 6: Anchor check (green)**

Run: `bash "$CHK" /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e/docs/reference/glossary.md`
Expected: `… 0 dangling`, exit 0.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e
command git add docs/reference/glossary.md
command git commit -m "docs(glossary): rename colliding terms and add Obsolete terms section (change 0468)"
```

---

### Task 2: Row 60, coordination-key fence → shared-setting guard

**Build profile:** economy

**Files:**
- Modify: `docs/concepts/config-layers.md`, `docs/install/config-layers.md`, `docs/README.md`, `docs/concepts/README.md`, `docs/install/README.md`, `docs/comparison/ai-native-sdlc-playbook.md`, `skills/docket-convention/SKILL.md`, `.docket.example.yml`
- Regenerate: `internal/assets/embedded/**`

**Interfaces:**
- Consumes: the Task 1 glossary heading `Coordination key / scope tag / shared-setting guard` (for wording consistency only; no link is added).
- Produces: nothing later tasks use.

- [ ] **Step 1: Red scan**

Run the scan helper with `coordination(-key)?[\s-]+fenc` and then with `\bfenc(e|ed|es)\b`.
Expected: the first lists the `coordination…fence(d)` sites; the second additionally lists bare config-sense "fence"/"fenced" in the files above. Hits under `internal/config/**` are out of scope (Global Constraints). The run fence in `AGENTS.md`, `CLAUDE.md`, `cursor-rules/run-gate.md`, and the glossary's `Epoch fence` / `Cancel` entries is also out of scope. So are markdown code fences (`fenced code`, `fenced recipe`, `inside a fence`).

- [ ] **Step 2: Edit the sites**

Rule: every "fence"/"fenced" that means the config mechanism which ignores a coordination key set outside the committed layer becomes **shared-setting guard** (noun) or **shared-setting guarded** / **guarded** (adjective). ADR link *targets* keep their filenames.

- `docs/concepts/config-layers.md`: title `# Config layers and the coordination fence` → `# Config layers and the shared-setting guard`. Then:
  - `draws a fence through them` → `draws a guard through them`
  - `fence forbids it` → `guard forbids it`
  - The ASCII diagram label `┌───────────────── the fence ──────────────────┐` → `┌──────────── the shared-setting guard ────────────┐`. Keep the total width equal to the original line's by trimming `─` characters, and check the box's closing lines still align.
  - `The fenced keys today are` → `The guarded keys today are`
  - `the layering and the fence are always` → `the layering and the guard are always`
  - `coordination-key fence classification rule` → `shared-setting guard classification rule`
- `docs/install/config-layers.md`: `## The coordination fence` → `## The shared-setting guard`; `The fenced keys are` → `The guarded keys are`; the `fence — why a per-clone value…` sentence's `fence` → `guard`; link text `[Config layers and the coordination fence]` → `[Config layers and the shared-setting guard]`.
- `docs/README.md`: `the coordination fence` → `the shared-setting guard`. Link text `[Config layers and the coordination fence]` → `[Config layers and the shared-setting guard]`, and its continuation `and the fence that keeps a shared setting from being overridden` → `and the guard that keeps a shared setting from being overridden`.
- `docs/concepts/README.md`: the same two edits as `docs/README.md`'s link line.
- `docs/install/README.md`: `the coordination fence` → `the shared-setting guard`.
- `docs/comparison/ai-native-sdlc-playbook.md`: `coordination-key fence` → `shared-setting guard`; `Projects v2 fenced but unwired` → `Projects v2 guarded but unwired`; `` `terminal_publish`; parseable, fenced, inert. `` → `` `terminal_publish`; parseable, guarded, inert. ``; `Layers and the fence` → `Layers and the shared-setting guard`.
- `skills/docket-convention/SKILL.md`:
  - the `terminal_publish` comment's `Per-repo-only (coordination-key fenced)` → `Per-repo-only (shared-setting guarded)`
  - the `skip_results_only_delta` comment's `Per-repo-only (fenced)` → `Per-repo-only (shared-setting guarded)`. If that pushes the comment past the block's existing longest line, use `(guarded)` instead.
  - the Config layers paragraph's `**Coordination-key fence:**` → `**Shared-setting guard:**`, and `Which keys are fenced` → `Which keys are guarded`
  - the close-out paragraph's `parseable and coordination-fenced` → `parseable and shared-setting guarded`
- `.docket.example.yml`:
  - every `(coordination-fenced, ADR-0019)` → `(shared-setting guarded, ADR-0019)`
  - `coordination-fenced and warned-and-ignored there` → `shared-setting guarded and warned-and-ignored there`
  - both `coordination-key fence` → `shared-setting guard`
  - `enforced by the fence` → `enforced by the guard`
  - `coordination-fenced configuration value` → `shared-setting guarded configuration value`
  - Do not touch any `# scope: <tag>` prefix. `internal/config/example_correspondence_test.go`'s `scopeTagRe` keys on `# scope: repo-only|any layer|local-only`.

- [ ] **Step 3: Regenerate the mirror and verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e
go generate ./internal/assets/
go test -count=1 ./internal/assets/ ./internal/config/ ./internal/repoguard/
```

Expected: PASS. Re-run the Step 1 scans. The only remaining `coordination…fenc` hits must be under `internal/config/**`. Remaining bare `fence` hits must all be run-fence or markdown-code-fence senses. Run the protected-token check: all `ok`.

- [ ] **Step 4: Commit**

```bash
command git add docs/concepts/config-layers.md docs/install/config-layers.md docs/README.md docs/concepts/README.md docs/install/README.md docs/comparison/ai-native-sdlc-playbook.md skills/docket-convention/SKILL.md .docket.example.yml internal/assets/embedded
command git commit -m "docs: call the coordination-key fence the shared-setting guard (change 0468)"
```

---

### Task 3: Rows 61–63: PR handoff, and drop the "merge gate" / "rebase-retest gate" / "test gate" aliases

**Build profile:** standard

**Files:**
- Modify: `README.md`, `agents/docket-implement-next.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-implement-next/references/fix-loop.md`, `cursor-rules/dispatch/docket-implement-next.md`, `cursor-rules/dispatch/docket-finalize-change.md`, `skills/docket-convention/SKILL.md`, `skills/docket-convention/references/stacked-changes.md`, `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-execution.md`, `.docket.example.yml`, `docs/README.md`, `docs/guide/README.md`, `docs/concepts/README.md`, `docs/concepts/build-profiles-and-gate.md`, `docs/guide/building-without-supervision.md`, `docs/guide/capturing-work.md`, `docs/guide/landing-changes.md`, `docs/guide/proving-the-build.md`, `docs/guide/reviewing-before-the-human.md`, `docs/reference/harness/validation.md`, `docs/comparison/ai-native-sdlc-playbook.md`, `tests/README.md`, `tests/test_go_toolchain.sh`, `internal/evidence/record.go`, `internal/reposetup/health.go`, `internal/app/named_isolation_integration_test.go`
- Regenerate: `internal/assets/embedded/**`, `internal/harness/{claude,codex,cursor,opencode}/testdata/golden/*`

**Interfaces:**
- Consumes: glossary anchors `#pr-handoff`, `#finalize-gate`, `#suite-gate` (Task 1). No new links are required.
- Produces: the `docket-implement-next` description text `…reviewing, and stopping at the PR handoff.…`, identical in `agents/docket-implement-next.md` and `skills/docket-implement-next/SKILL.md`.

- [ ] **Step 1: Red scan**

Run the scan helper with `merge[\s-]+gate`, `rebase-retest(\s+\w+)?\s+gate`, and `\btest\s+gate`.
Expected: the sites below plus the glossary's now-clean lines (nothing in the glossary after Task 1). Two hits are **not** edited: `internal/app/finalize_merge_test.go` `merge gates on` (the verb "gates"), and ADR link targets `0010-finalize-merge-gate-split-agents.md`.

- [ ] **Step 2: Apply the edits**

Decide each site by meaning. The `implemented` stop where a human merges → **PR handoff**. Finalize's rebase-and-retest before merging → **finalize gate**. A whole-suite run → **suite gate**. `build.gate: off` → **build gate**.

- `README.md`: `**You stay at the merge gate.**` → `**You own the PR handoff.**`; `unattended\n  repair at the merge gate blocks` → `unattended\n  repair at the finalize gate blocks`.
- `agents/docket-implement-next.md` and `skills/docket-implement-next/SKILL.md` frontmatter `description:`: `stopping at the human merge gate.` → `stopping at the PR handoff.` The two descriptions must stay byte-identical; diff them after editing.
- `skills/docket-implement-next/SKILL.md` body: `then stops at the human merge gate.` → `then stops at the PR handoff.`
- `skills/docket-implement-next/references/fix-loop.md`: `The human's merge gate does not move:` → `The PR handoff does not move:`
- `cursor-rules/dispatch/docket-implement-next.md`: `stops at the human merge gate.` → `stops at the PR handoff.`; the example prompt `stop at the merge gate.")` → `stop at the PR handoff.")`
- `cursor-rules/dispatch/docket-finalize-change.md`: `through the\nrebase-retest gate,` → `through the\nfinalize gate,`
- `skills/docket-convention/SKILL.md`:
  - the `finalize:` config-block comment `# merge gate: rebase onto base + re-test before merge` → `# finalize gate: rebase onto base + re-test before merge`
  - `**\`finalize\` — the rebase-retest merge gate.**` → `**\`finalize\` — the finalize gate.**`
  - the status table `built, PR open — **human merge gate**` → `built, PR open — **PR handoff**`
- `skills/docket-convention/references/stacked-changes.md`: `\`open_child_prs\` is the open subset the merge gate keys on.` → `\`open_child_prs\` is the open subset finalize's merge step keys on.`
- `skills/docket-build/SKILL.md`: `the repo declares no build test gate.` → `the repo declares no build gate.`
- `skills/docket-build/references/gate-execution.md`: heading suffix `(pre-merge gate)` → `(before merge)`.
- `.docket.example.yml`:
  - banner `# ═══ finalize — the merge gate ═══…` → `# ═══ finalize — the finalize gate ═══…`, trimming `═` so the banner keeps its original total width
  - `# The finalize merge gate (change 0015):` → `# The finalize gate (change 0015):`
  - `the suite finalize's merge gate runs before it merges` → `the suite the finalize gate runs before it merges`
  - `declares it has no build test gate` → `declares it has no build gate`
- `docs/README.md`, `docs/guide/README.md`: link text `[Test gate: Proving the build]` → `[Suite gate: Proving the build]`. `docs/README.md` and `docs/concepts/README.md`: link text `[Build profiles and the test gate]` → `[Build profiles and the suite gate]`.
- `docs/concepts/build-profiles-and-gate.md`: title `# Build profiles and the test gate` → `# Build profiles and the suite gate` ("profiles" stays; 0473 owns it).
- `docs/guide/proving-the-build.md`:
  - title `# Test gate: Proving the build` → `# Suite gate: Proving the build`
  - `has no build test gate` → `has no build gate`
  - `finalize's merge gate runs before it merges` → `the finalize gate runs before it merges`
  - `on/off switch for the merge gate` → `on/off switch for the finalize gate`
- `docs/guide/building-without-supervision.md`:
  - `stops at the human merge gate. It never merges.` → `stops at the PR handoff. It never merges.`
  - `the one invariant the driver never breaks is the merge gate:` → `… is the PR handoff:`
  - link text `[Build profiles and the test gate]` → `[Build profiles and the suite gate]`
- `docs/guide/capturing-work.md`: `chains serialize on the merge gate.` → `chains serialize on the PR handoff.`
- `docs/guide/landing-changes.md`:
  - `(the merge gate)` → `(the finalize gate)`
  - `the on/off switch for that merge gate` → `the on/off switch for that finalize gate`
  - `passes the rebase-retest gate` → `passes the finalize gate`
  - `A change whose merge gate fails` → `A change whose finalize gate fails`
  - `runs its rebase-retest gate` → `runs its finalize gate`
- `docs/guide/reviewing-before-the-human.md`: `so the merge gate does not move.` → `so the PR handoff does not move.`; link text `[Build profiles and the test gate]` → `[Build profiles and the suite gate]`.
- `docs/reference/harness/validation.md`:
  - `## The merge-gate obligation` → `## The PR-handoff obligation`
  - `does not clear the human merge gate on its own` → `does not clear the PR handoff on its own`
  - `the human at the\nmerge gate knows` → `the human at the\nPR handoff knows`
- `docs/comparison/ai-native-sdlc-playbook.md`: table cell `| Human merge gate |` → `| PR handoff |`.
- `tests/README.md`: `what the merge gate runs (change 0318)` → `what the finalize gate runs (change 0318)`.
- `tests/test_go_toolchain.sh`: in the comment near `# merge gate and fails it outright offline.`, replace `merge gate` with `finalize gate`. Read the surrounding comment lines first so the sentence still parses. Comment only.
- `internal/evidence/record.go`: comment `explicitly disabled its build test gate (build.gate: off)` → `explicitly disabled its build gate (build.gate: off)`.
- `internal/reposetup/health.go`: doc comment `a local test gate cannot run` → `a local suite gate cannot run`; the `Message:` string `"A local test gate has no configured command …"` → `"A local suite gate has no configured command …"` (the rest of the string unchanged).
- `internal/app/named_isolation_integration_test.go`: comment `so a merge gate that validated the whole corpus` → `so a finalize gate that validated the whole corpus`.

- [ ] **Step 3: Check that no test pins the changed message**

Run: `command git grep -n -F "local test gate" -- '*_test.go' tests/`
Expected: no output. If a test does pin it, update that assert to the new spelling in this same task and name it in the commit body.

- [ ] **Step 4: Regenerate the mirror and goldens, then verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e
go generate ./internal/assets/
go test -count=1 ./internal/harness/claude/ ./internal/harness/codex/ ./internal/harness/cursor/ ./internal/harness/opencode/ -update
go test -count=1 ./internal/harness/... ./internal/assets/ ./internal/reposetup/ ./internal/evidence/ ./internal/repoguard/
gofmt -l internal/evidence/record.go internal/reposetup/health.go internal/app/named_isolation_integration_test.go
go vet -tags integration ./internal/app/
```

Expected: tests PASS, `gofmt -l` prints nothing, vet is clean. Then run `command git status --short internal/harness`. Only golden files whose rendered content includes the implement-next description or the cursor dispatch rules may change. Open each changed golden's diff: it must differ only by the renamed phrase.

- [ ] **Step 5: Green scan and protected tokens**

Re-run the Step 1 scans. The only residuals allowed are the `finalize_merge_test.go` verb and ADR filename link targets. Run the protected-token check: all `ok`.

- [ ] **Step 6: Commit**

```bash
command git add README.md agents/docket-implement-next.md skills/docket-implement-next/SKILL.md skills/docket-implement-next/references/fix-loop.md cursor-rules/dispatch/docket-implement-next.md cursor-rules/dispatch/docket-finalize-change.md skills/docket-convention/SKILL.md skills/docket-convention/references/stacked-changes.md skills/docket-build/SKILL.md skills/docket-build/references/gate-execution.md .docket.example.yml docs/README.md docs/guide/README.md docs/concepts/README.md docs/concepts/build-profiles-and-gate.md docs/guide/building-without-supervision.md docs/guide/capturing-work.md docs/guide/landing-changes.md docs/guide/proving-the-build.md docs/guide/reviewing-before-the-human.md docs/reference/harness/validation.md docs/comparison/ai-native-sdlc-playbook.md tests/README.md tests/test_go_toolchain.sh internal/evidence/record.go internal/reposetup/health.go internal/app/named_isolation_integration_test.go internal/assets/embedded internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden
command git commit -m "docs: PR handoff; drop merge/rebase-retest/test gate aliases (change 0468)"
```

---

### Task 4: Row 65: fold autonomous-eligible into auto-groomable

**Build profile:** economy

**Files:**
- Modify: `agents/docket-auto-groom.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-convention/SKILL.md`
- Regenerate: `internal/assets/embedded/**`, `internal/harness/{claude,codex,cursor,opencode}/testdata/golden/*`

**Interfaces:**
- Consumes: the Task 1 glossary definition. An **auto-groomable** stub is needs-brainstorm *and* has an effective `auto_groomable` value of `true`.
- Produces: the `docket-auto-groom` description text `…selecting each auto-groomable stub deterministically…`, identical in the agent and the skill.

- [ ] **Step 1: Red scan**

Run the scan helper with `autonomous[\s-]+eligible`.
Expected: 1 hit in `agents/docket-auto-groom.md`, 7 in `skills/docket-auto-groom/SKILL.md`, 1 in `skills/docket-convention/SKILL.md` (the glossary is already clean after Task 1).

- [ ] **Step 2: Edit**

- `agents/docket-auto-groom.md` and `skills/docket-auto-groom/SKILL.md` `description:`: `selecting each autonomous-eligible stub deterministically` → `selecting each auto-groomable stub deterministically`. The two descriptions stay byte-identical.
- `skills/docket-auto-groom/SKILL.md` body. Every remaining `autonomous-eligible` → `auto-groomable`, with these adjustments:
  - In the vocabulary list `(needs-brainstorm, effective auto-groomable, autonomous-eligible, the abstain rule, …)`, drop the now-duplicate item. The result is `(needs-brainstorm, effective auto-groomable, auto-groomable, the abstain rule, …)`.
  - In `Rank every **autonomous-eligible** stub (per the convention: needs-brainstorm AND effective auto-groomable; …`, the result is `Rank every **auto-groomable** stub (per the convention: needs-brainstorm AND an effective \`auto_groomable\` value of true; …`. The rest of the parenthetical is unchanged.
- `skills/docket-convention/SKILL.md`: `A stub is **autonomous-eligible** — selectable by \`docket-auto-groom\` — when it is needs-brainstorm (\`proposed\`, no \`spec:\`, not \`trivial: true\`) AND effective auto-groomable.` → `A stub is **auto-groomable** — selectable by \`docket-auto-groom\` — when it is needs-brainstorm (\`proposed\`, no \`spec:\`, not \`trivial: true\`) AND its effective auto-groomable value is \`true\`.`

- [ ] **Step 3: Regenerate and verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e
go generate ./internal/assets/
go test -count=1 ./internal/harness/claude/ ./internal/harness/codex/ ./internal/harness/cursor/ ./internal/harness/opencode/ -update
go test -count=1 ./internal/harness/... ./internal/assets/ ./internal/repoguard/
```

Expected: PASS. Changed goldens differ only by the description phrase. Re-run the Step 1 scan: no output (verify the helper with its positive control first). Run the protected-token check: all `ok`. `rearm` and `re-arm` must be unchanged.

- [ ] **Step 4: Commit**

```bash
command git add agents/docket-auto-groom.md skills/docket-auto-groom/SKILL.md skills/docket-convention/SKILL.md internal/assets/embedded internal/harness/claude/testdata/golden internal/harness/codex/testdata/golden internal/harness/cursor/testdata/golden internal/harness/opencode/testdata/golden
command git commit -m "docs(auto-groom): fold autonomous-eligible into auto-groomable (change 0468)"
```

---

### Task 5: Row 64 in docs, skills and config: final status / archived record / merged-PR sweep / close-out

**Build profile:** standard

**Files:**
- Modify: `docs/concepts/finalize-sequencer.md`, `docs/concepts/two-branches.md`, `docs/guide/keeping-the-backlog-honest.md`, `docs/guide/landing-changes.md`, `docs/guide/where-the-metadata-lives.md`, `docs/install/cursor.md`, `.docket.example.yml`, `.docket.yml`, `skills/docket-convention/SKILL.md`, `skills/docket-convention/references/stacked-changes.md`, `skills/docket-convention/references/terminal-close-out.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-implement-next/references/edge-paths.md`, `skills/docket-new-change/SKILL.md`
- Regenerate: `internal/assets/embedded/**`

**Interfaces:**
- Consumes: glossary terms "final status", "archived record", "merged-PR sweep" (Task 1).
- Produces: nothing later tasks use.

- [ ] **Step 1: Red scan**

Run the scan helper with `terminal\s+(status(es)?|records?|sweep|close-?out|transition|outcomes?|state|change|shape|transactions?|artifacts)`, then `(happy|sad)\s+terminal`, then `non-terminal`, restricting attention to `docs/`, `skills/`, `.docket.example.yml`, `.docket.yml`.
Expected: the lifecycle sites below and some process-level sites. Leave the process-level ones untouched: everything in `skills/docket-build/**`, `skills/docket-build-task/**`, and `skills/docket-status/SKILL.md`'s sweep "terminal result/envelope/disposition"; the `## Terminal disposition (driver contract)` headings; `docs/concepts/run-gate.md`'s ADR-0095 link and text; `.docket.example.yml`'s observation-budget "terminal result" lines; the glossary.

- [ ] **Step 2: Edit the lifecycle-sense sites**

Keep every `terminal publish` / `terminal publication` / `terminal-publish`, every `terminal_publish`, every `terminal half`, and the path `references/terminal-close-out.md` unchanged (Global Constraints).

- `docs/concepts/finalize-sequencer.md`: `Archiving copies the terminal record` → `Archiving copies the archived record`; `an expected terminal record is never lost silently` → `an expected archived record is never lost silently`.
- `docs/concepts/two-branches.md`:
  - the diagram's `terminal record copied ──►` → `archived record copied ──►` (the same character count, so alignment holds; verify visually)
  - each prose `terminal record(s)` → `archived record(s)`
  - leave `a deferred terminal publish` unchanged
- `docs/guide/keeping-the-backlog-honest.md`: `## Status versus the terminal sweep` → `## Status versus the merged-PR sweep`; `The **terminal sweep** is close-out.` → `The **merged-PR sweep** is close-out.`; `terminal records stay there` → `archived records stay there`.
- `docs/guide/landing-changes.md`: `On a **terminal transition** —` → `On a **final transition** —`.
- `docs/guide/where-the-metadata-lives.md`: `reaches a terminal state` → `reaches a final status`; `*is* the terminal record` → `*is* the archived record`; `keeping terminal records` → `keeping archived records`. Leave the `terminal-publish copy` table cells and the `no terminal-publish copy` phrase.
- `docs/install/cursor.md`: `publishes terminal records onto the` → `publishes archived records onto the`. Leave `terminal-publish's direct push`.
- `.docket.example.yml`: `published terminal records` → `published archived records`; `Terminal records stay on the metadata branch` → `Archived records stay on the metadata branch`.
- `.docket.yml` (comments only): both `terminal records` → `archived records`.
- `skills/docket-convention/SKILL.md`:
  - `terminal_publish` comment: `# terminal records stay on the metadata branch.` → `# archived records stay on the metadata branch.`
  - Directory tree: `# every NON-terminal change:   ` → `# every NON-final change:      `, and `# the two terminal outcomes:    ` → `# the two final outcomes:       `. Pad with spaces so the `<id>-<slug>.md` / `<YYYY-MM-DD>-…` column stays aligned.
  - `## Closeout notes` bullet: `terminal-only` → `final-only`; `of a terminal record` → `of an archived record`.
  - `## Publish deferred` bullet: `a terminal close-out's publish step` → `a close-out's publish step`.
  - Status table: `(happy terminal)` → `(happy final status)`; `(sad terminal)` → `(sad final status)`.
  - **Rules** paragraph:
    - `every non-terminal status` → `every non-final status`
    - `the two terminal outcomes` → `the two final outcomes`
    - `is non-terminal for that reason` → `is non-final for that reason`
    - `on the terminal transition` → `on the final transition`
    - `no-op if already terminal` → `no-op if already final`
  - Close-out paragraph:
    - `On a terminal transition` → `On a final transition`
    - `the shared **terminal close-out** sequence` → `the shared **close-out** sequence`
    - `before driving any terminal transition` → `before driving any final transition`
    - `The terminal close-out itself` → `The close-out itself`
    - Keep `**Terminal publication is deferred from Go v1**` and the link path.
  - Keep `at terminal publish` in the frozen-records paragraph (it names the retired feature).
- `skills/docket-convention/references/stacked-changes.md`: `the sixth **active**, non-terminal status` → `the sixth **active**, non-final status`. In the next sentence, `no terminal record is published — there is no terminal …` becomes `no archived record is published — there is no final …`: read the full sentence and keep its meaning. Leave `Terminal publication of stacked descendants`.
- `skills/docket-convention/references/terminal-close-out.md` (the filename stays):
  - `# Terminal close-out — the shared per-change sequence` → `# Close-out — the shared per-change sequence`
  - `a terminal transition (\`done\` or \`killed\`)` → `a final transition (\`done\` or \`killed\`)`
  - `The two terminal outcomes split here` → `The two final outcomes split here`
  - both `Both terminal transactions` → `Both close-out transactions`
  - `at terminal state` → `at final state`
  - `no terminal record is` → `no archived record is`
  - `*published* terminal artifacts` → `*published* archived artifacts`
  - `proven owned by *this* terminal change` → `proven owned by *this* closed change`
  - `the same terminal transition` → `the same final transition`
  - Keep every `Terminal publication` / `terminal publication` phrase and the `3. **Terminal publication (deferred).**` step title.
- `skills/docket-finalize-change/SKILL.md`:
  - `the terminal record's \`## Closeout` → `the archived record's \`## Closeout`
  - heading `### 9. Closeout — archive the terminal records` → `### 9. Closeout — archive the records`
  - `the promised terminal state already exists` → `the promised final state already exists`
  - `revives a terminal change` → `revives a closed change`
  - Leave `terminal half` and `## Terminal disposition (driver contract)`.
- `skills/docket-implement-next/SKILL.md`: `via the convention's terminal close-out (` → `via the convention's close-out (`. Keep the path and the `### Terminal disposition` heading.
- `skills/docket-implement-next/references/edge-paths.md`: `applies the one verified terminal shape` → `applies the one verified final shape`; `The convention's terminal close-out reference` → `The convention's close-out reference`. Keep `Terminal publication is deferred`.
- `skills/docket-new-change/SKILL.md`: `the \`killed\` terminal state` → `the \`killed\` final status`. Keep the `references/terminal-close-out.md` path.

Grep the whole repo for heading-anchor links to the two changed headings before committing: `command git grep -n -e "#status-versus-the-terminal-sweep" -e "#9-closeout" -e "#terminal-close-out" -- . ':!docs/changes' ':!docs/results' ':!docs/superpowers' ':!docs/adrs'`. Expected: no output. Fix any hit by retargeting its anchor.

- [ ] **Step 3: Regenerate and verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e
go generate ./internal/assets/
go test -count=1 ./internal/assets/ ./internal/repoguard/ ./internal/config/
```

Expected: PASS. `TestSkillSizeBudgets` covers `docket-convention/references/terminal-close-out.md`. Re-run the Step 1 scans over `docs/`, `skills/`, `.docket.example.yml`, `.docket.yml`. Every remaining hit must be process-level (per Step 1's list) or a retired-feature name. List the remaining hits in the task report with one word of classification each. Run the protected-token check: all `ok` (in particular `terminal half`, `terminal publication`, `terminal_publish`).

- [ ] **Step 4: Commit**

```bash
command git add docs/concepts/finalize-sequencer.md docs/concepts/two-branches.md docs/guide/keeping-the-backlog-honest.md docs/guide/landing-changes.md docs/guide/where-the-metadata-lives.md docs/install/cursor.md .docket.example.yml .docket.yml skills/docket-convention/SKILL.md skills/docket-convention/references/stacked-changes.md skills/docket-convention/references/terminal-close-out.md skills/docket-finalize-change/SKILL.md skills/docket-implement-next/SKILL.md skills/docket-implement-next/references/edge-paths.md skills/docket-new-change/SKILL.md internal/assets/embedded
command git commit -m "docs: final status, archived record, merged-PR sweep, close-out (change 0468)"
```

---

### Task 6: Row 64 in Go comments and messages (change-lifecycle sense only)

**Build profile:** economy

**Files:**
- Modify (comments and message strings only; no identifier, code token, test name, or subtest-name string changes):
  - Non-test source: `internal/app/finalize_closeout.go`, `internal/app/finalize_closeout_notes.go`, `internal/app/finalize_cleanup.go`, `internal/domain/actions.go`, `internal/render/board.go`, `internal/render/closeout_notes.go`, `internal/repository/evolution.go`, `internal/repository/validate.go`
  - Tests: `internal/app/change_create_test.go`, `internal/app/change_groom_test.go`, `internal/app/change_lifecycle_test.go`, `internal/app/finalize_closeout_integration_test.go`, `internal/app/maintenance_test.go`, `internal/app/maintenance_traffic_integration_test.go`, `internal/app/repomigration_integration_test.go`, `internal/app/status_corpus_test.go`, `internal/domain/lease_test.go`, `internal/render/board_test.go`, `internal/repository/validate_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: the refusal message texts `change %04d is already final; a retry carrying different notes is not a replay and cannot rewrite the archived record` and `change %04d is already stacked-merged; a retry carrying different notes is not a replay and cannot rewrite the closed-out record`.

- [ ] **Step 1: Red scan and pin check**

Run the scan helper with `terminal\s+(status(es)?|records?)|non-terminal\s+status|already\s+terminal|now-terminal|terminal\s+emoji`, filtering to `*.go`.
Expected: the lifecycle sites listed in Step 2 plus many process-level sites. The process-level sites are **left alone**: everything in `internal/process/**`, `internal/gatedrive/**`, `internal/app/rungate_*`, `internal/app/evidence_ops.go`, `internal/app/gate_integration_test.go`, `internal/app/change_integration_test.go`, `internal/codexentry/**`, `internal/app/finalize_cleanup.go`'s gate-run lines (`durable terminal record`, `terminal record, no live lock/group`), and `internal/app/finalize_cleanup_test.go`.

Then check that no test pins the messages you will change:
`command git grep -n -e "cannot rewrite the terminal record" -e "is already terminal; a retry" -e "terminal records must not be closed out" -- '*_test.go' tests/`
Expected: only `internal/app/maintenance_test.go` (the `t.Errorf` text itself, which this task edits). If any other assert pins a message, update it in this task to the new text.

- [ ] **Step 2: Edit**

- `internal/app/finalize_closeout.go`:
  - `ReasonCloseoutNotesFrozen` doc comment: `the change is already terminal and the request\n// carries notes that differ from the terminal record; refused — a terminal\n// record is never rewritten.` → `the change is already final and the request\n// carries notes that differ from the archived record; refused — an archived\n// record is never rewritten.`
  - `// Terminal-state short circuits keyed on the promised state` → `// Final-state short circuits keyed on the promised state`
  - `// killed/other terminal record is an illegal source.` → `// killed/other archived record is an illegal source.`
  - In the replay comment, each `terminal record` → `archived record`
  - The `StatusDone` refusal message becomes `"change %04d is already final; a retry carrying different notes is not a replay and cannot rewrite the archived record"`
  - The stacked-merged refusal message becomes `"change %04d is already stacked-merged; a retry carrying different notes is not a replay and cannot rewrite the closed-out record"` (a stacked-merged record is not archived)
  - `closeoutNotesMatchTerminal` doc comment: `terminal record` → `archived record` (the identifier stays)
- `internal/app/finalize_closeout_notes.go`: `the terminal record's` → `the archived record's`.
- `internal/app/finalize_cleanup.go`: `now-terminal record` → `now-archived record` (only this line).
- `internal/domain/actions.go`: both `non-terminal status` → `non-final status`; `a terminal record holds no lease` → `an archived record holds no lease`.
- `internal/render/board.go`: in the numbered doc list, `at least one archived\n//     terminal record exists` → `at least one archived\n//     record exists`, and `the present\n//     terminal emoji` → `the present\n//     final-status emoji`.
- `internal/render/closeout_notes.go`: `of a terminal record` → `of an archived record`.
- `internal/repository/evolution.go`: `a terminal status reopened` → `a final status reopened`; `a terminal status is closed` → `a final status is closed`.
- `internal/repository/validate.go`: `marks an archived terminal record that still` → `marks an archived record that still` (the `CodeChangeTerminalClaimStamp` identifier and its code value stay); `the claim stamp a terminal record must not carry` → `the claim stamp an archived record must not carry`.
- Test comments and failure-message strings in the listed test files: change lifecycle-sense `terminal status` → `final status`, `terminal record(s)` → `archived record(s)`, `TERMINAL records` → `ARCHIVED records`, `already-terminal record` → `already-archived record`. Do **not** change `t.Run` names, table `name:` fields, or any identifier, because shard scripts may select tests by name. `internal/repository/validate_test.go`'s `name: "terminal status left in active"` and `name: "archived terminal record still holding a claim stamp"` therefore stay.

- [ ] **Step 3: Verify**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e
gofmt -l internal/app internal/domain internal/render internal/repository
go vet ./internal/app/ ./internal/domain/ ./internal/render/ ./internal/repository/
go vet -tags integration ./internal/app/
go test -count=1 ./internal/domain/ ./internal/render/ ./internal/repository/
go test -count=1 ./internal/app/ -run 'Closeout|Maintenance|Status|Lifecycle|Create|Groom'
```

Expected: `gofmt -l` prints nothing; vet is clean; tests PASS. Then diff the task: `command git diff --stat` shows only the files above, and `command git diff -U0 -- '*.go' | grep -E '^[-+][^-+]' | grep -v -E '^\s*[-+]\s*(//|"|t\.(Errorf|Fatalf|Fatal|Error)|fmt\.Sprintf|Message)'` prints nothing (every changed line is a comment or string). Run the protected-token check: all `ok`.

- [ ] **Step 4: Commit**

```bash
command git add internal/app/finalize_closeout.go internal/app/finalize_closeout_notes.go internal/app/finalize_cleanup.go internal/domain/actions.go internal/render/board.go internal/render/closeout_notes.go internal/repository/evolution.go internal/repository/validate.go internal/app/change_create_test.go internal/app/change_groom_test.go internal/app/change_lifecycle_test.go internal/app/finalize_closeout_integration_test.go internal/app/maintenance_test.go internal/app/maintenance_traffic_integration_test.go internal/app/repomigration_integration_test.go internal/app/status_corpus_test.go internal/domain/lease_test.go internal/render/board_test.go internal/repository/validate_test.go
command git commit -m "refactor: final/archived wording in Go lifecycle comments and messages (change 0468)"
```

Stage only files that actually changed. If `git add` names a file with no diff, that is harmless. If a listed file had no lifecycle-sense hit, say so in the task report.

---

### Task 7: Residual audit and the full suite

**Build profile:** standard

**Files:**
- No planned modifications. Fix-ups found here go into the file that owns them, in one commit.

**Interfaces:**
- Consumes: the results of Tasks 1–6.
- Produces: the residual report the parent's results artifact quotes.

- [ ] **Step 1: Whole-repo residual scan**

Run the scan helper once per regex:

1. `coordination(-key)?[\s-]+fenc`
2. `human(['’]s)?\s+merge\s+gate`
3. `merge[\s-]+gate`
4. `rebase-retest(\s+\w+)?\s+gate`
5. `\btest\s+gate`
6. `autonomous[\s-]+eligible`
7. `terminal\s+(status(es)?|records?|sweep|close-?out|transition|outcomes?)`
8. `(happy|sad)\s+terminal`
9. `non-terminal`

Classify every printed line into exactly one allowed bucket:

- **(a) process-level "terminal"** (Decision 6)
- **(b) retired-feature or excluded phrase**: `terminal publish(ing/ication)`, `terminal half`
- **(c) guard pin asserting an old phrase is ABSENT**: `internal/repoguard/prose_contracts_test.go`'s `"terminal sweep evidence for implementation scope"` in an `absent:` list
- **(d) file path or ADR filename**: e.g. `0010-finalize-merge-gate-split-agents.md`, `terminal-close-out.md`
- **(e) family-owned prose left for 0474**: `internal/config/**` fence comments
- **(f) English verb, not the term**: `finalize_merge_test.go`'s `merge gates on`

Regexes 2, 4, 5, and 6 must print nothing. Any line that fits no bucket is a missed site: fix it in its owning file (regenerating the mirror/goldens if it is an authored root), then re-run this step. Paste the classified list into the task report.

- [ ] **Step 2: Maintained always-loaded files**

Run: `command git diff --stat 4b318461c4b0d28916c3acffc2add03f7272fff4 -- CLAUDE.md AGENTS.md` and the scan helper over the regexes above, restricted to those two files.
Expected: no diff and no hits. The base already contains none of these phrases in their renamed senses; `nonterminal` there is process-level.

- [ ] **Step 3: Scope and integrity checks**

```bash
cd /Users/homer/dev/docket/.worktrees/rename-colliding-docket-terms-and-retire-obsolete-glossary-e
command git diff --name-only 4b318461c4b0d28916c3acffc2add03f7272fff4 -- docs/adrs docs/changes docs/results testdata internal/install/legacydata docs/codex/fixtures docs/reference/harness/fixtures internal/config
command git diff --name-only 4b318461c4b0d28916c3acffc2add03f7272fff4 -- 'docs/superpowers' | grep -v -F 'docs/superpowers/plans/2026-09-29-0468-rename-colliding-docket-terms-and-retire-obsolete-glossary-e.md'
command git diff --name-status --no-renames 4b318461c4b0d28916c3acffc2add03f7272fff4 | grep -v '^M'
bash "$CHK" docs/reference/glossary.md   # rebuild $CHK per Task 1 Step 1 if this is a new shell
```

Expected: the first two commands print nothing. The third prints nothing: no file was added, deleted, or renamed; only modified. The anchor check reports `0 dangling`. Run the protected-token check: all `ok`.

- [ ] **Step 4: Full suite (the build gate command)**

Run from the worktree root: `go run ./cmd/docket development test`
Expected: the `SUITE …` summary line reports green. Read the budget report even when green. Report any `BUDGET WATCH:`, `PARALLEL-SENSITIVE:` or `SERIAL CONFIRMED OVER BUDGET:` line (AGENTS.md *Guards and tests*). For a red result, root-cause it: a golden or embedded-drift failure means a regeneration step was skipped in the task that touched that root. Never weaken a test to pass.

- [ ] **Step 5: Commit fix-ups (only if Steps 1–4 required edits)**

```bash
command git add <each fixed path, explicitly> 
command git commit -m "docs: residual row 60–65 sites found in the audit (change 0468)"
```

If no edit was needed, make no commit and say so in the task report.
