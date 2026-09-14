# Codex feature binding

Read only the supplied assignment and checker before repository content. Run the exact `agent.check-inputs` argv at `entry`, then direct every Git and file operation to the assignment's absolute feature root. Startup cwd may be the primary or assigned feature root; cwd is not ownership. The checker binds primary/common directory, registered feature, short branch, pinned entry HEAD, role, mode, resources, and owned paths.

At `active`, recheck before returning. Workers may carry only assigned edits and descendant task commits. Reviewers stay at their pinned review HEAD and never edit or run tests. Planners start clean and write only their artifact. Resolver and integration-repair exceptions require their existing finalize reservation or repair authority; a mode string alone grants nothing.
