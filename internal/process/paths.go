package process

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// runIDPattern is the run-ID shape: 32 lowercase hex characters (128 bits).
var runIDPattern = regexp.MustCompile("^[0-9a-f]{32}$")

// LaunchRequest is the validated input to a gate launch. It moves here so the
// launch state machine (Task 6) and the validators share one definition.
type LaunchRequest struct {
	Root string
	Cwd  string
	Argv []string
}

// validateLaunchRequest refuses a request with FailInvalidInput unless the
// root is absolute, the cwd is an absolute existing directory, and argv has a
// non-empty argv0. It reads only the request; it never touches the run slot.
func validateLaunchRequest(req LaunchRequest) error {
	if !filepath.IsAbs(req.Root) {
		return failf(FailInvalidInput, "validate-launch", "root must be an absolute path")
	}
	if !filepath.IsAbs(req.Cwd) {
		return failf(FailInvalidInput, "validate-launch", "cwd must be an absolute path")
	}
	fi, err := os.Stat(req.Cwd)
	if err != nil || !fi.IsDir() {
		return failf(FailInvalidInput, "validate-launch", "cwd must be an existing directory")
	}
	if len(req.Argv) < 1 || req.Argv[0] == "" {
		return failf(FailInvalidInput, "validate-launch", "argv must include a non-empty program")
	}
	return nil
}

// resolveRunDir proves clause 1 of the spec's ownership conjunction:
// containment. Both root and the run dir's parent are canonicalised with
// filepath.EvalSymlinks on every hop (an absolute symlink target is still a
// spelling), the run slot is Lstat'd so a symlink there is refused rather than
// followed, and the base name must be a run id. Shape violations are
// FailInvalidInput; a symlinked run slot or a containment breach is
// FailBlocked.
func resolveRunDir(root, runDir string) (string, string, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(runDir) {
		return "", "", failf(FailInvalidInput, "resolve-run-dir", "root and run dir must be absolute")
	}
	canonRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", failf(FailExternal, "resolve-run-dir", "canonicalising root: %v", err)
	}
	li, err := os.Lstat(runDir)
	if err != nil {
		return "", "", failf(FailExternal, "resolve-run-dir", "inspecting run dir: %v", err)
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return "", "", failf(FailBlocked, "resolve-run-dir", "run slot is a symlink")
	}
	if !li.IsDir() {
		return "", "", failf(FailInvalidInput, "resolve-run-dir", "run path is not a directory")
	}
	canonParent, err := filepath.EvalSymlinks(filepath.Dir(runDir))
	if err != nil {
		return "", "", failf(FailExternal, "resolve-run-dir", "canonicalising parent: %v", err)
	}
	if canonParent != canonRoot {
		return "", "", failf(FailBlocked, "resolve-run-dir", "run dir is not an immediate child of the root")
	}
	id := filepath.Base(runDir)
	if !runIDPattern.MatchString(id) {
		return "", "", failf(FailInvalidInput, "resolve-run-dir", "directory name is not a run id")
	}
	return filepath.Join(canonParent, id), id, nil
}

// boundReason makes an arbitrary string safe to embed in a protocol reason:
// whitespace runs collapse to single spaces, remaining control characters are
// dropped, and the result is truncated to 200 runes. The truncation is on
// runes, never bytes — a byte-offset cut through multibyte text splits runes.
func boundReason(s string) string {
	flattened := strings.Join(strings.Fields(s), " ")
	var b strings.Builder
	for _, r := range flattened {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	runes := []rune(b.String())
	if len(runes) > 200 {
		runes = runes[:200]
	}
	return string(runes)
}
