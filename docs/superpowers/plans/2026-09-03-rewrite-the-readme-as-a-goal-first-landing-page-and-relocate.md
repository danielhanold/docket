<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0400 — Rewrite the README as a goal-first landing page and relocate its technical body to docs/](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0400-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate.md)**
<!-- docket:backlink:end -->
# Rewrite the README as a goal-first landing page — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan
> task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the 1,075-line mechanism-organised `README.md` with a ≤200-line goal-first
landing page, relocate the current README body **verbatim** into `docs/guide/README.md`, commit
the playbook-comparison research as `docs/comparison/ai-native-sdlc-playbook.md`, and repoint
every repo guard that pins README phrases so the whole suite stays green.

**Architecture:** A mechanical relocation first (so guards can be repointed at a file that
already exists), then the comparison page (so the new README's map links resolve), then the
README rewrite **in the same commit** as the guard-table repoint (the guards pin phrases the
rewrite removes — splitting them makes an intermediate red commit). A final derivation sweep
retargets any in-repo link into the moved body. Docs + one Go test-table edit only.

**Tech Stack:** Markdown, Go test table (`internal/repoguard`), shell for mechanical
extraction/verification.

**Spec:** `docs/superpowers/specs/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-design.md`
(on the `docket` metadata branch; readable in this checkout at the **synchronized metadata
worktree** path `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-design.md`).

## Global Constraints

- Whole-suite gate: `go run ./cmd/docket development test` must be green (run by docket-build's
  final gate; do not substitute a subset).
- New `README.md` ≤ 200 lines; the proper name `AI-Native SDLC Playbook` appears **exactly
  once** (case-sensitive fixed-string count; the lowercase file path
  `ai-native-sdlc-playbook.md` does not count); both map links `](docs/guide/README.md)` and
  `](docs/comparison/ai-native-sdlc-playbook.md)` present.
- `docs/guide/README.md` carries the pre-change README body byte-identical **except**
  rewritten relative link paths; **path rewriting inside `](…)` link targets is the only
  permitted edit in the moved body**; intra-page `#anchor` links and all prose/code-fence
  mentions of paths stay untouched. The `<!-- docket:config-fence: values -->` marker moves
  with its fence, untouched.
- No YAML fence in the new README (the spec permits at most one — the `.docket.yml` example
  under Install — and this plan includes none; ADR-0053's fence guard is retired).
- No line-number cross-references anywhere (AGENTS.md rule).
- No changes to skills, agents, the CLI, or `.docket.example.yml`.
- Point-in-time records (`docs/changes/archive/`, `docs/results/`, specs, plans, Accepted
  ADRs, `docs/adrs/*`) are **never** edited by the link sweep.
- Mutation-probe discipline (learnings: `cached-runner-serves-a-mutated-tree`,
  `mutation-restore-needs-a-backup-copy`, `assert-detects-removal-not-replacement`):
  every Go probe uses `go test -count=1`; every mutation is restored from a `cp` backup
  (never `git checkout -- <file>`); every mutation is proven to have **landed** with
  `/usr/bin/grep -cF` before/after (PATH `grep` is ugrep — use `/usr/bin/grep` for landing
  checks) before its result is believed.
- All commands below run from the worktree root
  `/Users/homer/dev/docket/.worktrees/rewrite-the-readme-as-a-goal-first-landing-page-and-relocate`.

---

### Task 1: Relocate the README body verbatim into `docs/guide/README.md`

**Files:**
- Create: `docs/guide/README.md`
- Test: byte-equality proof via `diff` (recorded for the PR body); suite stays green because
  `README.md` is untouched in this task and nothing yet guards the new file.

**Interfaces:**
- Consumes: current `README.md` (1,075 lines; body starts at the `## Table of contents`
  heading).
- Produces: `docs/guide/README.md` — the file Task 3's repointed guard rows read by path, and
  the target of Task 3's new-README map link and Task 4's retargeted anchors. Section anchors
  inside it are identical to the old README's (headings are unchanged), e.g.
  `#install`, `#quickstart-the-daily-loop`, `#tuning-agent-models--effort`,
  `#configuration--docketyml-global-config-and-machine-local-overrides`.

- [ ] **Step 1: Snapshot the pre-change README for the byte-equality proof**

```bash
S=/private/tmp/claude-501/-Users-homer-dev-docket/5ac2a595-fc9d-4d67-98f1-3d51add9ed1a/scratchpad
mkdir -p "$S"
git show HEAD:README.md > "$S/readme-pre-0400.md"
awk '/^## Table of contents$/{f=1} f' "$S/readme-pre-0400.md" > "$S/moved-body-original.md"
wc -l "$S/moved-body-original.md"   # expect 1061 (lines 15–1075 of the old README)
```

- [ ] **Step 2: Write the authored header and append the moved body**

Create `docs/guide/README.md` with exactly this header block, then the body:

```bash
mkdir -p docs/guide
cat > docs/guide/README.md <<'EOF'
# docket — technical guide

This page is the former `README.md` body, relocated unchanged by change 0400 while the
goal-organised split of its content is pending. Only relative link paths were rewritten for the
new location. Start at the [landing page](../../README.md) for what docket does and where you
stay in control.

EOF
cat "$S/moved-body-original.md" >> docs/guide/README.md
```

- [ ] **Step 3: Rewrite relative link paths inside the moved body — link targets only**

First derive the complete set of link-target shapes in the moved body (never trust a
hand-list — AGENTS.md):

```bash
/usr/bin/grep -o '](\([^)#][^)]*\))' "$S/moved-body-original.md" | sort -u
```

Expected classes (verify the derivation shows nothing else; if a new class appears, apply the
same relocation rule — the file now lives two directories deeper):
`](docs/…)`, `](.docket.example.yml)`, `](skills/…)`, `](agents/…)`, `](https://…)` (external,
untouched), `](#…)` (intra-page, untouched — excluded by the grep above).

Apply the rewrites (link-target syntax `](` anchors each substitution so prose and code-fence
path mentions are untouched):

```bash
perl -pi -e 's{\]\(docs/}{](../}g; s{\]\(\.docket\.example\.yml}{](../../.docket.example.yml}g; s{\]\(skills/}{](../../skills/}g; s{\]\(agents/}{](../../agents/}g' docs/guide/README.md
```

- [ ] **Step 4: Prove byte-equality modulo link rewrites (the PR-body diff)**

```bash
awk '/^## Table of contents$/{f=1} f' docs/guide/README.md > "$S/moved-body-relocated.md"
diff "$S/moved-body-original.md" "$S/moved-body-relocated.md" > "$S/relocation-diff.txt"; echo "diff-exit=$?"
/usr/bin/grep -c '^[<>]' "$S/relocation-diff.txt"
/usr/bin/grep '^[<>]' "$S/relocation-diff.txt" | /usr/bin/grep -vc ']('  # MUST print 0
```

Expected: every changed line contains a `](` link target (the last count is **0**). If any
changed line has no link, the move corrupted prose — fix before proceeding. Also confirm the
config-fence marker moved intact:

```bash
/usr/bin/grep -cF '<!-- docket:config-fence: values -->' docs/guide/README.md   # expect 1
diff <(/usr/bin/grep -F '<!-- docket:config-fence' "$S/moved-body-original.md") <(/usr/bin/grep -F '<!-- docket:config-fence' docs/guide/README.md)   # expect no output
```

Save `$S/relocation-diff.txt`'s content: it is the byte-equality proof the spec requires in the
PR body (Acceptance 3). Report it in this task's completion report so it reaches the PR.

- [ ] **Step 5: Run the repoguard tests to confirm nothing reds (README untouched so far)**

```bash
go test -count=1 ./internal/repoguard/
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add docs/guide/README.md
git commit -m "docs(0400): relocate README technical body verbatim to docs/guide/README.md"
```

---

### Task 2: Commit the comparison research as `docs/comparison/ai-native-sdlc-playbook.md`

**Files:**
- Create: `docs/comparison/ai-native-sdlc-playbook.md`
- Test: byte-equality of the carried appendix against the spec (diff, recorded).

**Interfaces:**
- Consumes: the spec's Appendix, extracted from the synchronized metadata worktree copy at
  `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-design.md`
  (the appendix runs from the line `## Appendix — docket against the AI-Native SDLC Playbook`
  to EOF; its first body line is the italic `*Compared on 2026-09-03: …*` preamble that names
  the date and the docket revision `main` @ `ac782038`, satisfying the "opens with date and
  revision" requirement verbatim).
- Produces: the page Task 3's new README links as
  `](docs/comparison/ai-native-sdlc-playbook.md)` (pinned by the new guard row
  `change_0400_readme_landing`).

- [ ] **Step 1: Extract the appendix and assemble the page**

```bash
S=/private/tmp/claude-501/-Users-homer-dev-docket/5ac2a595-fc9d-4d67-98f1-3d51add9ed1a/scratchpad
SPEC=/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-03-rewrite-the-readme-as-a-goal-first-landing-page-and-relocate-design.md
awk 'f; /^## Appendix — docket against the AI-Native SDLC Playbook$/{f=1}' "$SPEC" > "$S/appendix-body.md"
test -s "$S/appendix-body.md" || { echo "EXTRACTION EMPTY — heading not found; STOP"; exit 1; }
mkdir -p docs/comparison
{ printf '# docket against the AI-Native SDLC Playbook\n\n'; cat "$S/appendix-body.md"; } > docs/comparison/ai-native-sdlc-playbook.md
```

- [ ] **Step 2: Prove the carried body is verbatim**

```bash
diff "$S/appendix-body.md" <(tail -n +3 docs/comparison/ai-native-sdlc-playbook.md) && echo VERBATIM-OK
```

Expected: `VERBATIM-OK` (only the H1 + blank line were prepended). Record this in the task
report.

- [ ] **Step 3: Sanity-check the page's own links resolve from `docs/comparison/`**

The appendix contains no relative links (its references are prose paths and one external URL);
verify:

```bash
/usr/bin/grep -o '](\([^)#][^)]*\))' docs/comparison/ai-native-sdlc-playbook.md | /usr/bin/grep -v 'https://' && echo "RELATIVE LINKS FOUND — retarget them for docs/comparison/ before committing" || echo NO-RELATIVE-LINKS
```

If any relative link is found, rewrite it to resolve from `docs/comparison/` (same relocation
rule as Task 1: `docs/X` → `../X`, repo-root file → `../../X`).

- [ ] **Step 4: Commit**

```bash
git add docs/comparison/ai-native-sdlc-playbook.md
git commit -m "docs(0400): commit the AI-Native SDLC playbook comparison as a dated docs page"
```

---

### Task 3: New landing-page `README.md` + guard-table repoint, one commit

The six repoguard rows pin phrases the rewrite removes from `README.md`, so the rewrite and
the repoint must land in the **same commit** to keep every commit green. TDD order: repoint the
table first and add the new row (the new row is the failing test — the new README does not
exist yet), then write the README to green it.

**Files:**
- Modify: `internal/repoguard/prose_contracts_test.go` (six `file:` fields; one new row)
- Modify: `README.md` (full replacement)
- Test: `go test -count=1 ./internal/repoguard/` + mutation probes below

**Interfaces:**
- Consumes: `docs/guide/README.md` (Task 1 — carries every pinned phrase, including the
  path-rewritten `](../cursor/permissions.md)`), `docs/comparison/ai-native-sdlc-playbook.md`
  (Task 2).
- Produces: the final `README.md`; the guard row `change_0400_readme_landing` later tasks and
  the suite rely on. Sentinel names are unchanged; only `file:` values move.

- [ ] **Step 1: Repoint the six rows and add the new row (the failing test)**

In `internal/repoguard/prose_contracts_test.go`, change `file: "README.md"` to
`file: "docs/guide/README.md"` on exactly these six rows, leaving every `present`/`absent`
phrase unchanged **except** the one path-rewritten phrase noted:

| sentinel | phrases after this edit |
|---|---|
| `test_consultant_brainstorm` | present `brainstorm: docket-brainstorm` (unchanged) |
| `test_skill_fork_dispatch` | present `completed (forked execution)`, `The right model for each step.` (unchanged) |
| `test_readme_finalize_docs` | present `auto-mode classifier`, `Fork-exclusion principle` (unchanged) |
| `test_readme_skill_catalog` | present `## Skills`; absent `#the-eight-skills` (unchanged) |
| `test_cursor_permissions_docs` (the README-file row only; the `docs/cursor/permissions.md` row stays) | present `](../cursor/permissions.md)` — **phrase updated** because the link was path-rewritten in the move |
| `test_typed_changes_docs` | present `untyped set can only shrink` (unchanged) |

Then add one new row (place it after the `test_change_types` row, before the
`change_0389_sweep_scope` block, matching the table's chronological tail):

```go
	// change 0400 — the goal-first landing page cannot silently lose its two
	// load-bearing map links (the relocated technical guide and the comparison page).
	{sentinel: "change_0400_readme_landing", file: "README.md",
		present: []string{"](docs/guide/README.md)", "](docs/comparison/ai-native-sdlc-playbook.md)"}},
```

- [ ] **Step 2: Run the guard test — expect exactly one failure**

```bash
go test -count=1 ./internal/repoguard/ -run TestProseContracts 2>&1 | tail -20
```

Expected: FAIL, and **only** on `change_0400_readme_landing` (the old README lacks both map
links). The six repointed rows must already pass against `docs/guide/README.md`. If any
repointed row fails, the Task 1 relocation dropped a phrase — fix that before proceeding.

- [ ] **Step 3: Replace `README.md` with the landing page**

Write `README.md` with exactly this content:

````markdown
# docket

docket keeps a backlog of planned work as plain markdown files inside your repo and ships
agent skills that drain that backlog to open pull requests. It is a repository-level
implementation of the Plan, Design, Build, Test, and Deploy stages of Anthropic's
[AI-Native SDLC Playbook](https://claude.com/blog/the-ai-native-sdlc-playbook) — git-native,
harness-neutral, with the human at the merge. Each unit of work is a **change**: one markdown
file, roughly one pull request's worth of work, that moves through a fixed lifecycle from idea
to archived record — coordinated entirely through git, with no service, no database, and no
CLI to install.

## What you get

- **A durable backlog that outlives the session.** Planned work is tracked in-repo as
  markdown, so a change you brainstorm today is still there, with its full context, when an
  agent picks it up next week.
- **Hands-off implementation.** An autonomous skill claims the next ready change, refreshes it
  against the current state of the code, builds it with test-driven development, and opens a
  PR — with no supervision in between.
- **You stay at the merge gate.** Agents never merge on their own authority. Your review of
  the pull request is the one required human checkpoint on the way to `done`.
- **No new infrastructure.** Markdown files, git, and skills any supported harness can run —
  Claude Code, Cursor, Codex, and opencode are first-class.
- **The right model for each step.** Every autonomous skill is pinned to its own model and
  effort, so a board refresh runs at a cheap tier while a build runs at a top one — see
  [Tuning agent models & effort](docs/guide/README.md#tuning-agent-models--effort).

## The committed artifact chain

Every stage ends in an artifact committed to git before the next stage reads it. The chain is
the audit trail — and the whole interface between you and the autonomous skills.

| Stage | What docket commits | Where it lives |
|---|---|---|
| Plan | The change file: why, what changes, out of scope, dependencies, priority, type | `docs/changes/active/` on the `docket` metadata branch; rendered on `BOARD.md` |
| Design | The spec, linked from the change (or `trivial: true` for small mechanical work) | `docs/superpowers/specs/` on the metadata branch |
| Build | A dated reconcile log, the implementation plan, one verified commit per task | The change body; the plan on the feature branch |
| Test | The build-evidence record — suite command, result, head SHA — plus the results file and the review's disposition table | The PR body; `docs/results/` |
| Deploy | The merge (proven reachable), the archived change record, a cleaned branch and worktree, a re-rendered board | `docs/changes/archive/`; `BOARD.md` |

## Why docket: plans rot

A change is drafted against a snapshot — the codebase, the decision ledger, and the other
in-flight changes as they stood the day you designed it. In an async backlog the implementer
may not pick it up for weeks. By then another change has shipped half of it, an architecture
decision has settled an open question the other way, or an interface it assumed has moved.
Most backlog systems build the ticket as written and let the implementer discover the
mismatch halfway through.

docket instead **reconciles at the last responsible moment**: after a change is claimed but
before any build work starts, the implementer re-reads it against related and archived
changes, recent decisions, and the current code — then rewrites its scope to what is true
now, records a dated reconcile log entry, kills it if it has become obsolete, or stops and
escalates if the design itself no longer holds.

The stance: plans rot, so refresh them just-in-time and never trust a stale backlog. The
`reconciled` flag on every built change is the visible proof it happened.

## Where you decide

- **Creating and grooming changes.** Capturing an idea and designing it to build-ready are
  interactive; autonomous grooming is opt-in per stub, and killing or deferring a change is
  never autonomous.
- **Merging the PR.** The one required checkpoint — the implementer stops at an open pull
  request every time.
- **Finalize's confirmations.** Close-out merges only with your authorization, and unattended
  repair at the merge gate blocks for your sign-off.
- **Promoting a learning.** Findings graduate into the always-loaded instructions file only by
  your hand.
- **Filing discovered work.** Runs report follow-up work; a human decides what enters the
  backlog.

Plan approval is deliberately **not** a human point: your checkpoint is the PR, where the plan,
the diff, and the evidence arrive together. And docket ends at the merge — it does not deploy,
monitor production, or feed incidents back into the backlog.

## Install and the daily loop

```bash
cd ~/dev/docket
git fetch --tags && git pull
bash install.sh
```

Re-run `install.sh` after every update — it is idempotent and machine-global. Full
prerequisites and what an install run does: [Install](docs/guide/README.md#install). To adopt
docket in an existing repo, run `docket repository migrate` from inside it
([Migration](docs/guide/README.md#migration)).

The daily loop, one skill per step
([Quickstart](docs/guide/README.md#quickstart-the-daily-loop)):

1. **Capture** an idea into the backlog — `docket-new-change`.
2. **Groom** rough stubs to build-ready — `docket-groom-next` (or `docket-auto-groom`).
3. **Drain** the backlog to open PRs — `docket-implement-next` (or `/loop` it hands-free).
4. **Review and merge** the PR — you.
5. **Close out** merged work — `docket-finalize-change`; `docket-status` keeps the board
   honest in between.

## Documentation map

- **[Technical guide](docs/guide/README.md)** — the full reference: how it works, install,
  quickstart, configuration and the layer model, docket-mode, model tuning, the skill catalog,
  learnings, customization, migration.
- **[How docket maps to the six-stage model](docs/comparison/ai-native-sdlc-playbook.md)** —
  the dated stage-by-stage comparison this page's framing rests on.
- **Harness setup** — [Cursor permissions](docs/cursor/permissions.md) ·
  [Cursor validation](docs/cursor/validation.md) · [Codex](docs/codex/setup.md) ·
  [opencode](docs/opencode/setup.md).
- **[Configuration reference](.docket.example.yml)** — every key, its default, and its scope.
- **[ADR index](docs/adrs/README.md)** — the immutable decision ledger.
- **[Test suite](tests/README.md)** — how docket's own suite runs and where a new test
  belongs.

## Status

docket-mode — planning metadata on its own orphan `docket` branch — is the supported default;
`main`-mode (`metadata_branch: main`) remains a simple opt-out that keeps everything on one
branch. Five documented features are deferred from Go v1 and activate nothing today:
`auto_capture`, `terminal_publish`, the automated learnings harvest/index/promotion,
`dummy_mode`, and `github_project`.
````

- [ ] **Step 4: Verify the landing-page constraints**

```bash
wc -l README.md                                                  # expect ≤ 200
/usr/bin/grep -cF 'AI-Native SDLC Playbook' README.md            # expect exactly 1
/usr/bin/grep -cF '](docs/guide/README.md)' README.md            # expect ≥ 1
/usr/bin/grep -cF '](docs/comparison/ai-native-sdlc-playbook.md)' README.md   # expect 1
/usr/bin/grep -c '^```yaml' README.md                            # expect 0 (no YAML fence)
```

- [ ] **Step 5: Run the guard test — expect green**

```bash
go test -count=1 ./internal/repoguard/
```

Expected: PASS (the new row now finds both links; the six repointed rows read
`docs/guide/README.md`).

- [ ] **Step 6: Sweep for other tests grepping removed README prose**

Learnings `restatement-accumulates-its-own-guards`: asserts reach into whichever copy was
nearest — before trusting the six-row list, grep the test surface for dependencies on the
prose the rewrite removed:

```bash
/usr/bin/grep -rln 'README' internal/ tests/ --include='*.go' --include='*.sh' | /usr/bin/grep -v _test_data
```

For each hit other than `internal/repoguard/prose_contracts_test.go`, read the matching lines
and confirm it does not assert phrases that lived only in the old README body (comments and
unrelated `README.md` mentions — e.g. the ADR-index renderer's — are fine). If one does,
repoint it at `docs/guide/README.md` in this same commit and record it in the task report.

- [ ] **Step 7: Mutation-test the new and repointed rows**

All probes: `cp` backup, `/usr/bin/grep -cF` landing check, `go test -count=1`, restore from
backup. Record each result verbatim for the results file (Acceptance 5).

```bash
# Probe A: delete the guide link from README → new row must redden
cp README.md README.md.bak
perl -pi -e 's{\]\(docs/guide/README\.md\)}{](docs/guide/GONE.md)}' README.md
/usr/bin/grep -cF '](docs/guide/README.md)' README.md            # MUST be 0 (mutation landed)
go test -count=1 ./internal/repoguard/ -run TestProseContracts 2>&1 | tail -3   # MUST FAIL naming change_0400_readme_landing
mv README.md.bak README.md

# Probe B: delete the comparison link from README → new row must redden
cp README.md README.md.bak
perl -pi -e 's{\]\(docs/comparison/ai-native-sdlc-playbook\.md\)}{](docs/comparison/GONE.md)}' README.md
/usr/bin/grep -cF '](docs/comparison/ai-native-sdlc-playbook.md)' README.md     # MUST be 0
go test -count=1 ./internal/repoguard/ -run TestProseContracts 2>&1 | tail -3   # MUST FAIL
mv README.md.bak README.md

# Probe C: restore file: "README.md" on ONE repointed row → it must redden against the new README
cp internal/repoguard/prose_contracts_test.go internal/repoguard/prose_contracts_test.go.bak
perl -pi -e 's{\{sentinel: "test_typed_changes_docs", file: "docs/guide/README\.md"}{\{sentinel: "test_typed_changes_docs", file: "README.md"}' internal/repoguard/prose_contracts_test.go
/usr/bin/grep -cF '{sentinel: "test_typed_changes_docs", file: "README.md"' internal/repoguard/prose_contracts_test.go   # MUST be 1
go test -count=1 ./internal/repoguard/ -run TestProseContracts 2>&1 | tail -3   # MUST FAIL ("untyped set can only shrink" is no longer in README.md)
mv internal/repoguard/prose_contracts_test.go.bak internal/repoguard/prose_contracts_test.go

go test -count=1 ./internal/repoguard/   # final green confirmation after restores
```

- [ ] **Step 8: Commit (README rewrite + guard repoint together)**

```bash
git add README.md internal/repoguard/prose_contracts_test.go
git commit -m "docs(0400): goal-first landing README; repoint prose guards to docs/guide/README.md"
```

---

### Task 4: Retarget in-repo links into the moved body, derived by whole-repo grep

**Files:**
- Modify: only maintained-source files the derivation surfaces (expected: none or very few —
  a pre-plan probe found **zero** `README.md#` anchor links and zero `](README.md`-shaped
  links outside point-in-time records — but the derivation, not that probe, is authoritative).
- Test: link-resolution check over the three authored/moved pages (Acceptance 4).

**Interfaces:**
- Consumes: `docs/guide/README.md` (Task 1) as the retarget destination; the final `README.md`
  (Task 3) whose surviving anchors are only its own eight sections.
- Produces: the recorded derivation + classification (for the results file) and the
  link-resolution evidence.

- [ ] **Step 1: Derive every candidate link — never hand-list (AGENTS.md)**

```bash
/usr/bin/grep -rn 'README\.md#' . --exclude-dir=.git --exclude-dir=.worktrees --exclude-dir=.docket 2>/dev/null
/usr/bin/grep -rn -e '](README\.md' -e '](\.\./README\.md' -e '](\.\./\.\./README\.md' --exclude-dir=.git --exclude-dir=.worktrees --exclude-dir=.docket . 2>/dev/null
```

- [ ] **Step 2: Classify every hit — maintained source vs point-in-time record**

Sort the hits into two lists and record both in the task report:
- **Point-in-time** (`docs/changes/archive/`, `docs/results/`, `docs/superpowers/specs/`,
  `docs/superpowers/plans/`, `docs/adrs/`): leave untouched — rewriting them falsifies
  history (AGENTS.md cross-reference rule).
- **Maintained source** (skills/, agents/, docs/cursor|codex|opencode, `.docket.example.yml`,
  `tests/README.md`, `AGENTS.md`, `internal/`, `cursor-rules/`): for each hit whose target is
  the root README **and** whose anchor (or content) lives in the moved body, retarget it to
  `docs/guide/README.md#<same-anchor>`, spelled relative to the linking file (e.g. from
  `docs/cursor/x.md` it is `../guide/README.md#…`; from a repo-root file it is
  `docs/guide/README.md#…`). Hits pointing at `tests/README.md`, `docs/*/README.md`, or the
  new landing page's own surviving sections stay as they are.

If the maintained-source list is empty, record `link-retarget: no maintained-source hits
(derivation output attached)` — an honest no-op, not a skipped step.

- [ ] **Step 3: Link-resolution check over the three pages (Acceptance 4 evidence)**

```bash
for f in README.md docs/guide/README.md docs/comparison/ai-native-sdlc-playbook.md; do
  d=$(dirname "$f")
  /usr/bin/grep -o '](\([^)#][^)]*\))' "$f" | sed 's/^](//; s/)$//' | /usr/bin/grep -v '^https\?://' | while read -r t; do
    [ -e "$d/$t" ] || echo "BROKEN: $f -> $t"
  done
done; echo LINK-CHECK-DONE
```

Expected: no `BROKEN:` lines before `LINK-CHECK-DONE`. Fix any broken target (this is the
one-off shell check the spec wants recorded in the results file — do not add it as a permanent
test). Record the full output.

- [ ] **Step 4: Run the focused tests, then commit**

```bash
go test -count=1 ./internal/repoguard/
```

Expected: PASS.

If Step 2 changed any files:

```bash
git add <exactly the retargeted maintained-source files>
git commit -m "docs(0400): retarget in-repo README-anchor links to docs/guide/README.md"
```

If it changed none, make no commit — report the recorded derivation instead.

---

## Final gate (owned by docket-build, not a task)

docket-build's suite gate runs the whole suite: `go run ./cmd/docket development test`.
Evidence to carry into the results file / PR body from the tasks above:
- Task 1's `relocation-diff.txt` (byte-equality proof — Acceptance 3, PR body),
- Task 2's `VERBATIM-OK` diff,
- Task 3's constraint checks and three mutation-probe records (Acceptance 5),
- Task 4's derivation, classification, and link-check output (Acceptance 4).
