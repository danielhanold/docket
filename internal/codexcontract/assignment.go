package codexcontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danielhanold/docket/internal/gatedrive"
)

type Resource struct {
	LogicalID string `json:"logical_id"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Source    string `json:"source"`
}

type Assignment struct {
	SchemaVersion        int                    `json:"schema_version" docket:"required"`
	ChangeID             int                    `json:"change_id" docket:"required"`
	Role                 string                 `json:"role" docket:"required,enum=agent_roles"`
	Phase                string                 `json:"phase" docket:"required,enum=assignment_phases"`
	TaskID               string                 `json:"task_id,omitempty"`
	Mode                 string                 `json:"mode" docket:"required,enum=assignment_modes"`
	Primary              string                 `json:"primary" docket:"required"`
	Feature              string                 `json:"feature" docket:"required"`
	CommonDir            string                 `json:"common_dir" docket:"required"`
	Branch               string                 `json:"branch" docket:"required"`
	EntryHEAD            string                 `json:"entry_head" docket:"required"`
	MetadataRevision     string                 `json:"metadata_revision" docket:"required"`
	ChangePath           string                 `json:"change_path" docket:"required"`
	ArtifactPath         string                 `json:"artifact_path,omitempty"`
	DocketExecutable     string                 `json:"docket_executable" docket:"required"`
	DocketCommit         string                 `json:"docket_commit" docket:"required"`
	Resources            []Resource             `json:"resources" docket:"required"`
	ResourceDependencies map[string][]string    `json:"resource_dependencies,omitempty"`
	ReadRoots            []string               `json:"read_roots"`
	WritePaths           []string               `json:"write_paths"`
	InheritedPaths       []string               `json:"inherited_paths"`
	InheritedFingerprint *gatedrive.Fingerprint `json:"inherited_fingerprint,omitempty"`
	RootIdentity         *RootIdentity          `json:"root_identity,omitempty"`
	TestArgv             []string               `json:"test_argv,omitempty"`
	RunRoot              string                 `json:"run_root,omitempty"`
	PlanSkill            string                 `json:"plan_skill,omitempty"`
	BuildSkill           string                 `json:"build_skill,omitempty"`
	LearningsEnabled     bool                   `json:"learnings_enabled,omitempty"`
	LearningsIndex       string                 `json:"learnings_index,omitempty"`
	ReviewBase           string                 `json:"review_base,omitempty"`
	ReviewHEAD           string                 `json:"review_head,omitempty"`
	BuildEvidence        string                 `json:"build_evidence,omitempty"`
}

var (
	AllAgentRoles = []string{
		"docket-plan-writer",
		"docket-build-economy", "docket-build-standard", "docket-build-premium", "docket-build-max",
		"docket-review-lean", "docket-review-standard", "docket-review-deep",
		"docket-rebase-resolver", "docket-integration-repair",
	}
	AllAssignmentPhases = []string{"plan", "build", "review", "resolver", "repair"}
	AllAssignmentModes  = []string{"fresh", "continuation", "escalation", "review", "resolver", "repair"}
)

func ReadAssignment(path, digest string) (Assignment, error) {
	if !filepath.IsAbs(path) {
		return Assignment{}, fmt.Errorf("assignment path must be absolute")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Assignment{}, err
	}
	if len(b) > 1<<20 {
		return Assignment{}, fmt.Errorf("assignment exceeds 1 MiB")
	}
	want, err := hex.DecodeString(digest)
	if err != nil || len(want) != sha256.Size {
		return Assignment{}, fmt.Errorf("assignment sha256 is invalid")
	}
	got := sha256.Sum256(b)
	if !bytes.Equal(want, got[:]) {
		return Assignment{}, fmt.Errorf("assignment sha256 mismatch")
	}
	if err := rejectDuplicateKeys(b); err != nil {
		return Assignment{}, err
	}
	var a Assignment
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return Assignment{}, fmt.Errorf("invalid assignment: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return Assignment{}, err
	}
	if err := ValidateAssignment(a); err != nil {
		return Assignment{}, err
	}
	return a, nil
}

var objectID = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

func ValidateAssignment(a Assignment) error {
	if a.SchemaVersion != 1 {
		return fmt.Errorf("unsupported assignment schema_version %d", a.SchemaVersion)
	}
	if a.ChangeID <= 0 || a.Role == "" || a.Phase == "" || a.Branch == "" {
		return fmt.Errorf("assignment identity is incomplete")
	}
	if !oneOf(a.Mode, AllAssignmentModes...) {
		return fmt.Errorf("unknown assignment mode %q", a.Mode)
	}
	if !validRolePhaseMode(a.Role, a.Phase, a.Mode) {
		return fmt.Errorf("role, phase, and mode are inconsistent")
	}
	for name, p := range map[string]string{"primary": a.Primary, "feature": a.Feature, "common_dir": a.CommonDir, "docket_executable": a.DocketExecutable} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return fmt.Errorf("%s must be an absolute clean path", name)
		}
	}
	if a.Primary == a.Feature {
		return fmt.Errorf("feature must not be the primary worktree")
	}
	if !objectID.MatchString(a.EntryHEAD) || !objectID.MatchString(a.MetadataRevision) || !objectID.MatchString(a.DocketCommit) {
		return fmt.Errorf("assignment revisions must be full object ids")
	}
	if !safeRelative(a.ChangePath) || (a.ArtifactPath != "" && !safeRelative(a.ArtifactPath)) {
		return fmt.Errorf("assignment artifact paths must be safe repository-relative paths")
	}
	for _, p := range append(append([]string{}, a.WritePaths...), a.InheritedPaths...) {
		if !safeRelative(p) {
			return fmt.Errorf("owned path %q is unsafe", p)
		}
	}
	for _, p := range a.ReadRoots {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return fmt.Errorf("read root %q is not absolute and clean", p)
		}
	}
	if !withinReadRoots(a.ReadRoots, a.DocketExecutable) {
		return fmt.Errorf("docket_executable is outside declared read roots")
	}
	if (strings.Contains(a.Role, "build") || a.Mode == "continuation" || a.Mode == "escalation") && a.TaskID == "" {
		return fmt.Errorf("worker assignment requires task_id")
	}
	if (a.Mode == "continuation" || a.Mode == "escalation") && a.InheritedFingerprint == nil {
		return fmt.Errorf("continued worker assignment requires inherited_fingerprint")
	}
	if a.Mode == "review" {
		if !objectID.MatchString(a.ReviewBase) || !objectID.MatchString(a.ReviewHEAD) || a.EntryHEAD != a.ReviewHEAD || a.BuildEvidence == "" {
			return fmt.Errorf("review assignment requires identical full entry/review head, a full base, and evidence")
		}
		foundEvidence := false
		for _, r := range a.Resources {
			if r.LogicalID == a.BuildEvidence {
				foundEvidence = true
				break
			}
		}
		if !foundEvidence {
			return fmt.Errorf("review build_evidence must name a declared hashed resource")
		}
	}
	for _, r := range a.Resources {
		joined := strings.ToLower(r.LogicalID + " " + r.Source)
		if strings.Contains(joined, "gate_context") || strings.Contains(joined, "parent_cap") || strings.Contains(joined, "child_cap") {
			return fmt.Errorf("resource %q contains forbidden live authority", r.LogicalID)
		}
	}
	return nil
}

func validRolePhaseMode(role, phase, mode string) bool {
	if !oneOf(role, AllAgentRoles...) {
		return false
	}
	switch {
	case role == "docket-plan-writer":
		return phase == "plan" && mode == "fresh"
	case strings.HasPrefix(role, "docket-build-"):
		return phase == "build" && oneOf(mode, "fresh", "continuation", "escalation")
	case strings.HasPrefix(role, "docket-review-"):
		return phase == "review" && mode == "review"
	case role == "docket-rebase-resolver":
		return phase == "resolver" && mode == "resolver"
	case role == "docket-integration-repair":
		return phase == "repair" && mode == "repair"
	default:
		return false
	}
}

func withinReadRoots(roots []string, path string) bool {
	for _, root := range roots {
		if pathWithin(root, path) {
			return true
		}
	}
	return false
}

func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func safeRelative(p string) bool {
	return p != "" && !filepath.IsAbs(p) && filepath.Clean(p) == p && p != "." && p != ".." && !strings.HasPrefix(p, ".."+string(filepath.Separator))
}

func ensureJSONEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func rejectDuplicateKeys(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				k, ok := kt.(string)
				if !ok {
					return fmt.Errorf("object key is not a string")
				}
				if seen[k] {
					return fmt.Errorf("duplicate JSON key %q", k)
				}
				seen[k] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		case '[':
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		}
		return nil
	}
	if err := walk(); err != nil {
		return fmt.Errorf("invalid assignment JSON: %w", err)
	}
	return nil
}
