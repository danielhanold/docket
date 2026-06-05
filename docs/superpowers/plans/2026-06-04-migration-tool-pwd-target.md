# migrate-to-docket.sh $PWD-target — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Retarget `migrate-to-docket.sh` to migrate the git repo containing `$PWD` (not its own `$SCRIPT_DIR`), guarded by a confirmation prompt (with `--yes` bypass), so it's usable on consuming repos.

**Architecture:** Small bash-script change, built TDD-for-docs (assertions in `tests/test_docket_metadata_branch.sh` are the red/green gate). Detailed design: `docs/superpowers/specs/2026-06-02-docket-metadata-branch-design.md`'s sibling — the change's own spec at `docs/superpowers/specs/2026-06-04-migration-tool-pwd-target-design.md` (read it; on the `docket` branch / `.docket/`). Three files change: the script, the test, the README.

**Tech Stack:** bash (`git rev-parse --show-toplevel`, `read … </dev/tty`), the existing `tests/*.sh` assertion harness.

---

## File Structure
- **Modify** `migrate-to-docket.sh` — target resolution + `--yes` flag + confirm prompt.
- **Modify** `tests/test_docket_metadata_branch.sh` — assertions for the retarget + bypass.
- **Modify** `README.md` — migration-usage section.

(`docs/changes/`, `docs/superpowers/specs/` are metadata on `docket` — NOT touched on this feature branch.)

---

### Task 1: Failing assertions (red)

**Files:** Modify `tests/test_docket_metadata_branch.sh`

- [ ] **Step 1:** Add a block to `tests/test_docket_metadata_branch.sh` (before `exit $fail`):

```bash
# Q. migrate-to-docket.sh targets $PWD's repo (not its own SCRIPT_DIR) + has a --yes bypass (change 0003).
assert "migrate resolves target via git rev-parse --show-toplevel" \
  'grep -q "rev-parse --show-toplevel" migrate-to-docket.sh'
assert "migrate no longer cd's to SCRIPT_DIR" \
  '! grep -q "cd \"\$SCRIPT_DIR\"" migrate-to-docket.sh'
assert "migrate has a --yes/-y confirmation bypass" \
  'grep -qE "\-\-yes\b|\b-y\b" migrate-to-docket.sh'
assert "migrate prompts for confirmation (reads /dev/tty)" \
  'grep -q "/dev/tty" migrate-to-docket.sh'
```

- [ ] **Step 2:** Run `bash tests/test_docket_metadata_branch.sh 2>&1 | grep -E "migrate (resolves|no longer|has a|prompts)"` → all 4 **NOT OK** (red). Commit: `git add tests/test_docket_metadata_branch.sh && git commit -m "test(0003): assertions for \$PWD-target migration tool (red)"`.

### Task 2: Retarget the script + confirm prompt (green)

**Files:** Modify `migrate-to-docket.sh`

- [ ] **Step 1:** Reorder so the output helpers (`say`/`step`/`die`) are defined **before** any use, then replace the `SCRIPT_DIR` block (currently lines ~30–31, `SCRIPT_DIR="$(cd …)"; cd "$SCRIPT_DIR"`) with arg-parse + `$PWD`-repo resolution. Concretely, after `set -euo pipefail` and the helper definitions:

```bash
# --yes/-y skips the confirmation prompt (for automation).
ASSUME_YES=0
for arg in "$@"; do
  case "$arg" in
    -y|--yes) ASSUME_YES=1 ;;
    *) die "unknown argument: $arg  (usage: cd <target-repo> && bash /path/to/docket/migrate-to-docket.sh [--yes])" ;;
  esac
done

# Operate on the git repo containing the INVOCATION directory ($PWD) — NOT the script's own location.
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" \
  || die "not inside a git repo — cd into the repo you want to migrate, then re-run."
cd "$REPO_ROOT"
```

Delete the now-unused `SCRIPT_DIR` assignment (it is referenced nowhere else). Update the header-comment usage line to `cd <target-repo> && bash /path/to/docket/migrate-to-docket.sh [--yes]`.

- [ ] **Step 2:** Add `Target repo: $REPO_ROOT` to the "Resolved configuration" banner (the `step "Resolved configuration"` block), so the operator sees which repo will be migrated.

- [ ] **Step 3:** After the resolved-config banner and **before** the "Checking preconditions" step, add the confirm gate:

```bash
if [ "$ASSUME_YES" -ne 1 ]; then
  printf 'Migrate this repo (%s) to docket-mode? [y/N] ' "$REPO_ROOT"
  read -r reply </dev/tty || reply=""
  case "$reply" in
    y|Y|yes|YES) ;;
    *) die "aborted — no changes made." ;;
  esac
fi
```

- [ ] **Step 4:** Run `bash -n migrate-to-docket.sh` (syntax OK) and `bash tests/test_docket_metadata_branch.sh; echo "FULL=$?"` → the 4 new assertions PASS, **FULL=0**. Commit: `git commit -am "feat(0003): migrate-to-docket.sh targets \$PWD's repo + confirm prompt (--yes bypass)"`.

### Task 3: README migration usage

**Files:** Modify `README.md`

- [ ] **Step 1:** In the docket-mode/migration section, update the migration usage to show running it **from within the target repo**: `cd <target-repo> && bash /path/to/docket/migrate-to-docket.sh` (note the confirmation prompt + `--yes` for automation). Remove any wording implying it only migrates the docket repo / must be run from the docket repo.

- [ ] **Step 2:** Run the full suite: `bash tests/test_docket_metadata_branch.sh && bash tests/test_sync_convention.sh && bash tests/test_results_artifact.sh && bash tests/test_link_skills.sh; echo "ALL=$?"` → `ALL=0`. Commit: `git commit -am "docs(0003): README — run migrate-to-docket.sh from within the target repo"`.

---

## Self-review
- Spec coverage: §3 target resolution → T2.1; §3 confirm guard → T2.3 + T2.1 (`--yes`); §4 touch-points → T1 (test), T2 (script), T3 (README); §6 testing → T1 assertions + `bash -n`. All covered.
- No placeholders: the assertions and script edits are shown in full.
- Out of scope (spec §5) honored: no distribution/skill work; `.docket.yml`-flip and ff-merge gaps left alone.
