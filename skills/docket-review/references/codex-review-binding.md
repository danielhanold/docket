# Codex review binding

Validate the assignment at entry and require `entry_head` to equal the full `review_head`. The named `build_evidence` must identify a declared resource whose bytes still match its SHA-256. Require a clean assigned feature root and re-resolve its branch to that immutable HEAD. Do not create a child scope, run the test suite, edit files, or invoke a worker capability. If the branch, evidence bytes, or assignment changes, refuse the review; fixes require new inputs and a fresh review.
