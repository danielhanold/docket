# Auto-grant docket's integration-branch push permission (per-repo Claude settings) — design

Change: #27 · Status: proposed · Depends on: #26 (config-resolution script) · Related: #15, #22, #25, #26

## 1. Context / problem

Every docket terminal transition (`done` via `docket-finalize-change` / the `docket-status`
sweep, `killed` via the producer or implementer) runs the shared **terminal-publish**
procedure, whose final step pushes the change's terminal records onto the **integration
branch**:

```bash
git -C "$pub" push origin HEAD:<integration_branch>      # $pub = a mktemp transient worktree
```

Claude Code's permission classifier guards direct pushes to the repository's **default
branch**, so in an interactive session this push is refused unless the human approves it —
on **every** close-out. (Observed live while finalizing change #23: the archive push to
`origin/docket`, the branch delete, and `gh pr merge` all proceeded, but the
`git -C "$pub" push origin HEAD:main` step was blocked and required manual approval.)

The friction is small per occurrence but recurs on every terminal transition and is exactly
the kind of deterministic, well-understood action that should be pre-authorized — **narrowly,
and only in repos that actually use docket**.

## 2. Goals / non-goals

**Goals**
- Pre-authorize *only* docket's terminal-publish push to the integration branch, so close-out
  runs without a per-push prompt.
- Scope the grant **per-repo** (a repo that uses docket), never machine-globally — a global
  rule would silently authorize pushes to `main` in every unrelated repo Claude touches.
- Write the rule **automatically** at docket setup, creating the local Claude config from
  scratch if it does not exist, and remain idempotent + re-runnable.

**Non-goals**
- Broadening permissions beyond the single blocked action (no blanket `git push`).
- Changing *what* terminal-publish does, or how it is invoked (see §4, alternative rejected).
- Touching `install.sh` — it is **machine-level** setup (`link-skills.sh` + `sync-agents.sh`
  against the harness root `~/.claude`, via the `$HOME`/`DOCKET_HARNESS_ROOT` seam) and has no
  per-repo context. The correct owner is the per-repo tooling (§5).
- Any user-global (`~/.claude/…`) rule.

## 3. The permission rule

**Decision: write the single allow-rule**

```
Bash(git -C * push origin HEAD:<integration_branch>)
```

into the repo's **local** Claude config, `<repo>/.claude/settings.local.json`, under
`.permissions.allow`. `<integration_branch>` is the repo's resolved integration branch
(`main` here; `develop` in a GitFlow repo) — see §4 for resolution.

**Why this exact form.** Claude Code's Bash matcher is **left-anchored prefix matching** with
`*` permitted at any position (a `*` spans any run of characters including spaces; a space in
the pattern enforces a word boundary). docket's real command is always
`git -C "$pub" push origin HEAD:<branch>`, so:

- The literal prefix `git -C ` and the literal tail ` push origin HEAD:<branch>` pin the rule
  to docket's actual invocation shape; the single `*` absorbs only the volatile mktemp path.
- The fixed tail means the rule authorizes **only** a push whose refspec is `HEAD:<branch>`.
  A force-push (`git -C … push --force origin HEAD:<branch>`) does **not** match (the tail
  would be `push --force origin …`, not `push origin …`); a push to any other branch does not
  match. So the force-push and wrong-branch guardrails stay intact.
- It requires **no change to terminal-publish or the convention** — the rule mirrors the
  procedure as it already exists (single source untouched).

**Why not the alternatives** (considered, rejected):
- `Bash(git * push origin HEAD:<branch>)` — also works, but broader than needed: the leading
  `*` would match arbitrary flags, not just `-C <path>`. `git -C *` is strictly tighter and
  still matches docket's command.
- `cd "$pub" && git push origin HEAD:<branch>` + exact rule `Bash(git push origin HEAD:<branch>)`
  — the "purest" exact rule, but the `$pub` worktree lives in a **mktemp dir outside the
  workspace**, so `cd` into it risks merely *shifting* the permission prompt onto the `cd`
  subcommand (compound commands are split and evaluated per-subcommand; `cd` is auto-read-only
  only for in-workspace/added dirs). It also edits the convention's single-source
  terminal-publish procedure for no real safety gain. Net negative — rejected.
- A user-global rule in `~/.claude/settings.json` — rejected: it would authorize pushes to the
  default branch in **every** repo, defeating the per-repo scoping that is the whole point.

**Settings file & precedence.** `<repo>/.claude/settings.local.json` is read and merged by
Claude Code at a precedence above shared-project (`.claude/settings.json`) and user
(`~/.claude/settings.json`) settings, and is **gitignored by convention** (already true in
this repo) — so the grant is local to the user's machine and never committed or forced onto
collaborators. Schema:

```json
{ "permissions": { "allow": ["Bash(git -C * push origin HEAD:main)"] } }
```

## 4. The helper — `scripts/ensure-claude-settings.sh`

A new, self-contained, **idempotent** script that writes the rule. It is the single source of
the write; `migrate-to-docket.sh` and any standalone run both go through it.

