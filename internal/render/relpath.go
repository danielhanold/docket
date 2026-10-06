package render

import (
	"path"
	"strings"
)

// RelativeLink returns the slash-separated relative path from the directory of
// fromFile to toFile, both canonical repo-relative paths. It is the link a
// Markdown file on one branch uses to reach another file on the same branch:
// it resolves both on GitHub (against the branch being viewed) and in a local
// checkout, and path.Join(path.Dir(fromFile), RelativeLink(fromFile, toFile))
// is toFile.
func RelativeLink(fromFile, toFile string) string {
	fromDir := splitDir(path.Dir(fromFile))
	toParts := strings.Split(toFile, "/")
	toDir := toParts[:len(toParts)-1]

	common := 0
	for common < len(fromDir) && common < len(toDir) && fromDir[common] == toDir[common] {
		common++
	}
	out := make([]string, 0, len(fromDir)-common+len(toParts)-common)
	for range fromDir[common:] {
		out = append(out, "..")
	}
	out = append(out, toParts[common:]...)
	return strings.Join(out, "/")
}

// splitDir splits a repo-relative directory into its segments; the repository
// root (path.Dir's ".") has none.
func splitDir(dir string) []string {
	if dir == "." || dir == "" {
		return nil
	}
	return strings.Split(dir, "/")
}
