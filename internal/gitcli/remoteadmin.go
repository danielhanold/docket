package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

const (
	initBareOp  Operation = "init-bare"
	addRemoteOp Operation = "add-remote"
)

// InitBare creates a bare repository at path, creating missing parent
// directories. It is idempotent on an existing bare repository and refuses an
// existing path that is not one. path must be absolute: GIT_DIR is scrubbed
// from the environment, so the target is always passed as an argument.
func (c *Client) InitBare(ctx context.Context, path string) error {
	if !filepath.IsAbs(path) {
		return newFailure(initBareOp, KindInvalidRequest, "bare repository path must be absolute", nil)
	}
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return newFailure(initBareOp, KindInvalidRequest, "not a bare repository", nil)
		}
		res, f := c.run(ctx, runRequest{
			op:   initBareOp,
			dir:  path,
			args: []string{"rev-parse", "--is-bare-repository"},
		})
		if f != nil {
			return f
		}
		if res.exitCode != 0 || strings.TrimSpace(string(res.stdout)) != "true" {
			return newFailure(initBareOp, KindInvalidRequest, "not a bare repository", nil)
		}
		return nil
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return newFailure(initBareOp, KindCommandFailed, "create parent directory", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   initBareOp,
		dir:  parent,
		args: []string{"init", "--bare", "-q", path},
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(initBareOp, KindCommandFailed, "git init --bare failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}

// AddRemote registers remote name with url in repo. It never rewrites an
// existing remote: a name already configured fails as KindCommandFailed, and
// callers compare URLs (RemoteURL) first.
func (c *Client) AddRemote(ctx context.Context, repo Repository, name RemoteName, url string) error {
	if err := validateRemoteName(name); err != nil {
		return newFailure(addRemoteOp, KindInvalidRequest, "invalid remote name", err)
	}
	if url == "" || strings.HasPrefix(url, "-") || strings.ContainsAny(url, "\x00\n") {
		return newFailure(addRemoteOp, KindInvalidRequest, "invalid remote url", nil)
	}
	res, f := c.run(ctx, runRequest{
		op:   addRemoteOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"remote", "add", string(name), url},
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(addRemoteOp, KindCommandFailed, "git remote add failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}
