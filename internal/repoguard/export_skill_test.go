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

// assertInlineSkill pins skills/<name>/SKILL.md as an inline skill:
// frontmatter carries exactly name and description (no fork, no agent), the
// name matches, the collapsed description contains each clause, and no agent
// wrapper agents/<name>.md exists. A forked skill could not reach the
// session's own tools or ask the person.
func assertInlineSkill(t *testing.T, name string, clauses []string) {
	t.Helper()
	root := guardRoot(t)
	file := "skills/" + name + "/SKILL.md"
	body := readRepoFile(t, file)
	keys, err := skillFrontmatterKeys(body)
	if err != nil {
		t.Fatalf("%s frontmatter: %v", file, err)
	}
	if want := []string{"name", "description"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("%s frontmatter keys = %v, want exactly %v (an inline skill: no context, no agent)", file, keys, want)
	}
	if got := frontmatterValue(body, "name"); got != name {
		t.Errorf("%s name = %q, want %s", file, got, name)
	}
	desc := collapseWS(frontmatterValue(body, "description"))
	for _, clause := range clauses {
		if !strings.Contains(desc, clause) {
			t.Errorf("%s description lacks %q: %q", file, clause, desc)
		}
	}
	wrapper := filepath.Join(root, "agents", name+".md")
	_, err = os.Stat(wrapper)
	switch {
	case err == nil:
		t.Errorf("agents/%s.md exists; %s is an inline skill with no wrapper", name, name)
	case !errors.Is(err, fs.ErrNotExist):
		t.Errorf("stat agents/%s.md: %v (fail closed)", name, err)
	}
}

// TestExportSkillIsInline pins docket-export as an inline skill.
func TestExportSkillIsInline(t *testing.T) {
	assertInlineSkill(t, "docket-export", []string{
		"export a docket change, spec, plan, or results file",
		"put one into anything outside docket",
	})

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

// TestLinkTicketSkillIsInline pins docket-link-ticket as an inline skill whose
// description names its three asks, disclaims posting updates, and uses the
// ticket vocabulary only.
func TestLinkTicketSkillIsInline(t *testing.T) {
	const name = "docket-link-ticket"
	assertInlineSkill(t, name, []string{
		"link a change in docket to a ticket",
		"create a ticket from a change",
		"make a change from a ticket",
		"not for posting an update",
	})
	body := readRepoFile(t, "skills/"+name+"/SKILL.md")
	desc := strings.ToLower(collapseWS(frontmatterValue(body, "description")))
	for _, word := range []string{"issue", "work item"} {
		if strings.Contains(desc, word) {
			t.Errorf("%s description uses %q; the item is a ticket: %q", name, word, desc)
		}
	}
	// SKILL.md routes the push and import paths to these files; a missing one
	// leaves a blocking read with nothing to read.
	root := guardRoot(t)
	for _, ref := range []string{"references/create.md", "references/import.md"} {
		p := filepath.Join(root, "skills", name, filepath.FromSlash(ref))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s routes to %s, which cannot be read: %v", name, ref, err)
		}
	}
}
