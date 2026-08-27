package app

// These are the real-git integration tests for the workspace prepare/inspect/
// publish app operations: they drive the operations through a real
// workspace.Service over the same bare-remote temporary topologies the planning
// integration tests use (newMainModeRepo / newDocketModeRepo via planRepoModes),
// so the ownership-safe allocation, resume, and CAS-lease publish behavior is
// exercised end-to-end rather than faked. The change record lives on the mode's
// metadata branch; the change resolves its effective base to the integration
// branch (main), so a fresh workspace is created from origin/main's tip.
