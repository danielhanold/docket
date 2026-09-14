# Codex option 2 — explicit worktree targeting (POC only)

This user-authorized fixture replaces the strict startup-cwd equality requirement. The native child may start at the primary checkout. Do not modify the host's cwd, build a launcher, or use agent.enter. Use existing native named-agent dispatch and ordinary explicit file paths and shell workdir arguments. This document applies only to Codex in this disposable fixture. Production skills and other harnesses are unchanged.

## Child entry

Before reading skills, source, or ordinary metadata, run the external check-boundary.py with the exact supplied inputs. It only observes startup cwd and validates the assigned repository boundary; it does not launch, relocate, wrap tools, or enforce a sandbox. Reading this reference, the checker, and its supplied workspace receipt are authorized entry checks. Its Git identity reads are also authorized before normal repository work.

The coordinator must supply: Feature worktree (canonical absolute root), Primary checkout (canonical root), Feature ref (full refs/heads/... name), Pre-dispatch HEAD (full current SHA), Workspace receipt (absolute path to this run's successful workspace.prepare JSON), and Boundary checker (absolute path to check-boundary.py). Missing inputs halt. Use the first shell call with login false and no workdir override so its startup observation is honest. Invoke python3 with the checker path and --primary, --feature, --ref, --head, --receipt, and --change-id 1. On BINDING_FAIL stop; do not repair, relocate, retry against another path, or report completion. BINDING_OK is entry validation, never proof that later operations were safe.

The checker requires the supplied feature path to be canonical, a distinct registered linked worktree of the primary repository, on the expected branch, clean, and at the supplied current HEAD. Its path/ref/change must match this run's created-workspace receipt. The receipt's initial HEAD may differ from a later worker's pre-dispatch HEAD because the plan has been committed. The coordinator separately verifies live Docket ownership before each dispatch; a historical workspace receipt alone is not proof of a current claim.

## Every later operation

- Every shell call concerning feature source, plans, tests, commits, or skill execution explicitly sets workdir to the verified feature root. Do not rely on a previous cd persisting. Use login false to avoid shell startup changing the directory.
- Every file-tool read or edit uses an absolute path inside the verified feature root. Resolve symlinks and reject escapes. Repo-relative paths remain correct in Docket artifact payloads and Git pathspecs when their enclosing command runs in the verified feature root; do not convert a schema's relative path into an unsupported absolute path.
- The plan writer owns one plan file. The standard worker owns greeting.go and greeting_test.go only and must preserve the plan and baseline tests. The coordinator owns permitted results checkpoints. No child edits to AGENTS.md, RUN.md, configuration, skills, agent registrations, or validation helpers.
- Read fixture skill snapshots by their absolute feature paths. Synchronized metadata reads are allowed only as the role's original charter permits, using the explicitly supplied metadata path. The plan writer and build worker retain their existing prohibitions on Docket metadata mutation.
- Normal worker gate-driver scratch and capability state are authorized exceptions to feature-artifact placement. Use this fixture's external evidence directory for explicit --run-root paths; retain driver-managed local state under its normal contract. Do not expose capability tokens in reports. These test-driver processes are permitted; substitute agent processes are not.
- Never direct Git to a different checkout using environment variables, alternate indexes, --git-dir, --work-tree, or path aliases. Stop if the assigned root disappears, HEAD moves unexpectedly, or unrelated dirt appears. Recheck identity before staging and committing; entry cleanliness is not required after your own authorized edits.
- You are not alone in the repository. Do not revert or adopt other agents' work. This particular test runs its planner and worker sequentially; it does not certify concurrent-writer safety.

## Coordinator and parent

The primary checkout is permitted for initial orchestration and primary-integrity observations. After allocating the feature worktree, the coordinator explicitly targets that worktree for feature operations and the appropriate repository/metadata path for catalog-resolved Docket operations. Supplying --repo-dir never excuses a conflicting shell workdir. The parent may continue its gate bookkeeping from the primary checkout with an explicit workdir.

Before each child dispatch, verify current Docket ownership and clean expected HEAD, preserve the workspace receipt, and pass the full entry bundle. After each child and at the final halt, run the external primary-audit.py check against the preparation snapshot. This checks tracked and untracked primary source files (including ignored files), branch/HEAD, and index. Only .git, .docket, and .worktrees trees are excluded for necessary Git/metadata/workspace administration. External evidence is separate. Unexpected changes fail the test; do not clean them away.

Final review must also inspect recorded tool arguments, effective command directories, and edited paths. A clean final snapshot cannot exclude temporary misplaced writes that were later undone. Missing operation evidence means binding-incomplete, not passed. Report startup cwd separately from operation cwd; never call this native startup placement or hard filesystem isolation.

## Later production work

If this POC passes, design a Codex-only progressively disclosed reference for the relevant skills and generated agent files. Keep common role behavior and other harnesses neutral. The POC does not install that production change or certify all of change 423.



For the planner payload follow .codex/POC-PLAN-DISPATCH.md. For the standard worker, the exact entry_argv and immutable task inputs take precedence; after entry read .codex/POC-WORKER.md.
