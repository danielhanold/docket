package app

import "testing"

// schemaKeyPaths flattens every request/result key of the live schema into
// "<op> REQ|RES <dotted.path>" strings.
func schemaKeyPaths(t *testing.T) map[string]bool {
	t.Helper()
	doc, err := Schema(nil)
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	out := map[string]bool{}
	var walk func(scope, prefix string, fs []FieldDescriptor)
	walk = func(scope, prefix string, fs []FieldDescriptor) {
		for _, f := range fs {
			out[scope+" "+prefix+f.Key] = true
			walk(scope, prefix+f.Key+".", f.Fields)
		}
	}
	for _, op := range doc.Operations {
		if op.Request != nil {
			walk(op.ID+" REQ", "", op.Request.Fields)
		}
		walk(op.ID+" RES", "", op.Result.Fields)
	}
	return out
}

// TestSchemaRevisionKeys is the golden schema check for ADR-0129 rows 41-44
// (change 0472): each record/PR revision key is spelled revision. The
// whole-schema negative is TestRetiredVocabularySeal (internal/repoguard).
func TestSchemaRevisionKeys(t *testing.T) {
	keys := schemaKeyPaths(t)
	for _, p := range []string{
		"adr.record REQ change.revision",
		"adr.reverse REQ target.revision", "adr.reverse REQ successor.change.revision",
		"adr.supersede REQ target.revision", "adr.supersede REQ successor.change.revision",
		"change.attach-plan REQ revision", "change.attach-results REQ revision",
		"change.block REQ revision", "change.claim REQ revision", "change.defer REQ revision",
		"change.groom REQ revision", "change.groom REQ spec_revision",
		"change.halt REQ revision", "change.kill REQ revision", "change.mark-implemented REQ revision",
		"change.reclaim REQ revision", "change.reconcile REQ revision", "change.refresh-claim REQ revision",
		"change.relink REQ ExpectRevision",
		"change.resume-halted REQ revision", "change.revive REQ revision", "change.unblock REQ revision",
		"context.finalize RES candidates.revision", "context.finalize RES candidates.pr.revision",
		"context.implementation RES context.change.revision", "context.implementation RES context.spec.revision",
		"finalize.block REQ revision", "finalize.clear-block REQ revision",
		"finalize.merge REQ revision", "finalize.merge RES merge.pr_revision",
		"finalize.rebase REQ revision",
		"finalize.retarget-children REQ revision", "finalize.retarget-children REQ children.pr_revision",
		"learning.update REQ revision",
		"status RES changes.revision", "status RES records.revision",
		"workspace.inspect REQ revision", "workspace.prepare REQ revision",
	} {
		if !keys[p] {
			t.Errorf("schema lacks %s", p)
		}
	}
}

// TestSchemaVocabularyRevisionCodes: the finding_codes vocabulary lists the
// row-44a codes in their revision spelling and none of the retired ones.
func TestSchemaVocabularyRevisionCodes(t *testing.T) {
	doc, err := Schema(nil)
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	members := map[string]bool{}
	for _, m := range doc.Vocabularies["finding_codes"].Members {
		members[m] = true
	}
	for _, c := range []string{"empty-revision", "empty-change-revision", "empty-target-revision",
		"empty-spec_revision", "invalid-spec_revision", "empty-child_pr_revision"} {
		if !members[c] {
			t.Errorf("finding_codes lacks %s", c)
		}
	}
	for _, c := range []string{"empty-version", "empty-change-version", "empty-target-version",
		"empty-spec_version", "invalid-spec_version", "empty-child_pr_version"} {
		if members[c] {
			t.Errorf("finding_codes still lists the retired %s", c)
		}
	}
}
