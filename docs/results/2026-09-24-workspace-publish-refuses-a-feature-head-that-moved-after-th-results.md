<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0451 — Workspace publish refuses a feature head that moved after the app-level check](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-24-0451-workspace-publish-refuses-a-feature-head-that-moved-after-th.md)**
<!-- docket:backlink:end -->
# Workspace publish refuses a feature head that moved after the app-level check — Results

**Human action:** None required. Review the PR diff as usual before merging. The change is small and fully covered by automated tests.

## Outcome

Before this change, `workspace.publish` checked that the feature worktree's head matched the head the caller expected. It then pushed whatever head it found a moment later, when it re-read the worktree under its own lock. If someone committed into the worktree during that short window, the push sent a commit nobody had checked. The remote was never overwritten, because the push is fast-forward-only with a lease. Change 0444 had already made sure such a push could not count as verified evidence.

Now the expected head is passed down to the publish step. It is compared again under the lock, right before the remote is probed. If the head differs, whether it moved forward, was rewound, or was rebased, the publish is refused and nothing is pushed. The PR path (`pr.publish`) already refused a moved head this way, and the two paths now agree:

- The refusal is a `failed` / invalid-state result, the same class the PR path uses.
- At the operation level it carries the same `head-mismatch` reason as the existing pre-lock check. A caller can therefore tell a moved head apart from a dirty or non-ready workspace.
- The run-epoch journal still records a refused publish as completed and unverified, so the 0444 accounting is unchanged.

Passing no expected head keeps the old behavior, so no other caller is affected. `WorkspacePublish` is the only production caller.

## Verification performed

- Focused tests: a moved head is refused with nothing pushed to origin, a matching head still publishes, the app layer passes its checked head down, and the moved-head refusal surfaces as `head-mismatch`. Each new assertion was observed failing before its fix was added, so removing the guard or the wiring turns a test red.
- Whole-branch review found two minor issues, both fixed in-branch: the reason token did not match the pre-lock check, and the refusal message said "moved past", which is wrong for a rewound head.
- The full suite (`go run ./cmd/docket development test`) passed at the build head through the gate driver. The final certification run on the head containing this file is recorded in the PR's build-evidence block.
