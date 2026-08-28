package gitcli

import (
	"context"
	"os"
	"path/filepath"
)

// disableHooksOp labels every Failure from the worktree hook-disabling surface.
const disableHooksOp Operation = "disable-worktree-hooks"

// worktreeConfigRelocateKeys are the config keys that, once
// extensions.worktreeConfig is enabled, read per-worktree — so a value left in
// the COMMON config would silently stop applying to linked worktrees. They are
// relocated to the main worktree's per-worktree config before the extension is
// enabled, exactly as scripts/disable-worktree-hooks.sh does.
var worktreeConfigRelocateKeys = []string{"core.worktree", "core.bare"}

// DisableWorktreeHooks disables git hooks on one docket-owned worktree,
// idempotently, so docket's bookkeeping commits skip the repository's shared hook
// framework. It ports scripts/disable-worktree-hooks.sh: ensure
// extensions.worktreeConfig=true (relocating any core.worktree/core.bare value
// from the common config to the main worktree's per-worktree config first, and
// failing closed with the extension rolled back if that cannot be done safely),
// create an empty absolute hooks directory under the worktree's common git dir,
// and point this worktree's core.hooksPath at it with `git config --worktree`.
func (c *Client) DisableWorktreeHooks(ctx context.Context, worktreeDir string) error {
	if worktreeDir == "" {
		return newFailure(disableHooksOp, KindInvalidRequest, "worktree dir is empty", nil)
	}
	if info, err := os.Stat(worktreeDir); err != nil || !info.IsDir() {
		return newFailure(disableHooksOp, KindInvalidRequest, "worktree dir not found", err)
	}

	// Resolve the common git dir relative to the worktree, then canonicalize it —
	// the script's `cd "$WT" && cd "$(git rev-parse --git-common-dir)" && pwd -P`.
	commonRes, f := c.run(ctx, runRequest{
		op:   disableHooksOp,
		dir:  worktreeDir,
		args: []string{"rev-parse", "--git-common-dir"},
	})
	if f != nil {
		return f
	}
	if commonRes.exitCode != 0 {
		return newFailure(disableHooksOp, KindCommandFailed, "cannot resolve git common dir: "+stderrExcerpt(commonRes.stderr), nil).withExitCode(commonRes.exitCode)
	}
	commonLines := stdoutLines(commonRes.stdout)
	if len(commonLines) != 1 {
		return newFailure(disableHooksOp, KindInvalidOutput, "unexpected git-common-dir output", nil)
	}
	common, err := resolveGitPath(commonLines[0], worktreeDir)
	if err != nil {
		return newFailure(disableHooksOp, KindInvalidOutput, "cannot canonicalize git common dir", err)
	}

	// Absolute, empty, docket-owned hooks dir under the common git dir — never
	// tracked, never leaks into a commit, and a real (empty) dir avoids
	// "hooksPath does not exist" surprises.
	empty := filepath.Join(common, "docket", "empty-hooks")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		return newFailure(disableHooksOp, KindCommandFailed, "cannot create empty hooks dir", err)
	}

	if err := c.ensureWorktreeConfig(ctx, worktreeDir); err != nil {
		return err
	}

	// Point THIS worktree's hook lookup at the empty dir (worktree-scoped).
	// Idempotent: a repeat write is the same value, and --worktree replaces
	// rather than appends, so there is never a duplicate entry.
	setRes, f := c.run(ctx, runRequest{
		op:   disableHooksOp,
		dir:  worktreeDir,
		args: []string{"config", "--worktree", "core.hooksPath", empty},
	})
	if f != nil {
		return f
	}
	if setRes.exitCode != 0 {
		return newFailure(disableHooksOp, KindCommandFailed, "cannot set core.hooksPath: "+stderrExcerpt(setRes.stderr), nil).withExitCode(setRes.exitCode)
	}
	return nil
}

