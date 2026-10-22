package repoguard

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// frontmatterKeyLine matches a top-level frontmatter key at column 0.
var frontmatterKeyLine = regexp.MustCompile(`^([a-z_]+):`)

// skillFrontmatterKeys returns the top-level keys between the first `---` line
// and the next `---` line, in file order. It errors when the content does not
// open with `---` or the frontmatter has no closing fence.
func skillFrontmatterKeys(content string) ([]string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return nil, errors.New("no opening frontmatter fence")
	}
	var keys []string
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if line == "---" {
			return keys, nil
		}
		if m := frontmatterKeyLine.FindStringSubmatch(line); m != nil {
			keys = append(keys, m[1])
		}
	}
	return nil, errors.New("no closing frontmatter fence")
}

// frontmatterValue returns the one-line value of key in the first frontmatter block.
func frontmatterValue(content, key string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if line == "---" {
			break
		}
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}

// TestExportSkillIsInline pins docket-export as an inline skill: frontmatter
// carries exactly name and description (no fork, no agent), and no agent
// wrapper exists. A forked skill could not reach the session's posting tools
// or ask the person.
func TestExportSkillIsInline(t *testing.T) {
	root := guardRoot(t)
	body := readRepoFile(t, "skills/docket-export/SKILL.md")
	keys, err := skillFrontmatterKeys(body)
	if err != nil {
		t.Fatalf("skills/docket-export/SKILL.md frontmatter: %v", err)
	}
	if want := []string{"name", "description"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("frontmatter keys = %v, want exactly %v (an inline skill: no context, no agent)", keys, want)
	}
	if got := frontmatterValue(body, "name"); got != "docket-export" {
		t.Errorf("name = %q, want docket-export", got)
	}
	desc := collapseWS(frontmatterValue(body, "description"))
	for _, clause := range []string{
		"export a docket change, spec, plan, or results file",
		"put one into anything outside docket",
	} {
		if !strings.Contains(desc, clause) {
			t.Errorf("description lacks %q: %q", clause, desc)
		}
	}
	_, err = os.Stat(filepath.Join(root, "agents", "docket-export.md"))
	switch {
	case err == nil:
		t.Errorf("agents/docket-export.md exists; docket-export is an inline skill with no wrapper")
	case !errors.Is(err, fs.ErrNotExist):
		t.Errorf("stat agents/docket-export.md: %v (fail closed)", err)
	}

	t.Run("non_vacuity", func(t *testing.T) {
		got, err := skillFrontmatterKeys("---\nname: x\ndescription: y\ncontext: fork\nagent: x\n---\nbody\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []string{"name", "description", "context", "agent"}; !reflect.DeepEqual(got, want) {
			t.Errorf("keys = %v, want %v", got, want)
		}
		if _, err := skillFrontmatterKeys("name: x\n"); err == nil {
			t.Errorf("no opening fence was accepted")
		}
		if _, err := skillFrontmatterKeys("---\nname: x\n"); err == nil {
			t.Errorf("no closing fence was accepted")
		}
	})
}
