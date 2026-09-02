// Package gitbg is a dependency-free leaf holding the single source of the
// git-background-off config (change 0373). It imports nothing from internal/ so
// that both the suite runner's sandbox (internal/suiterunner) and the shared
// real-process test fixture (internal/testsupport) can consume BackgroundOff
// without forming an import cycle: testsupport is imported from suiterunner's
// own test files, so testsupport must not depend on suiterunner.
package gitbg

// BackgroundOff disables every git mechanism that detaches a child which can
// outlive the invoking command and keep writing into the repository — the
// t.TempDir() "directory not empty" mechanism (change 0373). One source: the
// runner sandbox appends it to the synthetic global config, and
// internal/testsupport embeds it in each fixture's GIT_CONFIG_GLOBAL, so gate
// runs and solo runs agree.
const BackgroundOff = "[gc]\n\tauto = 0\n\tautoDetach = false\n[maintenance]\n\tauto = false\n[core]\n\tfsmonitor = false\n"