// ensureWorktreeConfig enables extensions.worktreeConfig when it is not already
// on, relocating any per-worktree-sensitive value out of the common config
// first, and rolling the enable back if a needed relocation cannot be done
// safely (fail closed). It is a no-op when the extension is already true.
func (c *Client) ensureWorktreeConfig(ctx context.Context, worktreeDir string) error {
	enabled, f := c.configGet(ctx, worktreeDir, "extensions.worktreeConfig")
	if f != nil {
		return f
	}
	if enabled == "true" {
		return nil
	}

	// git lists the MAIN worktree first, so its path is the first "worktree "
	// stanza header in the porcelain output.
	listRes, f := c.run(ctx, runRequest{
		op:   disableHooksOp,
		dir:  worktreeDir,
		args: []string{"worktree", "list", "--porcelain", "-z"},
	})
	if f != nil {
		return f
	}
	// git lists the main worktree first; firstWorktreePath (discover.go) parses
	// the NUL-terminated porcelain. A false "ok" leaves mainWt empty, which the
	// relocation path treats as fail-closed if a value actually needs moving.
	mainWt, _ := firstWorktreePath(listRes.stdout)

	// git requires the extension enabled BEFORE any --worktree write, so enable it
	// once up front; a failed relocation below rolls it back.
	if enRes, f := c.run(ctx, runRequest{
		op:   disableHooksOp,
		dir:  worktreeDir,
		args: []string{"config", "extensions.worktreeConfig", "true"},
	}); f != nil {
		return f
	} else if enRes.exitCode != 0 {
		return newFailure(disableHooksOp, KindCommandFailed, "cannot enable worktreeConfig: "+stderrExcerpt(enRes.stderr), nil).withExitCode(enRes.exitCode)
	}

	for _, key := range worktreeConfigRelocateKeys {
		val, f := c.configGet(ctx, worktreeDir, key)
		if f != nil {
			return f
		}
		if val == "" {
			continue
		}
		// false is git's harmless default for core.bare — leave it in common config.
		if key == "core.bare" && val != "true" {
			continue
		}
		if err := c.relocateCommonKey(ctx, worktreeDir, mainWt, key, val); err != nil {
			// Roll the enable back so the repo is never left with the extension on
			// but a now-ignored core.worktree/core.bare stranded in common config.
			_, _ = c.run(ctx, runRequest{
				op:   disableHooksOp,
				dir:  worktreeDir,
				args: []string{"config", "--local", "--unset", "extensions.worktreeConfig"},
			})
			return err
		}
	}
	return nil
}

// relocateCommonKey moves one common-config key value to the main worktree's
// per-worktree config and unsets it from the common config; either step failing
// (or an unknown main worktree) is a fail-closed error.
func (c *Client) relocateCommonKey(ctx context.Context, worktreeDir, mainWt, key, val string) error {
	if mainWt == "" {
		return newFailure(disableHooksOp, KindInvalidRepository,
			"refusing to enable worktreeConfig — common "+key+" present and the main worktree could not be resolved", nil)
	}
	if setRes, f := c.run(ctx, runRequest{
		op:   disableHooksOp,
		dir:  mainWt,
		args: []string{"config", "--worktree", key, val},
	}); f != nil {
		return f
	} else if setRes.exitCode != 0 {
		return newFailure(disableHooksOp, KindInvalidRepository,
			"refusing to enable worktreeConfig — common "+key+" could not be relocated safely", nil).withExitCode(setRes.exitCode)
	}
	if unsetRes, f := c.run(ctx, runRequest{
		op:   disableHooksOp,
		dir:  worktreeDir,
		args: []string{"config", "--local", "--unset", key},
	}); f != nil {
		return f
	} else if unsetRes.exitCode != 0 {
		return newFailure(disableHooksOp, KindInvalidRepository,
			"refusing to enable worktreeConfig — common "+key+" could not be unset after relocation", nil).withExitCode(unsetRes.exitCode)
	}
	return nil
}

// WorktreeHooksDisabled reports whether one docket-owned worktree has its git
// hooks disabled the way DisableWorktreeHooks leaves them: this worktree's
// per-worktree core.hooksPath points at an existing directory. It is the
// read-only mirror of DisableWorktreeHooks — a bounded probe that performs no
// write — so a health read can confirm the effect without re-applying it. A
// worktree without extensions.worktreeConfig enabled, or without a per-worktree
// core.hooksPath, reports false (not disabled); a start failure is an error.
func (c *Client) WorktreeHooksDisabled(ctx context.Context, worktreeDir string) (bool, error) {
	if worktreeDir == "" {
		return false, newFailure(disableHooksOp, KindInvalidRequest, "worktree dir is empty", nil)
	}
	res, f := c.run(ctx, runRequest{
		op:   disableHooksOp,
		dir:  worktreeDir,
		args: []string{"config", "--worktree", "--get", "core.hooksPath"},
	})
	if f != nil {
		return false, f
	}
	if res.exitCode != 0 {
		// Value absent, or extensions.worktreeConfig not enabled: hooks are not
		// disabled the docket way.
		return false, nil
	}
	lines := stdoutLines(res.stdout)
	if len(lines) == 0 || lines[0] == "" {
		return false, nil
	}
	info, err := os.Stat(lines[0])
	if err != nil || !info.IsDir() {
		return false, nil
	}
	return true, nil
}

// configGet reads a single local config value, mapping git's "not found" exit
// (the value is simply absent) to the empty string — the imperative-effect
// counterpart of the script's `|| true`. A start failure is still a *Failure.
func (c *Client) configGet(ctx context.Context, dir, key string) (string, *Failure) {
	res, f := c.run(ctx, runRequest{
		op:   disableHooksOp,
		dir:  dir,
		args: []string{"config", "--local", "--get", key},
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", nil
	}
	lines := stdoutLines(res.stdout)
	if len(lines) == 0 {
		return "", nil
	}
	return lines[0], nil
}
