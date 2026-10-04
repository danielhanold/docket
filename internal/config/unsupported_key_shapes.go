package config

import (
	"regexp"
	"strings"
)

// UnsupportedKeyShape is one registry-derived spelling of an unsupported
// configuration key: Name is its display name ("skills.build", "skills:",
// "skills.<child>", "cap:") and Re matches the spelling in raw text.
type UnsupportedKeyShape struct {
	Name string
	Re   *regexp.Regexp
}

// unsupportedKeySegClass stands in for a dynamic "*" path segment.
const unsupportedKeySegClass = `[A-Za-z0-9_<>*-]+`

// UnsupportedKeyShapes derives, from paths (normally SettingPaths()), the
// spellings of unsupported configuration keys that documentation must not
// carry. It derives four kinds of shape:
//
//   - each unsupported dotted path;
//   - for a top-level segment with no supported path beneath it, its YAML-key
//     form at a line start (behind any number of comment markers and an
//     optional list dash) or opening a code span;
//   - for the same segment, any dotted child of it;
//   - inside a block that also holds supported keys, an unsupported leaf's
//     YAML-key form at a line start (behind any number of comment markers) or
//     inside a flow mapping, when that leaf name is no segment of any
//     supported path. This restriction keeps `build:`, `review:`, and the
//     change-frontmatter `plan:` (also leaf names under skills.*) from
//     matching.
//
// TestExampleSchemaCorrespondence (Direction C) and repoguard's
// TestLivingDocsAlignment both call it. Regexps are compiled per call, so
// the function has no init-time cost.
func UnsupportedKeyShapes(paths []SettingPath) []UnsupportedKeyShape {
	supportedTop, supportedSeg := map[string]bool{}, map[string]bool{}
	for _, p := range paths {
		if p.Supported {
			segs := strings.Split(p.Path, ".")
			supportedTop[segs[0]] = true
			for _, s := range segs {
				supportedSeg[s] = true
			}
		}
	}
	var shapes []UnsupportedKeyShape
	seenTop, seenLeaf := map[string]bool{}, map[string]bool{}
	for _, p := range paths {
		if p.Supported {
			continue
		}
		segs := strings.Split(p.Path, ".")
		if len(segs) > 1 {
			parts := make([]string, len(segs))
			for i, s := range segs {
				if s == "*" {
					parts[i] = unsupportedKeySegClass
				} else {
					parts[i] = regexp.QuoteMeta(s)
				}
			}
			shapes = append(shapes, UnsupportedKeyShape{p.Path, regexp.MustCompile(`(?:^|[^\w.-])` + strings.Join(parts, `\.`) + `(?:[^\w-]|$)`)})
		}
		top := segs[0]
		if !supportedTop[top] && !seenTop[top] {
			seenTop[top] = true
			q := regexp.QuoteMeta(top)
			shapes = append(shapes, UnsupportedKeyShape{top + ":", regexp.MustCompile("(?m)(?:^[ \\t]*(?:#[ \\t]*)*(?:-[ \\t]+)?|`)" + q + ":")})
			if len(segs) > 1 {
				shapes = append(shapes, UnsupportedKeyShape{top + ".<child>", regexp.MustCompile(`(?:^|[^\w.-])` + q + `\.` + unsupportedKeySegClass)})
			}
		}
		leaf := segs[len(segs)-1]
		if len(segs) > 1 && supportedTop[top] && leaf != "*" && !supportedSeg[leaf] && !seenLeaf[leaf] {
			seenLeaf[leaf] = true
			shapes = append(shapes, UnsupportedKeyShape{leaf + ":", regexp.MustCompile(`(?m)(?:^[ \t]*(?:#[ \t]*)*|[{,][ \t]*)` + regexp.QuoteMeta(leaf) + `:`)})
		}
	}
	return shapes
}
