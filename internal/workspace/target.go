package workspace

import (
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// Target is the fully validated description of the one feature branch a
// workspace operation acts on. Every field is validated by NewTarget:
// FeatureRef is always refs/heads/ + the caller-supplied recorded branch,
// validated here — never derived from the slug — so a feature ref that is not a
// well-formed branch ref is unrepresentable, and BaseRef is the base branch
// qualified once as refs/heads/<branch>.
type Target struct {
	ChangeID   domain.ChangeID
	Slug       string
	FeatureRef gitcli.RefName       // refs/heads/<recorded branch>, validated — supplied from the change record
	Base       domain.EffectiveBase // must be Kind == domain.BaseResolved
	BaseRef    gitcli.RefName       // refs/heads/<base branch>, validated once here
}

// FeatureBranch returns the short feature branch — the recorded branch the
// caller supplied to NewTarget — i.e. FeatureRef without its refs/heads/ prefix.
func (t Target) FeatureBranch() string {
	return strings.TrimPrefix(string(t.FeatureRef), "refs/heads/")
}

// NewTarget validates its inputs and qualifies the feature and base refs. It
// rejects a non-positive change id, an invalid slug (domain.ValidSlugToken), a
// base whose kind is not domain.BaseResolved, an empty or malformed base
// branch, and an empty, ref-qualified, or malformed featureBranch. The base and
// feature branches are each qualified once to refs/heads/<branch> and checked
// against gitcli's ref rules; an empty branch is NEVER treated as the
// integration branch. featureBranch is the recorded feature branch the caller
// supplies from the change record — NewTarget validates it here and never
// derives it from the slug. The caller obtains base from the landed resolver
// (domain.ResolveEffectiveBase); NewTarget spends that decision and shadows
// none of its rules.
func NewTarget(id domain.ChangeID, slug string, base domain.EffectiveBase, featureBranch string) (Target, error) {
	if id <= 0 {
		return Target{}, invalidTarget(fmt.Sprintf("non-positive change id %d", int(id)))
	}
	if !domain.ValidSlugToken(slug) {
		return Target{}, invalidTarget("invalid slug")
	}
	if base.Kind != domain.BaseResolved {
		return Target{}, invalidTarget(fmt.Sprintf("effective base is not resolved (kind %q)", base.Kind))
	}
	if base.Branch == "" {
		return Target{}, invalidTarget("resolved base has empty branch")
	}
	// A base branch is a short branch name; an already-qualified value smuggles a
	// second refs/heads/ segment and is rejected before qualification.
	if strings.HasPrefix(base.Branch, "refs/") {
		return Target{}, invalidTarget("base branch is already ref-qualified")
	}
	baseRef := gitcli.RefName("refs/heads/" + base.Branch)
	if err := validBranchRef(baseRef); err != nil {
		return Target{}, invalidTarget("malformed base branch")
	}

	// The feature branch is the recorded branch the caller supplies; it is
	// validated here and never derived from the slug. A ref-qualified value
	// smuggles a second refs/heads/ segment and is rejected before qualification.
	if featureBranch == "" {
		return Target{}, invalidTarget("empty feature branch")
	}
	if strings.HasPrefix(featureBranch, "refs/") {
		return Target{}, invalidTarget("feature branch is already ref-qualified")
	}
	// A leading dash on the SHORT branch is an option-injection hazard whenever it
	// is used bare (e.g. `git push <remote> <branch>`); validBranchRef sees only
	// the qualified ref, whose leading segment is refs/, so it cannot catch it.
	if strings.HasPrefix(featureBranch, "-") {
		return Target{}, invalidTarget("feature branch has a leading dash")
	}
	featureRef := gitcli.RefName("refs/heads/" + featureBranch)
	if err := validBranchRef(featureRef); err != nil {
		return Target{}, invalidTarget("malformed feature branch")
	}

	return Target{
		ChangeID:   id,
		Slug:       slug,
		FeatureRef: featureRef,
		Base:       base,
		BaseRef:    baseRef,
	}, nil
}

// invalidTarget builds the invalid-input Failure NewTarget returns for every
// rejection. Op is "prepare" — NewTarget is the validate stage of a prepare.
func invalidTarget(detail string) *Failure {
	return &Failure{Op: "prepare", Stage: "validate", Kind: KindInvalidInput, Detail: detail}
}

// validBranchRef checks a fully qualified refs/heads/... ref against the same
// rules gitcli's validateRefName enforces (gitcli keeps that helper private, so
// the constructor mirrors its shape-based rules to reject a base branch before
// it reaches a Git process). Kept shape-based, not an enumerated spelling list.
func validBranchRef(r gitcli.RefName) error {
	s := string(r)
	if s == "" {
		return fmt.Errorf("empty ref")
	}
	if strings.HasPrefix(s, "-") {
		return fmt.Errorf("leading dash")
	}
	if strings.IndexByte(s, 0) >= 0 {
		return fmt.Errorf("NUL")
	}
	if strings.ContainsAny(s, " \t\r\n\v\f") {
		return fmt.Errorf("whitespace")
	}
	if strings.Contains(s, "@{") {
		return fmt.Errorf("@{")
	}
	if strings.Contains(s, "\\") {
		return fmt.Errorf("backslash")
	}
	if strings.Contains(s, "*") {
		return fmt.Errorf("glob")
	}
	if !strings.HasPrefix(s, "refs/") {
		return fmt.Errorf("not refs/-qualified")
	}
	comps := strings.Split(s, "/")
	if len(comps) < 3 {
		return fmt.Errorf("too few components")
	}
	for _, c := range comps {
		switch {
		case c == "":
			return fmt.Errorf("empty component")
		case c == "." || c == "..":
			return fmt.Errorf(". or .. component")
		case strings.HasPrefix(c, "."):
			return fmt.Errorf("leading-dot component")
		case strings.Contains(c, ".."):
			return fmt.Errorf(".. sequence")
		case strings.HasSuffix(c, ".lock"):
			return fmt.Errorf(".lock component")
		}
	}
	return nil
}
