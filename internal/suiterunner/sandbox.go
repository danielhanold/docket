// This file owns per-target isolation: the child's HOME/TMPDIR/git-config layout
// and the exact environment overrides the Bash oracle's launch() exports
// (scripts/run-tests.sh). A target runs against a synthetic git identity and no
// interactive prompts, in a private HOME/TMPDIR, so nothing it does touches the
// developer's real home or blocks on a human.
package suiterunner

import (
	"fmt"
	"os"
	"path/filepath"
)

// gitIdentityConfig is the synthetic global git config every target sees: a
// present-but-fake identity (a test that commits must still be able to) and a
// deterministic default branch. Byte-for-byte the oracle's launch() writes this.
const gitIdentityConfig = "[user]\n\tname = docket test\n\temail = test@docket.invalid\n[init]\n\tdefaultBranch = main\n"

// Sandbox builds the isolated child environment for one target under jobdir and
// creates the directories and git-config files it references. It returns the
// full env slice: the base os.Environ() with the isolation overrides appended
// last, so exec (which honors the last value for a duplicated key) uses the
// override. The override set is exactly launch()'s: private HOME/TMPDIR/
// XDG_CONFIG_HOME, synthetic GIT_CONFIG_GLOBAL/SYSTEM, and the no-prompt/
// no-pager/no-autoedit git knobs.
func Sandbox(jobdir string) ([]string, error) {
	home := filepath.Join(jobdir, "home")
	tmp := filepath.Join(jobdir, "tmp")
	configHome := filepath.Join(home, ".config")
	gitGlobal := filepath.Join(home, ".gitconfig")
	gitSystem := filepath.Join(home, ".gitconfig-system")

	for _, d := range []string{configHome, tmp} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("suiterunner: sandbox mkdir %q: %w", d, err)
		}
	}
	// An empty system config, and the synthetic identity as the global config.
	// Written directly (not via `git config`) — it lands at $HOME/.gitconfig so
	// a pre-2.32 git that ignores GIT_CONFIG_GLOBAL still reads exactly this.
	if err := os.WriteFile(gitSystem, nil, 0o644); err != nil {
		return nil, fmt.Errorf("suiterunner: sandbox write system config: %w", err)
	}
	if err := os.WriteFile(gitGlobal, []byte(gitIdentityConfig), 0o644); err != nil {
		return nil, fmt.Errorf("suiterunner: sandbox write global config: %w", err)
	}

	overrides := []string{
		"HOME=" + home,
		"TMPDIR=" + tmp,
		"XDG_CONFIG_HOME=" + configHome,
		"GIT_CONFIG_GLOBAL=" + gitGlobal,
		"GIT_CONFIG_SYSTEM=" + gitSystem,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true",
		"GIT_EDITOR=true",
		"EDITOR=true",
		"VISUAL=true",
		"GIT_PAGER=cat",
		"PAGER=cat",
		"GIT_MERGE_AUTOEDIT=no",
	}
	return append(os.Environ(), overrides...), nil
}
