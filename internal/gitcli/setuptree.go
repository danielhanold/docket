package gitcli

import (
	"context"
	"errors"
	"os"
	"strings"
)

// Operation labels for the repository setup-tree surface (empty-tree resolution,
// parentless commit creation, temp-index tree composition, root inspection).
const (
	emptyTreeOp  Operation = "empty-tree-oid"
	treeOIDOp    Operation = "tree-oid"
	commitTreeOp Operation = "commit-tree"
	buildTreeOp  Operation = "build-tree"
	rootsOp      Operation = "root-commits"
)

// EmptyTreeOID returns the repository's empty-tree object id via
// `git hash-object -t tree /dev/null`. Computing it this way is
// hash-algorithm-agnostic — a SHA-256 repository returns its own empty-tree id —
// so no truncated or hardcoded SHA-1 literal is ever assumed.
func (c *Client) EmptyTreeOID(ctx context.Context, repo Repository) (ObjectID, error) {
	res, f := c.run(ctx, runRequest{
		op:   emptyTreeOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"hash-object", "-t", "tree", "/dev/null"},
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(emptyTreeOp, KindCommandFailed, "hash-object failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return singleObjectID(emptyTreeOp, res.stdout)
}

// TreeOID resolves <commit>^{tree} to the tree object id it points at.
func (c *Client) TreeOID(ctx context.Context, repo Repository, commit ObjectID) (ObjectID, error) {
	if err := validateObjectID(commit); err != nil {
		return "", newFailure(treeOIDOp, KindInvalidRequest, "invalid commit id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   treeOIDOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"rev-parse", "--verify", string(commit) + "^{tree}"},
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(treeOIDOp, KindCommandFailed, "rev-parse ^{tree} failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return singleObjectID(treeOIDOp, res.stdout)
}

// IncludePrefixOp mounts the source subtree at From^{tree}:Prefix onto the index
// at Prefix/ (`git read-tree --prefix=<Prefix>/ <From>^{tree}:<Prefix>`). An
// absent source prefix is an error, never a silent skip.
type IncludePrefixOp struct {
	From   ObjectID
	Prefix RepoPath
}

// RemovePrefixOp drops every index entry under Prefix
// (`git rm --cached -r --ignore-unmatch -- <Prefix>/`).
type RemovePrefixOp struct {
	Prefix RepoPath
}

// RemovePathOp drops one index entry (`git update-index --force-remove`).
type RemovePathOp struct {
	Path RepoPath
}

// PutBlobOp writes Content as a blob and stages it at Path with Mode
// (`git hash-object -w --stdin` + `git update-index --add --cacheinfo`).
type PutBlobOp struct {
	Path    RepoPath
	Content []byte
	Mode    FileMode
}

// TreeOp is a closed tree-composition instruction applied in order to a private
// temporary index. Exactly one field must be set.
type TreeOp struct {
	IncludePrefix *IncludePrefixOp
	RemovePrefix  *RemovePrefixOp
	RemovePath    *RemovePathOp
	PutBlob       *PutBlobOp
}

// setFieldCount reports how many of the op's mutually exclusive fields are set.
func (op TreeOp) setFieldCount() int {
	n := 0
	if op.IncludePrefix != nil {
		n++
	}
	if op.RemovePrefix != nil {
		n++
	}
	if op.RemovePath != nil {
		n++
	}
	if op.PutBlob != nil {
		n++
	}
	return n
}

// BuildTree composes a tree object by applying ops, in order, to a private
// temporary index (a fresh GIT_INDEX_FILE under an os.MkdirTemp dir, removed on
// return, so no live worktree index is ever touched). When base is empty the
// index starts empty; otherwise it is seeded with `read-tree <base>^{tree}`
// first. Each op must set exactly one field. Returns write-tree's tree OID.
func (c *Client) BuildTree(ctx context.Context, repo Repository, base ObjectID, ops []TreeOp) (ObjectID, error) {
	if base != "" {
		if err := validateObjectID(base); err != nil {
			return "", newFailure(buildTreeOp, KindInvalidRequest, "invalid base id", err)
		}
	}
	for _, op := range ops {
		if op.setFieldCount() != 1 {
			return "", newFailure(buildTreeOp, KindInvalidRequest, "each tree op must set exactly one field", nil)
		}
		if err := validateTreeOp(op); err != nil {
			return "", newFailure(buildTreeOp, KindInvalidRequest, "invalid tree op", err)
		}
	}

	tmp, err := os.MkdirTemp("", "docket-buildtree-")
	if err != nil {
		return "", newFailure(buildTreeOp, KindInvalidRequest, "cannot create temp index dir", err)
	}
	defer os.RemoveAll(tmp)
	indexEnv := []string{"GIT_INDEX_FILE=" + tmp + "/index"}

	if base != "" {
		if f := c.indexGit(ctx, repo, indexEnv, []string{"read-tree", string(base) + "^{tree}"}, nil, "read-tree base"); f != nil {
			return "", f
		}
	}

	for _, op := range ops {
		if f := c.applyTreeOp(ctx, repo, indexEnv, op); f != nil {
			return "", f
		}
	}

	res, f := c.run(ctx, runRequest{
		op:   buildTreeOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"write-tree"},
		env:  indexEnv,
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(buildTreeOp, KindCommandFailed, "write-tree failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return singleObjectID(buildTreeOp, res.stdout)
}

// applyTreeOp executes one composition op against the temp index.
func (c *Client) applyTreeOp(ctx context.Context, repo Repository, indexEnv []string, op TreeOp) *Failure {
	switch {
	case op.IncludePrefix != nil:
		p := string(op.IncludePrefix.Prefix)
		treeish := string(op.IncludePrefix.From) + "^{tree}:" + p
		return c.indexGit(ctx, repo, indexEnv,
			[]string{"read-tree", "--prefix=" + p + "/", treeish}, nil, "read-tree --prefix")
	case op.RemovePrefix != nil:
		p := string(op.RemovePrefix.Prefix)
		return c.indexGit(ctx, repo, indexEnv,
			[]string{"rm", "--cached", "-r", "--ignore-unmatch", "--", p + "/"}, nil, "rm --cached prefix")
	case op.RemovePath != nil:
		return c.indexGit(ctx, repo, indexEnv,
			[]string{"update-index", "--force-remove", "--", string(op.RemovePath.Path)}, nil, "update-index --force-remove")
	case op.PutBlob != nil:
		res, f := c.run(ctx, runRequest{
			op:    buildTreeOp,
			dir:   repo.PrimaryWorktree,
			args:  []string{"hash-object", "-w", "--stdin"},
			stdin: op.PutBlob.Content,
			env:   indexEnv,
		})
		if f != nil {
			return f
		}
		if res.exitCode != 0 {
			return newFailure(buildTreeOp, KindCommandFailed, "hash-object -w failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
		}
		oid, err := singleObjectID(buildTreeOp, res.stdout)
		if err != nil {
			return failureOf(err)
		}
		cacheinfo := string(op.PutBlob.Mode) + "," + string(oid) + "," + string(op.PutBlob.Path)
		return c.indexGit(ctx, repo, indexEnv,
			[]string{"update-index", "--add", "--cacheinfo", cacheinfo}, nil, "update-index --cacheinfo")
	default:
		return newFailure(buildTreeOp, KindInvalidRequest, "empty tree op", nil)
	}
}

// indexGit runs one index-mutating git command against the temp index and maps a
// non-zero exit to a command-failed Failure labelled with what.
func (c *Client) indexGit(ctx context.Context, repo Repository, indexEnv, args []string, stdin []byte, what string) *Failure {
	res, f := c.run(ctx, runRequest{
		op:    buildTreeOp,
		dir:   repo.PrimaryWorktree,
		args:  args,
		stdin: stdin,
		env:   indexEnv,
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(buildTreeOp, KindCommandFailed, what+" failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}

// validateTreeOp checks the paths and ids carried by whichever field is set.
func validateTreeOp(op TreeOp) error {
	switch {
	case op.IncludePrefix != nil:
		if err := validateObjectID(op.IncludePrefix.From); err != nil {
			return err
		}
		return validateRepoPath(op.IncludePrefix.Prefix, false)
	case op.RemovePrefix != nil:
		return validateRepoPath(op.RemovePrefix.Prefix, false)
	case op.RemovePath != nil:
		return validateRepoPath(op.RemovePath.Path, false)
	case op.PutBlob != nil:
		if err := validateRepoPath(op.PutBlob.Path, false); err != nil {
			return err
		}
		return validateFileMode(op.PutBlob.Mode)
	}
	return errors.New("gitcli: empty tree op")
}

// validateFileMode requires a non-empty octal mode text with no comma (which
// would corrupt the --cacheinfo triple) and no control bytes.
func validateFileMode(m FileMode) error {
	s := string(m)
	if s == "" {
		return errors.New("gitcli: empty file mode")
	}
	if strings.IndexByte(s, ',') >= 0 {
		return errors.New("gitcli: file mode contains a comma")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '7' {
			return errors.New("gitcli: file mode is not octal")
		}
	}
	return nil
}

// CommitTree creates a commit object for tree with the given parents (an empty
// slice makes a parentless root) via `git commit-tree`. Repository hooks and
// signing are disabled with the same -c flag set CommitPaths uses
// (core.hooksPath, commit.gpgsign=false), the repository's own configured
// identity is used (a missing identity surfaces as a typed *Failure from git's
// own exit, never a hardcoded person), and the subject plus trailer block are
// composed and validated by the shared composeCommitMessage / validateCommitSubject
// / validateTrailer helpers.
func (c *Client) CommitTree(ctx context.Context, repo Repository, tree ObjectID, parents []ObjectID, subject string, trailers []Trailer) (ObjectID, error) {
	if err := validateObjectID(tree); err != nil {
		return "", newFailure(commitTreeOp, KindInvalidRequest, "invalid tree id", err)
	}
	for _, p := range parents {
		if err := validateObjectID(p); err != nil {
			return "", newFailure(commitTreeOp, KindInvalidRequest, "invalid parent id", err)
		}
	}
	if err := validateCommitSubject(subject); err != nil {
		return "", newFailure(commitTreeOp, KindInvalidRequest, "invalid commit subject", err)
	}
	for _, tr := range trailers {
		if err := validateTrailer(tr); err != nil {
			return "", newFailure(commitTreeOp, KindInvalidRequest, "invalid trailer", err)
		}
	}

	// commit-tree never invokes repository hooks, but the same -c core.hooksPath
	// override CommitPaths carries is passed for parity so no configured hook path
	// can ever be consulted; commit.gpgsign=false defeats a repo that would sign.
	args := []string{
		"-c", "core.hooksPath=",
		"-c", "commit.gpgsign=false",
		"commit-tree", string(tree),
	}
	for _, p := range parents {
		args = append(args, "-p", string(p))
	}
	args = append(args, "-F", "-")

	msg := composeCommitMessage(subject, trailers)
	res, f := c.run(ctx, runRequest{
		op:    commitTreeOp,
		dir:   repo.PrimaryWorktree,
		args:  args,
		stdin: []byte(msg),
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(commitTreeOp, KindCommandFailed, "commit-tree failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return singleObjectID(commitTreeOp, res.stdout)
}

// RootCommits lists the parentless roots reachable from tip
// (`git rev-list --max-parents=0 <tip>`). A caller proves orphan ancestry by
// checking len == 1 and that root's tree/receipt.
func (c *Client) RootCommits(ctx context.Context, repo Repository, tip ObjectID) ([]ObjectID, error) {
	if err := validateObjectID(tip); err != nil {
		return nil, newFailure(rootsOp, KindInvalidRequest, "invalid tip id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   rootsOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"rev-list", "--max-parents=0", string(tip)},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(rootsOp, KindCommandFailed, "rev-list --max-parents=0 failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	lines := stdoutLines(res.stdout)
	roots := make([]ObjectID, 0, len(lines))
	for _, line := range lines {
		id := ObjectID(line)
		if err := validateObjectID(id); err != nil {
			return nil, newFailure(rootsOp, KindInvalidOutput, "rev-list produced a malformed object id", err)
		}
		roots = append(roots, id)
	}
	return roots, nil
}

// singleObjectID parses exactly one validated object id from a git command's
// stdout.
func singleObjectID(op Operation, stdout []byte) (ObjectID, error) {
	lines := stdoutLines(stdout)
	if len(lines) != 1 {
		return "", newFailure(op, KindInvalidOutput, "expected exactly one object id line", nil)
	}
	id := ObjectID(lines[0])
	if err := validateObjectID(id); err != nil {
		return "", newFailure(op, KindInvalidOutput, "malformed object id", err)
	}
	return id, nil
}

// failureOf returns err as a *Failure when it already is one, else wraps it as an
// invalid-output build-tree failure. It lets applyTreeOp return the *Failure that
// singleObjectID already produced without discarding its kind.
func failureOf(err error) *Failure {
	if f, ok := AsFailure(err); ok {
		return f
	}
	return newFailure(buildTreeOp, KindInvalidOutput, err.Error(), err)
}