**Interface**
```
ensure-claude-settings.sh            # operate on the repo containing $PWD
```
- Resolves the **repo root** from `$PWD` (`git rev-parse --show-toplevel`), the same model as
  `migrate-to-docket.sh` (usable from any consuming repo, not docket's own checkout).
- Resolves `<integration_branch>` by **consuming change #26's resolver** —
  `eval "$(scripts/docket-config.sh --export)"`, then using `$INTEGRATION_BRANCH`. No
  duplicated config parsing (this is why #27 is gated on #26 — see §8).
- Ensures `<repo>/.claude/` exists and `<repo>/.claude/settings.local.json` exists (creating
  it as `{}` if absent — *"create the whole local Claude config if it doesn't exist"*).
- **Idempotently merges** the rule into `.permissions.allow` with `jq` (already a repo
  dependency — `scripts/github-mirror.sh` uses it): add the rule iff not already present;
  preserve every pre-existing key and rule; a second run is a no-op.
- Prints a one-line summary of what it did (created / added / already-present).
- Does **no git writes** (the file is gitignored).

**jq merge sketch** (exact form settled at build):
```bash
tmp="$(mktemp)"
jq --arg rule "Bash(git -C * push origin HEAD:$INTEGRATION_BRANCH)" '
  .permissions = (.permissions // {}) |
  .permissions.allow = ((.permissions.allow // []) + [$rule] | unique)
' "$settings" > "$tmp" && mv "$tmp" "$settings"
```
(`unique` is the idempotency guard; `// {}` / `// []` create the nested shape when absent.)

**Test seam.** To keep `tests/` hermetic and isolate the helper from the resolver, allow the
integration branch to be injected — either a `--integration-branch <name>` flag or a
`DOCKET_INTEGRATION_BRANCH` env override consulted before invoking `docket-config.sh`
(final choice at build). The hermetic fixture can also ship a `.docket.yml` + bare origin and
exercise the real resolver path.

## 5. Wiring — `migrate-to-docket.sh`

`migrate-to-docket.sh` is the **per-repo** setup tool (run inside the repo being migrated;
already idempotent and interrupted-run-safe; already mutates per-repo files such as
`.gitignore`). Add a step — adjacent to the existing `.gitignore` extension (its step 5) —
that invokes `scripts/ensure-claude-settings.sh`. Migrating a repo to docket-mode thus grants
the rule as part of setup. Mention it in the script's closing "next steps".

## 6. Standalone / fresh-cloner path

`settings.local.json` is gitignored and per-user, and `migrate-to-docket.sh` is a one-time
per-repo operation — so a developer who **clones an already-migrated repo** never runs migrate
and would not have the rule. Because the write lives in the standalone
`scripts/ensure-claude-settings.sh`, that developer can simply run it directly to grant
themselves the rule. Document this in the repo README (and the migrate "next steps") so the
recovery path is discoverable. This closes the per-user gap without committing the grant.

## 7. Tests — `tests/test_ensure_claude_settings.sh`

Hermetic (temp repo, no network), matching the style of `tests/test_github_mirror.sh`:
- **Create-when-absent:** no `.claude/` → run → `.claude/settings.local.json` exists and
  contains the rule under `.permissions.allow`.
- **Idempotent:** a second run adds no duplicate (array length unchanged; mutation-genuine —
  assert the *count* of the rule is exactly 1, not merely "present").
- **Preserve existing:** a pre-existing `settings.local.json` with unrelated keys and an
  unrelated allow-rule keeps them all; only the new rule is appended.
- **Branch resolution:** a `main` fixture and a `develop` fixture each produce the rule with
  the correct `HEAD:<branch>` tail (via the test seam / a fixture `.docket.yml`).
- **No git writes:** the run leaves the git index/tree unchanged (the file is gitignored).

## 8. Dependencies & relations

- **`depends_on: [26]`** — the helper consumes `scripts/docket-config.sh --export`'s
  `INTEGRATION_BRANCH` rather than re-parsing `.docket.yml`. #26 is `implemented` (PR #38);
  #27 becomes build-ready when #26 reaches `done`. Gating here is deliberate: it eliminates a
  second config-resolution site (the project's standing direction — lift deterministic blocks
  into one tested place: #11, #22, #25, #26).
- **`related: [15, 22, 25, 26]`** — #15 (finalize merge gate, where the push lives), #22/#25
  (the script-extraction precedent and the close-out mechanics that issue the push), #26 (the
  resolver consumed here).

## 9. Risks & boundary

- **Matcher coupling to the `-C` form.** The rule is pinned to `git -C * push …`. If
  terminal-publish ever stopped using `-C "$pub"`, the rule would stop matching. That is
  acceptable and even desirable — the rule should mirror the real command, and terminal-publish's
  `-C` form is its established single-source shape (it deliberately never switches the main
  tree's directory). A future invocation change would update both together.
- **Integration-branch change after setup.** If a repo later changes `integration_branch`,
  re-running the helper appends a rule for the new branch (leaving the old one). Harmless;
  re-run is the remedy. Not worth pruning logic now (YAGNI).
- **ADR?** The decision "docket grants its own narrow permissions **per-repo**, never
  user-global" is a small but genuine boundary. It is captured here and in the change body;
  an ADR is **optional** and not required for this change. (If desired later, it would
  generalize the same per-repo-scoping rationale.)

## 10. Out of scope

- `install.sh` changes (machine-level; wrong scope).
- Any user-global permission rule.
- Editing terminal-publish's invocation or the convention's terminal-publish procedure.
- Authorizing pushes to `origin/<metadata_branch>` (docket branch) or branch deletes — never
  blocked by the classifier (only the default branch is guarded).
- `gh pr merge` (not blocked).
