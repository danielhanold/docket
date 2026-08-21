package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// resolveRepoDir fulfils the advertised "--repo-dir ... (default: current
// directory)" contract at the CLI boundary. A non-empty explicit value is
// returned verbatim; an omitted value resolves through the process working
// directory; a failure to determine the working directory is an argument error
// returned before dependencies are constructed or an operation runs. The
// application and adapter layers keep requiring a concrete directory — empty
// input stays invalid at those boundaries. The resolved directory is the
// invocation working directory; it is never replaced with a primary worktree or
// a Git-discovered root.
func resolveRepoDir(c *cobra.Command) (string, error) {
	dir, _ := c.Flags().GetString("repo-dir")
	if dir != "" {
		return dir, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("--repo-dir omitted and the current directory could not be determined: %w", err)
	}
	return wd, nil
}
